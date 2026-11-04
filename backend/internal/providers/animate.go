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
// expected in the host's input folder as pc_driver_<motion>.mp4; with DriverDir
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
	img, err := c.upload(ctx, "pc_"+randID()+".png", req.Image)
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
	var queued struct {
		PromptID   string          `json:"prompt_id"`
		NodeErrors json.RawMessage `json:"node_errors"`
	}
	if err := doJSON(ctx, c.Auth, "POST", c.URL+"/prompt", map[string]any{"prompt": wf, "client_id": "partycockpit"}, &queued); err != nil {
		return nil, "", "", fmt.Errorf("comfy prompt: %w", err)
	}
	id := queued.PromptID
	poll := c.Poll
	if poll == 0 {
		poll = 2 * time.Second
	}
	for {
		if err := sleep(ctx, poll); err != nil {
			return nil, "", id, err
		}
		var hist map[string]comfyHistory
		if err := doJSON(ctx, c.Auth, "GET", c.URL+"/history/"+id, nil, &hist); err != nil {
			return nil, "", id, fmt.Errorf("comfy history: %w", err)
		}
		h, ok := hist[id]
		if !ok {
			continue // still queued or running
		}
		if h.Status.StatusStr == "error" {
			return nil, "", id, fmt.Errorf("comfy run failed: %s", h.errorText())
		}
		f, ok := h.clip()
		if !ok {
			if h.Status.Completed {
				return nil, "", id, errors.New("comfy finished without a video output")
			}
			continue
		}
		q := url.Values{"filename": {f.Filename}, "subfolder": {f.Subfolder}, "type": {f.Type}}
		var clip []byte
		if err := doJSON(ctx, c.Auth, "GET", c.URL+"/view?"+q.Encode(), nil, &clip); err != nil {
			return nil, "", id, fmt.Errorf("comfy download: %w", err)
		}
		return clip, strings.TrimPrefix(filepath.Ext(f.Filename), "."), id, nil
	}
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

// clip finds the first video file among the outputs.
func (h comfyHistory) clip() (comfyFile, bool) {
	for _, kinds := range h.Outputs {
		for _, raw := range kinds {
			var files []comfyFile
			if json.Unmarshal(raw, &files) != nil {
				continue
			}
			for _, f := range files {
				switch strings.ToLower(filepath.Ext(f.Filename)) {
				case ".mp4", ".webm", ".gif", ".webp":
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
	name := "pc_driver_" + motion + ".mp4"
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
