# STATUS

## 2026-11-03 — M0–M2 implementováno (bez nasazení)

| Milník | Stav |
|---|---|
| M0 Skeleton | ✅ Go backend + SQLite + WS hub, Svelte app (Stage/Cockpit/Guest/Home), Slideshow z FineTuneGallery, přepínání scén, QR overlay, Dockerfile + Compose |
| M1 Guest + restyle | ✅ upload → Restyler `restyle_flux` (nebo kontext/mock) → klasifikátor → moderace → show fronta (loop/shuffle) → Galerie hostů; emoji reakce |
| M1b Vzkazy | ✅ Sticker Wall (font Caveat, ne HandWrittenStickers) |
| M2 DJfy | ✅ now playing, fronta, hledání/přidání/hlasování přes relay, BPM → BeatClock, tap tempo, audio fallback |
| navíc | ✅ losování hráčů (D6), náměty na webtoon + export JSON (D7), Webtoon reader, panic, janitor (mazání 7 dní po konci) |

Ověřeno lokálně: `go test` (store, djfy proti fake relay, Restyler request shape),
svelte-check bez chyb, e2e přes curl na mock provideru (upload → hotovo za 3 s →
schválení → fronta → scéna) a screenshoty Stage, Cockpitu i Guest v Chrome.

**Neověřeno proti živým službám:**
- Restyler API: nemám `RESTYLER_APP_TOKEN`. Tvar requestu je převzatý z `tsumiki_shared`
  a otestovaný proti fake serveru.
- FineTuneGallery: vrací 302 na Cloudflare Access a potřebuje service token povolený
  v Access aplikaci.
- DJfy: testováno proti fake relay. Na živém relay jsem session nezakládal.
- AiStack kontext: hodnota `aspect_ratio: match_input_image` a pole `image` jako data URI
  jsou odhad (nim-kontext-proxy je jen přeposílá do NIM).

## 2026-11-03: M3 živé streamy (PartyCockpit strana)

| Část | Stav |
|---|---|
| signalizace WebRTC přes party WS, stream registry, moderace, PANIC | ✅ `api/streams.go` + integrační test |
| scéna `pipwall` (1 → 2×2 → 3×3, rámeček na beat) | ✅ |
| Cockpit: náhledy streamů, pustit / stáhnout / ukončit | ✅ |
| Guest záložka „Živě“: kamera + canvas filtry → stream | ✅ (potřebuje https) |
| HTTPS listener (`PC_TLS_*`), `PC_ICE_SERVERS` | ✅ |
| MirrorBooth „Stream to party“ | ⏳ zadání v `docs/mirrorbooth-stream.md` (MacBook session) |

Ověřeno v Chrome: dva simulované telefony (canvas) se po „✓ pustit“ v Cockpitu objevily
na plátně ve 2 sloupcích, Cockpit ukazoval náhledy pending streamů a PANIC je stáhl.
Simulace přepisovala mDNS kandidáty na 127.0.0.1, protože dvě záložky bez kamery
na jednom Macu se přes mDNS nespojí.
**Neověřeno:** skutečný telefon na LAN, https certifikát, latence pod 1 s, 4 a více streamů.

Pozn.: na tomhle Macu drží port 8098 zapomenutý `python -m http.server 8098` (od 18. 10.).

## 2026-11-03: M4 živé portréty a tanec (PartyCockpit strana)

| Část | Stav |
|---|---|
| provider `Comfy` (upload, /prompt, /history, /view, driver), `RestylerAnimator`, `MockAnimator` | ✅ testy proti fake ComfyUI |
| workflow `live_portrait.json` (BlazeFace cropper), `wan_i2v.json` (14B + lightx2v) | ✅ ověřeno v L40S session |
| driver `alive` (syntetický, 10 s, pingpong) | ✅ `assets/drivers/alive.mp4`, na VM `input/pc_driver_alive.mp4` |
| auto-oživení schválených fotek, „✨ Oživit galerii“, galerie hraje smyčky | ✅ |
| „Roztanči mě“ v telefonu, schválení, scéna Tančírna (tempo ±10 %) | ✅ |

