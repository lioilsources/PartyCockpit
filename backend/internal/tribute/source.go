// Package tribute brings PromoClown tributes onto the stage (D24). A tribute
// is a picture by another artist that PromoClown restyled into up to four
// variants and drafted as a post crediting "@artist". The Studio imports the
// variants into $PC_DATA_DIR/tributes/<id>/ on demand; the stage then reads
// only the disk, so a tribute plays offline and without promo-api.
//
// PartyCockpit only reads promo-api (GET with the agent token in a header);
// it never writes to it and never shows the artist's original picture.
package tribute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TokenHeader is how promo-api authenticates its clients.
const TokenHeader = "X-Promo-Token"

const (
	maxJSON   = 4 << 20  // one list answer
	maxImage  = 25 << 20 // one variant (PNG of ~1 MP)
	maxFetch  = 20       // posts fetched one by one when missing from the list
	postsPage = 500      // promo-api caps a list at 500
)

var (
	// ErrAuth: promo-api refused the token (401/403).
	ErrAuth = errors.New("promo-api odmítl token")
	// ErrNotFound: no such tribute, or it has no images to show.
	ErrNotFound = errors.New("tribut nenalezen")
)

// Remote is a tribute as promo-api offers it, joined with its draft post.
type Remote struct {
	ID         string   `json:"id"`
	Credit     string   `json:"credit"`
	Project    string   `json:"project"`
	Styles     []string `json:"styles"`
	Labels     []string `json:"labels"`
	PostStatus string   `json:"postStatus"`
	CreatedAt  string   `json:"createdAt"`
	N          int      `json:"n"`

	media []string // asset paths on promo-api; never leave the backend
}

// Source lists tributes and opens one of their images.
type Source interface {
	List(ctx context.Context, limit int) ([]Remote, error)
	Open(ctx context.Context, r Remote, n int) (io.ReadCloser, error)
}

// HTTPSource reads promo-api.
type HTTPSource struct {
	Base  string // e.g. http://promo-api.example:8094
	Token string // agent token; sent only in TokenHeader
	HTTP  *http.Client
	// LabelFor names a style id from PartyCockpit's own catalog (optional).
	LabelFor func(id string) (string, bool)
}

// NewHTTPSource builds a client that never follows redirects (the token must
// not travel to another host) and gives up after 30 s.
func NewHTTPSource(base, token string, labelFor func(string) (string, bool)) *HTTPSource {
	return &HTTPSource{
		Base:     strings.TrimRight(base, "/"),
		Token:    token,
		LabelFor: labelFor,
		HTTP: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

type apiTribute struct {
	ID        int64    `json:"id"`
	Project   string   `json:"project"`
	Credit    string   `json:"credit"`
	Status    string   `json:"status"`
	PostID    int64    `json:"post_id"`
	Styles    []string `json:"styles"`
	CreatedAt string   `json:"created_at"`
}

type apiPost struct {
	ID         int64    `json:"id"`
	Text       string   `json:"text"`
	MediaPaths []string `json:"media_paths"`
	Status     string   `json:"status"`
}

func (c *HTTPSource) get(ctx context.Context, path string, limit int64) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(TokenHeader, c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("promo-api: %w", err)
	}
	switch {
	case res.StatusCode == 401 || res.StatusCode == 403:
		res.Body.Close()
		return nil, ErrAuth
	case res.StatusCode == 404:
		res.Body.Close()
		return nil, ErrNotFound
	case res.StatusCode != 200:
		res.Body.Close()
		return nil, fmt.Errorf("promo-api: HTTP %d", res.StatusCode)
	}
	return limitedBody{io.LimitReader(res.Body, limit+1), res.Body, limit}, nil
}

// limitedBody fails the read instead of silently cutting a too-big answer.
type limitedBody struct {
	r     io.Reader
	c     io.Closer
	limit int64
}

func (b limitedBody) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b limitedBody) Close() error               { return b.c.Close() }

