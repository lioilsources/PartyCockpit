# Zadání: Cockpit v Reactu (druhá, nezávislá implementace)

Jsi Claude session v repu PartyCockpit (veřejné repo
https://github.com/lioilsources/PartyCockpit). Napiš druhou implementaci
Cockpitu (operátorské UI) v **React 19 + TypeScript**, vedle stávající ve
Svelte 5. Obě implementace žijí v repu trvale, mluví se stejným backendem
a dají se přepínat. Zadání je samostatné: všechno, co potřebuješ, je tady,
v `CLAUDE.md`, `DECISIONS.md` a v kódu.

## Proč

Portfolio: pozice vyžaduje working-to-advanced TypeScript, JavaScript
a React. Dvě implementace stejného UI nad jedním protokolem jsou zároveň
A/B test protokolu: druhý klient odhalí, kde se Cockpit opírá o náhodu
(pořadí zpráv, implicitní stav). Rozhodnutí zapiš jako **D21** do
`DECISIONS.md` (šablona dole). Odchylka od `~/.claude/rules/node.md`
(„web je Astro nebo SvelteKit“) je vědomá a patří do D21.

## Než začneš

1. Přečti `CLAUDE.md` (invarianty, build, mapa), D12, D17, D19, D20
   v `DECISIONS.md` a celý `frontend/src/cockpit/Cockpit.svelte` (937 řádků,
   referenční implementace). Protokol: `frontend/src/lib/ws.ts`,
   `lib/api.ts`, `lib/types.ts`; routy `backend/internal/api/server.go`
   řádky 125–200.
2. `make check` musí projít na začátku i po každém commitu. Výchozí stav:
   Go testy zelené, `shellcheck ok`, `svelte-check` 0 chyb, `vite build` ok.
3. Go testy pouštěj s `-timeout 120s`. První překlad `modernc.org/sqlite`
   trvá minuty; když Bash vyprší, pusť znovu.
4. Testovací server: `PC_ADDR=127.0.0.1:18098`, data do dočasného adresáře,
   mock providery (bez `.env` tokenů). Proces zabíjej `pkill -x partycockpit`
   (`pkill -f` trefí i shell, který ho spouští).
5. Git: hook blokuje `git add -A`; přidávej soubory jmenovitě. Commituj po
   každém kroku, **nepushuj** bez pokynu uživatele. Do repa nikdy LAN IP,
   tokeny ani domácí cesty.
6. Chrome ověření přes `mcp__claude-in-chrome__*`. Screenshoty pro uživatele
   posílej `SendUserFile`. Snímky přes redirect občas padají na timeout;
   pak ověř DOM přes `javascript_tool`.

## Rozhodnutí, která jsou daná

| Věc | Rozhodnutí | Proč |
|---|---|---|
| Umístění | `frontend-react/` jako vlastní Vite projekt (vlastní `package.json`, `tsconfig`, lockfile) | Nezávislá implementace, vlastní toolchain, přehledné pro čtenáře portfolia |
| Sdílený kód | Transport a typy se **sdílejí**: alias `@pc` → `../frontend/src`, importy `@pc/lib/api`, `@pc/lib/ws`, `@pc/lib/types`, `@pc/lib/rtc`, `@pc/lib/mic`, `@pc/lib/beat`, `@pc/lib/stylemap`, `@pc/app.css`. Nic ze `.svelte` se neimportuje. | Protokol má jeden zdroj pravdy; obojí UI jde přes stejné klíče v `localStorage`. Co je React, je React: hooky, komponenty, reducer. |
| Stav | `useReducer` nad WS zprávami: čistá funkce `cockpitReducer(state, msg)` v `src/state/reducer.ts`, bez knihoven (žádný Redux, Zustand, TanStack Query) | Server posílá celý stav při každém connectu; reducer je testovatelný bez DOM |
| Styly | CSS Modules (`*.module.css`), tokeny z `@pc/app.css` (`--bg-2`, `--panel`, `--pink`, …), fonty `@fontsource/space-grotesk` a `@fontsource/caveat` jako závislosti | Stejný vzhled jako Svelte Cockpit bez globálních kolizí |
| Router | Žádný. Parse cesty jako `frontend/src/App.svelte`: `/react/cockpit/<CODE>` | Jedna routa |
| Testy | Vitest + `@testing-library/react` + `jsdom`; ESLint flat config s `typescript-eslint`, `eslint-plugin-react-hooks`, `eslint-plugin-react-refresh`; Prettier stejné nastavení jako `frontend/` (pokud tam config není, `printWidth: 160, singleQuote: true`, jak odpovídá stávajícímu kódu) | Zadání pozice: TS a React na úrovni, kterou lze obhájit v code review |
| Verze | React 19.x, Vite 7.x, TypeScript 5.9+ (stejný major jako `frontend/`, pokud @vitejs/plugin-react nezvládne vyšší), `@vitejs/plugin-react` | |
| Servírování | Build do `backend/internal/web/dist/react/` s `base: '/react/'`. Backend servíruje `react/index.html` pro cesty pod `/react/`. | Jedna binárka, obě UI vložená |
| Přepínač | `PC_COCKPIT_UI=svelte` (výchozí) nebo `react`. S `react` backend přesměruje `GET /cockpit/<code>` 302 na `/react/cockpit/<code>`. Prohlížeč zachová `#token=…` (fragment se při 302 nese dál), takže párovací QR z boxu i odkazy z Home fungují beze změny. | Výběr implementace na jednom místě, QR a odkazy se nemění |
| Home | Vedle tlačítka „Cockpit“ přibude „Cockpit (React)“ → `/react/cockpit/<code>` | Obě UI dostupná bez přepínače |

## Parita: co React Cockpit umí (všechno ze Svelte verze)

Pořadí sekcí a texty UI zachovej (česky, stejné popisky), aby se dala obě UI
porovnat vedle sebe. Zdroj pravdy je `Cockpit.svelte`; tady je kontrolní seznam.

1. **Login** (`src/components/Login.tsx`): token z `#token=` (nebo starší
   `?token=`) se přečte jednou, uloží do `pc.cockpit.<code>` a smaže z URL
   (`history.replaceState`), D19. Bez tokenu formulář (`type=password`).
   `GET /api/parties/<code>/cockpit` ověří token; 401 → „Neplatný cockpit
   token“.
2. **Socket**: `connect(code, 'cockpit', onMsg, onStatus, async () => ({ticket}))`,
   ticket z `POST …/cockpit/ticket` před každým (re)connectem. Zprávy:
   `party`, `cockpit`, `queue`, `music`, `tempo`, `reaction` (posledních 14),
   `streams`, `rtc` → `viewer.handle`, `hello` → `viewer.reset()`,
   `subtitle` (update podle `id`, max 6), `subtitle.clear`.
3. **Header**: název, kód, tečka online/offline, počty klientů, restyle/animate
   z `config`, odkaz „Otevřít plátno“ (`/stage/<code>?key=<stageKey>`),
   `joinUrl`, „končí HH:MM“ + tlačítko „+2 h“ (stejný výpočet stropu 24 h),
   PANIC (`POST /panic`).
4. **Scény**: 12 tlačítek (`slideshow`…`blank`), aktivní zvýrazněná; parametry
   `beats` (interlude min. 16), `style`, `episode` (`GET /api/webtoon`
   + „🎉 Party epizoda“), `shader`, StyleMap `pack` + `transition`
   (`listPacks()`, uložené v `pc.stylemap`, `pc.stylemap.transition`),
   odkaz na recap (`recapUrl`).
5. **Před party** (`<details>`): při prvním otevření `GET /preflight`,
   tlačítko ↻, souhrn „N problémů / upozornění / v pořádku“ s českými
   plurály (`plural`), ruční seznam s checkboxy v `pc.runbook.<code>`,
   „Vyměnit cockpit token“ (`POST /cockpit/rotate` → nový token, uložit,
   restart socketu; místo `confirm()` dvoukrokové tlačítko „Vyměnit → Opravdu
   vyměnit“, viz Chrome dialogy), „Párovací QR na plátno“ jen když
   `party.box`.
6. **Ke schválení**: pending karty (sticker barevně, jinak `Clip` přes
   `fileUrl(code, file, config.fileKey)`), nickname, tanec, styl, fallback,
   NSFW skóre (varování > 0,15), ✓/✕, „Schválit vše“ sekvenčně, přepínač
   strict/relaxed s vysvětlivkou, „maluje se“ s `ago()`, selhalo, zamítnuté
   (12, „vrátit“).
7. **Show fronta**: `autoLive` checkbox, „✨ Oživit galerii“ (`queued: 0` →
   hláška), loop/shuffle, seznam s ↑ ↓ ✕ (`PUT /queue {ids}`,
   `DELETE /queue/<id>`), náhled `a.live ?? a.file`.
8. **Živé streamy**: `Viewer` z `@pc/lib/rtc` v `useRef`; `sync(previews ?
   streams : [])` a `setIce(party.iceServers)` v efektu; `<video>` s
   `srcObject` přes komponentu `StreamVideo` (efekt nastaví
   `video.srcObject`, ne atribut); pustit / stáhnout / ✕; „N/9 na plátně“.
   Čti `maxLive` z `party.box` jen pokud ho server posílá; jinak 9.
9. **Hudba**: DJfy kód (`PATCH {djfyCode}`), now playing, TAP tempo
   (`TapTempo`, „(n/4)“), zdroj tempa, „zrušit tap“, reakce.
10. **Titulky**: `Mic` z `@pc/lib/mic` (vyžaduje `window.isSecureContext`),
    jazyky A/B v `pc.sub.langA/B`, meter, psaný řádek (`POST /subtitle`),
    seznam s překladem, „smazat“ (`DELETE /subtitle`). Bez
    `config.subtitles` jen psaní.
11. **Losování**: hráči `wantsToPlay`, formulář (hra, počet 1–8),
    „Spustit Kruh“ (`POST /game`), „Konec hry“ (`DELETE /game` + scéna blank).
12. **Náměty**: seznam od nejnovějšího, „stáhnout JSON“ (`GET /ideas`
    s Bearer → Blob → download).

Chování ke zachování: po `rotate` restart socketu; chyby z `act()` do jednoho
`err` řádku pod headerem; `err` se maže před každou akcí.

## Struktura `frontend-react/`

```
frontend-react/
  package.json  vite.config.ts  tsconfig.json  eslint.config.js  index.html
  src/
    main.tsx            # StrictMode, fonty, @pc/app.css
    App.tsx             # parse /react/cockpit/<CODE>, jinak odkaz na Home
    state/
      reducer.ts        # CockpitStateModel, cockpitReducer(state, WsMsg)
      reducer.test.ts
    hooks/
      useCockpitSocket.ts  # connect + reducer + ticket; vrací {state, online, send}
      useViewer.ts         # Viewer v useRef, rev tick přes useSyncExternalStore
      useMic.ts            # Mic přes useSyncExternalStore
      useLocalStorage.ts   # store.get/set s JSON a fallbackem
      useAction.ts         # act(): err state + wrapper
      useTapTempo.ts
    api/
      cockpitApi.ts        # typované volání endpointů (p(path), token) nad @pc/lib/api
    components/
      Login.tsx Header.tsx ScenePanel.tsx Preflight.tsx Moderation.tsx
      ShowQueue.tsx Streams.tsx StreamVideo.tsx Music.tsx Subtitles.tsx
      Lottery.tsx Ideas.tsx Clip.tsx AssetCard.tsx
      *.module.css
    test/
      setup.ts             # jsdom, fake WebSocket, fetch mock
      Moderation.test.tsx  # schválení volá správný endpoint
      Login.test.tsx       # #token → localStorage, URL bez tokenu
```

React zásady, které bude posuzovat čtenář: komponenty jen s props a hooky,
žádný globální stav mimo `CockpitProvider` (context nad reducerem);
instance s životním cyklem (`Viewer`, `Mic`, `TapTempo`) v `useRef`
s úklidem v efektu; `useSyncExternalStore` pro třídy s vlastním `onChange`;
`useCallback`/`useMemo` jen tam, kde to měří důvod (seznamy karet s videi);
žádné `any` kromě hranice WS zprávy (`WsMsg.data`), tam `satisfies`/úzké
parsery; `key` vždy id, nikdy index; formuláře řízené; `onToggle` na
`<details>`; žádný `dangerouslySetInnerHTML`. StrictMode zapnutý: dvojí
mount efektů nesmí otevřít dva sockety (cleanup zavře).

## Backend (malé)

- `config.go`: `CockpitUI string // PC_COCKPIT_UI=svelte|react`, validace
  hodnot; `.env.example` řádek s komentářem.
- `server.go` `spa()`: cesta začínající `/react/` padá na `react/index.html`
  (když existuje; jinak 404 „react UI not built“). Cache hlavička immutable
  pro vše, co obsahuje `/assets/`, ne jen prefix.
- Routa `GET /cockpit/{code}`: s `CockpitUI == "react"` 302 na
  `/react/cockpit/{code}` (bez query, fragment nese prohlížeč). Jinak `spa`.
- CSP Report-Only (`server.go` ~209): React build nepotřebuje nic nad
  `script-src 'self'`; ověř v konzoli, že nepřibyly report hlášky.
- Test `backend/internal/api/web_test.go`: `/react/cockpit/X` servíruje
  `react/index.html` z fake FS; redirect při `react`, `spa` při `svelte`.

## Build a nástroje

- `Makefile` `web`: po Svelte buildu `cd frontend-react && npm ci && npm run build`
  a kopie do `backend/internal/web/dist/react/`. `check`: navíc
  `cd frontend-react && npm run check` (`tsc --noEmit && eslint . && vitest run`).
- `dev`: React dev server na `:5174` s proxy `/api /ws /files /webtoon
  /stylemaps /glyphs` na backend (zkopíruj z `frontend/vite.config.ts`),
  `server.fs.allow` musí povolit `../frontend/src` kvůli aliasu.
- `.gitignore`: `frontend-react/dist/`, `frontend-react/node_modules/`
  (`node_modules/` už je obecně).
- `make dist` dědí z `web`, nic navíc.

## Commity

Každý commit: `make check` zelený, anglická zpráva, trailer podle instrukcí
session.

1. **„React cockpit 1/4: scaffold, embed, route“**: `frontend-react/`
   s Vite + React + TS + ESLint + Vitest, alias `@pc`, Login + Header +
   online tečka nad `useCockpitSocket` a reducerem (jen `party`, `cockpit`),
   backend `/react/`, `PC_COCKPIT_UI`, Makefile, `.gitignore`, test spa.
   Ověř v Chrome: `/react/cockpit/<code>#token=…` se přihlásí a ukáže header.
2. **„React cockpit 2/4: scenes, moderation, queue“**: reducer kompletní
   (všechny typy zpráv) + `reducer.test.ts`; ScenePanel, Moderation,
   ShowQueue, Clip, AssetCard, Preflight.
3. **„React cockpit 3/4: streams, music, subtitles, lottery, ideas“**:
   useViewer + StreamVideo, Music + useTapTempo, Subtitles + useMic,
   Lottery, Ideas. Konec parity.
4. **„React cockpit 4/4: tests, docs, Home link“**: komponentové testy,
   Home tlačítko, D21, `STATUS.md`, `README.md` sekce „Cockpit v Reactu“,
   `CLAUDE.md` mapa (1 řádek: `frontend-react/` — React Cockpit, D21;
   strop 80 řádků) a `make check` popis.

## Ověření před posledním commitem (Chrome, mock providery)

1. Jedna party, dvě záložky: `/cockpit/<code>` (Svelte) a
   `/react/cockpit/<code>` (React), plátno ve třetí. Oba Cockpity online.
2. Host (čtvrtá záložka, `/join/<code>`) pošle vzkaz. Objeví se v obou
   Cockpitech „Ke schválení“. Schválit v Reactu → zmizí v obou, je ve frontě
   obou, plátno ho ukáže.
3. Přepnout scénu v Reactu → Svelte Cockpit i plátno ji ukážou; a naopak.
4. Vypnout backend na 20 s → tečka červená, po návratu zelená bez reloadu,
   stav obnovený (server posílá vše na connect).
5. `PC_COCKPIT_UI=react`: `curl -sI http://127.0.0.1:18098/cockpit/<code>`
   dá 302 na `/react/cockpit/<code>`; v Chrome odkaz s `#token=` se přihlásí.
6. Konzole React záložky: 0 chyb, 0 varování React (keys, controlled inputs),
   0 CSP reportů.
7. Screenshot obou Cockpitů vedle sebe pro uživatele.

## D21 šablona do `DECISIONS.md`

```
## D21: Cockpit podruhé, v Reactu (2026-11-xx)
Zadání: `Prompts/03-COCKPIT-REACT.md`.
- Druhá implementace Cockpitu v React 19 + TS v `frontend-react/`, vedle
  Svelte. Důvod: portfolio pro pozici s požadavkem na React a TypeScript;
  vedlejší efekt je druhý klient nad stejným protokolem.
- Sdílí se jen transport a typy (`frontend/src/lib/*.ts` přes alias `@pc`)
  a klíče v localStorage, UI a stav jsou nezávislé (reducer + hooky).
- Obě UI vložené v binárce: `/cockpit/<code>` Svelte, `/react/cockpit/<code>`
  React; `PC_COCKPIT_UI=react` přesměruje první na druhé (fragment s tokenem
  přežije 302).
- Odchylka od pravidla „web je Astro nebo SvelteKit“: vědomá, jen pro Cockpit.
- Co druhý klient odhalil o protokolu: … (doplň, i když nic)
```

## Co nedělat

- Nepřepisuj Svelte Cockpit, Stage, Guest ani Home kromě jednoho tlačítka.
- Neměň WS protokol ani API; když React potřebuje něco, co Svelte nepotřebuje,
  je to nález do D21, ne změna serveru (výjimka: `spa`, redirect, config).
- Žádné UI knihovny (MUI, shadcn, Tailwind). Žádné state knihovny.
- Žádný `confirm()`/`alert()`: blokují Chrome automatizaci i kiosk.
- Nepushuj. Na konci napiš uživateli česky, co je hotové, co bylo ověřeno
  v Chrome a co druhý klient odhalil.
