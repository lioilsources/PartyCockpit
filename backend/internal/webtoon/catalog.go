// Package webtoon reads finished stories for the stage's webtoon reader (D23):
// the Toonfork content catalog (public, GitHub Pages + Releases), pack zips
// imported to the data dir, and the older hand-made episode folders (D7).
package webtoon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Catalog is the subset of webtoon-content catalog.json (schema 2) we use.
// Packs and gamebooks share one shape; gamebook parts have no "part" letter.
type Catalog struct {
	Schema    int      `json:"schema"`
	BaseURLs  BaseURLs `json:"base_urls"`
	Packs     []Pack   `json:"packs"`
	Gamebooks []Pack   `json:"gamebooks"`
}

type BaseURLs struct {
	Pages    string `json:"pages"`
	Releases string `json:"releases"`
	R2       string `json:"r2"` // null in the JSON = ""
}

type Pack struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	TitleI18n map[string]string `json:"title_i18n"`
	Free      bool              `json:"free"`
	Version   int               `json:"version"`
	Bytes     int64             `json:"bytes"`
	Audience  string            `json:"audience"` // kids | teen | adult
	Languages []string          `json:"languages"`
	Parts     []Part            `json:"parts"`
}

type Part struct {
	Part    string `json:"part"`
	Release string `json:"release"`
	Asset   string `json:"asset"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
	Stories int    `json:"stories"`
	URL     string `json:"url"`
}

// All lists packs, then gamebooks.
func (c Catalog) All() []Pack { return append(append([]Pack{}, c.Packs...), c.Gamebooks...) }

func (c Catalog) Find(id string) (Pack, bool) {
	for _, p := range c.All() {
		if p.ID == id {
			return p, true
		}
	}
	return Pack{}, false
}

// ZipURL is where one part's zip lives: part.url, else the R2 or Releases base.
func (c Catalog) ZipURL(p Part) string {
	if p.URL != "" {
		return p.URL
	}
	base := c.BaseURLs.R2
	if base == "" {
		base = c.BaseURLs.Releases
	}
	return strings.TrimRight(base, "/") + "/" + p.Release + "/" + p.Asset
}

// Label is the title in lang, falling back to the plain title.
func (p Pack) Label(lang string) string {
	if t := p.TitleI18n[lang]; t != "" {
		return t
	}
	return p.Title
}

// CatalogSource fetches catalog.json and keeps it for 10 minutes; the last good
// copy is also on disk, so a party without uplink still lists what it knew.
type CatalogSource struct {
	URL  string
	Path string // disk copy
	HTTP *http.Client

	mu  sync.Mutex
	at  time.Time
	cat *Catalog
}

func (s *CatalogSource) Get(ctx context.Context) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat != nil && time.Since(s.at) < 10*time.Minute {
		return *s.cat, nil
	}
	cat, err := s.fetch(ctx)
	if err == nil {
		s.cat, s.at = &cat, time.Now()
		return cat, nil
	}
	if s.cat != nil {
		return *s.cat, nil
	}
	if b, rerr := os.ReadFile(s.Path); rerr == nil {
		var c Catalog
		if json.Unmarshal(b, &c) == nil {
			s.cat, s.at = &c, time.Now().Add(-9*time.Minute) // retry the network soon
			return c, nil
		}
	}
	return Catalog{}, err
}

func (s *CatalogSource) fetch(ctx context.Context) (Catalog, error) {
	if s.URL == "" {
		return Catalog{}, errors.New("WEBTOON_CATALOG_URL není nastavené")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
	if err != nil {
		return Catalog{}, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return Catalog{}, fmt.Errorf("webtoon catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Catalog{}, fmt.Errorf("webtoon catalog: %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return Catalog{}, fmt.Errorf("webtoon catalog: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return Catalog{}, fmt.Errorf("webtoon catalog: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err == nil {
		os.WriteFile(s.Path, b, 0o644)
	}
	return c, nil
}
