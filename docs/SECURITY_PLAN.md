# Bezpečnostní průchod API — plán oprav

**Hotovo 2026-11-05** ve třech commitech (Security 1/3–3/3), shrnutí v D19.
Odchylka: CSP je zatím `Report-Only`, dokud ji někdo neověří na skutečném plátně.

Stav k 2026-11-04 (po commitu `2a393c7`). Model hrozby: party box v LAN, kde jsou
cizí telefony (hosté i náhodní lidé na Wi-Fi podniku). Útočník zná kód party,
protože je na plátně. Neřešíme internet ani cílený útok na box samotný.

Co už platí: cockpit token (32 hex), ID hosta jako jeho klíč (D17: už nejde do
fronty ani losování), originály se na plátno nedostanou, názvy souborů jsou
náhodné, cesty u glyphs/stylemaps/webtoon jsou regexem omezené, JSON těla max
64 KB, fotka max 15 MB, WS 64 KB/zprávu a 40 zpráv/s na spojení, reakce
omezené na hosta.

## Priorita 1 — před první party

### 1. Role `stage` bez ověření → kdokoli vidí živé kamery
`GET /ws/parties/{code}?role=stage` nic nekontroluje. Kdo zná kód, dostane seznam
živých streamů a může jako „plátno“ poslat WebRTC offer, takže si na vlastním
telefonu pustí kameru kteréhokoli hosta (P2P, server to jen přepojí). Dostane
i show frontu a titulky, to je méně vážné (fronta je stejně veřejná v `/queue`).

- Fix: `stage_key` na party (migrace 3, `NewID(16)`). Cockpit tlačítko „Otevřít
  plátno“ otevře `/stage/<kód>?key=…`; plátno si klíč uloží do localStorage
  (stejně jako Cockpit token) a smaže z URL. `ws role=stage` bez správného klíče
  → 401. Klíč vrací `createParty` i `cockpitState` (`config.stageKey`).
- Test: `role=stage` bez klíče 401; se špatným 401; se správným dostane
  `streams`. Guest socket nikdy nedostane `streams` ani `rtc` od streamera.
- Dotkne se: `store/schema.go`, `store.go` (Party.StageKey, `json:"-"`),
  `api/server.go` (ws), `handlers.go` (createParty, cockpitPayload),
  `Stage.svelte`, `Cockpit.svelte`, `Home.svelte`, `preflight.go` (varovat,
  když je plátno otevřené bez klíče → nemůže být, ale hlásit 0 pláten).
- Pracnost: 2 h.

### 2. Nic nekončí s koncem party
`party()` nehlídá `ends_at`. Hosté mohou nahrávat, psát a reagovat do doby,
než janitor party smaže (7 dní po konci), a `/recap` běží dál (to je záměr).
- Fix: guest zápisy (`POST`/`PUT`/`DELETE` pod `/guests/`) a `POST /guests`
  po `ends_at` → 410 „party skončila“. Čtení a `/recap` zůstávají. Cockpit může
  party prodloužit: `PATCH {endsAt}` (nový, validovat max +24 h od teď).
- Test: po posunutí `Store.Now` za `ends_at` upload 410, recap 200.
- Pracnost: 1 h.

### 3. Zahlcení: hosté, vzkazy, náměty, spojení bez limitu
`POST /guests` nemá limit (DB a Cockpit zaplaví tisíce „hostů“), vzkazy
a náměty jsou omezené jen délkou, WS spojení na IP nejsou omezená.
- Fix:
  - max 400 hostů na party (409 „party je plná“), max 200 vzkazů a 100 námětů
    na hosta, max 20 vzkazů za minutu na hosta;
  - token bucket na IP pro zápisy (`POST`/`PUT`/`DELETE` mimo cockpit):
    30 požadavků/10 s, nad to 429. Jednoduchá mapa `ip → bucket` s úklidem,
    IP z `RemoteAddr` (za proxy `X-Forwarded-For` jen když `PC_TRUST_PROXY=1`);
  - max 8 WS spojení z jedné IP na party (mobil s několika záložkami stačí),
    stage/cockpit mimo limit;
  - `recap.zip` nejvýš 1× za 10 s na IP (stejný bucket, vyšší cena).
- Test: 31. POST z jedné IP 429; 401. host 409; 9. socket odmítnut.
- Pracnost: 3 h.

### 4. Vstupy pro Translator a hru bez stropu na socketu
`micSocket` přijímá binární rámce 256 KB bez omezení rychlosti a `onGameInput`
jede přes obecný limit 40/s. Hub limit platí jen na textové zprávy přes
`Serve`; mic socket má vlastní smyčku bez limitu.
- Fix: mic max 50 rámců/s a 64 KB/rámec (20 ms PCM16 16 kHz = 640 B, rezerva
  stačí); `game.input` max 30/s na spojení (ostatní zahodit, ne odpojit).
- Pracnost: 1 h.

## Priorita 2 — před veřejným nasazením (B2B, cizí Wi-Fi)

### 5. Cockpit token v URL
`/cockpit/<kód>?token=` (odkaz z Home), `/ideas?download=1&token=`, oba WS
(`?token=`). Query string končí v historii prohlížeče, v logu proxy a ve
screenshotech. Server token neloguje (`logRequests` loguje jen path).
- Fix: Home předává token přes `location.hash` (`#token=`) nebo
  `sessionStorage` místo query; stahování námětů přes `fetch` + blob
  s hlavičkou `Authorization`; WS: krátkodobý ticket `POST /cockpit/ticket`
  (30 s, jednorázový) místo tokenu v query. Pro `?token=` u WS zatím nechat
  fallback, odstranit v další verzi.
