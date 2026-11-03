<script lang="ts">
  import { onDestroy } from 'svelte';
  import { fade } from 'svelte/transition';
  import { connect } from '../lib/ws';
  import { store } from '../lib/api';
  import { BeatClock } from '../lib/beat';
  import type { Asset, MusicSnapshot, Party, Tempo } from '../lib/types';
  import QrOverlay from './QrOverlay.svelte';
  import NowPlayingBar from './NowPlayingBar.svelte';
  import Reactions from './Reactions.svelte';
  import Slideshow from './scenes/Slideshow.svelte';
  import Gallery from './scenes/Gallery.svelte';
  import Stickers from './scenes/Stickers.svelte';
  import NowPlaying from './scenes/NowPlaying.svelte';
  import Webtoon from './scenes/Webtoon.svelte';
  import Lottery from './scenes/Lottery.svelte';
  import Blank from './scenes/Blank.svelte';
  import PipWall from './scenes/PipWall.svelte';
  import DanceFloor from './scenes/DanceFloor.svelte';
  import { Viewer, type RtcMsg } from '../lib/rtc';
  import type { LiveStream } from '../lib/types';

  let { code }: { code: string } = $props();

  // The stage survives a backend outage (plan §8): last state is cached and the
  // scenes keep running on what they already have.
  const cacheKey = `pc.stage.${code}`;
  const cached = (() => {
    try {
      return JSON.parse(store.get(cacheKey) ?? 'null');
    } catch {
      return null;
    }
  })();

  let party = $state<Party | null>(cached?.party ?? null);
  let queue = $state<Asset[]>(cached?.queue ?? []);
  let music = $state<MusicSnapshot | null>(cached?.music ?? null);
  let online = $state(false);
  let reactions: Reactions;
  let pulse = $state(false);
  let needsAudioClick = $state(false);
  let streams = $state<LiveStream[]>([]);
  let tempo = $state<Tempo | null>(null);
  let rtcRev = $state(0);
  // created before the socket; sends go through it once it exists
  const viewer = new Viewer((m) => sock?.send(m) ?? false, [], () => rtcRev++);

  const clock = new BeatClock();
  clock.on(() => {
    pulse = true;
    setTimeout(() => (pulse = false), 90);
  });

  function save() {
    store.set(cacheKey, JSON.stringify({ party, queue, music }));
  }

  const sock = connect(
    code,
    'stage',
    (m) => {
      switch (m.type) {
        case 'party':
          party = m.data;
          break;
        case 'queue':
          queue = m.data ?? [];
          break;
        case 'music':
          music = m.data;
          break;
        case 'tempo':
          tempo = m.data as Tempo;
          clock.setTempo(tempo);
          break;
        case 'reaction':
          reactions?.add(m.data.emoji);
          break;
        case 'panic':
          reactions?.clear();
          break;
        case 'streams':
          streams = m.data ?? [];
          break;
        case 'rtc':
          viewer.handle(m.data as RtcMsg);
          return;
        case 'hello':
          viewer.reset(); // new socket = new peer id; old offers are orphaned
          return;
      }
      save();
    },
    (o) => (online = o),
  );

  // Audio-input beat detection when ?audio=1 (Chrome kiosk with
  // --use-fake-ui-for-media-stream skips the prompt; otherwise one click).
  if (new URLSearchParams(location.search).get('audio') === '1') {
    clock.enableAudio().catch(() => (needsAudioClick = true));
  }

  onDestroy(() => {
    viewer.destroy();
    sock.close();
    clock.destroy();
  });

  const scene = $derived(party?.scene ?? { kind: 'slideshow', params: {}, startedAt: 0 });
  const loopMode = $derived(party?.loopMode ?? 'loop');
  const sceneKey = $derived(`${scene.kind}:${scene.startedAt}`);

  // phones are only connected while the wall is on screen (decoding 9 hidden
  // streams would eat the mini PC)
  $effect(() => {
    viewer.setIce(party?.iceServers);
    viewer.sync(scene.kind === 'pipwall' ? streams : []);
  });

  function fullscreen() {
    if (needsAudioClick) {
      clock.enableAudio().then(() => (needsAudioClick = false)).catch(() => {});
    }
    document.documentElement.requestFullscreen?.().catch(() => {});
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="stage" class:pulse onclick={fullscreen}>
  {#key sceneKey}
    <div class="scene" transition:fade={{ duration: 700 }}>
      {#if scene.kind === 'slideshow'}
        <Slideshow params={scene.params} {clock} />
      {:else if scene.kind === 'gallery'}
        <Gallery {code} {queue} {loopMode} params={scene.params} {clock} />
      {:else if scene.kind === 'stickers'}
        <Stickers {queue} {loopMode} {clock} />
      {:else if scene.kind === 'nowplaying'}
        <NowPlaying {music} joinUrl={party?.joinUrl ?? ''} {clock} />
      {:else if scene.kind === 'webtoon'}
        <Webtoon params={scene.params} />
      {:else if scene.kind === 'lottery'}
        <Lottery params={scene.params} />
      {:else if scene.kind === 'dance'}
        <DanceFloor {code} {queue} {loopMode} params={scene.params} {tempo} {clock} />
      {:else if scene.kind === 'pipwall'}
        <PipWall {streams} {viewer} rev={rtcRev} {clock} />
      {:else}
        <Blank name={party?.name ?? ''} {clock} />
      {/if}
    </div>
  {/key}

  <Reactions bind:this={reactions} />
  {#if scene.kind !== 'nowplaying'}
    <NowPlayingBar {music} />
  {/if}
  {#if party && scene.kind !== 'lottery'}
    <QrOverlay url={party.joinUrl} code={party.code} />
  {/if}
  {#if !online}<div class="offline" title="backend nedostupný — jedu z cache"></div>{/if}
  {#if needsAudioClick}<div class="hint">klikni pro zapnutí mikrofonu (beat)</div>{/if}
</div>

<style>
  .stage {
    position: fixed;
    inset: 0;
    overflow: hidden;
    background: #000;
    cursor: none;
    user-select: none;
  }
  .scene {
    position: absolute;
    inset: 0;
  }
  .stage::after {
    content: '';
    position: absolute;
    inset: 0;
    pointer-events: none;
    box-shadow: inset 0 0 0 0 rgba(255, 61, 139, 0);
    transition: box-shadow 0.25s ease-out;
  }
  .stage.pulse::after {
    box-shadow: inset 0 0 80px 6px rgba(255, 61, 139, 0.35);
    transition: none;
  }
  .offline {
    position: absolute;
    top: 14px;
    left: 14px;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--red);
    opacity: 0.7;
  }
  .hint {
    position: absolute;
    top: 12px;
    left: 50%;
    transform: translateX(-50%);
    background: rgba(0, 0, 0, 0.6);
    padding: 6px 14px;
    border-radius: 999px;
    font-size: 14px;
  }
</style>
