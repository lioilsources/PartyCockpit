package tribute

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders for promo-api variants
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/lioilsources/PartyCockpit/backend/internal/imgutil"
)

const (
	fullSide  = 1344 // SDXL's longest bucket side: a tribute is never upscaled
	smallSide = 640
	thumbSide = 320
	maxSide   = 8192
	maxPixels = 40_000_000
	listTTL   = time.Minute
	thumbAge  = 14 * 24 * time.Hour
)

// ErrOff: no promo-api is configured on this machine.
var ErrOff = errors.New("stahování tributů není nastavené (PROMO_API_URL)")

// Job is the progress of one import, polled by the Studio.
type Job struct {
	ID     string `json:"id"`
	Pct    int    `json:"pct"`
	Status string `json:"status"` // queued | running | done | failed
	Error  string `json:"error,omitempty"`
}

// Importer copies tributes from a Source into the Library, one at a time
// (the uplink is shared with the guests), and serves small previews of
// tributes that are not on disk yet.
type Importer struct {
	Src Source // nil = import off
	Lib *Library
	Now func() time.Time

	mu       sync.Mutex
	jobs     map[string]*Job
	queue    chan Remote
	once     sync.Once
	list     []Remote
	listAt   time.Time
	thumbSem chan struct{}
}

func (im *Importer) now() time.Time {
	if im.Now != nil {
		return im.Now()
	}
	return time.Now()
}

func (im *Importer) init() {
	im.once.Do(func() {
		im.jobs = map[string]*Job{}
		im.queue = make(chan Remote, 64)
		im.thumbSem = make(chan struct{}, 3)
		os.MkdirAll(im.Lib.Dir, 0o755)
		im.Lib.Clean()
		im.cleanThumbs()
		go im.run()
	})
}

// Configured: there is a promo-api to import from.
func (im *Importer) Configured() bool { return im.Src != nil }

// Remote is promo-api's list, kept for a minute.
func (im *Importer) Remote(ctx context.Context) ([]Remote, error) {
	if im.Src == nil {
		return nil, ErrOff
	}
	im.init()
	im.mu.Lock()
	if im.list != nil && im.now().Sub(im.listAt) < listTTL {
		l := im.list
		im.mu.Unlock()
		return l, nil
	}
	im.mu.Unlock()
	l, err := im.Src.List(ctx, 500)
	if err != nil {
		return nil, err
	}
	im.mu.Lock()
	im.list, im.listAt = l, im.now()
	im.mu.Unlock()
	return l, nil
}

func (im *Importer) find(ctx context.Context, id string) (Remote, error) {
	l, err := im.Remote(ctx)
	if err != nil {
		return Remote{}, err
	}
	for _, r := range l {
		if r.ID == id {
			return r, nil
		}
	}
	return Remote{}, ErrNotFound
}

// Start queues the imports; ids already queued or running are skipped.
func (im *Importer) Start(ctx context.Context, ids []string) error {
	if im.Src == nil {
		return ErrOff
	}
	im.init()
	var rs []Remote
	for _, id := range ids {
		if !ValidID.MatchString(id) {
			return ErrNotFound
		}
		r, err := im.find(ctx, id)
		if err != nil {
			return err
		}
		rs = append(rs, r)
	}
	for _, r := range rs {
		im.mu.Lock()
		if j, ok := im.jobs[r.ID]; ok && (j.Status == "queued" || j.Status == "running") {
			im.mu.Unlock()
			continue
		}
		im.jobs[r.ID] = &Job{ID: r.ID, Status: "queued"}
		im.mu.Unlock()
		select {
		case im.queue <- r:
		default:
			im.set(r.ID, func(j *Job) { j.Status, j.Error = "failed", "fronta je plná, zkus to za chvíli" })
		}
	}
	return nil
}

// ImportID imports one tribute synchronously (show prefetch).
func (im *Importer) ImportID(ctx context.Context, id string) error {
	if im.Src == nil {
		return ErrOff
	}
	r, err := im.find(ctx, id)
	if err != nil {
		return err
	}
	return im.Import(ctx, r)
}

// Jobs: a copy of every import since start, by tribute id.
func (im *Importer) Jobs() map[string]Job {
	im.mu.Lock()
	defer im.mu.Unlock()
	out := make(map[string]Job, len(im.jobs))
	for k, j := range im.jobs {
		out[k] = *j
	}
	return out
}

func (im *Importer) set(id string, f func(*Job)) {
	im.mu.Lock()
	if j, ok := im.jobs[id]; ok {
		f(j)
	}
	im.mu.Unlock()
}

