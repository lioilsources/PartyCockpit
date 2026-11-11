# PartyCockpit

## Účel
Party show na plátno: Stage (TV), Cockpit (operátor), Guest (telefon přes QR). Plán
`Prompts/00-PARTYKOKPIT_PLAN.md`, odpovědi a odchylky `DECISIONS.md`, stav `STATUS.md`.

## Build a test
```sh
make build      # npm build → embed do backend/internal/web/dist → bin/partycockpit
make run        # s .env
make dev        # backend :8098 + vite :5173 (Svelte) a :5174/react/ (React Cockpit), proxy na backend
make check      # go test + shellcheck + svelte-check + tsc/eslint/vitest Reactu + oba buildy — brána
make dist       # party box: dist/partycockpit-linux-{arm64,amd64}.tar.gz
make styles     # přegeneruje katalog stylů z ../Restyler
```

## Mapa
- `backend/cmd/partycockpit` — main: config → store → providery → api.Server
- `backend/internal/api` — HTTP + WS; `jobs.go` worker pool (restyle → klasifikace → pending),
  `music.go` DJfy + tempo, `handlers.go` vše ostatní
- `backend/internal/store` — SQLite (`modernc.org/sqlite`, bez CGO); migrace v `schema.go`
- `backend/internal/api/animate.go` — M4: živé portréty (dědí schválení) a tance (čekají na operátora)
- `backend/internal/providers/animate.go` — Comfy (L40S), Restyler, mock animátor; `workflows/*.json`
- `backend/internal/providers` — Restyler backend, AiStack kontext, mock, WD14 klasifikátor;
  `restyle_styles.json` je generovaný z Restyleru
- `backend/internal/api/streams.go` — živé streamy: registry + WebRTC signalizace (P2P, LAN)
- `backend/internal/api/recap.go`, `preflight.go` — M7: recap + zip, kontroly „Před party“
- `backend/internal/subtitles` + `api/subtitles.go` — titulky přes Translator; `frontend/src/lib/mic.ts` VAD
- `backend/internal/api/box.go`, `deploy/box/` — party box (Pi): /box, párování, systemd, runbook
- `frontend-react/` — druhý Cockpit v React 19 + TS (D21), `/react/cockpit/<kód>`; sdílí `frontend/src/lib` přes `@pc`
- `backend/internal/glyphs` + `frontend/src/lib/handwriting.ts` — ručně psané vzkazy; `api/game.go` + `stage/scenes/Kruh.svelte` hra
- `backend/internal/djfy` — klient DJfy relay (SSE + guest zprávy)
- `backend/internal/gallery` — FineTuneGallery list + disková cache obrázků
- `frontend/src/stage` — scény (`scenes/*.svelte`), overlaye, PixiJS reakce
- `frontend/src/lib/beat.ts` — BeatClock (tempo ze serveru / audio / idle), TapTempo
- `frontend/src/lib/rtc.ts` — Viewer (plátno/Cockpit) a Publisher (telefon); MirrorBooth: `docs/mirrorbooth-stream.md`
- `frontend/src/lib/showqueue.ts` — výběr z show fronty (nové napřed, pak loop/shuffle); `lib/rand.ts` seed scén
- `backend/internal/webtoon` — Toonfork katalog, import zip packů, epizody; `stage/scenes/Strip`, `Bubbles` (D23)
- `backend/internal/stylemap` + `api/stylemaps.go` — StyleMap packy ze session FineTuneGallery (D23)
- `api/show.go`, `showlib.go`, `studio.go` + `frontend/src/studio` — Scene Studio, offline show (`docs/studio.md`)

## Invarianty
- **Nic od hosta na plátno bez schválení.** Stage čte jen `stage_queue` (approved).
  Originál po selhaném restyle se nikdy neschvaluje automaticky.
- Originální fotky (`kind=upload`, `status=source`) se na plátno nedostanou nikdy.
- Živý portrét dědí schválení fotky. Tanec i vlasy ve větru čekají na operátora vždy, i v relaxed režimu.
- Fotky ze Studia (skrytý host) se po restylu schválí samy; jejich originál po selhání ne (D23).
- GPU pro animace jede přes L40S session (ComfyUI). SPARK jen přes broker.
- Živý stream jde na plátno až po „pustit“ v Cockpitu. Server plátnu neposílá pending streamy.
- GPU se nečeká synchronně: upload vrátí 202, job běží ve worker poolu.
- Limit 1 rozpracovaný restyle na hosta.
- ID hosta je jeho tajný klíč: nikdy do fronty, scén ani zpráv pro ostatní (D17).
- Plátno se připojuje jen se `stage_key`; zápisy hostů končí s `ends_at` a mají limity (D19).
- Tokeny a klíče nepatří do URL ani logu; `/files` vydá veřejně jen schválené (D19).
- Repo je veřejné: žádné LAN IP, tokeny, domácí cesty. Vše přes `.env` (`.env.example`).
- Migrace, které už vyšly, se needitují; přidává se nová na konec `migrations`.
- Stage musí přežít výpadek backendu: cache stavu v localStorage, scény jedou dál.

## Odchylky od plánu
Odchylka od node.md: Svelte + Vite bez SvelteKitu a druhý Cockpit v Reactu (D21).
SQLite místo Postgres (přenosný party box), port 8098, restyle přes Restyler backend,
stickery fontem místo HandWrittenStickers, streamy P2P bez LiveKit, animace přímo přes ComfyUI. Detail v `DECISIONS.md` D2, D11–D13.

## Kam dál
`STATUS.md`: ostrý test, M3 MirrorBooth stream, M4 na L40S, M5 3D.
