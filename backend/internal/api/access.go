package api

// Access to party files and cockpit sockets without secrets in URLs (D19).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

// fileKey lets the Cockpit's <img> tags show pending files (they can't send
// headers). It is derived from the cockpit token, so it only opens files and
// changes when the token is rotated.
func fileKey(p store.Party) string {
	m := hmac.New(sha256.New, []byte(p.CockpitToken))
	m.Write([]byte("partycockpit-files:" + p.Code))
	return hex.EncodeToString(m.Sum(nil))[:32]
}

func validFileName(code, name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\`) && !strings.HasPrefix(name, ".") && !strings.ContainsAny(code, `/\.`)
}

// serveFile: approved files for anyone with the code, everything for the
// Cockpit's file key. Originals and pending or rejected files stay private.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	code, name := strings.ToUpper(r.PathValue("code")), r.PathValue("name")
	if !validFileName(code, name) {
		http.NotFound(w, r)
		return
	}
	p, err := s.Store.Party(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !eq(r.URL.Query().Get("k"), fileKey(p)) {
		if ok, err := s.Store.FilePublic(r.Context(), code, name); err != nil || !ok {
			http.NotFound(w, r)
			return
		}
	}
	sendPartyFile(w, r, filepath.Join(s.partyDir(code), name))
}

// guestFile: a guest's own files in any state (their pending restyle, a failed one).
func (s *Server) guestFile(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	name := r.PathValue("name")
	if !validFileName(p.Code, name) {
		http.NotFound(w, r)
		return
	}
	if ok, err := s.Store.GuestOwnsFile(r.Context(), p.Code, g.ID, name); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	sendPartyFile(w, r, filepath.Join(s.partyDir(p.Code), name))
}

func sendPartyFile(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// ── socket tickets ──────────────────────────────────────────────────────────

// A ticket stands in for the cockpit token in a WebSocket URL: single use,
// valid 30 s, so a URL that lands in a log or a history is worthless.
const ticketTTL = 30 * time.Second

type tickets struct {
	mu sync.Mutex
	m  map[string]ticket
}

type ticket struct {
	code string
	exp  time.Time
}

func (s *Server) issueTicket(w http.ResponseWriter, r *http.Request, p store.Party) {
	t := randHex(16)
	now := s.Store.Now()
	s.tickets.mu.Lock()
	for k, v := range s.tickets.m {
		if now.After(v.exp) {
			delete(s.tickets.m, k)
		}
	}
	s.tickets.m[t] = ticket{code: p.Code, exp: now.Add(ticketTTL)}
	s.tickets.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ticket": t})
}

// cockpitAuthorized accepts the bearer token (header, or ?token= from older
// clients) or a fresh ticket for this party.
func (s *Server) cockpitAuthorized(r *http.Request, p store.Party) bool {
	if eq(bearer(r), p.CockpitToken) {
		return true
	}
	t := r.URL.Query().Get("ticket")
	if t == "" {
		return false
	}
	s.tickets.mu.Lock()
	defer s.tickets.mu.Unlock()
	v, ok := s.tickets.m[t]
	delete(s.tickets.m, t)
	return ok && v.code == p.Code && !s.Store.Now().After(v.exp)
}

// ── origin ──────────────────────────────────────────────────────────────────

// acceptOptions: a socket from another web page is refused. Same host always
// passes (that is coder/websocket's default); PC_PUBLIC_URL's host and
// localhost (vite dev on :5173) are added.
func (s *Server) acceptOptions() *websocket.AcceptOptions {
	pats := []string{"localhost:*", "127.0.0.1:*"}
	if u, err := url.Parse(s.Cfg.PublicURL); err == nil && u.Host != "" {
		pats = append(pats, u.Host)
	}
	return &websocket.AcceptOptions{OriginPatterns: pats}
}

// rotateToken (cockpit): a leaked token (a photo of the tablet) can be
// replaced. Open cockpit sockets drop; the caller gets the new token.
func (s *Server) rotateToken(w http.ResponseWriter, r *http.Request, p store.Party) {
	token := store.NewID(16)
	if err := s.Store.SetCockpitToken(r.Context(), p.Code, token); err != nil {
		serverErr(w, r, err)
		return
	}
	s.Hub.Close(p.Code, func(c hub.Peer) bool { return c.Role == hub.Cockpit })
	writeJSON(w, 200, map[string]string{"cockpitToken": token})
}
