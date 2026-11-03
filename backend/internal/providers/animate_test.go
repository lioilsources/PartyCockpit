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
			"outputs":{"8":{"gifs":[{"filename":"partycockpit_live_00001.mp4","subfolder":"","type":"output"}]}}}}`))
	})
	mux.HandleFunc("GET /view", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filename") != "partycockpit_live_00001.mp4" || r.URL.Query().Get("type") != "output" {
			http.Error(w, "bad view", 400)
			return
		}
		w.Write([]byte("MP4DATA"))
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
		if string(clip) != "MP4DATA" || ext != "mp4" || id != "p1" {
			t.Fatalf("got %q %q %q", clip, ext, id)
		}
		f.polls = 0
	}
	// driver uploaded once, photo every time
	drivers := 0
	for _, u := range f.uploads {
		if u == "pc_driver_alive.mp4" {
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
	if !strings.Contains(string(raw), `"pc_driver_alive.mp4"`) {
		t.Fatal("driver not in workflow")
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
