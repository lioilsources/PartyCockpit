<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../lib/api';
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

  const filters = [
    { id: 'none', label: 'Bez filtru' },
    { id: 'mirrorL', label: '🪞 Zrcadlo L' },
    { id: 'mirrorR', label: '🪞 Zrcadlo P' },
    { id: 'kaleido', label: '✳️ Kaleido' },
    { id: 'ufo', label: '👽 Ufon' },
    { id: 'pop', label: '🌈 Pop' },
  ] as const;
  type FilterId = (typeof filters)[number]['id'];

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
      onmsg(e?.name === 'NotAllowedError' ? 'Bez povolení kamery to nepůjde' : `Kamera: ${e?.message ?? e}`);
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
              onmsg(m.data.reason ? `Stream skončil: ${m.data.reason}` : 'Stream skončil');
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

<h2>Živě na plátně</h2>
{#if !secure}
  <p class="muted">
    Vysílání z prohlížeče potřebuje zabezpečenou adresu (https). Na této party ho zkus přes aplikaci
    <strong>MirrorBooth</strong> → „Stream to party“, nebo řekni operátorovi o https odkaz.
  </p>
{:else}
  {#if out}
    <!-- svelte-ignore a11y_media_has_caption -->
    <video class="preview" use:srcObject={out} autoplay playsinline muted></video>
    <div class="chips">
      {#each filters as f (f.id)}
        <button class:on={filter === f.id} onclick={() => (filter = f.id)}>{f.label}</button>
      {/each}
    </div>
    {#if !stream}
      <button class="primary wide" onclick={goLive}>📡 Vysílat na plátno</button>
      <button class="wide" onclick={stopCamera}>Vypnout kameru</button>
    {:else}
      <p class="state">
        {#if stream.status === 'live'}
          🔴 <strong>Jsi na plátně</strong>{viewers ? '' : ' — připojuji…'}
        {:else}
          ⏳ Čekáš, až tě operátor pustí na plátno
        {/if}
      </p>
      <p class="muted">Nech telefon odemčený a stránku otevřenou.</p>
      <button class="bad wide" onclick={stopLive}>Ukončit vysílání</button>
    {/if}
  {:else}
    <p class="muted">Tvoje kamera se objeví v mřížce na plátně (když tě operátor schválí). Zvuk se neposílá.</p>
    <button class="primary wide" disabled={starting} onclick={startCamera}>{starting ? '…' : 'Zapnout kameru'}</button>
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
