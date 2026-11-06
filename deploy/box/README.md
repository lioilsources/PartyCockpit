# Party box: runbook

K projektoru nebo TV se připojí jen box (Raspberry Pi 5 nebo mini PC). Na boxu
běží backend PartyCockpitu i plátno v kiosku. Cockpit i hosté jsou v telefonech.
Rozhodnutí a limity: `DECISIONS.md` D20.

## Co koupit

| Položka | Poznámka |
|---|---|
| Raspberry Pi 5, 8 GB | Pi 4 nestačí. Alternativa: mini PC Intel N100 (hardwarové dekódování videa, 9 živých streamů místo 4) |
| oficiální zdroj 27 W USB-C | slabší zdroj = podtaktování |
| Active Cooler + krabička s místem na něj | bez chladiče Pi při shaderech zpomaluje |
| microSD 64 GB A2, nebo NVMe HAT + SSD | SSD je rychlejší a vydrží víc zápisů |
| kabel micro-HDMI → HDMI | |
| cestovní router (GL.iNet Beryl AX / Slate AX) | Wi-Fi pro hosty, DHCP, DNS a připojení k internetu místa. Wi-Fi na Pi utáhne jednotky telefonů, ne desítky. |
| USB mikrofon (volitelně) | beat z hudby v sále (`PC_BOX_AUDIO=1`) |

## 1. Image

1. Raspberry Pi Imager → **Raspberry Pi OS Lite (64-bit), Bookworm**. V nastavení:
   hostname `partybox`, uživatel `admin` s SSH klíčem (heslo vypnout), Wi-Fi
   nevyplňovat (box jede po Ethernetu do routeru), locale a časová zóna.
2. První start na Ethernetu s internetem, pak `ssh admin@partybox.local`.
3. Aktualizace:
   ```sh
   sudo apt update && sudo apt full-upgrade -y && sudo rpi-eeprom-update -a
   ```
4. `sudo raspi-config` → Display: rozlišení HDMI 1920×1080 (4K Pi nezvládne
   plynule). `sudo reboot`.
5. Vzdálená správa (doporučeno):
   ```sh
   curl -fsSL https://tailscale.com/install.sh | sh && sudo tailscale up
   ```
   Pak `ssh admin@partybox` funguje odkudkoli a box dosáhne na GPU servery
   v tailnetu.

Na mini PC: Debian 12 netinst bez desktopu, uživatel `admin`, SSH server;
zbytek je stejný.

## 2. PartyCockpit

Balíček se staví na vývojovém stroji: `make dist` →
`dist/partycockpit-linux-arm64.tar.gz` (Pi) a `…-amd64.tar.gz` (mini PC).

```sh
scp dist/partycockpit-linux-arm64.tar.gz admin@partybox:
ssh admin@partybox
tar xzf partycockpit-linux-arm64.tar.gz && cd partycockpit-*-linux-arm64
sudo ./deploy/box/install.sh
```

`install.sh` doinstaluje `cage`, Chromium, `lego` a emoji font, založí
uživatele `party` a `/opt/partycockpit/` (`bin/`, `data/`, `certs/`, `.env`),
nastaví služby `partycockpit` (backend) a `kiosk` (Chromium přes celé HDMI
na tty1) a obě spustí. Opakované spuštění aktualizuje binárku a služby,
`.env` a data nechá.

Nastavení: `sudo nano /opt/partycockpit/.env` (šablona `box.env`, všechny
proměnné v `.env.example`), pak
`sudo systemctl restart partycockpit kiosk`.

Co se stane po startu:
- s `PC_BOX_AUTOPARTY=1` box založí party a na TV ukáže plátno s
  **párovacím QR** vpravo nahoře. Naskenuj ho telefonem → máš Cockpit. QR
  zmizí, jakmile se Cockpit připojí, nebo po 10 minutách. Znovu ho ukáže
  Cockpit („Před party“ → Párovací QR na plátno) nebo restart boxu;
- bez automatické party ukáže TV obrazovku boxu s QR na úvodní stránku;
  party založíš z telefonu (první party na boxu nepotřebuje admin token)
  a TV se do 5 s přepne na plátno.

Logy: `journalctl -u partycockpit -u kiosk -f`.

## 3. HTTPS (jednou, s internetem)

