package api

// Live video from guests' phones (plan M3, DECISIONS.md D12).
//
// The party is on one LAN (D2), so there is no SFU: every viewer (the stage, and
// the cockpit for its preview) opens one peer-to-peer WebRTC connection to every
// stream it shows, and PartyCockpit only relays the signalling over the party
// WebSocket. A stream is in memory only; it ends when its phone goes away.
//
//	phone:   POST /guests/{gid}/stream → {stream, key}
//	         WS /ws/parties/{code}?role=streamer&key=KEY
//	cockpit: POST /streams/{sid}/approve → status live → stage sees it
//	viewer → server → phone:  {type:"rtc", data:{kind:"offer", stream, sdp}}
//	phone  → server → viewer: {type:"rtc", data:{kind:"answer", to, sdp}}
//	both ways:                {type:"rtc", data:{kind:"ice"|"bye", ...}}
//
// Nothing reaches the stage before the operator approves the stream: the stage is
// told only about live streams, and the server refuses to relay its offers to
// pending ones.

import (
	"fmt"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

const (
	maxLiveStreams  = 9                // 3×3 wall; every viewer decodes each one (PC_MAX_LIVE lowers it)
	maxStreams      = 24               // pending + live per party
	streamConnectIn = 90 * time.Second // a new stream must connect its socket by then
	streamGrace     = 20 * time.Second // a dropped phone may reconnect within this
)

type Stream struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"` // browser | mirrorbooth
	Status    string `json:"status"` // pending | live
	Online    bool   `json:"online"` // the phone's streamer socket is connected
	CreatedAt int64  `json:"createdAt"`

	guestID string
	key     string
	expire  *time.Timer
}

type rtcMsg struct {
	Kind      string          `json:"kind"` // offer | answer | ice | bye
	Stream    string          `json:"stream,omitempty"`
	To        string          `json:"to,omitempty"`
	From      string          `json:"from,omitempty"`
	SDP       string          `json:"sdp,omitempty"`
	Candidate json.RawMessage `json:"candidate,omitempty"`
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// streamList returns the party's streams, oldest first; liveOnly for the stage.
// Caller holds s.smu.
func (s *Server) streamList(code string, liveOnly bool) []Stream {
	out := []Stream{}
	for _, st := range s.streams[code] {
		if liveOnly && st.Status != "live" {
			continue
		}
		out = append(out, *st)
	}
	slices.SortFunc(out, func(a, b Stream) int { return int(a.CreatedAt - b.CreatedAt) })
	return out
}

// broadcastStreams tells the stage (live only), the cockpit (all) and each
// phone (its own stream) what changed. Caller holds s.smu.
func (s *Server) broadcastStreams(code string) {
	s.Hub.Broadcast(code, hub.Msg{Type: "streams", Data: s.streamList(code, true)}, hub.Stage)
	s.Hub.Broadcast(code, hub.Msg{Type: "streams", Data: s.streamList(code, false)}, hub.Cockpit)
	for _, st := range s.streams[code] {
		s.Hub.SendTo(code, hub.Msg{Type: "stream", Data: *st}, isStreamer(st.ID))
	}
}

func isStreamer(id string) func(hub.Peer) bool {
	return func(p hub.Peer) bool { return p.Role == hub.Streamer && p.StreamID == id }
}

// endStream removes a stream and tells its phone why. Caller holds s.smu.
func (s *Server) endStream(code, id, reason string) {
	st := s.streams[code][id]
	if st == nil {
		return
	}
	if st.expire != nil {
		st.expire.Stop()
	}
	delete(s.streams[code], id)
	if len(s.streams[code]) == 0 {
		delete(s.streams, code)
	}
	s.Hub.SendTo(code, hub.Msg{Type: "stream", Data: map[string]any{"id": id, "status": "ended", "reason": reason}}, isStreamer(id))
	s.broadcastStreams(code)
}

// armExpiry ends the stream after d unless something stops the timer first.
// Caller holds s.smu.
func (s *Server) armExpiry(code string, st *Stream, d time.Duration, reason string) {
	if st.expire != nil {
		st.expire.Stop()
	}
	id := st.ID
	st.expire = time.AfterFunc(d, func() {
		s.smu.Lock()
		defer s.smu.Unlock()
		if cur := s.streams[code][id]; cur == st && !st.Online {
			s.endStream(code, id, reason)
		}
	})
}

// ── HTTP ───────────────────────────────────────────────────────────────────

// startStream registers the guest's phone as a camera. One stream per guest:
// asking again returns the same stream and key (page reload, MirrorBooth retry).
func (s *Server) startStream(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		Source string `json:"source"`
	}
	_ = readJSON(r, &in)
	if in.Source != "mirrorbooth" {
		in.Source = "browser"
	}
	s.smu.Lock()
	defer s.smu.Unlock()
	for _, st := range s.streams[p.Code] {
		if st.guestID == g.ID {
			writeJSON(w, 200, map[string]any{"stream": *st, "key": st.key})
			return
		}
	}
	if len(s.streams[p.Code]) >= maxStreams {
		writeErr(w, 429, "na party už streamuje moc telefonů")
		return
	}
	st := &Stream{
		ID: "s" + randHex(6), Name: g.Nickname, Source: in.Source, Status: "pending",
		CreatedAt: time.Now().UnixMilli(), guestID: g.ID, key: randHex(16),
	}
	if s.streams[p.Code] == nil {
		s.streams[p.Code] = map[string]*Stream{}
	}
	s.streams[p.Code][st.ID] = st
	s.armExpiry(p.Code, st, streamConnectIn, "telefon se nepřipojil")
	s.broadcastStreams(p.Code)
	writeJSON(w, 201, map[string]any{"stream": *st, "key": st.key})
}

