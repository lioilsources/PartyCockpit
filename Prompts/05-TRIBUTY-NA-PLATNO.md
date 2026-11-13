# Zadání: Tributy z PromoClown na plátno (proužek a fotomřížka)

Jsi Claude session (Opus) v repu PartyCockpit (veřejné repo
https://github.com/lioilsources/PartyCockpit). Jeden požadavek, jeden PR.

**Co vznikne.** PromoClown dělá „tributy“: obrázek cizího autora, ke kterému
noční úloha vyrenderuje 4 restylované varianty a připraví post s poděkováním
`@autor`. PartyCockpit si tyhle čtveřice stáhne na disk a nová scéna
`tributes` je hraje na plátně. Každý tribut je jeden bílý arch jako
z Restyleru: buď **mřížka 2×2** (`grid`), nebo **proužek 1×4** z fotobudky
(`strip`). Archy rolují ve sloupcích cik cak jako Pásy. Scéna jde zařadit do
show ve Studiu a na party běží offline.

## Než začneš

- Čti `CLAUDE.md`, `DECISIONS.md` (D19, D21, D22, D23), `STATUS.md`,
  `docs/studio.md` a `Prompts/03-COCKPIT-REACT.md` sekci „Než začneš“ (stejná
  pravidla: `make check` jako brána, Go testy s `-timeout 120s`, testovací
  server na `127.0.0.1:18098` s mock providery, `git add` jmenovitě, commit po
  kroku, **nepushovat bez pokynu**, do repa žádné LAN IP, tokeny ani domácí
  cesty). Testy bez sítě a bez `time.Now` (fake servery přes `httptest`, čas
  z `s.Store.Now()`).
- **Základ větve.** Zadání staví na D23 (PR #2 až #5, poslední větev
  `feat/studio-show`). Zjisti `gh pr list --state all`. Když jsou sloučené,
  větev `feat/tributes` z `main` a PR do `main`. Když ne, větev z
  `feat/studio-show` a PR s base `feat/studio-show`.
- Pracuj ve worktree. V hlavním checkoutu má Olin rozdělané
  `backend/internal/api/preflight.go` a `frontend/src/stage/Stage.svelte`:
  nech je být a necommituj je.
- Tenhle soubor leží v hlavním checkoutu necommitnutý. Zkopíruj ho do své
  větve a přidej do prvního commitu.
- GPU ani SPARK tahle práce nepotřebuje.

## Předpoklady (Olin je nepotvrdil, platí jako výchozí)

Když Olin některý změní, uprav jen dotčenou část.

1. „PromoTributes“ = tributy v PromoClown (`cmd/promo-tribute`, tabulka
   `tributes`). Jiné repo toho jména není.
2. Nabízí se tributy ve stavu `drafted`, jejichž post není `rejected`. Tedy
   i drafty, které ještě nikdo neschválil k publikaci. Studio u každého ukáže
   stav postu. Na plátno jde jen to, co operátor ve Studiu výslovně stáhl:
   stažení je schválení pro plátno.
3. Kredit `@autor` je na archu vidět vždy a nejde vypnout.
4. Původní obrázek autora se nestahuje a neukazuje, jen restyly.
5. PartyCockpit používá agent token PromoClown a posílá jen `GET`. Read-only
   role v PromoClown by byl samostatný PR v tamním repu.
6. Tributy jsou globální knihovna na disku jako webtoon packy. Sdílí je
   všechny party a do uložené show se nekopírují.

## Co průzkum sousedních rep zjistil (neprozkoumávej znovu)

**PromoClown** (`../PromoClown`, veřejné). promo-api běží jen v LAN na portu
8094. Adresa patří do `.env`; výchozí adresu ze zdrojáků PromoClown sem
**nekopíruj** (je to LAN IP).

- Auth: hlavička `X-Promo-Token`. Agent token čte a zakládá drafty, admin
  token navíc schvaluje a publikuje. Chyby jsou JSON `{"error": "…"}`.
- `GET /tributes?status=<s>&limit=<≤500>` (řazeno `id DESC`) vrací pole
  `{id, project, credit, source_path, note, status, post_id, styles[], error,
  submitted_by, created_at, attempts}`. Stavy: `queued | running | drafted |
  failed | cancelled`. `drafted` je konečný úspěch. `credit` je vždy
  `@handle` (`^@[A-Za-z0-9_.-]{1,64}$`). `styles[]` jsou ID stylů.
  `GET /tributes/{id}` vrací jeden.
- `GET /posts?status=&project=&platform=&limit=` (výchozí 100, strop 500,
  `id DESC`) a `GET /posts/{id}` vrací `{id, project, platform, kind, title,
  text, media_paths[], status, …}`. Stavy postu: `draft | approved |
  scheduled | published | rejected | failed`.
- Varianty jsou v `media_paths[]` postu, ve stejném pořadí jako `styles[]`
  tributu. Bývají 4, úloha jich umí 1 až 4 (`--variants`).
- `GET /assets/{path...}` vydá soubor (`http.ServeContent`). Cesta je hodnota
  z `media_paths` (relativní, tvar `<projekt>/uploads/<čas>-<jméno>.png`).
  Každý segment zvlášť přes `url.PathEscape`.
- Obrázky jsou PNG z SDXL kolem 1 MP: 1024×1024, 896×1152, 1152×896,
  832×1216, 1216×832, 768×1344, 1344×768. Čtveřice mívá stejný poměr stran,
  ale nespoléhej na to.
- Text postu má šablonu `"Tribute to {credit} 🎨\n\n{styles}\n\nby {by}"`.
  `{styles}` jsou názvy stylů spojené `" · "`. Šablona jde změnit přepínačem
  a řádek se styly vypadne, když se text nevejde do 280 znaků.
- Katalog stylů (ID → název) je v PromoClown vložený soubor a API ho nevydává.
- Časy jsou text `2006-01-02T15:04:05Z`.

**Restyler** (`lib/services/strip.dart`). `enum StripLayout { grid(2),
strip(1) }` je počet sloupců archu. `kStripCellWidth = 512`,
`kStripMargin = 24`, `kStripLabelHeight = 44`. Arch je bílý papír, buňky jsou
stejně velké v poměru stran prvního snímku, popisek je pod každou buňkou.
Šířka archu: `strip` 560 (24 + 512 + 24), `grid` 1096 (3×24 + 2×512).

**PartyCockpit** (stav po D23).

- `frontend/src/stage/scenes/Strips.svelte`: sloupce cik cak. Pozice je funkce
  času (`elapsed(startedAt)`), sloupec je vykreslený dvakrát pro smyčku bez
  švu, perioda je `offsetHeight / 2`. Proto mají položky `margin-bottom`, ne
  `gap`. Tempo sloupců je `rng(params.seed, 2)`. Rozměry v `cqw`.
- Soli `rng(params.seed, n)` 1 až 8 jsou obsazené, volná je 9.
- Globální knihovny na disku: `internal/webtoon` (`Library`, `Installer`
  s `Job` a dotazováním, instalace přes dočasnou složku a `os.Rename`),
  `internal/stylemap`. Veřejné trasy `GET /api/webtoon`,
  `GET /webtoon/{ep}/{file}`, cockpit trasy přes `s.cockpit(...)`.
- `internal/gallery`: vzor HTTP klienta (bez redirectů, disková cache).
- `internal/stylemap`: `fit(img, long)` a `writeJPEG(path, img, q)` nad
  `golang.org/x/image/draw` (už v `go.mod`), neexportované.
- `api/access.go`: `fileKey(p)` je HMAC z cockpit tokenu, aby `<img>` ve
  Studiu načetl soubory jen pro cockpit (`?k=`), viz `serveFile`.
- Show: `sceneKinds` v `api/handlers.go`, `showReady` a `showPrefetch`
  v `api/show.go`, `paramStrings(e.Params, …)`. Studio: `studio/kinds.ts`
  (`KINDS`), `EntryEditor.svelte`, `Studio.svelte` (záložky),
  `WebtoonLibrary.svelte` jako vzor knihovny.
- Cockpity: Svelte `cockpit/Cockpit.svelte` (seznam scén, `stripCols`),
  React `frontend-react/src/lib/scene.ts` (`sceneParams`, `SceneOptions`),
  `components/ScenePanel.tsx`, test `lib/lib.test.ts`.
- Plátno si seznamy pro výpadek ukládá přes `store` z `lib/api.ts`
  (vzor: `Slideshow.svelte`).

## Rozhodnutí, která jsou daná

- **Stažení, ne živé čtení.** Backend tributy stahuje na výslovný pokyn ze
  Studia do `$PC_DATA_DIR/tributes/<id>/`. Plátno čte jen disk. promo-api na
  party být nemusí.
- **Arch skládá plátno v CSS**, server obrázky neslepuje (stejně jako Pásy).
- **Tributy nejsou assety party.** Nejdou do `assets`, `stage_queue`, recapu
  ani losování. Žádná migrace.
- **Token jen v hlavičce** odchozího požadavku. Nikdy v URL, logu, odpovědi
  API ani ve frontendu. Do UI nejde ani `PROMO_API_URL`, ani cesty assetů.
- **Průběh dotazováním** každou sekundu, ne přes WS (jako D23).
- **PromoClown se nemění** a PartyCockpit do něj nic nezapisuje.

## Backend

### Konfigurace

`config.go`: `PromoURL` (`PROMO_API_URL`, výchozí prázdné) a `PromoToken`
(`PROMO_TOKEN`). Prázdná adresa = stahování vypnuté, stažené tributy hrají
dál. `PROMO_API_URL=mock` zapne vestavěný zdroj pro vývoj a ověření.
`.env.example`:

```
# Tributes from PromoClown (Studio → 🎨 Tributy). Empty = import off, tributes
# already on disk still play. Use the agent token, never the admin token.
# Only the machine that prepares the show needs it. Example:
# PROMO_API_URL=http://promo-api.example:8094
PROMO_API_URL=
PROMO_TOKEN=
```

### Balík `backend/internal/tribute`

- `Source` (rozhraní): `List(ctx, limit) ([]Remote, error)` a
  `Open(ctx, r Remote, n int) (io.ReadCloser, error)`.
- `HTTPSource{Base, Token, HTTP}`: klient bez redirectů
  (`http.ErrUseLastResponse`), timeout 30 s, jen `GET`. JSON čte přes
  `io.LimitReader` (4 MB), obrázek nejvýš 25 MB. `List` spojí
  `GET /tributes?status=drafted&limit=N` s `GET /posts?limit=500` podle
  `post_id`. Post, který v seznamu není, dotáhne přes `GET /posts/{id}`
  (nejvýš 20 takových). Vyřadí tributy bez postu, s postem `rejected` a bez
  médií. `401` a `403` vrací jako `ErrAuth`.
- `Remote{ID, Credit, Project, Styles, Labels, PostStatus, CreatedAt, N}`
  a neveřejné `media []string` (`json:"-"`).
- **Názvy stylů** (`Labels`): druhý odstavec textu postu rozdělený `" · "`,
  když jich je stejně jako médií. Jinak pro každé ID funkce `LabelFor`
  dodaná zvenku (katalog PartyCockpitu, `providers.Catalog.Style`), a když
  styl nezná, ID zlidštěné (`-` a `_` na mezery, velké první písmeno).
- `MockSource`: 6 tributů bez sítě (4× na výšku, 1× na šířku, 1× se
  3 snímky), obrázky generované v Go (přechod a iniciála stylu).
- `Library{Dir}`: `List()` (nejnovější napřed, přeskočí `.tmp-*`, `.thumbs`
  a složky bez `tribute.json`), `Has(id)`, `Remove(id)`, `File(id, name)`.
  ID jen `^[0-9]{1,9}$`.
- `Importer`: fronta v jedné gorutině, `Job{id, pct, status (running | done |
  failed), error}`. Pro každý snímek: stáhnout, ověřit typ
  (`http.DetectContentType`: png, jpeg, webp), `image.DecodeConfig` (nejvýš
  8192 px na stranu a 40 MP), dekódovat, uložit `N.jpg` (delší strana
  ≤ 1344, q90) a `Ns.jpg` (≤ 640, q85). Jména souborů jsou pevná, nikdy
  z upstreamu. Zapisuje do `.tmp-<id>-<náhoda>`, nakonec `tribute.json`,
  `os.RemoveAll(cíl)` a `os.Rename`. Selhání nechá předchozí verzi být.
  Zbylé `.tmp-*` se smažou při startu.
- `tribute.json`: `{schema: 1, id, credit, project, styles[], labels[],
  frames[{w, h, sw, sh}], postStatus, createdAt, importedAt}`. Bez cest
  assetů.
- `fit` a `writeJPEG` vytáhni ze `stylemap` do sdíleného balíku (např.
  `internal/imgutil`) a použij v obou. Neduplikuj.

### Trasy (`api/tributes.go`)

Veřejné (čte plátno):

- `GET /api/tributes` → `[{id, credit, labels[], frames[{w,h,sw,sh}], v}]`,
  `v` je `importedAt`. Jen to, co je na disku.
- `GET /tributes/{id}/{file}`: `id` podle regexu, `file` jen
  `^[0-3]s?\.jpg$`. S `?v=` dlouhá cache (`immutable`), jinak krátká.

Cockpit (`s.cockpit(...)`):

- `GET /api/parties/{code}/tributes?limit=60` → `{configured, error?,
  items[{id, credit, project, labels[], n, postStatus, createdAt, installed,
  remote, job?}]}`. Spojí vzdálený seznam s diskem. Když promo-api
  neodpovídá nebo není nastavené, vrátí stažené tributy a `error`
  (`configured: false` bez chyby). Vzdálený seznam drž v paměti 60 s.
- `POST /api/parties/{code}/tributes/import` s `{ids: […]}` (nejvýš 40)
  → `202` a joby. Bez zdroje `409` s českou hláškou.
- `DELETE /api/parties/{code}/tributes/{id}`.
- `GET /api/parties/{code}/tributes/{id}/thumb/{n}?k=<fileKey>`: náhled
  nestaženého tributu (delší strana 320). Ověřuje `k` jako `serveFile`, ne
  bearer. Cache v `tributes/.thumbs/`, nejvýš 3 stahování naráz, při startu
  smaž náhledy starší 14 dní.

Hlášky do UI jsou obecné („promo-api neodpovídá“, „token odmítnut“, „tribut
nenalezen“). Detail jde do `slog`, bez tokenu.

### Show

- `sceneKinds["tributes"] = true`.
- `showReady`: vybraná ID, která na disku nejsou, jsou `missing` (fail).
  Bez výběru detail „všechny na disku (N)“. Když je disk prázdný, fail
  „žádný tribut na disku (Studio → 🎨 Tributy)“.
- `showPrefetch`: se zdrojem stáhne chybějící vybrané tributy a započítá
  je do `done` / `failed`.
- Janitor na `tributes/` nesahá. Ověř testem.

## Plátno

- `frontend/src/lib/tributes.ts` (čisté funkce, sdílené přes `@pc`): typy,
  `tributeOptions(params)` s mezemi a `sheetAspect(t, layout, labels)`
  (výška archu ku šířce z konstant Restyleru a `frames[0]`).
- `frontend/src/stage/ZigZag.svelte`: rolování cik cak vytažené ze
  `Strips.svelte` (RAF, `elapsed(startedAt)`, dvojí vykreslení sloupce,
  poskočení v rytmu). Bere `columns`, `pace`, `speed`, `startedAt`, `clock`
  a snippet položky. `Strips.svelte` ho použije a musí vypadat a hýbat se
  stejně jako dřív (sůl 2 zůstává).
- `frontend/src/stage/scenes/Tributes.svelte`:
  - `params.layout` `'grid'` (výchozí) nebo `'strip'`
  - `params.cols` počet sloupců archů: `grid` 2 až 4 (výchozí 2), `strip`
    2 až 8 (výchozí 4)
  - `params.speed` px za sekundu při 1080p (výchozí 40, meze jako Pásy)
  - `params.labels` název stylu pod každým snímkem (výchozí ano)
  - `params.tributes` ID v tomto pořadí (výchozí: všechny na disku,
    nejnovější napřed, nejvýš 24; výběr nejvýš 40)
  - `params.seed` tempo sloupců přes `rng(params.seed, 9)`, 0,8 až 1,2
- **Arch**: bílý papír (`#fbfaf7`), okraj, popisek a písmo podle konstant
  Restyleru přepočtené na `cqw` sloupce. `grid` má 2 sloupce buněk, `strip`
  1. Buňky mají poměr stran prvního snímku a `object-fit: cover`. V `grid`
  je lichý poslední snímek vycentrovaný v šířce jedné buňky, jediný snímek
  jde přes celou šířku. Dole na archu patka s kreditem „Tribute to
  @autor“ (jedna konstanta, text jako v postu), o polovinu vyšší než
  popisek stylu. Mezera mezi archy je `margin-bottom`.
- **Sloupce**: archy se rozdají dokola v pořadí výběru. Krátký sloupec se
  opakuje, dokud jedna kopie není aspoň 1,2× vyšší než okno (spočti ze
  `sheetAspect` a šířky sloupce).
- **Obrázky**: `Ns.jpg`, velké `N.jpg` jen když je buňka širší než malý
  obrázek (4K plátno, málo sloupců). URL s `?v=`. `decoding="async"`.
- **Výpadek**: seznam z `GET /api/tributes` ulož přes `store`
  (`pc.tributes`), při chybě čti uložený.
- Prázdný stav: jen pozadí. V náhledu (`?preview`) nápověda „žádné tributy
  na disku“.
- `Stage.svelte`: jedna větev `{:else if scene.kind === 'tributes'}`. Na
  řádek s QR nesahej.

## Studio a Cockpity

- **Záložka „🎨 Tributy“** (`studio/TributeLibrary.svelte`, vzor
  `WebtoonLibrary.svelte`). Řádek stavu: připojeno, „není nastavené
  (`PROMO_API_URL`)“, nebo chyba. Karty: 4 náhledy 2×2, kredit, názvy
  stylů, projekt, datum, stav postu (draft = „čeká na schválení“, approved,
  scheduled, published). Zaškrtnutí a „Stáhnout vybrané (N)“, „Stáhnout
  všechny nové“. U stažených „✓ na disku“, „Stáhnout znovu“, „Smazat“ na
  dvě kliknutí. Filtr podle projektu (když jich je víc) a „jen nestažené“.
  Náhledy stažených z `/tributes/<id>/<n>s.jpg`, ostatních z trasy `thumb`
  s `fileKey`, `loading="lazy"`. Dotazování po sekundě, jen když běží job.
- **Scéna ve show**: `kinds.ts` přidá `{ kind: 'tributes', label:
  '🎨 Tributy', seconds: 90, params: { layout: 'grid', cols: 2, speed: 40,
  labels: true, tributes: [] } }`, typ `SceneKind`. `EntryEditor.svelte`:
  rozložení (Mřížka 2×2 / Proužek 1×4), sloupce v mezích rozložení,
  rychlost, popisky, výběr stažených tributů náhledem (`toggle`, strop 40).
  Prázdný výběr = všechny.
- **Oba Cockpity**: tlačítko scény „🎨 Tributy“, volby rozložení a sloupců
  uložené jako `pc.tributes.layout` a `pc.tributes.cols`. Výběr tributů je
  jen ve Studiu. React: `SceneOptions.tributeLayout`, `tributeCols`
  a `sceneParams`.

## Testy

Go, `internal/tribute` (fake promo-api přes `httptest`):

- jen `GET`, token v hlavičce a ne v URL, redirect se nesleduje, `401`
  → `ErrAuth`, příliš velká odpověď → chyba
- spojení s posty, vyřazení `rejected`, dotažení přes `/posts/{id}`
- názvy stylů: z textu, z katalogu, ze zlidštěného ID
- import: soubory a rozměry, `tribute.json`, opakovaný import nahradí
  celek a nenechá `.tmp-*`, odmítne ne-obrázek, příliš velký soubor, příliš
  velké rozměry a segment `..`; selhání nechá starou verzi
- knihovna: pořadí, přeskočení smetí, validace ID

Go, `internal/api`: trasy cockpitu bez bearer `401`, import `202` a job až
do `done`, mazání, `thumb` jen s `k`, veřejný seznam jen z disku, allowlist
souborů (`tribute.json`, `4.jpg`, `..` → 404), vypnutá funkce, promo-api
dole (seznam vrátí stažené a `error`), `sceneKinds`, `showReady`,
`showPrefetch`, janitor.

React `lib.test.ts`: `sceneParams('tributes', …)`. Když už testy berou moduly
z `@pc`, přidej i `tributeOptions` a `sheetAspect`.

## Commity (každý s `make check` zeleným)

1. `Tributes 1/3: PromoClown source, on-disk library, import API`
   (balík, konfigurace, trasy, testy, tenhle soubor)
2. `Tributes 2/3: stage scene with grid and strip sheets, shared zig-zag`
3. `Tributes 3/3: Studio tab, show entry, both Cockpits, D24, docs`

## Ověření v Chrome (mock providery, `127.0.0.1:18098`, `PROMO_API_URL=mock`)

1. Studio → 🎨 Tributy: 6 tributů s náhledy a stavy. Stáhni 3, průběh doběhne
   na „✓ na disku“. Smazání na dvě kliknutí.
2. Show: přidej scénu Tributy. Náhled hraje mřížku ve 2 sloupcích. Přepni na
   proužek ve 4 sloupcích, vypni popisky, 🎲 změní tempo. Stejný seed
   a reload plátna skončí na stejném místě.
3. Plátno: sloupce jdou proti sobě, smyčka je bez švu (zkus `speed: 300`
   a sleduj dvě periody). Kredit je na každém archu. Okno 1280×720
   a 3840×2160: texty čitelné, nic nepřetéká.
4. Pásy vypadají a hýbou se stejně jako před vytažením `ZigZag`.
5. Zastav server: plátno jede dál. Spusť ho s prázdným `PROMO_API_URL`:
   Studio hlásí „není nastavené“, stažené tributy jsou vidět a hrají.
6. ✅ Offline: scéna s nestaženým ID je ❌, po „Stáhnout do cache“ ✅.
7. Oba Cockpity: tlačítko scény, volby se pamatují.
8. Konzole bez chyb.

## Dokumentace

- `DECISIONS.md` D24 (šablona níže), `STATUS.md`, `README.md` (funkce
  a proměnné).
- `docs/studio.md`: krok přípravy „🎨 Tributy“, složka `tributes` mezi
  těmi, které se kopírují na notebook, řádek v tabulce „Co poběží bez
  sítě“.
- `CLAUDE.md`: jeden řádek do mapy, soubor nejvýš 80 řádků.

## D24 šablona do `DECISIONS.md`

```
## D24: Tributy z PromoClown na plátně (2026-11-xx)
Zadání: `Prompts/05-TRIBUTY-NA-PLATNO.md`.
- **Zdroj** je promo-api (`PROMO_API_URL`, `PROMO_TOKEN`). PartyCockpit
  posílá jen GET s agent tokenem v hlavičce. Bere tributy `drafted`,
  jejichž post není `rejected`.
- **Stažení je schválení.** Operátor tribut ve Studiu vybere a stáhne do
  `data/tributes/<id>/`. Plátno čte jen disk, proto scéna běží offline
  a bez promo-api. Původní obrázek autora se nestahuje.
- **Scéna `tributes`**: jeden tribut je jeden arch podle Restyleru, mřížka
  2×2 nebo proužek 1×4, s názvem stylu pod snímkem. Kredit `@autor` je na
  archu vždy. Archy rolují cik cak jako Pásy, arch skládá plátno.
- Tributy jsou globální knihovna jako webtoon packy: nejsou assety party,
  nejdou do recapu a do uložené show se nekopírují.
- Názvy stylů jsou z textu postu, jinak z katalogu PartyCockpitu, jinak
  z ID. PromoClown katalog přes API nevydává.
```

## Co nedělat

- Neměň PromoClown a nic do něj nezapisuj (žádný `POST`, žádné schvalování).
- Nekopíruj adresu promo-api ze zdrojáků PromoClown ani jinou LAN IP.
- Nedávej možnost kredit skrýt. Neukazuj původní obrázek.
- Neanimuj tributy (`alive`, `breeze`, Wan). Žádné GPU.
- Nemíchej tributy s fotkami hostů v jedné scéně.
- Žádné vodorovné pásy ani režim jednoho archu přes celé plátno.
- Neskládej archy na serveru do jednoho obrázku.
- Nesahej na `preflight.go` ani na řádek s QR ve `Stage.svelte`.
- Needituj vydané migrace. Nová není potřeba.
- Nepushuj a nezakládej PR bez Olinova pokynu.
