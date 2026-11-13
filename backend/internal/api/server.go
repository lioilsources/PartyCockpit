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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
	"github.com/lioilsources/PartyCockpit/backend/internal/djfy"
	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
	"github.com/lioilsources/PartyCockpit/backend/internal/glyphs"
	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/subtitles"
	"github.com/lioilsources/PartyCockpit/backend/internal/tribute"
	"github.com/lioilsources/PartyCockpit/backend/internal/webtoon"
)

type Server struct {
	Cfg       config.Config
	Store     *store.Store
	Hub       *hub.Hub
	Gallery   *gallery.Client
	DJfy      *djfy.Manager
	Catalog   *providers.Catalog
	Restyler  providers.Restyler
	Animator  providers.Animator
	Subtitles *subtitles.Client // nil = typed subtitles only
	Glyphs    *glyphs.Set       // nil = notes in a font
	// Webtoon episodes and the pack importer (D23); Init fills them from Cfg when nil.
	Webtoon       *webtoon.Library
	WebtoonImport *webtoon.Installer
	// PromoClown tributes (D24); Init fills them from Cfg when nil.
	Tributes      *tribute.Library
	TributeImport *tribute.Importer
	Classifier    providers.Classifier
	Web           fs.FS  // built frontend (may be empty in dev)
	Version       string // build version, shown on the box screen

	jobs  chan jobTask
	anims chan animTask

	mu        sync.Mutex
	tempo     map[string]Tempo     // party → current tempo
	tap       map[string]Tempo     // party → operator tap-tempo override
	lastReact map[string]time.Time // guest → last reaction

	smu     sync.Mutex
	streams map[string]map[string]*Stream // party → stream id → live phone camera

	limits     *limiter // per-IP budget for unauthenticated writes (D19)
	gameInputs *limiter // per-socket budget for game.input

	mmu  sync.Mutex
	mics map[string]*mic // party → the MC microphone feeding subtitles

	tickets tickets // single-use cockpit socket tickets

	pmu       sync.Mutex
	pairUntil map[string]time.Time // party → the box shows the pairing QR until (D20)

	shmu     sync.Mutex
	showRuns map[string]*showRun // party → Studio show player (D23)
	showUnit time.Duration       // one show "second" (tests shorten it)

	bmu    sync.Mutex
	smJobs map[string]*smJob // session/prompt key → StyleMap build (D23)
	smBusy bool

	gmu   sync.Mutex
	draws map[string][]string // party → guest ids of the last lottery draw
	games map[string]*game    // party → the intermission game in progress
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
	s.tickets.m = map[string]ticket{}
	s.pairUntil = map[string]time.Time{}
	s.limits = newLimiter(func() time.Time { return s.Store.Now() }, writeRate, writeBurst)
	s.gameInputs = newLimiter(time.Now, 30, 30)
	s.draws, s.games = map[string][]string{}, map[string]*game{}
	s.smJobs = map[string]*smJob{}
	s.showRuns = map[string]*showRun{}
	s.jobs = make(chan jobTask, 256)
	s.anims = make(chan animTask, 128)
	s.DJfy.OnSnapshot = s.onDJfySnapshot
	s.initWebtoon()
	s.initTributes()
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
	s.boxStart(ctx)
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
	mux.HandleFunc("PUT /api/parties/{code}/guests/{gid}/play", s.guestWrite(s.setPlay))
	mux.HandleFunc("GET /api/parties/{code}/guests/{gid}/assets", s.guest(s.guestAssets))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/photos", s.guestWrite(s.uploadPhoto))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/stickers", s.guestWrite(s.createSticker))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/ideas", s.guestWrite(s.createIdea))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/reactions", s.guestWrite(s.react))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/music/search", s.guestWrite(s.musicSearch))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/music/propose", s.guestWrite(s.musicPropose))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/music/vote", s.guestWrite(s.musicVote))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/stream", s.guestWrite(s.startStream))
	mux.HandleFunc("DELETE /api/parties/{code}/guests/{gid}/stream", s.guestWrite(s.stopStream))
	mux.HandleFunc("POST /api/parties/{code}/guests/{gid}/dance", s.guestWrite(s.dance))
	mux.HandleFunc("GET /api/parties/{code}/guests/{gid}/files/{name}", s.guest(s.guestFile))
	mux.HandleFunc("GET /api/parties/{code}/queue", s.party(s.getQueue))
	mux.HandleFunc("GET /api/parties/{code}/music", s.party(s.getMusic))
	mux.HandleFunc("GET /api/parties/{code}/recap", s.party(s.recap))
	mux.HandleFunc("GET /api/parties/{code}/recap.zip", s.party(s.recapZip))
	mux.HandleFunc("GET /files/{code}/{name}", s.serveFile)

	// cockpit
	mux.HandleFunc("GET /api/parties/{code}/cockpit", s.cockpit(s.cockpitState))
	mux.HandleFunc("POST /api/parties/{code}/cockpit/ticket", s.cockpit(s.issueTicket))
	mux.HandleFunc("POST /api/parties/{code}/cockpit/rotate", s.cockpit(s.rotateToken))
	mux.HandleFunc("PATCH /api/parties/{code}", s.cockpit(s.patchParty))
	mux.HandleFunc("POST /api/parties/{code}/assets/{id}/approve", s.cockpit(s.moderate("approved")))
	mux.HandleFunc("POST /api/parties/{code}/assets/{id}/reject", s.cockpit(s.moderate("rejected")))
	mux.HandleFunc("PUT /api/parties/{code}/queue", s.cockpit(s.reorderQueue))
	mux.HandleFunc("DELETE /api/parties/{code}/queue/{id}", s.cockpit(s.moderate("rejected")))
	mux.HandleFunc("POST /api/parties/{code}/scene", s.cockpit(s.setScene))
	mux.HandleFunc("POST /api/parties/{code}/panic", s.cockpit(s.panic))
	mux.HandleFunc("POST /api/parties/{code}/tempo", s.cockpit(s.setTempo))
	mux.HandleFunc("POST /api/parties/{code}/lottery", s.cockpit(s.lottery))
	mux.HandleFunc("POST /api/parties/{code}/game", s.cockpit(s.startGame))
	mux.HandleFunc("DELETE /api/parties/{code}/game", s.cockpit(s.stopGame))
	mux.HandleFunc("GET /api/parties/{code}/ideas", s.cockpit(s.listIdeas))
	mux.HandleFunc("GET /api/parties/{code}/streams", s.cockpit(s.listStreams))
	mux.HandleFunc("GET /api/parties/{code}/preflight", s.cockpit(s.preflight))
	mux.HandleFunc("POST /api/parties/{code}/subtitle", s.cockpit(s.typedSubtitle))
	mux.HandleFunc("DELETE /api/parties/{code}/subtitle", s.cockpit(s.clearSubtitles))
	mux.HandleFunc("GET /ws/parties/{code}/mic", s.party(s.micAuth))
	mux.HandleFunc("POST /api/parties/{code}/animate", s.cockpit(s.animateGallery))
	// Scene Studio (D23)
	mux.HandleFunc("GET /api/parties/{code}/show", s.cockpit(s.getShow))
	mux.HandleFunc("PUT /api/parties/{code}/show", s.cockpit(s.putShow))
	mux.HandleFunc("POST /api/parties/{code}/show/play", s.cockpit(s.showControl("play")))
	mux.HandleFunc("POST /api/parties/{code}/show/pause", s.cockpit(s.showControl("pause")))
	mux.HandleFunc("POST /api/parties/{code}/show/next", s.cockpit(s.showControl("next")))
	mux.HandleFunc("POST /api/parties/{code}/show/prev", s.cockpit(s.showControl("prev")))
	mux.HandleFunc("GET /api/parties/{code}/show/ready", s.cockpit(s.showReady))
	mux.HandleFunc("POST /api/parties/{code}/show/prefetch", s.cockpit(s.showPrefetch))
	mux.HandleFunc("POST /api/parties/{code}/studio/photos", s.cockpit(s.studioPhoto))
	mux.HandleFunc("GET /api/parties/{code}/shows", s.cockpit(s.listShows))
	mux.HandleFunc("POST /api/parties/{code}/shows", s.cockpit(s.saveShow))
	mux.HandleFunc("POST /api/parties/{code}/shows/{slug}/load", s.cockpit(s.loadShow))
	mux.HandleFunc("DELETE /api/parties/{code}/shows/{slug}", s.cockpit(s.deleteShow))
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
	mux.HandleFunc("GET /stylemaps/{rel...}", s.serveStyleMaps)
	mux.HandleFunc("GET /api/parties/{code}/finetune/sessions", s.cockpit(s.finetuneSessions))
	mux.HandleFunc("GET /api/parties/{code}/finetune/sessions/{id}/groups", s.cockpit(s.finetuneGroups))
	mux.HandleFunc("POST /api/parties/{code}/stylemaps/build", s.cockpit(s.buildStyleMaps))
	mux.HandleFunc("DELETE /api/parties/{code}/stylemaps/{id}", s.cockpit(s.deleteStyleMap))
	mux.HandleFunc("GET /api/glyphs", func(w http.ResponseWriter, r *http.Request) {
		if s.Glyphs == nil {
			writeJSON(w, 200, []glyphs.Sheet{})
			return
		}
		writeJSON(w, 200, s.Glyphs.Sheets)
	})
	mux.HandleFunc("GET /glyphs/{id}/{file}", func(w http.ResponseWriter, r *http.Request) {
		s.Glyphs.Serve(w, r, r.PathValue("id"), r.PathValue("file"))
	})
	mux.HandleFunc("GET /api/webtoon", s.listWebtoon)
	mux.HandleFunc("GET /api/webtoon/{id}", s.getWebtoon)
	mux.HandleFunc("GET /webtoon/{ep}/{file}", s.serveWebtoon)
	mux.HandleFunc("GET /api/parties/{code}/webtoon/catalog", s.cockpit(s.webtoonCatalog))
	mux.HandleFunc("POST /api/parties/{code}/webtoon/install", s.cockpit(s.webtoonInstall))
	mux.HandleFunc("DELETE /api/parties/{code}/webtoon/packs/{dir}", s.cockpit(s.webtoonRemove))

	mux.HandleFunc("GET /api/tributes", s.listTributes)
	mux.HandleFunc("GET /tributes/{id}/{file}", s.serveTribute)
	mux.HandleFunc("GET /api/parties/{code}/tributes", s.cockpit(s.tributeLibrary))
	mux.HandleFunc("POST /api/parties/{code}/tributes/import", s.cockpit(s.tributeImport))
	mux.HandleFunc("DELETE /api/parties/{code}/tributes/{id}", s.cockpit(s.tributeRemove))
	mux.HandleFunc("GET /api/parties/{code}/tributes/{id}/thumb/{n}", s.party(s.tributeThumb))

	mux.HandleFunc("GET /box", s.box)
	mux.HandleFunc("GET /api/box", s.boxInfo)
	mux.HandleFunc("POST /api/parties/{code}/pair", s.cockpit(s.pair))
	mux.HandleFunc("GET /ws/parties/{code}", s.ws)
	mux.HandleFunc("GET /cockpit/{code}", s.cockpitPage)
	mux.HandleFunc("GET /", s.spa)
	return securityHeaders(logRequests(mux))
}

