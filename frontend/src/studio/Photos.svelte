<script lang="ts">
  import { api } from '../lib/api';
  import Clip from '../lib/Clip.svelte';
  import { fileUrl, type Asset } from '../lib/types';
  import type { StudioApi } from './kinds';

  // The operator's own photos for the show (D23): restyled on the GPU once,
  // kept as files, then they play offline. Restyles approve themselves; the
  // generated clips (hair in the breeze) wait for ✓ here.
  let { call, code, fileKey, assets, onerror }: { call: StudioApi; code: string; fileKey: string; assets: Asset[]; onerror: (m: string) => void } = $props();

  type Style = { id: string; label: string; group: string };
  let styles = $state<Style[]>([]);
  api
    .get<{ styles: Style[] }>('/api/styles')
    .then((r) => (styles = r.styles))
    .catch(() => {});
  let style = $state('anime');
  let medium = $state('photo');
  let nickname = $state('');
  let files = $state<FileList | null>(null);
  let sending = $state(0);
  let picked = $state<string[]>([]);

  const photos = $derived(assets.filter((a) => a.meta && (a.meta as any).studio && (a.kind === 'restyle' || a.kind === 'upload')));
  const ids = $derived(new Set(photos.map((a) => a.id)));
  const clips = $derived(assets.filter((a) => (a.kind === 'live' || a.kind === 'breeze') && a.srcId && ids.has(a.srcId)));
  const pendingClips = $derived(clips.filter((a) => a.status === 'pending'));
  const busy = $derived(photos.filter((a) => a.status === 'processing').length + clips.filter((a) => a.status === 'processing').length);
  const clipOf = (id: string) => clips.filter((c) => c.srcId === id && c.status === 'approved').map((c) => (c.kind === 'breeze' ? '💨' : '✨')).join('');

  async function upload(e: Event) {
    e.preventDefault();
    if (!files?.length) return;
    const list = [...files];
    sending = list.length;
    for (const f of list) {
      const fd = new FormData();
      fd.append('photo', f);
      fd.append('style', style);
      fd.append('medium', medium);
      fd.append('nickname', nickname);
      try {
        await call.post('/studio/photos', fd);
      } catch (err: any) {
        onerror(`${f.name}: ${err.message}`);
      }
      sending--;
    }
    files = null;
  }

  async function animate(motion: 'alive' | 'breeze') {
    const target = picked.length ? picked : photos.filter((a) => a.status === 'approved').map((a) => a.id);
    try {
      const r = await call.post<{ queued: number }>('/animate', { ids: target, motion });
      if (!r.queued) onerror(motion === 'breeze' ? 'vybrané fotky už vlasy ve větru mají' : 'vybrané fotky už živé jsou');
    } catch (err: any) {
      onerror(err.message);
    }
  }
  const moderate = (a: Asset, ok: boolean) => call.post(`/assets/${a.id}/${ok ? 'approve' : 'reject'}`).catch((err) => onerror(err.message));
  const pick = (id: string) => (picked = picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]);
</script>

<form class="row" onsubmit={upload}>
  <input type="file" accept="image/*" multiple onchange={(e) => (files = e.currentTarget.files)} />
  <select bind:value={style}>
    {#each styles as s (s.id)}<option value={s.id}>{s.group} · {s.label}</option>{/each}
  </select>
  <select bind:value={medium}>
    <option value="photo">fotka</option>
    <option value="illustration">ilustrace</option>
  </select>
  <input class="nick" placeholder="popisek (např. Ema)" bind:value={nickname} />
  <button class="primary" disabled={!files?.length || sending > 0}>{sending ? `posílám (${sending})…` : 'Nahrát a restylovat'}</button>
</form>
<p class="muted small">
  Restyle i klipy počítá GPU (COMFY_URL, okno na SPARKu přes broker). Výsledky se uloží jako soubory a show je pak přehraje offline.
  {#if busy}<strong>Na GPU čeká {busy}.</strong>{/if}
</p>

<div class="row small">
  <button onclick={() => animate('alive')}>✨ Oživit {picked.length ? `vybrané (${picked.length})` : 'všechny'}</button>
  <button onclick={() => animate('breeze')}>💨 Vlasy ve větru {picked.length ? `vybraným (${picked.length})` : 'všem'}</button>
  {#if picked.length}<button onclick={() => (picked = [])}>zrušit výběr</button>{/if}
</div>

{#if pendingClips.length}
  <h3>Klipy ke schválení ({pendingClips.length})</h3>
  <div class="cards">
    {#each pendingClips as a (a.id)}
      <div class="card">
        <Clip src={fileUrl(code, a.file, fileKey)} />
        <span class="small">{a.kind === 'breeze' ? '💨 vlasy' : '✨ živý'}</span>
        <div class="row"><button class="ok" onclick={() => moderate(a, true)}>✓</button><button class="bad" onclick={() => moderate(a, false)}>✕</button></div>
      </div>
    {/each}
  </div>
{/if}

<div class="cards">
  {#each photos as a (a.id)}
    <button class="card photo" class:on={picked.includes(a.id)} onclick={() => a.status === 'approved' && pick(a.id)}>
      {#if a.file}<img src={fileUrl(code, a.file, fileKey)} alt="" />{:else}<div class="ph">maluje se…</div>{/if}
      <span class="small">
        {a.meta.nickname || '–'} · {a.meta.styleLabel}
        {#if a.status === 'processing'}<span class="muted">· na GPU</span>{/if}
        {#if a.status === 'pending'}<span class="warn">· originál, restyle selhal</span>{/if}
        {#if a.status === 'failed'}<span class="err">· selhalo</span>{/if}
        {clipOf(a.id)}
      </span>
    </button>
    {#if a.status === 'pending'}
      <div class="row"><button class="ok" onclick={() => moderate(a, true)}>přesto použít</button><button class="bad" onclick={() => moderate(a, false)}>✕</button></div>
    {/if}
  {:else}
    <p class="muted wide">Zatím žádné fotky ze Studia.</p>
  {/each}
</div>

<style>
  .row {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    align-items: center;
    margin-bottom: 8px;
  }
  .row input[type='file'] {
    width: auto;
    flex: 1;
  }
  .row select,
  .nick {
    width: auto;
  }
  .small {
    font-size: 0.85rem;
  }
  h3 {
    font-size: 0.9rem;
    margin: 10px 0 6px;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
    gap: 8px;
  }
  .card {
    background: var(--panel);
    border-radius: 10px;
    padding: 6px;
    display: grid;
    gap: 4px;
    text-align: left;
  }
  .card.on {
    outline: 3px solid var(--pink);
  }
  .card img,
  .card :global(video),
  .card :global(img),
  .ph {
    width: 100%;
    aspect-ratio: 3 / 4;
    object-fit: cover;
    border-radius: 6px;
  }
  .ph {
    display: grid;
    place-items: center;
    background: var(--bg-2);
    color: var(--muted);
  }
  .warn {
    color: var(--yellow);
  }
  .wide {
    grid-column: 1 / -1;
  }
</style>
