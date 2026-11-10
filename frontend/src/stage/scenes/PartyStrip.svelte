<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { fileUrl, type Asset } from '../../lib/types';
  import Handwritten from '../../lib/Handwritten.svelte';

  // Party episode (D18): tonight as a webtoon strip, made only from what the
  // operator approved. Panels are the guests' restyled photos in show order; a
  // panel's speech bubble is a note the same guest stuck on the wall (in the
  // handwriting they picked), otherwise a narrator caption. No GPU, no new
  // images: the episode grows as the party does.
  let { code, queue, title }: { code: string; queue: Asset[]; title: string } = $props();

  const vw = innerWidth / 100;

  type Panel = { photo: Asset; note?: Asset; caption: string };
  const panels = $derived.by(() => {
    const photos = queue.filter((a) => (a.kind === 'restyle' || a.kind === 'upload') && a.file);
    // each guest's notes, oldest first, handed out one per panel of theirs
    const notes = new Map<string, Asset[]>();
    for (const n of queue.filter((a) => a.kind === 'sticker' && a.meta.text)) {
      notes.set(n.guestId, [...(notes.get(n.guestId) ?? []), n]);
    }
    const used = new Map<string, number>();
    const lines = [0, 1, 2, 3, 4, 5].map((i) => t(`stage.strip.l${i}`));
    return photos.map((photo, i): Panel => {
      const k = used.get(photo.guestId) ?? 0;
      const note = notes.get(photo.guestId)?.[k];
      if (note) used.set(photo.guestId, k + 1);
      const who = photo.meta.nickname ?? t('stage.guest');
      const line = i === 0 ? lines[0] : lines[i % lines.length];
      const style = photo.meta.styleLabel && !photo.meta.fallback ? photo.meta.styleLabel : '';
      return { photo, note, caption: style ? t('stage.strip.captionAs', { line, who, style }) : t('stage.strip.caption', { line, who }) };
    });
  });
</script>

<h1>{title}</h1>
<p class="sub">{t('stage.strip.sub')}</p>
{#each panels as p, i (p.photo.id)}
  <figure class="panel" style:--tilt={`${i % 2 ? 0.8 : -0.8}deg`}>
    <div class="caption">{p.caption}</div>
    <img src={fileUrl(code, p.photo.live ?? p.photo.file)} alt="" />
    {#if p.note}
      <div class="bubble" class:right={i % 2 === 1}>
        {#if p.note.meta.sheet}
          <Handwritten text={p.note.meta.text ?? ''} sheet={p.note.meta.sheet} cell={2.2 * vw} width={18 * vw} density={1.5} />
        {:else}
          <p>{p.note.meta.text}</p>
        {/if}
      </div>
    {/if}
  </figure>
{:else}
  <p class="sub">{t('stage.strip.empty')}</p>
{/each}

<style>
  h1 {
    font-size: 3.4vw;
    margin: 6vh 0 0;
  }
  .sub {
    color: var(--muted);
    font-size: 1.5vw;
    margin: 0.5vh 0 5vh;
  }
  .panel {
    position: relative;
    width: fit-content;
    max-width: min(46vw, 900px);
    margin: 0 0 6vh;
    background: #fff;
    padding: 0.6vw;
    transform: rotate(var(--tilt));
    box-shadow: 0 1.5vw 3vw rgba(0, 0, 0, 0.6);
  }
  .panel img {
    display: block;
    max-width: 100%;
    max-height: 80vh;
  }
  .caption {
    position: absolute;
    top: 1.4vw;
    left: 1.4vw;
    max-width: 70%;
    background: var(--yellow);
    color: #1a1000;
    font-size: 1.3vw;
    padding: 0.4vw 0.9vw;
    border: 0.15vw solid #1a1000;
    z-index: 1;
  }
  .bubble {
    position: absolute;
    bottom: 2vw;
    left: 2vw;
    max-width: 80%;
    background: #fff;
    color: #1d1208;
    border: 0.18vw solid #1d1208;
    border-radius: 2vw;
    padding: 1vw 1.4vw;
    font-family: var(--hand);
    font-size: 1.8vw;
  }
  .bubble::after {
    content: '';
    position: absolute;
    top: -1.4vw;
    left: 3vw;
    border: 0.7vw solid transparent;
    border-bottom: 1.4vw solid #1d1208;
  }
  .bubble.right {
    left: auto;
    right: 2vw;
  }
  .bubble.right::after {
    left: auto;
    right: 3vw;
  }
  .bubble p {
    margin: 0;
  }
</style>
