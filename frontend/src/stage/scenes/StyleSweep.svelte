<script lang="ts">
  import type { BeatClock } from '../../lib/beat';
  import { rng } from '../../lib/rand';
  import SweepTile from './SweepTile.svelte';

  // StyleMap sweep (M7, D23): one pack on the whole screen, or several side by
  // side (2, 2×2, 3×3); every beat jumps all of them ahead.
  //   params.packs  pack ids, up to 9 (params.pack, one id, still works)
  //   params.fps    frames per second
  //   params.seed   fixes where in each tour the tiles start (Studio show)
  // As the stage's scene transition it gets `transition` and one pack.
  let {
    params,
    clock,
    transition = false,
  }: { params: Record<string, any>; clock: BeatClock; transition?: boolean } = $props();

  const list: string[] = Array.isArray(params.packs) ? params.packs.filter((x: unknown) => typeof x === 'string' && x) : [];
  const packs = (list.length ? list : [params.pack || params.transition || '']).slice(0, transition ? 1 : 9);
  const rand = rng(params.seed, 3);
  // seeded: random starts; otherwise staggered so the same pack twice differs
  const starts = packs.map((_, i) => (params.seed !== undefined ? rand() : i / packs.length));
  const cols = packs.length <= 1 ? 1 : packs.length <= 4 ? 2 : 3;
  const rows = Math.ceil(packs.length / cols);
</script>

{#if packs.length <= 1}
  <SweepTile pack={packs[0]} fps={params.fps} start={starts[0]} {clock} {transition} />
{:else}
  <div class="grid" style:--cols={cols} style:--rows={rows}>
    {#each packs as id, i (`${i}:${id}`)}
      <div class="cell"><SweepTile pack={id} fps={params.fps} start={starts[i]} small {clock} /></div>
    {/each}
  </div>
{/if}

<style>
  .grid {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: repeat(var(--cols), 1fr);
    grid-template-rows: repeat(var(--rows), 1fr);
    gap: 0.6vw;
    background: #05020a;
  }
  .cell {
    position: relative;
    overflow: hidden;
  }
</style>
