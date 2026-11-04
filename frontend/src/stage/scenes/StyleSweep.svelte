<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { BeatClock } from '../../lib/beat';
  import { listPacks, loadPack, preload, type Pack } from '../../lib/stylemap';

  // StyleMap sweep (M7): one subject morphing through hundreds of styles, played
  // along the pack's `tour`. As a scene it loops; as a transition (`transition`)
  // it plays a short stretch from a random point and the stage fades it out.
  //   params.pack  pack id (default: the first one in the index)
  //   params.fps   frames per second (default 12; a beat jumps 3 frames ahead)
  let {
    params,
    clock,
    transition = false,
  }: { params: Record<string, any>; clock: BeatClock; transition?: boolean } = $props();

  const fps = Math.min(30, Math.max(2, Number(params.fps) || (transition ? 16 : 12)));
  let pack = $state<Pack | null>(null);
  let failed = $state('');
  let idx = $state(0);
  let hit = $state(false);
  let timer = 0;

  (async () => {
    try {
      const id = params.pack || params.transition || (await listPacks())[0]?.id;
      if (!id) throw new Error('žádný StyleMap pack');
      const p = await loadPack(id);
      if (!p.frames.length) throw new Error('prázdný pack');
      idx = transition ? Math.floor(Math.random() * p.frames.length) : 0;
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

<div class="sweep" class:transition class:hit>
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
  .fail {
    color: var(--muted);
    font-size: 2vw;
  }
</style>
