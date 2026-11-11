package api

// Studio photos (D23): the operator prepares the show with their own photos.
// They belong to the party's hidden "Studio" guest, are restyled like a
// guest's and approve themselves once the restyle is done: the operator is the
// one looking. A failed restyle still waits (its original never auto-approves),
// and generated clips (breeze, dance) wait for ✓ as everywhere.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

func (s *Server) isStudioAsset(ctx context.Context, id string) bool {
	a, err := s.Store.AssetByID(ctx, id)
	if err != nil {
		return false
	}
	var m struct {
		Studio bool `json:"studio"`
	}
	json.Unmarshal(a.Meta, &m)
	return m.Studio
}

func (s *Server) studioPhoto(w http.ResponseWriter, r *http.Request, p store.Party) {
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeErr(w, 400, "fotka je moc velká nebo chybí")
		return
	}
	f, _, err := r.FormFile("photo")
	if err != nil {
		writeErr(w, 400, "chybí fotka")
		return
	}
	raw, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		writeErr(w, 400, "chyba při čtení fotky")
		return
	}
	style, ok := s.Catalog.Style(r.FormValue("style"))
	if !ok {
		writeErr(w, 400, "neznámý styl")
		return
	}
	g, err := s.Store.StudioGuest(ctx, p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	g.Nickname = clean(r.FormValue("nickname"), 24) // the caption under the photo, may be empty
	a, err := s.submitRestyle(ctx, p, g, raw, style, r.FormValue("medium"), map[string]any{"studio": true})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, a)
}
