package webtoon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

// SafeName is a file or folder name we serve: no separators, no leading dot.
var SafeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,80}$`)

// PacksDir is the folder under the webtoon dir that holds imported packs; it is
// not an episode folder.
const PacksDir = "packs"

// Episode is one strip on the stage. List leaves Panels out.
type Episode struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Story    string   `json:"story,omitempty"`
	StoryID  string   `json:"storyId,omitempty"`
	Index    int      `json:"index"`
	Cover    string   `json:"cover,omitempty"`
	Langs    []string `json:"langs"`
	Audience string   `json:"audience,omitempty"`
	Source   string   `json:"source"`         // folder | pack
	Pack     string   `json:"pack,omitempty"` // install folder of the pack
	N        int      `json:"n"`
	Panels   []Panel  `json:"panels,omitempty"`
}

type Panel struct {
	Image   string   `json:"image"`
	Loop    string   `json:"loop,omitempty"` // animated webp of the same panel
	W       int      `json:"w,omitempty"`
	H       int      `json:"h,omitempty"`
	Gap     string   `json:"gap,omitempty"` // small | medium | large
	Bubbles []Bubble `json:"bubbles,omitempty"`
	Sfx     []Sfx    `json:"sfx,omitempty"`
}

// Bubble text is drawn by the stage: the panel images carry none.
type Bubble struct {
	Kind   string            `json:"kind"` // speech | thought | shout | narration
	Text   map[string]string `json:"text"` // lang → text, the source language included
	Anchor *Point            `json:"anchor,omitempty"`
}

type Sfx struct {
	Text  string `json:"text"`
	Style string `json:"style,omitempty"`
	Pos   *Point `json:"pos,omitempty"`
}

// Point is a position inside the panel, 0–1 on both axes.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// InstallInfo is written next to an imported pack (.install.json).
type InstallInfo struct {
	Dir      string `json:"dir"`
	Pack     string `json:"pack"`
	Part     int    `json:"part"`
	Title    string `json:"title"`
	Audience string `json:"audience"`
	Version  int    `json:"version"`
	SHA256   string `json:"sha256"`
	Bytes    int64  `json:"bytes"`
}

// Library indexes episode folders ($dir/<id>/ with panels in name order and an
// optional title.txt) and imported packs ($dir/packs/<pack>-<part>/pack.json).
type Library struct {
	Dir        string
	AllowAdult bool

	mu       sync.Mutex
	loaded   bool
	list     []Episode
	byID     map[string]*entry
	installs []InstallInfo
}

type entry struct {
	ep    Episode
	media string // folder with the panel files
	cover string // file path of the cover, packs only
}

// Invalidate makes the next call rescan the disk (after an import or delete).
func (l *Library) Invalidate() {
	l.mu.Lock()
	l.loaded = false
	l.mu.Unlock()
}

func (l *Library) load() {
	if l.loaded {
		return
	}
	l.list, l.byID, l.installs = nil, map[string]*entry{}, nil
	dirs, _ := os.ReadDir(l.Dir)
	for _, d := range dirs {
		if d.IsDir() && d.Name() != PacksDir && SafeName.MatchString(d.Name()) {
			l.addFolder(d.Name())
		}
	}
	packs, _ := os.ReadDir(filepath.Join(l.Dir, PacksDir))
	for _, d := range packs {
		if d.IsDir() && SafeName.MatchString(d.Name()) {
			l.addPack(d.Name())
		}
	}
	l.loaded = true
}

func (l *Library) add(e *entry) {
	if _, dup := l.byID[e.ep.ID]; dup {
		return
	}
	e.ep.N = len(e.ep.Panels)
	if e.ep.N == 0 {
		return
	}
	l.byID[e.ep.ID] = e
	sum := e.ep
	sum.Panels = nil
	l.list = append(l.list, sum)
}

func (l *Library) addFolder(name string) {
	dir := filepath.Join(l.Dir, name)
	e := &entry{media: dir, ep: Episode{ID: name, Title: name, Source: "folder", Langs: []string{}}}
	if t, err := os.ReadFile(filepath.Join(dir, "title.txt")); err == nil {
		e.ep.Title = strings.TrimSpace(string(t))
	}
	files, _ := os.ReadDir(dir)
	var names []string
	for _, f := range files {
		if isImage(f.Name()) && SafeName.MatchString(f.Name()) {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		e.ep.Panels = append(e.ep.Panels, Panel{Image: "/webtoon/" + name + "/" + n})
	}
	l.add(e)
}

func isImage(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}

// pack.json (schema 1), Webtoon PACK_FORMAT.md.
type packFile struct {
	Worlds []struct {
		ID       string `json:"id"`
		Audience string `json:"audience"`
	} `json:"worlds"`
	Stories []struct {
		ID        string            `json:"id"`
		WorldID   string            `json:"world_id"`
		Title     string            `json:"title"`
		TitleI18n map[string]string `json:"title_i18n"`
		Episodes  []string          `json:"episodes"`
		Cover     string            `json:"cover"`
	} `json:"stories"`
	Episodes []struct {
		ID             string     `json:"id"`
		Index          int        `json:"index"`
		Title          string     `json:"title"`
		Status         string     `json:"status"`
		SourceLanguage string     `json:"source_language"`
		Languages      []string   `json:"languages"`
		Panels         []rawPanel `json:"panels"`
	} `json:"episodes"`
}

type rawPanel struct {
	Status      string `json:"status"`
	ImageURL    string `json:"image_url"`
	VideoURL    string `json:"video_url"`
	VideoStatus string `json:"video_status"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	GapAfter    string `json:"gap_after"`
	Bubbles     []struct {
		Text   string            `json:"text"`
		Kind   string            `json:"kind"`
		I18n   map[string]string `json:"i18n"`
		Anchor *Point            `json:"anchor"`
	} `json:"bubbles"`
	Sfx []struct {
		Text     string `json:"text"`
		Style    string `json:"style"`
		Position *Point `json:"position"`
	} `json:"sfx"`
}

