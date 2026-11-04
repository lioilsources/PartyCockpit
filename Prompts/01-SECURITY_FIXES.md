# Zadání: bezpečnostní opravy PartyCockpitu

Jsi Claude session v repu PartyCockpit (veřejné repo
https://github.com/lioilsources/PartyCockpit). Proveď opravy z
`docs/SECURITY_PLAN.md` ve třech commitech. Tohle zadání je samostatné: všechno,
co potřebuješ vědět, je tady, v `CLAUDE.md`, `DECISIONS.md` a v kódu.

## Než začneš

1. Přečti `CLAUDE.md` (invarianty, build, mapa), `docs/SECURITY_PLAN.md`
   (nálezy a odůvodnění) a D12, D14, D17 v `DECISIONS.md`.
2. `make check` musí projít na začátku i po každém commitu. Výchozí stav:
   Go testy zelené, `svelte-check` 0 chyb (varování
   `state_referenced_locally` jsou známá, nepřidávej nová), `vite build` ok.
3. Go testy pouštěj s `-timeout 120s`. První překlad `modernc.org/sqlite`
   trvá minuty; když Bash vyprší, pusť znovu, nic nevisí.
4. Testovací server: `PC_ADDR=127.0.0.1:18098` (8098 drží cizí proces).
   Data do dočasného adresáře, ne do `./data`.
5. Git: hook blokuje `git add -A`; přidávej soubory jmenovitě. Commituj po
   každém kroku, **nepushuj** bez pokynu uživatele. Do repa nikdy LAN IP,
   tokeny ani domácí cesty.
6. Chrome ověření dělej přes `mcp__claude-in-chrome__*` (plátno, Cockpit,
   host). Screenshoty, které má uživatel vidět, posílej `SendUserFile`.

## Konvence v kódu

- Backend: Go 1.26, `net/http` ServeMux s `{code}` patterny, `coder/websocket`,
  `modernc.org/sqlite`, `slog`. Chyby `%w`. Migrace se neupravují, přidává se
  nová na konec `migrations` v `store/schema.go`. Testy bez sítě (httptest je
  v pořádku) a bez `time.Now` v tělech testů: používej `Store.Now`
  (`e.srv.Store.Now`), ten jde v testu podstrčit.
- Testovací helpery v `backend/internal/api/streams_test.go`: `newEnv(t)`
  (celý server + party), `e.do(method, path, body, out)` (s cockpit tokenem),
  `e.dial(query)` → `sock` s `waitFor`, `none`, `send`; `e.upload(gid)`
  a `e.waitAsset` v `animate_test.go`. Nové testy dej do
  `backend/internal/api/security_test.go`.
- Frontend: Svelte 5 runes, TS. `lib/ws.ts` `connect(code, role, onMsg,
  onStatus, params)`; `lib/api.ts` `api.get/post/…(path, body, token)`
  a `store` (localStorage s try/catch). Texty v UI česky.
- Komentáře v kódu anglicky, krátké, říkají proč.

## Commit 1: „Stage key, party end, limits“ (body 1–4 plánu)

### 1a. Klíč plátna
- Migrace 3: `ALTER TABLE parties ADD COLUMN stage_key TEXT NOT NULL DEFAULT '';`
  Pro existující party doplň klíč líně: když je `stage_key` prázdný při čtení
  party, vygeneruj `NewID(16)` a ulož (v `Store.Party`), aby staré DB fungovaly.
- `Party.StageKey` s `json:"-"`. `createParty` vrací `stageKey` vedle
  `cockpitToken`; `cockpitPayload().Config["stageKey"]`.
- `ws`: `role=stage` vyžaduje `?key=` rovný `StageKey` (constant time `eq`),
  jinak 401. `role=cockpit` zůstává přes `bearer`.
- Frontend: `Home.svelte` odkaz Plátno `/stage/<kód>?key=…` (Home má tokeny
  z `createParty`); `Cockpit.svelte` „Otevřít plátno“ s klíčem z
  `st.config.stageKey`. `Stage.svelte`: klíč z `?key=` uloží do
  `pc.stage.key.<kód>`, smaže z URL (`history.replaceState`), předá do
  `connect(..., { key })`. Bez klíče ukáže plátno hlášku „otevři plátno
  z Cockpitu“ místo prázdné obrazovky.
- Preflight: `stage` check hlásí počet připojených pláten, text doplň
  „plátno otevři tlačítkem v Cockpitu (má klíč)“.
- Test: `role=stage` bez klíče a se špatným → dial selže se 401;
  se správným dostane `party` a `streams`. Socket `role=guest` nikdy nedostane
  `streams` (použij `none`).

### 1b. Konec party
- Nový wrapper `s.guestWrite(h)` = `s.guest` + kontrola
  `p.EndsAt < Store.Now()` → 410 `{"error":"party skončila"}`. Použij pro
  všechny `POST/PUT/DELETE` pod `/guests/{gid}/` a pro `POST /guests`.
  Čtení (`GET …/assets`, `/queue`, `/recap`, `/files`) zůstává.
- `PATCH /api/parties/{code}` přijme `endsAt` (ms); povolit jen hodnoty mezi
  teď a teď + 24 h, jinak 400. Cockpit: v hlavičce vedle „končí za …“
  tlačítko „+2 h“.
- Test: `Store.Now` je pole `func() time.Time` (viz `store_test.go:102`);
  v `env` ho přepiš na čas za `ends_at`: upload 410, recap 200, PATCH
  `endsAt` dopředu → upload 202.

### 1c. Limity
- Stropy: 400 hostů na party (409 „party je plná“), 200 vzkazů a 100 námětů
  na hosta (409), 20 vzkazů/min na hosta (429). Počty přes jednoduché
  `SELECT COUNT(*)` ve `store`.
- Nový soubor `backend/internal/api/ratelimit.go`: token bucket na IP
  (30 požadavků / 10 s pro zápisy mimo cockpit), mapa s mutexem a úklidem
  starších 10 min, IP z `RemoteAddr`; `X-Forwarded-For` jen při
  `PC_TRUST_PROXY=1` (nový config). Middleware v `Routes()` před `logRequests`.
  `recap.zip` stojí 10 tokenů.
- WS: max 8 spojení na IP a party pro `role=guest` (hub potřebuje počítadlo
  podle IP: přidej do `hub.Peer` pole `IP`, `Hub` drží `map[code]map[ip]int`;
  nad limit `Accept` vůbec neproběhne, vrať 429).
- Test: 31. POST z jedné IP za 10 s → 429 (bucket přes `Store.Now`, ne
  reálný čas: dej bucketu `now func() time.Time`); 9. guest socket odmítnut;
  401. host 409.

### 1d. Stropy na socketech
- `micSocket`: `SetReadLimit(64<<10)`, max 50 rámců/s (počítadlo za sekundu,
  nad to rámce zahazuj, neodpojuj).
- `onGameInput`: max 30 zpráv/s na peer (počítadlo v `game` podle `GuestID`),
  nad to zahodit.
- Test: 60 binárních rámců za sekundu na mic → spojení žije, fake Translator
  dostal nejvýš 50.

Commit zpráva začíná `Security 1/3: stage key, party end, rate limits`.

## Commit 2: „Tokens out of URLs, admin list, files by status, origin, image size“ (body 5–9)

### 2a. Token mimo URL
- Home předává token přes `location.hash` (`#token=…`), Cockpit ho čte
  z `location.hash`, uloží a smaže. Zpětně podporuj i `?token=` ještě jednu
  verzi (poznámka v kódu).
- Stahování námětů: `fetch` s `Authorization` → blob → `a.download`.
- WS cockpit a mic: `POST /api/parties/{code}/cockpit/ticket` (cockpit auth)
  vrátí `{ticket}` platný 30 s, jednorázový, uložený v paměti serveru;
  `ws?role=cockpit&ticket=…` a `/ws/…/mic?ticket=…`. `bearer()` dál bere
  `?token=` kvůli zpětné kompatibilitě, ale `logRequests` nikdy neloguje
  query (ověř).
- Test: ticket použitý dvakrát → druhý 401; po 31 s 401 (čas přes `Store.Now`).

### 2b. Admin
- `GET /api/parties` vrací 401 vždy, když `PC_ADMIN_TOKEN` je prázdný
  (seznam s cockpit tokeny nesmí být bez autentizace). `POST /api/parties` bez
  tokenu zůstává; při startu `slog.Warn("admin token not set")` a preflight
  `warn` „kdokoli v síti může založit party“.
- Test: prázdný token → list 401, create 201.

### 2c. Soubory podle stavu
- `store`: index `assets(file)` (migrace 4) a `AssetByFile(code, file)`.
- `serveFile`: vydá jen `status=approved`, nebo `kind=live` jehož zdroj je
  approved. Jinak 404. Hlavička `Content-Disposition: inline`,
  `X-Content-Type-Options: nosniff`.
- Nový `GET /api/parties/{code}/guests/{gid}/files/{name}` (wrapper `guest`):
  vydá soubor jen když asset patří hostovi (jakýkoli stav). Frontend
  `fileUrl` pro vlastní položky v `Guest.svelte` („Moje“) použije tuhle cestu.
- Test: pending soubor veřejně 404, přes guest endpoint 200, cizí host 404;
  approved veřejně 200; originál (`status=source`) veřejně 404 i po schválení
  restylu.

### 2d. Origin
- `websocket.Accept` s `OriginPatterns`: host požadavku, host z
  `PC_PUBLIC_URL`, `localhost:*` a `127.0.0.1:*`. Odstraň
  `InsecureSkipVerify`. Pozor na vite dev (`:5173` proxy) — ověř `make dev`.
- Test: `Origin: https://evil.example` → dial selže 403.

### 2e. Rozměry fotky
- `providers.Normalise`: před `image.Decode` zavolej `image.DecodeConfig`;
  nad 36 Mpx vrať chybu „fotka je moc velká (max 36 Mpx)“. Upload → 400.
- Test: PNG 8000×8000 vygenerovaný v testu (plocha jedné barvy, malý
  soubor) → 400.

Commit zpráva `Security 2/3: tickets instead of tokens in URLs, admin list, files by status, origin, image size`.

## Commit 3: „Headers and hygiene“ (body 10–15)

- Middleware hlaviček: `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, CSP
  `default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:;
  connect-src 'self' ws: wss:; worker-src blob:; style-src 'self'
  'unsafe-inline'; font-src 'self'`. Svelte vkládá inline styly, proto
  `unsafe-inline` u stylů. **Ověř v Chrome**, že plátno (Pixi, WebGL
  Interlude, AudioWorklet mikrofonu v Cockpitu, QR, PWA manifest), Cockpit
  i host jedou bez chyb v konzoli (`read_console_messages`).
- Chyby hostovi: `failAnimation`, `fallbackToOriginal` a `writeErr(500, …)`
  v guest handlerech posílají obecný text; detail do `slog` a do
  `meta.errorDetail`, který `GuestAssets` z meta vymaže (Cockpit ho vidí).
- `clean()` odstraní znaky kategorie `Cf` (zero-width, bidi). Test
  s `​` a `‮`.
- Duplicitní přezdívky: `POST /guests` při shodě (case-insensitive) vrátí 409
  `{"error":"přezdívku už někdo má","suggest":"Žofie 2"}`; `Guest.svelte` to
  ukáže a předvyplní návrh.
- `POST /api/parties/{code}/cockpit/rotate` (cockpit auth) vygeneruje nový
  token, uloží, vrátí ho a odpojí cockpit sockety (`Hub` potřebuje
  `Close(code, role)`); Cockpit má tlačítko v „Před party“ s potvrzením.
- `DECISIONS.md`: nové D19 „Bezpečnostní průchod“ se shrnutím všech tří
  commitů (co a proč, co se nechalo: `/metrics` veřejné, http v domácí LAN).
  `CLAUDE.md` invarianty: „Plátno se připojuje jen s `stage_key`“, „Zápisy
  hostů končí s `ends_at`“, „Zápisy mají limity na IP a hosta“. `STATUS.md`
  sekce s datem. `README` a `.env.example` (`PC_TRUST_PROXY`, poznámka
  k `PC_ADMIN_TOKEN`).

Commit zpráva `Security 3/3: headers, error hygiene, nickname clashes, token rotation`.

## Ověření na konci

1. `make check` zelené.
2. V Chrome: Home → založit party → Cockpit (token přes hash, v URL po
   načtení nic) → „Otevřít plátno“ (klíč v URL zmizí, plátno hraje) → host
   `/join/<kód>` nahraje fotku (mock), schválit, na plátně → host otevře
   vlastní „Moje“ (soubor přes guest endpoint) → `/files/<kód>/<pending>`
   v novém tabu je 404.
3. Pošli uživateli 3 screenshoty (Cockpit s klíčem plátna, plátno, 404 na
   pending souboru) a shrnutí: co je hotové, co jsi neověřil, co zbývá.
4. Nepushuj. Napiš, že je to připravené k pushi.

## Čeho se nedotýkat

- GPU providery (`providers/animate.go`, `providers.go`), Translator klient,
  workflow JSONy. SPARK ani L40S nijak nevolat.
- Migrace 1 a 2.
- Chování show fronty (D5) a schvalování (invarianty v `CLAUDE.md`).
- Plán `Prompts/00-PARTYKOKPIT_PLAN.md` — je historický, neupravuj.
