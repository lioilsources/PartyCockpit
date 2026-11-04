package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
	"github.com/lioilsources/PartyCockpit/backend/internal/djfy"
	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

type env struct {
	t     *testing.T
	ts    *httptest.Server
	code  string
	token string
	key   string // stage key
	srv   *Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cat, err := providers.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: dir, PartyTTL: time.Hour, Workers: 1, ICEServers: json.RawMessage("[]")}
	s := &Server{
		Cfg: cfg, Store: st, Hub: hub.New(),
		Gallery: gallery.New("http://127.0.0.1:1", filepath.Join(dir, "gc"), false, nil),
		DJfy:    djfy.New("http://127.0.0.1:1"), Catalog: cat,
		Restyler: &providers.Mock{}, Animator: providers.MockAnimator{}, Classifier: providers.NoClassifier{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Routes())
	t.Cleanup(ts.Close)
	e := &env{t: t, ts: ts, srv: s}
	var p struct {
		Party struct {
			Code string `json:"code"`
		} `json:"party"`
		CockpitToken string `json:"cockpitToken"`
		StageKey     string `json:"stageKey"`
	}
	e.do("POST", "/api/parties", map[string]any{"name": "test"}, &p)
	e.code, e.token, e.key = p.Party.Code, p.CockpitToken, p.StageKey
	return e
}

func (e *env) do(method, path string, body, out any) int {
	e.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, e.ts.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

type wsMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// sock reads in the background: coder/websocket closes the connection when a
// read's context expires, so timeouts are taken on the channel instead.
type sock struct {
	t  *testing.T
	c  *websocket.Conn
	in chan wsMsg
}

func (e *env) dial(q string) *sock {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(e.ts.URL, "http") + "/ws/parties/" + e.code + "?" + q
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		e.t.Fatalf("dial %s: %v", q, err)
	}
	e.t.Cleanup(func() { c.CloseNow() })
	s := &sock{t: e.t, c: c, in: make(chan wsMsg, 64)}
	go func() {
		defer close(s.in)
		for {
			_, b, err := c.Read(context.Background())
			if err != nil {
				return
			}
			var m wsMsg
			json.Unmarshal(b, &m)
			s.in <- m
		}
	}()
	return s
}

func (s *sock) send(v any) {
	b, _ := json.Marshal(v)
	if err := s.c.Write(context.Background(), websocket.MessageText, b); err != nil {
		s.t.Fatal(err)
	}
}

// waitFor reads until a message of type typ satisfying ok arrives.
func (s *sock) waitFor(typ string, ok func(json.RawMessage) bool) json.RawMessage {
	s.t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case m, open := <-s.in:
			if !open {
				s.t.Fatalf("socket closed while waiting for %s", typ)
			}
			if m.Type == typ && (ok == nil || ok(m.Data)) {
				return m.Data
			}
		case <-timeout:
			s.t.Fatalf("timeout waiting for %s", typ)
		}
	}
}

// none asserts that no message of type typ arrives within d.
func (s *sock) none(typ string, d time.Duration) {
	s.t.Helper()
	timeout := time.After(d)
	for {
		select {
		case m := <-s.in:
			if m.Type == typ {
				s.t.Fatalf("unexpected %s: %s", typ, m.Data)
			}
		case <-timeout:
			return
		}
	}
}

func TestStreamSignalling(t *testing.T) {
	e := newEnv(t)
	var g struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Ufon"}, &g)
	var start struct {
		Stream Stream `json:"stream"`
		Key    string `json:"key"`
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/guests/"+g.ID+"/stream", map[string]any{"source": "mirrorbooth"}, &start); st != 201 {
		t.Fatalf("start stream: %d", st)
	}
	sid := start.Stream.ID

	if e.dialStatus("role=streamer&key=wrong") != 401 {
		t.Fatal("a wrong stream key must be refused")
	}
	phone := e.dial("role=streamer&key=" + start.Key)
	phone.waitFor("stream", nil)

	stage := e.dial("role=stage&key=" + e.key)
	stage.waitFor("streams", nil)

	// pending: the stage's offer must not reach the phone
	offer := map[string]any{"type": "rtc", "data": map[string]any{"kind": "offer", "stream": sid, "sdp": "v=0 stage"}}
	stage.send(offer)
	phone.none("rtc", 300*time.Millisecond)

	if st := e.do("POST", "/api/parties/"+e.code+"/streams/"+sid+"/approve", nil, nil); st != 200 {
		t.Fatalf("approve: %d", st)
	}
	stage.waitFor("streams", func(d json.RawMessage) bool { return strings.Contains(string(d), sid) })

	stage.send(offer)
	got := phone.waitFor("rtc", nil)
	var m rtcMsg
	json.Unmarshal(got, &m)
	if m.Kind != "offer" || m.SDP != "v=0 stage" || m.From == "" {
		t.Fatalf("phone got %s", got)
	}

	phone.send(map[string]any{"type": "rtc", "data": map[string]any{"kind": "answer", "to": m.From, "sdp": "v=0 phone"}})
	ans := stage.waitFor("rtc", nil)
	var a rtcMsg
	json.Unmarshal(ans, &a)
	if a.Kind != "answer" || a.Stream != sid || a.SDP != "v=0 phone" {
		t.Fatalf("stage got %s", ans)
	}

	// PANIC takes every stream off the stage
	e.do("POST", "/api/parties/"+e.code+"/panic", nil, nil)
	stage.waitFor("streams", func(d json.RawMessage) bool { return string(d) == "[]" })

	// the operator ends it: the phone is told
	e.do("DELETE", "/api/parties/"+e.code+"/streams/"+sid, nil, nil)
	phone.waitFor("stream", func(d json.RawMessage) bool { return strings.Contains(string(d), `"ended"`) })
}

func (e *env) dialStatus(q string) int {
	url := "ws" + strings.TrimPrefix(e.ts.URL, "http") + "/ws/parties/" + e.code + "?" + q
	c, res, err := websocket.Dial(context.Background(), url, nil)
	if err == nil {
		c.CloseNow()
		return 101
	}
	if res == nil {
		return 0
	}
	return res.StatusCode
}
