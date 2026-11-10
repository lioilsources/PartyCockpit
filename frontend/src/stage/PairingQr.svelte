<script lang="ts">
  import { t } from '../lib/i18n.svelte';
  import QRCode from 'qrcode';

  // Pairing QR on a party box's stage (D20): the organiser scans it and the
  // phone becomes the Cockpit. It goes away when a Cockpit connects or after
  // ten minutes; the token is on the TV only during that setup window.
  let { url }: { url: string } = $props();
  let src = $state('');
  $effect(() => {
    QRCode.toDataURL(url, { margin: 1, width: 440, color: { dark: '#0b0613', light: '#ffffff' } }).then((d) => (src = d));
  });
</script>

{#if src}
  <div class="pair">
    <img {src} alt={t('stage.pairAlt')} />
    <strong>{t('stage.pairScan')}</strong>
    <span>{t('stage.pairSub')}</span>
  </div>
{/if}

<style>
  .pair {
    position: absolute;
    top: 2.2vw;
    right: 2.2vw;
    z-index: 35;
    background: #fff;
    color: #0b0613;
    border-radius: 1.2vw;
    padding: 1vw 1vw 0.8vw;
    text-align: center;
    box-shadow: 0 0 3vw rgba(255, 61, 139, 0.6);
    animation: glow 2s ease-in-out infinite alternate;
  }
  img {
    display: block;
    width: 15vw;
    image-rendering: pixelated;
  }
  strong {
    display: block;
    font-size: 1.5vw;
    margin-top: 0.5vw;
  }
  span {
    font-size: 1vw;
  }
  @keyframes glow {
    to {
      box-shadow: 0 0 4vw rgba(61, 232, 255, 0.7);
    }
  }
</style>
