package api

// Show library (D23): a prepared show outlives its party. Saving copies the
// show and every photo and clip it needs to $PC_DATA_DIR/shows/<slug>/ (the
// janitor never touches it); loading copies them into another party as
// approved Studio assets. Webtoon episodes and StyleMap packs are shared by
// all parties already and are not copied.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

type libClip struct {
	Kind string          `json:"kind"` // live | breeze
	File string          `json:"file"`
	Meta json.RawMessage `json:"meta"`
}

type libAsset struct {
	ID    string          `json:"id"`
	Kind  string          `json:"kind"`
	File  string          `json:"file"`
	Meta  json.RawMessage `json:"meta"`
	Clips []libClip       `json:"clips,omitempty"`
}

type libShow struct {
	Name    string            `json:"name"`
	SavedAt int64             `json:"savedAt"`
	Loop    bool              `json:"loop"`
	Entries []store.ShowEntry `json:"entries"`
	Assets  []libAsset        `json:"assets"`
}

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

func (s *Server) showsDir() string { return filepath.Join(s.Cfg.DataDir, "shows") }

// plainLetters: Czech and Slovak letters without the accent, for slugs.
var plainLetters = strings.NewReplacer(
	"á", "a", "ä", "a", "č", "c", "ď", "d", "é", "e", "ě", "e", "í", "i", "ĺ", "l", "ľ", "l", "ň", "n",
	"ó", "o", "ô", "o", "ö", "o", "ŕ", "r", "ř", "r", "š", "s", "ť", "t", "ú", "u", "ů", "u", "ü", "u", "ý", "y", "ž", "z")

// slugify: "Ema 40 – narozky" → "ema-40-narozky".
func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range plainLetters.Replace(strings.ToLower(name)) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	return out
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// withoutOriginal drops the name of the guest's private upload from meta.
func withoutOriginal(raw json.RawMessage, studio bool) json.RawMessage {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		m = map[string]any{}
	}
	delete(m, "original")
	if studio {
		m["studio"] = true
	}
	b, _ := json.Marshal(m)
	return b
}

func (s *Server) listShows(w http.ResponseWriter, r *http.Request, p store.Party) {
	type row struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		SavedAt int64  `json:"savedAt"`
		Entries int    `json:"entries"`
		Assets  int    `json:"assets"`
	}
	out := []row{}
	dirs, _ := os.ReadDir(s.showsDir())
	for _, d := range dirs {
		if !d.IsDir() || !slugRE.MatchString(d.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.showsDir(), d.Name(), "show.json"))
		if err != nil {
			continue
		}
		var ls libShow
		if json.Unmarshal(b, &ls) == nil {
			out = append(out, row{d.Name(), ls.Name, ls.SavedAt, len(ls.Entries), len(ls.Assets)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt > out[j].SavedAt })
	writeJSON(w, 200, out)
}

// saveShow copies the party's show into the library: every photo the show
// names and every Studio photo, with the clips that play in their place.
func (s *Server) saveShow(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		Name string `json:"name"`
	}
	readJSON(r, &in)
	ctx := r.Context()
	sh, err := s.Store.Show(ctx, p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	name := clean(in.Name, 60)
	if name == "" {
		name = clean(sh.Name, 60)
	}
	slug := slugify(name)
	if !slugRE.MatchString(slug) {
		writeErr(w, 400, "pojmenuj show (písmena nebo číslice)")
		return
	}
	wanted := map[string]bool{}
	for _, e := range sh.Entries {
		for _, id := range paramStrings(e.Params, "assets") {
			wanted[id] = true
		}
	}
	queue, err := s.Store.StageQueue(ctx, p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	all, err := s.Store.Assets(ctx, p.Code, "approved")
	if err != nil {
		serverErr(w, r, err)
		return
	}
	tmp := filepath.Join(s.showsDir(), "."+slug+".tmp")
	os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "media"), 0o755); err != nil {
		serverErr(w, r, err)
		return
	}
	put := func(file string) error {
		if !validFileName(p.Code, file) {
			return fmt.Errorf("bad file %q", file)
		}
		return copyFile(filepath.Join(s.partyDir(p.Code), file), filepath.Join(tmp, "media", file))
	}
	ls := libShow{Name: name, SavedAt: s.Store.Now().UnixMilli(), Loop: sh.Loop, Entries: sh.Entries, Assets: []libAsset{}}
	for _, a := range queue {
		if !wanted[a.ID] && !s.isStudioAsset(ctx, a.ID) {
			continue
		}
		if a.File == "" || (a.Kind != "restyle" && a.Kind != "upload") {
			continue
		}
		if err := put(a.File); err != nil {
			serverErr(w, r, err)
			return
		}
		la := libAsset{ID: a.ID, Kind: a.Kind, File: a.File, Meta: withoutOriginal(a.Meta, false)}
		for _, c := range all {
			if c.SrcID == a.ID && (c.Kind == "live" || c.Kind == "breeze") && c.File != "" {
				if err := put(c.File); err != nil {
					serverErr(w, r, err)
					return
				}
				la.Clips = append(la.Clips, libClip{Kind: c.Kind, File: c.File, Meta: c.Meta})
			}
		}
		ls.Assets = append(ls.Assets, la)
	}
	b, _ := json.MarshalIndent(ls, "", " ")
	if err := os.WriteFile(filepath.Join(tmp, "show.json"), b, 0o644); err != nil {
		serverErr(w, r, err)
		return
	}
	final := filepath.Join(s.showsDir(), slug)
	os.RemoveAll(final)
	if err := os.Rename(tmp, final); err != nil {
		serverErr(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"slug": slug, "name": name, "assets": len(ls.Assets), "entries": len(ls.Entries)})
}

