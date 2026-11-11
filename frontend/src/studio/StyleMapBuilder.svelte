<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { PackInfo } from '../lib/stylemap';
  import type { StudioApi } from './kinds';

  // StyleMap packs from a FineTuneGallery session (D23): the session is split
  // by prompt, every group the operator ticks becomes a pack built here (no
  // GPU, seconds) and stored on disk.
  let { call, packs, onchange, onerror }: { call: StudioApi; packs: PackInfo[]; onchange: () => void; onerror: (m: string) => void } = $props();

  type Session = { id: string; title: string; imageCount: number };
  type Job = { status: string; done: number; total: number; error?: string };
  type Group = { key: string; prompt: string; n: number; thumbUrl: string; pack: string; built: boolean; job?: Job };
  let sessions = $state<Session[]>([]);
  let session = $state('');
  let groups = $state<Group[]>([]);
  let picked = $state<string[]>([]);
  let err = $state('');
  let loading = $state(false);
  let timer = 0;
  const MIN = 8;

  call
    .get<Session[]>('/finetune/sessions')
    .then((s) => (sessions = s))
    .catch((e) => (err = e.message));

  async function loadGroups() {
    if (!session) return;
    loading = true;
    try {
      groups = await call.get<Group[]>(`/finetune/sessions/${encodeURIComponent(session)}/groups`);
      err = '';
      const running = groups.some((g) => g.job && (g.job.status === 'running' || g.job.status === 'queued'));
      clearTimeout(timer);
      if (running) timer = window.setTimeout(loadGroups, 1000);
      else if (picked.length && groups.some((g) => picked.includes(g.key) && g.built)) {
        picked = [];
        onchange();
      }
    } catch (e: any) {
      err = e.message;
    } finally {
      loading = false;
    }
  }
  onDestroy(() => clearTimeout(timer));

  async function build() {
    try {
      await call.post('/stylemaps/build', { session, groups: picked });
      loadGroups();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  let confirmDel = $state('');
  async function remove(id: string) {
    if (confirmDel !== id) {
      confirmDel = id;
      return;
    }
    confirmDel = '';
    try {
      await call.del(`/stylemaps/${id}`);
      onchange();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  const local = $derived(packs.filter((p) => p.source === 'party'));
</script>

{#if err}<p class="err small">{err}</p>{/if}
<div class="row">
  <select bind:value={session} onchange={() => ((picked = []), loadGroups())}>
    <option value="">(vyber session z FineTuneGallery)</option>
    {#each sessions as s (s.id)}<option value={s.id}>{s.title || s.id} ({s.imageCount})</option>{/each}
  </select>
  <button disabled={!picked.length} class="primary" onclick={build}>Vytvořit packy ({picked.length})</button>
</div>
{#if loading && !groups.length}<p class="muted">načítám session…</p>{/if}
<div class="groups">
  {#each groups as g (g.key)}
    <label class="group" class:off={g.n < MIN}>
      <input type="checkbox" disabled={g.n < MIN} checked={picked.includes(g.key)} onchange={() => (picked = picked.includes(g.key) ? picked.filter((k) => k !== g.key) : [...picked, g.key])} />
      <img src={g.thumbUrl} alt="" loading="lazy" />
      <span>
        <strong>{g.prompt || '(bez promptu)'}</strong>
        <span class="muted">{g.n} obrázků{g.n < MIN ? ` · na pack aspoň ${MIN}` : ''}</span>
        {#if g.job?.status === 'running'}<span>stavím {g.job.done}/{g.job.total}</span>
        {:else if g.job?.status === 'failed'}<span class="err">{g.job.error}</span>
        {:else if g.built}<span class="ok">✓ pack na disku</span>{/if}
      </span>
    </label>
  {/each}
</div>
{#if local.length}
  <h3>Lokální packy</h3>
  <ul>
    {#each local as p (p.id)}
      <li>
        {p.title} <span class="muted">({p.n})</span>
        <button class:bad={confirmDel === p.id} onclick={() => remove(p.id)}>{confirmDel === p.id ? 'Opravdu smazat' : 'Smazat'}</button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .row {
    display: flex;
    gap: 8px;
    align-items: center;
    margin-bottom: 8px;
  }
  .row select {
    flex: 1;
  }
  .groups {
    display: grid;
    gap: 6px;
    max-height: 320px;
    overflow: auto;
  }
  .group {
    display: flex;
    gap: 8px;
    align-items: center;
    font-size: 0.88rem;
  }
  .group.off {
    opacity: 0.5;
  }
  .group img {
    width: 40px;
    height: 54px;
    object-fit: cover;
    border-radius: 6px;
  }
  .group span {
    display: grid;
  }
  h3 {
    font-size: 0.9rem;
    margin: 10px 0 4px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 4px;
    font-size: 0.88rem;
  }
  .ok {
    color: var(--green);
  }
  .small {
    font-size: 0.85rem;
  }
</style>
