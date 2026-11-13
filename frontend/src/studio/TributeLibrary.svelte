<script lang="ts">
  import { onDestroy } from 'svelte';
  import { creditLine } from '../lib/tributes';
  import type { StudioApi } from './kinds';

  // PromoClown tributes (D24): pick and download them once, then they play
  // from disk. Downloading is the approval for the stage. The list comes from
  // promo-api (home network only); what is on disk stays listed without it.
  let { call, code, fileKey, onerror }: { call: StudioApi; code: string; fileKey: string; onerror: (m: string) => void } = $props();

  type Job = { pct: number; status: 'queued' | 'running' | 'done' | 'failed'; error?: string };
  type Item = {
    id: string;
    credit: string;
    project: string;
    labels: string[];
    n: number;
    postStatus: string;
    createdAt: string;
    installed: boolean;
    remote: boolean;
    v?: number;
    job?: Job;
  };
  let items = $state<Item[]>([]);
  let configured = $state(true);
  let err = $state('');
  let loaded = $state(false);
  let picked = $state<string[]>([]);
  let project = $state('');
  let onlyNew = $state(false);
  let confirmDel = $state('');
  let timer = 0;

  const busy = (it: Item) => it.job?.status === 'queued' || it.job?.status === 'running';

  async function load() {
    try {
      const r = await call.get<{ configured: boolean; error?: string; items: Item[] }>('/tributes');
      items = r.items;
      configured = r.configured;
      err = r.error ?? '';
      loaded = true;
      clearTimeout(timer);
      if (items.some(busy)) timer = window.setTimeout(load, 1000);
    } catch (e: any) {
      err = e.message;
    }
  }
  load();
  onDestroy(() => clearTimeout(timer));

  const projects = $derived([...new Set(items.map((i) => i.project).filter(Boolean))]);
  const shown = $derived(items.filter((i) => (!project || i.project === project) && (!onlyNew || !i.installed)));
  const fresh = $derived(shown.filter((i) => i.remote && !i.installed && !busy(i)).map((i) => i.id));

  async function download(ids: string[]) {
    if (!ids.length) return;
    try {
      for (let i = 0; i < ids.length; i += 40) await call.post('/tributes/import', { ids: ids.slice(i, i + 40) });
      picked = [];
      load();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  async function remove(id: string) {
    if (confirmDel !== id) {
      confirmDel = id;
      return;
    }
    confirmDel = '';
    try {
      await call.del(`/tributes/${id}`);
      load();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  const pick = (id: string) => (picked = picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]);
  const thumb = (it: Item, n: number) =>
    it.installed ? `/tributes/${it.id}/${n}s.jpg?v=${it.v ?? 0}` : `/api/parties/${code}/tributes/${it.id}/thumb/${n}?k=${fileKey}`;
  const status: Record<string, string> = { draft: 'čeká na schválení', approved: 'schválený', scheduled: 'naplánovaný', published: 'publikovaný', failed: 'publikace selhala' };
  const date = (s: string) => (s ? new Date(s).toLocaleDateString('cs-CZ') : '');
</script>

<p class="small" class:err={!!err} class:muted={!err}>
  {#if !configured}
    Stahování není nastavené (<code>PROMO_API_URL</code> v <code>.env</code>). Stažené tributy hrají dál.
  {:else if err}
    {err}. Stažené tributy hrají dál.
  {:else if loaded}
    Připojeno k PromoClown. Na plátno jde jen to, co tady stáhneš.
  {:else}
    načítám…
  {/if}
</p>

<div class="row small">
  {#if configured}
    <button class="primary" disabled={!picked.length} onclick={() => download(picked)}>Stáhnout vybrané ({picked.length})</button>
    <button disabled={!fresh.length} onclick={() => download(fresh)}>Stáhnout všechny nové ({fresh.length})</button>
  {/if}
  {#if projects.length > 1}
    <select bind:value={project}>
      <option value="">všechny projekty</option>
      {#each projects as p (p)}<option value={p}>{p}</option>{/each}
    </select>
  {/if}
  <label><input type="checkbox" bind:checked={onlyNew} /> jen nestažené</label>
</div>

<div class="cards">
  {#each shown as it (it.id)}
    <div class="card" class:on={picked.includes(it.id)}>
      <button class="grid" disabled={!it.remote || it.installed || busy(it)} onclick={() => pick(it.id)} title={creditLine(it.credit)}>
        {#each Array.from({ length: Math.min(4, it.n) }, (_, i) => i) as n (n)}
          <img src={thumb(it, n)} alt="" loading="lazy" />
        {/each}
      </button>
      <strong>{it.credit}</strong>
      <span class="muted small">{it.labels.filter(Boolean).join(' · ')}</span>
      <span class="muted small">{it.project}{it.createdAt ? ` · ${date(it.createdAt)}` : ''}{it.postStatus ? ` · ${status[it.postStatus] ?? it.postStatus}` : ''}</span>
      <div class="row small">
        {#if busy(it)}
          <span class="muted">stahuju {it.job?.pct ?? 0} %</span>
        {:else if it.installed}
          <span class="ok">✓ na disku</span>
          {#if it.remote && configured}<button onclick={() => download([it.id])}>Stáhnout znovu</button>{/if}
          <button class:bad={confirmDel === it.id} onclick={() => remove(it.id)}>{confirmDel === it.id ? 'Opravdu smazat' : 'Smazat'}</button>
        {/if}
        {#if it.job?.status === 'failed'}<span class="err">{it.job.error || 'selhalo'}</span>{/if}
      </div>
    </div>
  {:else}
    {#if loaded}<p class="muted wide">{configured ? 'PromoClown nemá žádné hotové tributy.' : 'Na disku nejsou žádné tributy.'}</p>{/if}
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
  .row select {
    width: auto;
  }
  .small {
    font-size: 0.85rem;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 8px;
  }
  .card {
    background: var(--panel);
    border-radius: 10px;
    padding: 6px;
    display: grid;
    gap: 3px;
    align-content: start;
  }
  .card.on {
    outline: 3px solid var(--pink);
  }
  .card .row {
    margin: 0;
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 3px;
    padding: 4px;
    background: #fbfaf7;
    border-radius: 6px;
  }
  .grid:disabled {
    opacity: 1;
    cursor: default;
  }
  .grid img {
    width: 100%;
    aspect-ratio: 3 / 4;
    object-fit: cover;
    display: block;
    background: #ddd;
  }
  .ok {
    color: var(--green);
  }
  .wide {
    grid-column: 1 / -1;
  }
</style>
