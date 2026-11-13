package tribute

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func pngOf(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// fakePromo is promo-api: two drafted tributes with posts in the list, one
// whose post is only reachable by id, one rejected, one without a post.
type fakePromo struct {
	mu       sync.Mutex
	paths    []string
	methods  []string
	tokens   []string
	assets   map[string][]byte
	listSize int
}

func (f *fakePromo) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.RequestURI())
		f.methods = append(f.methods, r.Method)
		f.tokens = append(f.tokens, r.Header.Get(TokenHeader))
		f.mu.Unlock()
		if r.Header.Get(TokenHeader) != "agent-secret" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.URL.Path == "/tributes":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": 9, "project": "tsumiki", "credit": "@ink", "status": "drafted", "post_id": 90, "styles": []string{"anime", "pixel-art"}, "created_at": "2026-11-10T22:00:00Z"},
				{"id": 8, "project": "tsumiki", "credit": "@far", "status": "drafted", "post_id": 80, "styles": []string{"comic", "neon_glow"}},
				{"id": 7, "project": "tsumiki", "credit": "@no", "status": "drafted", "post_id": 70, "styles": []string{"anime"}},
				{"id": 6, "project": "tsumiki", "credit": "@nopost", "status": "drafted"},
			})
		case r.URL.Path == "/posts":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": 90, "status": "draft", "text": "Tribute to @ink 🎨\n\nAnime · Pixel art\n\nby TsumikiBot", "media_paths": []string{"tsumiki/uploads/a 1.png", "tsumiki/uploads/b.png"}},
				{"id": 70, "status": "rejected", "text": "x", "media_paths": []string{"tsumiki/uploads/c.png"}},
			})
		case r.URL.Path == "/posts/80":
			json.NewEncoder(w).Encode(map[string]any{"id": 80, "status": "published", "text": "short", "media_paths": []string{"tsumiki/uploads/d.png", "notes.txt", "tsumiki/uploads/e.png"}})
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			b, ok := f.assets[strings.TrimPrefix(r.URL.Path, "/assets/")]
			if !ok {
				w.WriteHeader(404)
				return
			}
			w.Write(b)
		default:
			w.WriteHeader(404)
		}
	})
}

func newFake(t *testing.T) (*fakePromo, *HTTPSource) {
	f := &fakePromo{assets: map[string][]byte{
		"tsumiki/uploads/a 1.png": pngOf(83, 121),
		"tsumiki/uploads/b.png":   pngOf(83, 121),
		"tsumiki/uploads/d.png":   pngOf(2000, 1400),
		"tsumiki/uploads/e.png":   []byte("<html>not a picture</html>"),
	}}
	ts := httptest.NewServer(f.handler())
	t.Cleanup(ts.Close)
	labels := func(id string) (string, bool) {
		if id == "comic" {
			return "Komiks", true
		}
		return "", false
	}
	return f, NewHTTPSource(ts.URL+"/", "agent-secret", labels)
}

func TestListJoinsPostsAndFilters(t *testing.T) {
	f, src := newFake(t)
	l, err := src.List(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 2 || l[0].ID != "9" || l[1].ID != "8" {
		t.Fatalf("want tributes 9 and 8 (7 rejected, 6 without a post): %+v", l)
	}
	if l[0].N != 2 || strings.Join(l[0].Labels, "|") != "Anime|Pixel art" || l[0].PostStatus != "draft" {
		t.Fatalf("labels from the post text %+v", l[0])
	}
	// 80 came from /posts/80; the text has no style line → catalog, then readable id
	if l[1].N != 2 || strings.Join(l[1].Labels, "|") != "Komiks|Neon glow" || l[1].PostStatus != "published" {
		t.Fatalf("labels from the catalog %+v", l[1])
	}
	for i, m := range f.methods {
		if m != "GET" {
			t.Fatalf("only GET, got %s %s", m, f.paths[i])
		}
		if f.tokens[i] != "agent-secret" || strings.Contains(f.paths[i], "secret") {
			t.Fatalf("token only in the header: %s", f.paths[i])
		}
	}
	if b, _ := json.Marshal(l); strings.Contains(string(b), "uploads") {
		t.Fatalf("asset paths must not leave the backend: %s", b)
	}
}

func TestAuthAndRedirects(t *testing.T) {
	_, src := newFake(t)
	src.Token = "wrong"
	if _, err := src.List(context.Background(), 5); !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	var hops int
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hops++ }))
	defer other.Close()
	redir := httptest.NewServer(http.RedirectHandler(other.URL+"/tributes", http.StatusFound))
	defer redir.Close()
	if _, err := NewHTTPSource(redir.URL, "t", nil).List(context.Background(), 5); err == nil || hops != 0 {
		t.Fatalf("redirect must not be followed: err=%v hops=%d", err, hops)
	}
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[" + strings.Repeat(" ", maxJSON+10) + "]"))
	}))
	defer big.Close()
	if _, err := NewHTTPSource(big.URL, "t", nil).List(context.Background(), 5); err == nil {
		t.Fatal("too big an answer must fail")
	}
}

func TestLabels(t *testing.T) {
	if got := Labels("Tribute to @x\n\nA · B · C\n\nby y", nil, 3, nil); strings.Join(got, "|") != "A|B|C" {
		t.Fatalf("%v", got)
	}
	// the count does not match → ids
	if got := Labels("Tribute to @x\n\nA · B\n\nby y", []string{"oil_paint", "x"}, 3, nil); strings.Join(got, "|") != "Oil paint|X|" {
		t.Fatalf("%v", got)
	}
}

