// Package stylemap builds StyleMap packs on the party box (D23): one
// FineTuneGallery session split by prompt, each group a pack the stage sweeps
// through. Ol1nLLM tools/stylemap orders packs with CLIP, UMAP and LPIPS in
// Python + torch; here a small colour fingerprint and a closed nearest-
// neighbour tour with 2-opt do, so a pack builds anywhere, offline, in
// seconds. The files follow Ol1nLLM's schema, so the stage and both Cockpits
// read them unchanged.
package stylemap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// MinFrames is the smallest group worth a pack (a sweep of 7 frames is a slideshow).
const MinFrames = 8

// Input is one gallery image of the session.
type Input struct {
	ID     int64
	SHA    string
	Prompt string
	Style  string
}

// Group is the images of one prompt.
type Group struct {
	Key    string  `json:"key"`
	Prompt string  `json:"prompt"`
	N      int     `json:"n"`
	Thumb  string  `json:"thumb"` // sha of the first image
	Items  []Input `json:"-"`
}

// PromptKey identifies a prompt regardless of case and spacing: a short hash,
// so it can go into a URL and a pack id.
func PromptKey(prompt string) string {
	norm := strings.ToLower(strings.Join(strings.Fields(prompt), " "))
	h := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(h[:4])
}

// GroupByPrompt splits a session's images by prompt, biggest group first.
func GroupByPrompt(in []Input) []Group {
	idx := map[string]int{}
	var out []Group
	for _, it := range in {
		k := PromptKey(it.Prompt)
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, Group{Key: k, Prompt: strings.TrimSpace(it.Prompt), Thumb: it.SHA})
		}
		out[i].Items = append(out[i].Items, it)
		out[i].N++
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].N > out[b].N })
	return out
}

// ── ordering ──────────────────────────────────────────────────────────────

const fp = 12 // fingerprint side: 12×12 RGB

func fingerprint(img image.Image) []float64 {
	small := image.NewRGBA(image.Rect(0, 0, fp, fp))
	xdraw.ApproxBiLinear.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	v := make([]float64, 0, fp*fp*3)
	for i := 0; i < len(small.Pix); i += 4 {
		v = append(v, float64(small.Pix[i]), float64(small.Pix[i+1]), float64(small.Pix[i+2]))
	}
	return v
}

func dist(a, b []float64) float64 {
	var s float64
	for i := range a {
		d := a[i] - b[i]
		s += d * d
	}
	return math.Sqrt(s)
}

// Order returns a closed tour through the images with small visual steps:
// start at the medoid, nearest neighbour, then 2-opt until nothing improves.
// It is deterministic.
func Order(imgs []image.Image) []int {
	n := len(imgs)
	if n < 3 {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	f := make([][]float64, n)
	for i, im := range imgs {
		f[i] = fingerprint(im)
	}
	d := make([][]float64, n)
	for i := range d {
		d[i] = make([]float64, n)
	}
	for i := range n {
		for j := i + 1; j < n; j++ {
			d[i][j] = dist(f[i], f[j])
			d[j][i] = d[i][j]
		}
	}
	start, best := 0, math.Inf(1)
	for i := range n {
		var s float64
		for j := range n {
			s += d[i][j]
		}
		if s < best {
			start, best = i, s
		}
	}
	tour := []int{start}
	used := make([]bool, n)
	used[start] = true
	for len(tour) < n {
		last, next, nd := tour[len(tour)-1], -1, math.Inf(1)
		for j := range n {
			if !used[j] && d[last][j] < nd {
				next, nd = j, d[last][j]
			}
		}
		tour = append(tour, next)
		used[next] = true
	}
	// 2-opt on the closed tour: reverse tour[i+1..k] when it shortens it
	for pass := 0; pass < 60; pass++ {
		improved := false
		for i := 0; i < n-1; i++ {
			for k := i + 2; k < n; k++ {
				a, b := tour[i], tour[i+1]
				c, e := tour[k], tour[(k+1)%n]
				if a == e {
					continue
				}
				if d[a][c]+d[b][e] < d[a][b]+d[c][e]-1e-9 {
					for l, r := i+1, k; l < r; l, r = l+1, r-1 {
						tour[l], tour[r] = tour[r], tour[l]
					}
					improved = true
				}
			}
		}
		if !improved {
			break
		}
	}
	return tour
}

// ── pack files ────────────────────────────────────────────────────────────

// Frame is one decoded image of a pack.
type Frame struct {
	Img   image.Image
	ID    int64
	Label string
	Style string
}

// Meta names a pack.
type Meta struct {
	ID     string
	Title  string
	Source string // e.g. "party:<session>"
}

// PackInfo is an index.json entry.
type PackInfo struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	N      int     `json:"n"`
	Aspect float64 `json:"aspect"`
	Atlas  string  `json:"atlas"`
	Source string  `json:"source,omitempty"`
}

// ValidID is a pack id that can be a folder and a URL segment.
var ValidID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const (
	thumbMax  = 384 // long side, as Ol1nLLM make_thumbs.py
	atlasCell = 32  // atlas cell width
)

