<script lang="ts">
  import { plural } from '../lib/types';
  import type { StudioApi } from './kinds';

  // "Is it all on this laptop?" (D23): every entry of the show checked against
  // the disk, and the caches filled from the uplink while it is still there.
  let { call, onerror }: { call: StudioApi; onerror: (m: string) => void } = $props();

  type Item = { entryId: string; title: string; kind: string; status: 'ok' | 'warn' | 'fail'; detail: string };
  let items = $state<Item[]>([]);
  let ok = $state(0);
  let processing = $state(0);
  let checked = $state(false);
  let busy = $state('');

  async function check() {
    busy = 'kontroluju…';
    try {
      const r = await call.get<{ items: Item[]; ok: number; processing: number }>('/show/ready');
      items = r.items;
      ok = r.ok;
      processing = r.processing;
      checked = true;
    } catch (e: any) {
      onerror(e.message);
    } finally {
      busy = '';
    }
  }
  async function prefetch() {
    busy = 'stahuju do cache…';
    try {
      const r = await call.post<{ done: number; failed: number }>('/show/prefetch');
      if (r.failed) onerror(`${r.failed} položek se stáhnout nepodařilo`);
    } catch (e: any) {
      onerror(e.message);
    } finally {
      busy = '';
    }
    check();
  }
</script>

<div class="row">
  <button onclick={check} disabled={!!busy}>Zkontrolovat offline</button>
  <button onclick={prefetch} disabled={!!busy}>Stáhnout do cache</button>
  {#if busy}<span class="muted">{busy}</span>{/if}
</div>
{#if checked}
  <p class:good={ok === items.length && !processing} class:bad={ok < items.length}>
    {ok}/{items.length} {plural(items.length, 'scéna poběží', 'scény poběží', 'scén poběží')} bez sítě{processing ? ` · na GPU ještě čeká ${processing}` : ''}
  </p>
  <ul>
    {#each items as it (it.entryId)}
      <li class={it.status}>
        <span>{it.status === 'ok' ? '✅' : it.status === 'warn' ? '⚠️' : '❌'}</span>
        <strong>{it.title || it.kind}</strong>
        <span class="muted">{it.detail}</span>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .row {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 4px;
    font-size: 0.88rem;
  }
  li {
    display: flex;
    gap: 8px;
  }
  .good {
    color: var(--green);
  }
  .bad {
    color: var(--red);
  }
</style>