Ověřeno: Go testy (celý tok na mocku, Comfy proti fake serveru, store) a v Chrome
na mocku Tančírna se 3 tanečníky, galerie s živým portrétem a „Roztanči mě“.
Opravena chyba: auto-oživení blokovalo hostovi nahrání fotky.
L40S session workflow ověřila přímo v ComfyUI: tanec 46 s, živý portrét 29 s (časy a
kvalita v DECISIONS D13). **Neověřeno:** PartyCockpit `ANIMATE_PROVIDER=comfy` proti
skutečnému ComfyUI. ComfyUI se po testech L40S vypnulo (GPU zpátky pro AlphaZero),
takže to chce okno. Nový prompt pro robota také zatím neběžel.

## 2026-11-04: M7 vyladění show (část)

| Část | Stav |
|---|---|
| scéna Recap (mozaika, reflektor po dobách, QR na stažení) | ✅ |
| `/recap/<kód>` pro hosty: mřížka, detail, stažení po jednom i zip | ✅ |
| scéna Shader přestávka, 5 shaderů, střídání po N dobách | ✅ |
| Cockpit „Před party“: živé kontroly + ruční checklist | ✅ |
| show fronta bez `meta.original` | ✅ |
| titulky: mikrofon v Cockpitu → Translator → lišta na plátně, ruční titulky | ✅ testy proti fake Translatoru |
| StyleMap sweep: scéna a přechod mezi scénami, proxy + cache balíčků | ✅ na lokálním demo balíčku |

Ověřeno: Go testy (recap, zip, preflight, ping, titulky přes fake Translator) a v Chrome
na mocku všechny scény, ruční titulky, StyleMap scéna i přechod (lokální demo balíček).
**Neověřeno:** mikrofon v prohlížeči se skutečným Translatorem, reálné balíčky
z FineTuneGallery (za CF Access), výkon shaderů na TV a mini PC.

## 2026-11-04: menší věci (vzkazy, hra, party epizoda)

| Část | Stav |
|---|---|
| ručně psané vzkazy ze sad HandWrittenStickers, výběr písma s náhledem v telefonu | ✅ |
| hra Kruh: plátno, ovladač v telefonu, start z Cockpitu, autopilot, PANIC | ✅ |
| party epizoda webtoonu ze schválených fotek a vzkazů | ✅ |
| oprava: ID hostů už nejdou ve frontě ani v losování | ✅ test |

Ověřeno: Go testy (sady písmen, hra, soukromí ID) a v Chrome zeď vzkazů v šesti
písmech, Kruh se 4 a 3 hráči na autopilotu (ztráta životů, vypadnutí), ovladač
v telefonu a party epizoda. **Neověřeno:** hra s několika skutečnými telefony
naráz (latence na Wi-Fi), konec hry s vyhlášením vítěze.

## 2026-11-05: bezpečnostní průchod (D19)

| Část | Stav |
|---|---|
| klíč plátna, konec party, limity na IP, hosta a sockety | ✅ testy |
| tickety místo tokenů v URL, seznam party jen s admin tokenem, soubory podle stavu, origin, max 36 Mpx | ✅ testy |
| hlavičky (CSP zatím report-only), obecné chyby, přezdívky, rotace tokenu | ✅ testy |
| ikona appky (favicon, PWA, Apple touch) | ✅ |

**Neověřeno v prohlížeči:** Chrome pro automatizaci byl během práce nedostupný.
Projít: Cockpit → „Otevřít plátno“ (klíč zmizí z URL), host vidí svoje čekající
fotky, Cockpit náhledy čekajících, mikrofon s ticketem, CSP v konzoli bez hlášek.

## 2026-11-06: Party box (D20)

| Část | Stav |
|---|---|
| `make dist` (linux arm64/amd64), `deploy/box` instalátor, systemd, certifikát, update | ✅ cross-compile, `shellcheck` |
| box mode: `/box`, automatická party, párovací QR, LAN adresa v QR, limity Pi | ✅ testy + Chrome na Macu |
| sd_notify a watchdog, kontroly certifikátu a teploty | ✅ testy |
| runbook `deploy/box/README.md` | ✅ |

**Neověřeno:** nic z toho neběželo na Raspberry Pi. Na Pi projít runbook
od bodu 1: kiosk pod `cage` na tty1, výkon scén a 4 streamů, teplota, `lego`
s Cloudflare, router s vlastním DNS záznamem.

## 2026-11-06: Cockpit v Reactu (D21)

| Část | Stav |
|---|---|
| `frontend-react/`: React 19 + TS, parita všech 12 sekcí Svelte Cockpitu | ✅ |
| backend: `/react/` z binárky, `PC_COCKPIT_UI`, prázdné seznamy jako `[]` | ✅ testy |
| tsc strict, ESLint strict type-checked + react-hooks, Vitest (reducer, helpery, přihlášení → socket → schválení, rotace tokenu) | ✅ v `make check` |

