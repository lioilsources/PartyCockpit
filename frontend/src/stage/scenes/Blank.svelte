<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { BeatClock } from '../../lib/beat';

  // Interlude: party name over a beat-pulsing gradient (no GPU job).
  let { name, clock }: { name: string; clock: BeatClock } = $props();
  let hue = $state(300);
  let kick = $state(false);
  const off = clock.on(() => {
    hue = (hue + 23) % 360;
    kick = true;
    setTimeout(() => (kick = false), 100);
  });
  onDestroy(() => off());
</script>

<div class="b" style:--h={hue} class:kick>
  <h1>{name}</h1>
</div>

<style>
  .b {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background: radial-gradient(circle at 50% 60%, hsl(var(--h) 90% 35%), #0b0613 65%);
    transition: background 1.2s;
  }
  h1 {
    font-size: 8vw;
    letter-spacing: -0.04em;
    margin: 0;
    transition: transform 0.3s ease-out;
  }
  .kick h1 {
    transform: scale(1.05);
    transition: none;
  }
</style>