func (s *Server) stopStream(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	s.smu.Lock()
	defer s.smu.Unlock()
	for _, st := range s.streams[p.Code] {
		if st.guestID == g.ID {
			s.endStream(p.Code, st.ID, "ukončeno")
		}
	}
	w.WriteHeader(204)
}

func (s *Server) listStreams(w http.ResponseWriter, r *http.Request, p store.Party) {
	s.smu.Lock()
	defer s.smu.Unlock()
	writeJSON(w, 200, s.streamList(p.Code, false))
}

// approveStream puts a stream on the stage; status "pending" takes it back off.
func (s *Server) streamStatus(status string) partyHandler {
	return func(w http.ResponseWriter, r *http.Request, p store.Party) {
		s.smu.Lock()
		defer s.smu.Unlock()
		st := s.streams[p.Code][r.PathValue("sid")]
		if st == nil {
			writeErr(w, 404, "stream nenalezen")
			return
		}
		if status == "live" && st.Status != "live" && len(s.streamList(p.Code, true)) >= s.maxLive() {
			writeErr(w, 409, fmt.Sprintf("na plátně je už %d streamů", s.maxLive()))
			return
		}
		st.Status = status
		s.broadcastStreams(p.Code)
		writeJSON(w, 200, *st)
	}
}

func (s *Server) kickStream(w http.ResponseWriter, r *http.Request, p store.Party) {
	s.smu.Lock()
	defer s.smu.Unlock()
	if s.streams[p.Code][r.PathValue("sid")] == nil {
		writeErr(w, 404, "stream nenalezen")
		return
	}
	s.endStream(p.Code, r.PathValue("sid"), "ukončil operátor")
	w.WriteHeader(204)
}

// unliveAll is part of PANIC: every stream goes back to pending (phones stay
// connected, the operator re-approves who should be on).
func (s *Server) unliveAll(code string) {
	s.smu.Lock()
	defer s.smu.Unlock()
	for _, st := range s.streams[code] {
		st.Status = "pending"
	}
	s.broadcastStreams(code)
}

// ── WebSocket ──────────────────────────────────────────────────────────────

// streamByKey authenticates a streamer socket. Caller holds s.smu.
func (s *Server) streamByKey(code, key string) *Stream {
	if key == "" {
		return nil
	}
	for _, st := range s.streams[code] {
		if eq(st.key, key) {
			return st
		}
	}
	return nil
}

func (s *Server) streamHello(code string, peer hub.Peer) []hub.Msg {
	s.smu.Lock()
	defer s.smu.Unlock()
	switch peer.Role {
	case hub.Stage:
		return []hub.Msg{{Type: "streams", Data: s.streamList(code, true)}}
	case hub.Cockpit:
		return []hub.Msg{{Type: "streams", Data: s.streamList(code, false)}}
	case hub.Streamer:
		if st := s.streams[code][peer.StreamID]; st != nil {
			st.Online = true
			if st.expire != nil {
				st.expire.Stop()
			}
			s.broadcastStreams(code)
			return []hub.Msg{{Type: "stream", Data: *st}}
		}
	}
	return nil
}

func (s *Server) onLeave(code string, peer hub.Peer) {
	switch peer.Role {
	case hub.Stage, hub.Cockpit:
		// the viewer's peer connections are gone; let the phones free them
		b := hub.Msg{Type: "rtc", Data: rtcMsg{Kind: "bye", From: peer.ID}}
		s.Hub.SendTo(code, b, func(p hub.Peer) bool { return p.Role == hub.Streamer })
	case hub.Streamer:
		s.smu.Lock()
		defer s.smu.Unlock()
		st := s.streams[code][peer.StreamID]
		if st == nil || len(s.Hub.Find(code, isStreamer(st.ID))) > 0 {
			return
		}
		st.Online = false
		s.armExpiry(code, st, streamGrace, "telefon se odpojil")
		s.broadcastStreams(code)
	}
}

// onMsg relays WebRTC signalling. Viewers address a stream, phones answer a
// viewer peer; nobody else can be reached, and the stage only reaches live streams.
func (s *Server) onMsg(code string, from hub.Peer, raw []byte) {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return
	}
	switch head.Type {
	case "rtc":
		s.onRTC(code, from, raw)
	case "game.input":
		s.onGameInput(code, from, raw)
	}
}

func (s *Server) onRTC(code string, from hub.Peer, raw []byte) {
	var env struct {
		Type string `json:"type"`
		Data rtcMsg `json:"data"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return
	}
	m := env.Data
	if len(m.SDP) > 32<<10 || len(m.Candidate) > 2<<10 {
		return
	}
	switch from.Role {
	case hub.Stage, hub.Cockpit:
		if m.Kind != "offer" && m.Kind != "ice" && m.Kind != "bye" {
			return
		}
		s.smu.Lock()
		st := s.streams[code][m.Stream]
		ok := st != nil && (from.Role == hub.Cockpit || st.Status == "live")
		s.smu.Unlock()
		if !ok {
			return
		}
		m.From, m.To = from.ID, ""
		s.Hub.SendTo(code, hub.Msg{Type: "rtc", Data: m}, isStreamer(m.Stream))
	case hub.Streamer:
		if m.Kind != "answer" && m.Kind != "ice" && m.Kind != "bye" {
			return
		}
		s.smu.Lock()
		st := s.streams[code][from.StreamID]
		live := st != nil && st.Status == "live"
		s.smu.Unlock()
		if st == nil {
			return
		}
		to := m.To
		m.From, m.To, m.Stream = from.ID, "", from.StreamID
		s.Hub.SendTo(code, hub.Msg{Type: "rtc", Data: m}, func(p hub.Peer) bool {
			return p.ID == to && (p.Role == hub.Cockpit || (p.Role == hub.Stage && live))
		})
	}
}
