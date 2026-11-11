package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
	"github.com/lioilsources/PartyCockpit/backend/internal/webtoon"
)

func TestWebtoonRoutes(t *testing.T) {
	cat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"schema": 2, "base_urls": map[string]any{"releases": "http://127.0.0.1:1/"},
			"packs": []any{
				map[string]any{"id": "kids-1", "title": "Děti", "audience": "kids", "parts": []any{map[string]any{"part": "a", "release": "r", "asset": "a.zip", "bytes": 10}}},
				map[string]any{"id": "harlequin-1", "title": "Romantika", "audience": "adult", "parts": []any{map[string]any{"part": "a"}}},
			},
			"gamebooks": []any{map[string]any{"id": "sara", "title": "Sára", "audience": "kids", "free": true, "parts": []any{map[string]any{"release": "g", "asset": "s.zip"}}}},
		})
	}))
	defer cat.Close()
	e := newEnvWith(t, func(c *config.Config) { c.WebtoonCatalogURL = cat.URL })
	dir := filepath.Join(e.srv.Cfg.DataDir, "webtoon", "pilot")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "01.png"), testPNG(), 0o644)
	e.srv.Webtoon.Invalidate()

	var list []webtoon.Episode
	e.do("GET", "/api/webtoon", nil, &list)
	if len(list) != 1 || list[0].ID != "pilot" || list[0].N != 1 {
		t.Fatalf("list %+v", list)
	}
	var ep webtoon.Episode
	if c := e.do("GET", "/api/webtoon/pilot", nil, &ep); c != 200 || len(ep.Panels) != 1 {
		t.Fatalf("detail %d %+v", c, ep)
	}
	if res, err := http.Get(e.ts.URL + ep.Panels[0].Image); err != nil || res.StatusCode != 200 {
		t.Fatalf("panel file: %v %v", err, res.StatusCode)
	}
	if res, _ := http.Get(e.ts.URL + "/webtoon/pilot/..%2Fx"); res.StatusCode != 404 {
		t.Fatalf("path escape: %d", res.StatusCode)
	}

	var c struct {
		Items []struct {
			ID    string `json:"id"`
			Kind  string `json:"kind"`
			Parts []struct {
				Dir string `json:"dir"`
			} `json:"parts"`
		} `json:"items"`
	}
	if code := e.do("GET", "/api/parties/"+e.code+"/webtoon/catalog", nil, &c); code != 200 {
		t.Fatalf("catalog %d", code)
	}
	if len(c.Items) != 2 || c.Items[0].ID != "kids-1" || c.Items[1].Kind != "gamebook" || c.Items[1].Parts[0].Dir != "sara-1" {
		t.Fatalf("catalog items (adult must be hidden) %+v", c.Items)
	}
	if code := e.do("POST", "/api/parties/"+e.code+"/webtoon/install", map[string]any{"pack": "harlequin-1", "part": 0}, nil); code != 403 {
		t.Fatalf("adult install: %d", code)
	}
	e.token = "wrong"
	if code := e.do("GET", "/api/parties/"+e.code+"/webtoon/catalog", nil, nil); code != 401 {
		t.Fatalf("catalog without the cockpit token: %d", code)
	}
}
