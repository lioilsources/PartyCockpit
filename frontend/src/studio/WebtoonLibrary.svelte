<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { StudioApi } from './kinds';

  // Toonfork packs (D23): download a part once (zip, sha256 checked), then its
  // episodes play from disk. The list comes from the public catalog; what is
  // installed stays listed without the uplink.
  let { call, onchange, onerror }: { call: StudioApi; onchange: () => void; onerror: (m: string) => void } = $props();

  type Job = { pct: number; status: 'running' | 'done' | 'failed'; error?: string };
  type Part = { index: number; part: string; bytes: number; stories: number; dir: string; installed: boolean; job?: Job };
  type Item = { id: string; kind: 'pack' | 'gamebook'; title: string; type: string; audience: string; free: boolean; bytes: number; parts: Part[] };
  type Installed = { dir: string; pack: string; title: string; audience: string; bytes: number };
  let items = $state<Item[]>([]);
  let installed = $state<Installed[]>([]);
  let err = $state('');
  let loaded = $state(false);
  let timer = 0;
  let wasRunning = false;

  async function load() {
    try {
      const r = await call.get<{ items: Item[]; installed: Installed[]; error?: string }>('/webtoon/catalog');
      items = r.items;
      installed = r.installed;
      err = r.error ?? '';
      loaded = true;
      const running = items.some((i) => i.parts.some((p) => p.job?.status === 'running'));
      if (wasRunning && !running) onchange();
      wasRunning = running;
      clearTimeout(timer);
      if (running) timer = window.setTimeout(load, 1000);
    } catch (e: any) {
      err = e.message;
    }
  }
  load();
  onDestroy(() => clearTimeout(timer));

  async function install(it: Item, p: Part) {
    try {
      await call.post('/webtoon/install', { pack: it.id, part: p.index });
      wasRunning = true;
      load();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  let confirmDel = $state('');
  async function remove(dir: string) {
    if (confirmDel !== dir) {
      confirmDel = dir;
      return;
    }
    confirmDel = '';
    try {
      await call.del(`/webtoon/packs/${dir}`);
      await load();
      onchange();
    } catch (e: any) {
      onerror(e.message);
    }
  }
  const mb = (b: number) => `${Math.round(b / 1e6)} MB`;
  const audience = (a: string) => (a === 'kids' ? 'děti' : a === 'teen' ? 'teen' : a === 'adult' ? 'dospělí' : a);
</script>

{#if err}<p class="err small">{err}</p>{/if}
{#if installed.length}
  <h3>Na disku</h3>
  <ul>
    {#each installed as i (i.dir)}
      <li>
        <strong>{i.title || i.pack}</strong> <span class="muted">{i.dir} · {mb(i.bytes)}</span>
        <button class:bad={confirmDel === i.dir} onclick={() => remove(i.dir)}>{confirmDel === i.dir ? 'Opravdu smazat' : 'Smazat'}</button>
      </li>
    {/each}
  </ul>
{/if}
<h3>Katalog</h3>
{#if !loaded}<p class="muted">načítám katalog…</p>{/if}
<ul>
  {#each items as it (it.id)}
    <li>
      <strong>{it.title}</strong>
      <span class="tag">{it.kind === 'gamebook' ? 'gamebook' : it.type}</span>
      <span class="tag">{audience(it.audience)}</span>
      <span class="tag">{it.free ? 'zdarma' : 'placený v appce'}</span>
      {#each it.parts as p (p.index)}
        <span class="part">
          {it.parts.length > 1 ? `část ${p.part || p.index + 1}` : ''}
          <span class="muted">{p.stories} příběhů · {mb(p.bytes)}</span>
          {#if p.job?.status === 'running'}
            <progress max="100" value={p.job.pct}></progress> {p.job.pct} %
          {:else if p.installed}
            <span class="ok">✓ na disku</span>
          {:else}
            <button onclick={() => install(it, p)}>Stáhnout</button>
          {/if}
          {#if p.job?.status === 'failed'}<span class="err">{p.job.error}</span>{/if}
        </span>
      {/each}
    </li>
  {/each}
</ul>

<style>
  h3 {
    font-size: 0.9rem;
    margin: 8px 0 4px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 6px;
    font-size: 0.9rem;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 10px;
    align-items: center;
  }
  .tag {
    font-size: 0.75rem;
    border: 1px solid var(--line);
    border-radius: 999px;
    padding: 0 8px;
    color: var(--muted);
  }
  .part {
    display: inline-flex;
    gap: 6px;
    align-items: center;
  }
  .ok {
    color: var(--green);
  }
  .small {
    font-size: 0.85rem;
  }
  progress {
    width: 90px;
  }
</style>