func (c *HTTPSource) getJSON(ctx context.Context, path string, v any) error {
	body, err := c.get(ctx, path, maxJSON)
	if err != nil {
		return err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("promo-api: %w", err)
	}
	if len(data) > maxJSON {
		return errors.New("promo-api: odpověď je příliš velká")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("promo-api: %w", err)
	}
	return nil
}

// List: drafted tributes (newest first) joined with their posts. Tributes
// without a post, with a rejected post or without images are left out.
func (c *HTTPSource) List(ctx context.Context, limit int) ([]Remote, error) {
	limit = min(max(limit, 1), 500)
	var ts []apiTribute
	if err := c.getJSON(ctx, "/tributes?status=drafted&limit="+strconv.Itoa(limit), &ts); err != nil {
		return nil, err
	}
	var ps []apiPost
	if err := c.getJSON(ctx, "/posts?limit="+strconv.Itoa(postsPage), &ps); err != nil {
		return nil, err
	}
	posts := make(map[int64]apiPost, len(ps))
	for _, p := range ps {
		posts[p.ID] = p
	}
	fetched := 0
	out := []Remote{}
	for _, t := range ts {
		if t.Status != "drafted" || t.PostID <= 0 {
			continue
		}
		p, ok := posts[t.PostID]
		if !ok && fetched < maxFetch {
			fetched++
			if err := c.getJSON(ctx, "/posts/"+strconv.FormatInt(t.PostID, 10), &p); err == nil {
				ok = true
			} else if errors.Is(err, ErrAuth) {
				return nil, err
			}
		}
		if !ok || p.Status == "rejected" {
			continue
		}
		media := images(p.MediaPaths)
		if len(media) == 0 {
			continue
		}
		out = append(out, Remote{
			ID: strconv.FormatInt(t.ID, 10), Credit: t.Credit, Project: t.Project,
			Styles: orEmpty(t.Styles), Labels: Labels(p.Text, t.Styles, len(media), c.LabelFor),
			PostStatus: p.Status, CreatedAt: t.CreatedAt, N: len(media), media: media,
		})
	}
	return out, nil
}

// images keeps at most four picture paths (the stage sheet has four cells).
func images(paths []string) []string {
	var out []string
	for _, p := range paths {
		switch strings.ToLower(p[strings.LastIndex(p, ".")+1:]) {
		case "png", "jpg", "jpeg", "webp":
			out = append(out, p)
		}
		if len(out) == 4 {
			break
		}
	}
	return out
}

// Open streams variant n from promo-api's asset store.
func (c *HTTPSource) Open(ctx context.Context, r Remote, n int) (io.ReadCloser, error) {
	if n < 0 || n >= len(r.media) {
		return nil, ErrNotFound
	}
	segs := strings.Split(strings.TrimLeft(r.media[n], "/"), "/")
	for i, s := range segs {
		if s == "" || s == "." || s == ".." {
			return nil, ErrNotFound
		}
		segs[i] = url.PathEscape(s)
	}
	return c.get(ctx, "/assets/"+strings.Join(segs, "/"), maxImage)
}

// Labels names the n images: the style line of the post text when it has
// exactly n names ("Tribute to @x 🎨\n\nAnime · Pixel art\n\nby …"), else the
// catalog name of each style id, else the id made readable.
func Labels(text string, styles []string, n int, labelFor func(string) (string, bool)) []string {
	paras := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	if len(paras) >= 3 {
		parts := strings.Split(strings.TrimSpace(paras[1]), " · ")
		if len(parts) == n {
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			return parts
		}
	}
	out := make([]string, n)
	for i := range out {
		if i >= len(styles) {
			continue
		}
		if labelFor != nil {
			if l, ok := labelFor(styles[i]); ok && l != "" {
				out[i] = l
				continue
			}
		}
		out[i] = readable(styles[i])
	}
	return out
}

func readable(id string) string {
	s := strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(id))
	if s == "" {
		return ""
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func orEmpty[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
