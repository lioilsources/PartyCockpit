<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { BeatClock } from '../../lib/beat';
  import { listPacks, loadPack, preload, type Pack } from '../../lib/stylemap';

  // One StyleMap sweep (M7): one subject morphing through hundreds of styles,
  // played along the pack's `tour`. As a scene it loops; as a transition
  // (`transition`) it plays a short stretch from a random point and the stage
  // fades it out. StyleSweep lays out one or more of these (D23).
  //   pack   pack id (default: the first one in the index)
  //   fps    frames per second (default 12; a beat jumps 3 frames ahead)
  //   start  0–1, where in the tour to begin (default 0; random for a transition)
  //   small  a tile in a grid: smaller label
  let {
    pack: packId = '',
    fps: fpsIn = 0,
    start = 0,
    small = false,
    clock,
    transition = false,
  }: { pack?: string; fps?: number; start?: number; small?: boolean; clock: BeatClock; transition?: boolean } = $props();

  const fps = Math.min(30, Math.max(2, Number(fpsIn) || (transition ? 16 : 12)));
  let pack = $state<Pack | null>(null);
  let failed = $state('');
  let idx = $state(0);
  let hit = $state(false);
  let timer = 0;

  (async () => {
    try {
      const id = packId || (await listPacks())[0]?.id;
      if (!id) throw new Error('žádný StyleMap pack');
      const p = await loadPack(id);
      if (!p.frames.length) throw new Error('prázdný pack');
      idx = Math.floor((transition ? Math.random() : Math.min(0.999, Math.max(0, start))) * p.frames.length);
      preload(p.frames, idx, 24);
      pack = p;
      timer = window.setInterval(() => step(1), 1000 / fps);
    } catch (e: any) {
      failed = e?.message ?? String(e);
    }
  })();

  function step(n: number) {
    if (!pack) return;
    idx = (idx + n) % pack.frames.length;
    preload(pack.frames, idx + 20, n); // keep ~1.5 s of frames warm
  }

  const off = clock.on(() => {
    step(3);
    hit = true;
    setTimeout(() => (hit = false), 110);
  });
  onDestroy(() => {
    off();
    clearInterval(timer);
  });

  const frame = $derived(pack ? pack.frames[idx] : null);
</script>

<div class="sweep" class:transition class:hit class:small>
  {#if pack && frame}
    <div class="atlas" style:background-image={`url(${pack.atlas})`}></div>
    <img class="frame" src={frame.src} alt="" style:aspect-ratio={pack.aspect} />
    {#if !transition}
      <div class="label">
        <strong>{pack.title}</strong>
        <span>{frame.label}</span>
      </div>
    {/if}
  {:else if failed && !transition}
    <div class="fail">StyleMap: {failed}</div>
  {/if}
</div>

<style>
  .sweep {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background: #05020a;
    overflow: hidden;
  }
  .atlas {
    position: absolute;
    inset: -5%;
    background-size: cover;
    background-position: center;
    filter: blur(2vw) brightness(0.35) saturate(1.3);
  }
  .frame {
    position: relative;
    height: 86vh;
    max-width: 92vw;
    object-fit: cover;
    border-radius: 1.2vw;
    box-shadow: 0 2vw 5vw rgba(0, 0, 0, 0.7);
    transition: transform 0.15s ease-out;
  }
  .transition .frame {
    height: 100vh;
    max-width: 100vw;
    border-radius: 0;
  }
  .hit .frame {
    transform: scale(1.015);
    transition: none;
  }
  .label {
    position: absolute;
    left: 3vw;
    bottom: 3vw;
    display: grid;
    gap: 0.3vw;
    background: rgba(0, 0, 0, 0.55);
    padding: 0.8vw 1.4vw;
    border-radius: 0.8vw;
  }
  .label strong {
    font-size: 1.8vw;
  }
  .label span {
    font-size: 1.3vw;
    color: var(--yellow);
  }
  .small .frame {
    height: 92%;
    max-width: 94%;
    border-radius: 0.6vw;
  }
  .small .label {
    left: 1vw;
    bottom: 1vw;
    padding: 0.4vw 0.8vw;
  }
  .small .label strong {
    font-size: 1.1vw;
  }
  .small .label span {
    font-size: 0.9vw;
  }
  .fail {
    color: var(--muted);
    font-size: 2vw;
  }
</style>
