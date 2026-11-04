package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRecapAndPreflight(t *testing.T) {
	e := newEnv(t)
	var g struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Žofie K."}, &g)
	photo := e.upload(g.ID)
	e.waitAsset(g.ID, photo.ID, "pending")

	var rc struct {
		URL   string      `json:"url"`
		Items []recapItem `json:"items"`
	}
	e.do("GET", "/api/parties/"+e.code+"/recap", nil, &rc)
	if len(rc.Items) != 0 {
		t.Fatalf("recap must hold approved items only, got %+v", rc.Items)
	}
	e.do("POST", "/api/parties/"+e.code+"/assets/"+photo.ID+"/approve", nil, nil)
	e.do("GET", "/api/parties/"+e.code+"/recap", nil, &rc)
	if len(rc.Items) != 1 || rc.Items[0].Nickname != "Žofie K." || rc.Items[0].File == "" {
		t.Fatalf("recap items %+v", rc.Items)
	}
	if !strings.HasSuffix(rc.URL, "/recap/"+e.code) {
		t.Fatalf("recap url %q", rc.URL)
	}

	// the public queue never names the guest's original upload
	res, _ := http.Get(e.ts.URL + "/api/parties/" + e.code + "/queue")
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if bytes.Contains(raw, []byte(`"original"`)) {
		t.Fatalf("queue leaks the original: %s", raw)
	}

	res, err := http.Get(e.ts.URL + "/api/parties/" + e.code + "/recap.zip")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 1 || !strings.HasPrefix(zr.File[0].Name, "01-zofie-k-restyle.") {
		names := []string{}
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		t.Fatalf("zip entries %v", names)
	}

	// preflight is cockpit-only and reports the obvious problems of a test box
	req, _ := http.NewRequest("GET", e.ts.URL+"/api/parties/"+e.code+"/preflight", nil)
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != 401 {
		t.Fatalf("preflight without token: %d", res.StatusCode)
	}
	var pf struct {
		Checks []check `json:"checks"`
	}
	e.do("GET", "/api/parties/"+e.code+"/preflight", nil, &pf)
	got := map[string]string{}
	for _, c := range pf.Checks {
		got[c.ID] = c.Status
	}
	want := map[string]string{"join": "fail", "stage": "fail", "restyle": "warn", "animate": "warn", "djfy": "warn", "https": "warn", "ends": "warn"}
	for id, st := range want {
		if got[id] != st {
			b, _ := json.Marshal(pf.Checks)
			t.Fatalf("check %s = %q, want %q: %s", id, got[id], st, b)
		}
	}
}
