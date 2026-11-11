package api

// StyleMap packs (D15, D23): Ol1nLLM packs proxied from FineTuneGallery, and
// packs the Studio builds here from one FineTuneGallery session, one pack per
// prompt. Both are served under /stylemaps/; local packs win.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/stylemap"
)

func (s *Server) stylemapDir() string { return filepath.Join(s.Cfg.DataDir, "stylemaps") }

// serveStyleMaps: index.json merges upstream and local packs; files come from
// the local pack folder when it exists, otherwise through the gallery cache.
func (s *Server) serveStyleMaps(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("rel")
	if !gallery.StyleMapPath.MatchString(rel) {
		http.NotFound(w, r)
		return
	}
	if rel == "index.json" {
		writeJSON(w, 200, map[string]any{"packs": s.stylemapIndex(r.Context())})
		return
	}
	if local := filepath.Join(s.stylemapDir(), filepath.FromSlash(rel)); fileExists(local) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		http.ServeFile(w, r, local)
		return
	}
	s.Gallery.ServeStyleMap(w, r, rel)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// stylemapIndex: local packs first, then upstream ones (from the cache when
// the uplink is down; none when it never answered).
func (s *Server) stylemapIndex(ctx context.Context) []stylemap.PackInfo {
	out := stylemap.Index(s.stylemapDir())
	have := map[string]bool{}
	for _, p := range out {
		have[p.ID] = true
	}
	if b, err := s.Gallery.StyleMapIndex(ctx); err == nil {
		var up struct {
			Packs []stylemap.PackInfo `json:"packs"`
		}
		if json.Unmarshal(b, &up) == nil {
			for _, p := range up.Packs {
				if !have[p.ID] && stylemap.ValidID.MatchString(p.ID) {
					p.Source = "finetune"
					out = append(out, p)
				}
			}
		}
	}
	return out
}

// ── building packs from a session ─────────────────────────────────────────

type smJob struct {
	Session string `json:"session"`
	Key     string `json:"key"`
	Pack    string `json:"pack"`
	Status  string `json:"status"` // queued | running | done | failed
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Error   string `json:"error,omitempty"`
}

