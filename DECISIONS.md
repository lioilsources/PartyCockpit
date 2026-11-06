# DECISIONS.md

Rozhodnutí k `Prompts/00-PARTYKOKPIT_PLAN.md` §10 (odpovědi Olina 2026-11-03) a co z nich
plyne pro implementaci. Pozdější rozhodnutí přidávej na konec s datem.

## D1 — Frontend stack: Svelte 5 + Vite + PixiJS / Three.js
Jedna SPA (`frontend/`), tři role podle cesty: `/stage`, `/cockpit`, `/join/<code>`.
PixiJS pro 2D scény (slideshow, galerie, stickery, emoji), Three.js až pro 3D hry/tanečníky.
Žádný SvelteKit — build je statický a Go binárka ho embeduje (`backend/web/dist`).

## D2 — Party je vždy on-site (LAN)
- Žádný TURN / Cloudflare Calls. WebRTC (LiveKit, M3) jen na LAN.
- QR na plátně vede na **LAN URL** (`PC_PUBLIC_URL`, např. `http://<LAN-IP-party-boxu>:8098`).
- Protože party nemusí být u JODA, backend je **jedna přenosná Go binárka** s vestavěným
  frontendem a **SQLite** (pure-Go `modernc.org/sqlite`, bez CGO) místo Postgres — pustí se
  na JODA (Compose) i na Mac Mini / Windows mini PC na místě. Odchylka od plánu §4 (Postgres).
- GPU joby jdou na Spark přes síť (AiStack/ComfyUI); když Spark není dosažitelný, providery
  spadnou do `failed` a host vidí originál.

## D3 — DJfy: existující HTTP endpoint (na JODA, s QR)
PartyCockpit DJfy nepřehrává, jen čte now-playing/frontu a přidává do ní. Adapter
`backend/internal/djfy` mluví s existujícím DJfy serverem (`DJFY_URL`). Viz poznámky
k reálným endpointům v `backend/internal/djfy/client.go`.

## D4 — Audio výstup: Apple TV / Mac Mini / Windows mini PC (HDMI)
- Stage běží v prohlížeči na Mac Mini nebo Windows mini PC (HDMI do TV). Apple TV nemá
  prohlížeč → jen jako AirPlay cíl z Macu.
- Beat: (1) BPM z DJfy metadat, pokud je; (2) Web Audio z audio vstupu Stage stroje
  (BlackHole na Macu, VB-Cable/Stereo Mix na Windows, nebo mikrofon když hraje Apple TV);
  (3) **tap tempo** v Cockpitu. Stage si vybírá to nejlepší dostupné.

## D5 — Moderace: pre-play approval + smyčková fronta
- Vše od hostů → `pending` → operátor schválí → asset jde do **show fronty** (`stage_queue`).
- Stage hraje frontu postupně; když doběhne, **rotuje od začátku** nebo **shuffle**
  (`parties.loop_mode = 'loop'|'shuffle'`). Olin tedy schválí várku a jde tancovat.
- Nic se nedostane na plátno bez schválení. `relaxed` (auto-approve pod SFW prahem) je
  implementovaný, ale default je `strict`; práh `SFW_AUTO_APPROVE_MAX=0.15` (NSFW skóre 0–1),
  použije se jen když běží reálný klasifikátor.
- Panic tlačítko v Cockpitu = okamžitý skok na Slideshow + vyčištění overlaye.

## D6 — 3D / hry o přestávce (zatím ne priorita)
- Hráče vybírá **losování** z hostů, kteří se přihlásili („chci hrát“); los umí N hráčů
  (co-op, např. Kiriana). Implementováno jako scéna `Lottery` + API; samotné hry později.

## D7 — Webtoon: hotové divadelní hry + náměty od hostů
- Scéna `Webtoon` přehrává předpřipravené epizody (auto-scroll).
- Hosté posílají **náměty** (`ideas`) z telefonu; Cockpit je vidí a exportuje (JSON) jako
  vstup pro pozdější stripy. Živá generace epizody na party se nedělá.

