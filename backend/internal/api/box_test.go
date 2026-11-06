package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
)

func boxEnv(t *testing.T) *env {
	return newEnvWith(t, func(c *config.Config) {
		c.Box, c.BoxAutoParty, c.BoxName = true, true, "Box"
		c.PublicURL = "http://box.test:8098"
		c.MaxLive = 4
		c.StageScale = 0.5
	})
}

func TestBoxCreatesAPartyAtBoot(t *testing.T) {
	e := boxEnv(t)
	ps, _ := e.srv.Store.Parties(t.Context())
	names := []string{}
	for _, p := range ps {
		names = append(names, p.Name)
	}
	if len(ps) != 2 || !strings.Contains(strings.Join(names, ","), "Box") {
		t.Fatalf("parties %v: the box must create one at boot (the env adds its own)", names)
	}
}

func TestBoxPageIsLoopbackOnly(t *testing.T) {
	e := boxEnv(t)
	// httptest dials from 127.0.0.1: the kiosk is sent to the newest open party's stage, key included
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(e.ts.URL + "/box")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	loc := res.Header.Get("Location")
	open, _ := e.srv.openParty(t.Context())
	if res.StatusCode != 302 || loc != "/stage/"+open.Code+"?key="+open.StageKey {
		t.Fatalf("/box: %d %q", res.StatusCode, loc)
	}
	// from the LAN: nothing
	for _, path := range []string{"/box", "/api/box"} {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "192.0.2.7:51000"
		rec := httptest.NewRecorder()
		e.srv.Routes().ServeHTTP(rec, req)
		if rec.Code != 404 {
			t.Fatalf("%s from the LAN: %d, want 404", path, rec.Code)
		}
	}
	// not a box: no /box at all
	plain := newEnv(t)
	res, _ = client.Get(plain.ts.URL + "/box")
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("/box without PC_BOX: %d", res.StatusCode)
	}
}

func TestBoxPairing(t *testing.T) {
	e := boxEnv(t)
	e.srv.startPairing(e.code)
	stage := e.dial("role=stage&key=" + e.key)
	party := stage.waitFor("party", nil)
	if !strings.Contains(string(party), `"box":{"scale":0.5}`) {
		t.Fatalf("box defaults missing: %s", party)
	}
	raw := stage.waitFor("pairing", func(d json.RawMessage) bool { return string(d) != "null" })
	var pr struct{ URL string }
	json.Unmarshal(raw, &pr)
	if pr.URL != "http://box.test:8098/cockpit/"+e.code+"#token="+e.token {
		t.Fatalf("pairing url %q", pr.URL)
	}
	guest := e.dial("role=guest")
	guest.waitFor("party", nil)

	// the first Cockpit takes the QR off the TV; guests never see it
	cockpit := e.dial("role=cockpit&token=" + e.token)
	cockpit.waitFor("party", nil)
	stage.waitFor("pairing", func(d json.RawMessage) bool { return string(d) == "null" })
	guest.none("pairing", 150*time.Millisecond)

	// a Cockpit can show it again
	if st := e.do("POST", "/api/parties/"+e.code+"/pair", nil, nil); st != 204 {
		t.Fatalf("pair: %d", st)
	}
	stage.waitFor("pairing", func(d json.RawMessage) bool { return string(d) != "null" })
	// and it expires on its own
	real := e.srv.Store.Now
	e.srv.Store.Now = func() time.Time { return real().Add(pairWindow + time.Minute) }
	defer func() { e.srv.Store.Now = real }()
	again := e.dial("role=stage&key=" + e.key)
	again.waitFor("pairing", func(d json.RawMessage) bool { return string(d) == "null" })
}

func TestBoxLiveLimit(t *testing.T) {
	e := boxEnv(t)
	for i := range 5 {
		gid := e.guest("Kamera " + string(rune('A'+i)))
		var started struct{ Stream struct{ ID string } }
		e.do("POST", "/api/parties/"+e.code+"/guests/"+gid+"/stream", map[string]any{}, &started)
		st := e.do("POST", "/api/parties/"+e.code+"/streams/"+started.Stream.ID+"/approve", nil, nil)
		if i < 4 && st/100 != 2 {
			t.Fatalf("stream %d: %d", i+1, st)
		}
		if i == 4 && st != 409 {
			t.Fatalf("5th live stream on a Pi box: %d, want 409", st)
		}
	}
}

func TestLoopbackHost(t *testing.T) {
	for h, want := range map[string]bool{"127.0.0.1:8098": true, "localhost": true, "[::1]:80": true, "box.test:8098": false, "192.0.2.1": false} {
		if isLoopbackHost(h) != want {
			t.Fatalf("isLoopbackHost(%q) != %v", h, want)
		}
	}
}
