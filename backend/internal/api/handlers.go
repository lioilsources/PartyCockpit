package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

// ── parties ────────────────────────────────────────────────────────────────

// Limits on what a party's guests can pile up (D19). A real party has tens
// of guests; these only stop a script.
const (
	maxGuests            = 400
	maxNotesPerGuest     = 200
	maxNotesPerMinute    = 20
	maxIdeasPerGuest     = 100
	maxGuestSocketsPerIP = 8
	maxPartyExtension    = 24 * time.Hour
)

func (s *Server) createParty(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.AdminToken == "" && !s.allowIP(w, r, 5) {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	readJSON(r, &in)
	name := clean(in.Name, 60)
	if name == "" {
		name = "Party"
	}
	p, err := s.Store.CreateParty(r.Context(), name, s.Cfg.PartyTTL)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	pub, _ := s.publicParty(r, p)
	writeJSON(w, 201, map[string]any{"party": pub, "cockpitToken": p.CockpitToken, "stageKey": p.StageKey})
}

func (s *Server) listParties(w http.ResponseWriter, r *http.Request) {
	ps, err := s.Store.Parties(r.Context())
	if err != nil {
		serverErr(w, r, err)
		return
	}
	type row struct {
		store.Party
		CockpitToken string `json:"cockpitToken"`
		StageKey     string `json:"stageKey"`
	}
	out := []row{}
	for _, p := range ps {
		out = append(out, row{p, p.CockpitToken, p.StageKey})
	}
	writeJSON(w, 200, out)
}

type PublicParty struct {
	store.Party
	JoinURL string      `json:"joinUrl"`
	Scene   store.Scene `json:"scene"`
	Clients any         `json:"clients"`
	// IceServers for RTCPeerConnection; empty on a LAN party (host candidates suffice).
	IceServers json.RawMessage `json:"iceServers"`
}

func (s *Server) joinURL(r *http.Request, code string) string {
	base := s.Cfg.PublicURL
	if base == "" {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/join/" + code
}

func (s *Server) publicParty(r *http.Request, p store.Party) (PublicParty, error) {
	sc, err := s.Store.Scene(r.Context(), p.Code)
	return PublicParty{Party: p, JoinURL: s.joinURL(r, p.Code), Scene: sc, Clients: s.Hub.Counts(p.Code), IceServers: s.Cfg.ICEServers}, err
}

func (s *Server) getParty(w http.ResponseWriter, r *http.Request, p store.Party) {
	pub, err := s.publicParty(r, p)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	writeJSON(w, 200, pub)
}

func (s *Server) broadcastParty(r *http.Request, code string) {
	p, err := s.Store.Party(r.Context(), code)
	if err != nil {
		return
	}
	if pub, err := s.publicParty(r, p); err == nil {
		s.Hub.Broadcast(code, hub.Msg{Type: "party", Data: pub})
	}
}

func (s *Server) patchParty(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in store.PartyPatch
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	if in.Mode != nil && *in.Mode != "strict" && *in.Mode != "relaxed" {
		writeErr(w, 400, "mode: strict|relaxed")
		return
	}
	if in.LoopMode != nil && *in.LoopMode != "loop" && *in.LoopMode != "shuffle" {
		writeErr(w, 400, "loopMode: loop|shuffle")
		return
	}
	if in.Name != nil {
		n := clean(*in.Name, 60)
		in.Name = &n
	}
	if in.EndsAt != nil {
		now := s.Store.Now()
		if *in.EndsAt <= now.UnixMilli() || *in.EndsAt > now.Add(maxPartyExtension).UnixMilli() {
			writeErr(w, 400, "konec party: mezi teď a +24 h")
			return
		}
	}
	if in.DJfyCode != nil {
		c := strings.ToUpper(strings.TrimSpace(*in.DJfyCode))
		if c != "" && !djfyCodeRE.MatchString(c) {
			writeErr(w, 400, "neplatný DJfy kód")
			return
		}
		in.DJfyCode = &c
	}
	if err := s.Store.UpdateParty(r.Context(), p.Code, in); err != nil {
		serverErr(w, r, err)
		return
	}
	if in.DJfyCode != nil {
		s.DJfy.Watch(p.Code, *in.DJfyCode)
		if *in.DJfyCode == "" {
			s.onDJfySnapshot(p.Code, nil)
		}
	}
	s.broadcastParty(r, p.Code)
	w.WriteHeader(204)
}

var djfyCodeRE = regexp.MustCompile(`^[A-Z0-9]{3,12}$`)

// ── guests ─────────────────────────────────────────────────────────────────

func (s *Server) createGuest(w http.ResponseWriter, r *http.Request, p store.Party) {
	if !s.partyOpen(w, p) || !s.allowIP(w, r, 2) {
		return
	}
	var in struct {
		Nickname string `json:"nickname"`
	}
	readJSON(r, &in)
	nick := clean(in.Nickname, 24)
	if nick == "" {
		writeErr(w, 400, "zadej přezdívku")
		return
	}
	if n, err := s.Store.GuestCount(r.Context(), p.Code); err != nil || n >= maxGuests {
		writeErr(w, 409, "party je plná")
		return
	}
	// two "Žofie" can't be told apart on the stage or in the recap
	if taken, _ := s.Store.NicknameTaken(r.Context(), p.Code, nick); taken {
		suggest := ""
		base := string([]rune(nick)[:min(len([]rune(nick)), 21)])
		for i := 2; i < 100 && suggest == ""; i++ {
			if t, _ := s.Store.NicknameTaken(r.Context(), p.Code, fmt.Sprintf("%s %d", base, i)); !t {
				suggest = fmt.Sprintf("%s %d", base, i)
			}
		}
		writeJSON(w, 409, map[string]string{"error": "přezdívku už někdo má", "suggest": suggest})
		return
	}
	g, err := s.Store.CreateGuest(r.Context(), p.Code, nick)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushCockpit(r.Context(), p)
	writeJSON(w, 201, g)
}

func (s *Server) setPlay(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		Wants bool `json:"wants"`
	}
	readJSON(r, &in)
	if err := s.Store.SetWantsToPlay(r.Context(), p.Code, g.ID, in.Wants); err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushCockpit(r.Context(), p)
	w.WriteHeader(204)
}

func (s *Server) guestAssets(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	as, err := s.Store.GuestAssets(r.Context(), p.Code, g.ID)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	for i, a := range as {
		var m map[string]any
		if json.Unmarshal(a.Meta, &m) == nil && m["error"] != nil {
			// provider errors name hosts and URLs; the operator sees them in the Cockpit
			m["error"] = "nepovedlo se, zkus to znovu"
			as[i].Meta, _ = json.Marshal(m)
		}
	}
	writeJSON(w, 200, map[string]any{"guest": g, "assets": as})
}

func (s *Server) uploadPhoto(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	ctx := r.Context()
	if busy, _ := s.Store.GuestBusy(ctx, p.Code, g.ID); busy {
		writeErr(w, 429, "počkej, tvoje předchozí fotka se ještě maluje")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 15<<20)
	if err := r.ParseMultipartForm(15 << 20); err != nil {
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
	medium := r.FormValue("medium")
	a, err := s.submitRestyle(ctx, p, g, raw, style, medium)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, a)
}

func (s *Server) createSticker(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		Text  string `json:"text"`
		Color string `json:"color"`
		Sheet string `json:"sheet"`
	}
	readJSON(r, &in)
	text := clean(in.Text, 140)
	if text == "" {
		writeErr(w, 400, "napiš vzkaz")
		return
	}
	ctx := r.Context()
	if n, err := s.Store.GuestPosts(ctx, p.Code, g.ID, "sticker", 0); err != nil || n >= maxNotesPerGuest {
		writeErr(w, 409, "už jsi napsal dost vzkazů")
		return
	}
	if n, _ := s.Store.GuestPosts(ctx, p.Code, g.ID, "sticker", s.Store.Now().Add(-time.Minute).UnixMilli()); n >= maxNotesPerMinute {
		writeErr(w, 429, "pomaleji, zeď nestíhá :)")
		return
	}
	if !colorRE.MatchString(in.Color) {
		in.Color = "#ffd84d"
	}
	m := map[string]string{"text": text, "nickname": g.Nickname, "color": in.Color}
	if s.Glyphs.Has(in.Sheet) {
		m["sheet"] = in.Sheet // handwriting the guest picked (D16)
	}
	meta, _ := json.Marshal(m)
	a, err := s.Store.CreateAsset(r.Context(), store.Asset{PartyCode: p.Code, GuestID: g.ID, Kind: "sticker", Meta: meta, Status: "pending"})
	if err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushCockpit(r.Context(), p)
	writeJSON(w, 201, a)
}

var colorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Server) createIdea(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		Text string `json:"text"`
	}
	readJSON(r, &in)
	text := clean(in.Text, 1000)
	if utf8.RuneCountInString(text) < 5 {
		writeErr(w, 400, "námět je moc krátký")
		return
	}
	if n, err := s.Store.GuestIdeas(r.Context(), p.Code, g.ID); err != nil || n >= maxIdeasPerGuest {
		writeErr(w, 409, "už jsi poslal dost námětů")
		return
	}
	i, err := s.Store.CreateIdea(r.Context(), p.Code, g, text)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushCockpit(r.Context(), p)
	writeJSON(w, 201, i)
}

var emojis = map[string]bool{"❤️": true, "🔥": true, "😂": true, "🎉": true, "👏": true, "😍": true, "🕺": true, "💃": true, "🍍": true, "🍌": true}

func (s *Server) react(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		Emoji string `json:"emoji"`
	}
	readJSON(r, &in)
	if !emojis[in.Emoji] {
		writeErr(w, 400, "neznámé emoji")
		return
	}
	s.mu.Lock()
	last := s.lastReact[g.ID]
	ok := time.Since(last) > 700*time.Millisecond
	if ok {
		s.lastReact[g.ID] = time.Now()
	}
	s.mu.Unlock()
	if !ok {
		writeErr(w, 429, "pomaleji :)")
		return
	}
	s.Hub.Broadcast(p.Code, hub.Msg{Type: "reaction", Data: map[string]string{"emoji": in.Emoji, "nickname": g.Nickname}}, hub.Stage, hub.Cockpit)
	w.WriteHeader(204)
}

