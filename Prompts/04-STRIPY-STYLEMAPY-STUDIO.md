# Zadání: Webtoon stripy, Pásy, StyleMapy ze session a Scene Studio (offline show)

Jsi Claude session (Opus) v repu PartyCockpit (veřejné repo
https://github.com/lioilsources/PartyCockpit). Zadání má čtyři části, každá je
samostatný PR v tomto pořadí (čtvrtá staví na prvních třech):

1. **Webtoon**: načítání hotových příběhů z katalogu Toonfork a auto-scroll
   několika stripů najednou (sloupce vedle sebe).
2. **Pásy**: scéna s restylovanými fotkami v pásech jako v Restyleru, sloupce
   rolují cik cak (jeden nahoru, druhý dolů), tváře žijí (úsměv, mrknutí už
   umí `alive`), nově „vlasy ve větru“.
3. **StyleMapy ze session**: z FineTuneGallery session vzniknou StyleMap packy
   rozsekané podle promptu (stejná předloha, jiný prompt = jiný pack) a plátno
   jich přehrává několik naráz.
4. **Scene Studio**: show složená ze scén a položek, připravená dopředu.
   Všechno se vygeneruje předem (GPU na SPARKu přes broker), artefakty leží na
   notebooku, na party běží offline a hýbe se přesně tak, jak bylo uložené.

Čti `CLAUDE.md`, `DECISIONS.md` (D5, D11, D13, D15, D18, D19, D21, D22),
`STATUS.md` a `Prompts/03-COCKPIT-REACT.md` sekci „Než začneš“ (stejná pravidla:
`make check` jako brána, Go testy s `-timeout 120s`, testovací server na
`127.0.0.1:18098` s mock providery, `git add` jmenovitě, commit po kroku,
**nepushovat bez pokynu**, do repa žádné LAN IP, tokeny ani domácí cesty).
Testy bez sítě a bez `time.Now` (fake servery přes `httptest`, čas injektovaný).

## Upřesnění od Olina (2026-11-11)

- „stream několika mapstyle“ = přehrávání na plátně v dlaždicích (1, 2, 2×2,
  3×3), ne video stream. **Potvrzeno.**
- StyleMap packy ze session staví PartyCockpit sám v Go. Ol1nLLM
  `tools/stylemap` řadí obrázky neuronkou (CLIP, LPIPS) v Pythonu s torch, což
  na notebooku ani boxu bez instalace nepoběží. Řazení v Go porovnává
  zmenšené obrázky: funguje všude a offline, přechody jsou o něco méně
  plynulé. Packy z Ol1nLLM přes FineTuneGallery fungují dál.
- „recordnul“ = **výstupy ze SPARKu se uloží jednou** (restyly, klipy jako
  soubory) ve formátu, který plátno přehraje bez GPU; show (pořadí scén,
  položky, délky, seed náhody) se přehraje stejně. Ne videozáznam plátna.
- Aby příprava přežila konec party (TTL 24 h, úklid po 7 dnech), show jde
  **uložit do knihovny** (`$PC_DATA_DIR/shows/<slug>/show.json` + `media/`)
  a **načíst do jiné party** (soubory se zkopírují, assety vzniknou znovu jako
  schválené). Knihovnu janitor nemaže.
- Studio jen ve Svelte na `/studio/<kód>`; React Cockpit dostane jen ovládání
  show (▶ ⏸ ⏭) a nové scény. **Potvrzeno.**
- Generace na SPARKu = uživatel si vezme okno přes broker
  (`sparkctl lease`) a nastaví `COMFY_URL`. PartyCockpit s brokerem nemluví.
  **Potvrzeno.**
- Zjednodušení při implementaci: položky scény jsou přímo v jejích
  parametrech (`episodes`, `packs`, `assets`, `images`), ne zvláštní seznam
  `items`; průběh stahování packů a stavby StyleMap Studio zjišťuje
  dotazováním (každou sekundu), ne přes WS; knihovna Webtoon a StyleMap ze
  session jsou jen ve Studiu, Cockpit na něj odkazuje.

## Co průzkum sousedních rep zjistil (neprozkoumávej znovu)

**Webtoon (Toonfork).** Hotové příběhy jsou content packy. Veřejný katalog
`https://lioilsources.github.io/webtoon-content/catalog.json` (schema 2):
`base_urls{pages, releases, r2}`, `packs[]{id, type, n, title, title_i18n,
free, version, stories[], bytes, audience, languages, cover, parts[]{part,
release, asset, bytes, sha256, stories[]}}`, `stories[]{id, pack_id, type,
world_id, title, title_i18n, cover, episodes (počet), audience}`. Zip:
`part.url`, jinak `(base_urls.r2 ?? base_urls.releases) + release + "/" + asset`
(GitHub Releases, 302 na objects.githubusercontent.com, 85–100 MB na part, bez
auth, Range funguje). Panely jednotlivě přes HTTP **nejdou**, jen zip. V zipu:
`pack.json` (schema 1: `worlds[]`, `stories[]{id, world_id, title, title_i18n,
episodes[], cover, languages}`, `episodes[]{id, campaign_id, index, title,
title_i18n, source_language, languages, status, panels_ready, panels_total,
panels[]}`), `media/<ep_id>/pNN.webp` (832×1216, 100–200 kB), volitelně
`media/<ep_id>/pNN_loop.webp` (animovaný, ~1 MB), `covers/<story_id>.webp`.
Panel: `index, status, image_url, video_url, width, height, shot, gap_after
(small|medium|…), bubbles[]{speaker_id, text, kind, i18n{cs,en,…}, anchor{x,y}},
sfx[]{text, style, position{x,y}}, characters[]`. **Text bublin není v obrázku**,
kreslí ho klient: `Webtoon/app/lib/reader/bubble_layout.dart`,
`bubble_painter.dart`, model `app/lib/models/episode.dart`. Spec
`Webtoon/PACK_FORMAT.md`. `audience` kids|teen|adult (harlequin = adult). Žádný
auto-scroll v appce není. Obsah je vlastní tvorba uživatele.

**Restyler.** Flutter appka; mřížka (2 sloupce) a pás (1 sloupec) vznikají
v telefonu (`Restyler/lib/services/strip.dart`, `StripLayout.grid(2)/strip(1)`,
`kStripCellWidth=512`, `kStripMargin=24`, `kStripLabelHeight=44`, center-crop na
poměr prvního snímku). Server (`MangaPrompts-restyler`, větev `feat/restyler`)
nemá galerii ani historii; `live_portrait`/`wan_motion` nikdy neběžely (D13).
Manifest pohybů `assets/drivers/manifest.json` definuje `hair-breeze` (engine
wan, 5 s, prompt „a gentle breeze moves the person's hair softly, the person
stays still and looks at the camera“) a `alive` = blink + look-around + smile,
tedy úsměv i mrknutí už náš driver `alive.mp4` dělá.

**Ol1nLLM `tools/stylemap` + FineTuneGallery.** Pipeline Python (CLIP ViT-L,
UMAP, LPIPS, 2-opt tour) na M2, publikuje rsyncem na NAS; bez upload API.
FineTuneGallery (Go, za CF Access, hlavičky `CF-Access-Client-Id/Secret`):
`GET /api/sessions` → `[{id, title, modelId, lastIngestedAt, imageCount}]`;
`GET /api/images?session=<id>&limit=200&cursor=…` → `{items[], nextCursor}`,
item: `id, sha256, idx, nodeId, sessionId, prompt, promptSent, modelId,
loraName, styleId, mediumId, seed, score, isImg2img, isRepose, origin
(generated|upload), taxo?{band}`. Blob `/img/{sha}`, `/thumb/{sha}` (384 px).
„Restyle session“ = jedna session s jedním `origin:"upload"` a mnoha
generovanými potomky (každý vlastní `prompt` + `styleId`, často `isRepose`).
Filtr podle promptu API neumí → seskupit na naší straně. `source_image_id`
API nevrací (jen DB); předloha = ten jeden `upload` v session.

## Rozhodnutí, která jsou daná

| Věc | Rozhodnutí | Proč |
|---|---|---|
| Webtoon zdroj | Katalog Toonfork (`WEBTOON_CATALOG_URL`, výchozí veřejná adresa výše), import zipu po partech do `$PC_DATA_DIR/webtoon/packs/<pack>/`, ověřit `sha256` z katalogu | Jediná cesta k panelům; vlastní obsah uživatele |
| Webtoon filtr | `audience == adult` se nenabízí, pokud není `WEBTOON_ALLOW_ADULT=1`; `free` jen jako štítek | Stejně jako Kids profil appky; placené packy jsou uživatelovy |
| Webtoon bubliny | Kreslí plátno (port rozmístění z `bubble_layout.dart`, text v `params.lang`, fallback `source_language`); bez bublin příběh nedává smysl | Obrázky text nenesou |
| Stávající složky | `data/webtoon/<id>/` s panely dál fungují (bez bublin); `packs` je rezervované jméno | Zpětná kompatibilita |
| Pásy = nová scéna `strips` | Sloupce z show fronty (`restyle`/`upload` s `file`, přehrává `a.live ?? a.file`), směr střídavě, nekonečná smyčka, popisek jako Restyler | Zadání |
| Vlasy ve větru | Nový pohyb `breeze` (Wan I2V, `engine: "breeze"`, prompt z Restyler manifestu, 5 s). Generativní → **vždy čeká na operátora** (jako tanec, D13). Po schválení **nejde do fronty jako položka**, hraje místo fotky (`Asset.live`, přednost před `alive`) | Invariant „tanec čeká vždy“ + „živý portrét hraje místo stillu“ |
| Smile/wink | Žádné nové LivePortrait drivery; `alive` je má. Pokud by Olin chtěl samostatné (`smile.mp4`, `wink.mp4`), vznik `alive.mp4` není v repu zdokumentovaný → zeptej se, nevyrábej | Nevíme, jak driver vznikl |
| StyleMapy lokálně | Balík `backend/internal/stylemap`: pack ze session seskupené podle `prompt`, řazení v Go (odstup 24×24 šedotón + nejbližší soused + 2-opt), výstup ve **stejném schématu** jako Ol1nLLM (`map.json` s `images[{i,label,style,tour,hilbert}]`, `atlas`, `thumbs`, `aspect`, navíc `source:"party"`), uložené v `$PC_DATA_DIR/stylemaps/<id>/` | Plátno i Cockpity čtou packy beze změny; žádný torch |
| Servírování packů | `/stylemaps/index.json` = sloučení upstream indexu (cache) a lokálních; `/stylemaps/<id>/…` lokální napřed, jinak proxy | Jeden namespace |
| SFW u lokálních packů | Když session nese `taxo`, bere se jen `band == "safe"`; bez taxonomie vše (D15: packy vybírá operátor z vlastních sad) | Využít informaci, když je |
| Sweep dlaždice | `stylesweep` bere `params.packs: string[]` (1 → celá plocha jako dnes, 2 → vedle sebe, 3–4 → 2×2, 5–9 → 3×3); `params.pack` dál funguje | Zadání „několik naráz“ |
| Show (Studio) | Tabulka `shows` (nová migrace na konec `migrations`): entry `{id, kind, params, items[], seconds, seed, transition?}` na party; runner na serveru přepíná scény podle `seconds`; stav `show` po WS | Server je zdroj pravdy, plátno jen kreslí |
| Determinismus | Každá scéna s `params.seed` používá seeded PRNG (mulberry32) místo `Math.random` (shuffle v `showqueue.ts`, `Slideshow`, start sweepu, jitter rychlostí pásů); bez seedu chování jako dnes | „Hýbe se, jak jsem uložil“ |
| Položky show | Explicitní seznamy: `webtoon.episodes[]`, `stylesweep.packs[]`, `strips.assets[]` (prázdné = celá fronta), `slideshow.images[]` (sha z galerie, předem stažené do cache); `gallery`/`stickers`/`dance`/`interlude`/`nowplaying`/`blank` bez položek | Offline = jen lokální soubory |
| Studio média | Operátor nahrává fotky ve Studiu pod skrytým hostem „Studio“ (nelosuje se, nehlásí se v počtech). Restyle operátorovy fotky se po úspěchu **schválí automaticky** (meta `studio: true`); originál po selhání nikdy. Generativní klipy (`breeze`, tanec) čekají na ✓ ve Studiu | Operátor je ve Studiu a dívá se; invarianty platí pro hosty |
| Offline kontrola | `GET …/show/ready` projde každou položku (soubor fotky/klipu, pack lokálně, epizoda lokálně, obrázek galerie v cache) a preflight ukáže „Show: N/M položek offline“ | „Při spuštění už můžu být offline“ |
| Preview | Plátno s `?preview=<entryId>` vykreslí daný entry ze show místo živé scény (bez QR), Studio ho má v `<iframe>` | Náhled bez vlivu na TV |
| GPU | `COMFY_URL` (SPARK přes broker nebo L40S), workery jako dnes; PartyCockpit broker nevolá | Pravidla SPARKu |
| Parita React | Nové scény, parametry (`episodes`, `packs`, `cols`, `motion`), tlačítko „💨 Vlasy“, ovládání show (▶ ⏸ ⏭ + název entry) a odkaz „Studio ↗“; Studio samo jen Svelte | D21 parita pro party, ne pro přípravu |

## Část 1: Webtoon

### Backend
- `config.go`: `WebtoonCatalogURL` (`WEBTOON_CATALOG_URL`), `WebtoonAllowAdult`
  (`WEBTOON_ALLOW_ADULT`); `.env.example` s komentářem.
- Nový `backend/internal/webtoon` (bez závislosti na `api`): `Catalog` (fetch
  + cache 10 min + poslední dobrá kopie na disku `webtoon/catalog.json`),
  `Importer` (stáhne part do temp souboru vlastním `http.Client`, který
  **sleduje redirecty** — klient galerie je nesleduje záměrně; ověří sha256;
  rozbalí `archive/zip` jen `pack.json`, `gamebook.json`, `media/**.webp`,
  `covers/*.webp`; odmítne `..`, absolutní cesty a součet nad 400 MB; zapíše do
  `packs/<pack>.tmp` a přejmenuje), `Library` (index epizod ze složek i packů:
  id epizody je `ep_…` z packu nebo jméno složky; jen epizody s `status ==
  ready` a všemi panely), progress callback.
- Routy (čtení veřejné jako dnes, zápis `s.cockpit`):
  - `GET /api/webtoon` → `[{id, title, story, storyId, index, cover, n, langs[],
    source: "folder"|"pack"}]` (bez `panels`; `Episode` typ v `types.ts`
    změnit a opravit obě Cockpit implementace, které z něj berou jen `id` a
    `title`).
  - `GET /api/webtoon/{id}` → `{id, title, story, langs[], panels[{image, loop?,
    w, h, gap, bubbles[{text{lang:…}, kind, anchor{x,y}}], sfx[{text, style,
    position{x,y}}]}]}`; souřadnice normalizuj na 0–1 (model appky toleruje
    pixely i podíly, viz `episode.dart` ~ř. 147).
  - `GET /webtoon/{ep}/{file}` servíruje ze složky nebo z
    `packs/<pack>/media/<ep>/`; `GET /webtoon/{ep}/cover` obálku příběhu.
  - `GET /api/webtoon/catalog` (cockpit) → příběhy s `installed`, `bytes`,
    `audience`, `free`, `part`; `POST /api/webtoon/packs/{pack}/{part}` → 202,
    jeden import naráz, průběh WS `webtoon.import {pack, part, pct, status,
    error}` pro Cockpit; `DELETE /api/webtoon/packs/{pack}`.
- Preflight: řádek „Webtoon: N epizod lokálně“ (info), katalog nedostupný =
  warn jen když není nic lokálně.
- Testy: fake katalog + fake zip přes `httptest` (redirect 302 → zip), sha
  mismatch → chyba a nic na disku, zip-slip odmítnut, výpis epizod ze složky
  i packu, detail s bublinami.

### Plátno
- `Webtoon.svelte`: `params.episodes: string[]` (1–4; `params.episode` string
  dál funguje jako jeden sloupec, `party` může být jeden ze sloupců),
  `params.speed` (px/s při 1080p, škáluj s `vh`), `params.lang` (výchozí `cs`),
  `params.seed`. Každý sloupec = `Strip.svelte`: vlastní fetch detailu, vlastní
  rAF scroll s rychlostí ±15 % (seeded), start s posunem, na konci 3 s pauza
  a plynulý návrat nahoru. Mezery podle `gap_after`. Animované `_loop.webp`
  jako `<img>`; když Pi nestíhá, mimo viewport zobraz still (volitelné,
  změř).
- `Bubbles.svelte`: bublina u `anchor` (strana podle polohy anchoru v panelu,
  ocásek směrem k anchoru, zaoblený rámeček, text v `lang` s fallbackem),
  `sfx` jako tučný nakloněný text; rozměry v % šířky panelu. Přečti
  `bubble_layout.dart` kvůli pravidlům umístění; nemusí být pixel-přesné,
  musí být čitelné z 5 m.
- Šířka sloupce `min(100vw/n − mezery, 46vw)`; 4 sloupce 2:3 na 1080p ≈ 24vw.

### Cockpit (Svelte i React)
- Výběr epizod jako multi-select (checkboxy, max 4) místo jednoho `<select>`,
  jazyk, rychlost. `sceneParams` v Reactu zrcadlí Svelte (test).
- Sekce „📚 Webtoon knihovna“ (v `<details>`): seznam z katalogu s obálkou,
  štítky `audience`/`free`/MB, tlačítko „Stáhnout“ s průběhem, „Smazat“.
  Jen Svelte Cockpit + Studio; React má odkaz na Studio (knihovna je příprava).

## Část 2: Pásy

### Scéna `strips`
- `scenes/Strips.svelte`, label „🎞 Pásy“. `params.cols` (2–5, výchozí 3),
  `params.speed`, `params.labels` (bool), `params.assets: string[]`
  (prázdné = všechny fotky fronty), `params.seed`.
- Obsah se rozdělí do sloupců round-robin; sloupec vykreslí položky dvakrát za
  sebou a posouvá `translateY` v rAF modulo výšky → nekonečná smyčka bez skoku.
  Sudé sloupce nahoru, liché dolů; rychlost ±20 % (seeded). Buňka: bílý rámek
  s okrajem a popiskem (přezdívka · styl) v poměru Restyleru, škálované ve `vw`.
- Živé klipy: `Clip` jen pro dlaždice ve viewportu, ostatní still (Pi zvládne
  ~4 videa; čti `party.box` → limit 4, jinak 9). Na dobu lehký `scale(1.015)`
  jako sweep.
- Prázdná fronta: výzva „Pošli selfie“ jako v Galerii.

### Pohyb `breeze`
- `providers.Motions` + `{ID:"breeze", Label:"💨 Vlasy ve větru", Engine:"breeze",
  Seconds:5, Prompt: <z manifestu Restyleru, doplněný „same face and outfit,
  static camera“>}`. `Comfy.Animate`: `Engine != "live"` → `wan_i2v.json`
  (už tak funguje); `RestylerAnimator` → `ErrUnsupportedMotion`; `MockAnimator`
  → GIF s vlněním horní třetiny snímku (sinusový posun řádků). `types.ts`
  `Motion.engine: 'live' | 'dance' | 'breeze'`.
- Asset `kind: "breeze"`, `src_id` = fotka. `submitAnimation` zůstává; výsledek
  vždy `pending` (jen `live` dědí). `SetAssetStatus`: `breeze` se **nevkládá do
  `stage_queue`** (jako `live`). `StageQueue`: `Live` = nejnovější schválený
  `breeze`, jinak `live`. `AnimationOf(src, "breeze")` pro „už objednáno“.
  Host `dance` endpoint `breeze` odmítá (jen operátor).
- `POST /api/parties/{code}/animate` přijme `{ids?, motion?}` (výchozí
  `alive`); `breeze` → pending karty v „Ke schválení“ s videem a štítkem
  „💨 vlasy“. Cockpit (oba): tlačítko „💨 Rozfoukat vlasy“ vedle „✨ Oživit
  galerii“ s poznámkou „~1 min/fotku na GPU“. Recap a `/files` (D19): klip
  `breeze` je veřejný až po schválení, stejně jako tanec.
- Testy: tok na mocku (fotka → breeze pending → schválit → fronta má fotku
  s `live` = breeze, bez nové položky), guest `dance` s `breeze` → 400,
  `StageQueue` přednost breeze před alive.

## Část 3: StyleMapy ze session

### Backend
- `gallery.Client`: `Sessions(ctx)` (`/api/sessions`), `SessionImages(ctx, id)`
  (stránkuje přes `cursor`, všechny položky), `Thumb(ctx, sha) ([]byte, ext)`
  přes stávající diskovou cache.
- Nový `backend/internal/stylemap`: `Group(items) []Group` (klíč = `prompt`
  po trim + sjednocení bílých znaků + lowercase; bez `origin == upload`; label
  = původní prompt), `Order(thumbs []image.Image) []int` (24×24 šedotón,
  L2 odstupy, start v medoidu, nejbližší soused, 2-opt; deterministické),
  `Build(dir, Group, thumbs)` → `map.json` (`version 1`, `id`, `title`
  „<session title> · <prompt do 40 znaků>“, `source:"party"`, `n`, `aspect`,
  `atlas:"atlas.jpg"` (32 px buňky, stdlib jpeg), `thumbs:"t/{i}.<ext>"`
  (kopie bytů z cache, přípona podle `http.DetectContentType`),
  `images[{i, id, label: styleId|"základ", style, tour, hilbert: tour}]`),
  `Index(dir) []PackInfo`. Minimum 8 obrázků na skupinu.
- Routy (cockpit): `GET /api/finetune/sessions`, `GET
  /api/finetune/sessions/{id}/groups` → `[{key, prompt, n, thumb, safe}]`,
  `POST /api/stylemaps/build {session, groups: [key]}` → 202, průběh WS
  `stylemap.build {id, pct, status}`, `DELETE /api/stylemaps/{id}` (jen
  lokální). Id packu `[A-Za-z0-9_-]{1,64}`: slug session + pořadí skupiny.
- `ServeStyleMap`: lokální adresář napřed (vlastní regex povolí `atlas.jpg`,
  `t/N.(webp|jpg|png)`), `index.json` sloučený (upstream s cache + lokální,
  lokální `source:"party"`; upstream nedostupný → jen lokální).
- Testy: řazení na syntetických obrázcích (gradient barev → tour je monotónní),
  grupování promptů, build zapíše schéma čitelné `frontend/src/lib/stylemap.ts`
  (zkontroluj ručně proti `MapJson` typu), sloučený index s fake upstreamem
  i bez něj.

### Plátno a Cockpit
- `StyleSweep.svelte` → obal `SweepGrid` + dlaždice `SweepTile` (dnešní logika).
  `params.packs[]`, doba skáče všem; `params.seed` pro start index. Přechod
  (`transition`) beze změny.
- Oba Cockpity: packy jako checkboxy (max 9) s počtem; `pc.stylemap` v
  localStorage drží pole. Svelte Cockpit + Studio: „🧪 StyleMap ze session“:
  session → skupiny s náhledem a počtem → „Vytvořit packy“ s průběhem.

## Část 4: Scene Studio (offline show)

### Model a server
- Migrace (nová na konec): `shows(party_code PK, entries TEXT json, loop INTEGER,
  updated_at)`; entry `{id, kind, params, items[{type: episode|pack|asset|image,
  ref}], seconds, seed, transition?}`. `store.Show/SaveShow`.
- Runner (`api/show.go`): per party goroutina; stav `{playing, index, entryId,
  startedAt, endsAt, loop}` v paměti, broadcast `show` na cockpit i stage.
  Start entry = složit `params` z `items` (+ `seed`, `transition`) →
  `SetScene` → `broadcastParty`; po `seconds` další; konec → loop nebo pauza.
  PANIC pauzne show. Restart serveru = show pauznutá na posledním indexu.
- Routy (cockpit): `GET/PUT …/show` (celé entries; PUT validuje `kind` přes
  `sceneKinds`, `seconds` 5–3600, položky existují), `POST …/show/play
  {index?}`, `POST …/show/pause`, `POST …/show/next`, `POST …/show/prev`,
  `GET …/show/ready` → `[{entryId, item, ok, detail}]` + souhrn, `POST
  …/show/prefetch` (stáhne do cache obrázky galerie a chybějící náhledy).
  Veřejné (kód party, jako `/queue`): `GET …/show/entries/{id}` pro preview
  plátna (jen `kind`+`params`, žádné tokeny ani id hostů).
- Studio média: `POST …/studio/photos` (multipart, `style`, `medium`) → host
  „Studio“ (`guests.hidden` přes meta nebo pevný nickname `__studio__`;
  nelosuje se, nepočítá se do „hostů“), `submitRestyle` jako dnes; v
  `finishAsset` při `meta.studio` → `approved` rovnou (po klasifikaci, když
  běží); fallback na originál nikdy. `POST …/studio/animate {ids, motion}` =
  `animateGallery`. Preflight: „Show: N/M položek offline“, „Studio: K položek
  se ještě generuje“.
- GPU: workery a `COMFY_URL` jako dnes; README sekce „Příprava show“ popíše
  postup: `sparkctl lease partycockpit comfy <min>` (uživatel), `COMFY_URL`
  z okna, `RESTYLE_PROVIDER=comfy`, `ANIMATE_PROVIDER=comfy`, Studio →
  „Vygenerovat vše“, počkat na zelenou, `sparkctl done`. Žádný kód v
  PartyCockpitu broker nevolá.

### Studio UI (`/studio/<kód>`, Svelte)
- Přihlášení a token jako Cockpit (`pc.cockpit.<code>`, `#token=`, ticket pro
  socket); sdílej kód z `Cockpit.svelte` vytažením do `lib/cockpitAuth.ts`
  (nepřepisuj Cockpit nad nutnost).
- Levý sloupec: seznam entries (karta: pořadí, ikona scény, název, délka,
  stav položek ✅/⏳/❌), ↑ ↓ ✕, „+ Přidat scénu“ na konec (výběr druhu),
  přepínač loop, ▶ od vybrané, ⏸, ⏭. Pravý sloupec: editor vybraného entry
  (parametry podle druhu, položky: picker epizod z knihovny, packů, fotek ze
  Studia/fronty, obrázků galerie s filtrem stylu; „+ přidat položku“, pořadí,
  odstranit), `seconds`, `seed` („🎲 nový seed“), přechod. Dole náhled:
  `<iframe src="/stage/<kód>?preview=<entryId>">` s klíčem plátna z Cockpit
  stavu (`config.stageKey` jde přes `?key=` jen do iframu).
- Panel „Příprava“: knihovna Webtoon (import), StyleMap ze session, nahrání
  fotek (styl, médium), „💨 Vlasy“ / „✨ Oživit“ pro vybrané, „Vygenerovat vše“
  (zařadí chybějící), „Zkontrolovat offline“ (`/show/ready`) se seznamem
  chybějících a tlačítkem „Stáhnout do cache“.
- Ukládání: každá změna `PUT …/show` s debounce 500 ms; konflikt dvou tabletů
  neřeš (poslední vyhrává), ale zobraz `updated_at`.
- Plátno: `?preview=<entryId>` → `Stage.svelte` načte entry a drží `scene`
  z něj (bez QR, bez `pairing`, bez cache zápisu); zpráva `show` → malý štítek
  „show 3/12 · 0:42“ v rohu jen když `?hud=1`.
- React Cockpit: řádek „Show: ▶ ⏸ ⏭ · <název entry> · Studio ↗“ v hlavičce
  scén; reducer zpracuje `show`.

## Commity (každý s `make check` zeleným)

1. **„Webtoon 1/2: Toonfork catalog, pack import, episode detail“** — balík
   `webtoon`, konfigurace, routy, preflight, testy.
2. **„Webtoon 2/2: multi-strip scene with bubbles, library in Cockpit“** —
   `Strip`, `Bubbles`, parametry v obou Cockpitech, knihovna ve Svelte.
3. **„Strips 1/2: zig-zag strips scene“** — `Strips.svelte`, scéna v obou
   Cockpitech, `sceneKinds`.
4. **„Strips 2/2: breeze motion (Wan), pending clips play in place“** —
   motion, store sémantika, mock, tlačítka, testy.
5. **„StyleMap 1/2: packs from a FineTuneGallery session, local serving“** —
   balík `stylemap`, gallery klient, routy, sloučený index, testy.
6. **„StyleMap 2/2: sweep tiles, pack multi-select, session builder UI“**.
7. **„Studio 1/3: shows table, runner, ready check, preview“** — model, runner,
   routy, `?preview`, testy (runner s injektovaným časem).
8. **„Studio 2/3: /studio page: entries, items, generation panel“**.
9. **„Studio 3/3: React show controls, docs (D23, STATUS, README, map)“** —
   `CLAUDE.md` mapa max 3 řádky navíc, **strop 80 řádků** (dnes 61).

Po částech 1–3 se zastav a napiš uživateli krátké hlášení (česky); část 4
začni bez čekání, pokud nepřišla námitka.

## Ověření v Chrome (mock providery, `127.0.0.1:18098`)

1. Webtoon: v knihovně stáhnout nejmenší volný pack (reálný katalog, jediný
   test se sítí; `bytes` v katalogu), zvolit 3 epizody → plátno roluje tři
   sloupce různou rychlostí, bubliny česky, `_loop.webp` se hýbe. Nastavit
   `WEBTOON_CATALOG_URL` na neplatnou adresu → knihovna hlásí chybu, scéna
   jede z lokálních packů.
2. Pásy: 8 fotek ze dvou simulovaných hostů, 3 sloupce cik cak, popisky; „💨
   Rozfoukat vlasy“ → mock klipy v „Ke schválení“ → ✓ → dlaždice hraje klip,
   fronta nemá novou položku. Recap klip nezobrazuje před schválením.
3. StyleMap: fake FineTuneGallery (`httptest` nestačí pro Chrome → malý Go
   nebo Node fake server se 2 prompty × 12 obrázků s barevnými gradienty)
   → „Vytvořit packy“ → 2 packy v seznamu, sweep 2 dlaždice, tour plynulý.
4. Studio: show 4 entries (webtoon 3 sloupce, pásy, sweep 2×2, mezihra) →
   „Zkontrolovat offline“ zelené → zastavit fake galerii a katalog
   (`WEBTOON_CATALOG_URL` + `FINETUNE_URL` na neexistující port, restart
   serveru) → ▶ → plátno přehraje všechny čtyři scény v pořadí a délkách,
   dvakrát za sebou se stejným seedem = stejné pořadí dlaždic a start sweepu
   (screenshoty ve stejném čase od startu porovnej). PANIC pauzne.
5. React Cockpit: nové scény a parametry odesílají totéž co Svelte
   (`sceneParams` test), ▶ ⏸ ⏭ fungují, konzole bez chyb.
6. Screenshoty pro uživatele: tři sloupce webtoonu, pásy, sweep 2×2, Studio.

## D23 šablona do `DECISIONS.md`

```
## D23: Stripy, Pásy, StyleMapy ze session a Scene Studio (2026-11-xx)
Zadání: `Prompts/04-STRIPY-STYLEMAPY-STUDIO.md`.
- Webtoon příběhy jdou z katalogu Toonfork (zip packy po partech, sha256),
  panely bez textu + bubliny kreslené plátnem; `adult` jen s WEBTOON_ALLOW_ADULT.
- Pásy jsou vlastní scéna z fronty; vzhled podle Restyleru, mřížku skládá plátno.
- `breeze` (Wan) je generativní: vždy čeká na operátora, po schválení hraje
  místo fotky (jako živý portrét), nikdy jako položka fronty.
- StyleMap packy ze session staví PartyCockpit sám v Go (bez CLIP/UMAP/LPIPS),
  ve schématu Ol1nLLM; řazení obrazovým odstupem. Ol1nLLM packy zůstávají
  „velké“, lokální mají `source:"party"`.
- Show = seznam scén s položkami a seedem; runner na serveru; plátno s
  `?preview=` kreslí entry bez vlivu na TV. Operátorův obsah ze Studia se
  schvaluje automaticky (restyle), generativní klipy ne.
- GPU přes `COMFY_URL` z okna SPARK broker nebo L40S; PartyCockpit broker nevolá.
- Studio jen Svelte; React Cockpit má paritu pro ovládání na party.
- Co se ukázalo: … (doplň)
```

## Co nedělat

- Neměň sémantiku schvalování pro hosty (D5, D13, D19). Studio výjimka platí
  jen pro `meta.studio` a jen pro restyle.
- Nepřepisuj Ol1nLLM `tools/stylemap`, FineTuneGallery ani Webtoon; všechno
  potřebné je v PartyCockpitu. Když by FineTuneGallery potřebovala
  `source_image_id` v API, zapiš to do D23 jako nález, neimplementuj tam.
- Žádné nové závislosti ve frontendu (drag & drop = ↑ ↓ tlačítka); v Go jen
  stdlib + `golang.org/x/image`.
- Žádný `confirm()`/`alert()`; žádný token ani klíč plátna v URL kromě
  `?key=` do iframu náhledu (stejný mechanismus jako „Otevřít plátno“).
- Nic na SPARKu nespouštěj ani nezastavuj; GPU jen přes skill `spark-job`,
  a jen když to uživatel chce ověřit naostro.
- Nepushuj. Na konci napiš česky, co je hotové, co bylo ověřeno v Chrome,
  co zůstalo neověřené (reálný Comfy, Pi) a co průzkum odhalil o protokolu.