func (l *Library) addPack(dir string) {
	root := filepath.Join(l.Dir, PacksDir, dir)
	b, err := os.ReadFile(filepath.Join(root, "pack.json"))
	if err != nil {
		return
	}
	var pf packFile
	if json.Unmarshal(b, &pf) != nil {
		return
	}
	var info InstallInfo
	if b, err := os.ReadFile(filepath.Join(root, ".install.json")); err == nil {
		json.Unmarshal(b, &info)
	}
	info.Dir = dir
	l.installs = append(l.installs, info)

	audience := map[string]string{}
	for _, w := range pf.Worlds {
		audience[w.ID] = w.Audience
	}
	eps := map[string]int{}
	for i, e := range pf.Episodes {
		eps[e.ID] = i
	}
	for _, st := range pf.Stories {
		aud := audience[st.WorldID]
		if aud == "" || info.Audience == "adult" { // the catalog's label of the whole pack wins
			aud = info.Audience
		}
		if aud == "adult" && !l.AllowAdult {
			continue
		}
		title := st.TitleI18n["cs"]
		if title == "" {
			title = st.Title
		}
		for _, id := range st.Episodes {
			i, ok := eps[id]
			if !ok || !SafeName.MatchString(id) {
				continue
			}
			raw := pf.Episodes[i]
			if raw.Status != "ready" {
				continue
			}
			langs := append([]string{}, raw.Languages...)
			if raw.SourceLanguage != "" && !slices.Contains(langs, raw.SourceLanguage) {
				langs = append([]string{raw.SourceLanguage}, langs...)
			}
			e := &entry{
				media: filepath.Join(root, "media", id),
				ep: Episode{ID: id, Title: raw.Title, Story: title, StoryID: st.ID, Index: raw.Index, Langs: langs,
					Audience: aud, Source: "pack", Pack: dir},
			}
			if st.Cover != "" && SafeName.MatchString(filepath.Base(st.Cover)) {
				e.cover = filepath.Join(root, "covers", filepath.Base(st.Cover))
				e.ep.Cover = "/webtoon/" + id + "/cover"
			}
			complete := true
			for _, p := range raw.Panels {
				img := filepath.Base(p.ImageURL)
				if p.Status != "ready" || p.ImageURL == "" || !SafeName.MatchString(img) {
					complete = false // the story ends before an unfinished episode
					break
				}
				e.ep.Panels = append(e.ep.Panels, convertPanel(id, raw.SourceLanguage, p))
			}
			if complete {
				l.add(e)
			}
		}
	}
}

func convertPanel(ep, srcLang string, p rawPanel) Panel {
	out := Panel{Image: "/webtoon/" + ep + "/" + filepath.Base(p.ImageURL), W: p.Width, H: p.Height, Gap: p.GapAfter}
	if loop := filepath.Base(p.VideoURL); p.VideoURL != "" && p.VideoStatus == "ready" && SafeName.MatchString(loop) && isImage(loop) {
		out.Loop = "/webtoon/" + ep + "/" + loop
	}
	// positions come as pixels or as 0–1 (episode.dart accepts both)
	norm := func(pt *Point) *Point {
		if pt == nil {
			return nil
		}
		q := *pt
		if (q.X > 1 || q.Y > 1) && p.Width > 0 && p.Height > 0 {
			q.X, q.Y = q.X/float64(p.Width), q.Y/float64(p.Height)
		}
		q.X, q.Y = min(max(q.X, 0), 1), min(max(q.Y, 0), 1)
		return &q
	}
	for _, b := range p.Bubbles {
		text := map[string]string{}
		for k, v := range b.I18n {
			if v != "" {
				text[k] = v
			}
		}
		if b.Text != "" {
			lang := srcLang
			if lang == "" {
				lang = "cs"
			}
			text[lang] = b.Text
		}
		if len(text) == 0 {
			continue
		}
		kind := b.Kind
		switch kind {
		case "speech", "thought", "shout", "narration":
		default:
			kind = "speech"
		}
		out.Bubbles = append(out.Bubbles, Bubble{Kind: kind, Text: text, Anchor: norm(b.Anchor)})
	}
	for _, s := range p.Sfx {
		if s.Text != "" {
			out.Sfx = append(out.Sfx, Sfx{Text: s.Text, Style: s.Style, Pos: norm(s.Position)})
		}
	}
	return out
}

// List returns every playable episode without its panels.
func (l *Library) List() []Episode {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	return append([]Episode{}, l.list...)
}

// Episode returns one episode with its panels.
func (l *Library) Episode(id string) (Episode, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	e, ok := l.byID[id]
	if !ok {
		return Episode{}, false
	}
	return e.ep, true
}

// Has reports whether the episode is on disk (the offline check of a show).
func (l *Library) Has(id string) bool {
	_, ok := l.Episode(id)
	return ok
}

// File maps /webtoon/<ep>/<name> to a path on disk; "cover" is the story cover.
func (l *Library) File(ep, name string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	e, ok := l.byID[ep]
	if !ok || !SafeName.MatchString(name) {
		return "", false
	}
	if name == "cover" {
		return e.cover, e.cover != ""
	}
	return filepath.Join(e.media, name), true
}

// Installs lists the imported packs.
func (l *Library) Installs() []InstallInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	return append([]InstallInfo{}, l.installs...)
}
