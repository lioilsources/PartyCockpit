<script lang="ts">
  import { onDestroy } from 'svelte';
  import QRCode from 'qrcode';

  // Party box screen (D20): what the TV shows before there is a party. The
  // backend serves /box to the box's own kiosk only; once a party exists the
  // same URL redirects to its stage, so the page just reloads until then.
  type Info = { hostname: string; ips: string[]; url: string; version: string; autoParty: boolean; certUntil?: number };
  let info = $state<Info | null>(null);
  let qr = $state('');
  let err = $state('');

  async function load() {
    try {
      const r = await fetch('/api/box');
      if (!r.ok) throw new Error(`${r.status}`);
      info = await r.json();
      if (info?.url) qr = await QRCode.toDataURL(`${info.url}/`, { margin: 1, width: 520, color: { dark: '#0b0613', light: '#ffffff' } });
    } catch (e: any) {
      err = 'backend se ještě spouští…';
    }
  }
  load();
  // a party created from a phone (or the autoparty) turns /box into a redirect
  const timer = setInterval(() => location.reload(), 5000);
  onDestroy(() => clearInterval(timer));

  const certDays = $derived(info?.certUntil ? Math.floor((info.certUntil - Date.now()) / 864e5) : null);
</script>

<main class="box">
  <h1>Party<span>Cockpit</span> box</h1>
  {#if info}
    <div class="grid">
      <div class="qr">
        {#if qr}<img src={qr} alt="QR" />{/if}
        <p>{info.autoParty ? 'Zakládám party…' : 'Naskenuj a založ party z telefonu'}</p>
      </div>
      <dl>
        <dt>Adresa pro telefony</dt>
        <dd>{info.url || 'box nemá síť'}</dd>
        <dt>Zařízení</dt>
        <dd>{info.hostname} · {info.ips.join(', ') || 'bez IP'}</dd>
        <dt>Certifikát</dt>
        <dd class:bad={certDays !== null && certDays < 14}>
          {certDays === null ? 'není (kamera a mikrofon v telefonech nepojedou)' : certDays < 0 ? 'vypršel' : `platí ${certDays} dní`}
        </dd>
        <dt>Verze</dt>
        <dd>{info.version}</dd>
      </dl>
    </div>
  {:else}
    <p class="muted">{err || 'načítám…'}</p>
  {/if}
</main>

<style>
  .box {
    position: fixed;
    inset: 0;
    display: grid;
    place-content: center;
    gap: 4vh;
    background: radial-gradient(circle at 50% 40%, #2a0f45, #0b0613 70%);
    cursor: none;
  }
  h1 {
    margin: 0;
    font-size: 4vw;
    text-align: center;
  }
  h1 span {
    color: var(--pink);
  }
  .grid {
    display: grid;
    grid-template-columns: auto auto;
    gap: 5vw;
    align-items: center;
  }
  .qr {
    text-align: center;
  }
  .qr img {
    width: 22vw;
    border-radius: 1vw;
    image-rendering: pixelated;
  }
  .qr p {
    font-size: 1.6vw;
    color: var(--muted);
  }
  dl {
    margin: 0;
    font-size: 1.6vw;
  }
  dt {
    color: var(--muted);
    font-size: 1.1vw;
    margin-top: 1.4vh;
  }
  dd {
    margin: 0;
  }
  dd.bad {
    color: var(--red);
  }
</style>
