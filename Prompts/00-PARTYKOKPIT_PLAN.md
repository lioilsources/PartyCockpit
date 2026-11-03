# PARTYKOKPIT_PLAN.md — handoff pro Opus (Director)

> **Party kokpit** = webová aplikace, která spojí Ol1nova artworková díla do jedné
> live show běžící na velkém plátně (TV / projektor). Hosté se zapojují z telefonu
> (QR), obsah se generuje na Sparku, hudba jede z DJfy se sdílenou frontou.
>
> Tento soubor je vstup pro Claude Code Director session. Sekce 9 říká, jak práci
> rozdělit na sub-sessions. Sekce 10 jsou otázky, na které má Olin odpovědět
> před startem (nebo Director rozhodne a zapíše do DECISIONS.md).

---

## 0. Rychlý souhrn

| Co                | Jak                                                                      |
|-------------------|--------------------------------------------------------------------------|
| Výstup            | 1 webová app, 3 role: **Stage** (plátno), **Cockpit** (operátor/VJ), **Guest** (host na telefonu) |
| Repo              | `lioilsources/PartyKokpit` (Go backend + Svelte frontend, monorepo)       |
| Hosting           | JODA: Docker Compose, Caddy, Cloudflare Tunnel → `party.ol1n.com`; GPU joby na Sparku přes AiStack/ComfyUI |
| Realtime          | WebSocket (scene state, fronta, reakce); WebRTC přes self-hosted **LiveKit** pro MirrorBooth streamy |
| Integrace         | FinetuneGallery, Webtoon (Ol1nLLM), MirrorBooth, Ol1nLLM restyle, UGC Factory/FantasyCharacter (3D), Tsumiki/DANCE (tanec), **DJfy** (hudba + sdílený playlist), StyleMap, HandWrittenStickers, ShaderGen, Tlumočník (ASR) |
| První demo (M0–M2)| Stage + Cockpit + slideshow z FinetuneGallery + DJfy "Now playing" + QR portál pro hosty s restyle fotky |

---

## 1. Vize a scénář večera

1. Na plátně běží **slideshow** vybraných obrazů z FinetuneGallery; obrazy "žijí" (mrkání, vlání vlasů — idle animace vygenerované předem).
2. Na plátně je trvale **QR kód** → `party.ol1n.com/join/<party-code>`. Host otevře PWA, nic neinstaluje.
3. Host může:
   - **poslat fotku** → Ol1nLLM ji restyluje (vybraný styl ze StyleMap/FinetuneGallery) → objeví se v Guest Gallery na plátně,
   - **nechat svou postavu tancovat** (2D tanec přes Tsumiki/DANCE pipeline, později 3D model přes UGC Factory/FantasyCharacter + Mixamo-style animace),
   - **přidat skladbu do sdílené fronty / hlasovat** (DJfy, SoundCloud),
   - **napsat vzkaz** → HandWrittenStickers ho přetaví do rukopisné samolepky na Sticker Wall,
   - **streamovat z MirrorBooth** (ufoni, filtry) → PiP mřížka 2×2 (až N×N) na plátně.
4. Operátor (Olin nebo kdokoli s Cockpit linkem) přepíná **scény**, schvaluje obsah, řídí tempo; scény reagují na beat z DJfy.
5. Mezi scénami **Webtoon reader** — auto-scroll strip z Ol1nLLM webtoonu (party příběh, kde postavy = hosté?).
6. Na konci: **rekapitulace** — koláž všeho, co hosté vytvořili, ke stažení (QR).

Klíčová vlastnost: **vše generované na Sparku, nic neodchází ven** (vlastní infrastruktura, žádný cloud API účet).

---

## 2. Co z portfolia se zapojuje (a co Olin zapomněl)

### 2.1 Jádro (Olin vyjmenoval)
| Projekt           | Role v Party kokpitu                                              | Co potřebujeme od projektu |
|-------------------|-------------------------------------------------------------------|----------------------------|
| **FinetuneGallery** | zdroj obrazů pro slideshow, metadata (styl, prompt, LoRA)        | HTTP API: list/filter obrazů, SFW flag, style tag; tagy "animatable" |
| **Webtoon (Ol1nLLM)** | čtečka stripů na plátně, generování party epizody              | API: episode → panels (image + bubble text) |
| **MirrorBooth**   | hosté streamují z telefonu, PiP mřížka                            | Flutter: publish WebRTC do LiveKit room (`party-<code>`) |
| **Ol1nLLM**       | restyle fotek hostů, tanec, 3D                                    | job API (přes AiStack gateway) |
| **UGC Factory / FantasyCharacter** | 2D → textured GLB (TRELLIS), rig + animace       | job: image → GLB (+ animace klip) |
| **DJfy** (přidáno) | hudba z SoundCloud, **sdílená fronta**, now-playing, hlasování   | API: queue read/add/vote, now playing + progress, BPM/beat event |

