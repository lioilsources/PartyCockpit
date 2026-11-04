package gallery

// StyleMap packs (M7 sweep, DECISIONS D15): FineTuneGallery serves static packs
// built in Ol1nLLM (tools/stylemap) at /stylemaps/: index.json, <set>/map.json,
// <set>/atlas.webp and <set>/t/<i>.webp. We proxy them through the same disk
// cache, so a sweep that played once keeps playing without the uplink.

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var styleMapPath = regexp.MustCompile(`^(index\.json|[A-Za-z0-9_-]{1,64}/(map\.json|atlas\.webp|t/[0-9]{1,6}\.webp))$`)

// manifests are rebuilt under the same names; images change only with a rebuild
const (
	manifestTTL = 10 * time.Minute
	imageTTL    = 24 * time.Hour
)

// ServeStyleMap proxies one pack file; rel is the path below /stylemaps/.
func (c *Client) ServeStyleMap(w http.ResponseWriter, r *http.Request, rel string) {
	if !styleMapPath.MatchString(rel) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(c.CacheDir, "stylemaps", filepath.FromSlash(rel))
	ttl := imageTTL
	if filepath.Ext(rel) == ".json" {
		ttl = manifestTTL
	}
	st, err := os.Stat(path)
	if err != nil || time.Since(st.ModTime()) > ttl {
		if derr := c.downloadURL(r.Context(), c.Base+"/stylemaps/"+rel, path); derr != nil && err != nil {
			// nothing cached at all; a stale copy is still better than a gap
			http.Error(w, "stylemap unavailable", http.StatusBadGateway)
			return
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=600")
	http.ServeFile(w, r, path)
}