func (im *Importer) run() {
	for r := range im.queue {
		im.set(r.ID, func(j *Job) { j.Status = "running" })
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := im.Import(ctx, r)
		cancel()
		if err != nil {
			slog.Warn("tribute import", "id", r.ID, "err", err)
			im.set(r.ID, func(j *Job) { j.Status, j.Error = "failed", publicErr(err) })
			continue
		}
		im.set(r.ID, func(j *Job) { j.Status, j.Pct = "done", 100 })
	}
}

// publicErr is what the Studio may see (no hosts, no paths).
func publicErr(err error) string {
	switch {
	case errors.Is(err, ErrAuth), errors.Is(err, ErrNotFound), errors.Is(err, errBadImage):
		return err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "promo-api neodpovídá"
	}
	return "stažení selhalo"
}

var errBadImage = errors.New("obrázek tributu je poškozený nebo příliš velký")

// fetch reads and checks variant n: a real picture of sane size.
func (im *Importer) fetch(ctx context.Context, r Remote, n int) (image.Image, error) {
	body, err := im.Src.Open(ctx, r, n)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, maxImage+1))
	if err != nil {
		return nil, fmt.Errorf("read %d: %w", n, err)
	}
	if len(data) > maxImage {
		return nil, fmt.Errorf("%w: %d B", errBadImage, len(data))
	}
	switch http.DetectContentType(data) {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return nil, fmt.Errorf("%w: not a picture", errBadImage)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("%w: %dx%d", errBadImage, cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errBadImage, err)
	}
	return img, nil
}

// Import writes one tribute into the library. The folder is built aside and
// swapped in at the end: a failure leaves the previous copy as it was.
func (im *Importer) Import(ctx context.Context, r Remote) error {
	if !ValidID.MatchString(r.ID) || r.N < 1 {
		return ErrNotFound
	}
	if err := os.MkdirAll(im.Lib.Dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(im.Lib.Dir, ".tmp-"+r.ID+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	n := min(r.N, 4)
	meta := Meta{
		Schema: 1, ID: r.ID, Credit: r.Credit, Project: r.Project, Styles: orEmpty(r.Styles),
		PostStatus: r.PostStatus, CreatedAt: r.CreatedAt, ImportedAt: im.now().UnixMilli(),
	}
	for i := range n {
		img, err := im.fetch(ctx, r, i)
		if err != nil {
			return err
		}
		full, small := imgutil.Fit(img, fullSide), imgutil.Fit(img, smallSide)
		if err := imgutil.WriteJPEG(filepath.Join(tmp, strconv.Itoa(i)+".jpg"), full, 90); err != nil {
			return err
		}
		if err := imgutil.WriteJPEG(filepath.Join(tmp, strconv.Itoa(i)+"s.jpg"), small, 85); err != nil {
			return err
		}
		fb, sb := full.Bounds(), small.Bounds()
		meta.Frames = append(meta.Frames, Frame{W: fb.Dx(), H: fb.Dy(), SW: sb.Dx(), SH: sb.Dy()})
		im.set(r.ID, func(j *Job) { j.Pct = (i + 1) * 100 / (n + 1) })
	}
	meta.Labels = make([]string, n)
	copy(meta.Labels, r.Labels)
	data, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, "tribute.json"), data, 0o644); err != nil {
		return err
	}
	final := im.Lib.path(r.ID)
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func (im *Importer) thumbDir() string { return filepath.Join(im.Lib.Dir, ".thumbs") }

// Thumb is a small preview of variant n of a tribute that may not be on
// disk yet, cached in tributes/.thumbs.
func (im *Importer) Thumb(ctx context.Context, id string, n int) (string, error) {
	if !ValidID.MatchString(id) || n < 0 || n > 3 {
		return "", ErrNotFound
	}
	p := filepath.Join(im.thumbDir(), id+"-"+strconv.Itoa(n)+".jpg")
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p, nil
	}
	r, err := im.find(ctx, id)
	if err != nil {
		return "", err
	}
	if n >= r.N {
		return "", ErrNotFound
	}
	select {
	case im.thumbSem <- struct{}{}:
		defer func() { <-im.thumbSem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	img, err := im.fetch(ctx, r, n)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(im.thumbDir(), 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := imgutil.WriteJPEG(tmp, imgutil.Fit(img, thumbSide), 80); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

func (im *Importer) cleanThumbs() {
	ents, _ := os.ReadDir(im.thumbDir())
	for _, e := range ents {
		if fi, err := e.Info(); err == nil && im.now().Sub(fi.ModTime()) > thumbAge {
			os.Remove(filepath.Join(im.thumbDir(), e.Name()))
		}
	}
}
