# Zadání: Party box na Raspberry Pi

Cíl: k projektoru nebo TV se připojí jen Raspberry Pi (HDMI, Wi-Fi, karta).
Běží na něm backend PartyCockpitu i plátno v kiosku. Všechno ovládání je
v telefonu: Cockpit operátora i hosté. GPU úlohy (restyle, živé portréty,
tance, titulky) jdou dál na vzdálené servery.

Jsi Claude session v repu PartyCockpit (veřejné repo
https://github.com/lioilsources/PartyCockpit). Tohle zadání je samostatné.
Čti `CLAUDE.md`, `DECISIONS.md` (D12, D14, D17, D19), `README.md` (Kiosk)
a `docs/SECURITY_PLAN.md`. Pravidla pro práci jsou stejná jako
v `Prompts/01-SECURITY_FIXES.md` (sekce „Než začneš“ a „Konvence v kódu“):
`make check` jako brána, testy s `-timeout 120s`, testovací server na 18098,
`git add` jmenovitě, commit po každém kroku, nepushovat bez pokynu, do repa
žádné LAN IP, tokeny ani domácí cesty.

Raspberry Pi nemáš. Všechno, co jde, ověř na Macu (cross-compile, box mode
v prohlížeči, skripty projdou `shellcheck`). Co na Macu ověřit nejde, vypiš
do STATUS jako neověřené a nech to na runbook pro uživatele.

## Část A: posouzení (pro uživatele, hotové, neměň)

### Utáhne to?

**Raspberry Pi 5 (8 GB) ano, s omezeními. Pi 4 ne.**

| Co plátno dělá | Pi 5 | Poznámka |
|---|---|---|
| Go backend, SQLite, WebSocket, zip recapu | bez problému | `modernc.org/sqlite` je čisté Go, `GOARCH=arm64` stačí |
| Pixi 2D scény, QR, ručně psané vzkazy (canvas) | ano, 1080p 60 fps | |
| Shader přestávka (WebGL 1, 5 shaderů) | ano | kreslí se na 0,6 rozlišení; na Pi dát `scale` 0,5 |
| StyleMap sweep (12–16 obrázků/s) | ano | náhledy jsou malé WebP |
| Živé portréty a tance (mp4 h264 480×832) | 1–4 naráz ano | **Pi 5 nemá hardwarový dekodér H.264**, jen HEVC. Dekóduje se softwarově, 480p klipy zvládne; 1080p klipy ne. Nechat výstup z L40S na 480×832. |
| Živé streamy (WebRTC, VP8/H264) | **max 4**, ne 9 | 9 softwarově dekódovaných streamů Pi nezvládne. Limit nastavit konfigurací. |
| Kruh (canvas 2D) | ano | |
| 4K výstup | ne, nastavit 1080p | 4K zdvojnásobí práci GPU a Chromium na Pi při 4K padá na 30 fps |

Beat z mikrofonu: Pi nemá audio vstup. Tempo bere z DJfy nebo z tapu
v Cockpitu, případně USB mikrofon (200 Kč) a `?audio=1`.

**Srovnání s mini PC (Intel N100, ~4 500 Kč):** x86, hardwarový dekodér
H.264/HEVC, 16 GB RAM, Windows nebo Linux. Všech 9 streamů i 1080p klipy
bez obav. Pi 5 je o něco levnější a menší, ale na hraně u videa. Pokud má být
party box „zapoj a jede“ bez kompromisů, N100 je bezpečnější volba. Tenhle
plán počítá s Pi 5 a omezení zapisuje do konfigurace, takže stejný image
poběží i na N100 s Raspberry Pi OS nahrazeným Debianem.

### Co koupit (orientační ceny v Kč, listopad 2026, ověřit)

| Položka | Cena | Poznámka |
|---|---|---|
| Raspberry Pi 5, 8 GB | 2 300 | 16 GB netřeba |
| Oficiální 27W USB-C zdroj | 350 | slabší zdroj = podtaktování |
| Aktivní chladič (Active Cooler) | 150 | bez něj throttling po pár minutách shaderů |
| Krabička s místem na chladič | 300 | |
| microSD 64 GB, A2 (SanDisk Extreme) | 300 | nebo NVMe HAT + 256 GB SSD (~1 200), rychlejší a odolnější |
| Kabel micro-HDMI → HDMI 2 m | 150 | |
| **Cestovní router** (GL.iNet Beryl AX nebo Slate AX) | 1 500–2 500 | **důležité**: Wi-Fi na Pi utáhne ~10 telefonů, ne 50. Router dělá Wi-Fi pro hosty, DHCP, DNS a napojení na internet místa (Wi-Fi nebo LTE). |
| USB mikrofon (volitelně) | 200 | beat z hudby v sále |
| **Celkem** | **~4 000 bez routeru, ~6 500 s routerem** | |

Co je potřeba jinde: GPU server pro restyle (Restyler), L40S pro animace,
Translator pro titulky. Pi se k nim dostane přes VPN (Tailscale) nebo tunel;
bez internetu na místě jede party s mockem (žádné restyly, vzkazy, hra,
streamy, shader a recap fungují).

### Síť a https (nejtěžší část)

Kamera v telefonu a mikrofon v Cockpitu jdou jen přes https. Self-signed
certifikát telefony hostů nepřijmou (každý by klikal varování, iOS kameru
stejně nepustí). Řešení bez instalace čehokoli na telefony:

1. Doména, která patří uživateli, třeba `box.olin.now`.
2. Veřejný DNS záznam `box.olin.now → <IP-boxu>` (privátní IP Pi v síti
   routeru). Veřejné DNS smí ukazovat na privátní adresu.
3. Certifikát Let's Encrypt přes **DNS-01** (lego + Cloudflare API token),
   obnova na Pi cronem, když má Pi internet. Platí 90 dní, takže i box bez
   internetu na party má platný certifikát, když se obnovil doma.
4. Telefony na Wi-Fi routeru se ptají veřejného DNS a dostanou LAN IP.
   Když místo nemá internet, router má v nastavení vlastní DNS záznam
   `box.olin.now → <IP-boxu>` (GL.iNet: „Custom DNS“ / dnsmasq
   `address=/box.olin.now/<IP-boxu>`). Nastavit obojí.
5. `PC_PUBLIC_URL=https://box.olin.now:8443`, `PC_TLS_CERT`/`PC_TLS_KEY`
   na certifikát. QR na plátně pak vede na https a kamera i mikrofon fungují.

Router: Wi-Fi hostů bez „client isolation“ (WebRTC je P2P), Pi po Ethernetu
do routeru s pevnou IP (DHCP rezervace), upstream routeru = Wi-Fi místa nebo
LTE. Na Pi Tailscale pro GPU servery a pro SSH správu odkudkoli.

## Část B: změny v PartyCockpitu (udělat)

### Commit 1: build pro arm64 a instalace

- `Makefile`: `make dist` postaví `bin/partycockpit-linux-arm64` a
  `bin/partycockpit-linux-amd64` (`CGO_ENABLED=0 GOOS=linux GOARCH=…`, frontend
  embedovaný) a zabalí `dist/partycockpit-<verze>-linux-<arch>.tar.gz`
  s binárkou, `assets/drivers/`, `.env.example` a `deploy/box/`.
  Verze z `git describe --tags --always`.
- `deploy/box/` (nová složka, pro Pi i Debian):
  - `install.sh`: idempotentní, spouští se na Pi jako root: založí uživatele
    `party`, `/opt/partycockpit/{bin,data,certs}`, zkopíruje binárku,
    `.env` z `.env.example` (když neexistuje), nainstaluje systemd jednotky,
    `chromium` a `cage` (viz runbook), zapne služby. Cílový OS: Raspberry Pi
    OS Bookworm Lite 64-bit (a Debian 12 na x86).
  - `partycockpit.service`: `User=party`, `EnvironmentFile=/opt/partycockpit/.env`,
    `WorkingDirectory=/opt/partycockpit`, `Restart=always`, `RestartSec=2`,
    `ProtectSystem=strict`, `ReadWritePaths=/opt/partycockpit/data`.
  - `kiosk.service`: `After=partycockpit.service`, uživatel `party`, spustí
    `cage -- chromium --kiosk --ozone-platform=wayland --noerrdialogs
    --disable-infobars --no-first-run --disable-features=WebRtcHideLocalIpsWithMdns
    --use-fake-ui-for-media-stream --autoplay-policy=no-user-gesture-required
    http://127.0.0.1:8098/box`. `Restart=always`. Čeká na `/healthz`
    (`ExecStartPre` smyčka s `curl`, max 60 s).
  - `renew-cert.sh` + `renew-cert.timer`: lego DNS-01 (Cloudflare), výstup do
    `/opt/partycockpit/certs/`, po obnově `systemctl restart partycockpit`.
    Token v `/opt/partycockpit/lego.env` (mode 600), nikdy v repu.
  - `update.sh`: stáhne nový tar.gz z GitHub Release (nebo z cesty), vymění
    binárku, restartuje. Data a `.env` zůstávají.
- `shellcheck` na všechny skripty (přidej do `make check`, když je
  `shellcheck` k dispozici; jinak přeskoč s hláškou).
- Test: `make dist` na Macu projde a `file bin/partycockpit-linux-arm64`
  hlásí ARM aarch64. Tar obsahuje vše ze seznamu.

### Commit 2: box mode

Pi nemá klávesnici ani laptop vedle sebe. Plátno musí nabootovat do
smysluplné obrazovky a operátor musí Cockpit otevřít z telefonu.

- Config: `PC_BOX=1` zapne box mode; `PC_BOX_AUTOPARTY=1` při startu založí
  party, když žádná neběží (název z `PC_BOX_NAME`, default „Party“);
  `PC_MAX_LIVE` (default 9; v `.env` pro Pi 4) omezí `maxLiveStreams`
  (dnes konstanta v `streams.go`); `PC_STAGE_SCALE` (default 0.6, Pi 0.5)
  pošle plátnu výchozí `scale` pro shadery (přes `party` zprávu, pole
  `box: {scale}`).
- `GET /box` **jen z loopbacku** (`RemoteAddr` 127.0.0.1/::1, jinak 404):
  - když běží party (nejmladší s `ends_at` v budoucnu): 302 na
    `/stage/<kód>?key=<stage_key>&audio=<PC_BOX_AUDIO>`;
  - jinak stránka „Party box“: název zařízení, IP adresy, `PC_PUBLIC_URL`,
    stav certifikátu (platný do), stav GPU providerů (ping), a **párovací QR
    pro Cockpit**: odkaz `/cockpit/<kód>#token=…`. Bez party nejde párovat,
    proto bez `PC_BOX_AUTOPARTY` stránka nabídne jen QR na `/` (Home)
    s textem „založ party z telefonu“ a Home v box mode nevyžaduje admin
    token, když jde o první party (žádná neexistuje).
- **Párování Cockpitu**: po založení party ukazuje plátno v rohu párovací QR
  (token v `#hash`), dokud se nepřipojí první Cockpit nebo neuplyne 10 min.
  Pak zmizí. Znovu ho ukáže `POST /api/parties/{code}/pair` z Cockpitu
  (kdo má Cockpit, může přizvat další), nebo reboot boxu. Ten QR je
  jediný moment, kdy je token na obrazovce, a je to vědomé: v setup fázi je
  v sále jen pořadatel. Zapiš do D20.
  - Implementace: nová zpráva `pairing {url}` jen pro `role=stage`
    z loopbacku (`hub.Peer.IP`), plátno kreslí QR vpravo nahoře s textem
    „Naskenuj = Cockpit“. Připojení Cockpitu `pairing` vypne (`pairing null`).
- `Stage.svelte`: `/stage/<kód>` bez klíče v box mode nic nemění; klíč
  přijde v redirectu z `/box`. Box info (scale) z `party.box`.
- Preflight: v box mode kontroluje i certifikát (dny do expirace, `warn`
  pod 14 dní, `fail` po expiraci), teplotu CPU
  (`/sys/class/thermal/thermal_zone0/temp`, `warn` nad 75 °C) a volné místo
  na datové partition (už je).
- Testy: `/box` z ne-loopback adresy 404 (httptest má 127.0.0.1, použij
  `r.RemoteAddr` podstrčený přes vlastní handler test); `/box` bez party
  dává stránku, s party 302 s klíčem; `pairing` dostane jen stage
  z loopbacku, po připojení Cockpitu přijde `pairing null`; `PC_MAX_LIVE=4`
  → pátý `approve` 409.

### Commit 3: odolnost boxu

- Plátno už cachuje stav v localStorage. Doplň: `Stage.svelte` po 30 s
  offline ukáže v rohu malý „offline“ štítek s textem (dnes jen tečka),
  a každých 60 s zkusí `location.reload()` jen když je offline **a** cache
  je prázdná (čerstvý box bez party).
- `kiosk.service` a `partycockpit.service` s `WatchdogSec` tam, kde to dává
  smysl (backend: `sd_notify` přes `github.com/coreos/go-systemd/v22/daemon`,
  jen když `NOTIFY_SOCKET` existuje; jinak nic).
- `deploy/box/README.md`: runbook níže zkopíruj do repa a doplň přesné
  příkazy tak, jak je `install.sh` skutečně dělá.
- `DECISIONS.md` D20 (box mode, párování, limity Pi, https přes DNS-01),
  `STATUS.md` sekce „Party box“ s tím, co je ověřeno jen na Macu,
  `README.md` sekce „Party box (Raspberry Pi)“ s odkazem na runbook,
  `.env.example` nové proměnné.

### Čeho se nedotýkat

GPU providery, Translator, workflow JSONy, migrace 1–4, schvalování (D5),
bezpečnostní invarianty D19 (stage key zůstává; `/box` ho vydává jen
loopbacku).

## Část C: runbook (pro uživatele; Opus ho zkopíruje do `deploy/box/README.md`)

### 1. Image

1. Raspberry Pi Imager → Raspberry Pi OS **Lite 64-bit (Bookworm)**.
   V nastavení: hostname `partybox`, uživatel `admin` s SSH klíčem (heslo
   vypnout), Wi-Fi nevyplňovat (Pi jede po Ethernetu do routeru), locale
   `cs_CZ`, časová zóna Praha.
2. První boot na Ethernetu s internetem. `ssh admin@partybox.local`.
3. `sudo apt update && sudo apt full-upgrade -y && sudo rpi-eeprom-update -a`.
4. `sudo raspi-config`: Advanced → Wayland: **labwc** nebo Lite + `cage`
   (install.sh instaluje `cage chromium`); Display → HDMI 1080p; Performance →
   Fan: zapnout aktivní chladič. `sudo reboot`.
5. Tailscale: `curl -fsSL https://tailscale.com/install.sh | sh && sudo tailscale up`.
   Pak `ssh admin@partybox` funguje odkudkoli.

### 2. PartyCockpit

```sh
curl -LO https://github.com/lioilsources/PartyCockpit/releases/latest/download/partycockpit-linux-arm64.tar.gz
tar xzf partycockpit-linux-arm64.tar.gz && cd partycockpit-*
sudo ./deploy/box/install.sh
sudo nano /opt/partycockpit/.env     # PC_BOX=1, PC_BOX_AUTOPARTY=1, PC_PUBLIC_URL, PC_MAX_LIVE=4, PC_STAGE_SCALE=0.5, tokeny
sudo systemctl restart partycockpit kiosk
```

Na HDMI se objeví stránka Party box s párovacím QR. Naskenovat telefonem =
Cockpit. „Otevřít plátno“ v Cockpitu na boxu není potřeba, plátno už běží.

### 3. Certifikát (jednou, s internetem)

1. DNS u Cloudflare: `A box.olin.now → <IP-boxu>` (IP Pi v síti routeru,
   rezervovaná v DHCP routeru). API token jen s právem DNS edit pro tu zónu.
2. `sudo nano /opt/partycockpit/lego.env` → `CLOUDFLARE_DNS_API_TOKEN=…`,
   `LEGO_EMAIL=…`, `LEGO_DOMAIN=box.olin.now`; `chmod 600`.
3. `sudo /opt/partycockpit/bin/renew-cert.sh` → certifikát v `certs/`,
   `.env`: `PC_TLS_CERT=/opt/partycockpit/certs/box.olin.now.crt`,
   `PC_TLS_KEY=…key`, `PC_PUBLIC_URL=https://box.olin.now:8443`.
4. `renew-cert.timer` obnovuje každý týden, když je internet. Před party
   zkontrolovat v Cockpitu „Před party“ → Certifikát.

### 4. Router (GL.iNet)

- Wi-Fi hostů: SSID `Party`, WPA2 nebo bez hesla, **client isolation vypnout**,
  2,4 i 5 GHz.
- LAN: DHCP rezervace pro Pi (`<IP-boxu>`), Pi po kabelu.
- DNS: vlastní záznam `box.olin.now → <IP-boxu>` (pro místa bez internetu).
- Upstream: „Repeater“ na Wi-Fi místa, nebo LTE modem; bez upstreamu jede
  party s mockem.

### 5. Golden image

Po nastavení: `sudo rpi-clone` na druhou kartu, nebo na Macu `dd` z karty
do `.img` a `pishrink`. Před zálohou smazat `data/` (party z testů) a
`lego.env` nechat (token je potřeba k obnově certifikátu). Na novém boxu
jen změnit hostname a DHCP rezervaci.

### 6. Checklist před párty (navíc k „Před party“ v Cockpitu)

- Box 20 min napřed: teplota po shaderech < 70 °C, chladič běží.
- Telefon na Wi-Fi hostů: QR z plátna otevře https bez varování, kamera
  v záložce „Živě“ se spustí.
- Druhý telefon: vzkaz na zeď, schválit z Cockpitu, je na plátně.
- Odpojit kabel internetu na 1 min: plátno jede dál, Cockpit se po připojení
  obnoví.
- Náhradní karta s golden image v kapse.

### 7. Aktualizace

`sudo /opt/partycockpit/bin/update.sh` (stáhne poslední release), nebo
`scp bin/partycockpit-linux-arm64 admin@partybox:/tmp/ && sudo
/opt/partycockpit/bin/update.sh /tmp/partycockpit-linux-arm64`.