// Build orders the frames and writes <root>/<id>/: map.json, atlas.jpg and
// t/<i>.jpg. An existing pack with the same id is replaced.
func Build(root string, m Meta, frames []Frame) (PackInfo, error) {
	if !ValidID.MatchString(m.ID) {
		return PackInfo{}, errors.New("bad pack id")
	}
	if len(frames) < MinFrames {
		return PackInfo{}, fmt.Errorf("pack needs at least %d images, got %d", MinFrames, len(frames))
	}
	imgs := make([]image.Image, len(frames))
	for i, f := range frames {
		imgs[i] = f.Img
	}
	tour := Order(imgs)
	pos := make([]int, len(frames))
	for k, i := range tour {
		pos[i] = k
	}

	b0 := frames[0].Img.Bounds()
	aspect := float64(b0.Dx()) / float64(max(1, b0.Dy()))
	tmp := filepath.Join(root, "."+m.ID+".tmp")
	os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "t"), 0o755); err != nil {
		return PackInfo{}, err
	}

	type entry struct {
		I       int    `json:"i"`
		ID      int64  `json:"id"`
		Label   string `json:"label"`
		Style   string `json:"style,omitempty"`
		Tour    int    `json:"tour"`
		Hilbert int    `json:"hilbert"`
	}
	images := make([]entry, len(frames))
	for i, f := range frames {
		if err := writeJPEG(filepath.Join(tmp, "t", fmt.Sprintf("%d.jpg", i)), fit(f.Img, thumbMax), 82); err != nil {
			return PackInfo{}, err
		}
		label := f.Label
		if label == "" {
			label = "základ"
		}
		images[i] = entry{I: i, ID: f.ID, Label: label, Style: f.Style, Tour: pos[i], Hilbert: pos[i]}
	}

	// atlas: every frame as a small cell, in tour order (the stage blurs it as a backdrop)
	cellH := max(1, int(math.Round(atlasCell/aspect)))
	cols := int(math.Ceil(math.Sqrt(float64(len(frames)))))
	rows := (len(frames) + cols - 1) / cols
	atlas := image.NewRGBA(image.Rect(0, 0, cols*atlasCell, rows*cellH))
	xdraw.Draw(atlas, atlas.Bounds(), &image.Uniform{color.RGBA{5, 2, 10, 255}}, image.Point{}, xdraw.Src)
	for k, i := range tour {
		r := image.Rect((k%cols)*atlasCell, (k/cols)*cellH, (k%cols+1)*atlasCell, (k/cols+1)*cellH)
		xdraw.ApproxBiLinear.Scale(atlas, r, frames[i].Img, frames[i].Img.Bounds(), xdraw.Src, nil)
	}
	if err := writeJPEG(filepath.Join(tmp, "atlas.jpg"), atlas, 80); err != nil {
		return PackInfo{}, err
	}

	mj, _ := json.MarshalIndent(map[string]any{
		"version": 1, "id": m.ID, "title": m.Title, "source": m.Source, "n": len(frames),
		"aspect": aspect, "atlas": "atlas.jpg", "thumbs": "t/{i}.jpg", "images": images,
	}, "", " ")
	if err := os.WriteFile(filepath.Join(tmp, "map.json"), mj, 0o644); err != nil {
		return PackInfo{}, err
	}
	final := filepath.Join(root, m.ID)
	os.RemoveAll(final)
	if err := os.Rename(tmp, final); err != nil {
		return PackInfo{}, err
	}
	return PackInfo{ID: m.ID, Title: m.Title, N: len(frames), Aspect: aspect, Atlas: m.ID + "/atlas.jpg", Source: "party"}, nil
}

func fit(img image.Image, long int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= long && h <= long {
		return img
	}
	if w >= h {
		w, h = long, max(1, h*long/w)
	} else {
		w, h = max(1, w*long/h), long
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}

func writeJPEG(path string, img image.Image, q int) error {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: q}); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// Index lists the packs under root (folders with a map.json).
func Index(root string) []PackInfo {
	dirs, _ := os.ReadDir(root)
	out := []PackInfo{}
	for _, d := range dirs {
		if !d.IsDir() || !ValidID.MatchString(d.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, d.Name(), "map.json"))
		if err != nil {
			continue
		}
		var m struct {
			Title  string  `json:"title"`
			N      int     `json:"n"`
			Aspect float64 `json:"aspect"`
			Atlas  string  `json:"atlas"`
		}
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		out = append(out, PackInfo{ID: d.Name(), Title: m.Title, N: m.N, Aspect: m.Aspect, Atlas: d.Name() + "/" + m.Atlas, Source: "party"})
	}
	return out
}

// Has reports whether a pack is on disk under root.
func Has(root, id string) bool {
	if !ValidID.MatchString(id) {
		return false
	}
	_, err := os.Stat(filepath.Join(root, id, "map.json"))
	return err == nil
}
