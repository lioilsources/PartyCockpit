<script lang="ts">
  import { onDestroy } from 'svelte';
  import QRCode from 'qrcode';
  import type { BeatClock } from '../../lib/beat';
  import { fileUrl, recapUrl, type Asset, type Party } from '../../lib/types';
  import { lang, moveLabel, t, tn } from '../../lib/i18n.svelte';
  import Clip from '../../lib/Clip.svelte';

  // Recap (M7): the closing scene. A mosaic of everything approved tonight, one
  // tile at a time in the spotlight, and a QR to /recap/<code> where guests take
  // their photos and dances home. Same items as the show queue, nothing more.
  let { code, party, queue, params, clock }: { code: string; party: Party | null; queue: Asset[]; params: Record<string, any>; clock: BeatClock } =
    $props();

  const media = $derived(queue.filter((a) => a.file && (a.kind === 'restyle' || a.kind === 'upload' || a.kind === 'dance')));
  const notes = $derived(queue.filter((a) => a.kind === 'sticker' && a.meta.text));
  const people = $derived(new Set(queue.map((a) => a.guestId)).size);
  const beats = $derived(Math.max(2, Number(params.beats) || 8));
  const url = $derived(party ? recapUrl(party.joinUrl) : '');
  const purge = $derived(party ? new Date(party.endsAt + 7 * 864e5).toLocaleDateString(lang(), { day: 'numeric', month: 'numeric' }) : '');

  let qr = $state('');
  $effect(() => {
    if (url) QRCode.toDataURL(url, { margin: 1, width: 520, color: { dark: '#0b0613', light: '#ffffff' } }).then((d) => (qr = d));
  });

  let spot = $state(0);
  let count = 0;
  const off = clock.on(() => {
    if (++count % beats === 0 && media.length) spot = (spot + 1) % media.length;
  });
  onDestroy(() => off());

  const hero = $derived(media.length ? media[spot % media.length] : null);
  // enough tiles to fill the wall even with few photos; repeats are scattered
  // (a stride coprime-ish to the 6 columns) so the same face doesn't form a column
  const tiles = $derived(
    media.length ? Array.from({ length: Math.max(media.length, 24) }, (_, i) => media[(i * 5 + Math.floor(i / 6) * 2) % media.length]) : [],
  );
  const lit = $derived(hero ? tiles.findIndex((t) => t.id === hero.id) : -1);
</script>

<div class="recap">
  <div class="wall">
    {#each tiles as a, i (i)}
      <img class:lit={i === lit} src={fileUrl(code, a.kind === 'dance' ? a.meta.photo : a.file)} alt="" loading="lazy" />
    {/each}
  </div>

  <div class="panel">
    <h1>{t('stage.recap.thanks')}</h1>
    <p class="stats">
      {notes.length
        ? t('stage.recap.statsNotes', { images: tn('recap.images', media.length), people: tn('recap.people', people), notes: tn('recap.notes', notes.length) })
        : t('stage.recap.stats', { images: tn('recap.images', media.length), people: tn('recap.people', people) })}
    </p>
    {#if hero}
      {#key hero.id}
        <figure class="hero">
          <Clip src={fileUrl(code, hero.kind === 'restyle' ? (hero.live ?? hero.file) : hero.file)} />
          <figcaption>{hero.meta.nickname ?? t('stage.guest')}{#if hero.kind === 'dance'} · {moveLabel(hero.meta.motion, hero.meta.motionLabel ?? t('stage.recap.dance'))}{/if}</figcaption>
        </figure>
      {/key}
    {/if}
    {#if qr}
      <div class="qr">
        <img src={qr} alt={t('stage.recap.qrAlt')} />
        <div>
          <strong>{t('stage.recap.dl')}</strong>
          <span>{t('stage.recap.scan', { date: purge })}</span>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .recap {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: 1fr 34vw;
    background: #0b0613;
  }
  .wall {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    grid-auto-rows: calc((100vh - 3vw) / 4);
    gap: 0.5vw;
    padding: 1.5vw;
    overflow: hidden;
    mask-image: linear-gradient(90deg, #000 85%, transparent);
  }
  .wall img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 0.5vw;
    filter: saturate(0.7) brightness(0.55);
    transition: filter 0.6s, transform 0.6s;
  }
  .wall img.lit {
    filter: none;
    transform: scale(1.06);
    box-shadow: 0 0 0 0.3vw var(--pink, #ff3d8b);
  }
  .panel {
    display: flex;
    flex-direction: column;
    gap: 1.2vw;
    padding: 2.5vw 2.5vw 2vw 1vw;
    min-height: 0;
  }
  h1 {
    margin: 0;
    font-size: 3.4vw;
    line-height: 1.05;
  }
  .stats {
    margin: 0;
    font-size: 1.4vw;
    color: var(--muted);
  }
  .hero {
    margin: 0;
    flex: 1;
    min-height: 0;
    position: relative;
  }
  .hero :global(img),
  .hero :global(video) {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
    filter: drop-shadow(0 1.5vw 2vw rgba(0, 0, 0, 0.6));
  }
  figcaption {
    position: absolute;
    bottom: 0.6vw;
    left: 50%;
    transform: translateX(-50%) rotate(-2deg);
    background: var(--yellow);
    color: #1a1000;
    padding: 0.3vw 1.2vw;
    border-radius: 0.5vw;
    font-size: 1.5vw;
    white-space: nowrap;
  }
  .qr {
    display: flex;
    gap: 1.2vw;
    align-items: center;
    background: #fff;
    color: #0b0613;
    border-radius: 1vw;
    padding: 0.8vw 1.2vw 0.8vw 0.8vw;
  }
  .qr img {
    width: 11vw;
    height: 11vw;
    image-rendering: pixelated;
  }
  .qr strong {
    display: block;
    font-size: 2vw;
  }
  .qr span {
    font-size: 1.2vw;
  }
</style>
