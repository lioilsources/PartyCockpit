<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Application, Text, TextStyle } from 'pixi.js';

  // Emoji from guests float up across the stage — a PixiJS layer over any scene.
  let host: HTMLDivElement;
  let app: Application | null = null;
  type Floater = { t: Text; vy: number; phase: number; life: number };
  const floaters: Floater[] = [];
  const pending: string[] = [];

  export function add(emoji: string) {
    if (!app) {
      pending.push(emoji);
      return;
    }
    if (floaters.length > 80) return;
    const size = 48 + Math.random() * 40;
    const t = new Text({ text: emoji, style: new TextStyle({ fontSize: size }) });
    t.anchor.set(0.5);
    t.x = app.screen.width * (0.08 + Math.random() * 0.84);
    t.y = app.screen.height + size;
    app.stage.addChild(t);
    floaters.push({ t, vy: 2 + Math.random() * 2.5, phase: Math.random() * Math.PI * 2, life: 0 });
  }

  export function clear() {
    for (const f of floaters) f.t.destroy();
    floaters.length = 0;
  }

  onMount(async () => {
    const a = new Application();
    await a.init({ backgroundAlpha: 0, resizeTo: host, antialias: true, autoDensity: true, resolution: Math.min(2, devicePixelRatio) });
    host.appendChild(a.canvas);
    app = a;
    a.ticker.add((tk) => {
      for (let i = floaters.length - 1; i >= 0; i--) {
        const f = floaters[i];
        f.life += tk.deltaTime;
        f.t.y -= f.vy * tk.deltaTime;
        f.t.x += Math.sin(f.life / 18 + f.phase) * 0.9;
        f.t.rotation = Math.sin(f.life / 25 + f.phase) * 0.25;
        f.t.alpha = Math.min(1, (f.t.y / a.screen.height) * 1.6);
        if (f.t.y < -100) {
          f.t.destroy();
          floaters.splice(i, 1);
        }
      }
    });
    pending.splice(0).forEach(add);
  });

  onDestroy(() => {
    app?.destroy(true);
    app = null;
  });
</script>

<div class="layer" bind:this={host}></div>

<style>
  .layer {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 15;
  }
  .layer :global(canvas) {
    width: 100%;
    height: 100%;
  }
</style>
