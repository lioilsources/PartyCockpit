<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, store } from '../lib/api';
  import { connect, type Socket } from '../lib/ws';
  import { listPacks, type PackInfo } from '../lib/stylemap';
  import { plural, type Asset, type CockpitState, type Episode, type Party, type SceneKind, type ShowEntry, type ShowState } from '../lib/types';
  import EntryEditor from './EntryEditor.svelte';
  import Photos from './Photos.svelte';
  import WebtoonLibrary from './WebtoonLibrary.svelte';
  import StyleMapBuilder from './StyleMapBuilder.svelte';
  import Ready from './Ready.svelte';
  import ShowLibrary from './ShowLibrary.svelte';
  import { KINDS, kindLabel, newEntry, studioApi } from './kinds';

  // Scene Studio (D23): the show is prepared here ahead of the party, the GPU
  // work is done once and kept as files, and on the night the server plays the
  // show scene by scene, offline, the same way every time.
  let { code }: { code: string } = $props();

  // the cockpit token, like the Cockpit (D19): #token= once, then localStorage
  const fromUrl = new URLSearchParams(location.hash.slice(1)).get('token');
  let token = $state(fromUrl ?? store.get(`pc.cockpit.${code}`) ?? '');
  if (fromUrl) {
    store.set(`pc.cockpit.${code}`, token);
    history.replaceState(null, '', location.pathname);
  }
  const call = studioApi(code, () => token);

  let party = $state<Party | null>(null);
  let st = $state<CockpitState | null>(null);
  let queue = $state<Asset[]>([]);
  let show = $state<ShowState | null>(null);
  let online = $state(false);
  let err = $state('');
  let tokenInput = $state('');
  let sock: Socket | null = null;

  // the editable copy of the show; the server's copy wins only while nothing is unsaved
  let name = $state('');
  let loop = $state(true);
  let entries = $state<ShowEntry[]>([]);
  let selected = $state('');
  let dirty = false;
  let saveTimer = 0;
  let saving = $state(false);

  function adopt(s: ShowState) {
    show = s;
    if (dirty) return;
    name = s.name;
    loop = s.loop;
    entries = structuredClone($state.snapshot(s.entries)) as ShowEntry[];
    if (!entries.some((e) => e.id === selected)) selected = entries[0]?.id ?? '';
  }

  function start() {
    sock?.close();
    sock = connect(
      code,
      'cockpit',
      (m) => {
        if (m.type === 'party') party = m.data;
        else if (m.type === 'cockpit') st = m.data;
        else if (m.type === 'queue') queue = m.data ?? [];
        else if (m.type === 'show') adopt(m.data as ShowState);
      },
      (o) => (online = o),
      async () => ({ ticket: (await call.post<{ ticket: string }>('/cockpit/ticket')).ticket }),
    );
  }
  async function login() {
    try {
      await call.get('/cockpit');
      store.set(`pc.cockpit.${code}`, token);
      err = '';
      start();
    } catch (e: any) {
      err = e.status === 401 ? 'Neplatný cockpit token' : e.message;
      token = '';
    }
  }
  if (token) login();

  let now = $state(Date.now());
  const tick = setInterval(() => (now = Date.now()), 1000);
  onDestroy(() => {
    sock?.close();
    clearInterval(tick);
    clearTimeout(saveTimer);
  });

  // ── content the editor picks from
  let episodes = $state<Episode[]>([]);
  let packs = $state<PackInfo[]>([]);
  function refresh() {
    api.get<Episode[]>('/api/webtoon').then((e) => (episodes = e)).catch(() => {});
    listPacks().then((p) => (packs = p)).catch(() => {});
  }
  refresh();
  const photos = $derived(queue.filter((a) => (a.kind === 'restyle' || a.kind === 'upload') && a.file));

  // ── editing
  function changed() {
    dirty = true;
    clearTimeout(saveTimer);
    saveTimer = window.setTimeout(save, 500);
  }
  async function save() {
    saving = true;
    try {
      const s = await call.put<ShowState>('/show', { name, loop, entries: $state.snapshot(entries) });
      dirty = false;
      adopt(s);
    } catch (e: any) {
      err = e.message;
    } finally {
      saving = false;
    }
  }
  let addKind = $state<SceneKind>('webtoon');
  function add() {
    const e = newEntry(addKind);
    entries = [...entries, e];
    selected = e.id;
    changed();
  }
  function move(i: number, d: number) {
    const j = i + d;
    if (j < 0 || j >= entries.length) return;
    const next = [...entries];
    [next[i], next[j]] = [next[j], next[i]];
    entries = next;
    changed();
  }
  let confirmDel = $state('');
  function remove(id: string) {
    if (confirmDel !== id) {
      confirmDel = id;
      return;
    }
    confirmDel = '';
    entries = entries.filter((e) => e.id !== id);
    if (selected === id) selected = entries[0]?.id ?? '';
    changed();
  }
  function duplicate(i: number) {
    const copy = { ...structuredClone($state.snapshot(entries[i])), id: Math.random().toString(36).slice(2, 10) } as ShowEntry;
    entries = [...entries.slice(0, i + 1), copy, ...entries.slice(i + 1)];
    selected = copy.id;
    changed();
  }
  const selIndex = $derived(entries.findIndex((e) => e.id === selected));
  const total = $derived(entries.reduce((n, e) => n + (e.seconds || 0), 0));
  const fmt = (s: number) => `${Math.floor(s / 60)}:${String(Math.round(s % 60)).padStart(2, '0')}`;

  // ── player
  async function control(action: 'play' | 'pause' | 'next' | 'prev', index?: number) {
    try {
      if (dirty) await save();
      adopt(await call.post<ShowState>(`/show/${action}`, index === undefined ? {} : { index }));
    } catch (e: any) {
      err = e.message;
    }
  }
  const playingEntry = $derived(show?.playing ? (show.entries[show.index] ?? null) : null);
  const left = $derived(show?.playing ? Math.max(0, (show.endsAt - now) / 1000) : 0);

  // ── preview: the stage in an iframe, drawing the selected entry
  const previewUrl = $derived(st && selected ? `/stage/${code}?key=${st.config.stageKey}&preview=${selected}` : '');
  let previewOn = $state(true);

  let tab = $state<'photos' | 'webtoon' | 'stylemap' | 'ready' | 'library'>('photos');
  const onerror = (m: string) => (err = m);