var fixed = time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)

func TestImport(t *testing.T) {
	_, src := newFake(t)
	lib := &Library{Dir: t.TempDir()}
	im := &Importer{Src: src, Lib: lib, Now: func() time.Time { return fixed }}
	ctx := context.Background()
	l, _ := src.List(ctx, 10)
	if err := im.Import(ctx, l[0]); err != nil {
		t.Fatal(err)
	}
	m, ok := lib.Get("9")
	if !ok || len(m.Frames) != 2 || m.Credit != "@ink" || m.ImportedAt != fixed.UnixMilli() || m.Labels[1] != "Pixel art" {
		t.Fatalf("meta %+v", m)
	}
	for _, f := range []string{"0.jpg", "0s.jpg", "1.jpg", "1s.jpg"} {
		if _, ok := lib.File("9", f); !ok {
			t.Fatalf("missing %s", f)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(lib.Dir, "9", "tribute.json"))
	if strings.Contains(string(raw), "uploads") {
		t.Fatalf("tribute.json must not keep asset paths: %s", raw)
	}

	// tribute 8: the first image is fine (scaled down), the second is HTML →
	// the import fails and an older copy stays untouched
	os.MkdirAll(filepath.Join(lib.Dir, "8"), 0o755)
	old := Meta{Schema: 1, ID: "8", Frames: []Frame{{W: 1, H: 1}}}
	b, _ := json.Marshal(old)
	os.WriteFile(filepath.Join(lib.Dir, "8", "tribute.json"), b, 0o644)
	if err := im.Import(ctx, l[1]); !errors.Is(err, errBadImage) {
		t.Fatalf("want errBadImage, got %v", err)
	}
	if m, _ := lib.Get("8"); len(m.Frames) != 1 {
		t.Fatalf("a failed import must keep the old copy: %+v", m)
	}
	ents, _ := os.ReadDir(lib.Dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("leftover %s", e.Name())
		}
	}

	// re-import replaces the whole folder
	os.WriteFile(filepath.Join(lib.Dir, "9", "3.jpg"), []byte("stale"), 0o644)
	if err := im.Import(ctx, l[0]); err != nil {
		t.Fatal(err)
	}
	if _, ok := lib.File("9", "3.jpg"); ok {
		t.Fatal("re-import must replace the folder")
	}
	// a path segment that climbs out is never requested
	bad := l[0]
	bad.media = []string{"../secret.png"}
	if _, err := src.Open(ctx, bad, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for .., got %v", err)
	}
}

func TestImportLimits(t *testing.T) {
	for name, body := range map[string][]byte{
		"oversize": append(pngOf(4, 4), make([]byte, maxImage)...),
		"huge":     pngOf(9000, 2),
	} {
		f, src := newFake(t)
		f.assets["tsumiki/uploads/a 1.png"] = body
		im := &Importer{Src: src, Lib: &Library{Dir: t.TempDir()}, Now: func() time.Time { return fixed }}
		l, _ := src.List(context.Background(), 10)
		if err := im.Import(context.Background(), l[0]); !errors.Is(err, errBadImage) {
			t.Fatalf("%s: want errBadImage, got %v", name, err)
		}
	}
}

func TestImporterJobsAndMock(t *testing.T) {
	lib := &Library{Dir: t.TempDir()}
	im := &Importer{Src: MockSource{}, Lib: lib, Now: func() time.Time { return fixed }}
	ctx := context.Background()
	if err := im.Start(ctx, []string{"6", "1"}); err != nil {
		t.Fatal(err)
	}
	if err := im.Start(ctx, []string{"../x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		j := im.Jobs()
		if j["6"].Status == "done" && j["1"].Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := lib.List(); len(got) != 2 || got[0].ID != "6" || len(got[1].Frames) != 3 {
		t.Fatalf("library %+v / jobs %+v", got, im.Jobs())
	}
	if p, err := im.Thumb(ctx, "5", 0); err != nil || !strings.Contains(p, ".thumbs") {
		t.Fatalf("thumb %q %v", p, err)
	}
	if _, err := (&Importer{Lib: lib}).Remote(ctx); !errors.Is(err, ErrOff) {
		t.Fatalf("no source: %v", err)
	}
}

func TestLibrarySkipsJunk(t *testing.T) {
	lib := &Library{Dir: t.TempDir()}
	for _, d := range []string{".tmp-5-x", ".thumbs", "abc", "12"} {
		os.MkdirAll(filepath.Join(lib.Dir, d), 0o755)
	}
	os.WriteFile(filepath.Join(lib.Dir, "abc", "tribute.json"), []byte(`{"id":"abc","frames":[{}]}`), 0o644)
	if l := lib.List(); len(l) != 0 {
		t.Fatalf("junk listed: %+v", l)
	}
	lib.Clean()
	if _, err := os.Stat(filepath.Join(lib.Dir, ".tmp-5-x")); err == nil {
		t.Fatal("Clean must remove .tmp-*")
	}
	for _, f := range []string{"tribute.json", "4.jpg", "../x", "0.png"} {
		if _, ok := lib.File("12", f); ok {
			t.Fatalf("%s must not be served", f)
		}
	}
}
