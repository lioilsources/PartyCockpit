<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, store } from '../lib/api';
  import { connect } from '../lib/ws';
  import { TapTempo } from '../lib/beat';
  import { fileUrl, type Asset, type CockpitState, type Episode, type MusicSnapshot, type Party, type SceneKind, type Tempo } from '../lib/types';

  let { code }: { code: string } = $props();

  // token: ?token=… (link from Home) or remembered per party
  const qs = new URLSearchParams(location.search);
  let token = $state(qs.get('token') ?? store.get(`pc.cockpit.${code}`) ?? '');
  if (qs.get('token')) {
    store.set(`pc.cockpit.${code}`, token);
    history.replaceState(null, '', location.pathname);
  }

  let party = $state<Party | null>(null);
  let st = $state<CockpitState | null>(null);
  let queue = $state<Asset[]>([]);
  let music = $state<MusicSnapshot | null>(null);
  let tempo = $state<Tempo>({ bpm: 0, source: 'none', anchor: 0 });
  let online = $state(false);
  let err = $state('');
  let tokenInput = $state('');
  let stop: (() => void) | null = null;

  let reactions = $state<string[]>([]);

  function start() {
    stop?.();
    stop = connect(
      code,
      'cockpit',
      (m) => {
        if (m.type === 'party') party = m.data;
        else if (m.type === 'cockpit') st = m.data;
        else if (m.type === 'queue') queue = m.data ?? [];
        else if (m.type === 'music') music = m.data;
        else if (m.type === 'tempo') tempo = m.data;
        else if (m.type === 'reaction') reactions = [m.data.emoji, ...reactions].slice(0, 14);
      },
      (o) => (online = o),
      token,
    );
  }

  // check the token once, then go live
  async function login() {
    try {
      await api.get(`/api/parties/${code}/cockpit`, token);
      store.set(`pc.cockpit.${code}`, token);
      err = '';
      start();
    } catch (e: any) {
      err = e.status === 401 ? 'Neplatný cockpit token' : e.message;
      token = '';
    }
  }
  if (token) login();
  onDestroy(() => stop?.());

  async function act(fn: () => Promise<unknown>) {
    try {
      err = '';
      await fn();
    } catch (e: any) {
      err = e.message;
    }
  }

  const p = (path: string) => `/api/parties/${code}${path}`;

  // ── scenes
  const scenes: { kind: SceneKind; label: string }[] = [
    { kind: 'slideshow', label: '🖼 Slideshow' },
    { kind: 'gallery', label: '📸 Galerie hostů' },
    { kind: 'stickers', label: '✍️ Vzkazy' },
    { kind: 'nowplaying', label: '🎵 Now playing' },
    { kind: 'webtoon', label: '📖 Webtoon' },
    { kind: 'lottery', label: '🎲 Losování' },
    { kind: 'blank', label: '✨ Mezihra' },
  ];
  let slideStyle = $state('');
  let beats = $state(8);
  let episodes = $state<Episode[]>([]);
  let episode = $state('');
  api.get<Episode[]>('/api/webtoon').then((e) => (episodes = e)).catch(() => {});

  function setScene(kind: SceneKind) {
    const params: Record<string, unknown> = { beats };
    if (kind === 'slideshow' && slideStyle) params.style = slideStyle;
    if (kind === 'webtoon' && episode) params.episode = episode;
    act(() => api.post(p('/scene'), { kind, params }, token));
  }

  // ── moderation
  const pending = $derived((st?.assets ?? []).filter((a) => a.status === 'pending'));
  const processing = $derived((st?.assets ?? []).filter((a) => a.status === 'processing'));
  const failed = $derived((st?.assets ?? []).filter((a) => a.status === 'failed'));
  const rejected = $derived((st?.assets ?? []).filter((a) => a.status === 'rejected').slice(0, 12));

  const approve = (a: Asset) => act(() => api.post(p(`/assets/${a.id}/approve`), {}, token));
  const reject = (a: Asset) => act(() => api.post(p(`/assets/${a.id}/reject`), {}, token));
  const approveAll = () => act(async () => { for (const a of pending) await api.post(p(`/assets/${a.id}/approve`), {}, token); });

  function move(i: number, d: number) {
    const ids = queue.map((a) => a.id);
    const j = i + d;
    if (j < 0 || j >= ids.length) return;
    [ids[i], ids[j]] = [ids[j], ids[i]];
    act(() => api.put(p('/queue'), { ids }, token));
  }

  // ── settings
  const patch = (body: Record<string, unknown>) => act(() => api.patch(p(''), body, token));
  let djfyCode = $state('');
  $effect(() => {
    if (party && !djfyCode) djfyCode = party.djfyCode;
  });

  // ── tempo
  const tapper = new TapTempo();
  let taps = $state(0);
  function tap() {
    const r = tapper.tap();
    taps = tapper.count;
    if (r) act(() => api.post(p('/tempo'), r, token));
  }

  // ── lottery
  let lotCount = $state(2);
  let lotGame = $state('');
  const players = $derived((st?.guests ?? []).filter((g) => g.wantsToPlay));

  function panic() {
    act(() => api.post(p('/panic'), {}, token));
  }

  const ago = (t: number) => {
    const s = Math.round((Date.now() - t) / 1000);
    return s < 60 ? `${s} s` : `${Math.round(s / 60)} min`;
  };
