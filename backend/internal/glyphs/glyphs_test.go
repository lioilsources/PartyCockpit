package glyphs

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndServe(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("sheets.json", `{"default":"caveat","sheets":[
		{"id":"mat_clay","name":"Clay","category":"material"},
		{"id":"secret","name":"Unclear","category":"handwriting","bundle":false},
		{"id":"fancy","name":"Fancy","category":"calligraphy"},
		{"id":"caveat","name":"Caveat","category":"handwriting"},
		{"id":"missing","name":"Missing","category":"handwriting"},
		{"id":"../x","name":"Bad","category":"handwriting"}]}`)
	for _, id := range []string{"mat_clay", "secret", "fancy", "caveat"} {
		write(id+"/glyphs.json", `{"version":2}`)
		write(id+"/a.png", "png")
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sheets) != 2 || s.Sheets[0].ID != "caveat" || !s.Sheets[0].Default || s.Sheets[1].ID != "fancy" {
		t.Fatalf("sheets %+v", s.Sheets)
	}
	if s.Has("secret") || s.Has("mat_clay") || !s.Has("fancy") || (*Set)(nil).Has("caveat") {
		t.Fatal("Has")
	}
	get := func(id, file string) int {
		rec := httptest.NewRecorder()
		s.Serve(rec, httptest.NewRequest("GET", "/glyphs/"+id+"/"+file, nil), id, file)
		return rec.Code
	}
	if get("caveat", "a.png") != 200 || get("caveat", "glyphs.json") != 200 {
		t.Fatal("serve")
	}
	// an unlisted sheet on disk may still be a fallback; traversal never works
	if get("secret", "a.png") != 200 {
		t.Fatal("fallback sheet")
	}
	for _, c := range [][2]string{{"caveat", "../sheets.json"}, {"caveat", ".hidden.png"}, {"caveat", "a.txt"}, {"nope", "a.png"}, {"..", "a.png"}} {
		if get(c[0], c[1]) != 404 {
			t.Fatalf("%v must be 404", c)
		}
	}
}
