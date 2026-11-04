<script lang="ts">
  import { onDestroy } from 'svelte';

  // Game controller (D17): drag across the pad, the paddle follows. Sends the
  // position 0..1 at most 25× a second (the party socket allows 40 messages/s).
  let { color, send }: { color: string; send: (x: number) => void } = $props();

  let x = $state(0.5);
  let pad: HTMLDivElement;
  let dirty = false;
  const timer = setInterval(() => {
    if (dirty) {
      dirty = false;
      send(x);
    }
  }, 40);
  onDestroy(() => clearInterval(timer));

  function move(e: PointerEvent) {
    if (e.type !== 'pointerdown' && !e.buttons && e.pointerType === 'mouse') return;
    const r = pad.getBoundingClientRect();
    x = Math.min(1, Math.max(0, (e.clientX - r.left) / r.width));
    dirty = true;
  }
</script>

<div
  class="pad"
  bind:this={pad}
  style:--c={color}
  onpointerdown={(e) => {
    pad.setPointerCapture(e.pointerId);
    move(e);
  }}
  onpointermove={move}
  role="slider"
  aria-valuemin={0}
  aria-valuemax={100}
  aria-valuenow={Math.round(x * 100)}
  tabindex="0"
>
  <div class="knob" style:left={`${x * 100}%`}></div>
  <span>⟵ táhni ⟶</span>
</div>

<style>
  .pad {
    position: relative;
    height: 46vh;
    min-height: 220px;
    border-radius: 18px;
    background: linear-gradient(90deg, rgba(255, 255, 255, 0.04), rgba(255, 255, 255, 0.1), rgba(255, 255, 255, 0.04));
    border: 2px solid var(--c);
    touch-action: none;
    user-select: none;
    overflow: hidden;
  }
  .knob {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 22%;
    transform: translateX(-50%);
    background: var(--c);
    opacity: 0.85;
    border-radius: 14px;
    box-shadow: 0 0 30px var(--c);
  }
  span {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 12px;
    text-align: center;
    color: var(--muted);
    pointer-events: none;
  }
</style>