Ověřeno v Chrome: Svelte a React Cockpit nad jednou party s plátnem a hostem.
Vzkaz schválený v Reactu zmizel v obou a objevil se na plátně, scéna přepnutá
ze Svelte se ukázala v Reactu. Po vypnutí backendu React ukázal offline a po
návratu se sám připojil se stejným stavem. S `PC_COCKPIT_UI=react` vedl odkaz
`/cockpit/<kód>#token=…` do Reactu, ten se přihlásil a token z adresy smazal.
Konzole bez chyb. Neověřeno: náhledy živých streamů a mikrofon v Reactu
(potřebují kameru a https), React varování v dev buildu jen přes testy.

## 2026-11-06: Notebook jako party box, restyle přes ComfyUI (D22)

| Část | Stav |
|---|---|
| release: Linux arm64/amd64, Windows zip, návod pro notebook | ✅ `make dist` |
| `.env` načítaný binárkou, LAN adresa v QR z localhost, párovací QR mimo box | ✅ testy |
| `RESTYLE_PROVIDER=comfy` (restyle_flux v ComfyUI), `COMFY_TOKEN`, prefix `party_` | ✅ test proti fake ComfyUI |

Neověřeno: restyle_flux na skutečném ComfyUI na L40S (modely PuLID a depth
musí být na VM), Windows build na skutečném Windows, Cloudflare Tunnel.

## 2026-11-11: Stripy, Pásy, StyleMapy ze session, Scene Studio (D23)

| Část | Stav |
|---|---|
| Webtoon z katalogu Toonfork: import zipů (sha256), 1–4 stripy vedle sebe, bubliny z dat | ✅ testy + Chrome |
| Pásy: fotky v pásech cik cak, klipy místo fotek | ✅ Chrome (mock) |
| Vlasy ve větru (`breeze`, Wan): čeká na ✓, hraje místo fotky | ✅ testy, mock v Chrome |
| StyleMap packy ze session FineTuneGallery (Go, podle promptu), sweep až 3×3 | ✅ testy + Chrome (falešná galerie) |
| Scene Studio: show, editor scén, náhled, fotky, knihovna webtoonu a StyleMap, offline kontrola, knihovna show | ✅ testy + Chrome |
| Lišta show v obou Cockpitech | ✅ Chrome |

Ověřeno v Chrome na mocku (`127.0.0.1:18098`): stažený gamebook Sára
z živého katalogu, tři sloupce webtoonu s českými bublinami, pásy z 9 fotek
(sloupce střídavě nahoru a dolů), tři packy z jedné session (falešná
FineTuneGallery ze statických souborů) jako dlaždice. Show se čtyřmi
scénami: kontrola offline 4/4, pak server s nedostupnou galerií i
katalogem přehrál všechny scény v pořadí bez rozbitého obrázku. Dvě
spuštění pásů se stejným seedem se po 3 s lišila o 1 px. Uložení show do
knihovny a načtení do nové party funguje i z UI.
**Neověřeno:** restyle a klipy na skutečném ComfyUI (SPARK okno nebo L40S),
`breeze` na Wan (kvalita vlasů), výkon 4 sloupců s animovanými panely a pásů
s klipy na Raspberry Pi, reálná session FineTuneGallery (za CF Access).

## Další kroky
1. Vyplnit `.env` (Restyler token, CF Access token) a vyzkoušet s živým Restylerem a DJfy.
2. Rozhodnout, kde poběží party box. Na JODA je to Compose s portem v LAN (viz `deploy/`).
3. M3 dokončit: MirrorBooth podle `docs/mirrorbooth-stream.md`, test na LAN s telefony.
4. M4 dokončit: domluvit v L40S session okno s ComfyUI (profil D) a ověřit
   `ANIMATE_PROVIDER=comfy` celou cestou (telefon → Cockpit → plátno). Prověřit licenci
   vah LivePortrait pro B2B.
5. Party box: projít `deploy/box/README.md` na skutečném Raspberry Pi 5.
6. M7 ověřit naostro: Translator s ASR (okno na SPARKu přes broker), reálné StyleMap balíčky (CF token). M5 3D tanečníci.
7. Studio naostro (D23): SPARK okno přes broker, `COMFY_URL`, restyle a `breeze` na skutečném ComfyUI, pak show offline na notebooku podle `docs/studio.md`.
