package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/lioilsources/PartyCockpit/backend/internal/config"
)

// fakeWeb stands in for the embedded build: the Svelte SPA at the root and the
// React Cockpit under react/ (D21).
var fakeWeb = fstest.MapFS{
	"index.html":              {Data: []byte("svelte")},
	"assets/app.js":           {Data: []byte("svelte-js")},
	"react/index.html":        {Data: []byte("react")},
	"react/assets/cockpit.js": {Data: []byte("react-js")},
}

func getBody(t *testing.T, c *http.Client, url string) (*http.Response, string) {
	t.Helper()
	r, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b := make([]byte, 64)
	n, _ := r.Body.Read(b)
	return r, string(b[:n])
}

func TestReactCockpitServed(t *testing.T) {
	e := newEnv(t)
	e.srv.Web = fakeWeb
	cases := []struct{ path, body, cache string }{
		{"/cockpit/" + e.code, "svelte", "no-cache"},
		{"/react/cockpit/" + e.code, "react", "no-cache"},
		{"/react/assets/cockpit.js", "react-js", "public, max-age=31536000, immutable"},
		{"/stage/" + e.code, "svelte", "no-cache"},
	}
	for _, c := range cases {
		r, body := getBody(t, http.DefaultClient, e.ts.URL+c.path)
		if r.StatusCode != 200 || body != c.body || r.Header.Get("Cache-Control") != c.cache {
			t.Errorf("%s: %d %q cache %q, want %q %q", c.path, r.StatusCode, body, r.Header.Get("Cache-Control"), c.body, c.cache)
		}
	}
}

func TestReactCockpitNotBuilt(t *testing.T) {
	e := newEnv(t)
	e.srv.Web = fstest.MapFS{"index.html": {Data: []byte("svelte")}}
	if r, _ := getBody(t, http.DefaultClient, e.ts.URL+"/react/cockpit/"+e.code); r.StatusCode != 404 {
		t.Fatalf("status %d, want 404", r.StatusCode)
	}
}

func TestCockpitUIRedirect(t *testing.T) {
	e := newEnvWith(t, func(c *config.Config) { c.CockpitUI = "react" })
	e.srv.Web = fakeWeb
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, _ := getBody(t, noFollow, e.ts.URL+"/cockpit/"+e.code)
	if r.StatusCode != http.StatusFound || r.Header.Get("Location") != "/react/cockpit/"+e.code {
		t.Fatalf("got %d → %q", r.StatusCode, r.Header.Get("Location"))
	}
	// the React UI itself is not redirected again
	if r, body := getBody(t, noFollow, e.ts.URL+"/react/cockpit/"+e.code); r.StatusCode != 200 || body != "react" {
		t.Fatalf("react page: %d %q", r.StatusCode, body)
	}
}

func TestCockpitUIConfig(t *testing.T) {
	for in, want := range map[string]string{"react": "react", " React ": "react", "svelte": "svelte", "vue": "svelte", "": "svelte"} {
		t.Setenv("PC_COCKPIT_UI", in)
		if got := config.Load().CockpitUI; got != want {
			t.Errorf("PC_COCKPIT_UI=%q → %q, want %q", in, got, want)
		}
	}
}

// A fresh party has no guests or jobs yet; both Cockpits read these as arrays.
func TestCockpitStateEmptyListsAreArrays(t *testing.T) {
	e := newEnv(t)
	var st map[string]json.RawMessage
	if code := e.do("GET", "/api/parties/"+e.code+"/cockpit", nil, &st); code != 200 {
		t.Fatalf("status %d", code)
	}
	for _, k := range []string{"assets", "guests", "ideas", "jobs"} {
		if string(st[k]) != "[]" {
			t.Errorf("%s = %s, want []", k, st[k])
		}
	}
}
