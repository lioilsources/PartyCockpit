// Package glyphs serves HandWrittenStickers glyph sheets (../HandWritenStickers,
// handwritten_stickers/assets/glyphs): sheets.json plus one directory per sheet
// with glyphs.json and a PNG per character. The stage lays the sticker text out
// from them (frontend/src/lib/handwriting.ts), so guests' notes look written by
// hand instead of set in a font (DECISIONS D16).
package glyphs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type Sheet struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Default  bool   `json:"default,omitempty"`
}

type Set struct {
	Dir    string
	Sheets []Sheet
	byID   map[string]bool
	all    map[string]bool // every sheet on disk, fallbacks included
}

// categories a guest may pick; material sheets (AI letters) stay out
var allowed = map[string]int{"handwriting": 0, "kids": 1, "calligraphy": 2}

var (
	idRE   = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	fileRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,80}\.(png|json)$`)
)

// Load reads dir/sheets.json. Sheets marked "bundle": false (unclear licence)
// are skipped, like in the app's release builds.
func Load(dir string) (*Set, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "sheets.json"))
	if err != nil {
		return nil, fmt.Errorf("glyphs: %w", err)
	}
	var idx struct {
		Default string `json:"default"`
		Sheets  []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Bundle   *bool  `json:"bundle"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("glyphs sheets.json: %w", err)
	}
	s := &Set{Dir: dir, byID: map[string]bool{}, all: map[string]bool{}}
	for _, sh := range idx.Sheets {
		if !idRE.MatchString(sh.ID) {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, sh.ID, "glyphs.json")); err != nil {
			continue
		}
		s.all[sh.ID] = true
		if sh.Bundle != nil && !*sh.Bundle {
			continue
		}
		if _, ok := allowed[sh.Category]; !ok {
			continue
		}
		s.Sheets = append(s.Sheets, Sheet{ID: sh.ID, Name: sh.Name, Category: sh.Category, Default: sh.ID == idx.Default})
		s.byID[sh.ID] = true
	}
	sort.SliceStable(s.Sheets, func(i, j int) bool { return allowed[s.Sheets[i].Category] < allowed[s.Sheets[j].Category] })
	if len(s.Sheets) == 0 {
		return nil, fmt.Errorf("glyphs: no usable sheet in %s", dir)
	}
	return s, nil
}

// Has reports whether a guest may pick the sheet.
func (s *Set) Has(id string) bool { return s != nil && s.byID[id] }

// Serve answers /glyphs/{id}/{file}. Fallback sheets a pickable sheet names
// are on disk too, so any sheet in sheets.json is served.
func (s *Set) Serve(w http.ResponseWriter, r *http.Request, id, file string) {
	if s == nil || !idRE.MatchString(id) || !fileRE.MatchString(file) || !s.all[id] {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(s.Dir, id, file))
}
