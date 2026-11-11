package gallery

// StyleMap packs (M7 sweep, DECISIONS D15): FineTuneGallery serves static packs
// built in Ol1nLLM (tools/stylemap) at /stylemaps/: index.json, <set>/map.json,
// <set>/atlas.webp and <set>/t/<i>.webp. We proxy them through the same disk
// cache, so a sweep that played once keeps playing without the uplink.

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StyleMapPath is a file a pack may have: Ol1nLLM packs use webp, packs built
// by PartyCockpit (D23) jpg.
var StyleMapPath = regexp.MustCompile(`^(index\.json|[A-Za-z0-9_-]{1,64}/(map\.json|atlas\.(webp|jpg)|t/[0-9]{1,6}\.(webp|jpg)))$`)

// manifests are rebuilt under the same names; images change only with a rebuild
const (
	manifestTTL = 10 * time.Minute
	imageTTL    = 24 * time.Hour
)

// ServeStyleMap proxies one pack file; rel is the path below /stylemaps/.
func (c *Client) ServeStyleMap(w http.ResponseWriter, r *http.Request, rel string) {
	if !StyleMapPath.MatchString(rel) {
		http.NotFound(w, r)
		return
	}
	path, err := c.cachedStyleMap(r.Context(), rel)
	if err != nil {
		http.Error(w, "stylemap unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=600")
	http.ServeFile(w, r, path)
}

// cachedStyleMap returns the cache path of rel, refreshed when it is old; a
// stale copy is still better than a gap while the origin is down.
func (c *Client) cachedStyleMap(ctx context.Context, rel string) (string, error) {
	path := filepath.Join(c.CacheDir, "stylemaps", filepath.FromSlash(rel))
	ttl := imageTTL
	if filepath.Ext(rel) == ".json" {
		ttl = manifestTTL
	}
	st, err := os.Stat(path)
	if err != nil || time.Since(st.ModTime()) > ttl {
		if derr := c.downloadURL(ctx, c.Base+"/stylemaps/"+rel, path); derr != nil && err != nil {
			return "", derr
		}
	}
	return path, nil
}

// StyleMapIndex returns the upstream index.json (through the cache).
func (c *Client) StyleMapIndex(ctx context.Context) ([]byte, error) {
	path, err := c.cachedStyleMap(ctx, "index.json")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// StyleMapCached reports whether a pack file is in the cache.
func (c *Client) StyleMapCached(rel string) bool {
	_, err := os.Stat(filepath.Join(c.CacheDir, "stylemaps", filepath.FromSlash(rel)))
	return err == nil
}

// packFiles lists every file of an upstream pack from its cached map.json.
func (c *Client) packFiles(id string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(c.CacheDir, "stylemaps", id, "map.json"))
	if err != nil {
		return nil, err
	}
	var m struct {
		Atlas  string `json:"atlas"`
		Thumbs string `json:"thumbs"`
		Images []struct {
			I int `json:"i"`
		} `json:"images"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	rels := []string{id + "/map.json", id + "/" + cmp.Or(m.Atlas, "atlas.webp")}
	tpl := cmp.Or(m.Thumbs, "t/{i}.webp")
	for _, im := range m.Images {
		rels = append(rels, id+"/"+strings.ReplaceAll(tpl, "{i}", strconv.Itoa(im.I)))
	}
	return rels, nil
}

// PackMissing counts the files of an upstream pack that are not in the cache
// (no network); an uncached map.json is an error.
func (c *Client) PackMissing(id string) (int, error) {
	rels, err := c.packFiles(id)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, rel := range rels {
		if !StyleMapPath.MatchString(rel) || !c.StyleMapCached(rel) {
			n++
		}
	}
	return n, nil
}

// PrefetchPack downloads a whole upstream pack into the cache (map, atlas and
// every thumb), so a show plays it without the uplink. It returns how many
// files are still missing.
func (c *Client) PrefetchPack(ctx context.Context, id string) (int, error) {
	if !StyleMapPath.MatchString(id + "/map.json") {
		return 0, errors.New("bad pack id")
	}
	if _, err := c.cachedStyleMap(ctx, id+"/map.json"); err != nil {
		return 0, err
	}
	rels, err := c.packFiles(id)
	if err != nil {
		return 0, err
	}
	missing := 0
	for _, rel := range rels {
		if ctx.Err() != nil {
			return missing, ctx.Err()
		}
		if !StyleMapPath.MatchString(rel) || c.StyleMapCached(rel) {
			continue
		}
		if _, err := c.cachedStyleMap(ctx, rel); err != nil {
			missing++
		}
	}
	return missing, nil
}
