<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { onDestroy } from 'svelte';
  import { ShowQueue } from '../../lib/showqueue';
  import type { BeatClock } from '../../lib/beat';
  import type { Asset } from '../../lib/types';
  import Handwritten from '../../lib/Handwritten.svelte';

  // Sticker Wall: guests' messages as handwritten notes. With a glyph sheet
  // (meta.sheet, D16) the note is drawn from HandWrittenStickers letters, else in
  // the Caveat font (D11).
  let { queue, loopMode, clock }: { queue: Asset[]; loopMode: 'loop' | 'shuffle'; clock: BeatClock } = $props();

  // note content box ≈ 11.6vw wide, letters ≈ 3.2vw tall; drawn 2× for the focus zoom
  const vw = innerWidth / 100;

  const notes = $derived(queue.filter((a) => a.kind === 'sticker').slice(-24));
  const picker = new ShowQueue<Asset>();
  let focus = $state<string>('');
  let count = 0;

  // deterministic layout per id, so notes don't jump when the list changes
  function layout(id: string, i: number) {
    let h = 0;
    for (const c of id) h = (h * 31 + c.charCodeAt(0)) | 0;
    const cols = 6;
    const x = (i % cols) * (100 / cols) + 2 + (Math.abs(h) % 5);
    const y = Math.floor(i / cols) * 23 + 4 + (Math.abs(h >> 4) % 6);
    const r = ((Math.abs(h >> 8) % 15) - 7) * 1;
    return `left:${x}vw;top:${y}vh;--r:${r}deg`;
  }

  const off = clock.on(() => {
    if (++count % 6 === 0) focus = picker.next(notes, loopMode)?.id ?? '';
  });
  onDestroy(() => off());
</script>

<div class="wall">
  {#each notes as n, i (n.id)}
    <div class="note" class:focus={focus === n.id} style={layout(n.id, i)} style:background={n.meta.color ?? '#ffd84d'}>
      {#if n.meta.sheet}
        <Handwritten text={n.meta.text ?? ''} sheet={n.meta.sheet} cell={3.2 * vw} width={11.6 * vw} density={2.2} />
      {:else}
        <p>{n.meta.text}</p>
      {/if}
      <span>— {n.meta.nickname}</span>
    </div>
  {:else}
    <div class="empty">
      <h1>{t('stage.stickers.title')}</h1>
    </div>
  {/each}
</div>

<style>
  .wall {
    position: absolute;
    inset: 0;
    background: #2b1d14 repeating-linear-gradient(90deg, rgba(255, 255, 255, 0.025) 0 2px, transparent 2px 60px);
    overflow: hidden;
  }
  .note {
    position: absolute;
    width: 14vw;
    min-height: 10vw;
    padding: 1.1vw 1.2vw 0.8vw;
    color: #1d1208;
    font-family: var(--hand);
    transform: rotate(var(--r));
    box-shadow: 0 0.6vw 1.4vw rgba(0, 0, 0, 0.45);
    transition: transform 0.6s cubic-bezier(0.2, 1.4, 0.4, 1), z-index 0s;
    animation: drop 0.8s cubic-bezier(0.2, 1.4, 0.4, 1) both;
  }
  .note.focus {
    transform: rotate(0) scale(2.1);
    z-index: 10;
  }
  p {
    margin: 0;
    font-size: 1.7vw;
    line-height: 1.05;
    word-break: break-word;
  }
  span {
    display: block;
    text-align: right;
    font-size: 1.2vw;
    margin-top: 0.4vw;
    opacity: 0.75;
  }
  .empty {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    font-family: var(--hand);
    font-size: 2.4vw;
    color: var(--yellow);
  }
  @keyframes drop {
    from {
      transform: translateY(-60vh) rotate(30deg);
    }
  }
</style>
