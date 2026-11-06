package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func pngBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 60))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	img.Set(0, 0, color.White)
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// fakeComfy records what the provider sends and answers like ComfyUI.
type fakeComfy struct {
	mu       sync.Mutex
	uploads  []string
	workflow map[string]any
	polls    int
	out      string // output file name; default a LivePortrait mp4
}

func (f *fakeComfy) output() string {
	if f.out != "" {
		return f.out
	}
	return "party_live_00001.mp4"
}

func (f *fakeComfy) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload/image", func(w http.ResponseWriter, r *http.Request) {
		file, hdr, err := r.FormFile("image")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		io.Copy(io.Discard, file)
		f.mu.Lock()
		f.uploads = append(f.uploads, hdr.Filename)
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"name": hdr.Filename, "subfolder": "", "type": "input"})
	})
	mux.HandleFunc("POST /prompt", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt map[string]any `json:"prompt"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.workflow = body.Prompt
		f.mu.Unlock()
		w.Write([]byte(`{"prompt_id":"p1","number":1,"node_errors":{}}`))
	})
	mux.HandleFunc("GET /history/p1", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.polls++
		n := f.polls
		f.mu.Unlock()
		if n < 2 {
			w.Write([]byte(`{}`)) // still running
			return
		}
		w.Write([]byte(`{"p1":{"status":{"status_str":"success","completed":true},
			"outputs":{"6":{"images":[{"filename":"preview.png","subfolder":"","type":"temp"}]},
			"8":{"gifs":[{"filename":"` + f.output() + `","subfolder":"","type":"output"}]}}}}`))
	})
	mux.HandleFunc("GET /view", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filename") != f.output() || r.URL.Query().Get("type") != "output" {
			http.Error(w, "bad view", 400)
			return
		}
		w.Write([]byte("OUTDATA"))
	})
	return mux
}

func TestComfyLivePortrait(t *testing.T) {
	f := &fakeComfy{}
	ts := httptest.NewServer(f.handler())
	defer ts.Close()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alive.mp4"), []byte("driver"), 0o644)

	c := &Comfy{URL: ts.URL, DriverDir: dir, Poll: time.Millisecond}
	m, _ := MotionByID("alive")
	for range 2 {
		clip, ext, id, err := c.Animate(context.Background(), AnimateRequest{Image: pngBytes(), Motion: m})
		if err != nil {
			t.Fatal(err)
		}
		if string(clip) != "OUTDATA" || ext != "mp4" || id != "p1" {
			t.Fatalf("got %q %q %q", clip, ext, id)
		}
		f.polls = 0
	}
	// driver uploaded once, photo every time
	drivers := 0
	for _, u := range f.uploads {
		if u == "party_driver_alive.mp4" {
			drivers++
		}
	}
	if drivers != 1 || len(f.uploads) != 3 {
		t.Fatalf("uploads %v", f.uploads)
	}
	raw, _ := json.Marshal(f.workflow)
	if strings.Contains(string(raw), "__") {
		t.Fatalf("placeholder left in workflow: %s", raw)
	}
	if !strings.Contains(string(raw), `"party_driver_alive.mp4"`) {
		t.Fatal("driver not in workflow")
	}
}

func TestComfyRestyle(t *testing.T) {
	f := &fakeComfy{out: "party_restyle_00001_.png"}
	ts := httptest.NewServer(f.handler())
	defer ts.Close()
	c := &Comfy{URL: ts.URL, Poll: time.Millisecond}
	out, id, err := c.Restyle(context.Background(), RestyleRequest{Photo: pngBytes(), Prompt: "a Vermeer portrait"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "OUTDATA" || id != "p1" {
		t.Fatalf("got %q %q", out, id)
	}
	if len(f.uploads) != 1 || !strings.HasPrefix(f.uploads[0], "party_") {
		t.Fatalf("uploads %v (cleanup on the GPU host needs the party_ prefix)", f.uploads)
	}
	raw, _ := json.Marshal(f.workflow)
	if strings.Contains(string(raw), `"__`) {
		t.Fatalf("placeholder left: %s", raw)
	}
	// 40×60 photo → the 2:3 portrait bucket, in the resize and the latent
	for _, node := range []string{"5", "16"} {
		in := f.workflow[node].(map[string]any)["inputs"].(map[string]any)
		if in["width"] != 832.0 || in["height"] != 1216.0 {
			t.Fatalf("node %s size %v×%v", node, in["width"], in["height"])
		}
	}
	if p := f.workflow["8"].(map[string]any)["inputs"].(map[string]any)["text"]; p != "a Vermeer portrait" {
		t.Fatalf("prompt %v", p)
	}
}

func TestSDXLBucket(t *testing.T) {
	for _, c := range []struct{ w, h, bw, bh int }{
		{3024, 4032, 896, 1152}, {4032, 3024, 1152, 896}, {1080, 1920, 768, 1344}, {1000, 1000, 1024, 1024}, {0, 0, 832, 1216},
	} {
		if w, h := sdxlBucket(c.w, c.h); w != c.bw || h != c.bh {
			t.Errorf("%d×%d → %d×%d, want %d×%d", c.w, c.h, w, h, c.bw, c.bh)
		}
	}
}

func TestComfyDanceWorkflowFilled(t *testing.T) {
	m, _ := MotionByID("disco")
	wf, err := loadWorkflow("workflows/wan_i2v.json", map[string]any{
		"__IMAGE__": "x.png", "__SEED__": int64(42), "__PROMPT__": m.Prompt, "__NEGATIVE__": danceNegative,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(wf)
	if strings.Contains(string(raw), `"__`) {
		t.Fatalf("placeholder left: %s", raw)
	}
	if !strings.Contains(string(raw), `"seed":42`) && !strings.Contains(string(raw), `"noise_seed":42`) {
		t.Fatalf("seed must stay a number: %s", raw)
	}
}

func TestMockAnimatorGIF(t *testing.T) {
	for _, id := range []string{"alive", "jump"} {
		m, _ := MotionByID(id)
		clip, ext, _, err := MockAnimator{}.Animate(context.Background(), AnimateRequest{Image: pngBytes(), Motion: m})
		if err != nil || ext != "gif" {
			t.Fatal(err, ext)
		}
		g, err := gif.DecodeAll(bytes.NewReader(clip))
		if err != nil || len(g.Image) != 16 {
			t.Fatalf("%s: %v frames=%d", id, err, len(g.Image))
		}
	}
}