// loadShow brings a saved show into this party: photos and clips become
// approved Studio assets with new ids, and the show's entries point at them.
func (s *Server) loadShow(w http.ResponseWriter, r *http.Request, p store.Party) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		writeErr(w, 404, "show nenalezena")
		return
	}
	root := filepath.Join(s.showsDir(), slug)
	b, err := os.ReadFile(filepath.Join(root, "show.json"))
	if err != nil {
		writeErr(w, 404, "show nenalezena")
		return
	}
	var ls libShow
	if err := json.Unmarshal(b, &ls); err != nil {
		serverErr(w, r, err)
		return
	}
	ctx := r.Context()
	g, err := s.Store.StudioGuest(ctx, p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	dir := s.partyDir(p.Code)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		serverErr(w, r, err)
		return
	}
	bring := func(file string) (string, error) {
		if !validFileName(p.Code, file) {
			return "", fmt.Errorf("bad file %q", file)
		}
		name := store.NewID(10) + filepath.Ext(file)
		return name, copyFile(filepath.Join(root, "media", file), filepath.Join(dir, name))
	}
	ids := map[string]string{}
	for _, la := range ls.Assets {
		file, err := bring(la.File)
		if err != nil {
			serverErr(w, r, err)
			return
		}
		a, err := s.Store.CreateAsset(ctx, store.Asset{PartyCode: p.Code, GuestID: g.ID, Kind: la.Kind, File: file, Meta: withoutOriginal(la.Meta, true), Status: "pending"})
		if err == nil {
			err = s.Store.SetAssetStatus(ctx, p.Code, a.ID, "approved")
		}
		if err != nil {
			serverErr(w, r, err)
			return
		}
		ids[la.ID] = a.ID
		for _, c := range la.Clips {
			cf, err := bring(c.File)
			if err != nil {
				serverErr(w, r, err)
				return
			}
			ca, err := s.Store.CreateAsset(ctx, store.Asset{PartyCode: p.Code, GuestID: g.ID, Kind: c.Kind, SrcID: a.ID, File: cf, Meta: c.Meta, Status: "pending"})
			if err == nil {
				err = s.Store.SetAssetStatus(ctx, p.Code, ca.ID, "approved")
			}
			if err != nil {
				serverErr(w, r, err)
				return
			}
		}
	}
	for i, e := range ls.Entries {
		var m map[string]any
		if json.Unmarshal(e.Params, &m) != nil || m == nil {
			continue
		}
		if list, ok := m["assets"].([]any); ok {
			out := []any{}
			for _, x := range list {
				if id, ok := x.(string); ok && ids[id] != "" {
					out = append(out, ids[id])
				}
			}
			m["assets"] = out
			ls.Entries[i].Params, _ = json.Marshal(m)
		}
	}
	entries, err := cleanEntries(ls.Entries)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	s.showPause(p.Code)
	if _, err := s.Store.SaveShow(ctx, p.Code, store.Show{Name: ls.Name, Entries: entries, Loop: ls.Loop}); err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushQueue(ctx, p.Code)
	s.pushCockpit(ctx, p)
	s.pushShow(ctx, p.Code)
	s.Hub.Broadcast(p.Code, hub.Msg{Type: "asset.updated", Data: nil}, hub.Guest)
	writeJSON(w, 200, map[string]int{"assets": len(ids), "entries": len(entries)})
}

func (s *Server) deleteShow(w http.ResponseWriter, r *http.Request, p store.Party) {
	slug := r.PathValue("slug")
	path := filepath.Join(s.showsDir(), slug)
	if !slugRE.MatchString(slug) || !fileExists(filepath.Join(path, "show.json")) {
		writeErr(w, 404, "show nenalezena")
		return
	}
	if err := os.RemoveAll(path); err != nil {
		serverErr(w, r, err)
		return
	}
	w.WriteHeader(204)
}
