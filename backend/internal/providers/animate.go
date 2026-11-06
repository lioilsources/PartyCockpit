package providers

// Animation (plan M4): a still becomes a short clip.
//
//	live  — LivePortrait retargets face motion from a driver clip onto the photo
//	        (blink, look around, smile). Seconds of GPU; only the face moves.
//	dance — Wan 2.2 image-to-video with a dance prompt. Minutes of GPU; generative.
//
// Providers: Comfy (a ComfyUI host, e.g. the L40S), RestylerAnimator (Restyler
// /api/render live_portrait step, once Restyler ships it) and MockAnimator
// (animated GIF, no GPU) for development.

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	"image/gif"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
)

type Motion struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Engine  string  `json:"engine"` // live | dance
	Seconds float64 `json:"seconds"`
	Prompt  string  `json:"-"`
}

const danceNegative = "blurry, distorted face, deformed hands, extra limbs, extra people, text, watermark, " +
	"camera movement, zoom, scene cut, nudity"

// Motions is the catalogue. "alive" drives the gallery; the rest are dances a guest picks.
var Motions = []Motion{
	{ID: "alive", Label: "Živý portrét", Engine: "live", Seconds: 10},
	{ID: "disco", Label: "🪩 Disco", Engine: "dance", Seconds: 5,
		Prompt: "the person dances energetically to disco music, pointing one arm up and down, hips swaying, smiling, " +
			"full body visible, static camera, same face and outfit, party lights"},
	{ID: "hiphop", Label: "🧢 Hip-hop", Engine: "dance", Seconds: 5,
		Prompt: "the person does a hip-hop dance, bouncing knees, rolling shoulders and waving arms to the beat, " +
			"full body visible, static camera, same face and outfit"},
	{ID: "robot", Label: "🤖 Robot", Engine: "dance", Seconds: 5,
		Prompt: "the person does the robot dance: elbows bent at 90 degrees, forearms moving in sharp stop-motion steps, " +
			"head turning in jerky ticks, stiff torso, full body visible, static camera, same face and outfit"},
	{ID: "jump", Label: "🙌 Skákání", Engine: "dance", Seconds: 5,
		Prompt: "the person jumps up and down to the music with both arms raised, celebrating, " +
			"full body visible, static camera, same face and outfit"},
}

func MotionByID(id string) (Motion, bool) {
	for _, m := range Motions {
		if m.ID == id {
			return m, true
		}
	}
	return Motion{}, false
}

type AnimateRequest struct {
	Image  []byte // JPEG/PNG
	Motion Motion
}

type Animator interface {
	Name() string
	// Animate returns the clip, its file extension ("mp4", "gif") and the
	// provider's job id (for the jobs table).
	Animate(ctx context.Context, req AnimateRequest) (clip []byte, ext, externalID string, err error)
}

var ErrUnsupportedMotion = errors.New("provider does not support this motion")

// ── ComfyUI ────────────────────────────────────────────────────────────────

//go:embed workflows/*.json
var workflowFS embed.FS

// Comfy talks to a ComfyUI host directly. Driver clips for LivePortrait are
// expected in the host's input folder as party_driver_<motion>.mp4; with DriverDir
// set they are uploaded from there on first use.
type Comfy struct {
	URL       string
	Auth      Auth
	DriverDir string
	Poll      time.Duration

	mu       sync.Mutex
	uploaded map[string]bool
}

func (c *Comfy) Name() string { return "comfy" }

func (c *Comfy) Animate(ctx context.Context, req AnimateRequest) ([]byte, string, string, error) {
	img, err := c.upload(ctx, "party_"+randID()+".png", req.Image)
	if err != nil {
		return nil, "", "", fmt.Errorf("upload photo: %w", err)
	}
	vars := map[string]any{"__IMAGE__": img, "__SEED__": seed()}
	tpl := "workflows/wan_i2v.json"
	if req.Motion.Engine == "live" {
		tpl = "workflows/live_portrait.json"
		if vars["__DRIVER__"], err = c.driver(ctx, req.Motion.ID); err != nil {
			return nil, "", "", err
		}
	} else {
		vars["__PROMPT__"], vars["__NEGATIVE__"] = req.Motion.Prompt, danceNegative
	}
	wf, err := loadWorkflow(tpl, vars)
	if err != nil {
		return nil, "", "", err
	}
	f, out, id, err := c.run(ctx, wf, isVideoFile)
	if err != nil {
		return nil, "", id, err
	}
	return out, strings.TrimPrefix(filepath.Ext(f.Filename), "."), id, nil
}

