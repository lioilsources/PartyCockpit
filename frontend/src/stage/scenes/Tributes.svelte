<script lang="ts">
  import { api, store } from '../../lib/api';
  import type { BeatClock } from '../../lib/beat';
  import { rng } from '../../lib/rand';
  import { DEFAULT_TRIBUTES, SHEET, creditLine, sheetAspect, sheetCols, sheetWidth, tributeOptions, type Tribute } from '../../lib/tributes';
  import ZigZag from '../ZigZag.svelte';

  // Tributy (D24): PromoClown tributes on disk, each one a white sheet like
  // Restyler's photo strip: a 2×2 grid or a 1×4 booth strip, the style under
  // every frame and the credit "Tribute to @artist" at the foot (always).
  // The sheets scroll zig-zag like Pásy.
  //   params.layout    'grid' (default) | 'strip'
  //   params.cols      columns of sheets (grid 2–4, strip 2–8)
  //   params.speed     px per second at 1080p (default 40)
  //   params.labels    style name under each frame (default true)
  //   params.tributes  ids in this order (default: all on disk, newest first)
  //   params.seed      fixes each column's pace (Studio show)
  let {
    params,
    clock,
    startedAt = 0,
    preview = false,
  }: { params: Record<string, any>; clock: BeatClock; startedAt?: number; preview?: boolean } = $props();

  const o = tributeOptions(params);
  const rand = rng(params.seed, 9);
  const pace = Array.from({ length: o.cols }, () => 0.8 + rand() * 0.4);
  const W = sheetWidth(o.layout);
  const cacheKey = 'pc.tributes';

  let all = $state<Tribute[]>([]);
  let loaded = $state(false);
  async function load() {
    try {
      all = await api.get<Tribute[]>('/api/tributes');
      if (!preview) store.set(cacheKey, JSON.stringify(all));
    } catch {
      try {
        all = JSON.parse(store.get(cacheKey) ?? '[]');
      } catch {
        all = [];
      }
    }
    loaded = true;
  }
  load();

  const picked = $derived.by(() => {
    const ok = all.filter((t) => t.frames?.length);
    if (!o.ids.length) return ok.slice(0, DEFAULT_TRIBUTES);
    const byId = new Map(ok.map((t) => [t.id, t]));
    return o.ids.map((id) => byId.get(id)).filter((t): t is Tribute => !!t);
  });

  // a column's px width at mount: the stage does not resize mid-scene
  const colPx = (innerWidth * 0.92 - (o.cols - 1) * innerWidth * 0.02) / o.cols;

  // round-robin; a short column repeats until one copy is 1.2 screens tall
  const columns = $derived.by(() => {
    const out: Tribute[][] = Array.from({ length: o.cols }, () => []);
    picked.forEach((t, i) => out[i % o.cols].push(t));
    return out.map((c) => {
      if (!c.length) return c;
      const h = c.reduce((sum, t) => sum + colPx * (sheetAspect(t, o.layout, o.labels) + 0.047), 0);
      const reps = Math.max(1, Math.ceil((innerHeight * 1.2) / Math.max(1, h)));
      return Array.from({ length: reps }, () => c).flat();
    });
  });

  // small copies unless a cell is wider on screen than the small file
  function src(t: Tribute, i: number) {
    const f = t.frames[i];
    const cellPx = (colPx * (sheetCols(t, o.layout) === 1 ? W - 2 * SHEET.margin : SHEET.cell)) / W;
    const big = !f || cellPx * (devicePixelRatio || 1) > f.sw * 1.1;
    return `/tributes/${t.id}/${i}${big ? '' : 's'}.jpg?v=${t.v}`;
  }
  const ratio = (t: Tribute) => {
    const f = t.frames[0];
    return f && f.h > 0 ? `${f.w} / ${f.h}` : '3 / 4';
  };
</script>

<ZigZag {columns} {pace} speed={o.speed} {clock} {startedAt} key={(t: Tribute) => t.id}>
  {#snippet item(t: Tribute)}
    <article class="sheet" style:--u={`calc(100cqw / ${W})`} style:--ratio={ratio(t)}>
      <div class="cells" class:two={sheetCols(t, o.layout) === 2}>
        {#each t.frames as _, i (i)}
          <figure>
            <img src={src(t, i)} alt="" decoding="async" />
            {#if o.labels}<figcaption>{t.labels[i] ?? ''}</figcaption>{/if}
          </figure>
        {/each}
      </div>
      <footer>{creditLine(t.credit)}</footer>
    </article>
  {/snippet}
  {#snippet empty()}
    {#if preview && loaded}<p class="hint">žádné tributy na disku (Studio → 🎨 Tributy)</p>{/if}
  {/snippet}
</ZigZag>

<style>
  /* Restyler strip.dart in its own pixels (1 u = one of them): cell 512,
     margin 24, label 44; the credit foot is one and a half labels */
  .sheet {
    /* margin, not gap: half the column height is then exactly one period */
    margin: 0 0 4.7cqw;
    background: #fbfaf7;
    padding: calc(24 * var(--u)) calc(24 * var(--u)) 0;
    border-radius: 1.2cqw;
    box-shadow: 0 2cqw 5cqw rgba(0, 0, 0, 0.55);
    transition: transform 0.15s ease-out;
  }
  :global(.hit) .sheet {
    transform: scale(1.015);
    transition: none;
  }
  .cells {
    display: grid;
    grid-template-columns: 1fr;
    gap: calc(24 * var(--u));
  }
  .cells.two {
    grid-template-columns: 1fr 1fr;
  }
  /* an odd last frame in a grid sits centred, one cell wide */
  .cells.two figure:last-child:nth-child(odd) {
    grid-column: 1 / -1;
    justify-self: center;
    width: calc(512 * var(--u));
  }
  figure {
    margin: 0;
  }
  img {
    display: block;
    width: 100%;
    aspect-ratio: var(--ratio);
    object-fit: cover;
    background: #ddd;
  }
  figcaption {
    height: calc(44 * var(--u));
    display: flex;
    align-items: center;
    justify-content: center;
    color: #1d1208;
    font-size: calc(24 * var(--u));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  footer {
    height: calc(66 * var(--u));
    display: flex;
    align-items: center;
    justify-content: center;
    color: #1d1208;
    font-size: calc(30 * var(--u));
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .hint {
    font-size: 1.8vw;
    color: var(--muted);
  }
</style>
