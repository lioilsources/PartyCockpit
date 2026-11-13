package tribute

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ValidID: promo-api tribute ids are positive integers.
var ValidID = regexp.MustCompile(`^[0-9]{1,9}$`)

// validFile: the only files the stage may read from a tribute folder.
var validFile = regexp.MustCompile(`^[0-3]s?\.jpg$`)

// Frame is one variant on disk: full size (w×h) and the small copy (sw×sh).
type Frame struct {
	W  int `json:"w"`
	H  int `json:"h"`
	SW int `json:"sw"`
	SH int `json:"sh"`
}

// Meta is tribute.json.
type Meta struct {
	Schema     int      `json:"schema"`
	ID         string   `json:"id"`
	Credit     string   `json:"credit"`
	Project    string   `json:"project"`
	Styles     []string `json:"styles"`
	Labels     []string `json:"labels"`
	Frames     []Frame  `json:"frames"`
	PostStatus string   `json:"postStatus"`
	CreatedAt  string   `json:"createdAt"`
	ImportedAt int64    `json:"importedAt"` // ms epoch
}

// Library is $PC_DATA_DIR/tributes: one folder per imported tribute.
type Library struct{ Dir string }

func (l *Library) path(id string) string { return filepath.Join(l.Dir, id) }

// Get reads one tribute's meta.
func (l *Library) Get(id string) (Meta, bool) {
	if !ValidID.MatchString(id) {
		return Meta{}, false
	}
	data, err := os.ReadFile(filepath.Join(l.path(id), "tribute.json"))
	if err != nil {
		return Meta{}, false
	}
	var m Meta
	if json.Unmarshal(data, &m) != nil || m.ID != id || len(m.Frames) == 0 {
		return Meta{}, false
	}
	return m, true
}

// Has: the tribute is on disk and complete.
func (l *Library) Has(id string) bool {
	_, ok := l.Get(id)
	return ok
}

// List: every tribute on disk, newest (highest id) first.
func (l *Library) List() []Meta {
	ents, _ := os.ReadDir(l.Dir)
	out := []Meta{}
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if m, ok := l.Get(e.Name()); ok {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.Atoi(out[i].ID)
		b, _ := strconv.Atoi(out[j].ID)
		return a > b
	})
	return out
}

// File resolves a stage file (0.jpg … 3s.jpg) of a tribute on disk.
func (l *Library) File(id, name string) (string, bool) {
	if !ValidID.MatchString(id) || !validFile.MatchString(name) {
		return "", false
	}
	p := filepath.Join(l.path(id), name)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return "", false
	}
	return p, true
}

// Remove deletes one tribute from disk.
func (l *Library) Remove(id string) error {
	if !l.Has(id) {
		return ErrNotFound
	}
	return os.RemoveAll(l.path(id))
}

// Clean removes leftovers of interrupted imports.
func (l *Library) Clean() {
	ents, _ := os.ReadDir(l.Dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			os.RemoveAll(filepath.Join(l.Dir, e.Name()))
		}
	}
}
