package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPing(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/system_stats" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer ok.Close()
	if err := (&Comfy{URL: ok.URL}).Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := (&RestylerBackend{URL: ok.URL}).Ping(context.Background()); err != nil {
		t.Fatalf("404 still means the host answers: %v", err)
	}
	login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.cloudflareaccess.com/login", http.StatusFound)
	}))
	defer login.Close()
	if err := (&RestylerBackend{URL: login.URL}).Ping(context.Background()); err == nil {
		t.Fatal("a CF Access login redirect must fail the ping")
	}
}
