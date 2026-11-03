# MirrorBooth → PartyCockpit: „Stream to party“

Zadání pro session v repu MirrorBooth (M3). PartyCockpit má serverovou část hotovou a
otestovanou (`backend/internal/api/streams.go`, `streams_test.go`). Prohlížečová verze
stejného protokolu je `frontend/src/guest/Live.svelte` + `frontend/src/lib/rtc.ts`
(třída `Publisher`), takže se dá použít jako referenční implementace.

Buildy iOS a Androidu dělej na MacBooku, ne na M2.

## Tok

1. **Připojení k party.** Host naskenuje QR na plátně. QR obsahuje `<base>/join/<KÓD>`
   (např. `http://<LAN-IP>:8098/join/AB3KX`). MirrorBooth z něj vezme `base` a `KÓD`.
   Obrazovka „Stream to party“ může mít vlastní skener QR, nebo pole pro vložení odkazu.
2. **Host.** `POST <base>/api/parties/<KÓD>/guests` s `{"nickname": "..."}` vrátí
   `{"id": "pc…", "nickname": …}`. `id` si ulož podle party a při dalším streamu ho použij znovu.
3. **Stream.** `POST <base>/api/parties/<KÓD>/guests/<id>/stream` s `{"source": "mirrorbooth"}`
   vrátí `{"stream": {id, status: "pending", …}, "key": "<32 hex>"}`. Opakované volání
   vrátí stejný stream i klíč.
4. **Socket.** `ws(s)://<base>/ws/parties/<KÓD>?role=streamer&key=<key>`. Do 90 s od
   kroku 3, jinak stream propadne. Když se socket odpojí, stream vydrží 20 s a během té
   doby se dá znovu připojit.
5. **Zprávy ze serveru** (JSON `{type, data}`):
   - `party`: obsahuje `iceServers` (na LAN party `[]`). Předej je do `createPeerConnection`.
   - `stream`: `{id, status: pending|live|ended, reason?}`. Ukaž stav: „čeká na operátora“,
     „🔴 jsi na plátně“, nebo konec s důvodem.
   - `rtc` s `kind: "offer"`: `{from, stream, sdp}`. Divák (plátno nebo náhled v Cockpitu)
     chce video. Pro každé `from` vytvoř **vlastní** `RTCPeerConnection`, přidej video track,
     `setRemoteDescription(offer)`, `createAnswer`, `setLocalDescription` a pošli
     `{"type":"rtc","data":{"kind":"answer","to":from,"sdp":…}}`.
     Když přijde nový offer od stejného `from`, starou peer connection zavři.
   - `rtc` s `kind: "ice"`: `{from, candidate}` → `addIceCandidate`. Volání řaď za
     `setRemoteDescription` téže peer connection, ať se kandidát neztratí.
   - `rtc` s `kind: "bye"`: `{from}` → zavři peer connection pro `from`.
6. **Odchozí ICE.** `onIceCandidate` → `{"type":"rtc","data":{"kind":"ice","to":from,"candidate":{candidate,sdpMid,sdpMLineIndex}}}`.
7. **Konec.** `DELETE <base>/api/parties/<KÓD>/guests/<id>/stream`, pak zavři všechny peer
   connections a socket.

Posílá se jen video, bez zvuku (hudba hraje z reproduktorů). Doporučené parametry:
720p nebo méně, 24 fps, `maxBitrate` 1,2 Mbps na každou peer connection přes
`sender.setParameters`. Plátno přijímá až 9 streamů najednou.

## Filtry: to hlavní

Dnešní `webrtc_service.dart` posílá surovou kameru z `getUserMedia`. Shadery
(`shaders/filter_*.frag`) se aplikují jen v náhledu. Aby byli na plátně vidět ufoni,
musí do WebRTC jít **výstup shaderu**. Možnosti od nejlevnější:

1. **Zrcadlo bez shaderu.** Symetrický obličej jde udělat i nativně, podobně jako
   `mirror_channel.dart`. Je to rychlé, ale filtry tím nepokryješ.
2. **Snímky shaderu → vlastní video zdroj.** Snímek z `RepaintBoundary` nebo z fragment
   programu se přes platform channel (iOS `RTCVideoCapturer`/`RTCVideoSource`, Android
   `VideoSource.capturerObserver`) pošle jako `RTCVideoFrame`. `flutter_webrtc` vlastní
   capturer nemá, takže je potřeba malý nativní plugin.
3. **Shader nativně (Metal/GLES) přímo nad CVPixelBuffer z kamery** a výsledek rovnou do
   WebRTC. Nejvyšší výkon, ale shadery se musí přepsat.

Doporučení: začni variantou 2 na iOS. Snímky stačí v 15–24 fps a 540p.

## Síť (LAN)

- Nativní WebRTC posílá skutečné IP adresy (žádné mDNS), takže se na LAN spojí s plátnem
  i bez TURN.
- **iOS:** do `Info.plist` přidej `NSLocalNetworkUsageDescription`, jinak iOS
  blokuje přístup na LAN IP party boxu.
- **Android:** `android:usesCleartextTraffic="true"`, nebo network security config pro
  LAN, protože party běží přes `http://`.

## DoD (plán M3)

4 telefony s filtry MirrorBooth v mřížce na plátně, latence pod 1 s na LAN.
Ve scéně „📡 Živě“ v Cockpitu musí být vidět náhled, „✓ pustit“, „stáhnout“ a „✕“.
