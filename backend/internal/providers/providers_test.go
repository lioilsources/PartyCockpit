package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCatalogPromptMatchesRestyler(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Styles) < 80 {
		t.Fatalf("only %d styles — catalog sync broken?", len(c.Styles))
	}
	s, ok := c.Style("anime")
	if !ok {
		t.Fatal("anime missing")
	}
	p := c.Prompt(s, "illustration")
	if !strings.HasPrefix(p, "a painted illustration of a fully clothed person") || !strings.HasSuffix(p, s.Block) {
		t.Fatalf("prompt order (medium first, style last): %q", p)
	}
	if !strings.Contains(c.NegativePrompt("photo"), "nsfw") {
		t.Fatal("negative lost the base negative")
	}
}

func TestRestylerBackend(t *testing.T) {
	var body map[string]any
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "app tok" || r.Header.Get("X-User") != "pc" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/render":
			json.NewDecoder(r.Body).Decode(&body)
			json.NewEncoder(w).Encode(map[string]any{"job_id": "j1", "steps_total": 1, "cost": 1})
		case r.URL.Path == "/api/jobs/j1":
			polls++
			st := "running"
			if polls > 1 {
				st = "done"
			}
			json.NewEncoder(w).Encode(map[string]any{"status": st, "done": polls - 1, "total": 1,
				"steps": map[string]any{"style": map[string]any{"status": map[bool]string{true: "done", false: "running"}[st == "done"]}}})
		case r.URL.Path == "/api/jobs/j1/step/style":
			w.Write([]byte("\x89PNGfake"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rb := &RestylerBackend{URL: srv.URL, Poll: time.Millisecond,
		Auth: Auth{Extra: map[string]string{"Authorization": "app tok", "X-User": "pc"}}}
	out, ext, err := rb.Restyle(context.Background(), RestyleRequest{Photo: []byte("jpeg"), Prompt: "P", Negative: "N"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "\x89PNGfake" || ext != "j1" {
		t.Fatalf("out=%q ext=%q", out, ext)
	}
	steps := body["steps"].([]any)
	step := steps[0].(map[string]any)
	if step["graph"] != "restyle_flux" || step["input"] != "source" || step["prompt"] != "P" || step["id"] != "style" {
		t.Fatalf("step %v", step)
	}
	if body["source"] != "anBlZw==" { // base64("jpeg")
		t.Fatalf("source %v", body["source"])
	}
}
