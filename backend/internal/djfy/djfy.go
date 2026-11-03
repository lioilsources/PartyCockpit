// Package djfy connects a party to a DJfy relay session (services/djfy in the
// DJfy repo). The relay is not a music server: the host phone owns queue,
// playback, search and BPM; the relay forwards guest messages to it and fans its
// snapshot out over SSE. PartyCockpit joins as a guest:
//
//   - GET  /api/sessions/{code}/events?guest=ID   SSE: snapshot | private | banned | ended
//   - POST /api/sessions/{code}/messages          {type: join|search|propose|vote, guestId, name, ...}
//
// Each PartyCockpit guest talks to DJfy under its own guestId (pc + 16 hex, fits
// the relay's 8–40 [A-Za-z0-9_-] rule), so DJfy's per-guest rate limits and
// "max 3 queued per guest" apply per person, not to the whole party.
package djfy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrNoSession = errors.New("party has no DJfy session")

type Manager struct {
	Base       string
	OnSnapshot func(party string, snap json.RawMessage)

	http   *http.Client // no timeout: SSE streams are long-lived
	mu     sync.Mutex
	watch  map[string]*watcher // party → watcher
	joined map[string]bool     // djfyCode|guestId → join sent
}

type watcher struct {
	code   string // DJfy session code
	cancel context.CancelFunc
	snap   json.RawMessage
}

func New(base string) *Manager {
	return &Manager{Base: base, http: &http.Client{}, watch: map[string]*watcher{}, joined: map[string]bool{}}
}

// Watch (re)subscribes party to DJfy session code; empty code stops watching.
func (m *Manager) Watch(party, code string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.watch[party]; w != nil {
		if w.code == code {
			return
		}
		w.cancel()
		delete(m.watch, party)
	}
	if code == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &watcher{code: code, cancel: cancel}
	m.watch[party] = w
	go m.run(ctx, party, w)
}

func (m *Manager) Snapshot(party string) json.RawMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.watch[party]; w != nil {
		return w.snap
	}
	return nil
}

func (m *Manager) session(party string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.watch[party]; w != nil {
		return w.code, nil
	}
	return "", ErrNoSession
}

func (m *Manager) run(ctx context.Context, party string, w *watcher) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := m.stream(ctx, w.code, "pcstage"+strings.ToLower(party), func(ev, data string) bool {
			switch ev {
			case "snapshot":
				raw := json.RawMessage(data)
				m.mu.Lock()
				w.snap = raw
				m.mu.Unlock()
				if m.OnSnapshot != nil {
					m.OnSnapshot(party, raw)
				}
			case "ended":
				m.mu.Lock()
				w.snap = nil
				m.mu.Unlock()
				if m.OnSnapshot != nil {
					m.OnSnapshot(party, nil)
				}
			}
			return true
		})
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		slog.Warn("djfy stream dropped", "party", party, "session", w.code, "err", err, "retry", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

// stream reads SSE events until fn returns false, the context ends or the line drops.
// connected (optional) is closed once the response headers arrived.
func (m *Manager) streamWith(ctx context.Context, code, guest string, connected chan<- struct{}, fn func(ev, data string) bool) error {
	u := fmt.Sprintf("%s/api/sessions/%s/events?guest=%s", m.Base, url.PathEscape(code), url.QueryEscape(guest))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("events: %s", resp.Status)
	}
	if connected != nil {
		close(connected)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20) // snapshots are up to 256 KB
	var ev string
	var data strings.Builder
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if data.Len() > 0 || ev != "" {
				if !fn(ev, strings.TrimSuffix(data.String(), "\n")) {
					return nil
				}
			}
			ev = ""
			data.Reset()
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			ev = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			data.WriteString(strings.TrimPrefix(line[5:], " "))
			data.WriteByte('\n')
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("stream closed")
}

func (m *Manager) stream(ctx context.Context, code, guest string, fn func(ev, data string) bool) error {
	return m.streamWith(ctx, code, guest, nil, fn)
}