### 2.2 Co chybělo — doporučuji přidat
| Projekt | Proč do mashupu | Priorita |
|---------|-----------------|----------|
| **StyleMap widget** | "sweep" mezi styly jedné postavy = hotová primitivní animace pro plátno; na Guest telefonu jako style picker prstem | ★★★ |
| **Tsumiki bot (karty) / DANCE_PLAN** | přesně "postavy budou tancovat" — karta single-character dance už existuje; couple karta pro páry na party | ★★★ |
| **HandWrittenStickers** | vzkazy hostů → rukopisné samolepky, Sticker Wall scéna; narozeninová přání | ★★★ |
| **Tlumočník (ASR→MT→TTS)** | mikrofon u pódia → live titulky / shoutouty na plátno; vícejazyčná party; hlasové povely pro Cockpit ("další scéna") | ★★ |
| **ShaderGen / ShaderBattle + MirrorBooth shadery** | generativní audio-reaktivní pozadí mezi scénami, interlude bez GPU jobu | ★★ |
| **Meme shorts** | memečka z fotek hostů jako krátké vertikální klipy do rotace | ★★ |
| **Face avatar app (IP-Adapter / per-user LoRA)** | zachování identity hosta při restyle — bez toho restyle "není on/ona" | ★★ (technicky nutné pro kvalitu) |
| **RealEyes LoRA** | kvalitní oči na restylovaných postavách | ★ (style asset) |
| **Hajánek voice cloning / Storyteller TTS** | hlas "MC" plátna — hlášky, představení hosta, odpočet | ★ |
| **Hand Code (MediaPipe hand poses)** | ovládání scén gestem z parketu (kamera u plátna) — party gimmick | ★ |
| **CONTACT (shared ring physics, Supabase Realtime)** | dvouhráčová minihra na plátně, ovládaná z telefonů — pauza mezi bloky | ★ |
| **SMPL-16 sampler** | drum pad pro hosty → zvuky do mixu (opatrně, chaos) | ☆ |
| **3D tištěné vizitky s QR** | fyzický "join" token na stole místo QR na plátně | ☆ (hardware) |
| **FinetuneGallery SFW/temperature klasifikátor** | **moderace obsahu hostů před zobrazením na plátně** — povinné | ★★★ |
| **Discord bot (Ol1nBot)** | notifikace "party běží / host poslal fotku" do kanálu | ☆ |
| **AILineage** | podepsaný manifest ke každému vygenerovanému artefaktu (hoši si stáhnou "svůj" obrázek s provenance) | ☆ |

Nepatří sem: Běhánek, Papánek, Kačeny, PrincessChess, TerraReality, Předávka, Tlumočník-business část, genealogie.

---

## 3. Architektura

```
                         ┌──────────────────────────────────────┐
   TV / projektor  ◄──── │  STAGE  (Svelte, fullscreen, WebGL)  │◄─── WS scene state
   (Mac Mini M2,         │  scény: Slideshow, Webtoon, PiPWall, │◄─── LiveKit subscribe
    Chrome kiosk)        │  GuestGallery, DanceFloor3D, NowPlay │
                         └──────────────────────────────────────┘
   Operátor (laptop/tablet) ─► COCKPIT (Svelte /cockpit) ─► REST+WS ─┐
   Host (telefon, PWA)      ─► GUEST   (Svelte /join/<code>) ─► REST ─┤
   MirrorBooth (Flutter)    ─► LiveKit publish (WebRTC) ─────────────┤
                                                                     ▼
                      ┌───────────────────────────────────────────────┐
   JODA               │  party-backend (Go)  :8097                    │
   (Docker Compose)   │  - parties, guests, assets, scenes, queue     │
                      │  - WS hub (scene state, events, beat)         │
                      │  - job dispatcher → AiStack gateway           │
                      │  - moderation gate (SFW) → publish            │
                      │  - DJfy adapter, FinetuneGallery adapter,     │
                      │    Webtoon adapter, LiveKit token service     │
                      │  Postgres (sqlc, golang-migrate)              │
                      │  LiveKit server (SFU) + TURN                  │
                      │  MinIO/local FS pro assety                    │
                      └───────────────────────────────────────────────┘
                                                │ jobs (HTTP/queue)
                                                ▼
   DGX Spark          ComfyUI / vLLM přes AiStack:
                      restyle (SDXL/FLUX + IP-Adapter), idle-anim (I2V loop),
                      dance (Tsumiki card), TRELLIS GLB, webtoon panely, stickery
```

