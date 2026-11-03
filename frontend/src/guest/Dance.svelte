<script lang="ts">
  import { api } from '../lib/api';
  import { fileUrl, type Asset, type Motion } from '../lib/types';

  // "Roztanči mě" (M4): pick one of my finished photos and a dance; the GPU makes a
  // short clip, the operator approves it and it plays in the Tančírna scene.
  let {
    code,
    guestId,
    mine,
    busy,
    onmsg,
    onsent,
  }: { code: string; guestId: string; mine: Asset[]; busy: boolean; onmsg: (t: string) => void; onsent: () => void } = $props();

  let moves = $state<Motion[]>([]);
  api.get<Motion[]>('/api/motions').then((m) => (moves = m.filter((x) => x.engine === 'dance'))).catch(() => {});

  const photos = $derived(mine.filter((a) => a.kind === 'restyle' && a.file && a.status !== 'failed'));
  let photo = $state('');
  let move = $state('disco');
  let sending = $state(false);
  $effect(() => {
    if (!photo && photos.length) photo = photos[0].id;
  });

  async function send() {
    sending = true;
    try {
      await api.post(`/api/parties/${code}/guests/${guestId}/dance`, { assetId: photo, move });
      onmsg('Tanec se učí na GPU 💃 — za pár minut, po schválení, jsi na plátně');
      onsent();
    } catch (e: any) {
      onmsg(e.message);
    } finally {
      sending = false;
    }
  }
</script>

<h2>💃 Roztanči mě</h2>
{#if !photos.length}
  <p class="muted">Nejdřív pošli fotku (nejlépe celou postavu). Až bude hotová, roztančíme ji.</p>
{:else}
  <div class="pick">
    {#each photos.slice(0, 8) as a (a.id)}
      <button class:sel={photo === a.id} onclick={() => (photo = a.id)}><img src={fileUrl(code, a.file)} alt="" /></button>
    {/each}
  </div>
  <div class="moves">
    {#each moves as m (m.id)}
      <button class:on={move === m.id} onclick={() => (move = m.id)}>{m.label}</button>
    {/each}
  </div>
  <button class="primary wide" disabled={!photo || sending || busy} onclick={send}>
    {busy ? 'Ještě se ti něco maluje…' : sending ? 'Posílám…' : 'Roztančit'}
  </button>
{/if}

<style>
  .pick {
    display: flex;
    gap: 8px;
    overflow-x: auto;
    padding-bottom: 4px;
  }
  .pick button {
    padding: 0;
    border: 3px solid transparent;
    border-radius: 10px;
    flex: none;
    background: none;
  }
  .pick button.sel {
    border-color: var(--pink);
  }
  .pick img {
    width: 64px;
    height: 84px;
    object-fit: cover;
    border-radius: 7px;
    display: block;
  }
  .moves {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .wide {
    width: 100%;
    padding: 0.9em;
  }
</style>
