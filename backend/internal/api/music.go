package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/lioilsources/PartyCockpit/backend/internal/djfy"
	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

func (s *Server) getMusic(w http.ResponseWriter, r *http.Request, p store.Party) {
	snap := s.DJfy.Snapshot(p.Code)
	if snap == nil {
		snap = json.RawMessage("null")
	}
	writeJSON(w, 200, map[string]any{"djfyCode": p.DJfyCode, "snapshot": snap})
}

func who(g store.Guest) djfy.Who { return djfy.Who{GuestID: g.ID, Name: g.Nickname} }

func musicErr(w http.ResponseWriter, err error) {
	if errors.Is(err, djfy.ErrNoSession) {
		writeErr(w, 409, "na party zatím nehraje DJfy")
		return
	}
	writeErr(w, 502, err.Error())
}

func (s *Server) musicSearch(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		Q string `json:"q"`
	}
	readJSON(r, &in)
	q := clean(in.Q, 80)
	if utf8.RuneCountInString(q) < 2 {
		writeErr(w, 400, "hledej aspoň 2 znaky")
		return
	}
	res, err := s.DJfy.Search(r.Context(), p.Code, who(g), q)
	if err != nil {
		musicErr(w, err)
		return
	}
	if res == nil {
		res = json.RawMessage("[]")
	}
	writeJSON(w, 200, map[string]any{"results": res})
}

func (s *Server) musicPropose(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		TrackID string `json:"trackId"`
	}
	readJSON(r, &in)
	if in.TrackID == "" || len(in.TrackID) > 64 {
		writeErr(w, 400, "chybí skladba")
		return
	}
	toast, err := s.DJfy.Propose(r.Context(), p.Code, who(g), in.TrackID)
	if err != nil {
		musicErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"toast": toast})
}

func (s *Server) musicVote(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		EntryID string `json:"entryId"`
		Value   int    `json:"value"`
	}
	readJSON(r, &in)
	toast, err := s.DJfy.Vote(r.Context(), p.Code, who(g), in.EntryID, in.Value)
	if err != nil {
		musicErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"toast": toast})
}

// onDJfySnapshot forwards the host phone's snapshot to every client and updates
// the tempo from nowPlaying.bpm (unless the operator tapped one in).
func (s *Server) onDJfySnapshot(party string, snap json.RawMessage) {
	if snap == nil {
		snap = json.RawMessage("null")
	}
	s.Hub.Broadcast(party, hub.Msg{Type: "music", Data: snap})
	var sn struct {
		IsPlaying  bool `json:"isPlaying"`
		NowPlaying *struct {
			ID  string  `json:"id"`
			BPM float64 `json:"bpm"`
		} `json:"nowPlaying"`
	}
	json.Unmarshal(snap, &sn)
	t := Tempo{Source: "none"}
	if sn.NowPlaying != nil && sn.NowPlaying.BPM > 40 && sn.IsPlaying {
		t = Tempo{BPM: sn.NowPlaying.BPM, Source: "djfy"}
	}
	s.mu.Lock()
	prev := s.tempo[party]
	if t.Source == prev.Source && t.BPM == prev.BPM {
		s.mu.Unlock()
		return
	}
	t.Anchor = time.Now().UnixMilli()
	s.tempo[party] = t
	_, tapped := s.tap[party]
	s.mu.Unlock()
	if !tapped {
		s.Hub.Broadcast(party, hub.Msg{Type: "tempo", Data: t})
	}
}

func (s *Server) currentTempo(party string) Tempo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tap[party]; ok {
		return t
	}
	if t, ok := s.tempo[party]; ok {
		return t
	}
	return Tempo{Source: "none"}
}

// setTempo: operator tap tempo (bpm > 0) or clear back to DJfy/audio (bpm = 0).
// anchor is the ms timestamp of the last tap, so the stage lands on the beat.
func (s *Server) setTempo(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		BPM    float64 `json:"bpm"`
		Anchor int64   `json:"anchor"`
	}
	readJSON(r, &in)
	s.mu.Lock()
	if in.BPM <= 0 {
		delete(s.tap, p.Code)
	} else {
		if in.BPM < 40 || in.BPM > 220 {
			s.mu.Unlock()
			writeErr(w, 400, "bpm 40–220")
			return
		}
		if in.Anchor == 0 {
			in.Anchor = time.Now().UnixMilli()
		}
		s.tap[p.Code] = Tempo{BPM: in.BPM, Source: "tap", Anchor: in.Anchor}
	}
	s.mu.Unlock()
	t := s.currentTempo(p.Code)
	s.Hub.Broadcast(p.Code, hub.Msg{Type: "tempo", Data: t})
	writeJSON(w, 200, t)
}
