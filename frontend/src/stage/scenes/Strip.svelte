<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, store } from '../../lib/api';
  import { elapsed } from '../../lib/rand';
  import type { Asset, EpisodeDetail } from '../../lib/types';
  import Bubbles from './Bubbles.svelte';
  import PartyStrip from './PartyStrip.svelte';

  // One auto-scrolling webtoon column. The position is a function of the time
  // since the scene started, so a reload (or the Studio preview) lands on the
  // same panel, and a saved show scrolls the same way every time (D23).
  let {
    id,
    speed,
    delay,
    lang,
    loops,
    startedAt,
    code,
    queue,
    title,
  }: { id: string; speed: number; delay: number; lang: string; loops: boolean; startedAt: number; code: string; queue: Asset[]; title: string } = $props();

  const party = id === 'party';
  const cacheKey = `pc.webtoon.${id}`;
  let ep = $state<EpisodeDetail | null>(null);
  let err = $state('');
  let col: HTMLDivElement;
  let inner: HTMLDivElement;
  let raf = 0;

  if (!party) {
    try {
      ep = JSON.parse(store.get(cacheKey) ?? 'null');
    } catch {
      ep = null;
    }
    api
      .get<EpisodeDetail>(`/api/webtoon/${encodeURIComponent(id)}`)
      .then((e) => {
        ep = e;
        store.set(cacheKey, JSON.stringify(e));
      })
      .catch((e) => {
        if (!ep) err = e.message;
      });
  }

  const HOLD = 1.5; // seconds still at the top
  const PAUSE = 4; // seconds on the last panel before it starts over
  function frame() {
    if (col && inner) {
      const max = Math.max(0, inner.offsetHeight - col.clientHeight);
      const v = (speed * innerHeight) / 1080;
      const cycle = HOLD + max / v + PAUSE;
      const t = max > 0 ? Math.max(0, elapsed(startedAt) - delay) % cycle : 0;
      const y = Math.min(max, Math.max(0, (t - HOLD) * v));
      inner.style.transform = `translate3d(0, ${-y}px, 0)`;
    }
    raf = requestAnimationFrame(frame);
  }
  raf = requestAnimationFrame(frame);
  onDestroy(() => cancelAnimationFrame(raf));

  const gap = (g?: string) => (g === 'small' ? '2cqw' : g === 'large' ? '16cqw' : '7cqw');
</script>

<div class="col" bind:this={col}>
  <div class="inner" bind:this={inner}>
    {#if party}
      <PartyStrip {code} {queue} {title} />
      <div class="end">pokračování příště · fotka, vzkaz a jsi v dalším panelu 📸</div>
    {:else if ep}
      <header>
        <h1>{ep.story || ep.title}</h1>
        {#if ep.story}<p>{ep.index ? `${ep.index}. ` : ''}{ep.title}</p>{/if}
      </header>
      {#each ep.panels as p (p.image)}
        <figure style:aspect-ratio={p.w && p.h ? `${p.w} / ${p.h}` : undefined} style:margin-bottom={gap(p.gap)}>
          <img src={loops && p.loop ? p.loop : p.image} alt="" decoding="async" />
          <Bubbles bubbles={p.bubbles ?? []} sfx={p.sfx ?? []} {lang} />
        </figure>
      {/each}
      <div class="end">konec · pošli nám námět na další díl 💡</div>
    {:else}
      <p class="muted">{err || 'načítám…'}</p>
    {/if}
  </div>
</div>

<style>
  .col {
    position: relative;
    height: 100%;
    overflow: hidden;
    container-type: inline-size;
  }
  .inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    will-change: transform;
  }
  header {
    text-align: center;
    margin: 8cqw 0 6cqw;
  }
  h1 {
    font-size: 6.5cqw;
    margin: 0;
  }
  header p {
    margin: 1cqw 0 0;
    font-size: 4cqw;
    color: var(--muted);
  }
  figure {
    position: relative;
    width: 100%;
    margin: 0;
  }
  figure img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .end {
    padding: 20cqw 0 40cqw;
    font-size: 4cqw;
    color: var(--yellow);
    text-align: center;
  }
  /* the party episode (D18) was laid out for one wide column */
  .col :global(.panel) {
    max-width: 100%;
  }
  .col :global(h1),
  .col :global(.sub) {
    text-align: center;
  }
  p.muted {
    margin-top: 40vh;
    font-size: 1.6vw;
  }
</style>