// securityHeaders: no sniffing, no framing, no referrer. The CSP is report-only
// until it has been checked on a real stage (Pixi, WebGL, the AudioWorklet from
// a blob URL); switch the header name to enforce it (D19).
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self' blob:; worker-src 'self' blob:; " +
		"img-src 'self' data: blob:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; " +
		"font-src 'self'; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'none'"
	// the Studio previews a scene in an <iframe> of the stage (D23): only the
	// stage page may be framed, and only by this origin
	stageCSP := strings.Replace(csp, "frame-ancestors 'none'", "frame-ancestors 'self'", 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/stage/") {
			h.Set("X-Frame-Options", "SAMEORIGIN")
			h.Set("Content-Security-Policy-Report-Only", stageCSP)
		} else {
			h.Set("X-Frame-Options", "DENY")
			h.Set("Content-Security-Policy-Report-Only", csp)
		}
		next.ServeHTTP(w, r)
	})
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
			serverErr(w, r, err)
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
			serverErr(w, r, err)
			return
		}
		h(w, r, p, g)
	})
}

// guestWrite is guest for everything a phone changes: closed once the party
// ended (reading and the recap stay), and within the per-IP budget (D19).
func (s *Server) guestWrite(h guestHandler) http.HandlerFunc {
	return s.guest(func(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
		if !s.partyOpen(w, p) || !s.allowIP(w, r, 1) {
			return
		}
		h(w, r, p, g)
	})
}

