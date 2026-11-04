// Package providers wraps the GPU services a party uses. Each provider is only an
// envelope over an existing service (Restyler backend, AiStack); PartyCockpit
// never waits on the GPU synchronously — jobs run in the worker pool.
package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

// RestyleRequest is one guest photo plus the chosen look.
type RestyleRequest struct {
	Photo    []byte // normalised JPEG (see Normalise)
	Style    Style
	Prompt   string
	Negative string
}

type Restyler interface {
	Name() string
	Restyle(ctx context.Context, req RestyleRequest) (png []byte, externalID string, err error)
}

// Classifier returns an NSFW score in 0..1, or -1 when it cannot tell.
type Classifier interface {
	NSFW(ctx context.Context, img []byte) (float64, error)
}

// Auth carries the headers shared by services behind Cloudflare Access.
type Auth struct {
	CFClientID, CFClientSecret string
	Extra                      map[string]string
}

// Headers returns the CF Access headers alone (for clients outside this package).
func (a Auth) Headers() map[string]string {
	if a.CFClientID == "" {
		return nil
	}
	return map[string]string{"CF-Access-Client-Id": a.CFClientID, "CF-Access-Client-Secret": a.CFClientSecret}
}

func (a Auth) apply(r *http.Request) {
	if a.CFClientID != "" {
		r.Header.Set("CF-Access-Client-Id", a.CFClientID)
		r.Header.Set("CF-Access-Client-Secret", a.CFClientSecret)
	}
	for k, v := range a.Extra {
		r.Header.Set(k, v)
	}
}

var httpc = &http.Client{Timeout: 2 * time.Minute}

func doJSON(ctx context.Context, a Auth, method, url string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	a.apply(req)
	resp, err := httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: %d %s", method, url, resp.StatusCode, errDetail(b))
	}
	if out == nil {
		return nil
	}
	if p, ok := out.(*[]byte); ok {
		*p = b
		return nil
	}
	return json.Unmarshal(b, out)
}