## D8 — Repo: `PartyCockpit`, veřejné na GitHubu
- Žádná tajemství v repu: vše přes env (`.env.example`), žádné interní hostname natvrdo
  kromě defaultů v příkladech. Licence MIT.

## D9 — Hardware plátna: prohlížeč na Mac Mini / Windows mini PC (viz D4)
Three.js scény zůstávají ve hře. Kiosk: Chrome `--kiosk`, bez kurzoru.

## D10 — Monetizace
Odpověď: „B2B služba nebude to Ol1nPass mobilní appka“. **Výklad k potvrzení:** party jako
B2B služba se počítá, ale ne přes Ol1nPass mobilní appku. Dopad na kód teď: `parties` jsou
multi-tenant (každá má vlastní kód + cockpit token), billing ani Ol1nPass integrace se nedělá.

## D11 — Implementace M0–M2 (2026-11-03)
- **Port 8098**: 8097 drží `djfy-relay` (a `finetune-wd14` na SPARKu).
- **Restyle = Restyler backend** (`RESTYLE_PROVIDER=restyler`): `POST /api/render` s jedním
  krokem `restyle_flux` (drží identitu obličeje), prompt = medium věta + blok stylu přesně
  jako Restyler (`restylePrompt`). Katalog 90 stylů je převzatý z Restyleru
  (`make styles` ho přegeneruje). Záloha: AiStack flux-kontext (`kontext`), offline `mock`.
- **Když restyle selže**, host vidí originál a Cockpit dostane originál ke schválení —
  nikdy se neschvaluje automaticky, ani v `relaxed`.
- **DJfy**: relay nemá REST frontu, PartyCockpit se k ní připojuje jako host přes SSE
  (`/events?guest=`) a zprávy (`/messages`). Každý host PartyCockpitu má v DJfy vlastní
  `guestId`, takže platí limity DJfy na hosta. Hledání jde přes telefon s DJfy (relay
  SoundCloud nevolá) — když je telefon offline, hledání nejde.
- **Tempo**: tap tempo v Cockpitu > BPM z DJfy `nowPlaying.bpm` > audio vstup Stage
  (`/stage/<code>?audio=1`) > 60 bpm idle.
- **Sticker Wall**: HandWrittenStickers nemá HTTP API → vzkazy se kreslí fontem Caveat.
  Napojení na glyph extraktor je samostatný úkol.
- **FineTuneGallery** je za Cloudflare Access: backend posílá `CF_ACCESS_CLIENT_*`;
  service token musí být povolený v Access aplikaci finetune. Slideshow bere jen
  `taxo.band == "safe"` (bez taxonomie nic, pokud není `FINETUNE_ALLOW_UNCLASSIFIED=1`).
- **Webtoon** epizody = složky `data/webtoon/<id>/` s panely v pořadí názvů + `title.txt`.

## D12: M3 živé streamy bez LiveKit (2026-11-03)
- **Bez SFU.** Party je vždy v LAN (D2) a plátno má nejvýš 9 dlaždic, proto každý divák
  (plátno, náhled v Cockpitu) otevírá peer-to-peer WebRTC spojení na každý telefon.
  PartyCockpit jen přeposílá signalizaci po party WebSocketu (`api/streams.go`).
  Plán §3.1 počítal s LiveKit, ale ten by byl další server a s ním i TURN. Přenosná
  jedna binárka (D2) tím zůstává.
- **Moderace.** Stream začíná jako `pending` a na plátno jde, až ho operátor pustí.
  Cockpit vidí náhled i nepuštěných streamů. Server neposílá offer plátna na pending
  stream. PANIC vrátí všechny streamy do `pending`.
- **Telefon se ověřuje klíčem streamu** (`role=streamer&key=…`), ne ID hosta. ID hostů
  jsou vidět v datech plátna, takže by šla zneužít.
- **Prohlížeč potřebuje https kvůli kameře.** Plain http na LAN IP kameru nepovolí. Řešení:
  `PC_TLS_CERT`/`PC_TLS_KEY` (certifikát pro jméno, které v LAN vede na party box) a
  `PC_PUBLIC_URL=https://…`. MirrorBooth jako nativní appka https nepotřebuje.
  Filtry v prohlížeči jsou kreslené na canvasu (zrcadlo, kaleido, ufon, pop).
