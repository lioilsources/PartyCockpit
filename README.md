# PartyCockpit

Live show na velké plátno z vlastních obrázků a hostů na party. Hosté se připojí
telefonem přes QR: pošlou selfie, které se na GPU přemaluje do zvoleného stylu, přidají
skladbu do DJfy fronty, napíšou vzkaz na zeď, pošlou námět na webtoon nebo se přihlásí
do losování o hru. Operátor v Cockpitu schvaluje obsah a přepíná scény. Pak může jít
tancovat, protože show fronta se točí sama dokola.

Jedna Go binárka s vestavěnou Svelte appkou a SQLite. Spustí se na čemkoli v LAN party:
Mac Mini, Windows mini PC nebo domácí server.

| Role | URL | Kdo |
|---|---|---|
| Stage | `/stage/<KÓD>?key=…` (odkaz „Otevřít plátno“ v Cockpitu) | prohlížeč na Mac Mini / mini PC s HDMI do TV (kiosk) |
| Cockpit | `/cockpit/<KÓD>` | operátor (telefon, tablet, notebook) |
| Studio | `/studio/<KÓD>` | operátor: show připravená dopředu, na party offline |
| Guest | `/join/<KÓD>` | hosté (QR na plátně), bez instalace a účtu |

## Scény

- **Slideshow** z FineTuneGallery, jen SFW, Ken Burns, přepíná na beat.
- **Galerie hostů**: přemalované selfie ze show fronty.
- **Vzkazy**: zeď s ručně psanými lístky.
- **Now playing**: skladba a fronta z DJfy s hlasy.
- **Webtoon**: příběhy z katalogu Toonfork, 1 až 4 stripy vedle sebe, každý
  roluje svým tempem, bubliny kreslí plátno (čeština a další jazyky).
- **Pásy**: přemalované fotky v pásech jako v Restyleru, sloupce rolují cik cak.
  Místo fotky hraje její klip: mrknutí a úsměv, nebo vlasy ve větru.
- **Losování**: kdo hraje o přestávce.
- **Tančírna**: hosté tančí na plátně. Telefon → „Roztanči mě“ → Wan na GPU → schválení.
- **Živé portréty**: fotky v galerii mrkají a rozhlížejí se (LivePortrait).
- **StyleMap**: jedna postava v desítkách stylů jako plynulá proměna, až 9
  packů naráz. Packy jdou postavit i ze session FineTuneGallery (jeden prompt,
  jeden pack).
- **Živě**: mřížka živých kamer z telefonů (prohlížeč nebo MirrorBooth) až 3×3, po schválení operátorem.
- **Mezihra**.

Nad každou scénou běží emoji reakce (PixiJS), lišta s právě hrající skladbou a QR.

## Rychlý start