### 3.1 Komponenty

- **party-backend (Go)** — jediný stavový server. Doménové objekty: `Party`, `Guest`, `Asset` (upload / generated, stav `pending|approved|rejected`), `Job`, `Scene`, `QueueItem`. WS hub rozesílá `SceneState` a eventy (`asset.approved`, `queue.changed`, `beat`, `stream.joined`).
- **Stage** — jedna Svelte SPA v kiosk režimu. Každá scéna = komponenta s `enter/leave/tick(beat)`. Render: PixiJS pro 2D vrstvy, Three.js pro `DanceFloor3D` (GLB + animace), `<video>` elementy pro LiveKit tracky. Přechody mezi scénami = crossfade/wipe.
- **Cockpit** — stejná SPA, route `/cockpit`, token-chráněná. Seznam scén (drag → playlist večera), moderation inbox (schválit/zamítnout assety hostů), DJfy fronta, "Push to stage" tlačítka, náhled toho, co je na plátně.
- **Guest** — PWA `/join/<code>`, bez loginu (anonymní guest id v localStorage + jméno/přezdívka). Akce: foto → styl → odeslat; vzkaz; přidat skladbu / hlasovat; "chci tancovat" (foto celé postavy); reakce emoji (plují přes plátno).
- **LiveKit** — self-hosted SFU na JODA. Backend vydává tokeny. MirrorBooth = publisher, Stage = subscriber. PiP mřížka se sama přepočítá podle počtu tracků (1, 2×2, 3×3).
- **Moderation gate** — každý asset od hosta jde přes SFW klasifikátor (FinetuneGallery UC); `auto-approve` když score < práh a party je v režimu `relaxed`, jinak do Cockpit inboxu. **Nic od hosta se nedostane na plátno bez průchodu gatem.**
- **Job dispatcher** — mapuje "intent" (restyle / idle-anim / dance / 3d / sticker / webtoon) na konkrétní AiStack job. Priority: restyle (sekundy) > sticker > idle-anim > dance > 3D (minuty). Stage ukazuje "connecting…" teaser s originální fotkou, než dorazí výsledek.

### 3.2 Hudba a beat (DJfy)
- DJfy zůstává samostatná app; Party kokpit ji **nepřehrává**, jen čte stav a zapisuje do fronty.
- Adapter: `GET /now-playing` (track, artist, artwork, position, duration, bpm?), `GET/POST /queue`, `POST /queue/{id}/vote`.
- Beat: pokud DJfy neumí BPM/beat event, Stage si ho spočítá sama **Web Audio API** z loopback vstupu (BlackHole na Mac Mini) — `AnalyserNode` → onset detection → `beat` event lokálně. To je fallback, který funguje vždy.
- Scény dostávají `tick(beat, energy)` — slideshow přepíná na beat, PiP rámečky pulzují, 3D tanečníci drží tempo.

### 3.3 Síť — důležitá past
Cloudflare Tunnel přenese HTTP/WS, **ne UDP pro WebRTC**. Řešení:
1. Party je fyzicky v jednom místě → telefony i Stage na **stejné LAN** (MikroTik), LiveKit na JODA, Guest/Stage URL na LAN IP nebo `party.local`; QR vede na LAN URL + fallback na `party.ol1n.com` pro ne-stream funkce.
2. Pro vzdálené streamery: coturn (TURN over TCP 443) na JODA s port-forwardem, nebo Cloudflare Calls. **M3 rozhodne**, defaultně LAN-only.

### 3.4 Idle animace obrazů (mrkání, vlání vlasů)
- Předgenerované, ne live: pro každý "animatable" obraz z FinetuneGallery vyrobí Spark 3–5 s **smyčkový klip** (image-to-video s nízkým motion, seamless loop přes crossfade posledních 10 snímků) + uloží jako WebM/MP4 vedle originálu.
- Stage přehrává klip místo statiky; fallback na statický obraz + Ken Burns.
- Batch job přes noc, Cockpit ukáže procento zpracovaných.

