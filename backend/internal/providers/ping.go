package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Pinger is implemented by providers that can tell cheaply whether their host
// answers (Cockpit preflight, DECISIONS D14). Mocks don't implement it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// pingc does not follow redirects: behind Cloudflare Access a missing or wrong
// service token shows up as a redirect to the login page.
var pingc = &http.Client{
	Timeout:       4 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func ping(ctx context.Context, a Auth, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	a.apply(req)
	resp, err := pingc.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode/100 == 3:
		return fmt.Errorf("přesměrování na přihlášení (%d), zkontroluj CF Access token", resp.StatusCode)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("odmítnuto (%d), zkontroluj token", resp.StatusCode)
	case resp.StatusCode >= 500:
		return fmt.Errorf("chyba serveru %d", resp.StatusCode)
	}
	return nil
}

// Ping: ComfyUI answers /system_stats when it runs (on the L40S only in profile D).
func (c *Comfy) Ping(ctx context.Context) error { return ping(ctx, c.Auth, c.URL+"/system_stats") }

// Ping: any non-auth answer from the Restyler API means network and tokens are fine.
func (r *RestylerBackend) Ping(ctx context.Context) error { return ping(ctx, r.Auth, r.URL+"/") }

// Ping: the Restyler animator shares the Restyler host.
func (r RestylerAnimator) Ping(ctx context.Context) error { return r.R.Ping(ctx) }
