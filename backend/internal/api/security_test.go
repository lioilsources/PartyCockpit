package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

)

func (e *env) guest(nick string) string {
	e.t.Helper()
	var g struct {
		ID string `json:"id"`
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": nick}, &g); st != 201 {
		e.t.Fatalf("join %s: %d", nick, st)
	}
	return g.ID
}

func TestStageNeedsItsKey(t *testing.T) {
	e := newEnv(t)
	if st := e.dialStatus("role=stage"); st != 401 {
		t.Fatalf("stage without key: %d, want 401", st)
	}
	if st := e.dialStatus("role=stage&key=wrong"); st != 401 {
		t.Fatalf("stage with a wrong key: %d, want 401", st)
	}
	if strings.Contains(e.do2json("GET", "/api/parties/"+e.code), e.key) {
		t.Fatal("the public party must not carry the stage key")
	}
	stage := e.dial("role=stage&key=" + e.key)
	stage.waitFor("party", nil)

	// a phone streams and gets approved: the stage hears of it, other phones never do
	gid := e.guest("Kamera")
	other := e.dial("role=guest")
	other.waitFor("party", nil)
	var started struct {
		Stream struct{ ID string }
	}
	e.do("POST", "/api/parties/"+e.code+"/guests/"+gid+"/stream", map[string]any{}, &started)
	e.do("POST", "/api/parties/"+e.code+"/streams/"+started.Stream.ID+"/approve", nil, nil)
	stage.waitFor("streams", nil)
	other.none("streams", 200*time.Millisecond)
}

func TestGuestWritesEndWithTheParty(t *testing.T) {
	e := newEnv(t)
	gid := e.guest("Pozdě")
	real := e.srv.Store.Now
	later := real().Add(2 * time.Hour) // the env party lives one hour
	e.srv.Store.Now = func() time.Time { return later }
	defer func() { e.srv.Store.Now = real }()

	if st := e.do("POST", "/api/parties/"+e.code+"/guests/"+gid+"/stickers", map[string]any{"text": "ahoj"}, nil); st != 410 {
		t.Fatalf("note after the end: %d, want 410", st)
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Nový"}, nil); st != 410 {
		t.Fatalf("join after the end: %d, want 410", st)
	}
	if st := e.do("GET", "/api/parties/"+e.code+"/recap", nil, nil); st != 200 {
		t.Fatalf("recap after the end: %d, want 200", st)
	}
	if st := e.do("PATCH", "/api/parties/"+e.code, map[string]any{"endsAt": later.Add(25 * time.Hour).UnixMilli()}, nil); st != 400 {
		t.Fatalf("extending past 24 h: %d, want 400", st)
	}
	if st := e.do("PATCH", "/api/parties/"+e.code, map[string]any{"endsAt": later.Add(2 * time.Hour).UnixMilli()}, nil); st != 204 {
		t.Fatalf("extend: %d", st)
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/guests/"+gid+"/stickers", map[string]any{"text": "ahoj"}, nil); st != 201 {
		t.Fatalf("note after extending: %d, want 201", st)
	}
}

func TestWriteLimitPerIP(t *testing.T) {
	e := newEnv(t)
	gid := e.guest("Rychlík")
	clock := time.Now()
	e.srv.limits = newLimiter(func() time.Time { return clock }, writeRate, writeBurst)
	play := "/api/parties/" + e.code + "/guests/" + gid + "/play"
	for i := range 30 {
		if st := e.do("PUT", play, map[string]any{"wants": true}, nil); st/100 != 2 {
			t.Fatalf("request %d: %d", i+1, st)
		}
	}
	if st := e.do("PUT", play, map[string]any{"wants": true}, nil); st != 429 {
		t.Fatalf("31st write in a burst: %d, want 429", st)
	}
	clock = clock.Add(10 * time.Second)
	if st := e.do("PUT", play, map[string]any{"wants": true}, nil); st/100 != 2 {
		t.Fatalf("after 10 s: %d", st)
	}
	// the cockpit is not limited
	for range 40 {
		if st := e.do("POST", "/api/parties/"+e.code+"/tempo", map[string]any{"bpm": 120}, nil); st/100 != 2 {
			t.Fatalf("cockpit limited: %d", st)
		}
	}
}

func TestGuestAndNoteCaps(t *testing.T) {
	e := newEnv(t)
	e.srv.limits = newLimiter(time.Now, 1e6, 1e6) // this test is about the caps, not the rate
	ctx := context.Background()
	for i := range maxGuests - 1 {
		if _, err := e.srv.Store.CreateGuest(ctx, e.code, "h"+string(rune('a'+i%26))); err != nil {
			t.Fatal(err)
		}
	}
	gid := e.guest("Poslední")
	if st := e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Navíc"}, nil); st != 409 {
		t.Fatalf("guest over the cap: %d, want 409", st)
	}
	notes := "/api/parties/" + e.code + "/guests/" + gid + "/stickers"
	for i := range maxNotesPerMinute {
		if st := e.do("POST", notes, map[string]any{"text": "vzkaz"}, nil); st != 201 {
			t.Fatalf("note %d: %d", i+1, st)
		}
	}
	if st := e.do("POST", notes, map[string]any{"text": "vzkaz"}, nil); st != 429 {
		t.Fatalf("note over the per-minute cap: %d, want 429", st)
	}
}

func TestGuestSocketsPerIP(t *testing.T) {
	e := newEnv(t)
	for range maxGuestSocketsPerIP {
		e.dial("role=guest").waitFor("party", nil)
	}
	if st := e.dialStatus("role=guest"); st != 429 {
		t.Fatalf("socket over the per-IP cap: %d, want 429", st)
	}
	// the stage and the cockpit are not counted
	e.dial("role=stage&key=" + e.key).waitFor("party", nil)
}

// do2json returns the raw body of an unauthenticated request.
func (e *env) do2json(method, path string) string {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.ts.URL+path, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}
