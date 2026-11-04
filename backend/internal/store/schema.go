package store

// migrations are applied in order and tracked by PRAGMA user_version.
// Never edit a migration that has shipped; append a new one.
var migrations = []string{
	// 1 — initial schema
	`
CREATE TABLE parties (
  code          TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  mode          TEXT NOT NULL DEFAULT 'strict',   -- strict | relaxed
  loop_mode     TEXT NOT NULL DEFAULT 'loop',     -- loop | shuffle
  cockpit_token TEXT NOT NULL,
  djfy_code     TEXT NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL,
  ends_at       INTEGER NOT NULL
);

CREATE TABLE guests (
  id            TEXT PRIMARY KEY,
  party_code    TEXT NOT NULL REFERENCES parties(code) ON DELETE CASCADE,
  nickname      TEXT NOT NULL,
  wants_to_play INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL
);
CREATE INDEX guests_party ON guests(party_code);

-- kind: upload | restyle | sticker  (+ live | dance since M4, src_id = the animated photo)
-- status: processing | pending | approved | rejected | failed | source
CREATE TABLE assets (
  id          TEXT PRIMARY KEY,
  party_code  TEXT NOT NULL REFERENCES parties(code) ON DELETE CASCADE,
  guest_id    TEXT NOT NULL DEFAULT '',
  kind        TEXT NOT NULL,
  src_id      TEXT NOT NULL DEFAULT '',
  file        TEXT NOT NULL DEFAULT '',
  meta        TEXT NOT NULL DEFAULT '{}',
  nsfw_score  REAL NOT NULL DEFAULT -1,           -- -1 = not classified
  status      TEXT NOT NULL,
  created_at  INTEGER NOT NULL
);
CREATE INDEX assets_party ON assets(party_code, status);

CREATE TABLE jobs (
  id          TEXT PRIMARY KEY,
  party_code  TEXT NOT NULL REFERENCES parties(code) ON DELETE CASCADE,
  asset_id    TEXT NOT NULL,
  intent      TEXT NOT NULL,
  provider    TEXT NOT NULL,
  external_id TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL,                      -- queued | running | done | failed
  error       TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  finished_at INTEGER NOT NULL DEFAULT 0
);

-- What the stage may play: approved assets in show order (D5).
CREATE TABLE stage_queue (
  asset_id   TEXT PRIMARY KEY REFERENCES assets(id) ON DELETE CASCADE,
  party_code TEXT NOT NULL REFERENCES parties(code) ON DELETE CASCADE,
  position   INTEGER NOT NULL
);

CREATE TABLE scene_state (
  party_code TEXT PRIMARY KEY REFERENCES parties(code) ON DELETE CASCADE,
  kind       TEXT NOT NULL,
  params     TEXT NOT NULL DEFAULT '{}',
  started_at INTEGER NOT NULL
);

-- Story ideas from guests, raw material for later webtoon strips (D7).
CREATE TABLE ideas (
  id         TEXT PRIMARY KEY,
  party_code TEXT NOT NULL REFERENCES parties(code) ON DELETE CASCADE,
  guest_id   TEXT NOT NULL,
  nickname   TEXT NOT NULL,
  text       TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
`,
	// 2 — M4: animate approved photos automatically (LivePortrait loop per photo)
	`ALTER TABLE parties ADD COLUMN auto_live INTEGER NOT NULL DEFAULT 0;`,
	// 3 — the stage connects with its own key (live cameras, D19); existing parties get one now
	`ALTER TABLE parties ADD COLUMN stage_key TEXT NOT NULL DEFAULT '';
UPDATE parties SET stage_key = lower(hex(randomblob(16))) WHERE stage_key = '';`,
	// 4 — /files looks an asset up by its file to check its status (D19)
	`CREATE INDEX assets_file ON assets(party_code, file);`,
}
