package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
)

// fakeFineTune: one session, the source photo, two prompts × 9 renders and
// one unsafe render; plus an upstream Ol1nLLM pack in the index.
func fakeFineTune(t *testing.T) *httptest.Server {
	thumbs := map[string][]byte{}
	var items []map[string]any
	add := func(id int, prompt, origin, band string, c color.RGBA) {
		img := image.NewRGBA(image.Rect(0, 0, 30, 40))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
		var b bytes.Buffer
		png.Encode(&b, img)
		sum := sha256.Sum256(append(b.Bytes(), byte(id)))
		sha := hex.EncodeToString(sum[:])
		thumbs[sha] = b.Bytes()
		it := map[string]any{"id": id, "sha256": sha, "prompt": prompt, "styleId": fmt.Sprintf("s%d", id%3), "origin": origin}
		if band != "" {
			it["taxo"] = map[string]string{"band": band}
		}
		items = append(items, it)
	}
	add(1, "", "upload", "", color.RGBA{9, 9, 9, 255})
	for i := range 9 {
		add(10+i, "Hero in the rain", "generated", "safe", color.RGBA{uint8(20 * i), 50, 200, 255})
		add(30+i, "hero at the beach", "generated", "", color.RGBA{220, uint8(20 * i), 40, 255})
	}
	add(99, "Hero in the rain", "generated", "explicit", color.RGBA{255, 0, 0, 255})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{map[string]any{"id": 42, "title": "Ema restyle", "imageCount": len(items)}})
	})
	mux.HandleFunc("/api/images", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("session") != "42" {
			json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
			return
		}
		// two pages
		if r.URL.Query().Get("cursor") == "" {
			json.NewEncoder(w).Encode(map[string]any{"items": items[:10], "nextCursor": "p2"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items[10:], "nextCursor": nil})
	})
	mux.HandleFunc("/thumb/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(thumbs[strings.TrimPrefix(r.URL.Path, "/thumb/")])
	})
	mux.HandleFunc("/stylemaps/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"packs":[{"id":"vermeer","title":"Vermeer","n":300,"aspect":0.75,"atlas":"vermeer/atlas.webp"}]}`))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestStyleMapsFromSession(t *testing.T) {
	ft := fakeFineTune(t)
	e := newEnvWith(t, func(c *config.Config) {})
	e.srv.Gallery = gallery.New(ft.URL, filepath.Join(e.srv.Cfg.DataDir, "gc"), false, nil)

	var sessions []gallery.Session
	if c := e.do("GET", "/api/parties/"+e.code+"/finetune/sessions", nil, &sessions); c != 200 || len(sessions) != 1 || sessions[0].ID != "42" {
		t.Fatalf("sessions %d %+v", c, sessions)
	}
	var groups []struct {
		Key    string `json:"key"`
		Prompt string `json:"prompt"`
		N      int    `json:"n"`
		Pack   string `json:"pack"`
		Built  bool   `json:"built"`
		Job    *smJob `json:"job"`
	}
	e.do("GET", "/api/parties/"+e.code+"/finetune/sessions/42/groups", nil, &groups)
	// the upload and the unsafe render are out; two prompts of 9
	if len(groups) != 2 || groups[0].N != 9 || groups[1].N != 9 || groups[0].Built {
		t.Fatalf("groups %+v", groups)
	}
	if c := e.do("POST", "/api/parties/"+e.code+"/stylemaps/build", map[string]any{"session": "42", "groups": []string{groups[0].Key, groups[1].Key}}, nil); c != 202 {
		t.Fatalf("build: %d", c)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		e.do("GET", "/api/parties/"+e.code+"/finetune/sessions/42/groups", nil, &groups)
		if groups[0].Built && groups[1].Built {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !groups[0].Built || !groups[1].Built || groups[0].Job == nil || groups[0].Job.Status != "done" {
		t.Fatalf("not built: %+v %+v", groups[0], groups[0].Job)
	}

	var idx struct {
		Packs []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Source string `json:"source"`
		} `json:"packs"`
	}
	e.do("GET", "/stylemaps/index.json", nil, &idx)
	if len(idx.Packs) != 3 || idx.Packs[2].ID != "vermeer" || idx.Packs[2].Source != "finetune" || !strings.HasPrefix(idx.Packs[0].Title, "Ema restyle · ") {
		t.Fatalf("index %+v", idx.Packs)
	}
	if res, _ := http.Get(e.ts.URL + "/stylemaps/" + groups[0].Pack + "/t/0.jpg"); res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("local thumb: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if c := e.do("DELETE", "/api/parties/"+e.code+"/stylemaps/"+groups[0].Pack, nil, nil); c != 204 {
		t.Fatalf("delete: %d", c)
	}
	if c := e.do("DELETE", "/api/parties/"+e.code+"/stylemaps/vermeer", nil, nil); c != 404 {
		t.Fatalf("an upstream pack cannot be deleted: %d", c)
	}

	// uplink gone: the local pack and the cached upstream entry stay listed
	ft.Close()
	e.do("GET", "/stylemaps/index.json", nil, &idx)
	if len(idx.Packs) != 2 {
		t.Fatalf("offline index %+v", idx.Packs)
	}
}
