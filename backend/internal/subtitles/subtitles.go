// Package subtitles is a client for the Translator server (../Translator,
// docs/protocol.md v1): the MC's microphone goes in as PCM16 16 kHz mono,
// transcripts and translations come out. PartyCockpit uses streaming mode
// without TTS and always forces side "a" (the MC), so the translation direction
// is fixed: langA → langB.
package subtitles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Line is one subtitle update. The same ID is updated several times: partial
// transcripts, then the final one, then the translation.
type Line struct {
	ID          int    `json:"id"`
	Text        string `json:"text"`
	Lang        string `json:"lang,omitempty"`
	Translation string `json:"translation,omitempty"`
	ToLang      string `json:"toLang,omitempty"`
	Final       bool   `json:"final"` // transcript is final
	Done        bool   `json:"done"`  // nothing more will come for this ID
}

type Client struct {
	URL   string // Translator base URL, http(s)://host[:port]
	Token string
}

// Health checks GET /healthz (it also checks ASR and MT behind the server).
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.URL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 4 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("healthz %d (ASR běží na SPARKu jen přes broker)", resp.StatusCode)
	}
	return nil
}

// Session is one open Translator session.
type Session struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	lines  map[int]*Line
	partID int // id shown for partials before the server assigns one
	langA  string
	langB  string
	onLine func(Line)
	done   chan struct{}
}

// Open dials the Translator and starts a session; onLine is called from the
// read loop for every update.
func (c *Client) Open(ctx context.Context, langA, langB string, onLine func(Line)) (*Session, error) {
	url := strings.Replace(strings.Replace(c.URL, "https://", "wss://", 1), "http://", "ws://", 1) + "/v1/ws"
	hdr := http.Header{}
	if c.Token != "" {
		hdr.Set("Authorization", "Bearer "+c.Token)
	}
	dctx, cancelDial := context.WithTimeout(ctx, 8*time.Second)
	defer cancelDial()
	conn, _, err := websocket.Dial(dctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		return nil, fmt.Errorf("translator: %w", err)
	}
	conn.SetReadLimit(1 << 20)
	sctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s := &Session{conn: conn, ctx: sctx, cancel: cancel, lines: map[int]*Line{}, langA: langA, langB: langB, onLine: onLine, done: make(chan struct{}), partID: -1}
	start, _ := json.Marshal(map[string]any{
		"type": "session.start", "deviceId": "partycockpit", "langA": langA, "langB": langB,
		"mode": "streaming", "tts": false,
	})
	if err := conn.Write(dctx, websocket.MessageText, start); err != nil {
		cancel()
		conn.CloseNow()
		return nil, fmt.Errorf("translator start: %w", err)
	}
	// first message must be session.ready (or an error)
	_, raw, err := conn.Read(dctx)
	if err != nil {
		cancel()
		conn.CloseNow()
		return nil, fmt.Errorf("translator ready: %w", err)
	}
	var m msg
	json.Unmarshal(raw, &m)
	if m.Type != "session.ready" {
		cancel()
		conn.CloseNow()
		return nil, fmt.Errorf("translator: %s %s", m.Code, m.Message)
	}
	go s.read()
	return s, nil
}

type msg struct {
	Type        string `json:"type"`
	UtteranceID int    `json:"utteranceId"`
	Lang        string `json:"lang"`
	Text        string `json:"text"`
	Status      string `json:"status"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

func (s *Session) read() {
	defer close(s.done)
	for {
		typ, raw, err := s.conn.Read(s.ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue // TTS audio; we asked for none
		}
		var m msg
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		s.handle(m)
	}
}

func (s *Session) handle(m msg) {
	s.mu.Lock()
	var out *Line
	switch m.Type {
	case "asr.partial":
		// partials carry no utterance id: they belong to the one in progress
		l := s.lines[s.partID]
		if l == nil {
			l = &Line{ID: s.partID, Lang: s.langA}
			s.lines[s.partID] = l
		}
		l.Text = m.Text
		out = l
	case "asr.final":
		l := s.lines[s.partID]
		delete(s.lines, s.partID)
		if l == nil {
			l = &Line{}
		}
		l.ID, l.Text, l.Lang, l.Final = m.UtteranceID, m.Text, m.Lang, true
		s.lines[m.UtteranceID] = l
		s.partID--
		out = l
	case "mt.partial", "mt.final":
		if l := s.lines[m.UtteranceID]; l != nil && s.langB != s.langA {
			l.Translation, l.ToLang = m.Text, m.Lang
			out = l
		}
	case "utterance.end":
		if l := s.lines[m.UtteranceID]; l != nil {
			delete(s.lines, m.UtteranceID)
			if m.Status == "ok" || l.Text != "" {
				l.Done = true
				out = l
			}
		}
	}
	var cp Line
	if out != nil {
		cp = *out
	}
	s.mu.Unlock()
	if out != nil && strings.TrimSpace(cp.Text) != "" && s.onLine != nil {
		s.onLine(cp)
	}
}

// Audio appends PCM16LE 16 kHz mono to the utterance in progress.
func (s *Session) Audio(pcm []byte) error {
	return s.conn.Write(s.ctx, websocket.MessageBinary, pcm)
}

// End closes the utterance (the browser's VAD decided the MC stopped talking).
func (s *Session) End() error {
	return s.conn.Write(s.ctx, websocket.MessageText, []byte(`{"type":"audio.end","side":"a"}`))
}

// Close ends the session; safe to call more than once.
func (s *Session) Close() {
	_ = s.conn.Write(s.ctx, websocket.MessageText, []byte(`{"type":"session.end"}`))
	s.conn.Close(websocket.StatusNormalClosure, "bye")
	s.cancel()
}

// Done is closed when the Translator connection is gone.
func (s *Session) Done() <-chan struct{} { return s.done }

var ErrNotConfigured = errors.New("TRANSLATOR_URL není nastavené")