- **ICE.** Default `PC_ICE_SERVERS=[]` (host kandidáti). Plátno bez mikrofonu posílá jen
  mDNS kandidáty, ale telefon s kamerou nebo MirrorBooth posílá skutečné IP, a to ke
  spojení stačí. Kiosk Chrome přesto pouštěj s
  `--disable-features=WebRtcHideLocalIpsWithMdns`.
- **MirrorBooth** dělá vlastní session podle `docs/mirrorbooth-stream.md`. Klíčový úkol je
  dostat do WebRTC výstup shaderu, protože dnes posílá surovou kameru.

## D13: M4 živé portréty a tanec hostů (2026-11-03)
- **GPU jede na L40S** (pokyn uživatele 2026-11-03: „GPU jed pres @L40S“). PartyCockpit
  posílá úlohy přímo do ComfyUI (`ANIMATE_PROVIDER=comfy`, `COMFY_URL`). Restyler
  `live_portrait`/`wan_motion` je zatím jen na nemergnuté větvi a nikdy neběžel, proto
  je jen jako druhá cesta (`ANIMATE_PROVIDER=restyler`, umí jen živý portrét).
  Offline se používá `mock` (animovaný GIF).
- **Živý portrét** (LivePortrait, driver `alive`: mrknutí, pohled kolem, úsměv) jen
  pohne obličejem na fotce, kterou operátor už schválil. Proto **dědí schválení**
  a hraje v galerii místo statické fotky (`Asset.live`). Jako samostatná položka se do
  show fronty nedostane. Zapíná se v Cockpitu („oživovat schválené fotky“) nebo
  jednorázově tlačítkem „✨ Oživit galerii“.
- **Tanec** (Wan 2.2 I2V, 4 předvolby) je generativní, takže vždy čeká na operátora,
  ani v relaxed režimu se automaticky neschvaluje. Po schválení jde do show fronty
  a hraje ve scéně „💃 Tančírna“ (až 4 vedle sebe, rychlost videa podle tempa ±10 %).
- Animace mají vlastní worker (`PC_ANIM_WORKERS=1`), takže tanec nezdrží restyle.
  Do limitu „1 rozpracovaný úkol na hosta“ se počítá jen to, co host spustil sám.
- **Soukromí:** fotky hostů jdou na L40S (T-Mobile sandbox, není náš stroj). ComfyUI
  tam poslouchá jen na 127.0.0.1 a je dostupné přes VPN a SSH tunel. Party box mimo
  domov potřebuje OpenVPN profil a tunel.
- **Cropper** pro LivePortrait je FaceAlignment + BlazeFace (`LivePortraitLoadFaceAlignmentCropper`).
  InsightFace je nekomerční a PartyCockpit může být B2B (D10). MediaPipe cropper na L40S
  nejde: node chce staré `mediapipe.framework` API a downgrade by rozbil sdílený venv.
  Licence: face_alignment BSD-3, váhy BlazeFace Apache-2.0. **Otevřené:** licence vah
  LivePortrait (KwaiVGI) včetně `landmark.onnx` pro B2B zatím nikdo neprověřil.
- Ověřeno na L40S 2026-11-03: tanec 480×832×81 (5 s, 16 fps) za 46 s na volné GPU, ~40 GB
  VRAM; živý portrét 10 s (252 snímků, 25 fps) za 29 s (první běh 104 s), ~2 GB VRAM.
  Kvalita: disco a skákání dobré, hip-hop hlavně ruce, robot slabý (prompt zpřesněn, znovu
  neověřeno). Wan workflow ukládá přes VHS_VideoCombine (mp4 h264).
- ComfyUI na L40S běží jen v profilu D a GPU je sdílená (AlphaZero, Webtoon). Na party je
  potřeba domluvit okno v L40S session. Patch v `ComfyUI-LivePortraitKJ`
  (`face_alignment/api.py`, relativní import) se při `git pull` nodu ztratí.