### 3.5 Tanec a 3D
- **2D tanec (M4):** foto hosta (celá postava) → Tsumiki "dance" karta (driving video z knihovny tanců, výběr v Guest UI) → MP4 → `DanceFloor2D` scéna (více tanečníků vedle sebe, synchronizace na beat posunem playback rate ±10 %).
- **3D (M5):** foto → restyle do game-ready stylu → TRELLIS GLB (UGC Factory job) → auto-rig (Mixamo-style / open-source rig) → animace z katalogu → Stage načte GLB + anim clip do Three.js. UV/texture mapping tak, jak ho dělá UGC studio. Pokud auto-rig selže, fallback = 2D tanec.

---

## 4. Datový model (Postgres, sqlc)

```sql
parties(id, code, name, mode enum('strict','relaxed'), created_at, ends_at)
guests(id, party_id, nickname, device_hash, created_at)
assets(id, party_id, guest_id null, kind enum('upload','restyle','idle_anim','dance','model3d','sticker','webtoon_panel'),
       src_asset_id null, url, thumb_url, meta jsonb, sfw_score, status enum('pending','approved','rejected'), created_at)
jobs(id, party_id, asset_id, intent, provider, external_id, status, error, started_at, finished_at)
scenes(id, party_id, kind, params jsonb, position)       -- playlist večera
scene_state(party_id pk, active_scene_id, started_at, params jsonb)   -- co je teď na plátně
queue_items(id, party_id, provider enum('djfy'), external_id, title, artist, votes, added_by, status)
reactions(id, party_id, guest_id, emoji, created_at)      -- TTL 1 h
```

Assety fyzicky: `/data/party/<code>/<asset_id>.<ext>`, servírované Caddy s range requests.

---

## 5. API (návrh, OpenAPI v repu)

```
POST /api/parties                       (cockpit)  → {code, cockpit_token}
GET  /api/parties/{code}                (guest)    → public info, active scene
POST /api/parties/{code}/guests         → {guest_id}
POST /api/parties/{code}/assets         multipart (image) + intent + style_id → {asset_id, job_id}
GET  /api/parties/{code}/assets?status=approved&kind=...
POST /api/parties/{code}/assets/{id}/approve|reject   (cockpit)
GET  /api/parties/{code}/queue ; POST .../queue ; POST .../queue/{id}/vote
POST /api/parties/{code}/reactions
GET  /api/parties/{code}/livekit-token?role=publisher|subscriber
POST /api/parties/{code}/scene          (cockpit) {scene_id | kind, params}
WS   /ws/parties/{code}?role=stage|cockpit|guest
       ← scene.state, asset.approved, queue.changed, reaction, beat, stream.joined/left
GET  /api/styles                        → z FinetuneGallery/StyleMap (id, name, thumb)
GET  /api/gallery?tag=slideshow         → FinetuneGallery adapter
GET  /api/webtoon/episodes/{id}         → Webtoon adapter
```

Job providery (interface `Provider` v Go): `restyle`, `idle_anim`, `dance`, `model3d`, `sticker`, `webtoon` — každý jen obálka nad existující službou (AiStack gateway / comfyui.ol1n.com / ugc-backend :8095 / HandWrittenStickers Go extractor).

---

## 6. Scény (Stage)

| Scéna            | Zdroj                             | Beat-reaktivita                     | Milník |
|------------------|-----------------------------------|-------------------------------------|--------|
| `Slideshow`      | FinetuneGallery (tag/style filter)| přepnutí na beat, Ken Burns         | M0 |
| `NowPlaying`     | DJfy                              | artwork pulz, progress bar, fronta + hlasy | M2 |
| `GuestGallery`   | approved restyle assety           | nové přibývají s "pop" animací      | M1 |
| `LivingPictures` | idle_anim klipy                   | —                                   | M4 |
| `PiPWall`        | LiveKit tracky (MirrorBooth)      | rámečky pulzují                     | M3 |
| `DanceFloor2D`   | dance klipy hostů                 | playback rate k tempu               | M4 |
| `DanceFloor3D`   | GLB + anim (Three.js)             | anim speed k tempu, světla          | M5 |
| `Webtoon`        | Webtoon adapter                   | auto-scroll rychlost                | M6 |
| `StickerWall`    | sticker assety + reakce emoji     | —                                   | M1b |
| `ShaderInterlude`| ShaderGen/MirrorBooth shadery     | plně audio-reaktivní (energy)       | M7 |
| `Subtitles`overlay| Tlumočník ASR                    | — (overlay přes libovolnou scénu)   | M7 |
| `Recap`          | vše approved z party              | —                                   | M7 |

