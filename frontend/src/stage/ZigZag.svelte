<script lang="ts" generics="T">
  import { onDestroy, type Snippet } from 'svelte';
  import type { BeatClock } from '../lib/beat';
  import { elapsed } from '../lib/rand';

  // Columns that scroll zig-zag (D23): one up, the next down. Shared by Pásy
  // and Tributy (D24). The position is a function of the time since the scene
  // started, so a reloaded stage and the Studio preview land on the same spot.
  // Each column is rendered twice for a seamless loop: half its height is one
  // period, so items must space themselves with margin-bottom, not gap.
  let {
    columns,
    pace,
    speed,
    clock,
    startedAt = 0,
    key,
    item,
    empty,
  }: {
    columns: T[][];
    pace: number[];
    speed: number;
    clock: BeatClock;
    startedAt?: number;
    key: (x: T) => string;
    item: Snippet<[T]>;
    empty?: Snippet;
  } = $props();

  const any = $derived(columns.some((c) => c.length > 0));

  let inners: HTMLDivElement[] = $state([]);
  let raf = 0;
  function frame() {
    const t = elapsed(startedAt);
    const v = innerHeight / 1080;
    inners.forEach((el, i) => {
      if (!el) return;
      const h = el.offsetHeight / 2;
      if (h <= 0) return;
      const y = (t * speed * v * (pace[i] ?? 1)) % h;
      el.style.transform = `translate3d(0, ${i % 2 ? y - h : -y}px, 0)`;
    });
    raf = requestAnimationFrame(frame);
  }
  raf = requestAnimationFrame(frame);

  let hit = $state(false);
  const off = clock.on(() => {
    hit = true;
    setTimeout(() => (hit = false), 120);
  });
  onDestroy(() => {
    cancelAnimationFrame(raf);
    off();
  });
</script>

<div class="wrap" class:hit style:--cols={columns.length}>
  {#if any}
    {#each columns as col, i (i)}
      <div class="col">
        <div class="inner" bind:this={inners[i]}>
          {#each [0, 1] as copy (copy)}
            {#each col as x, j (`${copy}:${j}:${key(x)}`)}
              {@render item(x)}
            {/each}
          {/each}
        </div>
      </div>
    {/each}
  {:else if empty}
    <div class="empty">{@render empty()}</div>
  {/if}
</div>

<style>
  .wrap {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at 50% 30%, #1d1030, #07040c 75%);
    display: grid;
    grid-template-columns: repeat(var(--cols), 1fr);
    gap: 2vw;
    padding: 0 4vw;
    overflow: hidden;
  }
  .col {
    overflow: hidden;
    height: 100%;
    container-type: inline-size;
  }
  .inner {
    display: flex;
    flex-direction: column;
    will-change: transform;
  }
  .empty {
    grid-column: 1 / -1;
    place-self: center;
    text-align: center;
  }
</style>