// Restyle runs Restyler's restyle_flux graph (FLUX dev + PuLID keeps the face,
// depth ControlNet keeps the pose) straight on ComfyUI: workflows/flux_restyle.json.
func (c *Comfy) Restyle(ctx context.Context, req RestyleRequest) ([]byte, string, error) {
	img, err := c.upload(ctx, "party_"+randID()+".jpg", req.Photo)
	if err != nil {
		return nil, "", fmt.Errorf("upload photo: %w", err)
	}
	w, h := 832, 1216
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(req.Photo)); err == nil {
		w, h = sdxlBucket(cfg.Width, cfg.Height)
	}
	wf, err := loadWorkflow("workflows/flux_restyle.json", map[string]any{
		"__IMAGE__": img, "__PROMPT__": req.Prompt, "__SEED__": seed(), "__W__": w, "__H__": h,
	})
	if err != nil {
		return nil, "", err
	}
	_, out, id, err := c.run(ctx, wf, isImageFile)
	return out, id, err
}

// sdxlBuckets: the ~1 MP sizes Restyler renders in (tgbot/imagesize.py), so
// the depth hint and the face are not cropped off a landscape photo.
var sdxlBuckets = [][2]int{{1024, 1024}, {896, 1152}, {1152, 896}, {832, 1216}, {1216, 832}, {768, 1344}, {1344, 768}}

// sdxlBucket picks the bucket whose aspect ratio is closest to w×h.
func sdxlBucket(w, h int) (int, int) {
	best, bestD := sdxlBuckets[3], math.Inf(1)
	if w <= 0 || h <= 0 {
		return best[0], best[1]
	}
	r := math.Log(float64(w) / float64(h))
	for _, b := range sdxlBuckets {
		if d := math.Abs(math.Log(float64(b[0])/float64(b[1])) - r); d < bestD {
			best, bestD = b, d
		}
	}
	return best[0], best[1]
}

// run queues a workflow, polls its history and downloads the first output
// file that want accepts.
func (c *Comfy) run(ctx context.Context, wf map[string]any, want func(name string) bool) (comfyFile, []byte, string, error) {
	var queued struct {
		PromptID   string          `json:"prompt_id"`
		NodeErrors json.RawMessage `json:"node_errors"`
	}
	if err := doJSON(ctx, c.Auth, "POST", c.URL+"/prompt", map[string]any{"prompt": wf, "client_id": "partycockpit"}, &queued); err != nil {
		return comfyFile{}, nil, "", fmt.Errorf("comfy prompt: %w", err)
	}
	id := queued.PromptID
	poll := c.Poll
	if poll == 0 {
		poll = 2 * time.Second
	}
	for {
		if err := sleep(ctx, poll); err != nil {
			return comfyFile{}, nil, id, err
		}
		var hist map[string]comfyHistory
		if err := doJSON(ctx, c.Auth, "GET", c.URL+"/history/"+id, nil, &hist); err != nil {
			return comfyFile{}, nil, id, fmt.Errorf("comfy history: %w", err)
		}
		h, ok := hist[id]
		if !ok {
			continue // still queued or running
		}
		if h.Status.StatusStr == "error" {
			return comfyFile{}, nil, id, fmt.Errorf("comfy run failed: %s", h.errorText())
		}
		f, ok := h.output(want)
		if !ok {
			if h.Status.Completed {
				return comfyFile{}, nil, id, errors.New("comfy finished without the expected output")
			}
			continue
		}
		q := url.Values{"filename": {f.Filename}, "subfolder": {f.Subfolder}, "type": {f.Type}}
		var out []byte
		if err := doJSON(ctx, c.Auth, "GET", c.URL+"/view?"+q.Encode(), nil, &out); err != nil {
			return comfyFile{}, nil, id, fmt.Errorf("comfy download: %w", err)
		}
		return f, out, id, nil
	}
}

func isVideoFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".webm", ".gif", ".webp":
		return true
	}
	return false
}

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}

type comfyFile struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

type comfyHistory struct {
	Status struct {
		StatusStr string          `json:"status_str"`
		Completed bool            `json:"completed"`
		Messages  json.RawMessage `json:"messages"`
	} `json:"status"`
	// node id → output kind (VHS: "gifs", SaveVideo: "images"/"videos") → files
	Outputs map[string]map[string]json.RawMessage `json:"outputs"`
}

// output finds the first output file that want accepts (temp previews skipped).
func (h comfyHistory) output(want func(name string) bool) (comfyFile, bool) {
	for _, kinds := range h.Outputs {
		for _, raw := range kinds {
			var files []comfyFile
			if json.Unmarshal(raw, &files) != nil {
				continue
			}
			for _, f := range files {
				if f.Type != "temp" && want(f.Filename) {
					return f, true
				}
			}
		}
	}
	return comfyFile{}, false
}

func (h comfyHistory) errorText() string {
	s := string(h.Status.Messages)
	if len(s) > 400 {
		s = s[len(s)-400:]
	}
	return s
}

