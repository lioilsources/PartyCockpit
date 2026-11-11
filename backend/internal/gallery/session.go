package gallery

// FineTuneGallery sessions (D23): one session is typically one source photo
// re-rendered with many prompts and styles. The Studio splits a session by
// prompt into StyleMap packs that PartyCockpit builds itself.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

// Session is one item of FineTuneGallery's GET /api/sessions.
type Session struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	ModelID    string `json:"modelId"`
	ImageCount int    `json:"imageCount"`
}

// flexID accepts an id sent as a string or a number.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = flexID(s)
		return nil
	}
	var n json.Number
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&n); err != nil {
		return err
	}
	*f = flexID(n.String())
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.get(ctx, c.Base+path)
	if err != nil {
		return fmt.Errorf("gallery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 302 || resp.StatusCode == 403 {
		return errors.New("FineTuneGallery je za Cloudflare Access: nastav CF_ACCESS_CLIENT_ID/SECRET")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("gallery: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Sessions lists the gallery's sessions.
func (c *Client) Sessions(ctx context.Context) ([]Session, error) {
	var raw []struct {
		ID         flexID `json:"id"`
		Title      string `json:"title"`
		ModelID    string `json:"modelId"`
		ImageCount int    `json:"imageCount"`
	}
	if err := c.getJSON(ctx, "/api/sessions", &raw); err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(raw))
	for _, s := range raw {
		out = append(out, Session{ID: string(s.ID), Title: s.Title, ModelID: s.ModelID, ImageCount: s.ImageCount})
	}
	return out, nil
}

// SessionImage is a gallery image with what grouping by prompt needs.
type SessionImage struct {
	Image
	Origin string `json:"origin"` // generated | upload
}

// SessionImages pages through every image of one session.
func (c *Client) SessionImages(ctx context.Context, session string) ([]SessionImage, error) {
	var out []SessionImage
	cursor := ""
	for range 100 { // 100 pages × 200 is far more than any session
		q := url.Values{"session": {session}, "limit": {strconv.Itoa(200)}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var body struct {
			Items      []SessionImage `json:"items"`
			NextCursor flexID         `json:"nextCursor"`
		}
		if err := c.getJSON(ctx, "/api/images?"+q.Encode(), &body); err != nil {
			return nil, err
		}
		for _, it := range body.Items {
			if shaRE.MatchString(it.SHA) {
				out = append(out, it)
			}
		}
		if body.NextCursor == "" || len(body.Items) == 0 {
			break
		}
		cursor = string(body.NextCursor)
	}
	return out, nil
}

// File returns a gallery file ("img" or "thumb") from the disk cache,
// downloading it first when needed.
func (c *Client) File(ctx context.Context, kind, sha string) ([]byte, error) {
	if !shaRE.MatchString(sha) || (kind != "img" && kind != "thumb") {
		return nil, errors.New("bad image id")
	}
	path := filepath.Join(c.CacheDir, kind, sha)
	if _, err := os.Stat(path); err != nil {
		if err := c.download(ctx, kind, sha, path); err != nil {
			return nil, err
		}
	}
	return os.ReadFile(path)
}

// Cached reports whether a gallery file is on disk (the show's offline check).
func (c *Client) Cached(kind, sha string) bool {
	if !shaRE.MatchString(sha) || (kind != "img" && kind != "thumb") {
		return false
	}
	_, err := os.Stat(filepath.Join(c.CacheDir, kind, sha))
	return err == nil
}