func errDetail(b []byte) string {
	var e struct {
		Detail string `json:"detail"`
		Error  string `json:"error"`
	}
	if json.Unmarshal(b, &e) == nil && (e.Detail != "" || e.Error != "") {
		return e.Detail + e.Error
	}
	if len(b) > 200 {
		b = b[:200]
	}
	return strings.TrimSpace(string(b))
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ── Restyler backend (restyle_flux keeps the face; Restyler's own graph) ──

// RestylerBackend talks to the Restyler render API: POST /api/render with a
// single restyle_flux step, poll GET /api/jobs/{id}, fetch the step output.
type RestylerBackend struct {
	URL  string
	Auth Auth // Authorization: app <token>, X-User, CF Access
	Poll time.Duration
}

func (r *RestylerBackend) Name() string { return "restyler" }

func (r *RestylerBackend) Restyle(ctx context.Context, req RestyleRequest) ([]byte, string, error) {
	return r.render(ctx, req.Photo, map[string]any{
		"id": "style", "graph": "restyle_flux", "input": "source",
		"prompt": req.Prompt, "negative": req.Negative,
	})
}

// render runs a one-step /api/render job and returns that step's output
// (PNG for image graphs, mp4 for live_portrait / wan_motion).
func (r *RestylerBackend) render(ctx context.Context, source []byte, step map[string]any) ([]byte, string, error) {
	id, _ := step["id"].(string)
	body := map[string]any{
		"source": base64.StdEncoding.EncodeToString(source),
		"steps":  []map[string]any{step},
	}
	var job struct {
		JobID string `json:"job_id"`
	}
	if err := doJSON(ctx, r.Auth, "POST", r.URL+"/api/render", body, &job); err != nil {
		return nil, "", fmt.Errorf("restyler render: %w", err)
	}
	poll := r.Poll
	if poll == 0 {
		poll = 3 * time.Second
	}
	for {
		if err := sleep(ctx, poll); err != nil {
			return nil, job.JobID, err
		}
		var st struct {
			Status string `json:"status"`
			Error  string `json:"error"`
			Steps  map[string]struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"steps"`
		}
		if err := doJSON(ctx, r.Auth, "GET", r.URL+"/api/jobs/"+job.JobID, nil, &st); err != nil {
			return nil, job.JobID, fmt.Errorf("restyler poll: %w", err)
		}
		// status decides the end of a job, not done/total (Restyler CLAUDE.md).
		switch st.Status {
		case "done":
			if s := st.Steps[id]; s.Status != "done" {
				return nil, job.JobID, fmt.Errorf("restyler step %s: %s", s.Status, s.Error)
			}
			var out []byte
			if err := doJSON(ctx, r.Auth, "GET", r.URL+"/api/jobs/"+job.JobID+"/step/"+id, nil, &out); err != nil {
				return nil, job.JobID, fmt.Errorf("restyler output: %w", err)
			}
			return out, job.JobID, nil
		case "error":
			return nil, job.JobID, fmt.Errorf("restyler job: %s", st.Error)
		}
	}
}

// ── AiStack flux-kontext NIM (fallback when Restyler is not reachable) ────

type Kontext struct {
	URL  string // AiStack gateway
	Auth Auth
	Poll time.Duration
}

func (k *Kontext) Name() string { return "kontext" }

func (k *Kontext) Restyle(ctx context.Context, req RestyleRequest) ([]byte, string, error) {
	body := map[string]any{
		"prompt":       "Restyle this photo, keep the person's face, identity, pose and composition: " + req.Style.Block,
		"image":        "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(req.Photo),
		"aspect_ratio": "match_input_image", "steps": 30, "cfg_scale": 3.5,
	}
	var job struct {
		ID string `json:"id"`
	}
	base := k.URL + "/nim/flux-kontext"
	if err := doJSON(ctx, k.Auth, "POST", base+"/v1/infer", body, &job); err != nil {
		return nil, "", fmt.Errorf("kontext infer: %w", err)
	}
	poll := k.Poll
	if poll == 0 {
		poll = 2 * time.Second
	}
	for {
		if err := sleep(ctx, poll); err != nil {
			return nil, job.ID, err
		}
		var st struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := doJSON(ctx, k.Auth, "GET", base+"/jobs/"+job.ID, nil, &st); err != nil {
			return nil, job.ID, fmt.Errorf("kontext poll: %w", err)
		}
		switch st.Status {
		case "done":
			var png []byte
			err := doJSON(ctx, k.Auth, "GET", base+"/jobs/"+job.ID+"/result", nil, &png)
			return png, job.ID, err
		case "error":
			return nil, job.ID, fmt.Errorf("kontext: %s", st.Error)
		}
	}
}

// ── Mock (no GPU): posterise + tint, so the flow is testable at home ──────

type Mock struct{ Delay time.Duration }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Restyle(ctx context.Context, req RestyleRequest) ([]byte, string, error) {
	if err := sleep(ctx, m.Delay); err != nil {
		return nil, "", err
	}
	src, _, err := image.Decode(bytes.NewReader(req.Photo))
	if err != nil {
		return nil, "", fmt.Errorf("mock decode: %w", err)
	}
	h := 0
	for _, c := range req.Style.ID {
		h = h*31 + int(c)
	}
	tint := color.RGBA{uint8(h), uint8(h >> 8), uint8(h >> 16), 255}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			post := func(v uint32, t uint8) uint8 { return uint8((int(v>>8)/64*64+int(t))/2 + 40) }
			dst.Set(x, y, color.RGBA{post(r, tint.R), post(g, tint.G), post(bl, tint.B), 255})
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, "", err
	}
	return out.Bytes(), "", nil
}

// ── Moderation ─────────────────────────────────────────────────────────────

// NoClassifier: everything waits for the operator (strict is the default anyway).
type NoClassifier struct{}

func (NoClassifier) NSFW(context.Context, []byte) (float64, error) { return -1, nil }

// WD14 is FineTuneGallery's tagger behind AiStack: POST /v1/image/tag (raw bytes)
// → {"rating": {"general","sensitive","questionable","explicit"}}.
type WD14 struct {
	URL  string
	Auth Auth
}

func (w *WD14) NSFW(ctx context.Context, img []byte) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", w.URL+"/v1/image/tag", bytes.NewReader(img))
	if err != nil {
		return -1, err
	}
	req.Header.Set("Content-Type", http.DetectContentType(img))
	w.Auth.apply(req)
	resp, err := httpc.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return -1, fmt.Errorf("tagger: %d %s", resp.StatusCode, errDetail(b))
	}
	var out struct {
		Rating map[string]float64 `json:"rating"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return -1, err
	}
	if out.Rating == nil {
		return -1, errors.New("tagger: no rating")
	}
	return max(out.Rating["questionable"], out.Rating["explicit"]), nil
}

// ── Images ─────────────────────────────────────────────────────────────────

// MaxSide matches Restyler's normalisePhoto (≤ 1536).
const MaxSide = 1536

// Normalise decodes a guest upload, scales it to ≤ MaxSide and re-encodes it as
// JPEG, which also drops EXIF/GPS metadata. Orientation is fixed by the guest
// page (the browser applies EXIF when drawing to canvas).
// MaxUploadPixels bounds a decoded upload: a small PNG can claim 20 000 ×
// 20 000 px and take gigabytes to decode on the party box (D19).
const MaxUploadPixels = 36_000_000

func Normalise(raw []byte) ([]byte, int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode upload: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxUploadPixels {
		return nil, 0, 0, fmt.Errorf("fotka je moc velká (max 36 Mpx)")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode upload: %w", err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 64 || h < 64 {
		return nil, 0, 0, errors.New("image too small")
	}
	if s := max(w, h); s > MaxSide {
		w, h = w*MaxSide/s, h*MaxSide/s
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
		src = dst
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: 90}); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), w, h, nil
}
