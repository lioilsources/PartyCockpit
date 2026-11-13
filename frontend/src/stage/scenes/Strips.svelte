<script lang="ts">
  import type { BeatClock } from '../../lib/beat';
  import { rng } from '../../lib/rand';
  import ZigZag from '../ZigZag.svelte';
  import { fileUrl, isVideo, type Asset } from '../../lib/types';

  // Pásy (D23): restyled photos in strips like Restyler's photo strip
  // (white card, label under the picture), the columns scroll zig-zag: one up,
  // the next down. Approved clips (hair in the breeze, the living loop) play in
  // place of the photo while their tile is on screen.
  //   params.cols    2–5 columns (default 3)
  //   params.speed   px per second at 1080p (default 40)
  //   params.labels  nickname · style under each picture (default true)
  //   params.assets  asset ids in this order (default: every photo in the queue)
  //   params.seed    fixes each column's pace (Studio show)
  let {
    code,
    queue,
    params,
    clock,
    startedAt = 0,
    maxClips = 8,
  }: { code: string; queue: Asset[]; params: Record<string, any>; clock: BeatClock; startedAt?: number; maxClips?: number } = $props();

  const cols = Math.min(5, Math.max(2, Number(params.cols) || 3));
  const speed = Math.min(300, Math.max(5, Number(params.speed) || 40));
  const labels = params.labels !== false;
  const pick: string[] = Array.isArray(params.assets) ? params.assets.filter((x: unknown) => typeof x === 'string') : [];
  const rand = rng(params.seed, 2);
  const pace = Array.from({ length: cols }, () => 0.8 + rand() * 0.4);

  const photos = $derived.by(() => {
    const ok = queue.filter((a) => (a.kind === 'restyle' || a.kind === 'upload') && a.file);
    if (!pick.length) return ok;
    const byId = new Map(ok.map((a) => [a.id, a]));
    return pick.map((id) => byId.get(id)).filter((a): a is Asset => !!a);
  });

  // round-robin into columns; a short column repeats so it fills two screens
  const columns = $derived.by(() => {
    const out: Asset[][] = Array.from({ length: cols }, () => []);
    photos.forEach((a, i) => out[i % cols].push(a));
    return out.map((c) => {
      if (!c.length) return c;
      const reps = Math.max(1, Math.ceil(5 / c.length));
      return Array.from({ length: reps }, () => c).flat();
    });
  });

  // clips only for the first few photos that have one (a Pi decodes ~4 videos)
  const clipIds = $derived(new Set(photos.filter((a) => a.live && isVideo(a.live)).slice(0, maxClips).map((a) => a.id)));
  const src = (a: Asset) => fileUrl(code, a.live && (clipIds.has(a.id) || !isVideo(a.live)) ? a.live : a.file);
</script>

<ZigZag {columns} {pace} {speed} {clock} {startedAt} key={(a: Asset) => a.id}>
  {#snippet item(a: Asset)}
    <figure>
      {#if isVideo(src(a))}
        <video src={src(a)} autoplay loop muted playsinline></video>
      {:else}
        <img src={src(a)} alt="" decoding="async" />
      {/if}
      {#if labels}
        <figcaption>{[a.meta.nickname, a.meta.fallback ? '' : a.meta.styleLabel].filter(Boolean).join(' · ')}</figcaption>
      {/if}
    </figure>
  {/snippet}
  {#snippet empty()}
    <h1>Pošli selfie z telefonu</h1>
    <p>naskenuj QR vpravo dole → vyber styl → přidáš se do pásu</p>
  {/snippet}
</ZigZag>

<style>
  /* Restyler strip.dart: cell 512, margin 24, label 44 */
  figure {
    /* margin, not gap: half the column height is then exactly one period */
    margin: 0 0 4.7cqw;
    background: #fbfaf7;
    padding: 4.7cqw 4.7cqw 0;
    border-radius: 1.2cqw;
    box-shadow: 0 2cqw 5cqw rgba(0, 0, 0, 0.55);
    transition: transform 0.15s ease-out;
  }
  :global(.hit) figure {
    transform: scale(1.015);
    transition: none;
  }
  figure img,
  figure video {
    display: block;
    width: 100%;
    aspect-ratio: 3 / 4;
    object-fit: cover;
    background: #ddd;
  }
  figcaption {
    height: 8.6cqw;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #1d1208;
    font-size: 4.6cqw;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  figure:not(:has(figcaption)) {
    padding-bottom: 4.7cqw;
  }
  h1 {
    font-size: 5vw;
    margin: 0;
  }
  p {
    font-size: 1.8vw;
    color: var(--muted);
  }
</style>
