// Package hub fans party events out to WebSocket clients (stage, cockpit, guest).
package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type Role string

const (
	Stage   Role = "stage"
	Cockpit Role = "cockpit"
	Guest   Role = "guest"
	// Streamer is a phone publishing live video (browser guest or MirrorBooth),
	// authenticated by its stream key — see api/streams.go.
	Streamer Role = "streamer"
)

// Peer identifies one WebSocket connection.
type Peer struct {
	ID       string // random per connection; viewers are addressed by it
	Role     Role
	StreamID string // Streamer only
	GuestID  string // Guest only, when the phone said who it is (?guest=, verified)
	IP       string // remote address, for the per-IP connection limit
}

// Handlers receive what clients send. Both may be nil.
type Handlers struct {
	OnMsg   func(code string, from Peer, raw []byte)
	OnLeave func(code string, p Peer)
}

// Msg is the wire envelope: {"type": "...", "data": ...}.
type Msg struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type client struct {
	Peer
	send  chan []byte
	close context.CancelFunc
}

type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*client]struct{}
}

func New() *Hub { return &Hub{rooms: map[string]map[*client]struct{}{}} }

// Broadcast sends msg to every client of the party whose role is in roles
// (no roles = everyone). Slow clients drop messages instead of blocking.
func (h *Hub) Broadcast(code string, msg Msg, roles ...Role) {
	b, err := json.Marshal(msg)
	if err != nil {
		slog.Error("hub marshal", "type", msg.Type, "err", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[code] {
		if len(roles) > 0 && !has(roles, c.Role) {
			continue
		}
		select {
		case c.send <- b:
		default:
		}
	}
}

func has(roles []Role, r Role) bool {
	for _, x := range roles {
		if x == r {
			return true
		}
	}
	return false
}

// SendTo delivers msg to the clients of the party for which match is true and
// returns how many matched.
func (h *Hub) SendTo(code string, msg Msg, match func(Peer) bool) int {
	b, err := json.Marshal(msg)
	if err != nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.rooms[code] {
		if !match(c.Peer) {
			continue
		}
		n++
		select {
		case c.send <- b:
		default:
		}
	}
	return n
}

// Find returns the connected peers of the party for which match is true.
func (h *Hub) Find(code string, match func(Peer) bool) []Peer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []Peer
	for c := range h.rooms[code] {
		if match(c.Peer) {
			out = append(out, c.Peer)
		}
	}
	return out
}

// Count returns how many connected clients of the party match.
func (h *Hub) Count(code string, match func(Peer) bool) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.rooms[code] {
		if match(c.Peer) {
			n++
		}
	}
	return n
}

// Close disconnects the matching clients (a rotated cockpit token, D19).
func (h *Hub) Close(code string, match func(Peer) bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[code] {
		if match(c.Peer) && c.close != nil {
			c.close()
		}
	}
}

// Counts returns connected clients per role.
func (h *Hub) Counts(code string) map[Role]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := map[Role]int{}
	for c := range h.rooms[code] {
		out[c.Role]++
	}
	return out
}

// Serve runs one WebSocket connection until it closes. hello is sent first
// (the full state), so a reconnecting stage is consistent immediately.
// Client messages (WebRTC signalling) go to hs.OnMsg, at most 40 per second.
func (h *Hub) Serve(ctx context.Context, conn *websocket.Conn, code string, peer Peer, hello []Msg, hs Handlers) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c := &client{Peer: peer, send: make(chan []byte, 64), close: cancel}
	h.mu.Lock()
	if h.rooms[code] == nil {
		h.rooms[code] = map[*client]struct{}{}
	}
	h.rooms[code][c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.rooms[code], c)
		if len(h.rooms[code]) == 0 {
			delete(h.rooms, code)
		}
		h.mu.Unlock()
		if hs.OnLeave != nil {
			hs.OnLeave(code, peer)
		}
	}()

	conn.SetReadLimit(64 << 10) // an SDP is a few kB
	go func() {
		defer cancel()
		window, n := time.Now(), 0
		for {
			typ, b, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ != websocket.MessageText || hs.OnMsg == nil {
				continue
			}
			if time.Since(window) > time.Second {
				window, n = time.Now(), 0
			}
			if n++; n > 40 {
				continue
			}
			hs.OnMsg(code, peer, b)
		}
	}()

	for _, m := range hello {
		b, _ := json.Marshal(m)
		if err := write(ctx, conn, b); err != nil {
			return
		}
	}
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case b := <-c.send:
			if err := write(ctx, conn, b); err != nil {
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func write(ctx context.Context, conn *websocket.Conn, b []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, b)
}