## D14: M7 vyladění show (2026-11-04)
- **Recap** = přesně show fronta (schválené položky). Host si z `/recap/<kód>` stáhne jen
  to, co už bylo na plátně: fotky, tance, živé portréty a vzkazy, jednotlivě nebo jako zip.
  Přístup je stejný jako u `/queue`: stačí kód party. Stránka hlásí datum smazání
  (konec party + 7 dní, janitor). Vlastní fotky řadí napřed podle přezdívky v telefonu.
  To je jen pohodlí, ne řízení přístupu, protože přezdívky nejsou unikátní.
- Show fronta už neposílá `meta.original` (jméno souboru originální fotky). Plátno ho
  nepotřebovalo a s kódem party šlo originál stáhnout přes `/files`.
- **Shader přestávka** (`interlude`): 5 vlastních fragment shaderů (WebGL1, plazma, tunel,
  kaleidoskop, ekvalizér, polární záře). Pulzují s BeatClockem a energií zvuku plátna.
  Bez zvuku odvozují energii z dob. Renderuje se na 0,6 rozlišení kvůli mini PC (`scale`).
  Shadery z MirrorBooth/ShaderGen se nepřebíraly: mají jiné uniformy a vstup z kamery.
- **Před party** v Cockpitu: živé kontroly ze serveru (`GET /preflight`: adresa v QR,
  https, plátno, ping na Restyler a ComfyUI, poslední restyly, DJfy, tempo, moderace,
  fronta ke schválení, disk, konec party) a ruční checklist v localStorage tabletu.
  Ping na Restyler nesleduje přesměrování: login Cloudflare Access znamená špatný token.
- Titulky a StyleMap sweep doplněny 2026-11-04, viz D15.

## D15: M7 titulky a StyleMap sweep (2026-11-04)
- **Titulky** jdou z mikrofonu moderátora v Cockpitu, ne z telefonů hostů, takže je
  operátor neschvaluje. PANIC mikrofon vypne a lištu smaže. Operátor může titulek
  i napsat (oznámení, vzkaz). To funguje i bez Translatoru.
- Cesta: prohlížeč dělá VAD (energie, 700 ms ticha, nejvýš 12 s) a PCM16 16 kHz
  posílá na `/ws/parties/<kód>/mic`. Backend to přeposílá na Translator
  (`TRANSLATOR_URL`, WS protokol v1, režim `streaming`, bez TTS, mluvčí vždy `a`).
  Token Translatoru tak nikdy nejde do prohlížeče. Na plátno jde průběžný přepis,
  konečný přepis a překlad (`langA → langB`, nebo bez překladu).
- Mikrofon v prohlížeči jde jen přes https nebo localhost. Cockpit na tabletu proto
  potřebuje https stejně jako kamera v telefonu (D12).
- Translator ani jeho ASR (SPARK, jen přes broker) nikdo z PartyCockpitu nespouští.
  „Před party“ jen hlídá jeho `/healthz`.
- **StyleMap sweep**: balíčky z FineTuneGallery `/stylemaps/` (staví je Ol1nLLM
  `tools/stylemap`) jdou přes náš proxy s diskovou cache. Manifesty se obnovují po
  10 min, obrázky po dni, a při výpadku se servíruje starší kopie. Přehrává se
  pořadí `tour` (nejmenší kroky LPIPS), jinak Hilbertova křivka. Jako scéna
  `stylesweep` běží ve smyčce, jako přechod (`params.transition`) hraje 2,4 s
  z náhodného místa přes nově zapnutou scénu.
- Balíčky neprocházejí taxonomickým filtrem jako slideshow. Na plátno se dostanou
  jen ty, které vybere operátor, a vznikají z vlastních sad uživatele.

## D16: Ručně psané vzkazy z HandWrittenStickers (2026-11-04)
- Místo HTTP služby PartyCockpit čte sady písmen přímo z `assets/glyphs` aplikace
  (`PC_GLYPHS_DIR`) a servíruje je na `/glyphs/<id>/…`. Host vybírá z písem kategorií
  handwriting a kids. Sady s `"bundle": false` (nejasná licence) a materiálová písmena
  z AI se nenabízejí. Vzkaz si pamatuje sadu v `meta.sheet`.
