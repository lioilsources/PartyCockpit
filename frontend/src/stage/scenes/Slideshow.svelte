<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { onDestroy } from 'svelte';
  import { api, store } from '../../lib/api';
  import { shuffle } from '../../lib/showqueue';
  import type { BeatClock } from '../../lib/beat';
  import type { GalleryImage } from '../../lib/types';

  // FineTuneGallery slideshow; switches every `beats` beats (≈ 8 s idle), Ken Burns.
  let { params, clock }: { params: Record<string, any>; clock: BeatClock } = $props();

  const beats = $derived(Math.max(2, Number(params.beats) || 8));
  let images = $state<GalleryImage[]>([]);
  let slides = $state<{ key: number; url: string; dir: number }[]>([]);
  let idx = 0;
  let count = 0;
  let key = 0;
  let err = $state('');

  async function load() {
    const q = new URLSearchParams({ limit: '150' });
    if (params.style) q.set('style', params.style);
    if (params.score) q.set('score', String(params.score));
    const ck = `pc.gallery.${q}`;
    try {
      images = shuffle(await api.get<GalleryImage[]>(`/api/gallery?${q}`));
      store.set(ck, JSON.stringify(images));
      err = images.length ? '' : t('stage.galleryEmpty');
    } catch (e: any) {
      images = JSON.parse(store.get(ck) ?? '[]');
      err = images.length ? '' : e.message;
    }
    advance();
  }

  function preload(url: string) {
    return new Promise<void>((res) => {
      const i = new Image();
      i.onload = i.onerror = () => res();
      i.src = url;
    });
  }

  async function advance() {
    if (!images.length) return;
    const img = images[idx++ % images.length];
    await preload(img.url);
    slides = [...slides.slice(-1), { key: ++key, url: img.url, dir: key % 4 }];
    preload(images[idx % images.length].url);
  }

  $effect(() => {
    load();
  });
  const off = clock.on(() => {
    if (++count % beats === 0) advance();
  });
  onDestroy(() => off());
</script>

<div class="wrap">
  {#each slides as s (s.key)}
    <div class="slide" style:--bg={`url(${s.url})`}>
      <img class={`kb kb${s.dir}`} src={s.url} alt="" />
    </div>
  {/each}
  {#if err}<p class="err">{err}</p>{/if}
</div>

<style>
  .wrap {
    position: absolute;
    inset: 0;
    background: #000;
  }
  .slide {
    position: absolute;
    inset: 0;
    animation: in 1.4s ease-out both;
    overflow: hidden;
  }
  .slide::before {
    content: '';
    position: absolute;
    inset: -40px;
    background: var(--bg) center / cover;
    filter: blur(40px) brightness(0.45);
  }
  img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .kb {
    animation: 14s ease-in-out both;
  }
  .kb0 {
    animation-name: kb0;
  }
  .kb1 {
    animation-name: kb1;
  }
  .kb2 {
    animation-name: kb2;
  }
  .kb3 {
    animation-name: kb3;
  }
  @keyframes in {
    from {
      opacity: 0;
    }
  }
  @keyframes kb0 {
    from { transform: scale(1) translate(0, 0); }
    to { transform: scale(1.12) translate(-2%, -1%); }
  }
  @keyframes kb1 {
    from { transform: scale(1.12) translate(2%, 0); }
    to { transform: scale(1) translate(0, 0); }
  }
  @keyframes kb2 {
    from { transform: scale(1.05) translate(0, 2%); }
    to { transform: scale(1.14) translate(1%, -2%); }
  }
  @keyframes kb3 {
    from { transform: scale(1.14) translate(-1%, -1%); }
    to { transform: scale(1.03) translate(1%, 1%); }
  }
  .err {
    position: absolute;
    top: 40%;
    width: 100%;
    text-align: center;
    color: var(--muted);
    font-size: 1.6vw;
  }
</style>