func (s *Server) partyOpen(w http.ResponseWriter, p store.Party) bool {
	if p.EndsAt <= s.Store.Now().UnixMilli() {
		writeErr(w, 410, "party skončila")
		return false
	}
	return true
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
		// the list carries every party's cockpit token: never without a token (D19)
		if r.Method == "GET" && (s.Cfg.AdminToken == "" || !eq(r.Header.Get("X-Admin-Token"), s.Cfg.AdminToken)) {
			writeErr(w, 401, "seznam party jen s admin tokenem (PC_ADMIN_TOKEN)")
			return
		}
		// a fresh box (no open party) lets the organiser's phone create the first one
		firstOnBox := r.Method == "POST" && s.Cfg.Box && !s.hasOpenParty(r)
		if s.Cfg.AdminToken != "" && !firstOnBox && !eq(r.Header.Get("X-Admin-Token"), s.Cfg.AdminToken) {
			writeErr(w, 401, "neplatný admin token")
			return
		}
		h(w, r)
	}
}

func (s *Server) hasOpenParty(r *http.Request) bool {
	_, ok := s.openParty(r.Context())
	return ok
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

// serverErr logs the detail and answers a plain 500: a phone does not need
// paths, provider URLs or SQL in an error message (D19).
func serverErr(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeErr(w, 500, "chyba serveru, zkus to za chvíli")
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
	peer := hub.Peer{ID: "p" + randHex(6), Role: role, IP: s.clientIP(r)}
	switch role {
	case hub.Stage:
		// the stage sees and calls every live camera: it needs its key (D19)
		if !eq(r.URL.Query().Get("key"), p.StageKey) {
			writeErr(w, 401, "neplatný klíč plátna")
			return
		}
	case hub.Guest:
		if s.Hub.Count(p.Code, func(c hub.Peer) bool { return c.Role == hub.Guest && c.IP == peer.IP }) >= maxGuestSocketsPerIP {
			writeErr(w, 429, "z tohohle zařízení je otevřeno moc oken")
			return
		}
		// the guest id is the guest's secret; only a known one is attached
		if gid := r.URL.Query().Get("guest"); gid != "" {
			if g, err := s.Store.Guest(r.Context(), p.Code, gid); err == nil {
				peer.GuestID = g.ID
			}
		}
	case hub.Cockpit:
		if !s.cockpitAuthorized(r, p) {
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
	conn, err := websocket.Accept(w, r, s.acceptOptions())
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
	hello = append(hello, s.gameHello(p.Code, peer)...)
	if isBoxStage(peer) { // the stage on this machine (box or laptop)
		hello = append(hello, s.pairingMsg(r.Context(), p.Code))
	}
	if role == hub.Cockpit {
		s.endPairing(r.Context(), p.Code)
	}
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
		msgs = append(msgs, hub.Msg{Type: "queue", Data: publicQueue(q)})
	}
	if snap := s.DJfy.Snapshot(p.Code); snap != nil {
		msgs = append(msgs, hub.Msg{Type: "music", Data: snap})
	}
	msgs = append(msgs, hub.Msg{Type: "tempo", Data: s.currentTempo(p.Code)})
	if role == hub.Stage || role == hub.Cockpit {
		if sh, err := s.showState(ctx, p.Code); err == nil {
			msgs = append(msgs, hub.Msg{Type: "show", Data: sh})
		}
	}
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
				if strings.HasPrefix(name, "assets/") || strings.HasPrefix(name, "react/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFileFS(w, r, s.Web, name)
				return
			}
		}
	}
	index := "index.html"
	if strings.HasPrefix(r.URL.Path, "/react/") {
		// the React Cockpit (D21) is its own SPA under /react/
		index = "react/index.html"
		if _, err := fs.Stat(s.Web, index); err != nil {
			http.Error(w, "react UI not built (make web)", http.StatusNotFound)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, s.Web, index)
}

// cockpitPage: /cockpit/<code> is the Svelte Cockpit unless PC_COCKPIT_UI=react
// (D21). The browser keeps the #token= fragment across the redirect, so the
// box's pairing QR and links from Home work with either UI.
func (s *Server) cockpitPage(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.CockpitUI == "react" {
		http.Redirect(w, r, "/react/cockpit/"+url.PathEscape(r.PathValue("code")), http.StatusFound)
		return
	}
	s.spa(w, r)
}

func (s *Server) partyDir(code string) string { return filepath.Join(s.Cfg.DataDir, "party", code) }

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
