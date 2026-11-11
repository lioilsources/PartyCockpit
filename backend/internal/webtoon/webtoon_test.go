package webtoon

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testZip is a pack part like webtoon-content's: one story, one episode with
// two panels (one with an animated loop), a cover and a file that tries to
// escape the folder.
func testZip(t *testing.T) []byte {
	t.Helper()
	pack := map[string]any{
		"schema": 1, "id": "kids-1",
		"worlds": []any{map[string]any{"id": "w1", "audience": "kids"}},
		"stories": []any{map[string]any{"id": "cp_1", "world_id": "w1", "title": "Sára", "title_i18n": map[string]string{"cs": "Sára a koruna"},
			"episodes": []string{"ep_1", "ep_2"}, "cover": "covers/cp_1.webp"}},
		"episodes": []any{
			map[string]any{"id": "ep_1", "index": 1, "title": "Koruna", "status": "ready", "source_language": "cs", "languages": []string{"en"},
				"panels": []any{
					map[string]any{"status": "ready", "image_url": "media/ep_1/p00.webp", "video_url": "media/ep_1/p00_loop.webp", "video_status": "ready",
						"width": 832, "height": 1216, "gap_after": "medium",
						"bubbles": []any{map[string]any{"kind": "narration", "text": "Noc.", "i18n": map[string]string{"en": "Night."}},
							map[string]any{"kind": "speech", "text": "Ahoj", "anchor": map[string]float64{"x": 416, "y": 608}}},
						"sfx": []any{map[string]any{"text": "ŠŠŠ", "style": "bold"}}},
					map[string]any{"status": "ready", "image_url": "media/ep_1/p01.webp", "width": 1216, "height": 832, "gap_after": "small"},
				}},
			// unfinished: the story ends before it
			map[string]any{"id": "ep_2", "index": 2, "title": "Dál", "status": "ready",
				"panels": []any{map[string]any{"status": "rendering"}}},
		},
	}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	add := func(name string, data []byte) {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	pj, _ := json.Marshal(pack)
	add("pack.json", pj)
	add("media/ep_1/p00.webp", []byte("RIFFp00"))
	add("media/ep_1/p00_loop.webp", []byte("RIFFloop"))
	add("media/ep_1/p01.webp", []byte("RIFFp01"))
	add("covers/cp_1.webp", []byte("RIFFcover"))
	add("../evil.webp", []byte("x"))
	add("media/ep_1/run.sh", []byte("x"))
	zw.Close()
	return b.Bytes()
}

type fakeContent struct {
	srv  *httptest.Server
	zip  []byte
	sha  string
	hits int
}

func newFakeContent(t *testing.T, sha string, audience string) *fakeContent {
	f := &fakeContent{zip: testZip(t)}
	sum := sha256.Sum256(f.zip)
	f.sha = hex.EncodeToString(sum[:])
	if sha == "" {
		sha = f.sha
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"schema": 2, "base_urls": map[string]any{"pages": f.srv.URL + "/", "releases": f.srv.URL + "/releases/", "r2": nil},
			"packs": []any{map[string]any{"id": "kids-1", "title": "Děti", "audience": audience, "free": false, "version": 2, "bytes": len(f.zip),
				"parts": []any{map[string]any{"part": "a", "release": "pack-kids-1-v2", "asset": "kids-1-a.zip", "bytes": len(f.zip), "sha256": sha}}}},
			"gamebooks": []any{},
		})
	})
	// GitHub answers a release asset with a redirect to its storage
	mux.HandleFunc("/releases/pack-kids-1-v2/kids-1-a.zip", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/objects/kids-1-a.zip", http.StatusFound)
	})
	mux.HandleFunc("/objects/kids-1-a.zip", func(w http.ResponseWriter, r *http.Request) {
		f.hits++
		w.Write(f.zip)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newInstaller(t *testing.T, f *fakeContent) (*Installer, string) {
	dir := t.TempDir()
	lib := &Library{Dir: dir}
	src := &CatalogSource{URL: f.srv.URL + "/catalog.json", Path: filepath.Join(dir, "catalog.json"), HTTP: http.DefaultClient}
	return &Installer{Lib: lib, Catalog: src, HTTP: http.DefaultClient}, dir
}

func wait(t *testing.T, in *Installer) Job {
	t.Helper()
	for range 200 {
		for _, j := range in.Jobs() {
			if j.Status != "running" {
				return j
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("import never finished")
	return Job{}
}

func TestImportPackPart(t *testing.T) {
	f := newFakeContent(t, "", "kids")
	in, dir := newInstaller(t, f)
	if err := in.Start(context.Background(), "kids-1", 0); err != nil {
		t.Fatal(err)
	}
	if j := wait(t, in); j.Status != "done" {
		t.Fatalf("job %+v", j)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.webp")); err == nil {
		t.Fatal("zip entry escaped the pack folder")
	}
	if _, err := os.Stat(filepath.Join(dir, PacksDir, "kids-1-a", "media", "ep_1", "run.sh")); err == nil {
		t.Fatal("non-media file unpacked")
	}

	eps := in.Lib.List()
	if len(eps) != 1 || eps[0].ID != "ep_1" || eps[0].Story != "Sára a koruna" || eps[0].N != 2 || eps[0].Panels != nil {
		t.Fatalf("list %+v", eps)
	}
	ep, ok := in.Lib.Episode("ep_1")
	if !ok || len(ep.Panels) != 2 {
		t.Fatalf("episode %+v", ep)
	}
	p := ep.Panels[0]
	if p.Image != "/webtoon/ep_1/p00.webp" || p.Loop != "/webtoon/ep_1/p00_loop.webp" || p.Gap != "medium" {
		t.Fatalf("panel %+v", p)
	}
	if len(p.Bubbles) != 2 || p.Bubbles[0].Text["cs"] != "Noc." || p.Bubbles[0].Text["en"] != "Night." || p.Bubbles[1].Kind != "speech" {
		t.Fatalf("bubbles %+v", p.Bubbles)
	}
	if a := p.Bubbles[1].Anchor; a == nil || a.X != 0.5 || a.Y != 0.5 {
		t.Fatalf("anchor in pixels not normalised: %+v", a)
	}
	if len(p.Sfx) != 1 || p.Sfx[0].Text != "ŠŠŠ" {
		t.Fatalf("sfx %+v", p.Sfx)
	}
	if path, ok := in.Lib.File("ep_1", "cover"); !ok || filepath.Base(path) != "cp_1.webp" {
		t.Fatalf("cover %q", path)
	}
	if got := in.Lib.Installs(); len(got) != 1 || got[0].Dir != "kids-1-a" || got[0].SHA256 != f.sha {
		t.Fatalf("installs %+v", got)
	}

	if err := in.Remove("kids-1-a"); err != nil {
		t.Fatal(err)
	}
	if len(in.Lib.List()) != 0 {
		t.Fatal("removed pack still listed")
	}
}

func TestImportRejectsBadChecksum(t *testing.T) {
	f := newFakeContent(t, "00ff", "kids")
	in, dir := newInstaller(t, f)
	if err := in.Start(context.Background(), "kids-1", 0); err != nil {
		t.Fatal(err)
	}
	if j := wait(t, in); j.Status != "failed" {
		t.Fatalf("job %+v", j)
	}
	if _, err := os.Stat(filepath.Join(dir, PacksDir, "kids-1-a")); err == nil {
		t.Fatal("a corrupt zip was installed")
	}
}

func TestAdultPackNeedsOptIn(t *testing.T) {
	f := newFakeContent(t, "", "adult")
	in, _ := newInstaller(t, f)
	if err := in.Start(context.Background(), "kids-1", 0); err != ErrAdult {
		t.Fatalf("adult pack: %v", err)
	}
	in.Lib.AllowAdult = true
	if err := in.Start(context.Background(), "kids-1", 0); err != nil {
		t.Fatal(err)
	}
	wait(t, in)
	if len(in.Lib.List()) != 1 {
		t.Fatal("adult pack not listed with the opt-in")
	}
	in.Lib.AllowAdult = false
	in.Lib.Invalidate()
	if len(in.Lib.List()) != 0 {
		t.Fatal("adult stories listed without the opt-in")
	}
}

func TestCatalogFallsBackToDisk(t *testing.T) {
	f := newFakeContent(t, "", "kids")
	in, _ := newInstaller(t, f)
	if _, err := in.Catalog.Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.srv.Close()
	offline := &CatalogSource{URL: in.Catalog.URL, Path: in.Catalog.Path, HTTP: http.DefaultClient}
	c, err := offline.Get(context.Background())
	if err != nil || len(c.Packs) != 1 {
		t.Fatalf("offline catalog: %v %+v", err, c)
	}
}

func TestFolderEpisodes(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "pilot"), 0o755)
	os.WriteFile(filepath.Join(dir, "pilot", "title.txt"), []byte("Pilot\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "pilot", "02.png"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "pilot", "01.png"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(dir, PacksDir), 0o755)
	lib := &Library{Dir: dir}
	eps := lib.List()
	if len(eps) != 1 || eps[0].Title != "Pilot" || eps[0].Source != "folder" || eps[0].N != 2 {
		t.Fatalf("%+v", eps)
	}
	ep, _ := lib.Episode("pilot")
	if ep.Panels[0].Image != "/webtoon/pilot/01.png" {
		t.Fatalf("order %+v", ep.Panels)
	}
}
