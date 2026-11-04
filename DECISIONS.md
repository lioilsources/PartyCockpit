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
