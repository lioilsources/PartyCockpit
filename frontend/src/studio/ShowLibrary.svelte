<script lang="ts">
  import type { StudioApi } from './kinds';

  // Saved shows (D23): a prepared show outlives the party. Save copies it with
  // its photos and clips to the library on this machine; load brings it into
  // this party (it replaces the current show).
  let { call, name, onerror }: { call: StudioApi; name: string; onerror: (m: string) => void } = $props();

  type Saved = { slug: string; name: string; savedAt: number; entries: number; assets: number };
  let shows = $state<Saved[]>([]);
  let confirm = $state('');
  let note = $state('');

  async function load() {
    try {
      shows = await call.get<Saved[]>('/shows');
    } catch (e: any) {
      onerror(e.message);
    }
  }
  load();

  async function save() {
    try {
      const r = await call.post<{ slug: string; assets: number; entries: number }>('/shows', { name });
      note = `uloženo jako „${r.slug}“ (${r.entries} scén, ${r.assets} fotek)`;
      load();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  async function loadShow(slug: string) {
    if (confirm !== `load:${slug}`) {
      confirm = `load:${slug}`;
      return;
    }
    confirm = '';
    try {
      const r = await call.post<{ assets: number; entries: number }>(`/shows/${slug}/load`);
      note = `načteno: ${r.entries} scén, ${r.assets} fotek`;
    } catch (e: any) {
      onerror(e.message);
    }
  }
  async function remove(slug: string) {
    if (confirm !== `del:${slug}`) {
      confirm = `del:${slug}`;
      return;
    }
    confirm = '';
    try {
      await call.del(`/shows/${slug}`);
      load();
    } catch (e: any) {
      onerror(e.message);
    }
  }
</script>

<div class="row">
  <button class="primary" disabled={!name.trim()} onclick={save}>💾 Uložit do knihovny</button>
  {#if !name.trim()}<span class="muted small">nejdřív show pojmenuj</span>{/if}
  {#if note}<span class="muted small">{note}</span>{/if}
</div>
<ul>
  {#each shows as s (s.slug)}
    <li>
      <strong>{s.name}</strong>
      <span class="muted">{s.entries} scén · {s.assets} fotek · {new Date(s.savedAt).toLocaleString('cs-CZ')}</span>
      <button class:on={confirm === `load:${s.slug}`} onclick={() => loadShow(s.slug)}>{confirm === `load:${s.slug}` ? 'Nahradit současnou show' : 'Načíst'}</button>
      <button class:bad={confirm === `del:${s.slug}`} onclick={() => remove(s.slug)}>{confirm === `del:${s.slug}` ? 'Opravdu smazat' : 'Smazat'}</button>
    </li>
  {:else}
    <li class="muted">Knihovna je prázdná.</li>
  {/each}
</ul>

<style>
  .row {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
    margin-bottom: 8px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 6px;
    font-size: 0.88rem;
  }
  li {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
  }
  .small {
    font-size: 0.85rem;
  }
</style>
