<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ShowQueue } from '../../lib/showqueue';
  import type { BeatClock } from '../../lib/beat';
  import { fileUrl, type Asset } from '../../lib/types';

  // Guest Gallery: approved restyles from the show queue, one hero + a strip of
  // the latest. Runs through the queue, then loops/shuffles (D5).
  let {
    code,
    queue,
    loopMode,
    params,
    clock,
  }: { code: string; queue: Asset[]; loopMode: 'loop' | 'shuffle'; params: Record<string, any>; clock: BeatClock } = $props();

  const photos = $derived(queue.filter((a) => a.kind !== 'sticker' && a.file));
  const beats = $derived(Math.max(2, Number(params.beats) || 8));
  const picker = new ShowQueue<Asset>();
  let hero = $state<Asset | null>(null);
  let heroKey = $state(0);
  let count = 0;

  function advance() {
    const n = picker.next(photos, loopMode);
    if (n) {
      hero = n;
      heroKey++;
    }
  }

  $effect(() => {
    // first item as soon as something is approved
    if (!hero && photos.length) advance();
  });

  const off = clock.on(() => {
    if (++count % beats === 0) advance();
  });
  onDestroy(() => off());
</script>

<div class="wrap">
  {#if hero}
    {#key heroKey}
      <figure class="hero">
        <img src={fileUrl(code, hero.file)} alt="" />
        <figcaption>
          <strong>{hero.meta.nickname ?? 'host'}</strong>
          {#if hero.meta.styleLabel && !hero.meta.fallback}<span>jako {hero.meta.styleLabel}</span>{/if}
        </figcaption>
      </figure>
    {/key}
    <div class="strip">
      {#each photos.slice(-8).reverse() as a (a.id)}
        <img class:current={a.id === hero.id} src={fileUrl(code, a.file)} alt="" />
      {/each}
    </div>
  {:else}
    <div class="empty">
      <h1>Pošli selfie z telefonu</h1>
      <p>naskenuj QR vpravo dole → vyber styl → objevíš se tady</p>
    </div>
  {/if}
</div>

<style>
  .wrap {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at 30% 20%, #2a1240, #0b0613 70%);
    display: grid;
    place-items: center;
  }
  .hero {
    margin: 0;
    position: relative;
    height: 78vh;
    animation: pop 0.9s cubic-bezier(0.2, 1.4, 0.4, 1) both;
  }
  .hero img {
    height: 100%;
    border-radius: 1.4vw;
    box-shadow: 0 2vw 6vw rgba(0, 0, 0, 0.6), 0 0 0 0.4vw rgba(255, 255, 255, 0.08);
  }
  figcaption {
    position: absolute;
    left: 50%;
    bottom: -1.6vw;
    transform: translateX(-50%) rotate(-2deg);
    background: var(--yellow);
    color: #1a1000;
    padding: 0.5vw 1.6vw;
    border-radius: 0.6vw;
    white-space: nowrap;
    font-size: 1.8vw;
  }
  figcaption span {
    font-size: 1.3vw;
    margin-left: 0.6vw;
  }
  .strip {
    position: absolute;
    top: 2vw;
    right: 2vw;
    display: flex;
    flex-direction: column;
    gap: 0.6vw;
  }
  .strip img {
    width: 5vw;
    height: 5vw;
    object-fit: cover;
    border-radius: 0.6vw;
    opacity: 0.55;
  }
  .strip img.current {
    opacity: 1;
    outline: 0.25vw solid var(--pink);
  }
  .empty {
    text-align: center;
  }
  .empty h1 {
    font-size: 5vw;
    margin: 0;
  }
  .empty p {
    font-size: 1.8vw;
    color: var(--muted);
  }
  @keyframes pop {
    from {
      transform: scale(0.6) rotate(-6deg);
      opacity: 0;
    }
  }
</style>
