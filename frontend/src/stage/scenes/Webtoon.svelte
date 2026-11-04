<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../../lib/api';
  import type { Asset, Episode } from '../../lib/types';
  import PartyStrip from './PartyStrip.svelte';

  // Pre-made episodes ("divadelní hry", D7), auto-scrolling strip.
  // params: { episode?: string, speed?: px per second }; episode "party" = the
  // strip of tonight's approved photos and notes (D18).
  let { params, code, queue, title }: { params: Record<string, any>; code: string; queue: Asset[]; title: string } = $props();
  const party = params.episode === 'party';

  let ep = $state<Episode | null>(null);
  let err = $state('');
  let scroller: HTMLDivElement;
  let raf = 0;
  let last = 0;
  let y = 0;
  let pauseUntil = 0;

  $effect(() => {
    if (party) return;
    api
      .get<Episode[]>('/api/webtoon')
      .then((eps) => {
        ep = eps.find((e) => e.id === params.episode) ?? eps[0] ?? null;
        if (!ep) err = 'Žádné epizody v data/webtoon/';
      })
      .catch((e) => (err = e.message));
  });

  function frame(t: number) {
    const dt = last ? (t - last) / 1000 : 0;
    last = t;
    if (scroller && t > pauseUntil) {
      y += (Number(params.speed) || 60) * dt;
      const max = scroller.scrollHeight - scroller.clientHeight;
      if (y >= max + 1) {
        y = 0; // loop after a short pause at the end
        pauseUntil = t + 4000;
      }
      scroller.scrollTop = y;
    }
    raf = requestAnimationFrame(frame);
  }
  raf = requestAnimationFrame(frame);
  onDestroy(() => cancelAnimationFrame(raf));
</script>

<div class="wt" bind:this={scroller}>
  {#if party}
    <PartyStrip {code} {queue} {title} />
    <div class="end">pokračování příště · fotka, vzkaz a jsi v dalším panelu 📸</div>
  {:else if ep}
    <h1>{ep.title}</h1>
    {#each ep.panels as p (p)}<img src={p} alt="" />{/each}
    <div class="end">konec · pošli nám námět na další díl 💡</div>
  {:else}
    <p class="muted">{err || 'načítám…'}</p>
  {/if}
</div>

<style>
  .wt {
    position: absolute;
    inset: 0;
    overflow: hidden;
    background: #111;
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  h1 {
    font-size: 3vw;
    margin: 6vh 0;
  }
  img {
    width: min(46vw, 900px);
    display: block;
  }
  .end {
    padding: 20vh 0 40vh;
    font-size: 2vw;
    color: var(--yellow);
  }
  p {
    margin-top: 40vh;
    font-size: 1.6vw;
  }
</style>