- Pracnost: 2 h.

### 6. Admin bez tokenu
`PC_ADMIN_TOKEN` prázdný = kdokoli v LAN zakládá party a vylistuje
`GET /api/parties` **včetně cockpit tokenů všech party**. To je nejhorší
následek: cizí telefon si vylistuje party a převezme Cockpit.
- Fix: `GET /api/parties` vždy vyžaduje token (i když je prázdný → 401 vždy,
  tedy bez tokenu seznam není). `POST /api/parties` bez tokenu zůstává pro
  domácí použití, ale preflight hlásí `warn` a start serveru logne varování.
  Nasazení B2B: `PC_ADMIN_TOKEN` povinný, `deploy/` ho vyžaduje.
- Test: list bez tokenu 401 i při prázdné konfiguraci.
- Pracnost: 30 min.

### 7. `/files/{code}/{name}` vydá cokoli ze složky party
Názvy jsou náhodné (20 hex), ale soubor rejected/pending/source je po uhodnutí
nebo úniku jména ke stažení komukoli s kódem. Jméno originálu vidí Cockpit
(ok) a host vlastní (ok); dřív unikalo frontou (opraveno D17).
- Fix: `serveFile` dohledá asset podle `file` (index `assets(file)`) a vydá
  jen `approved`, plus `live` klipy schválených fotek. Vše ostatní 404.
  Hostovi vlastní soubory (náhled originálu, čekající restyle) jdou přes nový
  `GET /guests/{gid}/files/{name}` (ověřený jeho ID).
- Test: pending soubor 404 veřejně, 200 přes guest endpoint; approved 200.
- Pracnost: 2 h.

### 8. Origin u WebSocketů
`InsecureSkipVerify: true` = jakákoli webová stránka otevřená v prohlížeči
v LAN se může připojit na `/ws/…`. Bez tokenu/klíče nedostane nic citlivého
(po bodu 1), ale jako `guest` s uhodnutým… ne, ID nezná. Jen obrana do hloubky.
- Fix: povolit origin = `Host` požadavku, `PC_PUBLIC_URL` a `localhost`
  (vite dev). Jinak 403.
- Pracnost: 30 min.

### 9. Fotky: obří rozměry
`Normalise` dekóduje celý obrázek. 15 MB PNG může mít 20 000×20 000 px
(1,6 GB v paměti) → OOM na mini PC.
- Fix: `image.DecodeConfig` před dekódováním, max 36 Mpx, jinak 400.
- Test: syntetický 8 000×8 000 PNG (malý soubor) → 400.
- Pracnost: 30 min.

### 10. Hlavičky
Žádné `X-Content-Type-Options`, CSP ani `X-Frame-Options`. Svelte escapuje,
soubory hostů jsou obrázky a mp4, ale `/files` servíruje podle přípony.
- Fix: middleware: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`
  (plátno nikdo neembeduje), `Referrer-Policy: no-referrer`, CSP
  `default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:;
  connect-src 'self' ws: wss:; worker-src blob:` (AudioWorklet je z blob URL)
  — ověřit, že Pixi/QR/PWA manifest projdou.
- Pracnost: 1 h včetně ověření v Chrome.

## Priorita 3 — hygiena

### 11. Chybové hlášky prozrazují backend
`meta.error` a `writeErr(500, err.Error())` posílají hostovi text chyby
z providera včetně URL Restyleru/ComfyUI a CF detailu.
- Fix: hostovi obecná hláška („nepovedlo se, zkus to znovu“), detail jen do
  logu a do Cockpitu (`meta.errorDetail` filtrovat v `GuestAssets`).

### 12. Neviditelné znaky a stejné přezdívky
`clean()` propouští zero-width a bidi znaky; dva hosté „Žofie“ se nerozliší
(recap „moje“ je jen pohodlí, D14).
- Fix: `clean` odstraní kategorii `Cf`; Cockpit zvýrazní duplicitní přezdívky;
  `POST /guests` při shodě vrátí 409 s návrhem „Žofie 2“.

### 13. Rotace cockpit tokenu
Token platí celou dobu. Když unikne (screenshot tabletu), nejde vyměnit.
- Fix: `POST /api/parties/{code}/cockpit/rotate` (s platným tokenem) vrátí nový,
  staré sockety odpojí.

### 14. `/metrics` a `/healthz` veřejné
Metriky jsou agregované (intent, status, počty). Nechat, zapsat do D8.

### 15. Plátno bez https přenáší klíč v plain
Po bodu 1 jde stage key po Wi-Fi v plain, stejně jako cockpit token dnes.
Preflight už varuje. Pro B2B: `PC_TLS_*` povinné a http jen redirect.

## Pořadí commitů

1. **„Stage key, party end, limits“** = body 1, 2, 3, 4 + testy. Mění schéma
   (migrace 3) a `Home`/`Stage`/`Cockpit`.
2. **„Tokens out of URLs, admin list, files by status, origin, image size“**
   = body 5, 6, 7, 8, 9.
3. **„Headers and hygiene“** = body 10, 11, 12, 13, 14, 15 + D19 do DECISIONS.

Každý bod má test v `internal/api` (nový `security_test.go`), aby regrese
nešla pushnout přes `make check`. Po všech třech: aktualizovat `CLAUDE.md`
(invarianty: stage key, hosté po konci party, limity) a `README`.

Celkem odhad: 1 den práce bez ostrého testu na telefonech.
