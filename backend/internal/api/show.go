package api

// Scene Studio (D23): the party's show is a list of scenes, each with its
// items (in its params) and a length. The runner here plays it: it sets the
// scene like the Cockpit does, waits the entry's seconds and moves on. The
// stage only draws; with params.seed every scene plays the same way each time.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/stylemap"
)

const (
	maxShowEntries = 200
	maxEntryParams = 16 << 10
)

// showRun is the player state of one party's show (memory only: after a
// restart the show is paused where it was).
type showRun struct {
	playing   bool
	index     int
	entryID   string
	startedAt int64
	endsAt    int64
	gen       int // bumps on every start/stop; a stale timer sees it and quits
	timer     *time.Timer
	req       *http.Request // the operator's request, for the join URL in party broadcasts
}

// ShowState is what the Studio, the Cockpit and the stage get as "show".
type ShowState struct {
	store.Show
	Playing   bool   `json:"playing"`
	Index     int    `json:"index"`
	EntryID   string `json:"entryId"`
	StartedAt int64  `json:"startedAt"`
	EndsAt    int64  `json:"endsAt"`
}

func (s *Server) showSecond() time.Duration {
	if s.showUnit > 0 {
		return s.showUnit
	}
	return time.Second
}

func (s *Server) run(code string) *showRun {
	r, ok := s.showRuns[code]
	if !ok {
		r = &showRun{}
		s.showRuns[code] = r
	}
	return r
}

func (s *Server) showState(ctx context.Context, code string) (ShowState, error) {
	sh, err := s.Store.Show(ctx, code)
	if err != nil {
		return ShowState{}, err
	}
	s.shmu.Lock()
	defer s.shmu.Unlock()
	r := s.run(code)
	return ShowState{Show: sh, Playing: r.playing, Index: r.index, EntryID: r.entryID, StartedAt: r.startedAt, EndsAt: r.endsAt}, nil
}

func (s *Server) pushShow(ctx context.Context, code string) {
	if st, err := s.showState(ctx, code); err == nil {
		s.Hub.Broadcast(code, hub.Msg{Type: "show", Data: st}, hub.Stage, hub.Cockpit)
	}
}

// startEntry sets entry idx on the stage and arms the timer for the next one.
func (s *Server) startEntry(code string, idx int) error {
	ctx := context.Background()
	sh, err := s.Store.Show(ctx, code)
	if err != nil {
		return err
	}
	if len(sh.Entries) == 0 {
		return fmt.Errorf("show je prázdná")
	}
	idx = ((idx % len(sh.Entries)) + len(sh.Entries)) % len(sh.Entries)
	e := sh.Entries[idx]
	if _, err := s.Store.SetScene(ctx, code, e.Kind, e.Params); err != nil {
		return err
	}
	s.shmu.Lock()
	r := s.run(code)
	if r.timer != nil {
		r.timer.Stop()
	}
	now := s.Store.Now()
	r.playing, r.index, r.entryID = true, idx, e.ID
	r.startedAt, r.endsAt = now.UnixMilli(), now.Add(time.Duration(e.Seconds)*time.Second).UnixMilli()
	r.gen++
	gen := r.gen
	r.timer = time.AfterFunc(time.Duration(e.Seconds)*s.showSecond(), func() { s.showTick(code, gen) })
	req := r.req
	s.shmu.Unlock()
	if req != nil {
		s.broadcastParty(req, code)
	}
	s.pushShow(ctx, code)
	return nil
}

func (s *Server) showTick(code string, gen int) {
	ctx := context.Background()
	sh, err := s.Store.Show(ctx, code)
	if err != nil {
		return
	}
	s.shmu.Lock()
	r := s.run(code)
	if r.gen != gen || !r.playing {
		s.shmu.Unlock()
		return
	}
	next := r.index + 1
	// the entries may have changed while this one played: continue after it
	for i, e := range sh.Entries {
		if e.ID == r.entryID {
			next = i + 1
		}
	}
	if next >= len(sh.Entries) && !sh.Loop {
		r.playing = false
		r.gen++
		s.shmu.Unlock()
		s.pushShow(ctx, code)
		return
	}
	s.shmu.Unlock()
	s.startEntry(code, next)
}

