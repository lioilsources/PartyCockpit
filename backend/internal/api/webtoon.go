package api

// Webtoon reader (D7, D18, D23): episodes from hand-made folders and from
// Toonfork content packs imported into $PC_DATA_DIR/webtoon/packs.

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/webtoon"
)

func (s *Server) initWebtoon() {
	if s.Webtoon == nil {
		s.Webtoon = &webtoon.Library{Dir: filepath.Join(s.Cfg.DataDir, "webtoon"), AllowAdult: s.Cfg.WebtoonAllowAdult}
	}
	if s.WebtoonImport == nil {
		// GitHub Releases answer with a redirect to their storage: follow it
		hc := &http.Client{}
		s.WebtoonImport = &webtoon.Installer{
			Lib:     s.Webtoon,
			Catalog: &webtoon.CatalogSource{URL: s.Cfg.WebtoonCatalogURL, Path: filepath.Join(s.Webtoon.Dir, "catalog.json"), HTTP: hc},
			HTTP:    hc,
		}
	}
}

func (s *Server) listWebtoon(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Webtoon.List())
}

func (s *Server) getWebtoon(w http.ResponseWriter, r *http.Request) {
	ep, ok := s.Webtoon.Episode(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "epizoda nenalezena")
		return
	}
	writeJSON(w, 200, ep)
}

func (s *Server) serveWebtoon(w http.ResponseWriter, r *http.Request) {
	path, ok := s.Webtoon.File(r.PathValue("ep"), r.PathValue("file"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}

// webtoonCatalog (cockpit): what can be downloaded, what is on disk and what
// is downloading. Adult packs only with WEBTOON_ALLOW_ADULT.
func (s *Server) webtoonCatalog(w http.ResponseWriter, r *http.Request, p store.Party) {
	type part struct {
		Index     int          `json:"index"`
		Part      string       `json:"part"`
		Bytes     int64        `json:"bytes"`
		Stories   int          `json:"stories"`
		Dir       string       `json:"dir"`
		Installed bool         `json:"installed"`
		Job       *webtoon.Job `json:"job,omitempty"`
	}
	type item struct {
		ID       string   `json:"id"`
		Kind     string   `json:"kind"` // pack | gamebook
		Title    string   `json:"title"`
		Type     string   `json:"type"`
		Audience string   `json:"audience"`
		Free     bool     `json:"free"`
		Bytes    int64    `json:"bytes"`
		Langs    []string `json:"langs"`
		Parts    []part   `json:"parts"`
	}
	installed := map[string]bool{}
	for _, in := range s.Webtoon.Installs() {
		installed[in.Dir] = true
	}
	jobs := s.WebtoonImport.Jobs()
	out := struct {
		Items     []item                `json:"items"`
		Installed []webtoon.InstallInfo `json:"installed"`
		Error     string                `json:"error,omitempty"`
	}{Items: []item{}, Installed: orEmpty(s.Webtoon.Installs())}
	cat, err := s.WebtoonImport.Catalog.Get(r.Context())
	if err != nil {
		out.Error = "katalog Webtoon není dostupný: " + err.Error()
	}
	add := func(kind string, packs []webtoon.Pack) {
		for _, pk := range packs {
			if pk.Audience == "adult" && !s.Webtoon.AllowAdult {
				continue
			}
			it := item{ID: pk.ID, Kind: kind, Title: pk.Label("cs"), Type: pk.Type, Audience: pk.Audience, Free: pk.Free, Bytes: pk.Bytes, Langs: orEmpty(pk.Languages), Parts: []part{}}
			for i, pt := range pk.Parts {
				dir := webtoon.DirName(pk, i)
				x := part{Index: i, Part: pt.Part, Bytes: pt.Bytes, Stories: pt.Stories, Dir: dir, Installed: installed[dir]}
				if j, ok := jobs[pk.ID+"/"+strconv.Itoa(i)]; ok {
					x.Job = &j
				}
				it.Parts = append(it.Parts, x)
			}
			out.Items = append(out.Items, it)
		}
	}
	add("pack", cat.Packs)
	add("gamebook", cat.Gamebooks)
	writeJSON(w, 200, out)
}

func (s *Server) webtoonInstall(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		Pack string `json:"pack"`
		Part int    `json:"part"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	switch err := s.WebtoonImport.Start(r.Context(), in.Pack, in.Part); {
	case err == nil:
		w.WriteHeader(202)
	case errors.Is(err, webtoon.ErrBusy):
		writeErr(w, 409, err.Error())
	case errors.Is(err, webtoon.ErrNotFound):
		writeErr(w, 404, err.Error())
	case errors.Is(err, webtoon.ErrAdult):
		writeErr(w, 403, err.Error())
	default:
		writeErr(w, 502, err.Error())
	}
}

func (s *Server) webtoonRemove(w http.ResponseWriter, r *http.Request, p store.Party) {
	if err := s.WebtoonImport.Remove(r.PathValue("dir")); err != nil {
		writeErr(w, 404, "balík není nainstalovaný")
		return
	}
	w.WriteHeader(204)
}

// czPlural: 1 epizoda, 2–4 epizody, 5+ epizod (frontend/src/lib/types.ts plural).
func czPlural(n int, one, few, many string) string {
	switch {
	case n == 1:
		return one
	case n >= 2 && n <= 4:
		return few
	}
	return many
}