func (m *Manager) send(ctx context.Context, code string, msg map[string]any) (bool, error) {
	b, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/sessions/%s/messages", m.Base, url.PathEscape(code)), bytes.NewReader(b))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var out struct {
		HostOnline bool   `json:"hostOnline"`
		Error      string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	switch resp.StatusCode {
	case 202, 200:
		return out.HostOnline, nil
	case 429:
		return false, errors.New("příliš mnoho požadavků, zkus to za chvíli")
	case 403:
		return false, errors.New("DJ tě zablokoval")
	default:
		return false, fmt.Errorf("djfy %d %s", resp.StatusCode, out.Error)
	}
}

// Who identifies a PartyCockpit guest towards DJfy.
type Who struct{ GuestID, Name string }

func (w Who) base(kind string) map[string]any {
	name := w.Name
	if r := []rune(name); len(r) > 32 {
		name = string(r[:32])
	}
	return map[string]any{"type": kind, "guestId": w.GuestID, "name": name}
}

func (m *Manager) ensureJoined(ctx context.Context, code string, who Who) {
	key := code + "|" + who.GuestID
	m.mu.Lock()
	done := m.joined[key]
	m.joined[key] = true
	m.mu.Unlock()
	if !done {
		m.send(ctx, code, who.base("join"))
	}
}

// exchange opens the guest's private SSE channel, sends msg and waits for the
// first private reply that match accepts (or the timeout).
func (m *Manager) exchange(ctx context.Context, party string, who Who, msg map[string]any, wait time.Duration, match func(json.RawMessage) bool) (json.RawMessage, bool, error) {
	code, err := m.session(party)
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	connected := make(chan struct{})
	reply := make(chan json.RawMessage, 1)
	go m.streamWith(ctx, code, who.GuestID, connected, func(ev, data string) bool {
		if ev == "private" && match(json.RawMessage(data)) {
			reply <- json.RawMessage(data)
			return false
		}
		return true
	})
	select {
	case <-connected:
	case <-ctx.Done():
		return nil, false, errors.New("DJfy není dostupné")
	}
	m.ensureJoined(ctx, code, who)
	online, err := m.send(ctx, code, msg)
	if err != nil {
		return nil, online, err
	}
	if !online {
		return nil, false, errors.New("DJ (telefon s DJfy) je offline")
	}
	select {
	case r := <-reply:
		return r, true, nil
	case <-ctx.Done():
		return nil, true, nil
	}
}

func typeIs(t string, extra func(map[string]any) bool) func(json.RawMessage) bool {
	return func(raw json.RawMessage) bool {
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil || m["type"] != t {
			return false
		}
		return extra == nil || extra(m)
	}
}

// Search asks the host phone to search SoundCloud; results arrive privately.
func (m *Manager) Search(ctx context.Context, party string, who Who, q string) (json.RawMessage, error) {
	reqID := fmt.Sprintf("pc%d", time.Now().UnixNano()%1e9)
	msg := who.base("search")
	msg["q"], msg["reqId"] = q, reqID
	r, _, err := m.exchange(ctx, party, who, msg, 12*time.Second, typeIs("searchResults", func(m map[string]any) bool { return m["reqId"] == reqID }))
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("DJ neodpověděl včas")
	}
	var out struct {
		Results json.RawMessage `json:"results"`
	}
	json.Unmarshal(r, &out)
	return out.Results, nil
}

// Propose adds a track (by SoundCloud id from a previous search); the host may
// answer with a toast, which is returned as text.
func (m *Manager) Propose(ctx context.Context, party string, who Who, trackID string) (string, error) {
	msg := who.base("propose")
	msg["trackId"] = trackID
	return m.toast(ctx, party, who, msg)
}

func (m *Manager) Vote(ctx context.Context, party string, who Who, entryID string, value int) (string, error) {
	if value != 1 && value != -1 {
		return "", errors.New("value must be ±1")
	}
	msg := who.base("vote")
	msg["entryId"], msg["value"] = entryID, value
	return m.toast(ctx, party, who, msg)
}

func (m *Manager) toast(ctx context.Context, party string, who Who, msg map[string]any) (string, error) {
	r, _, err := m.exchange(ctx, party, who, msg, 3*time.Second, typeIs("toast", nil))
	if err != nil || r == nil {
		return "", err
	}
	var t struct {
		Text string `json:"text"`
	}
	json.Unmarshal(r, &t)
	return t.Text, nil
}
