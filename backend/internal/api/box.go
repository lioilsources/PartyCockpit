package api

// Party box mode (D20): the stage runs on the box itself (Chromium kiosk on
// 127.0.0.1) and everything else is a phone. The box opens its stage without
// a keyboard (/box hands the stage key to loopback only), creates a party at
// boot, and shows a pairing QR so the organiser's phone becomes the Cockpit.

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

// pairWindow: how long the pairing QR stays on the TV without a Cockpit.
const pairWindow = 10 * time.Minute

// BoxInfo goes to the stage with the party: defaults tuned for the box.
type BoxInfo struct {
	Scale float64 `json:"scale"` // shader render scale
}

func (s *Server) maxLive() int {
	if s.Cfg.MaxLive > 0 {
		return min(s.Cfg.MaxLive, maxLiveStreams)
	}
	return maxLiveStreams
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// fromLoopback: the request comes from the box itself (the kiosk), never
// through a proxy header.
func fromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// lanIPs lists the box's private IPv4 addresses, the wired ones first.
func lanIPs() []string {
	ifs, _ := net.Interfaces()
	var wired, other []string
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || !ipn.IP.IsPrivate() {
				continue
			}
			if strings.HasPrefix(ifc.Name, "eth") || strings.HasPrefix(ifc.Name, "en") {
				wired = append(wired, ipn.IP.String())
			} else {
				other = append(other, ipn.IP.String())
			}
		}
	}
	return append(wired, other...)
}

// boxBaseURL is what phones should open when PC_PUBLIC_URL is not set.
func (s *Server) boxBaseURL() string {
	if s.Cfg.PublicURL != "" {
		return s.Cfg.PublicURL
	}
	ips := lanIPs()
	if len(ips) == 0 {
		return ""
	}
	port := "8098"
	if _, p, err := net.SplitHostPort(s.Cfg.Addr); err == nil && p != "" {
		port = p
	}
	return "http://" + net.JoinHostPort(ips[0], port)
}

// openParty is the newest party that has not ended.
func (s *Server) openParty(ctx context.Context) (store.Party, bool) {
	ps, err := s.Store.Parties(ctx) // newest first
	if err != nil {
		return store.Party{}, false
	}
	now := s.Store.Now().UnixMilli()
	for _, p := range ps {
		if p.EndsAt > now {
			return p, true
		}
	}
	return store.Party{}, false
}

// boxStart runs at boot in box mode: a party to show, and the pairing QR on.
func (s *Server) boxStart(ctx context.Context) {
	if !s.Cfg.Box {
		return
	}
	p, ok := s.openParty(ctx)
	if !ok && s.Cfg.BoxAutoParty {
		var err error
		if p, err = s.Store.CreateParty(ctx, s.Cfg.BoxName, s.Cfg.PartyTTL); err != nil {
			slog.Error("box: create party", "err", err)
			return
		}
		ok = true
		slog.Info("box: party created", "party", p.Code)
	}
	if ok {
		s.startPairing(p.Code)
	}
}

func (s *Server) startPairing(code string) {
	s.pmu.Lock()
	s.pairUntil[code] = s.Store.Now().Add(pairWindow)
	s.pmu.Unlock()
}

// pairingMsg: the pairing QR for the box's own stage, or null when it is off.
func (s *Server) pairingMsg(ctx context.Context, code string) hub.Msg {
	s.pmu.Lock()
	until, on := s.pairUntil[code]
	s.pmu.Unlock()
	if !on || s.Store.Now().After(until) {
		return hub.Msg{Type: "pairing", Data: nil}
	}
	p, err := s.Store.Party(ctx, code)
	base := s.boxBaseURL()
	if err != nil || base == "" {
		return hub.Msg{Type: "pairing", Data: nil}
	}
	return hub.Msg{Type: "pairing", Data: map[string]any{
		"url":   base + "/cockpit/" + code + "#token=" + p.CockpitToken,
		"until": until.UnixMilli(),
	}}
}

func isBoxStage(p hub.Peer) bool { return p.Role == hub.Stage && isLoopbackHost(p.IP) }

// pushPairing tells the box's stage to show or hide the pairing QR.
func (s *Server) pushPairing(ctx context.Context, code string) {
	s.Hub.SendTo(code, s.pairingMsg(ctx, code), isBoxStage)
}

// endPairing: a Cockpit connected, the QR leaves the TV.
func (s *Server) endPairing(ctx context.Context, code string) {
	s.pmu.Lock()
	_, on := s.pairUntil[code]
	delete(s.pairUntil, code)
	s.pmu.Unlock()
	if on {
		s.pushPairing(ctx, code)
	}
}

// pair (cockpit): show the pairing QR again, e.g. for a second operator.
func (s *Server) pair(w http.ResponseWriter, r *http.Request, p store.Party) {
	if !s.Cfg.Box {
		writeErr(w, 409, "párování je jen na party boxu")
		return
	}
	s.startPairing(p.Code)
	s.pushPairing(r.Context(), p.Code)
	w.WriteHeader(204)
}

// box (loopback only): the kiosk's start page. With a party it goes straight
// to the stage, key included; otherwise the SPA shows the box screen.
func (s *Server) box(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.Box || !fromLoopback(r) {
		http.NotFound(w, r)
		return
	}
	if p, ok := s.openParty(r.Context()); ok {
		u := "/stage/" + p.Code + "?key=" + p.StageKey
		if s.Cfg.BoxAudio {
			u += "&audio=1"
		}
		http.Redirect(w, r, u, http.StatusFound)
		return
	}
	s.spa(w, r)
}

// boxInfo (loopback only): what the box screen shows before there is a party.
func (s *Server) boxInfo(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.Box || !fromLoopback(r) {
		http.NotFound(w, r)
		return
	}
	host, _ := os.Hostname()
	out := map[string]any{
		"hostname": host, "ips": lanIPs(), "url": s.boxBaseURL(),
		"version": s.Version, "autoParty": s.Cfg.BoxAutoParty,
	}
	if na, err := certNotAfter(s.Cfg.TLSCert); err == nil {
		out["certUntil"] = na.UnixMilli()
	}
	writeJSON(w, 200, out)
}

// certNotAfter reads the expiry of the first certificate in a PEM file.
func certNotAfter(path string) (time.Time, error) {
	if path == "" {
		return time.Time{}, os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return time.Time{}, os.ErrInvalid
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return c.NotAfter, nil
}

// cpuTemp in °C from the kernel's first thermal zone (Pi, most Linux boxes).
func cpuTemp() (float64, bool) {
	b, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, false
	}
	m, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return float64(m) / 1000, true
}