func (s *Server) showPause(code string) {
	s.shmu.Lock()
	r := s.run(code)
	if r.timer != nil {
		r.timer.Stop()
	}
	was := r.playing
	r.playing = false
	r.gen++
	s.shmu.Unlock()
	if was {
		s.pushShow(context.Background(), code)
	}
}

// ── HTTP ───────────────────────────────────────────────────────────────────

var entryIDRE = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// cleanEntries checks what the Studio saves: known scenes, sane lengths and
// parameters that are a JSON object.
func cleanEntries(in []store.ShowEntry) ([]store.ShowEntry, error) {
	if len(in) > maxShowEntries {
		return nil, fmt.Errorf("show má nejvýš %d scén", maxShowEntries)
	}
	out := make([]store.ShowEntry, 0, len(in))
	seen := map[string]bool{}
	for _, e := range in {
		if !sceneKinds[e.Kind] {
			return nil, fmt.Errorf("neznámá scéna %q", e.Kind)
		}
		if !entryIDRE.MatchString(e.ID) || seen[e.ID] {
			e.ID = randHex(4)
		}
		seen[e.ID] = true
		e.Title = clean(e.Title, 60)
		if e.Seconds <= 0 {
			e.Seconds = 30
		}
		e.Seconds = min(max(e.Seconds, 5), 3600)
		if len(e.Params) == 0 || string(e.Params) == "null" {
			e.Params = json.RawMessage("{}")
		}
		var obj map[string]any
		if len(e.Params) > maxEntryParams || json.Unmarshal(e.Params, &obj) != nil {
			return nil, fmt.Errorf("scéna %q: parametry nejsou objekt", e.Title)
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Server) getShow(w http.ResponseWriter, r *http.Request, p store.Party) {
	st, err := s.showState(r.Context(), p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	writeJSON(w, 200, st)
}

func (s *Server) putShow(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in store.Show
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&in); err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	entries, err := cleanEntries(in.Entries)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	in.Entries, in.Name = entries, clean(in.Name, 60)
	if _, err := s.Store.SaveShow(r.Context(), p.Code, in); err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushShow(r.Context(), p.Code)
	s.getShow(w, r, p)
}

func (s *Server) showControl(action string) partyHandler {
	return func(w http.ResponseWriter, r *http.Request, p store.Party) {
		s.shmu.Lock()
		run := s.run(p.Code)
		run.req = r.Clone(context.Background())
		idx := run.index
		s.shmu.Unlock()
		var err error
		switch action {
		case "play":
			var in struct {
				Index *int `json:"index"`
			}
			_ = readJSON(r, &in)
			if in.Index != nil {
				idx = *in.Index
			}
			err = s.startEntry(p.Code, idx)
		case "next":
			err = s.startEntry(p.Code, idx+1)
		case "prev":
			err = s.startEntry(p.Code, idx-1)
		case "pause":
			s.showPause(p.Code)
		}
		if err != nil {
			writeErr(w, 409, err.Error())
			return
		}
		s.getShow(w, r, p)
	}
}

// ── offline readiness ─────────────────────────────────────────────────────

type readyItem struct {
	EntryID string `json:"entryId"`
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Status  string `json:"status"` // ok | warn | fail
	Detail  string `json:"detail"`
}

func paramStrings(raw json.RawMessage, keys ...string) []string {
	var m map[string]any
	json.Unmarshal(raw, &m)
	var out []string
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if v != "" {
				out = append(out, v)
			}
		case []any:
			for _, x := range v {
				if s, ok := x.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return out
}

// showReady says, entry by entry, whether everything it plays is on this
// machine: the party can then run without the uplink and without the GPU.
func (s *Server) showReady(w http.ResponseWriter, r *http.Request, p store.Party) {
	ctx := r.Context()
	sh, err := s.Store.Show(ctx, p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	queue, _ := s.Store.StageQueue(ctx, p.Code)
	inQueue := map[string]store.Asset{}
	for _, a := range queue {
		inQueue[a.ID] = a
	}
	out := []readyItem{}
	for _, e := range sh.Entries {
		it := readyItem{EntryID: e.ID, Title: e.Title, Kind: e.Kind, Status: "ok"}
		var missing []string
		switch e.Kind {
		case "webtoon":
			eps := paramStrings(e.Params, "episodes", "episode")
			for _, id := range eps {
				if id != "party" && !s.Webtoon.Has(id) {
					missing = append(missing, id)
				}
			}
			it.Detail = fmt.Sprintf("%d epizod", len(eps))
			if len(eps) == 0 {
				it.Status, it.Detail = "warn", "bez vybraných epizod hraje první na disku"
			}
		case "stylesweep":
			packs := paramStrings(e.Params, "packs", "pack")
			for _, id := range packs {
				if !stylemap.Has(s.stylemapDir(), id) && !s.packCached(id) {
					missing = append(missing, id)
				}
			}
			it.Detail = fmt.Sprintf("%d packů", len(packs))
			if len(packs) == 0 {
				it.Status, it.Detail = "warn", "bez vybraného packu hraje první z indexu"
			}
		case "strips":
			ids := paramStrings(e.Params, "assets")
			for _, id := range ids {
				a, ok := inQueue[id]
				if !ok || !fileExists(s.partyFile(p.Code, a.File)) || (a.Live != "" && !fileExists(s.partyFile(p.Code, a.Live))) {
					missing = append(missing, id)
				}
			}
			it.Detail = fmt.Sprintf("%d fotek", len(ids))
			if len(ids) == 0 {
				it.Detail = "celá show fronta (" + fmt.Sprint(len(queue)) + ")"
			}
		case "slideshow":
			imgs := paramStrings(e.Params, "images")
			for _, sha := range imgs {
				if !s.Gallery.Cached("img", sha) {
					missing = append(missing, sha[:min(8, len(sha))])
				}
			}
			it.Detail = fmt.Sprintf("%d obrázků", len(imgs))
			if len(imgs) == 0 {
				it.Status, it.Detail = "warn", "bez vybraných obrázků potřebuje FineTuneGallery online"
			}
		case "gallery", "stickers", "dance", "recap":
			it.Detail = "show fronta (" + fmt.Sprint(len(queue)) + ")"
		case "nowplaying":
			it.Status, it.Detail = "warn", "potřebuje DJfy (síť)"
		}
		if t := paramStrings(e.Params, "transition"); len(t) > 0 && !stylemap.Has(s.stylemapDir(), t[0]) && !s.packCached(t[0]) {
			missing = append(missing, "přechod "+t[0])
		}
		if len(missing) > 0 {
			it.Status = "fail"
			it.Detail = fmt.Sprintf("chybí na disku: %v", missing)
		}
		out = append(out, it)
	}
	busy := 0
	if as, err := s.Store.Assets(ctx, p.Code, "processing"); err == nil {
		busy = len(as)
	}
	ok := 0
	for _, it := range out {
		if it.Status != "fail" {
			ok++
		}
	}
	writeJSON(w, 200, map[string]any{"items": out, "ok": ok, "total": len(out), "processing": busy})
}

func (s *Server) partyFile(code, name string) string {
	if name == "" || !validFileName(code, name) {
		return ""
	}
	return s.partyDir(code) + "/" + name
}

// packCached: an upstream pack whose map, atlas and thumbs are all in the cache.
func (s *Server) packCached(id string) bool {
	if !stylemap.ValidID.MatchString(id) {
		return false
	}
	missing, err := s.Gallery.PackMissing(id)
	return err == nil && missing == 0
}

// showPrefetch downloads into the caches what the show needs from the
// uplink: gallery images of slideshows and upstream StyleMap packs.
func (s *Server) showPrefetch(w http.ResponseWriter, r *http.Request, p store.Party) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	sh, err := s.Store.Show(ctx, p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	done, failed := 0, 0
	for _, e := range sh.Entries {
		if e.Kind == "slideshow" {
			for _, sha := range paramStrings(e.Params, "images") {
				if _, err := s.Gallery.File(ctx, "img", sha); err != nil {
					failed++
				} else {
					done++
				}
			}
		}
		packs := paramStrings(e.Params, "packs", "pack")
		packs = append(packs, paramStrings(e.Params, "transition")...)
		for _, id := range packs {
			if stylemap.Has(s.stylemapDir(), id) {
				continue
			}
			if missing, err := s.Gallery.PrefetchPack(ctx, id); err != nil || missing > 0 {
				failed++
			} else {
				done++
			}
		}
	}
	writeJSON(w, 200, map[string]int{"done": done, "failed": failed})
}
