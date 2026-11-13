package tribute

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
)

// MockSource offers six made-up tributes without any network
// (PROMO_API_URL=mock): four portrait, one landscape, one with three frames.
type MockSource struct{}

type mockTribute struct {
	credit, project string
	styles, labels  []string
	w, h            int
	status          string
}

var mockTributes = []mockTribute{
	{"@inkfox", "tsumiki", []string{"anime", "pixel-art", "watercolor", "comic"}, []string{"Anime", "Pixel art", "Akvarel", "Komiks"}, 832, 1216, "draft"},
	{"@mira.draws", "tsumiki", []string{"oil", "neon", "clay", "sketch"}, []string{"Olej", "Neon", "Plastelína", "Skica"}, 896, 1152, "approved"},
	{"@paperbird", "tsumiki", []string{"ukiyo-e", "lego", "vaporwave", "chalk"}, []string{"Ukiyo-e", "Kostičky", "Vaporwave", "Křída"}, 832, 1216, "published"},
	{"@nori_art", "kiran", []string{"ghibli", "noir", "mosaic", "retro"}, []string{"Ghibli", "Noir", "Mozaika", "Retro"}, 1216, 832, "scheduled"},
	{"@tomas.k", "kiran", []string{"pop-art", "glass", "origami", "graffiti"}, []string{"Pop art", "Sklo", "Origami", "Graffiti"}, 768, 1344, "draft"},
	{"@lumen", "tsumiki", []string{"cyberpunk", "pastel", "woodcut"}, []string{"Cyberpunk", "Pastel", "Dřevoryt"}, 1024, 1024, "approved"},
}

func (MockSource) List(ctx context.Context, limit int) ([]Remote, error) {
	out := []Remote{}
	for i, t := range mockTributes {
		if len(out) >= limit {
			break
		}
		out = append(out, Remote{
			ID: strconv.Itoa(len(mockTributes) - i), Credit: t.credit, Project: t.project,
			Styles: t.styles, Labels: t.labels, PostStatus: t.status,
			CreatedAt: "2026-11-1" + strconv.Itoa(i) + "T22:00:00Z", N: len(t.styles),
		})
	}
	return out, nil
}

func (MockSource) Open(ctx context.Context, r Remote, n int) (io.ReadCloser, error) {
	i := len(mockTributes) - mustAtoi(r.ID)
	if i < 0 || i >= len(mockTributes) || n < 0 || n >= len(mockTributes[i].styles) {
		return nil, ErrNotFound
	}
	t := mockTributes[i]
	// a small picture scaled up by the importer: one gradient per style
	w, h := t.w/8, t.h/8
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	base := []color.RGBA{{236, 72, 153, 255}, {59, 130, 246, 255}, {16, 185, 129, 255}, {245, 158, 11, 255}}[n]
	for y := range h {
		for x := range w {
			k := float64(y) / float64(h)
			img.Set(x, y, color.RGBA{uint8(float64(base.R) * (1 - k*0.6)), uint8(float64(base.G) * (1 - k*0.6)), uint8(float64(base.B)*(1-k*0.6) + 40*k), 255})
		}
	}
	// a pale disc as the "face", shifted per tribute
	cx, cy, rad := w/2+(i-3)*w/24, h*2/5, w/4
	for y := cy - rad; y <= cy+rad; y++ {
		for x := cx - rad; x <= cx+rad; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= rad*rad && x >= 0 && y >= 0 && x < w && y < h {
				img.Set(x, y, color.RGBA{250, 236, 220, 255})
			}
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return io.NopCloser(&b), nil
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
