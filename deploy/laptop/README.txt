PartyCockpit na notebooku (Windows nebo Linux)
==============================================

Notebook je "party box": běží na něm server a plátno v prohlížeči
(přes HDMI na TV nebo projektor). Cockpit a hosté jsou v telefonech
na stejné Wi-Fi.

1. Spuštění
-----------
Windows: rozbal zip, dvakrát klikni na partycockpit.exe.
         Firewall se zeptá na přístup: povol "Privátní sítě".
Linux:   tar xzf partycockpit-linux-amd64.tar.gz
         cd partycockpit-*-linux-amd64 && ./bin/partycockpit

Okno serveru vypíše adresu pro tento počítač (http://localhost:8098)
a adresu pro telefony. Nech ho běžet celou party.

2. Party
--------
- V prohlížeči na notebooku otevři http://localhost:8098, založ party.
- "Otevřít plátno" a F11 (celá obrazovka) = plátno na TV.
- Cockpit otevři v telefonu nebo tabletu (QR nebo odkaz z úvodní stránky).
- Hosté naskenují QR z plátna.

Bez dalšího nastavení běží styly a tance jen jako náhradní efekt (mock),
bez GPU. Vzkazy, schvalování, scény, hra, losování a recap fungují.

3. Nastavení
------------
Zkopíruj env.example (Linux .env.example) jako .env vedle programu
a uprav. Program ho načte sám při startu. Nejčastěji:
  PC_PUBLIC_URL   adresa, kterou otevírají telefony (jde do QR)
  RESTYLE_PROVIDER, ANIMATE_PROVIDER, COMFY_URL   GPU pro styly a tance
  PC_ADMIN_TOKEN  mimo domov: kdo smí zakládat party

4. Kamera hostů a mikrofon (https)
----------------------------------
Telefony povolí kameru (záložka "Živě") a mikrofon (titulky) jen přes
https. Nejjednodušší je Cloudflare Tunnel (potřebuje internet):

- na zkoušku, bez účtu:
    cloudflared tunnel --url http://localhost:8098
  vypíše https://<něco>.trycloudflare.com. Dej ho do .env jako
  PC_PUBLIC_URL, přidej PC_TRUST_PROXY=1 a restartuj server.
- natrvalo s vlastní doménou u Cloudflare:
    cloudflared tunnel login
    cloudflared tunnel create partycockpit
    cloudflared tunnel route dns partycockpit party.example.com
    cloudflared tunnel run --url http://localhost:8098 partycockpit
  a v .env PC_PUBLIC_URL=https://party.example.com a PC_TRUST_PROXY=1.

PC_TRUST_PROXY=1 je nutné: bez něj mají všichni hosté přes tunel stejnou
IP a narazí na limity.

Bez internetu: certifikát pro vlastní doménu (deploy/box/README.md, krok 3,
PC_TLS_CERT a PC_TLS_KEY).

Data party jsou ve složce data vedle programu. Týden po konci party se
smažou sama.
