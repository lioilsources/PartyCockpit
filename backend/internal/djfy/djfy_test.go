package djfy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRelay mimics services/djfy: SSE per guest, the host answers searches
// with a private searchResults and proposals with a toast.
type fakeRelay struct {
	mu      sync.Mutex
	streams map[string]chan string // guestId → events
	got     []map[string]any
}

func (f *fakeRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/events"):
		g := r.URL.Query().Get("guest")
		ch := make(chan string, 8)
		f.mu.Lock()
		f.streams[g] = ch
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: snapshot\ndata: {\"v\":1,\"isPlaying\":true,\"nowPlaying\":{\"id\":\"1\",\"bpm\":128}}\n\n")
		w.(http.Flusher).Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case ev := <-ch:
				fmt.Fprint(w, ev)
				w.(http.Flusher).Flush()
			}
		}
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/messages"):
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		f.mu.Lock()
		f.got = append(f.got, m)
		ch := f.streams[m["guestId"].(string)]
		f.mu.Unlock()
		switch m["type"] {
		case "search":
			ch <- fmt.Sprintf("event: private\ndata: {\"type\":\"searchResults\",\"reqId\":%q,\"results\":[{\"id\":\"42\",\"title\":\"Song\",\"artist\":\"A\",\"source\":\"soundcloud\"}]}\n\n", m["reqId"])
		case "propose":
			ch <- "event: private\ndata: {\"type\":\"toast\",\"text\":\"Přidáno do fronty\"}\n\n"
		}
		w.WriteHeader(202)
		fmt.Fprint(w, `{"hostOnline":true}`)
	default:
		http.NotFound(w, r)
	}
}

func TestSnapshotSearchPropose(t *testing.T) {
	relay := &fakeRelay{streams: map[string]chan string{}}
	srv := httptest.NewServer(relay)
	defer srv.Close()

	m := New(srv.URL)
	snaps := make(chan json.RawMessage, 4)
	m.OnSnapshot = func(party string, s json.RawMessage) { snaps <- s }
	m.Watch("PARTY", "abc12")
	defer m.Watch("PARTY", "")

	select {
	case s := <-snaps:
		if !strings.Contains(string(s), `"bpm":128`) {
			t.Fatalf("snapshot %s", s)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no snapshot")
	}

	ctx := context.Background()
	who := Who{GuestID: "pc0123456789abcdef", Name: "Olin"}
	res, err := m.Search(ctx, "PARTY", who, "daft punk")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res), `"id":"42"`) {
		t.Fatalf("results %s", res)
	}
	toast, err := m.Propose(ctx, "PARTY", who, "42")
	if err != nil || toast != "Přidáno do fronty" {
		t.Fatalf("propose toast=%q err=%v", toast, err)
	}

	relay.mu.Lock()
	defer relay.mu.Unlock()
	var types []string
	for _, g := range relay.got {
		types = append(types, g["type"].(string))
		if g["guestId"] != who.GuestID || g["name"] != "Olin" {
			t.Fatalf("message identity %v", g)
		}
	}
	if strings.Join(types, ",") != "join,search,propose" {
		t.Fatalf("message order %v (join must come once, first)", types)
	}
}

func TestNoSession(t *testing.T) {
	m := New("http://127.0.0.1:1")
	if _, err := m.Search(context.Background(), "NOPE", Who{"pc0123456789abcdef", "x"}, "q"); err != ErrNoSession {
		t.Fatalf("err = %v", err)
	}
}
