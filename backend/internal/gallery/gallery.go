// Package gallery reads slideshow images from FineTuneGallery and proxies the
// files through a disk cache, so the stage keeps running when the uplink drops.
package gallery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Image is the subset of FineTuneGallery's GET /api/images item the stage needs.
// FineTuneGallery serves files by sha256 (/img/{sha}, /thumb/{sha}); URLs here
// point at our proxy.
type Image struct {
	ID      int64  `json:"id"`
	SHA     string `json:"sha256"`
	Prompt  string `json:"prompt"`
	StyleID string `json:"styleId"`
	Lora    string `json:"loraName"`
	Score   *int   `json:"score"`
	URL     string `json:"url"`
	Thumb   string `json:"thumb"`
	Taxo    *struct {
		Band string `json:"band"`
	} `json:"taxo,omitempty"`
}

type Client struct {
	Base              string
	CacheDir          string
	AllowUnclassified bool
	Headers           map[string]string // CF Access service token (finetune.ol1n.com is behind Access)
	HTTP              *http.Client

	mu    sync.Mutex
	lists map[string]cached
}

type cached struct {
	at    time.Time
	items []Image
}

func New(base, cacheDir string, allowUnclassified bool, headers map[string]string) *Client {
	// Access answers an unauthenticated call with a 302 to its login page; don't follow it.
	hc := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{Base: base, CacheDir: cacheDir, AllowUnclassified: allowUnclassified, Headers: headers,
		HTTP: hc, lists: map[string]cached{}}
}

type Filter struct {
	Style    string
	Lora     string
	MinScore int
	Limit    int
}

// List returns SFW images only: band "safe" when FineTuneGallery runs its
// taxonomy, unclassified ones only with FINETUNE_ALLOW_UNCLASSIFIED=1.
// Results are cached 5 minutes; on upstream failure the last good list is served.
func (c *Client) List(ctx context.Context, f Filter) ([]Image, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 100
	}
	q := url.Values{"limit": {strconv.Itoa(f.Limit)}}
	if f.Style != "" {
		q.Set("style", f.Style)
	}
	if f.Lora != "" {
		q.Set("lora", f.Lora)
	}
	if f.MinScore > 0 {
		q.Set("score", strconv.Itoa(f.MinScore))
	}
	key := q.Encode()
	c.mu.Lock()
	hit, ok := c.lists[key]
	c.mu.Unlock()
	if ok && time.Since(hit.at) < 5*time.Minute {
		return hit.items, nil
	}
	items, err := c.fetch(ctx, key)
	if err != nil {
		if ok {
			return hit.items, nil
		}
		return nil, err
	}
	c.mu.Lock()
	c.lists[key] = cached{time.Now(), items}
	c.mu.Unlock()
	return items, nil
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	return c.HTTP.Do(req)
}

func (c *Client) fetch(ctx context.Context, query string) ([]Image, error) {
	resp, err := c.get(ctx, c.Base+"/api/images?"+query)
	if err != nil {
		return nil, fmt.Errorf("gallery list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("FineTuneGallery je za Cloudflare Access: nastav CF_ACCESS_CLIENT_ID/SECRET (service token povolený v Access aplikaci finetune)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gallery list: %d", resp.StatusCode)
	}
	var body struct {
		Items []Image `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("gallery decode: %w", err)
	}
	out := make([]Image, 0, len(body.Items))
	for _, it := range body.Items {
		if !shaRE.MatchString(it.SHA) {
			continue
		}
		if it.Taxo != nil && it.Taxo.Band != "safe" {
			continue
		}
		if it.Taxo == nil && !c.AllowUnclassified {
			continue
		}
		it.URL = "/api/gallery/img/" + it.SHA
		it.Thumb = "/api/gallery/thumb/" + it.SHA
		out = append(out, it)
	}
	return out, nil
}

var shaRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ServeFile proxies /img/{sha} or /thumb/{sha} through the disk cache.
func (c *Client) ServeFile(w http.ResponseWriter, r *http.Request, kind, sha string) {
	if !shaRE.MatchString(sha) || (kind != "img" && kind != "thumb") {
		http.Error(w, "bad image id", http.StatusBadRequest)
		return
	}
	path := filepath.Join(c.CacheDir, kind, sha)
	if _, err := os.Stat(path); err != nil {
		if err := c.download(r.Context(), kind, sha, path); err != nil {
			http.Error(w, "image unavailable", http.StatusBadGateway)
			return
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	http.ServeFile(w, r, path)
}

func (c *Client) download(ctx context.Context, kind, sha, path string) error {
	resp, err := c.get(ctx, c.Base+"/"+kind+"/"+sha)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dl-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 64<<20)); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), path)
}