- Plátno skládá text v prohlížeči (`lib/handwriting.ts`). Je to port `GlyphRenderer`
  z appky bez kurzivního spojování: účaří, proporcionální posun, varianty písmen bez
  opakování, diakritika z písmene a akcentu, záložní sada a deterministické kolísání.
  Emoji, které sada nemá, se dokreslí jako text. Inkoust se tónuje do barvy textu,
  barevné sady zůstávají. Bez `PC_GLYPHS_DIR` se píše fontem Caveat jako dřív (D11).
- Fontové sady jsou OFL. Laurinka je sken skutečné ruky: v repu PartyCockpitu není,
  čte se jen z disku party boxu.

## D17: Hra o přestávce „Kruh“ (2026-11-04)
- Kruhový pong pro 2–8 vylosovaných: každý brání svůj oblouk pálkou, kterou táhne
  prstem na telefonu. Má 3 životy, vyhrává poslední. Míček zrychluje na dobu a od
  5 hráčů jsou míčky dva. Vlastní návrh, žádná existující hra.
- Hru počítá plátno. Server jen ví, kdo má který slot (`POST /game` bere poslední
  losování), a pouští dál vstupy jen z telefonů hráčů (0..1, nejvýš 25×/s).
  Telefon, který 6 s nic nepošle, převezme pomalý autopilot (🤖), aby jeden vybitý
  telefon nezastavil hru. PANIC hru ukončí.
- **Bezpečnost:** ID hosta je jeho tajný klíč (`/guests/{gid}/…`). Show fronta
  a losování ho dřív posílaly všem s kódem party, takže šlo vystupovat jako jiný host
  a vidět jeho originály. Teď fronta nese jen jednosměrný tag (`g` + sha256), losování
  jen přezdívky a „vyhrál jsi“ jde jen na telefon výherce. Telefon se socketu
  představí přes `?guest=`, server ID ověří.

## D18: Party epizoda webtoonu (2026-11-04)
- Bez GPU: epizoda „🎉 Party“ ve čtečce webtoonu poskládá panely ze schválených fotek
  v pořadí show. Bublina panelu je schválený vzkaz téhož hosta (jeho písmem), jinak
  jen popisek vypravěče. Náměty se nepoužívají, protože se neschvalují.
- Generovaná epizoda s hosty jako postavami (Ol1nLLM character library) zůstává na
  později. Potřebuje GPU a vlastní pipeline.

## D19: Bezpečnostní průchod (2026-11-05)
Plán a nálezy: `docs/SECURITY_PLAN.md`. Model hrozby: cizí telefony v síti party,
které znají kód z plátna.
- **Plátno má klíč** (`stage_key`, migrace 3). Bez něj šlo s kódem party otevřít
  živé kamery hostů. Klíč nese jen odkaz z Cockpitu/Home; plátno si ho uloží
  a z URL ho smaže.
- **Party končí**: zápisy hostů a přidání nového hosta po `ends_at` vrací 410.
  Čtení a recap běží dál. Cockpit může prodloužit o 2 h (nejvýš +24 h od teď).
- **Limity**: 400 hostů, 200 vzkazů a 100 námětů na hosta, 20 vzkazů/min,
  token bucket na IP pro zápisy bez tokenu (30/10 s, zip recapu stojí 10),
  8 socketů hosta na IP, mikrofon 50 rámců/s, vstup hry 30/s. Cockpit limity
  nemá. Za reverzní proxy `PC_TRUST_PROXY=1`, jinak by všichni měli IP proxy.
- **Tokeny mimo URL**: sockety Cockpitu a mikrofonu berou jednorázový ticket
  na 30 s, token jde v `#token=` a stahování námětů přes `fetch`. `?token=` se
  kvůli starým odkazům ještě jednou přečte.
- **Seznam party** (obsahuje cockpit tokeny) vyžaduje admin token vždy. Bez
  `PC_ADMIN_TOKEN` se nevydá vůbec, zakládat jde dál (domácí použití).
- **Soubory**: `/files` vydá jen schválené soubory a stilly schválených tanců.
  Cockpit čekající soubory otevírá odvozeným `fileKey` (HMAC z tokenu, jen na
  čtení souborů), host svoje přes `/guests/{gid}/files/`. Originály zůstávají
  soukromé i po schválení restylu.
