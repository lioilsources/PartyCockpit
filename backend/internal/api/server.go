// Package api is the HTTP + WebSocket surface of PartyCockpit.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
	"github.com/lioilsources/PartyCockpit/backend/internal/djfy"
	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/subtitles"
)

type Server struct {
	Cfg        config.Config
	Store      *store.Store
	Hub        *hub.Hub
	Gallery    *gallery.Client
	DJfy       *djfy.Manager
	Catalog    *providers.Catalog
	Restyler   providers.Restyler
	Animator   providers.Animator
	Subtitles  *subtitles.Client // nil = typed subtitles only
	Classifier providers.Classifier
	Web        fs.FS // built frontend (may be empty in dev)

	jobs  chan jobTask
	anims chan animTask

	mu        sync.Mutex
	tempo     map[string]Tempo     // party → current tempo
	tap       map[string]Tempo     // party → operator tap-tempo override
	lastReact map[string]time.Time // guest → last reaction

	smu     sync.Mutex
	streams map[string]map[string]*Stream // party → stream id → live phone camera

	mmu  sync.Mutex
	mics map[string]*mic // party → the MC microphone feeding subtitles
}

// Tempo drives every scene's tick(beat). Source: tap | djfy | none; the stage
// falls back to its own audio analysis when source is none (DECISIONS.md D4).
type Tempo struct {
	BPM    float64 `json:"bpm"`
	Source string  `json:"source"`
	Anchor int64   `json:"anchor"` // ms epoch of a beat
}

