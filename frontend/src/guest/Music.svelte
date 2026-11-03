<script lang="ts">
  import { api } from '../lib/api';
  import type { MusicSnapshot, Track } from '../lib/types';

  // DJfy: search goes through the host phone (the relay never touches SoundCloud),
  // a proposal lands in DJfy's SmartQueue, votes reorder it.
  let {
    code,
    guestId,
    music,
    djfyCode,
    onmsg,
  }: { code: string; guestId: string; music: MusicSnapshot | null; djfyCode: string; onmsg: (t: string) => void } = $props();

  let q = $state('');
  let results = $state<Track[]>([]);
  let searching = $state(false);
  const base = $derived(`/api/parties/${code}/guests/${guestId}/music`);

  async function search() {
    searching = true;
    try {
      const r = await api.post<{ results: Track[] }>(`${base}/search`, { q });
      results = r.results ?? [];
      if (!results.length) onmsg('Nic nenalezeno');
    } catch (e: any) {
      onmsg(e.message);
    } finally {
      searching = false;
    }
  }

  async function propose(t: Track) {
    try {
      const r = await api.post<{ toast: string }>(`${base}/propose`, { trackId: t.id });
      onmsg(r.toast || `Přidáno: ${t.title}`);
      results = [];
      q = '';
    } catch (e: any) {
      onmsg(e.message);
    }
  }

  async function vote(entryId: string, value: 1 | -1) {
    try {
      const r = await api.post<{ toast: string }>(`${base}/vote`, { entryId, value });
      if (r.toast) onmsg(r.toast);
      navigator.vibrate?.(10);
    } catch (e: any) {
      onmsg(e.message);
    }
  }
</script>

{#if !djfyCode}
  <p class="muted">Hudba z DJfy zatím není připojená.</p>
{:else}
  {#if music?.nowPlaying}
    <div class="np">
      {#if music.nowPlaying.artworkUrl}<img src={music.nowPlaying.artworkUrl} alt="" />{/if}
      <div><strong>{music.nowPlaying.title}</strong><span>{music.nowPlaying.artist}</span></div>
    </div>
  {/if}

  <form onsubmit={(e) => { e.preventDefault(); search(); }}>
    <input placeholder="Hledat na SoundCloudu" bind:value={q} />
    <button class="primary" disabled={q.trim().length < 2 || searching}>{searching ? '…' : 'Hledat'}</button>
  </form>
  {#if results.length}
    <ul>
      {#each results as t (t.id)}
        <li>
          {#if t.artworkUrl}<img src={t.artworkUrl} alt="" />{/if}
          <div class="t"><strong>{t.title}</strong><span>{t.artist}</span></div>
          <button onclick={() => propose(t)}>+ přidat</button>
        </li>
      {/each}
    </ul>
  {/if}

  <h2>Fronta</h2>
  <ul>
    {#each music?.queue ?? [] as e (e.entryId)}
      <li class:blocked={e.blocked}>
        <div class="t"><strong>{e.track.title}</strong><span>{e.track.artist} · {e.proposedByName}</span></div>
        <button onclick={() => vote(e.entryId, 1)}>▲ {e.up}</button>
        <button onclick={() => vote(e.entryId, -1)}>▼</button>
      </li>
    {:else}
      <li class="muted">prázdná — přidej první skladbu</li>
    {/each}
  </ul>
{/if}

<style>
  form {
    display: flex;
    gap: 8px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 6px;
  }
  li {
    display: flex;
    gap: 8px;
    align-items: center;
    background: var(--panel);
    padding: 6px 8px;
    border-radius: 10px;
  }
  li.blocked {
    opacity: 0.5;
  }
  img {
    width: 44px;
    height: 44px;
    border-radius: 6px;
    object-fit: cover;
    flex: none;
  }
  .t,
  .np div {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .t strong,
  .t span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .t span,
  .np span {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .np {
    display: flex;
    gap: 10px;
    align-items: center;
  }
  h2 {
    font-size: 1rem;
    margin: 8px 0 0;
  }
</style>