Overlays (vždy nad scénou): QR + join kód, reakce emoji plující zdola, "Now playing" mini-lišta.

---

## 7. Milníky

**M0 — Skeleton (1 sub-session)**
- monorepo, Go backend + Postgres + migrace, WS hub, Svelte app se 3 routes, Docker Compose na JODA, Caddy + tunnel `party.ol1n.com`
- scény `Slideshow` (FinetuneGallery adapter) + přepínání z Cockpitu + QR overlay
- ✅ DoD: Stage na TV, Cockpit přepne scénu, slideshow jede z reálné galerie

**M1 — Guest portál + restyle**
- PWA `/join`, upload fotky, výběr stylu, restyle job přes AiStack (SDXL + IP-Adapter z Face avatar práce), moderation gate (SFW klasifikátor), `GuestGallery`, reakce emoji
- M1b: vzkaz → HandWrittenStickers → `StickerWall`
- ✅ DoD: host pošle selfie, do 30 s je restylovaná na plátně (po schválení)

**M2 — DJfy**
- DJfy adapter (now-playing, queue, vote), Guest "přidat skladbu", `NowPlaying` scéna, beat detekce (DJfy event nebo Web Audio fallback), `tick(beat)` do všech scén
- ✅ DoD: host přidá track, vidí ho na plátně ve frontě, slideshow švihá na beat

**M3 — MirrorBooth streamy**
- LiveKit na JODA, token service, MirrorBooth Flutter: tlačítko "Stream to party" (livekit_client), `PiPWall` 1 → 2×2 → 3×3, LAN setup + QR na LAN URL
- ✅ DoD: 4 telefony s MirrorBooth filtry v mřížce, latence < 1 s na LAN

**M4 — Oživení: idle animace + 2D tanec**
- batch I2V loop job pro galerii, `LivingPictures`; Guest "chci tancovat" → Tsumiki dance karta → `DanceFloor2D`
- ✅ DoD: galerie mrká/vlaje, host tančí na plátně do ~2 min

**M5 — 3D tanečníci**
- foto → TRELLIS GLB (ugc-backend) → rig → animace → Three.js `DanceFloor3D`, rotace, světla, fallback na 2D
- ✅ DoD: alespoň 1 host jako 3D postavička tančí v tempu

**M6 — Webtoon**
- `Webtoon` reader scéna, volitelně "party epizoda" s hosty jako postavami (character library Ol1nLLM)
- ✅ DoD: strip auto-scroluje mezi bloky

**M7 — Show polish**
- `ShaderInterlude`, `Subtitles` (Tlumočník), `Recap` + download QR, runbook večera (checklist: síť, audio loopback, Spark prewarm, Cockpit na tabletu), StyleMap sweep jako přechod mezi scénami

---

## 8. Nefunkční požadavky

- **Latence restyle < 30 s** p95 (prewarm modelů na Sparku při startu party, `POST /api/parties` spustí warmup).
- **Stage musí přežít výpadek backendu** 60 s (cache posledního scene state, lokální playlist assetů).
- **Moderace**: `strict` mode = vše ručně; `relaxed` = auto-approve pod prahem. Default `strict`.
- **Soukromí**: fotky hostů jen pro danou party, mazání po `ends_at + 7 dní` cron; žádné odesílání mimo JODA/Spark.
- **Kiosk**: Stage bez kurzoru, bez scrollbarů, 1080p/4K, 60 fps na Mac Mini M2 v Chrome.
- **Observabilita**: `/healthz`, Prometheus metriky (jobs by intent, latency), status.ol1n.com probe.

---

## 9. Rozdělení pro Director → sub-sessions

