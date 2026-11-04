package api

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
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
	e.dial("role=stage&key="+e.key).waitFor("party", nil)
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

func TestSocketTickets(t *testing.T) {
	e := newEnv(t)
	var tk struct{ Ticket string }
	if st := e.do("POST", "/api/parties/"+e.code+"/cockpit/ticket", nil, &tk); st != 200 || tk.Ticket == "" {
		t.Fatalf("ticket: %d", st)
	}
	if st := e.dialStatus("role=cockpit&ticket=" + tk.Ticket); st != 101 {
		t.Fatalf("fresh ticket: %d, want 101", st)
	}
	if st := e.dialStatus("role=cockpit&ticket=" + tk.Ticket); st != 401 {
		t.Fatalf("reused ticket: %d, want 401", st)
	}
	e.do("POST", "/api/parties/"+e.code+"/cockpit/ticket", nil, &tk)
	real := e.srv.Store.Now
	e.srv.Store.Now = func() time.Time { return real().Add(31 * time.Second) }
	defer func() { e.srv.Store.Now = real }()
	if st := e.dialStatus("role=cockpit&ticket=" + tk.Ticket); st != 401 {
		t.Fatalf("expired ticket: %d, want 401", st)
	}
}

func TestPartyListNeedsAdminToken(t *testing.T) {
	e := newEnv(t) // no PC_ADMIN_TOKEN
	if st := e.do("GET", "/api/parties", nil, nil); st != 401 {
		t.Fatalf("list without an admin token configured: %d, want 401", st)
	}
	if st := e.do("POST", "/api/parties", map[string]any{"name": "další"}, nil); st != 201 {
		t.Fatalf("create stays open at home: %d", st)
	}
}

func TestFilesByStatus(t *testing.T) {
	e := newEnv(t)
	gid := e.guest("Fotka")
	other := e.guest("Cizí")
	photo := e.upload(gid)
	a := e.waitAsset(gid, photo.ID, "pending")
	var orig struct {
		Assets []struct{ Kind, File, Status string }
	}
	e.do("GET", "/api/parties/"+e.code+"/guests/"+gid+"/assets", nil, &orig)
	source := ""
	for _, x := range orig.Assets {
		if x.Kind == "upload" {
			source = x.File
		}
	}
	get := func(path string) int {
		res, err := http.Get(e.ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	pub := "/files/" + e.code + "/"
	var cfg struct{ Config map[string]any }
	e.do("GET", "/api/parties/"+e.code+"/cockpit", nil, &cfg)
	key, _ := cfg.Config["fileKey"].(string)
	if get(pub+a.File) != 404 {
		t.Fatal("a pending file must not be public")
	}
	if get(pub+a.File+"?k="+key) != 200 {
		t.Fatal("the cockpit's file key opens pending files")
	}
	own := "/api/parties/" + e.code + "/guests/"
	if get(own+gid+"/files/"+a.File) != 200 || get(own+other+"/files/"+a.File) != 404 {
		t.Fatal("own file for the owner only")
	}
	e.do("POST", "/api/parties/"+e.code+"/assets/"+photo.ID+"/approve", nil, nil)
	if get(pub+a.File) != 200 {
		t.Fatal("an approved file is public")
	}
	if source != "" && source != a.File && get(pub+source) != 404 {
		t.Fatal("the original upload stays private after its restyle is approved")
	}
}

func TestSocketOrigin(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(e.ts.URL, "http") + "/ws/parties/" + e.code + "?role=guest"
	_, res, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil || res == nil || res.StatusCode != 403 {
		t.Fatalf("foreign origin must be refused: %v", err)
	}
}

func TestHugeUploadRefused(t *testing.T) {
	e := newEnv(t)
	gid := e.guest("Velká")
	img := image.NewGray(image.Rect(0, 0, 8000, 8000)) // 64 Mpx, a small file
	var buf bytes.Buffer
	png.Encode(&buf, img)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "big.png")
	fw.Write(buf.Bytes())
	mw.WriteField("style", "anime")
	mw.Close()
	res, err := http.Post(e.ts.URL+"/api/parties/"+e.code+"/guests/"+gid+"/photos", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("64 Mpx upload: %d, want 400", res.StatusCode)
	}
}

func TestHeadersAndHygiene(t *testing.T) {
	e := newEnv(t)
	res, err := http.Get(e.ts.URL + "/api/parties/" + e.code)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Content-Security-Policy-Report-Only"} {
		if res.Header.Get(h) == "" {
			t.Fatalf("missing header %s", h)
		}
	}
	if got := clean("Ahoj​ ‮ecnaL", 40); got != "Ahoj ecnaL" {
		t.Fatalf("clean kept format characters: %q", got)
	}

	// a second "Ema" is refused with a suggestion
	e.guest("Ema")
	var clash struct{ Error, Suggest string }
	if st := e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "ema"}, &clash); st != 409 || clash.Suggest != "ema 2" {
		t.Fatalf("nickname clash: %d %+v", st, clash)
	}
}

func TestRotateCockpitToken(t *testing.T) {
	e := newEnv(t)
	cockpit := e.dial("role=cockpit&token=" + e.token)
	cockpit.waitFor("party", nil)
	var out struct{ CockpitToken string }
	if st := e.do("POST", "/api/parties/"+e.code+"/cockpit/rotate", nil, &out); st != 200 || out.CockpitToken == "" || out.CockpitToken == e.token {
		t.Fatalf("rotate: %d", st)
	}
	timeout := time.After(2 * time.Second)
	for closed := false; !closed; {
		select {
		case _, open := <-cockpit.in:
			closed = !open
		case <-timeout:
			t.Fatal("the old cockpit socket must be closed")
		}
	}
	if st := e.do("GET", "/api/parties/"+e.code+"/cockpit", nil, nil); st != 401 {
		t.Fatalf("old token: %d, want 401", st)
	}
	e.token = out.CockpitToken
	if st := e.do("GET", "/api/parties/"+e.code+"/cockpit", nil, nil); st != 200 {
		t.Fatalf("new token: %d", st)
	}
}
