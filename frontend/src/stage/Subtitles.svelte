<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { SubtitleLine } from '../lib/types';

  // Subtitles overlay (M7): the MC's words (and their translation) over any
  // scene. A line updates in place from partial → final → translated, and fades
  // a few seconds after it is done. At most two lines stay on screen.
  const HOLD = 7000;
  let lines = $state<(SubtitleLine & { at: number })[]>([]);
  let now = $state(Date.now());
  const timer = setInterval(() => {
    now = Date.now();
    lines = lines.filter((l) => !l.done || now - l.at < HOLD + 800);
  }, 500);
  onDestroy(() => clearInterval(timer));

  export function add(l: SubtitleLine) {
    const i = lines.findIndex((x) => x.id === l.id);
    const next = { ...l, at: Date.now() };
    if (i >= 0) lines[i] = next;
    else lines = [...lines, next].slice(-2);
  }
  export function clear() {
    lines = [];
  }
</script>

{#if lines.length}
  <div class="subs">
    {#each lines as l (l.id)}
      <div class="line" class:partial={!l.final} class:gone={l.done && now - l.at > HOLD}>
        <p>{l.text}</p>
        {#if l.translation && l.translation !== l.text}<p class="tr">{l.translation}</p>{/if}
      </div>
    {/each}
  </div>
{/if}

<style>
  .subs {
    position: absolute;
    left: 50%;
    bottom: 9vh;
    transform: translateX(-50%);
    width: max-content;
    max-width: min(70vw, 1300px);
    display: grid;
    justify-items: center;
    gap: 0.8vh;
    z-index: 30;
    pointer-events: none;
    text-align: center;
  }
  .line {
    background: rgba(5, 2, 10, 0.78);
    border-radius: 1vw;
    padding: 0.9vh 2vw 1.1vh;
    transition: opacity 0.8s;
    box-shadow: 0 0 3vw rgba(0, 0, 0, 0.5);
  }
  .line.gone {
    opacity: 0;
  }
  p {
    margin: 0;
    font-size: 3vw;
    line-height: 1.2;
    font-weight: 700;
    color: #fff;
  }
  .partial p {
    color: rgba(255, 255, 255, 0.7);
    font-weight: 400;
  }
  .tr {
    font-size: 2.2vw;
    font-weight: 400;
    color: var(--yellow);
  }
</style>
