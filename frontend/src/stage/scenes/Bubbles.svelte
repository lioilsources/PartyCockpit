<script lang="ts">
  import type { WtBubble, WtSfx } from '../../lib/types';

  // Speech bubbles over a webtoon panel (D23). The pack images carry no text,
  // the Toonfork app draws it; this follows its placement rules
  // (Webtoon app/lib/reader/bubble_layout.dart): narration boxes stacked at the
  // top left, talk bubbles zig-zag down the panel (left, right, …; the first one
  // on the right when there is narration), tails toward the middle. Sizes are in
  // cqw of the strip column, so text scales with the panel.
  let { bubbles, sfx, lang }: { bubbles: WtBubble[]; sfx: WtSfx[]; lang: string } = $props();

  const pick = (t: Record<string, string>) => t[lang] ?? t.cs ?? t.en ?? Object.values(t)[0] ?? '';
  const narration = $derived(bubbles.filter((b) => b.kind === 'narration'));
  const talk = $derived(bubbles.filter((b) => b.kind !== 'narration'));
  const side = (b: WtBubble, i: number) => (b.anchor ? (b.anchor.x < 0.5 ? 'left' : 'right') : (i + (narration.length ? 1 : 0)) % 2 ? 'right' : 'left');
</script>

{#if bubbles.length || sfx.length}
  <div class="layer">
    {#each narration as b}<div class="narration">{pick(b.text)}</div>{/each}
    {#each talk as b, i}<div class={`bubble ${b.kind} ${side(b, i)}`}>{pick(b.text)}</div>{/each}
    {#each sfx as s, i}
      <div class="sfx" class:shaky={s.style === 'shaky'} style:left={`${(s.pos?.x ?? 0.62) * 100}%`} style:top={`${(s.pos?.y ?? 0.72 - i * 0.12) * 100}%`}>{s.text}</div>
    {/each}
  </div>
{/if}

<style>
  .layer {
    position: absolute;
    inset: 0;
    padding: 2.5cqw;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 1.8cqw;
    pointer-events: none;
    font-size: 3.6cqw;
    line-height: 1.25;
    color: #141414;
  }
  .narration {
    max-width: 80%;
    background: #fff7d1;
    border: 0.35cqw solid #141414;
    padding: 1.2cqw 1.8cqw;
    font-style: italic;
  }
  .bubble {
    position: relative;
    max-width: 62%;
    background: #fff;
    border: 0.35cqw solid #141414;
    border-radius: 4cqw;
    padding: 1.6cqw 2.6cqw;
    font-weight: 700;
  }
  .bubble.right {
    align-self: flex-end;
    margin-top: 2.5cqw;
  }
  /* the tail points down toward the panel's middle */
  .bubble::after {
    content: '';
    position: absolute;
    bottom: -2.6cqw;
    border: 1.3cqw solid transparent;
    border-top: 2.6cqw solid #141414;
    border-bottom: 0;
  }
  .bubble.left::after {
    right: 18%;
    transform: skewX(-25deg);
  }
  .bubble.right::after {
    left: 18%;
    transform: skewX(25deg);
  }
  .bubble.thought {
    border-radius: 50% / 40%;
    font-weight: 400;
    font-style: italic;
  }
  .bubble.thought::after {
    width: 2cqw;
    height: 2cqw;
    border: 0.35cqw solid #141414;
    border-radius: 50%;
    background: #fff;
    bottom: -3.4cqw;
    transform: none;
  }
  .bubble.shout {
    border-radius: 1cqw;
    border-width: 0.6cqw;
    font-size: 4.2cqw;
    text-transform: uppercase;
    transform: rotate(-1.5deg);
  }
  .sfx {
    position: absolute;
    transform: translate(-50%, -50%) rotate(-8deg);
    font-size: 9cqw;
    font-weight: 900;
    color: #ffd84d;
    -webkit-text-stroke: 0.5cqw #141414;
    letter-spacing: 0.04em;
    white-space: nowrap;
  }
  .sfx.shaky {
    transform: translate(-50%, -50%) rotate(6deg) skewX(-8deg);
  }
</style>