func (s *Server) Init(ctx context.Context) error {
	s.tempo, s.tap, s.lastReact = map[string]Tempo{}, map[string]Tempo{}, map[string]time.Time{}
	s.streams = map[string]map[string]*Stream{}
	s.mics = map[string]*mic{}
	s.jobs = make(chan jobTask, 256)
	s.anims = make(chan animTask, 128)
	s.DJfy.OnSnapshot = s.onDJfySnapshot
	parties, err := s.Store.Parties(ctx)
	if err != nil {
		return err
	}
	for _, p := range parties {
		if p.DJfyCode != "" {
			s.DJfy.Watch(p.Code, p.DJfyCode)
		}
	}
	abandoned, err := s.Store.AbandonRunningJobs(ctx)
	if err != nil {
		return err
	}
	for _, id := range abandoned {
		s.fallbackToOriginal(ctx, id, "server restarted")
	}
	for range max(1, s.Cfg.Workers) {
		go s.worker()
	}
	for range max(1, s.Cfg.AnimWorkers) {
		go s.animWorker()
	}
	go s.janitor()
	return nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /metrics", s.metrics)

	// parties (admin)
	mux.HandleFunc("POST /api/parties", s.admin(s.createParty))
	mux.HandleFunc("GET /api/parties", s.admin(s.listParties))

	// public / guest
	mux.HandleFunc("GET /api/parties/{code}", s.party(s.getParty))
	mux.HandleFunc("POST /api/parties/{code}/guests", s.party(s.createGuest))
	mux.HandleFunc("PUT /api/parties/{code}/guests/{gid}/play", s.guest(s.setPlay))
	mux.HandleFunc("GET /api/parties/{code}/guests/{gid}/assets", s.guest(s.guestAssets))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/photos", s.guest(s.uploadPhoto))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/stickers", s.guest(s.createSticker))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/ideas", s.guest(s.createIdea))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/reactions", s.guest(s.react))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/music/search", s.guest(s.musicSearch))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/music/propose", s.guest(s.musicPropose))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/music/vote", s.guest(s.musicVote))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/stream", s.guest(s.startStream))
	mux.HandleFunc("DELETE /api/parties/{code}/guests/{gid}/stream", s.guest(s.stopStream))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/dance", s.guest(s.dance))
	mux.HandleFunc("GET /api/parties/{code}/queue", s.party(s.getQueue))
	mux.HandleFunc("GET /api/parties/{code}/music", s.party(s.getMusic))
	mux.HandleFunc("GET /api/parties/{code}/recap", s.party(s.recap))
	mux.HandleFunc("GET /api/parties/{code}/recap.zip", s.party(s.recapZip))
	mux.HandleFunc("GET /files/{code}/{name}", s.serveFile)

	// cockpit
	mux.HandleFunc("GET /api/parties/{code}/cockpit", s.cockpit(s.cockpitState))
	mux.HandleFunc("PATCH /api/parties/{code}", s.cockpit(s.patchParty))
	mux.HandleFunc("POST /api/parties/{code}/assets/{id}/approve", s.cockpit(s.moderate("approved")))
	mux.HandleFunc("POST /api/parties/{code}/assets/{id}/reject", s.cockpit(s.moderate("rejected")))
	mux.HandleFunc("PUT /api/parties/{code}/queue", s.cockpit(s.reorderQueue))
	mux.HandleFunc("DELETE /api/parties/{code}/queue/{id}", s.cockpit(s.moderate("rejected")))
	mux.HandleFunc("POST /api/parties/{code}/scene", s.cockpit(s.setScene))
	mux.HandleFunc("POST /api/parties/{code}/panic", s.cockpit(s.panic))
	mux.HandleFunc("POST /api/parties/{code}/tempo", s.cockpit(s.setTempo))
	mux.HandleFunc("POST /api/parties/{code}/lottery", s.cockpit(s.lottery))
	mux.HandleFunc("GET /api/parties/{code}/ideas", s.cockpit(s.listIdeas))
	mux.HandleFunc("GET /api/parties/{code}/streams", s.cockpit(s.listStreams))
	mux.HandleFunc("GET /api/parties/{code}/preflight", s.cockpit(s.preflight))
	mux.HandleFunc("POST /api/parties/{code}/subtitle", s.cockpit(s.typedSubtitle))
	mux.HandleFunc("DELETE /api/parties/{code}/subtitle", s.cockpit(s.clearSubtitles))
	mux.HandleFunc("GET /ws/parties/{code}/mic", s.cockpit(s.micSocket))
	mux.HandleFunc("POST /api/parties/{code}/animate", s.cockpit(s.animateGallery))
	mux.HandleFunc("POST /api/parties/{code}/streams/{sid}/approve", s.cockpit(s.streamStatus("live")))
	mux.HandleFunc("POST /api/parties/{code}/streams/{sid}/hold", s.cockpit(s.streamStatus("pending")))
	mux.HandleFunc("DELETE /api/parties/{code}/streams/{sid}", s.cockpit(s.kickStream))

	// catalogs
	mux.HandleFunc("GET /api/motions", s.listMotions)
	mux.HandleFunc("GET /api/styles", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Catalog.Public()) })
	mux.HandleFunc("GET /api/gallery", s.listGallery)
	mux.HandleFunc("GET /api/gallery/{kind}/{sha}", func(w http.ResponseWriter, r *http.Request) {
		s.Gallery.ServeFile(w, r, r.PathValue("kind"), r.PathValue("sha"))
	})
	mux.HandleFunc("GET /stylemaps/{rel...}", func(w http.ResponseWriter, r *http.Request) { s.Gallery.ServeStyleMap(w, r, r.PathValue("rel")) })
	mux.HandleFunc("GET /api/webtoon", s.listWebtoon)
	mux.HandleFunc("GET /webtoon/{ep}/{file}", s.serveWebtoon)

	mux.HandleFunc("GET /ws/parties/{code}", s.ws)
	mux.HandleFunc("GET /", s.spa)
	return logRequests(mux)
}

// ── middleware ─────────────────────────────────────────────────────────────

type partyHandler func(w http.ResponseWriter, r *http.Request, p store.Party)
type guestHandler func(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest)

func (s *Server) party(h partyHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Store.Party(r.Context(), r.PathValue("code"))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 404, "party nenalezena")
			return
		} else if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		h(w, r, p)
	}
}

func (s *Server) guest(h guestHandler) http.HandlerFunc {
	return s.party(func(w http.ResponseWriter, r *http.Request, p store.Party) {
		g, err := s.Store.Guest(r.Context(), p.Code, r.PathValue("gid"))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 404, "host nenalezen")
			return
		} else if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		h(w, r, p, g)
	})
}

