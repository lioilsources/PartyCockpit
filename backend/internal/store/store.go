// Package store keeps all party state in one SQLite file (pure Go, no CGO).
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db  *sql.DB
	Now func() time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite: one writer; keeps busy errors away
	s := &Store{db: db, Now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	var v int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	for i := v; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// NewID returns n random bytes hex-encoded.
func NewID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// partyAlphabet avoids look-alikes (0/O, 1/I/L) — the code is read off a TV.
const partyAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newPartyCode() string {
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = partyAlphabet[int(b[i])%len(partyAlphabet)]
	}
	return string(b)
}

// ── parties ────────────────────────────────────────────────────────────────

type Party struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Mode         string `json:"mode"`
	LoopMode     string `json:"loopMode"`
	CockpitToken string `json:"-"`
	DJfyCode     string `json:"djfyCode"`
	AutoLive     bool   `json:"autoLive"` // animate every approved photo (M4)
	CreatedAt    int64  `json:"createdAt"`
	EndsAt       int64  `json:"endsAt"`
}

const partyCols = "code, name, mode, loop_mode, cockpit_token, djfy_code, auto_live, created_at, ends_at"

func scanParty(r interface{ Scan(...any) error }) (Party, error) {
	var p Party
	err := r.Scan(&p.Code, &p.Name, &p.Mode, &p.LoopMode, &p.CockpitToken, &p.DJfyCode, &p.AutoLive, &p.CreatedAt, &p.EndsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) CreateParty(ctx context.Context, name string, ttl time.Duration) (Party, error) {
	now := s.Now()
	p := Party{Name: name, Mode: "strict", LoopMode: "loop", CockpitToken: NewID(16),
		CreatedAt: now.UnixMilli(), EndsAt: now.Add(ttl).UnixMilli()}
	for range 5 {
		p.Code = newPartyCode()
		_, err := s.db.ExecContext(ctx, "INSERT INTO parties("+partyCols+") VALUES(?,?,?,?,?,?,?,?,?)",
			p.Code, p.Name, p.Mode, p.LoopMode, p.CockpitToken, p.DJfyCode, p.AutoLive, p.CreatedAt, p.EndsAt)
		if err == nil {
			_, err = s.db.ExecContext(ctx, "INSERT INTO scene_state(party_code, kind, started_at) VALUES(?, 'slideshow', ?)", p.Code, p.CreatedAt)
			return p, err
		}
		if !strings.Contains(err.Error(), "UNIQUE") {
			return p, fmt.Errorf("insert party: %w", err)
		}
	}
	return p, errors.New("could not allocate a party code")
}

func (s *Store) Party(ctx context.Context, code string) (Party, error) {
	return scanParty(s.db.QueryRowContext(ctx, "SELECT "+partyCols+" FROM parties WHERE code = ?", strings.ToUpper(code)))
}

func (s *Store) Parties(ctx context.Context) ([]Party, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+partyCols+" FROM parties ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Party
	for rows.Next() {
		p, err := scanParty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type PartyPatch struct {
	Name     *string `json:"name"`
	Mode     *string `json:"mode"`
	LoopMode *string `json:"loopMode"`
	DJfyCode *string `json:"djfyCode"`
	AutoLive *bool   `json:"autoLive"`
}

func (s *Store) UpdateParty(ctx context.Context, code string, p PartyPatch) error {
	set := func(col string, v *string) error {
		if v == nil {
			return nil
		}
		_, err := s.db.ExecContext(ctx, "UPDATE parties SET "+col+" = ? WHERE code = ?", *v, code)
		return err
	}
	var autoErr error
	if p.AutoLive != nil {
		_, autoErr = s.db.ExecContext(ctx, "UPDATE parties SET auto_live = ? WHERE code = ?", *p.AutoLive, code)
	}
	return errors.Join(set("name", p.Name), set("mode", p.Mode), set("loop_mode", p.LoopMode), set("djfy_code", p.DJfyCode), autoErr)
}

// ExpiredParties returns parties whose end lies more than grace in the past.
func (s *Store) ExpiredParties(ctx context.Context, grace time.Duration) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT code FROM parties WHERE ends_at < ?", s.Now().Add(-grace).UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteParty(ctx context.Context, code string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM parties WHERE code = ?", code)
	return err
}

// ── guests ─────────────────────────────────────────────────────────────────

type Guest struct {
	ID          string `json:"id"`
	PartyCode   string `json:"-"`
	Nickname    string `json:"nickname"`
	WantsToPlay bool   `json:"wantsToPlay"`
	CreatedAt   int64  `json:"createdAt"`
}

func (s *Store) CreateGuest(ctx context.Context, code, nickname string) (Guest, error) {
	g := Guest{ID: "pc" + NewID(8), PartyCode: code, Nickname: nickname, CreatedAt: s.Now().UnixMilli()}
	_, err := s.db.ExecContext(ctx, "INSERT INTO guests(id, party_code, nickname, created_at) VALUES(?,?,?,?)",
		g.ID, g.PartyCode, g.Nickname, g.CreatedAt)
	return g, err
}

func (s *Store) Guest(ctx context.Context, code, id string) (Guest, error) {
	var g Guest
	err := s.db.QueryRowContext(ctx, "SELECT id, party_code, nickname, wants_to_play, created_at FROM guests WHERE id = ? AND party_code = ?", id, code).
		Scan(&g.ID, &g.PartyCode, &g.Nickname, &g.WantsToPlay, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return g, ErrNotFound
	}
	return g, err
}

func (s *Store) Guests(ctx context.Context, code string) ([]Guest, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, party_code, nickname, wants_to_play, created_at FROM guests WHERE party_code = ? ORDER BY created_at", code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Guest
	for rows.Next() {
		var g Guest
		if err := rows.Scan(&g.ID, &g.PartyCode, &g.Nickname, &g.WantsToPlay, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) SetWantsToPlay(ctx context.Context, code, id string, wants bool) error {
	_, err := s.db.ExecContext(ctx, "UPDATE guests SET wants_to_play = ? WHERE id = ? AND party_code = ?", wants, id, code)
	return err
}

// ── assets ─────────────────────────────────────────────────────────────────

type Asset struct {
	ID        string          `json:"id"`
	PartyCode string          `json:"-"`
	GuestID   string          `json:"guestId"`
	Kind      string          `json:"kind"`
	SrcID     string          `json:"srcId,omitempty"`
	File      string          `json:"file,omitempty"`
	Meta      json.RawMessage `json:"meta"`
	NSFWScore float64         `json:"nsfwScore"`
	Status    string          `json:"status"`
	CreatedAt int64           `json:"createdAt"`
	// Live is the approved LivePortrait loop of this photo; StageQueue only.
	Live string `json:"live,omitempty"`
}

const assetCols = "id, party_code, guest_id, kind, src_id, file, meta, nsfw_score, status, created_at"

func scanAsset(r interface{ Scan(...any) error }) (Asset, error) {
	var a Asset
	var meta string
	err := r.Scan(&a.ID, &a.PartyCode, &a.GuestID, &a.Kind, &a.SrcID, &a.File, &meta, &a.NSFWScore, &a.Status, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	a.Meta = json.RawMessage(meta)
	return a, err
}

func (s *Store) CreateAsset(ctx context.Context, a Asset) (Asset, error) {
	if a.ID == "" {
		a.ID = NewID(10)
	}
	if len(a.Meta) == 0 {
		a.Meta = json.RawMessage("{}")
	}
	if a.CreatedAt == 0 {
		a.CreatedAt = s.Now().UnixMilli()
	}
	if a.NSFWScore == 0 {
		a.NSFWScore = -1
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO assets("+assetCols+") VALUES(?,?,?,?,?,?,?,?,?,?)",
		a.ID, a.PartyCode, a.GuestID, a.Kind, a.SrcID, a.File, string(a.Meta), a.NSFWScore, a.Status, a.CreatedAt)
	return a, err
}

func (s *Store) Asset(ctx context.Context, code, id string) (Asset, error) {
	return scanAsset(s.db.QueryRowContext(ctx, "SELECT "+assetCols+" FROM assets WHERE id = ? AND party_code = ?", id, code))
}

func (s *Store) AssetByID(ctx context.Context, id string) (Asset, error) {
	return scanAsset(s.db.QueryRowContext(ctx, "SELECT "+assetCols+" FROM assets WHERE id = ?", id))
}

func (s *Store) queryAssets(ctx context.Context, q string, args ...any) ([]Asset, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Assets lists a party's assets, newest first; statuses empty = all except sources.
func (s *Store) Assets(ctx context.Context, code string, statuses ...string) ([]Asset, error) {
	q := "SELECT " + assetCols + " FROM assets WHERE party_code = ? AND status != 'source'"
	args := []any{code}
	if len(statuses) > 0 {
		q += " AND status IN (?" + strings.Repeat(",?", len(statuses)-1) + ")"
		for _, st := range statuses {
			args = append(args, st)
		}
	}
	return s.queryAssets(ctx, q+" ORDER BY created_at DESC LIMIT 500", args...)
}

func (s *Store) GuestAssets(ctx context.Context, code, guestID string) ([]Asset, error) {
	return s.queryAssets(ctx, "SELECT "+assetCols+" FROM assets WHERE party_code = ? AND guest_id = ? AND status != 'source' ORDER BY created_at DESC LIMIT 50", code, guestID)
}

// GuestBusy reports whether the guest already has a job in flight (limit 1 per
// guest). Only what the guest started counts: a living loop the operator ordered
// for their photo must not block them.
func (s *Store) GuestBusy(ctx context.Context, code, guestID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM assets WHERE party_code = ? AND guest_id = ? AND status = 'processing' AND kind IN ('restyle', 'dance')", code, guestID).Scan(&n)
	return n > 0, err
}

func (s *Store) SetAssetResult(ctx context.Context, id, file, status string, nsfw float64, meta json.RawMessage) error {
	if len(meta) == 0 {
		_, err := s.db.ExecContext(ctx, "UPDATE assets SET file = ?, status = ?, nsfw_score = ? WHERE id = ?", file, status, nsfw, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE assets SET file = ?, status = ?, nsfw_score = ?, meta = ? WHERE id = ?", file, status, nsfw, string(meta), id)
	return err
}

// SetAssetStatus moves an asset to approved/rejected and keeps stage_queue in sync:
// approving appends to the show queue, anything else removes it.
func (s *Store) SetAssetStatus(ctx context.Context, code, id, status string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "UPDATE assets SET status = ? WHERE id = ? AND party_code = ? AND status != 'source'", status, id, code)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	var kind string
	if err := tx.QueryRowContext(ctx, "SELECT kind FROM assets WHERE id = ?", id).Scan(&kind); err != nil {
		return err
	}
	// a live loop is shown in place of its photo, never as its own queue item
	if status == "approved" && kind != "live" {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO stage_queue(asset_id, party_code, position)
			VALUES(?, ?, COALESCE((SELECT MAX(position) FROM stage_queue WHERE party_code = ?), 0) + 1)`, id, code, code)
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM stage_queue WHERE asset_id = ?", id)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ── stage queue ────────────────────────────────────────────────────────────

func (s *Store) StageQueue(ctx context.Context, code string) ([]Asset, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT a."+strings.ReplaceAll(assetCols, ", ", ", a.")+`,
		COALESCE((SELECT l.file FROM assets l WHERE l.src_id = a.id AND l.kind = 'live' AND l.status = 'approved'
		          ORDER BY l.created_at DESC LIMIT 1), '')
		FROM stage_queue q JOIN assets a ON a.id = q.asset_id WHERE q.party_code = ? ORDER BY q.position`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		var a Asset
		var meta string
		if err := rows.Scan(&a.ID, &a.PartyCode, &a.GuestID, &a.Kind, &a.SrcID, &a.File, &meta, &a.NSFWScore, &a.Status, &a.CreatedAt, &a.Live); err != nil {
			return nil, err
		}
		a.Meta = publicMeta(meta)
		out = append(out, a)
	}
	return out, rows.Err()
}

// publicMeta drops the name of the guest's original upload: the show queue goes
// to every stage and recap viewer, the original never does (unless the operator
// approved it as a fallback, in which case it is the asset's own file).
func publicMeta(raw string) json.RawMessage {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil || m["original"] == nil {
		return json.RawMessage(raw)
	}
	delete(m, "original")
	b, _ := json.Marshal(m)
	return b
}

// AnimationOf returns the newest live/dance asset made from src (any status).
func (s *Store) AnimationOf(ctx context.Context, srcID, kind string) (Asset, error) {
	return scanAsset(s.db.QueryRowContext(ctx, "SELECT "+assetCols+" FROM assets WHERE src_id = ? AND kind = ? AND status NOT IN ('failed', 'rejected') ORDER BY created_at DESC LIMIT 1", srcID, kind))
}

// ReorderQueue rewrites positions in the given order; ids not listed keep their
// relative order after the listed ones.
func (s *Store) ReorderQueue(ctx context.Context, code string, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE stage_queue SET position = position + 1000000 WHERE party_code = ?", code); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, "UPDATE stage_queue SET position = ? WHERE asset_id = ? AND party_code = ?", i+1, id, code); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── jobs ───────────────────────────────────────────────────────────────────

type Job struct {
	ID         string `json:"id"`
	PartyCode  string `json:"-"`
	AssetID    string `json:"assetId"`
	Intent     string `json:"intent"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	CreatedAt  int64  `json:"createdAt"`
	FinishedAt int64  `json:"finishedAt"`
}

func (s *Store) CreateJob(ctx context.Context, j Job) (Job, error) {
	j.ID = NewID(8)
	j.Status = "queued"
	j.CreatedAt = s.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, "INSERT INTO jobs(id, party_code, asset_id, intent, provider, status, created_at) VALUES(?,?,?,?,?,?,?)",
		j.ID, j.PartyCode, j.AssetID, j.Intent, j.Provider, j.Status, j.CreatedAt)
	return j, err
}

func (s *Store) UpdateJob(ctx context.Context, id, status, externalID, errMsg string) error {
	var fin int64
	if status == "done" || status == "failed" {
		fin = s.Now().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx, "UPDATE jobs SET status = ?, external_id = COALESCE(NULLIF(?, ''), external_id), error = ?, finished_at = ? WHERE id = ?",
		status, externalID, errMsg, fin, id)
	return err
}

// AbandonRunningJobs marks jobs left over from a previous process as failed and
// returns their asset ids, so the caller can fall back to the original photo.
func (s *Store) AbandonRunningJobs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT asset_id FROM jobs WHERE status IN ('queued','running')")
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	_, err = s.db.ExecContext(ctx, "UPDATE jobs SET status = 'failed', error = 'server restarted', finished_at = ? WHERE status IN ('queued','running')", s.Now().UnixMilli())
	return ids, err
}

// RecentJobs returns the party's newest jobs whose intent starts with prefix.
func (s *Store) RecentJobs(ctx context.Context, code, prefix string, n int) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, asset_id, intent, provider, external_id, status, error, created_at, finished_at
		FROM jobs WHERE party_code = ? AND intent LIKE ? ORDER BY created_at DESC LIMIT ?`, code, prefix+"%", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j := Job{PartyCode: code}
		if err := rows.Scan(&j.ID, &j.AssetID, &j.Intent, &j.Provider, &j.ExternalID, &j.Status, &j.Error, &j.CreatedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

type JobStats struct {
	Intent string  `json:"intent"`
	Status string  `json:"status"`
	Count  int     `json:"count"`
	AvgMs  float64 `json:"avgMs"`
}

func (s *Store) JobStats(ctx context.Context) ([]JobStats, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT intent, status, COUNT(*),
		COALESCE(AVG(CASE WHEN finished_at > 0 THEN finished_at - created_at END), 0) FROM jobs GROUP BY intent, status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobStats
	for rows.Next() {
		var j JobStats
		if err := rows.Scan(&j.Intent, &j.Status, &j.Count, &j.AvgMs); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ── scene ──────────────────────────────────────────────────────────────────

type Scene struct {
	Kind      string          `json:"kind"`
	Params    json.RawMessage `json:"params"`
	StartedAt int64           `json:"startedAt"`
}

func (s *Store) Scene(ctx context.Context, code string) (Scene, error) {
	var sc Scene
	var params string
	err := s.db.QueryRowContext(ctx, "SELECT kind, params, started_at FROM scene_state WHERE party_code = ?", code).Scan(&sc.Kind, &params, &sc.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Scene{Kind: "slideshow", Params: json.RawMessage("{}")}, nil
	}
	sc.Params = json.RawMessage(params)
	return sc, err
}

func (s *Store) SetScene(ctx context.Context, code, kind string, params json.RawMessage) (Scene, error) {
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	sc := Scene{Kind: kind, Params: params, StartedAt: s.Now().UnixMilli()}
	_, err := s.db.ExecContext(ctx, `INSERT INTO scene_state(party_code, kind, params, started_at) VALUES(?,?,?,?)
		ON CONFLICT(party_code) DO UPDATE SET kind = excluded.kind, params = excluded.params, started_at = excluded.started_at`,
		code, kind, string(params), sc.StartedAt)
	return sc, err
}

// ── ideas ──────────────────────────────────────────────────────────────────

type Idea struct {
	ID        string `json:"id"`
	GuestID   string `json:"guestId"`
	Nickname  string `json:"nickname"`
	Text      string `json:"text"`
	CreatedAt int64  `json:"createdAt"`
}

func (s *Store) CreateIdea(ctx context.Context, code string, g Guest, text string) (Idea, error) {
	i := Idea{ID: NewID(8), GuestID: g.ID, Nickname: g.Nickname, Text: text, CreatedAt: s.Now().UnixMilli()}
	_, err := s.db.ExecContext(ctx, "INSERT INTO ideas(id, party_code, guest_id, nickname, text, created_at) VALUES(?,?,?,?,?,?)",
		i.ID, code, i.GuestID, i.Nickname, i.Text, i.CreatedAt)
	return i, err
}

func (s *Store) Ideas(ctx context.Context, code string) ([]Idea, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, guest_id, nickname, text, created_at FROM ideas WHERE party_code = ? ORDER BY created_at", code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Idea{}
	for rows.Next() {
		var i Idea
		if err := rows.Scan(&i.ID, &i.GuestID, &i.Nickname, &i.Text, &i.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