Bez kompilace: stáhni balíček z [olin.now](https://olin.now) nebo
z GitHub Releases (Windows zip, Linux amd64/arm64) a postupuj podle
`README.txt` v něm (`deploy/laptop/README.txt`). Na Windows stačí dvojklik
na `partycockpit.exe`.

Ze zdrojáků:

```sh
cp .env.example .env        # RESTYLE_PROVIDER=mock funguje bez GPU
make build && make run      # → http://localhost:8098
```

Na úvodní stránce založíš party. Cockpit pak dá odkaz na plátno a join URL.
Do `PC_PUBLIC_URL` patří LAN adresa party boxu, protože ta jde do QR kódu.

Vývoj: `make dev` pustí backend na :8098 a Vite na :5173 s proxy. Testy: `make check`.

Docker: `docker compose -f deploy/docker-compose.yml --env-file .env up -d --build`

## Integrace

| Služba | K čemu | Konfigurace |
|---|---|---|
| Restyler backend | restyle selfie (`restyle_flux`, drží obličej) | `RESTYLE_PROVIDER=restyler`, `RESTYLER_*` |
| ComfyUI (L40S) | týž `restyle_flux` přímo v ComfyUI, bez Restyler API | `RESTYLE_PROVIDER=comfy`, `COMFY_URL` |
| AiStack | záloha restyle (flux-kontext), NSFW tagger (WD14) | `AISTACK_URL`, `MODERATION=aistack` |
| FineTuneGallery | obrázky do slideshow, sessions pro StyleMap packy | `FINETUNE_URL` |
| Toonfork (webtoon-content) | hotové webtoon příběhy (zip packy na GitHub Releases) | `WEBTOON_CATALOG_URL`, `WEBTOON_ALLOW_ADULT` |
| ComfyUI (L40S) | živé portréty (LivePortrait) a tanec (Wan 2.2 I2V) | `ANIMATE_PROVIDER=comfy`, `COMFY_URL` |
| DJfy | now playing, sdílená fronta, hlasování, BPM | `DJFY_URL` + v Cockpitu kód DJfy party |

Hosty za Cloudflare Access ověřuje `CF_ACCESS_CLIENT_ID/SECRET`.

## Scene Studio

Show se dá připravit dopředu: scény s položkami, délkami a seedem, náhled
scény, vlastní fotky přes GPU, stažené webtoony a StyleMap packy. Výstupy
z GPU se uloží jako soubory, takže na party stačí notebook bez internetu
a show se hýbe pokaždé stejně. Hotovou show jde uložit do knihovny a načíst
do jiné party. Postup i se SPARK oknem: [`docs/studio.md`](docs/studio.md).

## Party box (Raspberry Pi)

K TV nebo projektoru stačí Raspberry Pi 5 (nebo mini PC) s PartyCockpitem
a plátnem v kiosku; Cockpit i hosté jsou v telefonech. Balíček: `make dist`,
instalace, https a router: [`deploy/box/README.md`](deploy/box/README.md).

## Cockpit v Reactu

Cockpit má dvě nezávislé implementace nad stejným API a WebSocketem: Svelte
(`frontend/`, výchozí) a React 19 + TypeScript (`frontend-react/`). Obě jsou
v binárce. React běží na `/react/cockpit/<kód>`, na úvodní stránce je tlačítko
„Cockpit (React)“. `PC_COCKPIT_UI=react` přesměruje i `/cockpit/<kód>`
(párovací QR z boxu, odkazy z Home) na React. Sdílí se jen transport a typy
z `frontend/src/lib` (alias `@pc`) a klíče v localStorage, takže přepnutí
UI zachová přihlášení i nastavení.

```sh
cd frontend-react
npm run dev     # :5174/react/cockpit/<kód>, proxy na backend :8098
npm run check   # tsc + ESLint (strict type-checked, react-hooks) + Vitest
```

Stav je `useReducer` nad zprávami ze socketu (`src/state`), WebRTC a mikrofon
jsou sdílené třídy zabalené přes `useSyncExternalStore` (`src/hooks`).
Rozhodnutí: `DECISIONS.md` D21.

## Kiosk

```sh
# macOS
open -a "Google Chrome" --args --kiosk --use-fake-ui-for-media-stream --disable-features=WebRtcHideLocalIpsWithMdns "http://localhost:8098/stage/KOD?key=KLIC&audio=1"
# Windows
chrome.exe --kiosk --use-fake-ui-for-media-stream --disable-features=WebRtcHideLocalIpsWithMdns "http://localhost:8098/stage/KOD?key=KLIC&audio=1"
```

`KLIC` je klíč plátna z odkazu „Otevřít plátno“ v Cockpitu; plátno si ho po prvním
otevření pamatuje a z adresy ho smaže. `?audio=1` zapne detekci beatu z audio vstupu (BlackHole / VB-Cable / mikrofon).
Přednost má ale BPM z DJfy nebo tap tempo z Cockpitu.

## Živé streamy

Peer-to-peer WebRTC v LAN, bez SFU a TURN. Server jen přeposílá signalizaci
([DECISIONS.md D12](DECISIONS.md)). Kamera v mobilním prohlížeči potřebuje https: nastav
`PC_TLS_CERT`/`PC_TLS_KEY` a `PC_PUBLIC_URL=https://…`. MirrorBooth https nepotřebuje.
Protokol pro MirrorBooth je popsaný v [docs/mirrorbooth-stream.md](docs/mirrorbooth-stream.md).

Rozhodnutí a odchylky od původního plánu jsou v [DECISIONS.md](DECISIONS.md), stav v
[STATUS.md](STATUS.md) a plán v [Prompts/](Prompts/).

MIT License.
