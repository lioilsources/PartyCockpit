<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { onDestroy } from 'svelte';
  import QRCode from 'qrcode';
  import type { BeatClock } from '../../lib/beat';
  import type { MusicSnapshot } from '../../lib/types';

  let { music, joinUrl, clock }: { music: MusicSnapshot | null; joinUrl: string; clock: BeatClock } = $props();

  const np = $derived(music?.nowPlaying ?? null);
  let now = $state(Date.now());
  let beat = $state(false);
  let qr = $state('');

  // DJfy snapshots arrive on change only; position is estimated from updatedAt.
  const pos = $derived(
    np ? Math.min(np.durationMs, np.positionMs + (music!.isPlaying ? now - music!.updatedAt : 0)) : 0,
  );
  const fmt = (ms: number) => `${Math.floor(ms / 60000)}:${String(Math.floor((ms % 60000) / 1000)).padStart(2, '0')}`;

  const tick = setInterval(() => (now = Date.now()), 500);
  const off = clock.on(() => {
    beat = true;
    setTimeout(() => (beat = false), 120);
  });
  onDestroy(() => {
    clearInterval(tick);
    off();
  });
  $effect(() => {
    if (joinUrl) QRCode.toDataURL(joinUrl, { margin: 1, width: 300 }).then((d) => (qr = d));
  });
</script>

<div class="np" style:--art={np?.artworkUrl ? `url(${np.artworkUrl})` : 'none'}>
  {#if np}
    <div class="left">
      <div class="art" class:beat>
        {#if np.artworkUrl}<img src={np.artworkUrl} alt="" />{:else}<span>♪</span>{/if}
      </div>
      <h1>{np.title}</h1>
      <h2>{np.artist}</h2>
      {#if np.proposedByName}<p class="by">{t('stage.addedBy', { name: np.proposedByName })}</p>{/if}
      <div class="progress"><div style:width={`${(pos / Math.max(1, np.durationMs)) * 100}%`}></div></div>
      <div class="times"><span>{fmt(pos)}</span><span>{np.bpm ? `${Math.round(np.bpm)} BPM` : ''}</span><span>{fmt(np.durationMs)}</span></div>
    </div>
    <div class="right">
      <h3>{t('stage.np.next')}</h3>
      <ol>
        {#each (music?.queue ?? []).filter((q) => !q.blocked).slice(0, 7) as q (q.entryId)}
          <li>
            <div class="qt"><strong>{q.track.title}</strong><span>{q.track.artist} · {q.proposedByName}</span></div>
            <div class="votes">▲ {q.up}{#if q.down}<small> ▼ {q.down}</small>{/if}</div>
          </li>
        {:else}
          <li class="muted">{t('stage.np.empty')}</li>
        {/each}
      </ol>
      {#if qr}<div class="add"><img src={qr} alt="" /><span>{t('stage.np.add')}</span></div>{/if}
    </div>
  {:else}
    <div class="empty"><h1>{t('stage.np.idle')}</h1><p>{t('stage.np.idleHint')}</p></div>
  {/if}
</div>

<style>
  .np {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: 1.1fr 1fr;
    gap: 4vw;
    padding: 6vh 5vw;
    background: #0b0613;
    overflow: hidden;
  }
  .np::before {
    content: '';
    position: absolute;
    inset: -10%;
    background: var(--art) center / cover;
    filter: blur(60px) brightness(0.35) saturate(1.4);
  }
  .left,
  .right {
    position: relative;
    min-width: 0;
  }
  .art {
    width: 34vh;
    height: 34vh;
    border-radius: 1.4vw;
    overflow: hidden;
    background: var(--pink);
    display: grid;
    place-items: center;
    font-size: 10vh;
    box-shadow: 0 2vw 5vw rgba(0, 0, 0, 0.6);
    transition: transform 0.25s ease-out;
  }
  .art.beat {
    transform: scale(1.04);
    transition: none;
  }
  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  h1 {
    font-size: 4vw;
    line-height: 1;
    margin: 4vh 0 1vh;
    letter-spacing: -0.02em;
  }
  h2 {
    font-size: 2vw;
    margin: 0;
    color: var(--muted);
    font-weight: 400;
  }
  .by {
    color: var(--yellow);
    font-size: 1.4vw;
  }
  .progress {
    height: 0.5vw;
    background: rgba(255, 255, 255, 0.15);
    border-radius: 1vw;
    margin-top: 3vh;
    overflow: hidden;
  }
  .progress div {
    height: 100%;
    background: var(--pink);
    transition: width 0.5s linear;
  }
  .times {
    display: flex;
    justify-content: space-between;
    color: var(--muted);
    font-size: 1.1vw;
    margin-top: 0.6vh;
  }
  h3 {
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--muted);
    font-size: 1.2vw;
  }
  ol {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 1.4vh;
  }
  li {
    display: flex;
    justify-content: space-between;
    gap: 1vw;
    background: rgba(255, 255, 255, 0.06);
    padding: 1.2vh 1.2vw;
    border-radius: 0.8vw;
    font-size: 1.3vw;
  }
  .qt {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .qt strong,
  .qt span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .qt span {
    color: var(--muted);
    font-size: 1vw;
  }
  .votes {
    color: var(--green);
    white-space: nowrap;
  }
  .votes small {
    color: var(--red);
  }
  .add {
    position: absolute;
    bottom: 0;
    right: 0;
    display: flex;
    align-items: center;
    gap: 1vw;
    font-size: 1.3vw;
  }
  .add img {
    width: 8vw;
    border-radius: 0.6vw;
  }
  .empty {
    grid-column: 1 / -1;
    display: grid;
    place-content: center;
    text-align: center;
    position: relative;
  }
  .empty h1 {
    font-size: 4vw;
  }
  .empty p {
    color: var(--muted);
    font-size: 1.6vw;
  }
</style>
