# PartyCockpit

## Účel
Party show na plátno: Stage (TV), Cockpit (operátor), Guest (telefon přes QR). Plán
`Prompts/00-PARTYKOKPIT_PLAN.md`, odpovědi a odchylky `DECISIONS.md`, stav `STATUS.md`.

## Build a test
```sh
make build      # npm build → embed do backend/internal/web/dist → bin/partycockpit
make run        # s .env
make dev        # backend :8098 + vite :5173 (proxy /api /ws /files /webtoon)
make check      # go vet + go test + svelte-check + vite build — brána před pushem
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
- `backend/internal/djfy` — klient DJfy relay (SSE + guest zprávy)
- `backend/internal/gallery` — FineTuneGallery list + disková cache obrázků
- `frontend/src/stage` — scény (`scenes/*.svelte`), overlaye, PixiJS reakce
- `frontend/src/lib/beat.ts` — BeatClock (tempo ze serveru / audio / idle), TapTempo
- `frontend/src/lib/rtc.ts` — Viewer (plátno/Cockpit) a Publisher (telefon); MirrorBooth: `docs/mirrorbooth-stream.md`
- `frontend/src/lib/showqueue.ts` — výběr z show fronty (nové napřed, pak loop/shuffle)

## Invarianty
- **Nic od hosta na plátno bez schválení.** Stage čte jen `stage_queue` (approved).
  Originál po selhaném restyle se nikdy neschvaluje automaticky.
- Originální fotky (`kind=upload`, `status=source`) se na plátno nedostanou nikdy.
- Živý portrét dědí schválení fotky. Tanec čeká na operátora vždy, i v relaxed režimu.
- GPU pro animace jede přes L40S session (ComfyUI). SPARK jen přes broker.
- Živý stream jde na plátno až po „pustit“ v Cockpitu. Server plátnu neposílá pending streamy.
- GPU se nečeká synchronně: upload vrátí 202, job běží ve worker poolu.
- Limit 1 rozpracovaný restyle na hosta.
- Repo je veřejné: žádné LAN IP, tokeny, domácí cesty. Vše přes `.env` (`.env.example`).
- Migrace, které už vyšly, se needitují; přidává se nová na konec `migrations`.
- Stage musí přežít výpadek backendu: cache stavu v localStorage, scény jedou dál.

## Odchylky od plánu
SQLite místo Postgres (přenosný party box), port 8098, restyle přes Restyler backend,
stickery fontem místo HandWrittenStickers, streamy P2P bez LiveKit, animace přímo přes ComfyUI. Detail v `DECISIONS.md` D2, D11–D13.

## Kam dál
`STATUS.md`: M3 MirrorBooth stream, M4 ověření na L40S, tanec, 3D hry, HandWrittenStickers.
