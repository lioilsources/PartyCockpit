package gallery

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServeStyleMap(t *testing.T) {
	up := true
	hits := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up {
			http.Error(w, "down", 502)
			return
		}
		hits++
		if r.Header.Get("CF-Access-Client-Id") != "id" {
			http.Error(w, "no token", 403)
			return
		}
		w.Write([]byte("pack:" + r.URL.Path))
	}))
	defer origin.Close()
	dir := t.TempDir()
	c := New(origin.URL, dir, false, map[string]string{"CF-Access-Client-Id": "id"})

	get := func(rel string) (int, string) {
		rec := httptest.NewRecorder()
		c.ServeStyleMap(rec, httptest.NewRequest("GET", "/stylemaps/"+rel, nil), rel)
		b, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, string(b)
	}
	for _, bad := range []string{"../etc/passwd", "demo/../../x", "demo/secret.txt", ".git/config", "demo/t/x.webp"} {
		if code, _ := get(bad); code != 404 {
			t.Fatalf("%s: %d, want 404", bad, code)
		}
	}
	if code, body := get("demo/map.json"); code != 200 || body != "pack:/stylemaps/demo/map.json" {
		t.Fatalf("map: %d %q", code, body)
	}
	get("demo/map.json")
	if hits != 1 {
		t.Fatalf("fresh manifest must come from the cache, origin hit %d×", hits)
	}
	// an old manifest is refreshed, but while the origin is down the stale copy plays on
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "stylemaps", "demo", "map.json"), old, old)
	up = false
	if code, body := get("demo/map.json"); code != 200 || body != "pack:/stylemaps/demo/map.json" {
		t.Fatalf("stale fallback: %d %q", code, body)
	}
	if code, _ := get("demo/t/7.webp"); code != 502 {
		t.Fatalf("nothing cached and origin down: %d, want 502", code)
	}
}