Kamera v telefonech hostů (záložka „Živě“) a mikrofon v Cockpitu (titulky)
fungují jen přes https. Telefony nepřijmou vlastní certifikát, proto
skutečný certifikát pro vlastní doménu, ověřený přes DNS (box nemusí být
z internetu vidět):

1. Doména u Cloudflare, třeba `box.example.com`. Záznam
   `A box.example.com → <IP-boxu>` (LAN IP boxu v síti routeru, rezervovaná
   v DHCP). Veřejné DNS smí ukazovat na privátní adresu.
2. Cloudflare API token s právem **Zone → DNS → Edit** jen pro tu zónu.
3. Na boxu:
   ```sh
   sudo tee /opt/partycockpit/lego.env >/dev/null <<EOF
   CLOUDFLARE_DNS_API_TOKEN=…
   LEGO_EMAIL=…
   LEGO_DOMAIN=box.example.com
   EOF
   sudo chmod 600 /opt/partycockpit/lego.env
   sudo /opt/partycockpit/bin/renew-cert.sh
   sudo systemctl enable --now renew-cert.timer
   ```
4. Do `.env`:
   ```
   PC_PUBLIC_URL=https://box.example.com:8443
   PC_TLS_CERT=/opt/partycockpit/certs/box.example.com.crt
   PC_TLS_KEY=/opt/partycockpit/certs/box.example.com.key
   ```
   a `sudo systemctl restart partycockpit kiosk`.

Certifikát platí 90 dní a časovač ho obnovuje týdně, když má box internet.
Box obnovený doma tedy veze platný certifikát i na party bez internetu.
Stav ukazuje obrazovka boxu a „Před party“ v Cockpitu (varování 14 dní
před koncem).

## 4. Router

- Wi-Fi pro hosty: vlastní SSID, **izolaci klientů vypnout** (živé streamy
  jdou přímo mezi telefonem a plátnem), 2,4 i 5 GHz.
- Box po kabelu, v DHCP rezervace pevné IP.
- DNS: vlastní záznam `box.example.com → <IP-boxu>` (GL.iNet: Network → DNS →
  vlastní záznamy, nebo dnsmasq `address=/box.example.com/<IP-boxu>`), aby
  adresa fungovala i bez internetu.
- Internet: opakovač Wi-Fi místa nebo LTE modem. Bez internetu jede party
  s mockem: vzkazy, hra, streamy, shadery a recap fungují, restyly a tance ne.

## 5. Záložní karta (golden image)

Po nastavení smaž testovací party (`sudo systemctl stop partycockpit && sudo
rm -rf /opt/partycockpit/data/*`), pak klon na druhou kartu (`rpi-clone`) nebo
`dd` do souboru a zmenšit `pishrink`. `lego.env` na kartě nech, je potřeba
k obnově certifikátu. Náhradní kartu mít na party v kapse.

## 6. Před party (navíc k „Před party“ v Cockpitu)

- Box zapnout 20 min předem, pustit shader přestávku: teplota v „Před party“
  pod 75 °C.
- Telefon na Wi-Fi hostů: QR z plátna otevře https bez varování a kamera
  v „Živě“ se spustí.
- Druhý telefon: vzkaz na zeď, schválit z Cockpitu, objeví se na plátně.
- Vytáhnout internet na minutu: plátno hraje dál, Cockpit se po návratu
  připojí sám.

## 7. Aktualizace

```sh
scp dist/partycockpit-linux-arm64.tar.gz admin@partybox:
ssh admin@partybox sudo /opt/partycockpit/bin/update.sh partycockpit-linux-arm64.tar.gz
```

Bez argumentu `update.sh` stáhne poslední GitHub Release
(`partycockpit-linux-<arch>.tar.gz`), až budou releasy vycházet. Samotnou
binárku vymění `update.sh cesta/k/partycockpit`.

## Limity Raspberry Pi 5

- Živé streamy: nejvýš 4 (`PC_MAX_LIVE=4`), Pi nemá hardwarový dekodér H.264.
- Klipy tanců a živých portrétů: 480×832 (výstup L40S) zvládá, 1080p ne.
- Shadery na poloviční rozlišení (`PC_STAGE_SCALE=0.5`), výstup 1080p.
- Beat jen z DJfy, z tapu v Cockpitu nebo z USB mikrofonu.
