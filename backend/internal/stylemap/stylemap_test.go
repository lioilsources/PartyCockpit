package stylemap

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func solid(c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 60, 80))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	return img
}

func TestGroupByPrompt(t *testing.T) {
	g := GroupByPrompt([]Input{
		{ID: 1, Prompt: "A cat  in the rain"},
		{ID: 2, Prompt: "a cat in the rain "},
		{ID: 3, Prompt: "a dog"},
		{ID: 4, Prompt: "A CAT in the rain"},
	})
	if len(g) != 2 || g[0].N != 3 || g[1].N != 1 || g[0].Prompt != "A cat  in the rain" {
		t.Fatalf("%+v", g)
	}
	if g[0].Key != PromptKey("a cat in the rain") || len(g[0].Key) != 8 {
		t.Fatalf("key %q", g[0].Key)
	}
}

// A hue wheel shuffled: the tour must walk it around, every step a small one.
func TestOrderFollowsTheColours(t *testing.T) {
	const n = 12
	perm := []int{7, 2, 11, 0, 5, 9, 3, 10, 1, 6, 8, 4}
	imgs := make([]image.Image, n)
	for k, h := range perm {
		// red → green → blue → red around the wheel
		x := float64(h) / n * 3
		var c color.RGBA
		switch {
		case x < 1:
			c = color.RGBA{uint8(255 * (1 - x)), uint8(255 * x), 0, 255}
		case x < 2:
			c = color.RGBA{0, uint8(255 * (2 - x)), uint8(255 * (x - 1)), 255}
		default:
			c = color.RGBA{uint8(255 * (x - 2)), 0, uint8(255 * (3 - x)), 255}
		}
		imgs[k] = solid(c)
	}
	tour := Order(imgs)
	if len(tour) != n {
		t.Fatalf("tour %v", tour)
	}
	seen := map[int]bool{}
	for k := range tour {
		a, b := perm[tour[k]], perm[tour[(k+1)%n]]
		seen[tour[k]] = true
		if step := (a - b + n) % n; step != 1 && step != n-1 {
			t.Fatalf("tour jumps from hue %d to %d: %v", a, b, tour)
		}
	}
	if len(seen) != n {
		t.Fatal("tour skips an image")
	}
	if again := Order(imgs); again[0] != tour[0] || again[5] != tour[5] {
		t.Fatal("order is not deterministic")
	}
}

func TestBuildWritesOl1nLLMSchema(t *testing.T) {
	root := t.TempDir()
	var frames []Frame
	for i := range 10 {
		frames = append(frames, Frame{Img: solid(color.RGBA{uint8(i * 25), 40, 200, 255}), ID: int64(100 + i), Style: []string{"", "anime"}[i%2], Label: []string{"", "anime"}[i%2]})
	}
	info, err := Build(root, Meta{ID: "fs-abc-12345678", Title: "Session · prompt", Source: "party:abc"}, frames)
	if err != nil {
		t.Fatal(err)
	}
	if info.N != 10 || info.Atlas != "fs-abc-12345678/atlas.jpg" || info.Aspect != 0.75 {
		t.Fatalf("info %+v", info)
	}
	b, err := os.ReadFile(filepath.Join(root, "fs-abc-12345678", "map.json"))
	if err != nil {
		t.Fatal(err)
	}
	// what frontend/src/lib/stylemap.ts reads
	var m struct {
		ID     string  `json:"id"`
		Title  string  `json:"title"`
		Aspect float64 `json:"aspect"`
		Atlas  string  `json:"atlas"`
		Thumbs string  `json:"thumbs"`
		Images []struct {
			I       int    `json:"i"`
			Label   string `json:"label"`
			Style   string `json:"style"`
			Tour    *int   `json:"tour"`
			Hilbert int    `json:"hilbert"`
		} `json:"images"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Atlas != "atlas.jpg" || m.Thumbs != "t/{i}.jpg" || len(m.Images) != 10 || m.Images[0].Tour == nil || m.Images[0].Label != "základ" {
		t.Fatalf("map %+v", m)
	}
	for _, f := range []string{"atlas.jpg", "t/0.jpg", "t/9.jpg"} {
		if _, err := os.Stat(filepath.Join(root, "fs-abc-12345678", f)); err != nil {
			t.Fatal(err)
		}
	}
	if idx := Index(root); len(idx) != 1 || idx[0].ID != "fs-abc-12345678" || idx[0].Title != "Session · prompt" {
		t.Fatalf("index %+v", idx)
	}
	if _, err := Build(root, Meta{ID: "x"}, frames[:3]); err == nil {
		t.Fatal("a 3-image pack must be refused")
	}
	if _, err := Build(root, Meta{ID: "../x"}, frames); err == nil {
		t.Fatal("bad id accepted")
	}
}
