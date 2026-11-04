package api

// M7 subtitles (DECISIONS D15): the MC's microphone in the Cockpit goes through
// this server to the Translator, and transcripts with translations come back to
// the stage as a caption bar. The operator can also type a line. Nothing from a
// guest's phone reaches the subtitles, so they need no approval; PANIC stops the
// microphone and clears the bar.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"sync/atomic"

	"github.com/coder/websocket"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/subtitles"
)

var langRe = regexp.MustCompile(`^[a-z]{2,3}$`)

type mic struct {
	conn *websocket.Conn
	sess *subtitles.Session
}

func (m *mic) close(reason string) {
	if m.sess != nil {
		m.sess.Close()
	}
	m.conn.Close(websocket.StatusNormalClosure, reason)
}

func (s *Server) broadcastSubtitle(code string, l subtitles.Line) {
	s.Hub.Broadcast(code, hub.Msg{Type: "subtitle", Data: l}, hub.Stage, hub.Cockpit)
}

// stopMic ends the party's microphone, if any (PANIC, a second Cockpit taking over).
func (s *Server) stopMic(code string) {
	s.mmu.Lock()
	m := s.mics[code]
	delete(s.mics, code)
	s.mmu.Unlock()
	s.Hub.Broadcast(code, hub.Msg{Type: "subtitle.clear"}, hub.Stage, hub.Cockpit)
	if m != nil {
		// closing waits for both close handshakes; PANIC must not
		go m.close("stopped")
	}
}

var typedID atomic.Int64

// typedSubtitle: the operator types a line (shoutout, announcement).
func (s *Server) typedSubtitle(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		Text string `json:"text"`
	}
	if err := readJSON(r, &in); err != nil || clean(in.Text, 160) == "" {
		writeErr(w, 400, "prázdný titulek")
		return
	}
	// negative ids never collide with the Translator's utterance ids
	id := -1_000_000 - int(typedID.Add(1))
	s.broadcastSubtitle(p.Code, subtitles.Line{ID: id, Text: clean(in.Text, 160), Final: true, Done: true})
	w.WriteHeader(204)
}

func (s *Server) clearSubtitles(w http.ResponseWriter, r *http.Request, p store.Party) {
	s.Hub.Broadcast(p.Code, hub.Msg{Type: "subtitle.clear"}, hub.Stage, hub.Cockpit)
	w.WriteHeader(204)
}

// micSocket: text {"type":"start","langA":"cs","langB":"en"}, then binary PCM16LE
// 16 kHz mono frames, {"type":"end"} after each utterance (the browser's VAD) and
// {"type":"stop"}. The server answers {"type":"ready"} or {"type":"error"}.
func (s *Server) micSocket(w http.ResponseWriter, r *http.Request, p store.Party) {
	if s.Subtitles == nil {
		writeErr(w, 409, "titulky z mikrofonu potřebují TRANSLATOR_URL")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	conn.SetReadLimit(256 << 10)
	ctx := context.WithoutCancel(r.Context())
	reply := func(typ, message string) {
		b, _ := json.Marshal(map[string]string{"type": typ, "message": message})
		conn.Write(ctx, websocket.MessageText, b)
	}
	m := &mic{conn: conn}
	defer func() {
		s.mmu.Lock()
		if s.mics[p.Code] == m {
			delete(s.mics, p.Code)
		}
		s.mmu.Unlock()
		m.close("bye")
	}()
	for {
		typ, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			if m.sess != nil {
				if err := m.sess.Audio(raw); err != nil {
					reply("error", "spojení s Translatorem spadlo")
					return
				}
			}
			continue
		}
		var in struct {
			Type         string `json:"type"`
			LangA, LangB string
		}
		json.Unmarshal(raw, &in)
		switch in.Type {
		case "start":
			if m.sess != nil {
				continue
			}
			if !langRe.MatchString(in.LangA) || !langRe.MatchString(in.LangB) {
				reply("error", "neplatný jazyk")
				continue
			}
			sess, err := s.Subtitles.Open(ctx, in.LangA, in.LangB, func(l subtitles.Line) { s.broadcastSubtitle(p.Code, l) })
			if err != nil {
				slog.Warn("subtitles open", "party", p.Code, "err", err)
				reply("error", err.Error())
				continue
			}
			m.sess = sess
			// one microphone per party: the newest Cockpit wins
			s.mmu.Lock()
			old := s.mics[p.Code]
			s.mics[p.Code] = m
			s.mmu.Unlock()
			if old != nil {
				old.close("replaced")
			}
			go func() {
				<-sess.Done()
				conn.Close(websocket.StatusGoingAway, "translator gone")
			}()
			reply("ready", "")
		case "end":
			if m.sess != nil {
				m.sess.End()
			}
		case "stop":
			return
		}
	}
}
