# Scene Studio: show připravená dopředu, na party offline

Studio (`/studio/<kód>`, odkaz z Cockpitu i z úvodní stránky) skládá show ze
scén. Každá scéna má své položky (epizody webtoonu, fotky do pásů, StyleMap
dlaždice, obrázky slideshow), délku a seed. Server ji na party přehrává
scénu po scéně. Se stejným seedem se všechno hýbe pokaždé stejně: tempo
sloupců, začátky, pořadí. Rozhodnutí: `DECISIONS.md` D23.

## Příprava (doma, s internetem a GPU)

1. **Party na přípravu.** Založ party (název třeba „Příprava Ema 40“) a otevři
   Studio. Konec party (TTL 24 h) nevadí: hotovou show uložíš do knihovny.
2. **GPU okno.** Restyle a klipy počítá ComfyUI na `COMFY_URL`
   (`RESTYLE_PROVIDER=comfy`, `ANIMATE_PROVIDER=comfy`). Na SPARKu jen přes
   broker:

   ```sh
   sparkctl status && sparkctl catalog          # která služba je ComfyUI
   id=$(sparkctl lease partycockpit <služba> 120 3 "Studio: show Ema 40")
   # do .env COMFY_URL na adresu okna, restart PartyCockpitu
   sparkctl hb $id                              # aspoň každých 5 min
   ```

   Na ComfyUI musí být modely restyle_flux (FLUX dev, PuLID, depth),
   LivePortrait a Wan 2.2 (D13, D22). Alternativa je L40S (D13).
3. **Fotky** (Příprava → 📸 Fotky): nahraj, vyber styl a popisek. Hotový restyle
   se schválí sám a jde do show fronty. Pak „✨ Oživit“ (mrknutí, úsměv)
   nebo „💨 Vlasy ve větru“. Klipy vlasů schválíš ✓ ve Studiu. Na GPU se
   počítá jednou, výsledek je soubor v `PC_DATA_DIR/party/<kód>/`.
4. **Webtoon** (📚 Webtoon): stáhni balíky z katalogu Toonfork. Zipy se ověří
   (sha256) a rozbalí do `PC_DATA_DIR/webtoon/packs/`. Dospělé balíky jen
   s `WEBTOON_ALLOW_ADULT=1`.
5. **StyleMap** (🧪 StyleMap): vyber session z FineTuneGallery a zaškrtni
   prompty. Každý prompt je jeden pack v `PC_DATA_DIR/stylemaps/`, stavba
   trvá sekundy a GPU nepotřebuje.
6. **Show**: přidávej scény na konec, ↑ ↓ pořadí, ⧉ kopie, délka, seed
   (🎲 jiný průběh) a StyleMap přechod. Náhled kreslí vybranou scénu od
   začátku a po každé změně ji pustí znovu.
7. **✅ Offline**: „Stáhnout do cache“ (obrázky slideshow, packy
   z FineTuneGallery), pak „Zkontrolovat offline“. Zelené = scéna poběží
   bez sítě.
8. **💾 Knihovna → Uložit**: show s fotkami a klipy jde do
   `PC_DATA_DIR/shows/<název>/`. Úklid party ji nemaže.
9. `sparkctl done $id '{"images":{"ok":N,"failed":M}}'`

## Na party (offline)

1. Na notebooku u TV spusť PartyCockpit se stejným `PC_DATA_DIR`. Internet ani
   GPU nejsou potřeba. Když jsi připravoval na jiném počítači, zkopíruj z jeho
   `PC_DATA_DIR` složky `shows`, `webtoon`, `stylemaps` a `gallery-cache` do
   `data` vedle programu na notebooku. `partycockpit.db` a `party` nekopíruj:
   party se zakládá na notebooku znovu.
2. Založ party, otevři Studio → 💾 Knihovna → Načíst → „Nahradit současnou
   show“. Fotky a klipy se zkopírují do party jako schválené.
3. Otevři plátno (Cockpit → „Otevřít plátno“), ve Studiu nebo v Cockpitu ▶.
   `?hud=1` na plátně ukáže, kde show je.
4. PANIC v Cockpitu show zastaví (skočí na slideshow). ▶ ji pustí od místa,
   kde stála.

Hosté můžou dál posílat fotky a vzkazy: scény Galerie, Vzkazy a Pásy bez
vybraných fotek hrají celou show frontu, takže se k nim přidávají.

## Co poběží bez sítě

| Scéna | Offline, když |
|---|---|
| Webtoon | epizody stažené ve 📚 Webtoon (Party epizoda vždy) |
| Pásy | fotky a klipy jsou v party (Studio nebo knihovna) |
| StyleMap | lokální packy vždy, packy z FineTuneGallery po „Stáhnout do cache“ |
| Slideshow | jen s vybranými obrázky po „Stáhnout do cache“ |
| Galerie, Vzkazy, Tančírna, Recap | vždy (show fronta party) |
| Shader přestávka, Mezihra | vždy |
| Now playing | potřebuje DJfy (síť) |
