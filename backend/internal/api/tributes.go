package api

// PromoClown tributes (D24): imported from promo-api into
// $PC_DATA_DIR/tributes on the operator's request; the stage reads the disk.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/tribute"
)

func (s *Server) initTributes() {
	if s.Tributes == nil {
		s.Tributes = &tribute.Library{Dir: filepath.Join(s.Cfg.DataDir, "tributes")}
	}
	if s.TributeImport == nil {
		im := &tribute.Importer{Lib: s.Tributes, Now: func() time.Time { return s.Store.Now() }}
		switch s.Cfg.PromoURL {
		case "":
		case "mock":
			im.Src = tribute.MockSource{}
		default:
			im.Src = tribute.NewHTTPSource(s.Cfg.PromoURL, s.Cfg.PromoToken, s.styleLabel)
		}
		s.TributeImport = im
	}
}

func (s *Server) styleLabel(id string) (string, bool) {
	if s.Catalog == nil {
		return "", false
	}
	st, ok := s.Catalog.Style(id)
	return st.Label, ok
}

type stageTribute struct {
	ID     string          `json:"id"`
	Credit string          `json:"credit"`
	Labels []string        `json:"labels"`
	Frames []tribute.Frame `json:"frames"`
	V      int64           `json:"v"`
}

// listTributes (public): what the stage can play, from disk only.
func (s *Server) listTributes(w http.ResponseWriter, r *http.Request) {
	out := []stageTribute{}
	for _, m := range s.Tributes.List() {
		out = append(out, stageTribute{ID: m.ID, Credit: m.Credit, Labels: orEmpty(m.Labels), Frames: m.Frames, V: m.ImportedAt})
	}
	writeJSON(w, 200, out)
}

// serveTribute (public): only 0.jpg … 3s.jpg of a tribute on disk.
func (s *Server) serveTribute(w http.ResponseWriter, r *http.Request) {
	p, ok := s.Tributes.File(r.PathValue("id"), r.PathValue("file"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	http.ServeFile(w, r, p)
}

type tributeItem struct {
	ID         string       `json:"id"`
	Credit     string       `json:"credit"`
	Project    string       `json:"project"`
	Labels     []string     `json:"labels"`
	N          int          `json:"n"`
	PostStatus string       `json:"postStatus"`
	CreatedAt  string       `json:"createdAt"`
	Installed  bool         `json:"installed"`
	Remote     bool         `json:"remote"`
	V          int64        `json:"v,omitempty"`
	Job        *tribute.Job `json:"job,omitempty"`
}

// tributeLibrary (cockpit): promo-api's tributes joined with the disk. When
// promo-api is down or not configured, the ones on disk are still listed.
func (s *Server) tributeLibrary(w http.ResponseWriter, r *http.Request, p store.Party) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 60
	}
	out := struct {
		Configured bool          `json:"configured"`
		Error      string        `json:"error,omitempty"`
		Items      []tributeItem `json:"items"`
	}{Configured: s.TributeImport.Configured(), Items: []tributeItem{}}
	jobs := s.TributeImport.Jobs()
	disk := map[string]tribute.Meta{}
	for _, m := range s.Tributes.List() {
		disk[m.ID] = m
	}
	seen := map[string]bool{}
	if out.Configured {
		remote, err := s.TributeImport.Remote(r.Context())
		if err != nil {
			slog.Warn("promo-api list", "err", err)
			out.Error = "promo-api neodpovídá"
			if errors.Is(err, tribute.ErrAuth) {
				out.Error = tribute.ErrAuth.Error()
			}
		}
		for _, t := range remote {
			if len(out.Items) >= limit {
				break
			}
			it := tributeItem{ID: t.ID, Credit: t.Credit, Project: t.Project, Labels: orEmpty(t.Labels), N: t.N, PostStatus: t.PostStatus, CreatedAt: t.CreatedAt, Remote: true}
			if m, ok := disk[t.ID]; ok {
				it.Installed, it.V = true, m.ImportedAt
			}
			if j, ok := jobs[t.ID]; ok {
				it.Job = &j
			}
			seen[t.ID] = true
			out.Items = append(out.Items, it)
		}
	}
	for _, m := range s.Tributes.List() {
		if seen[m.ID] {
			continue
		}
		it := tributeItem{ID: m.ID, Credit: m.Credit, Project: m.Project, Labels: orEmpty(m.Labels), N: len(m.Frames), PostStatus: m.PostStatus, CreatedAt: m.CreatedAt, Installed: true, V: m.ImportedAt}
		out.Items = append(out.Items, it)
	}
	writeJSON(w, 200, out)
}

func (s *Server) tributeImport(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(r, &in); err != nil || len(in.IDs) == 0 {
		writeErr(w, 400, "chybí ids")
		return
	}
	if len(in.IDs) > 40 {
		writeErr(w, 400, "najednou nejvýš 40 tributů")
		return
	}
	switch err := s.TributeImport.Start(r.Context(), in.IDs); {
	case err == nil:
		writeJSON(w, 202, s.TributeImport.Jobs())
	case errors.Is(err, tribute.ErrOff):
		writeErr(w, 409, err.Error())
	case errors.Is(err, tribute.ErrNotFound):
		writeErr(w, 404, err.Error())
	case errors.Is(err, tribute.ErrAuth):
		writeErr(w, 502, err.Error())
	default:
		slog.Warn("tribute import", "err", err)
		writeErr(w, 502, "promo-api neodpovídá")
	}
}

// prefetchTribute imports one tribute now (the Studio's "Stáhnout do cache").
func (s *Server) prefetchTribute(ctx context.Context, id string) error {
	return s.TributeImport.ImportID(ctx, id)
}

func (s *Server) tributeRemove(w http.ResponseWriter, r *http.Request, p store.Party) {
	if err := s.Tributes.Remove(r.PathValue("id")); err != nil {
		writeErr(w, 404, "tribut na disku není")
		return
	}
	w.WriteHeader(204)
}

// tributeThumb: a preview of a tribute not on disk yet. <img> tags can't send
// the bearer, so it takes the Cockpit's file key like /files (D19).
func (s *Server) tributeThumb(w http.ResponseWriter, r *http.Request, p store.Party) {
	if !eq(r.URL.Query().Get("k"), fileKey(p)) {
		http.NotFound(w, r)
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	path, err := s.TributeImport.Thumb(r.Context(), r.PathValue("id"), n)
	if err != nil {
		if !errors.Is(err, tribute.ErrNotFound) && !errors.Is(err, tribute.ErrOff) {
			slog.Warn("tribute thumb", "id", r.PathValue("id"), "err", err)
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}