func bearer(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func eq(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func (s *Server) cockpit(h partyHandler) http.HandlerFunc {
	return s.party(func(w http.ResponseWriter, r *http.Request, p store.Party) {
		if !eq(bearer(r), p.CockpitToken) {
			writeErr(w, 401, "neplatný cockpit token")
			return
		}
		h(w, r, p)
	})
}

func (s *Server) admin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Cfg.AdminToken != "" && !eq(r.Header.Get("X-Admin-Token"), s.Cfg.AdminToken) {
			writeErr(w, 401, "neplatný admin token")
			return
		}
		h(w, r)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != "GET" {
			slog.Info("http", "method", r.Method, "path", r.URL.Path, "ms", time.Since(start).Milliseconds())
		}
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10)).Decode(v)
}

// ── websocket ──────────────────────────────────────────────────────────────

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.Party(r.Context(), r.PathValue("code"))
	if err != nil {
		writeErr(w, 404, "party nenalezena")
		return
	}
	role := hub.Role(r.URL.Query().Get("role"))
	peer := hub.Peer{ID: "p" + randHex(6), Role: role}
	switch role {
	case hub.Stage, hub.Guest:
	case hub.Cockpit:
		if !eq(bearer(r), p.CockpitToken) {
			writeErr(w, 401, "neplatný cockpit token")
			return
		}
	case hub.Streamer:
		s.smu.Lock()
		st := s.streamByKey(p.Code, r.URL.Query().Get("key"))
		if st != nil {
			peer.StreamID = st.ID
		}
		s.smu.Unlock()
		if st == nil {
			writeErr(w, 401, "neplatný klíč streamu")
			return
		}
	default:
		writeErr(w, 400, "role must be stage|cockpit|guest|streamer")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // LAN, any origin
	if err != nil {
		return
	}
	hello, err := s.helloFor(r, p, role)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "state")
		return
	}
	hello = append(hello, hub.Msg{Type: "hello", Data: map[string]string{"peer": peer.ID}})
	hello = append(hello, s.streamHello(p.Code, peer)...)
	s.Hub.Serve(context.WithoutCancel(r.Context()), conn, p.Code, peer, hello, hub.Handlers{OnMsg: s.onMsg, OnLeave: s.onLeave})
}

func (s *Server) helloFor(r *http.Request, p store.Party, role hub.Role) ([]hub.Msg, error) {
	ctx := r.Context()
	pub, err := s.publicParty(r, p)
	if err != nil {
		return nil, err
	}
	msgs := []hub.Msg{{Type: "party", Data: pub}}
	if role == hub.Stage || role == hub.Cockpit {
		q, err := s.Store.StageQueue(ctx, p.Code)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, hub.Msg{Type: "queue", Data: q})
	}
	if snap := s.DJfy.Snapshot(p.Code); snap != nil {
		msgs = append(msgs, hub.Msg{Type: "music", Data: snap})
	}
	msgs = append(msgs, hub.Msg{Type: "tempo", Data: s.currentTempo(p.Code)})
	if role == hub.Cockpit {
		st, err := s.cockpitPayload(ctx, p)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, hub.Msg{Type: "cockpit", Data: st})
	}
	return msgs, nil
}

// ── SPA + files ────────────────────────────────────────────────────────────

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if s.Web == nil {
		http.Error(w, "frontend not built (make web)", http.StatusNotFound)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "" {
		if f, err := s.Web.Open(name); err == nil {
			st, _ := f.Stat()
			f.Close()
			if st != nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFileFS(w, r, s.Web, name)
				return
			}
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, s.Web, "index.html")
}

func (s *Server) partyDir(code string) string { return filepath.Join(s.Cfg.DataDir, "party", code) }

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	code, name := strings.ToUpper(r.PathValue("code")), r.PathValue("name")
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") || strings.ContainsAny(code, `/\.`) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, filepath.Join(s.partyDir(code), name))
}

// janitor deletes parties (rows + files) 7 days after they ended (plan §8).
func (s *Server) janitor() {
	for {
		ctx := context.Background()
		codes, err := s.Store.ExpiredParties(ctx, 7*24*time.Hour)
		if err != nil {
			slog.Error("janitor", "err", err)
		}
		for _, c := range codes {
			os.RemoveAll(s.partyDir(c))
			if err := s.Store.DeleteParty(ctx, c); err != nil {
				slog.Error("janitor delete", "party", c, "err", err)
			} else {
				slog.Info("janitor purged party", "party", c)
			}
		}
		time.Sleep(6 * time.Hour)
	}
}