- Sockety odmítají cizí origin; upload nad 36 Mpx se odmítne před dekódováním.
- **Hlavičky**: nosniff, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`.
  CSP je zatím jen `Report-Only`: na skutečném plátně (Pixi, WebGL, AudioWorklet
  z blob URL) ji nikdo neověřil. Po ověření stačí změnit název hlavičky.
- **Hygiena**: 500 hostovi jen obecná hláška (detail v logu), chyby providerů
  host vidí obecně a Cockpit celé, `clean()` maže formátovací znaky (zero-width,
  bidi), stejná přezdívka → 409 s návrhem („Ema 2“), cockpit token jde vyměnit.
- Nechává se: `/metrics` a `/healthz` veřejné (jen agregáty), plain http v domácí
  LAN (preflight varuje). Pro B2B: `PC_TLS_*` a `PC_ADMIN_TOKEN` povinně.

## D20: Party box na Raspberry Pi (2026-11-06)
Zadání a posouzení: `Prompts/02-PARTY-BOX-PI.md`, runbook `deploy/box/README.md`.
- K TV jde jen box (Raspberry Pi 5 8 GB nebo mini PC N100): backend i plátno
  v kiosku (Chromium pod `cage` na tty1, systemd). Ovládání jen z telefonů.
- **Utáhne to Pi 5 s limity**: 2D scény, shadery a backend ano; Pi nemá
  hardwarový dekodér H.264, proto nejvýš 4 živé streamy (`PC_MAX_LIVE`) a klipy
  jen v 480×832. Výstup 1080p, shadery na 0,5 rozlišení (`PC_STAGE_SCALE`).
  Mini PC N100 nemá tyhle limity, stejný balíček (amd64) a stejný runbook.
- **Bez klávesnice**: `/box` vydá klíč plátna jen loopbacku (kiosku na boxu)
  a pošle ho na plátno otevřené party; jinak ukáže obrazovku boxu (adresa pro
  telefony, IP, certifikát). První party na boxu jde založit bez admin tokenu.
- **Párování**: s `PC_BOX_AUTOPARTY` box party založí sám a plátno na boxu
  ukáže QR s odkazem do Cockpitu (token v `#`). QR zmizí, když se Cockpit
  připojí, nebo za 10 min; Cockpit ho může ukázat znovu. Token je na plátně
  jen v této chvíli přípravy, kdy je v sále pořadatel; vědomý kompromis proti
  psaní tokenu na TV.
- Bez `PC_PUBLIC_URL` vede QR pro hosty na LAN adresu boxu (kiosk sám mluví
  na 127.0.0.1, což by telefonům nebylo k ničemu).
- **https přes DNS-01**: veřejný DNS záznam vlastní domény na LAN IP boxu,
  certifikát Let's Encrypt přes `lego` a Cloudflare API, obnova týdně doma.
  Router má stejný záznam pro místa bez internetu. Telefony hostů nic
  neinstalují.
- Backend hlásí systemd připravenost a krmí watchdog, dokud odpovídá DB
  (`internal/sdnotify`, bez závislosti). Release: `make dist` (static arm64
  a amd64 s vloženým frontendem).
- Neověřeno na skutečném Pi: kiosk pod `cage`, výkon, teplota, `lego`. Ověřeno
  na Macu: box mode v prohlížeči, testy, `shellcheck` skriptů, cross-compile.

## D21: Cockpit podruhé, v Reactu (2026-11-06)
Zadání: `Prompts/03-COCKPIT-REACT.md`.
- Druhá implementace Cockpitu v React 19 + TypeScript v `frontend-react/`,
  vedle Svelte. Důvod: portfolio pro pozici s požadavkem na React a TS;
  vedlejší efekt je druhý klient nad stejným protokolem.
- Sdílí se jen transport a typy (`frontend/src/lib/*.ts` přes alias `@pc`)
  a klíče v localStorage. UI a stav jsou nezávislé: `useReducer` nad zprávami
  ze socketu, hooky, kontext, CSS Modules; žádné knihovny na stav ani UI.
  Sdílené třídy s vlastním životem (WebRTC `Viewer`, `Mic`) se vytváří
  v efektech a čtou přes `useSyncExternalStore`.
