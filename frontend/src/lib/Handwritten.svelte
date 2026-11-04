<script lang="ts">
  import { renderHandwriting } from './handwriting';

  // Text in a HandWrittenStickers sheet; shows the plain text (in the hand font)
  // until the glyphs are drawn, and keeps it when the sheet is unavailable.
  let {
    text,
    sheet,
    cell = 48,
    width = 320,
    ink = '#1d1208',
    density = 1,
  }: { text: string; sheet: string; cell?: number; width?: number; ink?: string; density?: number } = $props();
  // drawn at `density`× the layout size (a focused note is scaled up), shown at 1×
  let shown = $state(0);

  let src = $state<string | null>(null);
  $effect(() => {
    src = null;
    const want = [text, sheet, cell, width, ink];
    renderHandwriting(text, sheet, cell * density, width * density, ink).then((d) => {
      if (want[0] === text && want[1] === sheet) src = d;
    });
  });
</script>

{#if src}
  <img {src} alt={text} style:width={shown ? `${shown}px` : undefined} onload={(e) => (shown = (e.currentTarget as HTMLImageElement).naturalWidth / density)} />
{:else}
  <p>{text}</p>
{/if}

<style>
  img {
    display: block;
    max-width: 100%;
    height: auto;
  }
  p {
    margin: 0;
    font-family: var(--hand);
    font-size: 1.2em;
    line-height: 1.05;
    word-break: break-word;
  }
</style>
