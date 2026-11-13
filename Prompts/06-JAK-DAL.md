# Zadání: Jak dál po D23 (opravy z notebooku, sloučení, tributy, ostrý test)

Jsi Claude session (Opus) v repu PartyCockpit (veřejné repo
https://github.com/lioilsources/PartyCockpit). Tohle není jedna funkce, ale
pořadí kroků. Každý krok, ve kterém vzniká kód, je samostatný požadavek
s vlastním PR. Jdi popořadě, po kroku krátce česky nahlas a pokračuj. Stůj
jen u 🛑: tam rozhoduje Olin nebo chybí vstup.

Kroky 1 až 3 čekají na Olina. Když na něj čekáš, dělej krok 4 (tributy),
ten od něj nic nepotřebuje.

## Než začneš

- Čti `CLAUDE.md`, `STATUS.md` (hlavně „Neověřeno“ a „Další kroky“),
  `DECISIONS.md` D22 a D23, `docs/studio.md` a `Prompts/03-COCKPIT-REACT.md`
  sekci „Než začneš“.
- Repo nemá CI. Jediná brána je lokální `make check`, pro Windows navíc
  `GOOS=windows GOARCH=amd64 go vet ./...` v `backend/`.
- Commit po kroku, `git add` jmenovitě. **Push, PR, merge, mazání větví
  a vydání jen na Olinův pokyn v chatu.**
- Žádný force push, žádné `reset --hard`, žádný rebase větví, které už jsou
  na GitHubu. Navazující větve se aktualizují mergem dopředu.
- Pracuj ve worktree. V hlavním checkoutu nic neměň (viz Stav).
- Veřejné repo: žádné LAN IP, tokeny, domácí cesty, ani v commitech a PR.

## Stav k 2026-11-12

Ověř jen `gh pr list --state all`, zbytek neprozkoumávej znovu.

- `main` je na D22, vydání `v0.1.0` z 6. 11. (tři balíčky: Windows zip,
  Linux amd64 a arm64).
- **Stack D23** čeká na sloučení, všechny čtyři PR jsou mergeable a každá
  špička prošla bránou:
  #2 `feat/webtoon` → `main`, #3 `feat/strips` → `feat/webtoon`,
  #4 `feat/stylemap-session` → `feat/strips`,
  #5 `feat/studio-show` → `feat/stylemap-session`.
- **PR #1** `i18n/all-langs` (lokalizace webu pro hosty, 80 souborů) je
  cizí rozdělaná práce. Se stackem se překrývá v 10 souborech, mimo jiné
  `Stage.svelte`, scény plátna, `handlers.go`, `server.go`, `CLAUDE.md`.
- **Hlavní checkout** je na `main` a má Olinovy necommitnuté změny ve dvou
  souborech: `frontend/src/stage/Stage.svelte` (QR se neukazuje ve scéně
  Now playing) a `backend/internal/api/preflight.go` (text nápovědy
  u ComfyUI). Stack mění oba soubory, takže `git pull` tam po sloučení
  neprojde. Neřeš to stashem ani zahozením, viz krok 3.
- V hlavním checkoutu leží necommitnuté `Prompts/05-TRIBUTY-NA-PLATNO.md`
  a tenhle soubor.
- Worktree `.claude/worktrees/studio` je na `feat/studio-show`. V jeho
  `dist/` jsou balíčky sestavené ze špičky #5, které Olin zkouší na
  notebooku. Worktree nemaž, dokud Olin neřekne (`dist/` je ignorovaný
  a zmizel by s ním).
- **Olin zkouší na notebooku:** přenos přípravy (složky `shows`, `webtoon`,
  `stylemaps`, `gallery-cache`), běh offline, výkon na TV (Webtoon ve
  4 sloupcích, Pásy s klipy, StyleMap 3×3), hodinový loop, F5 a PANIC
  uprostřed scény, lištu show v obou Cockpitech a na Windows práci se
  soubory (stažení a smazání balíku, uložení a načtení show).
- **Nikdy neběželo naostro:** restyle, živý portrét a vlasy ve větru
  z PartyCockpitu proti skutečnému ComfyUI. Všechno je ověřené jen na
  mocku a proti falešnému serveru.

## Krok 1: Nálezy z notebooku

🛑 Vstup: co Olin na notebooku zjistil. Když to v chatu není, zeptej se
jednou a mezitím dělej krok 4.

- Opravy commituj navrch stacku na `feat/studio-show` (PR #5). Nevracej se
  do nižších větví: všechny čtyři PR se slučují hned po sobě.
- Čekej hlavně dvě věci. Na Windows soubory (přejmenování přes existující
  složku, mazání balíku, který se zrovna čte, oddělovače cest). Na TV výkon:
  nejdřív zkus stávající páky (`PC_STAGE_SCALE`, počet sloupců, `maxClips`
  v Pásech) a navrhni lepší výchozí hodnoty, scény nepřepisuj.
- Po opravě `make check`, kontrola pro Windows, `make dist` a pošli Olinovi
  nový Windows balíček (SendUserFile). Push na pokyn.
- Když test dopadl čistě, krok přeskoč.

## Krok 2: Sloučení stacku

🛑 Jen na Olinovo „mergni“.

Merge commitem, nikdy squash (další PR by pak ukazoval cizí změny
a kolidoval). Vždy s `-R`, ať `gh` nesahá na lokální checkout:

```sh
R=lioilsources/PartyCockpit
gh pr merge 2 -R $R --merge
gh pr edit 3 -R $R --base main && gh pr merge 3 -R $R --merge
gh pr edit 4 -R $R --base main && gh pr merge 4 -R $R --merge
gh pr edit 5 -R $R --base main && gh pr merge 5 -R $R --merge
```

- Před každým merge ověř `gh pr view <n> -R $R --json mergeable,baseRefName`.
- Po posledním: čistý worktree na `origin/main` a `make check`. Když je
  červený, hned oprav vlastním PR a řekni to.
- Větve maž jen na „smaž větev“ (`git push origin --delete <větev>`).
- V hlavním checkoutu `git pull` nespouštěj.

## Krok 3: Po sloučení

🛑 Zeptej se Olina jednou, co s jeho dvěma změnami. Doporuč tohle:

- Uděláš je znovu ve worktree na čerstvém `main` jako jeden malý PR
  („QR pryč z Now playing, nápověda ComfyUI v preflightu“). Obě jsou na
  jeden řádek, přečti si je z `git diff` v hlavním checkoutu.
- Po sloučení toho PR si Olin lokální kopie zahodí a stáhne `main`.
  Zahodit je za něj smíš jen na jeho výslovný pokyn.

PR #1 (i18n): po sloučení stacku nejspíš koliduje. Větev needituj a nic do
ní nemerguj. Olinovi nahlas `mergeable` a seznam kolidujících souborů. Až
#1 projde, texty nových scén na plátně (Pásy, Webtoon) zůstanou česky
natvrdo: navrhni na ně samostatný PR stejným mechanismem, jaký #1 zavádí.

## Krok 4: Tributy

Implementuj podle `Prompts/05-TRIBUTY-NA-PLATNO.md`. Předpoklady v něm Olin
viděl 11. 11. a zeptal se jen na zdroj, takže platí. Základ větve podle
pravidla v plánu (po kroku 2 z `main`, jinak z `feat/studio-show`).

🛑 Na konci ostrá zkouška, vstup od Olina: `PROMO_API_URL` a `PROMO_TOKEN`
(agent token PromoClown) v lokálním `.env` na stroji v domácí síti. `.env`
sousedních rep nečti a adresu nikam do repa nepiš.

- Studio → 🎨 Tributy ukáže skutečné tributy, stáhni dva, pusť scénu
  v obou rozloženích a pošli Olinovi screenshot plátna.
- Zkontroluj, že název stylu pod snímkem sedí k obrázku. Pořadí
  `media_paths` proti `styles` je vyčtené jen ze zdrojáků PromoClown.
- Když se skutečnost liší od sekce „Co průzkum zjistil“ v plánu 05, oprav
  kód, plán i D24.

## Krok 5: Studio naostro na GPU

Kroky 4 a 5 jdou prohodit. Krok 5 potřebuje okno na GPU a fotky, krok 4 nic.

**Proč.** Workflow `flux_restyle.json`, `live_portrait.json` a `wan_i2v.json`
se ladily na L40S (D13). Na ComfyUI na SPARKu můžou chybět nody nebo
modely, a z PartyCockpitu je nikdo naostro nepustil.

**Broker.** SPARK jen přes něj, nic tam sám nespouštěj ani nezastavuj.

```sh
sparkctl status && sparkctl catalog            # služba comfyui, strop priority projektu
id=$(sparkctl lease partycockpit comfyui 90 <priorita> "Studio naostro: restyle, portrét, vlasy")
sparkctl hb $id                                # aspoň každých 5 min (nebo sparkctl hold $id)
sparkctl done $id '{"images":{"ok":N,"failed":M}}'   # nebo: sparkctl fail $id "<co se stalo>"
```

Prioritu dej nejvýš strop projektu z katalogu. Když úloha zůstane `waiting` přes
termín, řekni to Olinovi. Když Olin sám řekne, že chce okno, použij
`sparkctl window` s jeho doslovnými slovy. Na ComfyUI může zároveň běžet
okno jiné session: cizí joby nepřerušuj (`/interrupt`, `/free` nevolej).

`COMFY_URL` vezmi ze soukromých poznámek (`~/.claude/shared/infra.md`,
`ports.md`) a dej jen do lokálního `.env`.

**5a Inventura** (prvních 10 minut okna). Z `GET $COMFY_URL/object_info`
porovnej `class_type` všech nodů ve třech workflow a jména modelů, která
workflow chtějí. 🛑 Když něco chybí, pošli Olinovi tabulku (workflow, node
nebo model, co chybí) a skonči krok: instalovat na SPARK není na tobě.
Náhradní cesta je okno na L40S, to domlouvá Olin.

**5b Běh.** Server na `127.0.0.1:18098` s ostrým `.env`,
`RESTYLE_PROVIDER=comfy`, `ANIMATE_PROVIDER=comfy`, `PC_WORKERS=1`,
`PC_ANIM_WORKERS=1` (v ComfyUI vždy jen jeden job).

- 🛑 Vstup: tři portréty od Olina. Fotky skutečných lidí z internetu
  nestahuj.
- Studio → 📸 Fotky: každou fotku ve dvou stylech, pak ✨ na všechny
  a 💨 na jednu. Wan běží kolem 25 minut bez checkpointu a vyšší priorita
  ho může odsunout: po odsunutí zařaď znovu.
- U každého jobu čti `node_errors`. ComfyUI node s chybějícím vstupem tiše
  vyřadí a ohlásí úspěch.
- Zapiš časy, selhání a dojem z kvality. Ukázky (3 restyly, 1 klip) pošli
  Olinovi.
- Pak generálka podle `docs/studio.md`: show, ✅ Offline, 💾 Knihovna.
  Přenos na notebook dělá Olin.

Opravy jdou vlastním PR („Studio naostro: opravy“). Do `STATUS.md` přesuň
ověřené věci z „Neověřeno“ a zapiš naměřené časy.

## Krok 6: Vydání v0.2.0

🛑 Jen na Olinovo „vydej“ nebo „nasaď“. Vydání je zveřejnění.

- Podmínka: stack je v `main`, `make check` na `main` je zelený. Co dalšího
  má být uvnitř (tributy, opravy z kroku 5), rozhodne Olin.
- `make dist` z `main`, tag `v0.2.0`, `gh release create` se stejnými třemi
  balíčky jako `v0.1.0` a českými poznámkami (co je nového: D23, případně
  D24).
- Balíček pro macOS `make dist` nedělá. Když ho Olin chce, je to vlastní
  malý PR.

## Krok 7: Zbytek „Neověřeno“

Nedělej naslepo. Vyber, na co je zrovna vstup, zbytek nech v `STATUS.md`.

| Co | Co je k tomu potřeba |
|---|---|
| StyleMap ze skutečné session FineTuneGallery | CF Access token v `.env`, bez GPU |
| Titulky s ASR přes Translator | doplněk `asr` přes broker |
| Živé streamy z telefonů, hra Kruh na Wi-Fi | Olin, telefony na LAN, https certifikát |
| Party box na Raspberry Pi (`deploy/box/README.md`) | Pi 5 a Olin u něj |
| MirrorBooth „Stream to party“ | session v repu MirrorBooth podle `docs/mirrorbooth-stream.md` |
| CSP z report-only na ostré (D19) | projít konzoli na všech stránkách, vlastní PR |
| M5 3D tanečníci | 🛑 nemá zadání, řekni si o plán |

## Hlášení

- Po každém kroku tři řádky: co je hotové, co je ověřené a čím, co čeká na
  Olina.
- U 🛑 jedna otázka, ne výčet možností.
- `STATUS.md` „Další kroky“ uprav v PR toho kroku, který je splnil.
- Tenhle soubor přidej do prvního PR, který uděláš, pokud Olin neřekne jinak.

## Co nedělat

- Žádný merge, push, vydání ani mazání větví bez Olinova pokynu v chatu.
- Žádný force push, rebase vydaných větví, `reset --hard`, `git stash`
  ani `git pull` v hlavním checkoutu.
- Nesahej na větev `i18n/all-langs`.
- Na SPARKu nic mimo broker.
- Nezačínej nové funkce (M5, další scény) bez zadání.
- Nečti a nekopíruj tajemství ze sousedních rep.
