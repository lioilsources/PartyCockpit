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

## Další kroky
1. Vyplnit `.env` (Restyler token, CF Access token) a vyzkoušet s živým Restylerem a DJfy.
2. Rozhodnout, kde poběží party box. Na JODA je to Compose s portem v LAN (viz `deploy/`).
3. M3 LiveKit + MirrorBooth (LAN-only, bez TURN).
4. M4 idle animace galerie a 2D tanec. Restyler umí `live_portrait` a `wan_motion`,
   takže je to kandidát i na tanec / „Oživit“.
5. HandWrittenStickers jako HTTP služba (glyph extraktor).
6. Hry o přestávce (Three.js, co-op), napojené na výsledek losování.