- Obě UI vložená v binárce: `/cockpit/<kód>` Svelte, `/react/cockpit/<kód>`
  React; `PC_COCKPIT_UI=react` přesměruje první na druhé. Fragment
  `#token=` přežije 302, takže párovací QR z boxu funguje s oběma.
- Odchylka od pravidla „web je Astro nebo SvelteKit“: vědomá, jen pro Cockpit.
  Vite 8 místo 7 (plugin pro React 19 jiný nepodporuje), TypeScript 6.0 jako
  ve `frontend/` (typescript-eslint vyšší nepodporuje).
- `confirm()` u výměny tokenu nahrazuje dvoukrokové tlačítko (modální dialog
  blokuje kiosk i automatizaci prohlížeče).
- **Co druhý klient odhalil o protokolu:**
  - Server posílal prázdné seznamy ve stavu Cockpitu jako `null` (Go nil
    slice), typy tvrdí pole. Na party bez hostů padal i Svelte Cockpit
    (hlavička se nevykreslila celá). Opraveno na serveru (`orEmpty`, test);
    React navíc čte zprávu jako `Wire<T>`, kde seznam smí být `null`.
  - Cockpit nezná limit živých streamů na plátně (`PC_MAX_LIVE`, na Pi 4):
    oba ukazují „/9“. Nechané kvůli paritě; stačilo by poslat `maxLive`
    v `config`.
  - „Konec hry“ přepíná na mezihru s počtem dob ze scénového panelu; React
    posílá výchozích 8, protože panel scén drží svůj stav sám.


## D22: Notebook jako party box, restyle přes ComfyUI (2026-11-06)
- **Release ke stažení** (GitHub Release, odkazy na olin.now): `linux-arm64`
  (Pi), `linux-amd64` a `windows-amd64.zip`. Windows jen jako notebook u TV,
  bez instalátoru a služeb; kiosk a systemd zůstávají na Linuxu (D20).
  Návod `deploy/laptop/README.txt` je v obou balíčcích.
- Binárka sama načte `.env` z pracovního adresáře (proměnné prostředí mají
  přednost), aby šla na Windows spustit dvojklikem.
- Plátno otevřené na stejném počítači (localhost) dává do QR LAN adresu i mimo
  režim boxu. Párovací QR (Cockpit do telefonu) jde z Cockpitu i na notebooku;
  ukáže se jen na plátně otevřeném na serveru samém (loopback), jako na boxu.
- **`RESTYLE_PROVIDER=comfy`**: graf `restyle_flux` z Restyleru (FLUX dev +
  PuLID + depth ControlNet) posílá PartyCockpit přímo do ComfyUI na
  `COMFY_URL`, bez Restyler API. Šablona je kopie
  `MangaPrompts-restyler` `assets/comfyui/flux_restyle.api.json` (větev
  `feat/restyler`); rozměr je SDXL bucket nejbližší poměru stran fotky.
  Restyle a animace sdílí jeden klient a frontu ComfyUI. Kontext se na restyle
  nepoužívá: překreslí celý snímek a bez masky nedává smysl.
- Soubory na GPU hostu mají prefix `party_` (uploady, drivery, výstupy), aby
  úklid po party smazal jen je.
- `COMFY_TOKEN` (volitelný): `Authorization: Bearer` pro proxy s tokenem před
  ComfyUI, když ComfyUI není jen na 127.0.0.1.
- **Licence:** PuLID používá InsightFace antelopev2, který je jen pro
  nekomerční použití. Na soukromou party v pořádku, pro B2B (D10) otevřené,
  stejně jako váhy LivePortrait (D13).
- **https přes Cloudflare Tunnel** (návod pro notebook): telefony dostanou
  certifikát Cloudflare bez vlastního certifikátu na boxu. Potřebuje internet a
  `PC_TRUST_PROXY=1`, jinak mají všichni hosté přes tunel stejnou IP a narazí
  na limity. Bez internetu dál platí certifikát přes DNS (D20).