</script>

{#if !token}
  <main class="login">
    <h1>🎬 Studio</h1>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        token = tokenInput;
        login();
      }}
    >
      <input type="password" placeholder="cockpit token" bind:value={tokenInput} />
      <button class="primary">Otevřít</button>
    </form>
    {#if err}<p class="err">{err}</p>{/if}
  </main>
{:else}
  <main>
    <header>
      <h1>🎬 Studio <span class="muted">· {party?.name ?? code}</span></h1>
      <span class="dot" class:on={online}></span>
      <div class="player">
        <button onclick={() => control('prev')} title="předchozí scéna">⏮</button>
        {#if show?.playing}
          <button class="on" onclick={() => control('pause')}>⏸ pauza</button>
        {:else}
          <button class="primary" disabled={!entries.length} onclick={() => control('play', Math.max(0, selIndex))}>▶ přehrát od vybrané</button>
        {/if}
        <button onclick={() => control('next')} title="další scéna">⏭</button>
        <label><input type="checkbox" bind:checked={loop} onchange={changed} /> dokola</label>
        <span class="muted">
          {#if playingEntry}hraje {show!.index + 1}/{show!.entries.length} · {playingEntry.title || kindLabel(playingEntry.kind)} · zbývá {fmt(left)}{:else}stojí{/if}
        </span>
      </div>
      <nav>
        <a href={`/cockpit/${code}`} target="_blank">Cockpit ↗</a>
        {#if st}<a href={`/stage/${code}?key=${st.config.stageKey}&hud=1`} target="_blank">Plátno ↗</a>{/if}
      </nav>
    </header>
    {#if err}<p class="err">{err} <button onclick={() => (err = '')}>✕</button></p>{/if}

    <div class="layout">
      <section class="show">
        <h2>Show <span class="muted">{entries.length} {plural(entries.length, 'scéna', 'scény', 'scén')} · {fmt(total)}{saving ? ' · ukládám…' : ''}</span></h2>
        <input placeholder="název show (např. Ema 40)" bind:value={name} oninput={changed} />
        <ol>
          {#each entries as e, i (e.id)}
            <li class:sel={e.id === selected} class:playing={show?.playing && show.entryId === e.id}>
              <button class="pick" onclick={() => (selected = e.id)}>
                <span class="i">{i + 1}</span>
                <span class="t">{e.title || kindLabel(e.kind)}{#if e.title}<span class="muted"> · {kindLabel(e.kind)}</span>{/if}</span>
                <span class="muted">{fmt(e.seconds)}</span>
              </button>
              <button onclick={() => control('play', i)} title="přehrát odsud">▶</button>
              <button onclick={() => move(i, -1)}>↑</button>
              <button onclick={() => move(i, 1)}>↓</button>
              <button onclick={() => duplicate(i)} title="zkopírovat">⧉</button>
              <button class:bad={confirmDel === e.id} onclick={() => remove(e.id)}>{confirmDel === e.id ? 'smazat?' : '✕'}</button>
            </li>
          {:else}
            <li class="muted">Prázdná show. Přidej první scénu.</li>
          {/each}
        </ol>
        <div class="add">
          <select bind:value={addKind}>
            {#each KINDS as k (k.kind)}<option value={k.kind}>{k.label}</option>{/each}
          </select>
          <button class="primary" onclick={add} title="přidá scénu na konec show">+ Přidat</button>
        </div>
      </section>

      <section class="edit">
        {#if selIndex >= 0 && st}
          <EntryEditor bind:entry={entries[selIndex]} {code} fileKey={st.config.fileKey} {episodes} {packs} {photos} onchange={changed} />
          <div class="preview-head">
            <label><input type="checkbox" bind:checked={previewOn} /> náhled</label>
            <span class="muted small">plátno kreslí vybranou scénu od začátku; změna ji spustí znovu</span>
          </div>
          {#if previewOn && previewUrl}
            {#key previewUrl}
              <div class="preview"><iframe title="náhled scény" src={previewUrl}></iframe></div>
            {/key}
          {/if}
        {:else}
          <p class="muted">Vyber scénu vlevo, nebo přidej novou.</p>
        {/if}
      </section>

      <section class="prep">
        <nav class="tabs">
          <button class:on={tab === 'photos'} onclick={() => (tab = 'photos')}>📸 Fotky</button>
          <button class:on={tab === 'webtoon'} onclick={() => (tab = 'webtoon')}>📚 Webtoon</button>
          <button class:on={tab === 'stylemap'} onclick={() => (tab = 'stylemap')}>🧪 StyleMap</button>
          <button class:on={tab === 'ready'} onclick={() => (tab = 'ready')}>✅ Offline</button>
          <button class:on={tab === 'library'} onclick={() => (tab = 'library')}>💾 Knihovna</button>
        </nav>
        {#if tab === 'photos' && st}
          <Photos {call} {code} fileKey={st.config.fileKey} assets={st.assets} {onerror} />
        {:else if tab === 'webtoon'}
          <WebtoonLibrary {call} onchange={refresh} {onerror} />
        {:else if tab === 'stylemap'}
          <StyleMapBuilder {call} {packs} onchange={refresh} {onerror} />
        {:else if tab === 'ready'}
          <Ready {call} {onerror} />
        {:else if tab === 'library'}
          <ShowLibrary {call} {name} {onerror} />
        {/if}
      </section>
    </div>
  </main>
{/if}

<style>
  main {
    padding: 14px;
    max-width: 1700px;
    margin: 0 auto;
  }
  .login {
    max-width: 420px;
    margin-top: 12vh;
  }
  .login form {
    display: flex;
    gap: 8px;
  }
  header {
    display: flex;
    gap: 14px;
    align-items: center;
    flex-wrap: wrap;
  }
  h1 {
    margin: 0;
    font-size: 1.4rem;
  }
  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--red);
  }
  .dot.on {
    background: var(--green);
  }
  .player {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-wrap: wrap;
    flex: 1;
  }
  .player label {
    display: flex;
    gap: 4px;
    align-items: center;
  }
  header nav {
    display: flex;
    gap: 12px;
  }
  .layout {
    display: grid;
    grid-template-columns: minmax(330px, 400px) minmax(0, 1fr) minmax(320px, 420px);
    gap: 14px;
    margin-top: 14px;
    align-items: start;
  }
  @media (max-width: 1200px) {
    .layout {
      grid-template-columns: minmax(260px, 340px) minmax(0, 1fr);
    }
    .prep {
      grid-column: 1 / -1;
    }
  }
  @media (max-width: 760px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
  section {
    background: var(--bg-2);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 12px;
    min-width: 0;
    display: grid;
    gap: 10px;
  }
  h2 {
    font-size: 0.95rem;
    margin: 0;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    display: flex;
    gap: 8px;
    align-items: baseline;
    flex-wrap: wrap;
  }
  h2 .muted {
    text-transform: none;
    letter-spacing: 0;
    font-size: 0.85rem;
  }
  ol {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 4px;
  }
  ol li {
    display: flex;
    gap: 4px;
    align-items: center;
  }
  ol li > button:not(.pick) {
    padding: 0.3em 0.45em;
    font-size: 0.8rem;
  }
  .pick {
    flex: 1;
    display: flex;
    gap: 8px;
    align-items: center;
    text-align: left;
    min-width: 0;
  }
  .pick .t {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pick .i {
    color: var(--muted);
    width: 1.6em;
  }
  li.sel .pick {
    border-color: var(--pink);
  }
  li.playing .pick {
    background: #12402c;
  }
  .add {
    display: flex;
    gap: 8px;
  }
  .add select {
    flex: 1;
  }
  .preview-head {
    display: flex;
    gap: 10px;
    align-items: center;
  }
  .preview {
    position: relative;
    aspect-ratio: 16 / 9;
    border-radius: 10px;
    overflow: hidden;
    background: #000;
  }
  .preview iframe {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    border: 0;
  }
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .small {
    font-size: 0.85rem;
  }
</style>