var sessionIDRE = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,100}$`)

// packID is stable per session and prompt, so a rebuild replaces the pack.
func packID(session, key string) string {
	slug := regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(session, "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return "fs-" + strings.Trim(slug, "-") + "-" + key
}

func (s *Server) finetuneSessions(w http.ResponseWriter, r *http.Request, p store.Party) {
	list, err := s.Gallery.Sessions(r.Context())
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, orEmpty(list))
}

// sessionGroups loads one session and splits it by prompt. Images classified
// as not safe are left out; unclassified ones stay (packs are the operator's
// own sets, D15).
func (s *Server) sessionGroups(ctx context.Context, session string) ([]stylemap.Group, error) {
	imgs, err := s.Gallery.SessionImages(ctx, session)
	if err != nil {
		return nil, err
	}
	var in []stylemap.Input
	for _, im := range imgs {
		if im.Origin == "upload" || (im.Taxo != nil && im.Taxo.Band != "safe") {
			continue
		}
		in = append(in, stylemap.Input{ID: im.ID, SHA: im.SHA, Prompt: im.Prompt, Style: im.StyleID})
	}
	return stylemap.GroupByPrompt(in), nil
}

func (s *Server) finetuneGroups(w http.ResponseWriter, r *http.Request, p store.Party) {
	session := r.PathValue("id")
	if !sessionIDRE.MatchString(session) {
		writeErr(w, 400, "neplatná session")
		return
	}
	groups, err := s.sessionGroups(r.Context(), session)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	type row struct {
		stylemap.Group
		ThumbURL string `json:"thumbUrl"`
		Pack     string `json:"pack"`
		Built    bool   `json:"built"`
		Job      *smJob `json:"job,omitempty"`
	}
	out := []row{}
	s.bmu.Lock()
	for _, g := range groups {
		id := packID(session, g.Key)
		x := row{Group: g, ThumbURL: "/api/gallery/thumb/" + g.Thumb, Pack: id, Built: stylemap.Has(s.stylemapDir(), id)}
		if j, ok := s.smJobs[session+"/"+g.Key]; ok {
			c := *j
			x.Job = &c
		}
		out = append(out, x)
	}
	s.bmu.Unlock()
	writeJSON(w, 200, out)
}

func (s *Server) buildStyleMaps(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		Session string   `json:"session"`
		Groups  []string `json:"groups"`
	}
	if err := readJSON(r, &in); err != nil || !sessionIDRE.MatchString(in.Session) || len(in.Groups) == 0 || len(in.Groups) > 50 {
		writeErr(w, 400, "vyber session a aspoň jednu skupinu")
		return
	}
	var jobs []*smJob
	s.bmu.Lock()
	if s.smBusy {
		s.bmu.Unlock()
		writeErr(w, 409, "už se staví jiné packy, chvilku počkej")
		return
	}
	s.smBusy = true
	for _, k := range in.Groups {
		if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(k) {
			continue
		}
		j := &smJob{Session: in.Session, Key: k, Pack: packID(in.Session, k), Status: "queued"}
		s.smJobs[in.Session+"/"+k] = j
		jobs = append(jobs, j)
	}
	s.bmu.Unlock()
	go s.runStyleMapBuilds(in.Session, jobs)
	w.WriteHeader(202)
}

func (s *Server) setJob(j *smJob, f func(*smJob)) {
	s.bmu.Lock()
	f(j)
	s.bmu.Unlock()
}

func (s *Server) runStyleMapBuilds(session string, jobs []*smJob) {
	defer func() {
		s.bmu.Lock()
		s.smBusy = false
		s.bmu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	fail := func(err error) {
		for _, j := range jobs {
			s.setJob(j, func(j *smJob) {
				if j.Status != "done" {
					j.Status, j.Error = "failed", err.Error()
				}
			})
		}
	}
	groups, err := s.sessionGroups(ctx, session)
	if err != nil {
		fail(err)
		return
	}
	title := session
	if list, err := s.Gallery.Sessions(ctx); err == nil {
		for _, ss := range list {
			if ss.ID == session && ss.Title != "" {
				title = ss.Title
			}
		}
	}
	byKey := map[string]stylemap.Group{}
	for _, g := range groups {
		byKey[g.Key] = g
	}
	for _, j := range jobs {
		g, ok := byKey[j.Key]
		if !ok {
			s.setJob(j, func(j *smJob) { j.Status, j.Error = "failed", "skupina v session už není" })
			continue
		}
		s.setJob(j, func(j *smJob) { j.Status, j.Total = "running", g.N })
		var frames []stylemap.Frame
		for _, it := range g.Items {
			b, err := s.Gallery.File(ctx, "thumb", it.SHA)
			if err == nil {
				var img image.Image
				if img, _, err = image.Decode(bytes.NewReader(b)); err == nil {
					frames = append(frames, stylemap.Frame{Img: img, ID: it.ID, Label: it.Style, Style: it.Style})
				}
			}
			if err != nil {
				slog.Warn("stylemap frame", "sha", it.SHA, "err", err)
			}
			s.setJob(j, func(j *smJob) { j.Done++ })
		}
		prompt := g.Prompt
		if r := []rune(prompt); len(r) > 40 {
			prompt = string(r[:40]) + "…"
		}
		_, err := stylemap.Build(s.stylemapDir(), stylemap.Meta{ID: j.Pack, Title: title + " · " + prompt, Source: "party:" + session}, frames)
		if err != nil {
			s.setJob(j, func(j *smJob) { j.Status, j.Error = "failed", err.Error() })
			continue
		}
		s.setJob(j, func(j *smJob) { j.Status = "done" })
		slog.Info("stylemap built", "pack", j.Pack, "frames", len(frames))
	}
}

func (s *Server) deleteStyleMap(w http.ResponseWriter, r *http.Request, p store.Party) {
	id := r.PathValue("id")
	if !stylemap.Has(s.stylemapDir(), id) {
		writeErr(w, 404, "lokální pack nenalezen")
		return
	}
	if err := os.RemoveAll(filepath.Join(s.stylemapDir(), id)); err != nil {
		serverErr(w, r, fmt.Errorf("remove pack: %w", err))
		return
	}
	w.WriteHeader(204)
}