</script>

{#if !online && !party}
  <main class="login">
    <h1>Cockpit <code>{code}</code></h1>
    {#if !token}
      <form onsubmit={(e) => { e.preventDefault(); token = tokenInput.trim(); login(); }}>
        <input placeholder="Cockpit token" bind:value={tokenInput} type="password" />
        <button class="primary">Přihlásit</button>
      </form>
    {:else}
      <p class="muted">připojuji…</p>
    {/if}
    {#if err}<p class="err">{err}</p>{/if}
  </main>
{:else if party}
  <main class="ck">
    <header>
      <div>
        <h1>{party.name} <code>{party.code}</code></h1>
        <span class="dot" class:on={online}></span>
        <span class="muted">
          plátno {st?.clients?.stage ?? 0} · hosté {st?.guests.length ?? 0} (online {st?.clients?.guest ?? 0}) · restyle: {st?.config.restyle}
        </span>
      </div>
      <div class="links">
        <a href={`/stage/${code}`} target="_blank">Otevřít plátno ↗</a>
        <a href={party.joinUrl} target="_blank">{party.joinUrl}</a>
      </div>
      <button class="bad panic" onclick={panic}>PANIC</button>
    </header>
    {#if err}<p class="err">{err}</p>{/if}

    <section class="scenes">
      <h2>Scéna na plátně: <strong>{party.scene.kind}</strong></h2>
      <div class="row">
        {#each scenes as s (s.kind)}
          <button class:on={party.scene.kind === s.kind} onclick={() => setScene(s.kind)}>{s.label}</button>
        {/each}
      </div>
      <div class="row small">
        <label>přepnout každých <input type="number" min="2" max="64" bind:value={beats} /> dob</label>
        <label>styl slideshow <input placeholder="(všechny) styleId" bind:value={slideStyle} /></label>
        {#if episodes.length}
          <label>epizoda
            <select bind:value={episode}>
              {#each episodes as e (e.id)}<option value={e.id}>{e.title}</option>{/each}
            </select>
          </label>
        {/if}
      </div>
    </section>

    <div class="grid">
      <section>
        <h2>
          Ke schválení ({pending.length})
          {#if pending.length > 1}<button class="ok" onclick={approveAll}>Schválit vše</button>{/if}
        </h2>
        <div class="row small">
          <button class:on={party.mode === 'strict'} onclick={() => patch({ mode: 'strict' })}>strict</button>
          <button class:on={party.mode === 'relaxed'} onclick={() => patch({ mode: 'relaxed' })}>relaxed</button>
          <span class="muted">
            {party.mode === 'relaxed'
              ? st?.config.moderation === 'none'
                ? 'relaxed bez klasifikátoru = stejně ručně'
                : `auto-schválení při NSFW ≤ ${st?.config.autoApproveMax}`
              : 'vše ručně'}
          </span>
        </div>
        <div class="cards">
          {#each pending as a (a.id)}
            <div class="card">
              {#if a.kind === 'sticker'}
                <div class="sticker" style:background={a.meta.color}>{a.meta.text}</div>
              {:else}
                <img src={fileUrl(code, a.file)} alt="" />
              {/if}
              <div class="meta">
                <strong>{a.meta.nickname}</strong>
                {#if a.meta.styleLabel}<span>{a.meta.styleLabel}</span>{/if}
                {#if a.meta.fallback}<span class="warn">originál (restyle selhal)</span>{/if}
                {#if a.nsfwScore >= 0}<span class:warn={a.nsfwScore > 0.15}>NSFW {a.nsfwScore.toFixed(2)}</span>{/if}
              </div>
              <div class="row">
                <button class="ok" onclick={() => approve(a)}>✓</button>
                <button class="bad" onclick={() => reject(a)}>✕</button>
              </div>
            </div>
          {:else}
            <p class="muted">nic nečeká</p>
          {/each}
        </div>
        {#if processing.length}
          <p class="muted">maluje se: {processing.map((a) => `${a.meta.nickname} (${ago(a.createdAt)})`).join(', ')}</p>
        {/if}
        {#if failed.length}<p class="err">selhalo: {failed.length}</p>{/if}
        {#if rejected.length}
          <details>
            <summary class="muted">zamítnuté ({rejected.length})</summary>
            <div class="cards">
              {#each rejected as a (a.id)}
                <div class="card mini">
                  {#if a.kind === 'sticker'}<div class="sticker" style:background={a.meta.color}>{a.meta.text}</div>{:else}<img src={fileUrl(code, a.file)} alt="" />{/if}
                  <button onclick={() => approve(a)}>vrátit</button>
                </div>
              {/each}
            </div>
          </details>
        {/if}
      </section>

      <section>
        <h2>Show fronta ({queue.length})</h2>
        <div class="row small">
          <span class="muted">po doběhnutí:</span>
          <button class:on={party.loopMode === 'loop'} onclick={() => patch({ loopMode: 'loop' })}>od začátku</button>
          <button class:on={party.loopMode === 'shuffle'} onclick={() => patch({ loopMode: 'shuffle' })}>shuffle</button>
        </div>
        <ol class="queue">
          {#each queue as a, i (a.id)}
            <li>
              {#if a.kind === 'sticker'}<span class="qs" style:background={a.meta.color}>✍️</span>{:else}<img src={fileUrl(code, a.file)} alt="" />{/if}
              <span class="qn">{a.meta.nickname} · {a.kind === 'sticker' ? a.meta.text : a.meta.styleLabel}</span>
              <button onclick={() => move(i, -1)}>↑</button>
              <button onclick={() => move(i, 1)}>↓</button>
              <button class="bad" onclick={() => act(() => api.del(p(`/queue/${a.id}`), token))}>✕</button>
            </li>
          {:else}
            <li class="muted">prázdná — schválený obsah se sem řadí</li>
          {/each}
        </ol>
      </section>

      <section>
        <h2>Hudba (DJfy)</h2>
        <form class="row" onsubmit={(e) => { e.preventDefault(); patch({ djfyCode }); }}>
          <input placeholder="kód DJfy party" bind:value={djfyCode} style="max-width: 160px" />
          <button>Připojit</button>
          {#if party.djfyCode}<button onclick={() => { djfyCode = ''; patch({ djfyCode: '' }); }}>Odpojit</button>{/if}
        </form>
        {#if music?.nowPlaying}
          <p>▶ <strong>{music.nowPlaying.title}</strong> — {music.nowPlaying.artist} <span class="muted">· fronta {music.queue.length} · hosté DJfy {music.guests}</span></p>
        {:else if party.djfyCode}
          <p class="muted">čekám na snapshot z telefonu s DJfy…</p>
        {/if}
        <div class="row">
          <button class="tap" onclick={tap}>TAP {taps > 0 && taps < 4 ? `(${taps}/4)` : ''}</button>
          <span>
            {tempo.bpm > 0 ? `${tempo.bpm} BPM` : 'bez tempa'}
            <span class="muted">({tempo.source === 'tap' ? 'tap' : tempo.source === 'djfy' ? 'z DJfy' : 'plátno poslouchá audio / idle'})</span>
          </span>
          {#if tempo.source === 'tap'}<button onclick={() => act(() => api.post(p('/tempo'), { bpm: 0 }, token))}>zrušit tap</button>{/if}
        </div>
        {#if reactions.length}<p class="react">{reactions.join(' ')}</p>{/if}
      </section>

      <section>
        <h2>Losování hráčů</h2>
        <p class="muted">chtějí hrát: {players.length ? players.map((g) => g.nickname).join(', ') : 'nikdo (losuje se ze všech)'}</p>
        <form class="row" onsubmit={(e) => { e.preventDefault(); act(() => api.post(p('/lottery'), { count: lotCount, game: lotGame }, token)); }}>
          <input placeholder="hra (např. Kiriana co-op)" bind:value={lotGame} />
          <input type="number" min="1" max="8" bind:value={lotCount} style="max-width: 70px" />
          <button class="primary">Losovat</button>
        </form>
      </section>

      <section>
        <h2>
          Náměty na webtoon ({st?.ideas.length ?? 0})
          <a href={`${p('/ideas')}?download=1&token=${token}`} download>stáhnout JSON</a>
        </h2>
        <ul class="ideas">
          {#each [...(st?.ideas ?? [])].reverse() as i (i.id)}
            <li><strong>{i.nickname}:</strong> {i.text}</li>
          {:else}
            <li class="muted">zatím žádné</li>
          {/each}
        </ul>
      </section>
    </div>
  </main>
{/if}

<style>
  main.login {
    max-width: 420px;
    margin: 15vh auto;
    padding: 0 16px;
    display: grid;
    gap: 12px;
  }
  .login form {
    display: grid;
    gap: 10px;
  }
  .ck {
    padding: 16px;
    max-width: 1500px;
    margin: 0 auto;
  }
  header {
    display: flex;
    gap: 16px;
    align-items: center;
    flex-wrap: wrap;
    justify-content: space-between;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
  }
  code {
    color: var(--yellow);
  }
  .links {
    display: flex;
    flex-direction: column;
    font-size: 0.9rem;
  }
  .panic {
    font-weight: 700;
    font-size: 1.1rem;
    padding: 0.7em 1.6em;
  }
  .dot {
    display: inline-block;
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--red);
    margin-right: 6px;
  }
  .dot.on {
    background: var(--green);
  }
  section {
    background: var(--bg-2);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 14px;
    margin-top: 14px;
    min-width: 0;
  }
  h2 {
    font-size: 0.95rem;
    margin: 0 0 10px;
    display: flex;
    gap: 10px;
    align-items: center;
    flex-wrap: wrap;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  h2 strong {
    color: var(--yellow);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 440px), 1fr));
    gap: 0 14px;
  }
  .row {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    align-items: center;
    margin-bottom: 8px;
  }
  .row.small {
    font-size: 0.88rem;
  }
  .row label {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .row label input {
    width: 7em;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 10px;
  }
  .card {
    background: var(--panel);
    border-radius: 12px;
    padding: 8px;
    display: grid;
    gap: 6px;
  }
  .card img {
    width: 100%;
    aspect-ratio: 1;
    object-fit: cover;
    border-radius: 8px;
  }
  .card.mini img {
    aspect-ratio: 1;
  }
  .sticker {
    font-family: var(--hand);
    color: #1d1208;
    font-size: 1.4rem;
    padding: 10px;
    border-radius: 6px;
    min-height: 90px;
    word-break: break-word;
  }
  .meta {
    display: flex;
    flex-direction: column;
    font-size: 0.82rem;
  }
  .warn {
    color: var(--yellow);
  }
  .queue {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 6px;
    max-height: 420px;
    overflow: auto;
  }
  .queue li {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .queue img,
  .qs {
    width: 40px;
    height: 40px;
    border-radius: 6px;
    object-fit: cover;
    display: grid;
    place-items: center;
    flex: none;
  }
  .qn {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.9rem;
  }
  .queue button {
    padding: 0.3em 0.6em;
  }
  .tap {
    font-weight: 700;
    min-width: 120px;
    padding: 1em;
    background: var(--pink);
    border-color: var(--pink);
  }
  .react {
    font-size: 1.4rem;
  }
  .ideas {
    padding-left: 18px;
    max-height: 300px;
    overflow: auto;
    font-size: 0.92rem;
  }
  .ideas li {
    margin-bottom: 6px;
  }
</style>
