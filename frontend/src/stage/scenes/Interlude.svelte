<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { BeatClock } from '../../lib/beat';
  import { shaders, shaderById, type Shader } from '../shaders';

  // ShaderInterlude (M7): audio-reactive shaders between show blocks. params:
  //   shader  one id to keep (default: cycle through all)
  //   beats   swap every N beats when cycling (default 32)
  //   text    optional line over the shader (default: party name)
  //   scale   render resolution vs. the screen (default 0.6, the mini PC)
  let { params, name, clock }: { params: Record<string, any>; name: string; clock: BeatClock } = $props();

  const fixed = shaderById(params.shader ?? '');
  const beats = Math.max(4, Number(params.beats) || 32);
  const scale = Math.min(1, Math.max(0.25, Number(params.scale) || 0.6));
  const text = (params.text ?? name ?? '').toString().slice(0, 80);

  let canvas: HTMLCanvasElement;
  let gl: WebGLRenderingContext | null = null;
  let prog: WebGLProgram | null = null;
  let loc: Record<string, WebGLUniformLocation | null> = {};
  let current = $state<Shader>(fixed ?? shaders[Math.floor(Math.random() * shaders.length)]);
  let failed = $state(false);
  let raf = 0;
  let t0 = performance.now();
  let swapAt = t0;
  let beat = 0; // decaying pulse
  let lastBeat = 0;
  let period = 500;
  let energy = 0;
  let count = 0;
  let pop = $state(false);

  function compile(src: string): WebGLProgram | null {
    if (!gl) return null;
    const mk = (type: number, s: string) => {
      const sh = gl!.createShader(type)!;
      gl!.shaderSource(sh, s);
      gl!.compileShader(sh);
      if (!gl!.getShaderParameter(sh, gl!.COMPILE_STATUS)) {
        console.warn('[interlude] shader', gl!.getShaderInfoLog(sh));
        return null;
      }
      return sh;
    };
    const vs = mk(gl.VERTEX_SHADER, 'attribute vec2 p; void main() { gl_Position = vec4(p, 0., 1.); }');
    const fs = mk(gl.FRAGMENT_SHADER, src);
    if (!vs || !fs) return null;
    const pr = gl.createProgram()!;
    gl.attachShader(pr, vs);
    gl.attachShader(pr, fs);
    gl.bindAttribLocation(pr, 0, 'p');
    gl.linkProgram(pr);
    if (!gl.getProgramParameter(pr, gl.LINK_STATUS)) return null;
    return pr;
  }

  function use(s: Shader) {
    const pr = compile(s.src);
    if (!pr || !gl) return false;
    if (prog) gl.deleteProgram(prog);
    prog = pr;
    gl.useProgram(pr);
    loc = Object.fromEntries(['uRes', 'uTime', 'uBeat', 'uEnergy', 'uFade'].map((u) => [u, gl!.getUniformLocation(pr, u)]));
    current = s;
    swapAt = performance.now();
    return true;
  }

  function resize() {
    const w = Math.round(innerWidth * scale);
    const h = Math.round(innerHeight * scale);
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
      gl?.viewport(0, 0, w, h);
    }
  }

  function frame(now: number) {
    raf = requestAnimationFrame(frame);
    if (!gl || !prog) return;
    resize();
    // without audio input, energy follows the beat so the shaders still breathe
    const target = clock.energy > 0 ? clock.energy : Math.max(0, 1 - (now - lastBeat) / (period * 2)) * 0.5;
    energy += (target - energy) * 0.08;
    beat = Math.max(0, 1 - (now - lastBeat) / Math.min(period, 600));
    gl.uniform2f(loc.uRes, canvas.width, canvas.height);
    gl.uniform1f(loc.uTime, (now - t0) / 1000);
    gl.uniform1f(loc.uBeat, beat * beat);
    gl.uniform1f(loc.uEnergy, energy);
    gl.uniform1f(loc.uFade, Math.min(1, (now - swapAt) / 1200));
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }

  const off = clock.on((b) => {
    const now = performance.now();
    if (lastBeat) period = Math.min(1500, Math.max(250, now - lastBeat));
    lastBeat = now;
    pop = true;
    setTimeout(() => (pop = false), 140);
    if (!fixed && ++count % beats === 0) {
      const i = shaders.indexOf(current);
      use(shaders[(i + 1) % shaders.length]);
    }
  });

  onMount(() => {
    gl = canvas.getContext('webgl', { antialias: false, alpha: false, powerPreference: 'high-performance' });
    if (!gl) {
      failed = true;
      return;
    }
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    if (!use(current) && !shaders.some((s) => use(s))) failed = true;
    raf = requestAnimationFrame(frame);
  });

  onDestroy(() => {
    off();
    cancelAnimationFrame(raf);
    gl?.getExtension('WEBGL_lose_context')?.loseContext();
  });
</script>

<div class="interlude" class:failed>
  <canvas bind:this={canvas}></canvas>
  {#if text}<h1 class:pop>{text}</h1>{/if}
</div>

<style>
  .interlude {
    position: absolute;
    inset: 0;
    background: #05020a;
  }
  .interlude.failed {
    background: radial-gradient(circle at 50% 60%, #ff3d8b55, transparent 60%), #05020a;
  }
  canvas {
    width: 100%;
    height: 100%;
    display: block;
    image-rendering: auto;
  }
  h1 {
    position: absolute;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%) scale(1);
    margin: 0;
    font-size: 7vw;
    letter-spacing: 0.04em;
    color: #fff;
    text-shadow: 0 0 2vw rgba(0, 0, 0, 0.7), 0 0 4vw rgba(255, 61, 139, 0.6);
    mix-blend-mode: screen;
    white-space: nowrap;
    transition: transform 0.25s ease-out;
  }
  h1.pop {
    transform: translate(-50%, -50%) scale(1.04);
    transition: none;
  }
</style>
