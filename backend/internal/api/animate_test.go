package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

func testPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 96))
	for y := range 96 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 2), 120, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func (e *env) upload(gid string) store.Asset {
	e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "me.png")
	fw.Write(testPNG())
	mw.WriteField("style", "anime")
	mw.Close()
	res, err := http.Post(e.ts.URL+"/api/parties/"+e.code+"/guests/"+gid+"/photos", mw.FormDataContentType(), &body)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	var a store.Asset
	json.NewDecoder(res.Body).Decode(&a)
	if res.StatusCode/100 != 2 {
		e.t.Fatalf("upload: %d", res.StatusCode)
	}
	return a
}

// waitAsset polls the guest's assets until id reaches one of the statuses.
func (e *env) waitAsset(gid, id string, statuses ...string) store.Asset {
	e.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var r struct {
			Assets []store.Asset `json:"assets"`
		}
		e.do("GET", "/api/parties/"+e.code+"/guests/"+gid+"/assets", nil, &r)
		for _, a := range r.Assets {
			if a.ID == id {
				for _, st := range statuses {
					if a.Status == st {
						return a
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.t.Fatalf("asset %s never reached %v", id, statuses)
	return store.Asset{}
}

func (e *env) liveOf(gid, photoID string) (store.Asset, bool) {
	var r struct {
		Assets []store.Asset `json:"assets"`
	}
	e.do("GET", "/api/parties/"+e.code+"/guests/"+gid+"/assets", nil, &r)
	for _, a := range r.Assets {
		if a.Kind == "live" && a.SrcID == photoID {
			return a, true
		}
	}
	return store.Asset{}, false
}

func TestLivingPicturesAndDance(t *testing.T) {
	e := newEnv(t)
	var g struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Tanečník"}, &g)

	// auto-live on: approving a photo orders its living loop, which inherits approval
	if st := e.do("PATCH", "/api/parties/"+e.code, map[string]any{"autoLive": true}, nil); st/100 != 2 {
		t.Fatalf("patch autoLive: %d", st)
	}
	photo := e.upload(g.ID)
	e.waitAsset(g.ID, photo.ID, "pending")
	if st := e.do("POST", "/api/parties/"+e.code+"/assets/"+photo.ID+"/approve", nil, nil); st != 204 {
		t.Fatalf("approve: %d", st)
	}
	var live store.Asset
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a, ok := e.liveOf(g.ID, photo.ID); ok && a.Status == "approved" {
			live = a
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if live.ID == "" {
		t.Fatal("living loop was not made and auto-approved")
	}
	var q []store.Asset
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	if len(q) != 1 || q[0].ID != photo.ID || q[0].Live != live.File {
		t.Fatalf("queue must hold the photo with its loop attached, got %+v", q)
	}

	// a second animate request is a no-op: one loop per photo
	var n struct {
		Queued int `json:"queued"`
	}
	e.do("POST", "/api/parties/"+e.code+"/animate", map[string]any{}, &n)
	if n.Queued != 0 {
		t.Fatalf("animate queued %d, want 0", n.Queued)
	}

	// a dance waits for the operator, then joins the show queue
	var dance store.Asset
	if st := e.do("POST", "/api/parties/"+e.code+"/guests/"+g.ID+"/dance", map[string]any{"assetId": photo.ID, "move": "disco"}, &dance); st != 202 {
		t.Fatalf("dance: %d", st)
	}
	e.waitAsset(g.ID, dance.ID, "pending")
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	if len(q) != 1 {
		t.Fatal("a pending dance must not be in the show queue")
	}
	e.do("POST", "/api/parties/"+e.code+"/assets/"+dance.ID+"/approve", nil, nil)
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	if len(q) != 2 || q[1].Kind != "dance" {
		t.Fatalf("approved dance must join the queue, got %+v", q)
	}

	// someone else's photo cannot be danced
	var other struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Cizí"}, &other)
	if st := e.do("POST", "/api/parties/"+e.code+"/guests/"+other.ID+"/dance", map[string]any{"assetId": photo.ID, "move": "disco"}, nil); st != 404 {
		t.Fatalf("foreign photo: %d, want 404", st)
	}
}

// Hair in the breeze (D23): ordered by the operator, waits for approval like a
// dance, then plays in place of the photo (before its living loop) and never
// becomes a queue item of its own.
func TestBreezeWaitsThenPlaysInPlace(t *testing.T) {
	e := newEnv(t)
	var g struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Vlasatá"}, &g)
	photo := e.upload(g.ID)
	e.waitAsset(g.ID, photo.ID, "pending")
	e.do("POST", "/api/parties/"+e.code+"/assets/"+photo.ID+"/approve", nil, nil)

	var n struct {
		Queued int `json:"queued"`
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/animate", map[string]any{"motion": "breeze"}, &n); st != 200 || n.Queued != 1 {
		t.Fatalf("breeze: %d queued %d", st, n.Queued)
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/animate", map[string]any{"motion": "disco"}, nil); st != 400 {
		t.Fatalf("a dance from the gallery button: %d, want 400", st)
	}
	var breeze store.Asset
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && breeze.Status != "pending" {
		var r struct {
			Assets []store.Asset `json:"assets"`
		}
		e.do("GET", "/api/parties/"+e.code+"/guests/"+g.ID+"/assets", nil, &r)
		for _, a := range r.Assets {
			if a.Kind == "breeze" && a.SrcID == photo.ID {
				breeze = a
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if breeze.Status != "pending" || breeze.File == "" {
		t.Fatalf("breeze clip must wait for the operator, got %+v", breeze)
	}
	var q []store.Asset
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	if len(q) != 1 || q[0].Live != "" {
		t.Fatalf("a pending clip must not play, got %+v", q)
	}
	if res, _ := http.Get(e.ts.URL + "/files/" + e.code + "/" + breeze.File); res.StatusCode != 404 {
		t.Fatalf("pending clip served publicly: %d", res.StatusCode)
	}

	e.do("POST", "/api/parties/"+e.code+"/assets/"+breeze.ID+"/approve", nil, nil)
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	if len(q) != 1 || q[0].ID != photo.ID || q[0].Live != breeze.File {
		t.Fatalf("approved breeze must play in place of the photo, got %+v", q)
	}
	if res, _ := http.Get(e.ts.URL + "/files/" + e.code + "/" + breeze.File); res.StatusCode != 200 {
		t.Fatalf("approved clip: %d", res.StatusCode)
	}

	// a living loop made later does not push the breeze clip out
	e.do("POST", "/api/parties/"+e.code+"/animate", map[string]any{}, &n)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a, ok := e.liveOf(g.ID, photo.ID); ok && a.Status == "approved" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.do("GET", "/api/parties/"+e.code+"/queue", nil, &q)
	if q[0].Live != breeze.File {
		t.Fatalf("breeze must win over the living loop, got %q", q[0].Live)
	}

	// guests cannot order it as a dance
	if st := e.do("POST", "/api/parties/"+e.code+"/guests/"+g.ID+"/dance", map[string]any{"assetId": photo.ID, "move": "breeze"}, nil); st != 400 {
		t.Fatalf("guest breeze: %d, want 400", st)
	}
}
