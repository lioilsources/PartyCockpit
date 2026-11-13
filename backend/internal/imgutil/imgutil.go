// Package imgutil holds the small image helpers shared by the packages that
// write pictures to disk (StyleMap packs, tributes).
package imgutil

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"

	xdraw "golang.org/x/image/draw"
)

// Fit scales img down so its longer side is at most long (never up).
func Fit(img image.Image, long int) image.Image {
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

// WriteJPEG encodes img at quality q into path.
func WriteJPEG(path string, img image.Image, q int) error {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: q}); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}
