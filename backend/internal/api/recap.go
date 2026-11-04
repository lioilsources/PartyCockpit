package api

// M7 recap: everything the operator approved, for the closing scene and for
// guests to take home (DECISIONS D14). It is exactly the show queue, so nothing
// a guest can download here was not already on the screen.

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

// purgeGrace matches the janitor: files go 7 days after the party ended.
const purgeGrace = 7 * 24 * time.Hour

type recapItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // restyle | upload | dance | sticker
	File     string `json:"file,omitempty"`
	Live     string `json:"live,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Label    string `json:"label,omitempty"` // style or dance name
	Text     string `json:"text,omitempty"`  // sticker
	Color    string `json:"color,omitempty"`
}

type recapMeta struct {
	Nickname    string `json:"nickname"`
	StyleLabel  string `json:"styleLabel"`
	MotionLabel string `json:"motionLabel"`
	Text        string `json:"text"`
	Color       string `json:"color"`
}

func recapItems(q []store.Asset) []recapItem {
	out := []recapItem{}
	for _, a := range q {
		var m recapMeta
		json.Unmarshal(a.Meta, &m)
		it := recapItem{ID: a.ID, Kind: a.Kind, File: a.File, Live: a.Live, Nickname: m.Nickname, Text: m.Text, Color: m.Color}
		it.Label = m.StyleLabel
		if a.Kind == "dance" {
			it.Label = m.MotionLabel
		}
		if it.File == "" && it.Text == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

func (s *Server) recap(w http.ResponseWriter, r *http.Request, p store.Party) {
	q, err := s.Store.StageQueue(r.Context(), p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"name": p.Name, "code": p.Code, "endsAt": p.EndsAt,
		"purgeAt": p.EndsAt + purgeGrace.Milliseconds(),
		"url":     s.recapURL(r, p.Code),
		"items":   recapItems(q),
	})
}

func (s *Server) recapURL(r *http.Request, code string) string {
	return strings.Replace(s.joinURL(r, code), "/join/", "/recap/", 1)
}

// recapZip streams every approved file plus the stickers as text.
func (s *Server) recapZip(w http.ResponseWriter, r *http.Request, p store.Party) {
	if !s.allowIP(w, r, 10) { // the zip streams every file of the party
		return
	}
	q, err := s.Store.StageQueue(r.Context(), p.Code)
	if err != nil {
		serverErr(w, r, err)
		return
	}
	items := recapItems(q)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="party-%s.zip"`, p.Code))
	zw := zip.NewWriter(w)
	defer zw.Close()
	var notes strings.Builder
	n := 0
	add := func(file, name string) {
		f, err := os.Open(filepath.Join(s.partyDir(p.Code), file))
		if err != nil {
			return
		}
		defer f.Close()
		// photos and video are already compressed
		dst, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: s.Store.Now()})
		if err == nil {
			_, err = io.Copy(dst, f)
		}
		if err != nil {
			slog.Warn("recap zip", "party", p.Code, "file", file, "err", err)
		}
	}
	for _, it := range items {
		if it.Kind == "sticker" {
			fmt.Fprintf(&notes, "%s: %s\n", or(it.Nickname, "host"), it.Text)
			continue
		}
		n++
		base := fmt.Sprintf("%02d-%s-%s", n, slug(or(it.Nickname, "host")), it.Kind)
		add(it.File, base+filepath.Ext(it.File))
		if it.Live != "" {
			add(it.Live, base+"-zivy"+filepath.Ext(it.Live))
		}
	}
	if notes.Len() > 0 {
		if f, err := zw.CreateHeader(&zip.FileHeader{Name: "vzkazy.txt", Method: zip.Deflate, Modified: s.Store.Now()}); err == nil {
			io.WriteString(f, notes.String())
		}
	}
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ascii folds Czech and Slovak diacritics: zip tools differ in how they read
// non-ASCII names, so recap files stay plain.
var ascii = strings.NewReplacer(
	"á", "a", "č", "c", "ď", "d", "é", "e", "ě", "e", "í", "i", "ň", "n", "ó", "o", "ř", "r",
	"š", "s", "ť", "t", "ú", "u", "ů", "u", "ý", "y", "ž", "z", "ä", "a", "ľ", "l", "ĺ", "l", "ô", "o", "ŕ", "r", "ö", "o", "ü", "u",
)

// slug keeps ASCII letters and digits, the rest becomes '-'.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range ascii.Replace(strings.ToLower(s)) {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if r := []rune(out); len(r) > 24 {
		out = string(r[:24])
	}
	return or(out, "host")
}