// ── moderation + show queue ────────────────────────────────────────────────

func (s *Server) moderate(status string) partyHandler {
	return func(w http.ResponseWriter, r *http.Request, p store.Party) {
		ctx := r.Context()
		a, err := s.Store.Asset(ctx, p.Code, r.PathValue("id"))
		if err != nil {
			writeErr(w, 404, "asset nenalezen")
			return
		}
		if status == "approved" && a.Status != "pending" && a.Status != "rejected" {
			writeErr(w, 409, "schválit jde jen hotový obsah (stav "+a.Status+")")
			return
		}
		if err := s.Store.SetAssetStatus(ctx, p.Code, a.ID, status); err != nil {
			serverErr(w, r, err)
			return
		}
		if status == "approved" {
			s.onApproved(ctx, p, a.ID)
		}
		s.pushQueue(ctx, p.Code)
		s.pushCockpit(ctx, p)
		w.WriteHeader(204)
	}
}

func (s *Server) getQueue(w http.ResponseWriter, r *http.Request, p store.Party) {
	q, err := s.Store.StageQueue(r.Context(), p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	writeJSON(w, 200, publicQueue(q))
}

// publicQueue hides who made what: a guest's id is the key to their phone's
// endpoints (/guests/{gid}/…), and the queue goes to anyone with the party
// code. A one-way tag still tells one guest's items from another's.
func publicQueue(q []store.Asset) []store.Asset {
	out := make([]store.Asset, len(q))
	for i, a := range q {
		a.GuestID = guestTag(a.GuestID)
		out[i] = a
	}
	return out
}

func guestTag(id string) string {
	if id == "" {
		return ""
	}
	h := sha256.Sum256([]byte("partycockpit-guest:" + id))
	return "g" + hex.EncodeToString(h[:6])
}

func (s *Server) reorderQueue(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	if err := s.Store.ReorderQueue(r.Context(), p.Code, in.IDs); err != nil {
		serverErr(w, r, err)
		return
	}
	s.pushQueue(r.Context(), p.Code)
	w.WriteHeader(204)
}

func (s *Server) pushQueue(ctx context.Context, code string) {
	if q, err := s.Store.StageQueue(ctx, code); err == nil {
		s.Hub.Broadcast(code, hub.Msg{Type: "queue", Data: publicQueue(q)}, hub.Stage, hub.Cockpit)
	}
}

type cockpitState struct {
	Assets  []store.Asset    `json:"assets"`
	Guests  []store.Guest    `json:"guests"`
	Ideas   []store.Idea     `json:"ideas"`
	Jobs    []store.JobStats `json:"jobs"`
	Clients map[hub.Role]int `json:"clients"`
	Config  map[string]any   `json:"config"`
}

func (s *Server) cockpitPayload(ctx context.Context, p store.Party) (cockpitState, error) {
	var st cockpitState
	var err error
	if st.Assets, err = s.Store.Assets(ctx, p.Code); err != nil {
		return st, err
	}
	if st.Guests, err = s.Store.Guests(ctx, p.Code); err != nil {
		return st, err
	}
	if st.Ideas, err = s.Store.Ideas(ctx, p.Code); err != nil {
		return st, err
	}
	st.Jobs, _ = s.Store.JobStats(ctx)
	st.Clients = s.Hub.Counts(p.Code)
	st.Config = map[string]any{
		"restyle": s.Restyler.Name(), "animate": s.Animator.Name(), "moderation": s.Cfg.Moderation,
		"autoApproveMax": s.Cfg.AutoApproveMax, "djfyUrl": s.Cfg.DJfyURL, "subtitles": s.Subtitles != nil,
		"stageKey": p.StageKey, "fileKey": fileKey(p),
	}
	return st, nil
}

func (s *Server) cockpitState(w http.ResponseWriter, r *http.Request, p store.Party) {
	st, err := s.cockpitPayload(r.Context(), p)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	writeJSON(w, 200, st)
}

// pushCockpit sends the full cockpit state; parties are small (tens of guests).
func (s *Server) pushCockpit(ctx context.Context, p store.Party) {
	if st, err := s.cockpitPayload(ctx, p); err == nil {
		s.Hub.Broadcast(p.Code, hub.Msg{Type: "cockpit", Data: st}, hub.Cockpit)
	}
}

func (s *Server) listIdeas(w http.ResponseWriter, r *http.Request, p store.Party) {
	ideas, err := s.Store.Ideas(r.Context(), p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="namety-%s.json"`, p.Code))
	}
	writeJSON(w, 200, map[string]any{"party": p.Name, "code": p.Code, "ideas": ideas})
}

// ── scenes ─────────────────────────────────────────────────────────────────

var sceneKinds = map[string]bool{
	"slideshow": true, "gallery": true, "stickers": true, "nowplaying": true,
	"webtoon": true, "lottery": true, "blank": true, "pipwall": true, "dance": true,
	"interlude": true, "recap": true, "stylesweep": true, // M7
	// "game" is set only by POST /game, which knows the players
}

func (s *Server) setScene(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		Kind   string          `json:"kind"`
		Params json.RawMessage `json:"params"`
	}
	if err := readJSON(r, &in); err != nil || !sceneKinds[in.Kind] {
		writeErr(w, 400, "neznámá scéna")
		return
	}
	if _, err := s.Store.SetScene(r.Context(), p.Code, in.Kind, in.Params); err != nil {
		serverErr(w, r, err)
		return
	}
	s.broadcastParty(r, p.Code)
	w.WriteHeader(204)
}

// panic: back to the slideshow and clear overlays — the "NSFW on the TV" button.
func (s *Server) panic(w http.ResponseWriter, r *http.Request, p store.Party) {
	if _, err := s.Store.SetScene(r.Context(), p.Code, "slideshow", nil); err != nil {
		serverErr(w, r, err)
		return
	}
	s.unliveAll(p.Code)
	s.stopMic(p.Code)
	s.endGame(p.Code)
	s.Hub.Broadcast(p.Code, hub.Msg{Type: "panic", Data: nil})
	s.broadcastParty(r, p.Code)
	w.WriteHeader(204)
}

// lottery draws N players among guests who said "chci hrát" (D6); if nobody
// signed up, among everyone. The stage shows the draw.
func (s *Server) lottery(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		Count int    `json:"count"`
		Game  string `json:"game"`
	}
	readJSON(r, &in)
	in.Count = min(max(in.Count, 1), 8)
	guests, err := s.Store.Guests(r.Context(), p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	var pool []store.Guest
	for _, g := range guests {
		if g.WantsToPlay {
			pool = append(pool, g)
		}
	}
	if len(pool) == 0 {
		pool = guests
	}
	if len(pool) == 0 {
		writeErr(w, 409, "na party zatím nejsou žádní hosté")
		return
	}
	cands := make([]string, len(pool))
	for i, g := range pool {
		cands[i] = g.Nickname
	}
	for i := len(pool) - 1; i > 0; i-- { // Fisher–Yates with crypto/rand
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		pool[i], pool[j.Int64()] = pool[j.Int64()], pool[i]
	}
	// the scene goes to every screen and phone: nicknames only, never guest ids
	winners := []map[string]string{}
	var ids []string
	for _, g := range pool[:min(in.Count, len(pool))] {
		winners = append(winners, map[string]string{"nickname": g.Nickname})
		ids = append(ids, g.ID)
	}
	draw := map[string]any{"game": clean(in.Game, 40), "candidates": cands, "winners": winners, "at": time.Now().UnixMilli()}
	params, _ := json.Marshal(draw)
	if _, err := s.Store.SetScene(r.Context(), p.Code, "lottery", params); err != nil {
		serverErr(w, r, err)
		return
	}
	s.broadcastParty(r, p.Code)
	s.setDraw(p.Code, ids)
	for _, id := range ids {
		s.Hub.SendTo(p.Code, hub.Msg{Type: "lottery.won", Data: map[string]string{"game": clean(in.Game, 40)}}, isGuest(id))
	}
	writeJSON(w, 200, draw)
}

// ── gallery + webtoon ──────────────────────────────────────────────────────

func (s *Server) listGallery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	minScore, _ := strconv.Atoi(q.Get("score"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	items, err := s.Gallery.List(r.Context(), gallery.Filter{Style: q.Get("style"), Lora: q.Get("lora"), MinScore: minScore, Limit: limit})
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, items)
}

// Webtoon episodes are prepared folders in $PC_DATA_DIR/webtoon/<episode>/ with
// panel images named in reading order (D7); optional title.txt.
func (s *Server) webtoonDir() string { return filepath.Join(s.Cfg.DataDir, "webtoon") }

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,80}$`)

func (s *Server) listWebtoon(w http.ResponseWriter, r *http.Request) {
	type ep struct {
		ID     string   `json:"id"`
		Title  string   `json:"title"`
		Panels []string `json:"panels"`
	}
	out := []ep{}
	dirs, _ := os.ReadDir(s.webtoonDir())
	for _, d := range dirs {
		if !d.IsDir() || !safeName.MatchString(d.Name()) {
			continue
		}
		e := ep{ID: d.Name(), Title: d.Name()}
		if t, err := os.ReadFile(filepath.Join(s.webtoonDir(), d.Name(), "title.txt")); err == nil {
			e.Title = strings.TrimSpace(string(t))
		}
		files, _ := os.ReadDir(filepath.Join(s.webtoonDir(), d.Name()))
		for _, f := range files {
			n := strings.ToLower(f.Name())
			if safeName.MatchString(f.Name()) && (strings.HasSuffix(n, ".png") || strings.HasSuffix(n, ".jpg") || strings.HasSuffix(n, ".jpeg") || strings.HasSuffix(n, ".webp")) {
				e.Panels = append(e.Panels, "/webtoon/"+d.Name()+"/"+f.Name())
			}
		}
		sort.Strings(e.Panels)
		if len(e.Panels) > 0 {
			out = append(out, e)
		}
	}
	writeJSON(w, 200, out)
}

func (s *Server) serveWebtoon(w http.ResponseWriter, r *http.Request) {
	ep, f := r.PathValue("ep"), r.PathValue("file")
	if !safeName.MatchString(ep) || !safeName.MatchString(f) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.webtoonDir(), ep, f))
}

// ── metrics ────────────────────────────────────────────────────────────────

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	stats, _ := s.Store.JobStats(r.Context())
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# TYPE partycockpit_jobs_total counter")
	for _, j := range stats {
		fmt.Fprintf(w, "partycockpit_jobs_total{intent=%q,status=%q} %d\n", j.Intent, j.Status, j.Count)
	}
	fmt.Fprintln(w, "# TYPE partycockpit_job_duration_ms_avg gauge")
	for _, j := range stats {
		if j.Status == "done" {
			fmt.Fprintf(w, "partycockpit_job_duration_ms_avg{intent=%q} %.0f\n", j.Intent, j.AvgMs)
		}
	}
}

// clean trims, collapses whitespace/control chars and caps length in runes.
func clean(s string, n int) string {
	// format characters (zero-width, bidi overrides) can hide or flip text on the stage
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r < 32 || r == ' ' || r == ' ' }), " ")
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return strings.TrimSpace(s)
}
