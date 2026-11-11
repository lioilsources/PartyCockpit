<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { BeatClock } from '../../lib/beat';
  import { ShowQueue } from '../../lib/showqueue';
  import { rng } from '../../lib/rand';
  import { fileUrl, type Asset, type Tempo } from '../../lib/types';
  import Clip from '../../lib/Clip.svelte';

  // DanceFloor2D (M4): approved guest dances side by side. Every `beats` beats one
  // dancer is swapped for the next from the queue (new ones first, then loop or
  // shuffle). Video speed follows the tempo within ±10 %, so a 110 bpm clip still
  // looks on the beat at 100 or 120.
  let {
    code,
    queue,
    loopMode,
    params,
    tempo,
    clock,
  }: {
    code: string;
    queue: Asset[];
    loopMode: 'loop' | 'shuffle';
    params: Record<string, any>;
    tempo: Tempo | null;
    clock: BeatClock;
  } = $props();

  const NOMINAL_BPM = 110; // what the dance prompts tend to come out at
  const dances = $derived(queue.filter((a) => a.kind === 'dance' && a.file));
  const slots = $derived(Math.min(4, Math.max(1, dances.length)));
  const beats = $derived(Math.max(4, Number(params.beats) || 16));
  const rate = $derived(tempo && tempo.bpm > 0 ? Math.min(1.1, Math.max(0.9, tempo.bpm / NOMINAL_BPM)) : 1);

  const picker = new ShowQueue<Asset>(rng(params.seed, 7));
  let floor = $state<Asset[]>([]);
  let swap = 0;
  let count = 0;
  let hit = $state(false);

  $effect(() => {
    // fill the floor as dances get approved
    if (floor.length < slots) {
      const next = picker.next(dances.filter((d) => !floor.some((f) => f.id === d.id)), loopMode);
      if (next) floor = [...floor, next];
    }
  });

  const off = clock.on(() => {
    hit = true;
    setTimeout(() => (hit = false), 120);
    if (++count % beats !== 0 || dances.length <= floor.length) return;
    const next = picker.next(dances.filter((d) => !floor.some((f) => f.id === d.id)), loopMode);
    if (!next) return;
    const i = swap++ % floor.length;
    floor = floor.map((f, j) => (j === i ? next : f));
  });
  onDestroy(() => off());
</script>

<div class="floor" class:hit style:--n={floor.length || 1}>
  {#each floor as d (d.id)}
    <figure>
      <Clip src={fileUrl(code, d.file)} {rate} />
      <figcaption>💃 {d.meta.nickname ?? 'host'} <span>{d.meta.motionLabel ?? ''}</span></figcaption>
    </figure>
  {:else}
    <div class="empty">
      <h1>💃 Tančírna</h1>
      <p>V telefonu: Fotka → „Roztanči mě“. Za pár minut tančíš tady.</p>
    </div>
  {/each}
</div>

<style>
  .floor {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: repeat(var(--n), 1fr);
    gap: 1.5vw;
    padding: 4vh 3vw 8vh;
    align-items: end;
    background:
      radial-gradient(ellipse at 50% 110%, rgba(255, 61, 139, 0.45), transparent 60%),
      repeating-linear-gradient(90deg, rgba(255, 255, 255, 0.04) 0 2px, transparent 2px 8vw),
      #0b0613;
    transition: background-color 0.3s;
  }
  .floor.hit {
    background-color: #1c0b2b;
    transition: none;
  }
  figure {
    margin: 0;
    position: relative;
    height: 100%;
    display: grid;
    place-items: end center;
  }
  figure :global(img),
  figure :global(video) {
    max-width: 100%;
    max-height: 84vh;
    border-radius: 1vw;
    box-shadow: 0 2vw 4vw rgba(0, 0, 0, 0.6);
  }
  figcaption {
    position: absolute;
    bottom: -1.2vw;
    left: 50%;
    transform: translateX(-50%) rotate(-2deg);
    background: var(--yellow);
    color: #1a1000;
    padding: 0.4vw 1.4vw;
    border-radius: 0.6vw;
    font-size: 1.7vw;
    white-space: nowrap;
  }
  figcaption span {
    font-size: 1.2vw;
  }
  .empty {
    grid-column: 1 / -1;
    align-self: center;
    text-align: center;
  }
  .empty h1 {
    font-size: 6vw;
    margin: 0;
  }
  .empty p {
    font-size: 2vw;
    color: var(--muted);
  }
</style>