| Sub-session | Vstup | Výstup |
|-------------|-------|--------|
| `backend` | tento plán §3–5 | Go service, migrace, WS hub, Provider interface + mock providery, Compose |
| `stage` | §3.1, §6 | Svelte Stage + scény M0–M2 + overlay vrstva, kiosk build |
| `cockpit-guest` | §3.1, §5 | Cockpit UI, Guest PWA, moderation inbox |
| `providers` | §3.5, §5 + repa AiStack, ugc-backend, HandWrittenStickers, FinetuneGallery | reálné implementace Provider (restyle, sticker, idle_anim, dance, model3d, webtoon) |
| `djfy-adapter` | DJFY_PLAN.md + repo DJfy | API v DJfy (now-playing, queue, vote, beat), adapter v backendu |
| `mirrorbooth-stream` | MirrorBooth repo | livekit_client publish, UI tlačítko, LAN discovery |
| `livekit-infra` | §3.3 | LiveKit + coturn Compose, token service, LAN/tunnel runbook |
| `moderation` | FinetuneGallery SFW UC | klasifikátor jako HTTP služba, práh, test set |

Pořadí: `backend` + `stage` paralelně (M0) → `cockpit-guest` + `providers` (M1) → `djfy-adapter` (M2) → `livekit-infra` + `mirrorbooth-stream` (M3) → zbytek. Každá sub-session zapisuje do `agent-hub/partykokpit/<session>.md` stav + otevřené otázky; Director je slučuje do `STATUS.md`.

Před prvním commitem Director ověří reálný stav repozitářů (FinetuneGallery, DJfy, MirrorBooth, ugc-backend, AiStack, HandWrittenStickers) a aktualizuje §5 podle skutečných endpointů — **nepředpokládat API z tohoto dokumentu jako hotové**.

---

## 10. Otevřené otázky pro Olina

1. **Frontend stack**: Svelte (jako FinetuneGallery) + PixiJS/Three.js, nebo chceš Flutter Web, abys sdílel kód s mobilními appkami? (Doporučuji Svelte — výkon na plátně, Three.js ekosystém.)
2. **Party je vždy on-site (LAN)**, nebo i vzdálení hosté/streamery? Rozhoduje o M3 (TURN vs LAN-only).
3. **DJfy** umí dnes exponovat now-playing/queue přes HTTP, nebo to je čistě mobilní app? Pokud ne, přidáme do DJfy malý lokální server, nebo DJfy pojede jako web build na Mac Mini?
4. **Audio výstup**: DJfy hraje přes Sonos/SoundCloud na jakém zařízení? Potřebuju vědět, kde udělat loopback pro beat detekci (BlackHole na Mac Mini = nejjednodušší, pokud hraje Mac Mini).
5. **Moderace**: kdo je operátor během party? Když nikdo (Olin tancuje), musí být `relaxed` auto-approve dost dobrý — jaký práh?
6. **3D rig**: máš už v UGC Factory nějaký auto-rig/animaci, nebo je to čistě TRELLIS static GLB? Rozhoduje, jestli M5 je týden nebo měsíc.
7. **Webtoon**: má na party vzniknout živá epizoda s hosty, nebo jen čtečka hotových stripů? (Živá = další job pipeline.)
8. **Název repa / produktu**: `PartyKokpit`, `PartyCockpit`, nebo něco z Ananas&Bananas brandu?
9. **Hardware na plátno**: Mac Mini M2 (kiosk Chrome) je OK? Nebo Raspberry/TV browser — pak Three.js scény škrtáme.
10. Chceš **Ol1n Pass / monetizaci** (party jako placená služba pro bary — viz RattleRamp u Chřestýše, PR agentury)? Ovlivní multi-tenant model už v M0 (parties už je multi-tenant, ale billing ne).

---

## 11. Rizika

| Riziko | Dopad | Mitigace |
|--------|-------|----------|
| WebRTC přes Cloudflare Tunnel nefunguje | PiPWall mrtvá mimo LAN | LAN-first design, TURN jako M3 volitelně |
| GPU fronta se při 30 hostech ucpe | restyle trvá minuty | prioritní fronta, prewarm, limit 1 aktivní job/host, teaser místo spinneru |
| NSFW na plátně | katastrofa společensky | strict default, klasifikátor, Cockpit "panic" tlačítko = skok na Slideshow |
| 3D rig nespolehlivý | M5 se vleče | fallback 2D tanec, M5 až po M6 |
| Beat detekce z loopbacku šumí | scény trhané | smoothing, BPM z DJfy metadat jako primár |
| Příliš mnoho integrací najednou | nic nedotažené | M0–M2 je samostatně použitelná show; zbytek přidávat, jen když předchozí milník DoD prošel |
