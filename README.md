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
| Guest | `/join/<KÓD>` | hosté (QR na plátně), bez instalace a účtu |

## Scény

- **Slideshow** z FineTuneGallery, jen SFW, Ken Burns, přepíná na beat.
- **Galerie hostů**: přemalované selfie ze show fronty.
- **Vzkazy**: zeď s ručně psanými lístky.
- **Now playing**: skladba a fronta z DJfy s hlasy.
- **Webtoon**: předpřipravené epizody s auto-scrollem.
- **Losování**: kdo hraje o přestávce.
- **Tančírna**: hosté tančí na plátně. Telefon → „Roztanči mě“ → Wan na GPU → schválení.
- **Živé portréty**: fotky v galerii mrkají a rozhlížejí se (LivePortrait).
- **Živě**: mřížka živých kamer z telefonů (prohlížeč nebo MirrorBooth) až 3×3, po schválení operátorem.
- **Mezihra**.

Nad každou scénou běží emoji reakce (PixiJS), lišta s právě hrající skladbou a QR.

## Rychlý start

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
| AiStack | záloha restyle (flux-kontext), NSFW tagger (WD14) | `AISTACK_URL`, `MODERATION=aistack` |
| FineTuneGallery | obrázky do slideshow | `FINETUNE_URL` |
| ComfyUI (L40S) | živé portréty (LivePortrait) a tanec (Wan 2.2 I2V) | `ANIMATE_PROVIDER=comfy`, `COMFY_URL` |
| DJfy | now playing, sdílená fronta, hlasování, BPM | `DJFY_URL` + v Cockpitu kód DJfy party |

Hosty za Cloudflare Access ověřuje `CF_ACCESS_CLIENT_ID/SECRET`.

## Party box (Raspberry Pi)

K TV nebo projektoru stačí Raspberry Pi 5 (nebo mini PC) s PartyCockpitem
a plátnem v kiosku; Cockpit i hosté jsou v telefonech. Balíček: `make dist`,
instalace, https a router: [`deploy/box/README.md`](deploy/box/README.md).

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
