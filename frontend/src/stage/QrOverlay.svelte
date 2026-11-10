<script lang="ts">
  import { t } from '../lib/i18n.svelte';
  import QRCode from 'qrcode';

  let { url, code }: { url: string; code: string } = $props();
  let src = $state('');

  $effect(() => {
    QRCode.toDataURL(url, { margin: 1, width: 360, color: { dark: '#0b0613', light: '#ffffff' } }).then((d) => (src = d));
  });
</script>

{#if src}
  <div class="qr">
    <img {src} alt={t('stage.joinAlt')} />
    <div class="code">{code}</div>
  </div>
{/if}

<style>
  .qr {
    position: absolute;
    right: 2.2vw;
    bottom: 2.2vw;
    background: #fff;
    padding: 0.6vw 0.6vw 0.3vw;
    border-radius: 1vw;
    box-shadow: 0 0 40px rgba(0, 0, 0, 0.5);
    text-align: center;
    z-index: 20;
  }
  img {
    display: block;
    width: 9vw;
    height: 9vw;
    image-rendering: pixelated;
  }
  .code {
    color: #0b0613;
    font-weight: 700;
    letter-spacing: 0.2em;
    font-size: 1.2vw;
    padding: 0.2vw 0;
  }
</style>
