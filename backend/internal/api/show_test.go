package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

func (e *env) scene() string {
	e.t.Helper()
	var p struct {
		Scene struct {
			Kind string `json:"kind"`
		} `json:"scene"`
	}
	e.do("GET", "/api/parties/"+e.code, nil, &p)
	return p.Scene.Kind
}

func (e *env) waitScene(kind string) {
	e.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e.scene() == kind {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	e.t.Fatalf("scene never became %s (is %s)", kind, e.scene())
}

func (e *env) studioUpload(nick string) store.Asset {
	e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "me.png")
	fw.Write(testPNG())
	mw.WriteField("style", "anime")
	mw.WriteField("nickname", nick)
	mw.Close()
	req, _ := http.NewRequest("POST", e.ts.URL+"/api/parties/"+e.code+"/studio/photos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+e.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	var a store.Asset
	json.NewDecoder(res.Body).Decode(&a)
	if res.StatusCode != 202 {
		e.t.Fatalf("studio upload: %d", res.StatusCode)
	}
	return a
}

func (e *env) queue() []store.Asset {
	var q []store.Asset
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	return q
}

func TestShowRunnerPlaysEntriesInOrder(t *testing.T) {
	e := newEnv(t)
	e.srv.showUnit = 20 * time.Millisecond // a show "second"

	bad := map[string]any{"entries": []any{map[string]any{"kind": "game", "seconds": 5}}}
	if c := e.do("PUT", "/api/parties/"+e.code+"/show", bad, nil); c != 400 {
		t.Fatalf("game in a show: %d, want 400", c)
	}
	show := map[string]any{"name": "Večer", "loop": false, "entries": []any{
		map[string]any{"id": "a", "kind": "strips", "seconds": 5, "params": map[string]any{"cols": 3, "seed": 7}},
		map[string]any{"id": "b", "kind": "stylesweep", "seconds": 5, "params": map[string]any{"packs": []string{"x"}}},
		map[string]any{"kind": "blank", "seconds": 1}, // too short: 5
	}}
	var st ShowState
	if c := e.do("PUT", "/api/parties/"+e.code+"/show", show, &st); c != 200 || len(st.Entries) != 3 || st.Entries[2].Seconds != 5 || st.Entries[2].ID == "" {
		t.Fatalf("put %d %+v", c, st)
	}

	if c := e.do("POST", "/api/parties/"+e.code+"/show/play", map[string]any{"index": 0}, &st); c != 200 || !st.Playing || st.EntryID != "a" {
		t.Fatalf("play %d %+v", c, st)
	}
	if e.scene() != "strips" {
		t.Fatalf("first entry not on the stage: %s", e.scene())
	}
	e.waitScene("stylesweep")
	e.waitScene("blank")
	// no loop: the show stops on its last scene
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && st.Playing {
		e.do("GET", "/api/parties/"+e.code+"/show", nil, &st)
		time.Sleep(10 * time.Millisecond)
	}
	if st.Playing || e.scene() != "blank" {
		t.Fatalf("show must stop after the last entry: %+v %s", st, e.scene())
	}

	// next/prev move by hand; PANIC pauses the show
	e.do("POST", "/api/parties/"+e.code+"/show/play", map[string]any{"index": 1}, &st)
	e.do("POST", "/api/parties/"+e.code+"/show/prev", nil, &st)
	if st.EntryID != "a" || e.scene() != "strips" {
		t.Fatalf("prev: %+v %s", st, e.scene())
	}
	e.do("POST", "/api/parties/"+e.code+"/panic", nil, nil)
	e.do("GET", "/api/parties/"+e.code+"/show", nil, &st)
	time.Sleep(150 * time.Millisecond)
	if st.Playing || e.scene() != "slideshow" {
		t.Fatalf("panic must pause the show: %+v %s", st, e.scene())
	}
}

func TestStudioPhotosApproveThemselves(t *testing.T) {
	e := newEnv(t)
	a := e.studioUpload("Ema")
	deadline := time.Now().Add(5 * time.Second)
	var q []store.Asset
	for time.Now().Before(deadline) && len(q) == 0 {
		q = e.queue()
		time.Sleep(20 * time.Millisecond)
	}
	if len(q) != 1 || q[0].ID != a.ID {
		t.Fatalf("a restyled Studio photo must join the queue by itself, got %+v", q)
	}
	var st cockpitState
	e.do("GET", "/api/parties/"+e.code+"/cockpit", nil, &st)
	if len(st.Guests) != 0 {
		t.Fatalf("the Studio guest must not be listed: %+v", st.Guests)
	}
	// a guest's photo still waits
	var g struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Host"}, &g)
	photo := e.upload(g.ID)
	e.waitAsset(g.ID, photo.ID, "pending")
}

func TestShowReadyAndLibrary(t *testing.T) {
	e := newEnv(t)
	photo := e.studioUpload("Ema")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(e.queue()) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	// its living loop (inherits the approval) travels with it
	e.do("POST", "/api/parties/"+e.code+"/animate", map[string]any{"ids": []string{photo.ID}}, nil)
	for time.Now().Before(deadline) && e.queue()[0].Live == "" {
		time.Sleep(20 * time.Millisecond)
	}
	show := map[string]any{"name": "Ema 40 – narozky", "loop": true, "entries": []any{
		map[string]any{"id": "a", "kind": "strips", "seconds": 30, "params": map[string]any{"assets": []string{photo.ID}, "seed": 1}},
		map[string]any{"id": "b", "kind": "webtoon", "seconds": 30, "params": map[string]any{"episodes": []string{"ep_missing"}}},
	}}
	e.do("PUT", "/api/parties/"+e.code+"/show", show, nil)

	var ready struct {
		Items []readyItem `json:"items"`
		OK    int         `json:"ok"`
	}
	e.do("GET", "/api/parties/"+e.code+"/show/ready", nil, &ready)
	if ready.OK != 1 || ready.Items[0].Status != "ok" || ready.Items[1].Status != "fail" {
		t.Fatalf("ready %+v", ready)
	}

	var saved struct {
		Slug   string `json:"slug"`
		Assets int    `json:"assets"`
	}
	if c := e.do("POST", "/api/parties/"+e.code+"/shows", map[string]any{}, &saved); c != 201 || saved.Slug != "ema-40-narozky" || saved.Assets != 1 {
		t.Fatalf("save %d %+v", c, saved)
	}
	media, _ := os.ReadDir(filepath.Join(e.srv.Cfg.DataDir, "shows", saved.Slug, "media"))
	if len(media) != 2 {
		t.Fatalf("library must hold the photo and its clip, has %d files", len(media))
	}

	// another party loads it: new ids, approved, the show points at them
	var p2 struct {
		Party struct {
			Code string `json:"code"`
		} `json:"party"`
		CockpitToken string `json:"cockpitToken"`
	}
	e.do("POST", "/api/parties", map[string]any{"name": "druhá"}, &p2)
	e2 := &env{t: t, ts: e.ts, srv: e.srv, code: p2.Party.Code, token: p2.CockpitToken}
	var shows []struct {
		Slug string `json:"slug"`
	}
	e2.do("GET", "/api/parties/"+e2.code+"/shows", nil, &shows)
	if len(shows) != 1 {
		t.Fatalf("library list %+v", shows)
	}
	if c := e2.do("POST", "/api/parties/"+e2.code+"/shows/"+saved.Slug+"/load", nil, nil); c != 200 {
		t.Fatalf("load %d", c)
	}
	q := e2.queue()
	if len(q) != 1 || q[0].ID == photo.ID || q[0].Live == "" {
		t.Fatalf("loaded queue %+v", q)
	}
	var st ShowState
	e2.do("GET", "/api/parties/"+e2.code+"/show", nil, &st)
	var params struct {
		Assets []string `json:"assets"`
	}
	json.Unmarshal(st.Entries[0].Params, &params)
	if st.Name != "Ema 40 – narozky" || len(params.Assets) != 1 || params.Assets[0] != q[0].ID {
		t.Fatalf("loaded show %+v %+v", st, params)
	}
	if res, _ := http.Get(e.ts.URL + "/files/" + e2.code + "/" + q[0].Live); res.StatusCode != 200 {
		t.Fatalf("loaded clip not served: %d", res.StatusCode)
	}
}