// upload stores a file in the host's input folder (works for video too).
func (c *Comfy) upload(ctx context.Context, name string, data []byte) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", name)
	fw.Write(data)
	mw.WriteField("overwrite", "true")
	mw.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+"/upload/image", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Auth.apply(req)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("%d %s", resp.StatusCode, errDetail(b))
	}
	var out struct {
		Name      string `json:"name"`
		Subfolder string `json:"subfolder"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.Subfolder != "" {
		return out.Subfolder + "/" + out.Name, nil
	}
	return out.Name, nil
}

func (c *Comfy) driver(ctx context.Context, motion string) (string, error) {
	name := "party_driver_" + motion + ".mp4"
	if c.DriverDir == "" {
		return name, nil // placed on the host by hand
	}
	c.mu.Lock()
	done := c.uploaded[name]
	c.mu.Unlock()
	if done {
		return name, nil
	}
	data, err := os.ReadFile(filepath.Join(c.DriverDir, motion+".mp4"))
	if err != nil {
		return "", fmt.Errorf("driver %q: %w", motion, err)
	}
	if _, err := c.upload(ctx, name, data); err != nil {
		return "", fmt.Errorf("upload driver: %w", err)
	}
	c.mu.Lock()
	if c.uploaded == nil {
		c.uploaded = map[string]bool{}
	}
	c.uploaded[name] = true
	c.mu.Unlock()
	return name, nil
}

// loadWorkflow reads an API-format workflow and replaces "__X__" placeholder
// strings with vars (keeping the value's JSON type, so a seed stays a number).
func loadWorkflow(name string, vars map[string]any) (map[string]any, error) {
	b, err := workflowFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var wf map[string]any
	if err := json.Unmarshal(b, &wf); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	var walk func(v any) any
	walk = func(v any) any {
		switch t := v.(type) {
		case map[string]any:
			for k, x := range t {
				t[k] = walk(x)
			}
		case []any:
			for i, x := range t {
				t[i] = walk(x)
			}
		case string:
			if r, ok := vars[t]; ok {
				return r
			}
		}
		return v
	}
	walk(wf)
	return wf, nil
}

func randID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func seed() int64 {
	var b [8]byte
	rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]) >> 12)
}

// ── Restyler ───────────────────────────────────────────────────────────────

// RestylerAnimator uses Restyler's live_portrait step. Restyler has no dance
// preset, so dances need the Comfy provider.
type RestylerAnimator struct{ R *RestylerBackend }

func (r RestylerAnimator) Name() string { return "restyler" }

func (r RestylerAnimator) Animate(ctx context.Context, req AnimateRequest) ([]byte, string, string, error) {
	if req.Motion.Engine != "live" {
		return nil, "", "", ErrUnsupportedMotion
	}
	clip, id, err := r.R.render(ctx, req.Image, map[string]any{
		"id": "live", "graph": "live_portrait", "input": "source", "motion": req.Motion.ID,
	})
	return clip, "mp4", id, err
}

// ── mock ───────────────────────────────────────────────────────────────────

// MockAnimator makes an animated GIF from the photo: a slow "breathing" zoom for
// live, a bounce and sway for dances. Enough to build the stage without a GPU.
type MockAnimator struct{ Delay time.Duration }

func (MockAnimator) Name() string { return "mock" }

func (m MockAnimator) Animate(ctx context.Context, req AnimateRequest) ([]byte, string, string, error) {
	if err := sleep(ctx, m.Delay); err != nil {
		return nil, "", "", err
	}
	src, _, err := image.Decode(bytes.NewReader(req.Image))
	if err != nil {
		return nil, "", "", err
	}
	const w = 320
	b := src.Bounds()
	h := w * b.Dy() / b.Dx()
	base := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(base, base.Bounds(), src, b, xdraw.Src, nil)

	frames := 16
	out := &gif.GIF{LoopCount: 0}
	for i := range frames {
		ph := 2 * math.Pi * float64(i) / float64(frames)
		var dst image.Rectangle
		if req.Motion.Engine == "live" {
			z := 1 + 0.03*(1-math.Cos(ph))/2 // zoom in and back out
			dw, dh := int(float64(w)*z), int(float64(h)*z)
			dst = image.Rect((w-dw)/2, (h-dh)/2, (w-dw)/2+dw, (h-dh)/2+dh)
		} else {
			dx := int(14 * math.Sin(ph))              // sway
			dy := int(-18 * math.Abs(math.Sin(2*ph))) // two bounces per loop
			dst = image.Rect(dx, dy, w+dx, h+dy)
		}
		frame := image.NewRGBA(base.Bounds())
		xdraw.ApproxBiLinear.Scale(frame, dst, base, base.Bounds(), xdraw.Src, nil)
		p := image.NewPaletted(base.Bounds(), palette.Plan9)
		xdraw.FloydSteinberg.Draw(p, p.Bounds(), frame, image.Point{})
		out.Image = append(out.Image, p)
		out.Delay = append(out.Delay, 8)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, out); err != nil {
		return nil, "", "", err
	}
	return buf.Bytes(), "gif", "", nil
}
