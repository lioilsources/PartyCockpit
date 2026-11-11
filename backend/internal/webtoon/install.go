package webtoon

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Installer downloads a pack part (zip on GitHub Releases), checks its sha256
// against the catalog and unpacks the files the reader needs. One at a time:
// a part is 85–100 MB and the party box shares the uplink with the guests.
type Installer struct {
	Lib     *Library
	Catalog *CatalogSource
	HTTP    *http.Client // must follow redirects (Releases → objects.githubusercontent.com)
	// MaxUnpacked caps what one zip may write (default 1.5 GB).
	MaxUnpacked int64

	mu   sync.Mutex
	jobs map[string]*Job
	busy bool
}

// Job is the progress of one import, polled by the Studio.
type Job struct {
	Pack   string `json:"pack"`
	Part   int    `json:"part"`
	Pct    int    `json:"pct"`
	Status string `json:"status"` // running | done | failed
	Error  string `json:"error,omitempty"`
}

var (
	ErrBusy     = errors.New("už se stahuje jiný balík, chvilku počkej")
	ErrNotFound = errors.New("balík v katalogu není")
	ErrAdult    = errors.New("balík je pro dospělé (WEBTOON_ALLOW_ADULT)")
)

// DirName is the install folder of one part: <pack>-<letter or number>.
func DirName(p Pack, part int) string {
	key := strconv.Itoa(part + 1)
	if part < len(p.Parts) && p.Parts[part].Part != "" {
		key = p.Parts[part].Part
	}
	return p.ID + "-" + key
}

// Start begins an import in the background.
func (in *Installer) Start(ctx context.Context, packID string, part int) error {
	cat, err := in.Catalog.Get(ctx)
	if err != nil {
		return err
	}
	p, ok := cat.Find(packID)
	if !ok || part < 0 || part >= len(p.Parts) || !SafeName.MatchString(DirName(p, part)) {
		return ErrNotFound
	}
	if p.Audience == "adult" && !in.Lib.AllowAdult {
		return ErrAdult
	}
	in.mu.Lock()
	if in.busy {
		in.mu.Unlock()
		return ErrBusy
	}
	in.busy = true
	if in.jobs == nil {
		in.jobs = map[string]*Job{}
	}
	job := &Job{Pack: p.ID, Part: part, Status: "running"}
	in.jobs[jobKey(p.ID, part)] = job
	in.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		err := in.install(ctx, cat, p, part, func(pct int) {
			in.mu.Lock()
			job.Pct = pct
			in.mu.Unlock()
		})
		in.mu.Lock()
		in.busy = false
		if err != nil {
			job.Status, job.Error = "failed", err.Error()
			slog.Warn("webtoon import failed", "pack", p.ID, "part", part, "err", err)
		} else {
			job.Status, job.Pct = "done", 100
			slog.Info("webtoon import done", "pack", p.ID, "part", part)
		}
		in.mu.Unlock()
		in.Lib.Invalidate()
	}()
	return nil
}

func jobKey(pack string, part int) string { return pack + "/" + strconv.Itoa(part) }

// Jobs returns a copy of every import since the start.
func (in *Installer) Jobs() map[string]Job {
	in.mu.Lock()
	defer in.mu.Unlock()
	out := map[string]Job{}
	for k, j := range in.jobs {
		out[k] = *j
	}
	return out
}

// Remove deletes one imported part.
func (in *Installer) Remove(dir string) error {
	if !SafeName.MatchString(dir) {
		return ErrNotFound
	}
	path := filepath.Join(in.Lib.Dir, PacksDir, dir)
	if _, err := os.Stat(path); err != nil {
		return ErrNotFound
	}
	err := os.RemoveAll(path)
	in.Lib.Invalidate()
	return err
}

func (in *Installer) install(ctx context.Context, cat Catalog, p Pack, part int, progress func(int)) error {
	pt := p.Parts[part]
	packs := filepath.Join(in.Lib.Dir, PacksDir)
	if err := os.MkdirAll(packs, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(packs, ".download-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", cat.ZipURL(pt), nil)
	if err != nil {
		return err
	}
	resp, err := in.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("stažení: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("stažení: HTTP %d", resp.StatusCode)
	}
	total := pt.Bytes
	if total <= 0 {
		total = resp.ContentLength
	}
	limit := int64(4 << 30)
	if total > 0 {
		limit = total + total/10 + 1<<20
	}
	h := sha256.New()
	var got int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			got += int64(n)
			if got > limit {
				return errors.New("zip je větší, než katalog slibuje")
			}
			h.Write(buf[:n])
			if _, err := tmp.Write(buf[:n]); err != nil {
				return err
			}
			if total > 0 {
				progress(int(min(90, got*90/total)))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("stažení: %w", rerr)
		}
	}
	if pt.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), pt.SHA256) {
		return errors.New("kontrolní součet nesedí, zip je poškozený")
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	dir := DirName(p, part)
	stage := filepath.Join(packs, "."+dir+".tmp")
	os.RemoveAll(stage)
	defer os.RemoveAll(stage)
	if err := in.unpack(tmp.Name(), stage); err != nil {
		return err
	}
	info := InstallInfo{Dir: dir, Pack: p.ID, Part: part, Title: p.Label("cs"), Audience: p.Audience, Version: p.Version, SHA256: pt.SHA256, Bytes: pt.Bytes}
	b, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(filepath.Join(stage, ".install.json"), b, 0o644); err != nil {
		return err
	}
	progress(99)
	final := filepath.Join(packs, dir)
	in.Lib.mu.Lock() // no reader walks the folder while it is swapped
	os.RemoveAll(final)
	err = os.Rename(stage, final)
	in.Lib.loaded = false
	in.Lib.mu.Unlock()
	return err
}

// unpack writes only what the reader serves: pack.json, gamebook.json,
// covers/<name> and media/<episode>/<name>. Anything else (or a path that
// tries to leave the folder) is skipped.
func (in *Installer) unpack(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	defer zr.Close()
	maxBytes := in.MaxUnpacked
	if maxBytes <= 0 {
		maxBytes = 1536 << 20
	}
	var written int64
	hasPack := false
	for _, f := range zr.File {
		rel, ok := allowedEntry(f.Name)
		if !ok || f.FileInfo().IsDir() {
			continue
		}
		if rel == "pack.json" {
			hasPack = true
		}
		path := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("zip %s: %w", rel, err)
		}
		out, err := os.Create(path)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxBytes-written+1))
		rc.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("zip %s: %w", rel, err)
		}
		if written += n; written > maxBytes {
			return errors.New("zip je po rozbalení moc velký")
		}
	}
	if !hasPack {
		return errors.New("v zipu chybí pack.json")
	}
	return nil
}

func allowedEntry(name string) (string, bool) {
	parts := strings.Split(name, "/")
	for _, p := range parts {
		if !SafeName.MatchString(p) {
			return "", false
		}
	}
	switch {
	case len(parts) == 1 && (parts[0] == "pack.json" || parts[0] == "gamebook.json"):
	case len(parts) == 2 && parts[0] == "covers" && isImage(parts[1]):
	case len(parts) == 3 && parts[0] == "media" && (isImage(parts[2]) || strings.HasSuffix(strings.ToLower(parts[2]), ".mp4")):
	default:
		return "", false
	}
	return name, true
}
