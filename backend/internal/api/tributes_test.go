package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
)

type tributeList struct {
	Configured bool   `json:"configured"`
	Error      string `json:"error"`
	Items      []struct {
		ID        string `json:"id"`
		Installed bool   `json:"installed"`
		Remote    bool   `json:"remote"`
		Job       *struct {
			Status string `json:"status"`
		} `json:"job"`
	} `json:"items"`
}

func TestTributeRoutes(t *testing.T) {
	e := newEnvWith(t, func(c *config.Config) { c.PromoURL = "mock" })
	base := "/api/parties/" + e.code + "/tributes"

	var l tributeList
	if c := e.do("GET", base, nil, &l); c != 200 || !l.Configured || len(l.Items) != 6 || l.Items[0].Installed {
		t.Fatalf("remote list %d %+v", c, l)
	}
	if c := e.do("POST", base+"/import", map[string]any{"ids": []string{"6", "5"}}, nil); c != 202 {
		t.Fatalf("import %d", c)
	}
	if c := e.do("POST", base+"/import", map[string]any{"ids": []string{"404"}}, nil); c != 404 {
		t.Fatalf("unknown id %d", c)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !(e.srv.Tributes.Has("6") && e.srv.Tributes.Has("5")) {
		time.Sleep(20 * time.Millisecond)
	}
	var stage []stageTribute
	if c := e.do("GET", "/api/tributes", nil, &stage); c != 200 || len(stage) != 2 || stage[0].ID != "6" || stage[0].Credit == "" || len(stage[0].Frames) != 4 {
		t.Fatalf("stage list %d %+v", c, stage)
	}
	for path, want := range map[string]int{
		"/tributes/6/0s.jpg?v=1":     200,
		"/tributes/6/3.jpg":          200,
		"/tributes/6/tribute.json":   404,
		"/tributes/6/4.jpg":          404,
		"/tributes/6/..%2F5%2F0.jpg": 404,
		"/tributes/1/0.jpg":          404,
	} {
		res, err := http.Get(e.ts.URL + path)
		if err != nil || res.StatusCode != want {
			t.Fatalf("%s: %v %d, want %d", path, err, res.StatusCode, want)
		}
	}

	// the preview of a tribute not on disk needs the file key, not a bearer
	thumb := base + "/3/thumb/0"
	if res, _ := http.Get(e.ts.URL + thumb); res.StatusCode != 404 {
		t.Fatalf("thumb without key %d", res.StatusCode)
	}
	p, _ := e.srv.Store.Party(t.Context(), e.code)
	if res, _ := http.Get(e.ts.URL + thumb + "?k=" + fileKey(p)); res.StatusCode != 200 {
		t.Fatalf("thumb with key %d", res.StatusCode)
	}

	if c := e.do("DELETE", base+"/5", nil, nil); c != 204 || e.srv.Tributes.Has("5") {
		t.Fatalf("delete %d", c)
	}
	if c := e.do("DELETE", base+"/5", nil, nil); c != 404 {
		t.Fatalf("delete twice %d", c)
	}

	e.token = "wrong"
	for _, m := range []string{"GET", "POST"} {
		path := base
		if m == "POST" {
			path += "/import"
		}
		if c := e.do(m, path, map[string]any{"ids": []string{"4"}}, nil); c != 401 {
			t.Fatalf("%s without the cockpit token: %d", m, c)
		}
	}
}

func TestTributesOffAndDown(t *testing.T) {
	// not configured: tributes on disk still listed and played
	e := newEnv(t)
	dir := filepath.Join(e.srv.Cfg.DataDir, "tributes", "12")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "tribute.json"), []byte(`{"schema":1,"id":"12","credit":"@a","frames":[{"w":8,"h":8,"sw":8,"sh":8}]}`), 0o644)
	os.WriteFile(filepath.Join(dir, "0s.jpg"), []byte("jpg"), 0o644)
	var l tributeList
	if c := e.do("GET", "/api/parties/"+e.code+"/tributes", nil, &l); c != 200 || l.Configured || l.Error != "" || len(l.Items) != 1 || !l.Items[0].Installed || l.Items[0].Remote {
		t.Fatalf("off %d %+v", c, l)
	}
	if c := e.do("POST", "/api/parties/"+e.code+"/tributes/import", map[string]any{"ids": []string{"12"}}, nil); c != 409 {
		t.Fatalf("import while off %d", c)
	}

	// promo-api down: the disk is listed with an error
	d := newEnvWith(t, func(c *config.Config) { c.PromoURL = "http://127.0.0.1:1"; c.PromoToken = "x" })
	if c := d.do("GET", "/api/parties/"+d.code+"/tributes", nil, &l); c != 200 || !l.Configured || l.Error == "" || len(l.Items) != 0 {
		t.Fatalf("down %d %+v", c, l)
	}
}

func TestTributesInShow(t *testing.T) {
	e := newEnvWith(t, func(c *config.Config) { c.PromoURL = "mock" })
	if !sceneKinds["tributes"] {
		t.Fatal("tributes must be a scene kind")
	}
	show := map[string]any{"name": "T", "entries": []any{
		map[string]any{"id": "a", "kind": "tributes", "seconds": 30, "params": map[string]any{"tributes": []string{"4"}, "layout": "grid"}},
		map[string]any{"id": "b", "kind": "tributes", "seconds": 30, "params": map[string]any{}},
	}}
	if c := e.do("PUT", "/api/parties/"+e.code+"/show", show, nil); c != 200 {
		t.Fatalf("put show %d", c)
	}
	var ready struct {
		Items []readyItem `json:"items"`
	}
	e.do("GET", "/api/parties/"+e.code+"/show/ready", nil, &ready)
	if ready.Items[0].Status != "fail" || ready.Items[1].Status != "fail" {
		t.Fatalf("nothing on disk yet %+v", ready.Items)
	}
	var pf struct{ Done, Failed int }
	if c := e.do("POST", "/api/parties/"+e.code+"/show/prefetch", nil, &pf); c != 200 || pf.Done != 1 || pf.Failed != 0 {
		t.Fatalf("prefetch %d %+v", c, pf)
	}
	e.do("GET", "/api/parties/"+e.code+"/show/ready", nil, &ready)
	if ready.Items[0].Status != "ok" || ready.Items[1].Status != "ok" {
		t.Fatalf("after prefetch %+v", ready.Items)
	}
}
