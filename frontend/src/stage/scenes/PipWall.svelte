<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { onDestroy } from 'svelte';
  import type { BeatClock } from '../../lib/beat';
  import { srcObject, type Viewer } from '../../lib/rtc';
  import type { LiveStream } from '../../lib/types';

  // Live phone cameras (MirrorBooth / browser) in a grid that grows 1 → 2×2 → 3×3.
  // Only operator-approved streams ever reach this list (backend api/streams.go).
  let {
    streams,
    viewer,
    rev,
    clock,
  }: { streams: LiveStream[]; viewer: Viewer; rev: number; clock: BeatClock } = $props();

  const tiles = $derived(streams.filter((s) => s.online).slice(0, 9));
  const cols = $derived(tiles.length <= 1 ? 1 : tiles.length <= 4 ? 2 : 3);
  const rows = $derived(Math.max(1, Math.ceil(tiles.length / cols)));
  // rev bumps whenever a connection's media changes
  const media = $derived.by(() => {
    void rev;
    return new Map(tiles.map((s) => [s.id, viewer.media(s.id)]));
  });

  let beat = $state(0);
  const off = clock.on(() => (beat = (beat + 1) % 9));
  onDestroy(() => off());
</script>

<div class="wall" style:--cols={cols} style:--rows={rows}>
  {#each tiles as s, i (s.id)}
    <figure class:hit={beat % tiles.length === i}>
      {#if media.get(s.id)}
        <video use:srcObject={media.get(s.id)} autoplay playsinline muted></video>
      {:else}
        <div class="wait">{t('stage.pip.wait')}</div>
      {/if}
      <figcaption>{s.source === 'mirrorbooth' ? '🪞' : '📡'} {s.name}</figcaption>
    </figure>
  {:else}
    <div class="empty">
      <h1>{t('stage.pip.title')}</h1>
      <p>{t('stage.pip.hint')}</p>
    </div>
  {/each}
</div>

<style>
  .wall {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: repeat(var(--cols), 1fr);
    grid-template-rows: repeat(var(--rows), 1fr);
    gap: 1.2vh;
    padding: 1.2vh;
    background: radial-gradient(circle at 50% 40%, #2a0f3d, #0b0613 70%);
  }
  figure {
    position: relative;
    margin: 0;
    overflow: hidden;
    border-radius: 18px;
    background: #000;
    box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.08);
    transition: box-shadow 0.35s ease-out;
  }
  figure.hit {
    box-shadow: 0 0 0 5px var(--pink, #ff3d8b), 0 0 40px rgba(255, 61, 139, 0.6);
    transition: none;
  }
  video {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  figcaption {
    position: absolute;
    left: 12px;
    bottom: 10px;
    padding: 4px 12px;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.55);
    font-size: 2.2vh;
    font-weight: 600;
  }
  .wait {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    opacity: 0.6;
    font-size: 2.4vh;
  }
  .empty {
    grid-column: 1 / -1;
    grid-row: 1 / -1;
    display: grid;
    place-content: center;
    text-align: center;
  }
  .empty h1 {
    font-size: 6vw;
    margin: 0 0 1vh;
  }
  .empty p {
    font-size: 2.6vh;
    opacity: 0.8;
  }
</style>
