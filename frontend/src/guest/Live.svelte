<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../lib/api';
  import { t } from '../lib/i18n.svelte';
  import { connect, type Socket } from '../lib/ws';
  import { Publisher, srcObject, type RtcMsg } from '../lib/rtc';
  import type { LiveStream } from '../lib/types';

  // Live camera from the browser (M3). MirrorBooth does the same natively; here the
  // filters run on a canvas, so what the stage gets is what the preview shows.
  let {
    code,
    guestId,
    iceServers,
    onmsg,
  }: { code: string; guestId: string; iceServers: RTCIceServer[]; onmsg: (t: string) => void } = $props();

  const filters = ['none', 'mirrorL', 'mirrorR', 'kaleido', 'ufo', 'pop'] as const;
  type FilterId = (typeof filters)[number];

  const secure = window.isSecureContext && !!navigator.mediaDevices?.getUserMedia;
  let filter = $state<FilterId>('mirrorL');
  let camera: MediaStream | null = null;
  let out = $state<MediaStream | null>(null);
  let stream = $state<LiveStream | null>(null);
  let viewers = $state(0);
  let starting = $state(false);
  let sock: Socket | null = null;
  let pub: Publisher | null = null;
  let raf = 0;
  let stats: ReturnType<typeof setInterval>;
  let wake: { release: () => Promise<void> } | null = null;

  const video = document.createElement('video');
  video.muted = true;
  video.playsInline = true;
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  async function startCamera() {
    starting = true;
    try {
      camera = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: 'user', width: { ideal: 720 }, height: { ideal: 1280 }, frameRate: { ideal: 24 } },
        audio: false,
      });
      video.srcObject = camera;
      await video.play();
      const s = Math.min(1, 720 / Math.max(video.videoWidth, video.videoHeight));
      canvas.width = Math.round(video.videoWidth * s);
      canvas.height = Math.round(video.videoHeight * s);
      draw();
      out = canvas.captureStream(24);
    } catch (e: any) {
      onmsg(e?.name === 'NotAllowedError' ? t('live.denied') : t('live.cameraErr', { message: e?.message ?? e }));
      stopCamera();
    } finally {
      starting = false;
    }
  }

  // Selfie = mirrored like a mirror. The canvas is what everyone sees.
  function draw() {
    const w = canvas.width,
      h = canvas.height;
    ctx.save();
    ctx.filter = filter === 'ufo' ? 'hue-rotate(90deg) saturate(2.2) contrast(1.15)' : filter === 'pop' ? 'saturate(2.6) contrast(1.25)' : 'none';
    ctx.translate(w, 0);
    ctx.scale(-1, 1);
    ctx.drawImage(video, 0, 0, w, h);
    ctx.restore();
    if (filter === 'mirrorL' || filter === 'mirrorR' || filter === 'kaleido') {
      // copy one half over the other, flipped: the MirrorBooth symmetric face
      const half = Math.floor(w / 2);
      ctx.save();
      if (filter === 'mirrorR') {
        ctx.translate(half, 0);
        ctx.scale(-1, 1);
        ctx.drawImage(canvas, half, 0, w - half, h, -half, 0, half, h);
      } else {
        ctx.translate(w, 0);
        ctx.scale(-1, 1);
        ctx.drawImage(canvas, 0, 0, half, h, 0, 0, half, h);
      }
      ctx.restore();
      if (filter === 'kaleido') {
        const hh = Math.floor(h / 2);
        ctx.save();
        ctx.translate(0, h);
        ctx.scale(1, -1);
        ctx.drawImage(canvas, 0, 0, w, hh, 0, 0, w, hh);
        ctx.restore();
      }
    }
    if (filter === 'ufo') {
      ctx.fillStyle = 'rgba(80, 255, 120, 0.12)';
      ctx.fillRect(0, 0, w, h);
    }
    raf = requestAnimationFrame(draw);
  }

  async function goLive() {
    if (!out) return;
    try {
      const r = await api.post<{ stream: LiveStream; key: string }>(`/api/parties/${code}/guests/${guestId}/stream`, { source: 'browser' });
      stream = r.stream;
      pub = new Publisher((m) => sock?.send(m) ?? false, iceServers ?? [], out);
      sock = connect(
        code,
        'streamer',
        (m) => {
          if (m.type === 'stream') {
            stream = m.data;
            if (m.data.status === 'ended') {
              onmsg(m.data.reason ? t('live.endedReason', { reason: m.data.reason }) : t('live.ended'));
              endLocal();
            }
          } else if (m.type === 'rtc') pub?.handle(m.data as RtcMsg);
        },
        () => {},
        { key: r.key },
      );
      stats = setInterval(() => (viewers = pub?.viewers ?? 0), 1500);
      wake = await (navigator as any).wakeLock?.request('screen').catch(() => null);
    } catch (e: any) {
      onmsg(e.message);
    }
  }

  function endLocal() {
    clearInterval(stats);
    pub?.close();
    pub = null;
    sock?.close();
    sock = null;
    stream = null;
    viewers = 0;
    wake?.release().catch(() => {});
    wake = null;
  }

  async function stopLive() {
    await api.del(`/api/parties/${code}/guests/${guestId}/stream`).catch(() => {});
    endLocal();
  }

  function stopCamera() {
    cancelAnimationFrame(raf);
    camera?.getTracks().forEach((t) => t.stop());
    camera = null;
    out?.getTracks().forEach((t) => t.stop());
    out = null;
  }

  onDestroy(() => {
    if (stream) stopLive();
    stopCamera();
  });
</script>

<h2>{t('live.title')}</h2>
{#if !secure}
  <p class="muted">{t('live.insecure')}</p>
{:else}
  {#if out}
    <!-- svelte-ignore a11y_media_has_caption -->
    <video class="preview" use:srcObject={out} autoplay playsinline muted></video>
    <div class="chips">
      {#each filters as f (f)}
        <button class:on={filter === f} onclick={() => (filter = f)}>{t(`live.f.${f}`)}</button>
      {/each}
    </div>
    {#if !stream}
      <button class="primary wide" onclick={goLive}>{t('live.go')}</button>
      <button class="wide" onclick={stopCamera}>{t('live.cameraOff')}</button>
    {:else}
      <p class="state">
        {#if stream.status === 'live'}
          🔴 <strong>{t('live.onAir')}</strong>{viewers ? '' : ` — ${t('live.connecting')}`}
        {:else}
          {t('live.waiting')}
        {/if}
      </p>
      <p class="muted">{t('live.keepOpen')}</p>
      <button class="bad wide" onclick={stopLive}>{t('live.stop')}</button>
    {/if}
  {:else}
    <p class="muted">{t('live.intro')}</p>
    <button class="primary wide" disabled={starting} onclick={startCamera}>{starting ? '…' : t('live.cameraOn')}</button>
  {/if}
{/if}

<style>
  .preview {
    width: 100%;
    max-height: 52vh;
    object-fit: contain;
    background: #000;
    border-radius: 14px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .chips button {
    font-size: 0.85rem;
    padding: 0.4em 0.8em;
  }
  .wide {
    width: 100%;
    padding: 0.9em;
  }
  .state {
    font-size: 1.1rem;
    margin: 0;
  }
</style>
