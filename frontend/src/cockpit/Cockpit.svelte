<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, store } from '../lib/api';
  import { connect, type Socket } from '../lib/ws';
  import { TapTempo } from '../lib/beat';
  import { Viewer, srcObject, type RtcMsg } from '../lib/rtc';
  import { fileUrl, plural, recapUrl, type Asset, type Check, type CockpitState, type Episode, type MusicSnapshot, type Party, type LiveStream, type SceneKind, type SubtitleLine, type Tempo } from '../lib/types';
  import Clip from '../lib/Clip.svelte';
  import { Mic } from '../lib/mic';
  import { listPacks, type PackInfo } from '../lib/stylemap';

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
  let sock: Socket | null = null;

  let reactions = $state<string[]>([]);
  let streams = $state<LiveStream[]>([]);
  let rtcRev = $state(0);
  let viewer = $state<Viewer | null>(null);

  function start() {
    sock?.close();
    sock = connect(
      code,
      'cockpit',
      (m) => {
        if (m.type === 'party') party = m.data;
        else if (m.type === 'cockpit') st = m.data;
        else if (m.type === 'queue') queue = m.data ?? [];
        else if (m.type === 'music') music = m.data;
        else if (m.type === 'tempo') tempo = m.data;
        else if (m.type === 'reaction') reactions = [m.data.emoji, ...reactions].slice(0, 14);
        else if (m.type === 'streams') streams = m.data ?? [];
        else if (m.type === 'rtc') viewer?.handle(m.data as RtcMsg);
        else if (m.type === 'hello') viewer?.reset();
        else if (m.type === 'subtitle') subLine(m.data as SubtitleLine);
        else if (m.type === 'subtitle.clear') subs = [];
      },
      (o) => (online = o),
      { token },
    );
    viewer?.destroy();
    viewer = new Viewer(sock.send, party?.iceServers ?? [], () => rtcRev++);
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
  onDestroy(() => {
    viewer?.destroy();
    sock?.close();
    mic.stop();
  });

  // ── subtitles (M7): MC microphone → Translator → caption bar on the stage
  let subs = $state<SubtitleLine[]>([]);
  let micRev = $state(0);
  const mic = new Mic(() => micRev++);
  const micState = $derived((void micRev, mic.state));
  const micLevel = $derived((void micRev, mic.level));
  const micError = $derived((void micRev, mic.error));
  let langA = $state(store.get('pc.sub.langA') ?? 'cs');
  let langB = $state(store.get('pc.sub.langB') ?? 'en');
  let typed = $state('');
  const langs = [
    ['cs', 'čeština'],
    ['sk', 'slovenčina'],
    ['en', 'angličtina'],
    ['de', 'němčina'],
    ['uk', 'ukrajinština'],
    ['pl', 'polština'],
    ['es', 'španělština'],
  ];
  function subLine(l: SubtitleLine) {
    const i = subs.findIndex((x) => x.id === l.id);
    if (i >= 0) subs[i] = l;
    else subs = [l, ...subs].slice(0, 6);
  }
  function toggleMic() {
    if (micState === 'off' || micState === 'error') {
      store.set('pc.sub.langA', langA);
      store.set('pc.sub.langB', langB);
      mic.start(code, token, langA, langB);
    } else mic.stop();
  }
  function sendTyped(e: Event) {
    e.preventDefault();
    const text = typed.trim();
    if (!text) return;
    act(async () => {
      await api.post(p('/subtitle'), { text }, token);
      typed = '';
    });
  }

  // ── live streams: the cockpit previews every phone, pending ones included,
  // so the operator sees who they let on the wall
  let previews = $state(true);
  $effect(() => {
    // read the inputs first: `viewer?.` would skip them and the effect would never re-run
    const ice = party?.iceServers;
    const want = previews ? streams : [];
    viewer?.setIce(ice);
    viewer?.sync(want);
  });
  const streamMedia = $derived.by(() => {
    void rtcRev;
    return new Map(streams.map((s) => [s.id, viewer?.media(s.id)]));
  });
  const liveCount = $derived(streams.filter((s) => s.status === 'live').length);

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
    { kind: 'pipwall', label: '📡 Živě (streamy)' },
    { kind: 'dance', label: '💃 Tančírna' },
    { kind: 'interlude', label: '🌀 Shader přestávka' },
    { kind: 'stylesweep', label: '🎨 StyleMap' },
    { kind: 'recap', label: '🎞 Recap (konec)' },
    { kind: 'blank', label: '✨ Mezihra' },
  ];
  let shader = $state('');
  let packs = $state<PackInfo[]>([]);
  let pack = $state(store.get('pc.stylemap') ?? '');
  let sweepTransition = $state(store.get('pc.stylemap.transition') === '1');
  listPacks()
    .then((ps) => {
      packs = ps;
      if (!ps.some((x) => x.id === pack)) pack = ps[0]?.id ?? '';
    })
    .catch(() => {});
  $effect(() => {
    store.set('pc.stylemap', pack);
    store.set('pc.stylemap.transition', sweepTransition ? '1' : '0');
  });
  let slideStyle = $state('');
  let beats = $state(8);
  let episodes = $state<Episode[]>([]);
  let episode = $state('');
  api.get<Episode[]>('/api/webtoon').then((e) => (episodes = e)).catch(() => {});

  function setScene(kind: SceneKind) {
    const params: Record<string, unknown> = { beats };
    if (kind === 'slideshow' && slideStyle) params.style = slideStyle;
    if (kind === 'webtoon' && episode) params.episode = episode;
    if (kind === 'stylesweep' && pack) params.pack = pack;
    else if (sweepTransition && pack) params.transition = pack;
    if (kind === 'interlude') {
      params.beats = Math.max(beats, 16);
      if (shader) params.shader = shader;
    }
    act(() => api.post(p('/scene'), { kind, params }, token));
  }

  // ── preflight (M7): live checks from the server + the manual runbook
  let checks = $state<Check[]>([]);
  let checking = $state(false);
  async function preflight() {
    checking = true;
    try {
      checks = (await api.get<{ checks: Check[] }>(p('/preflight'), token)).checks;
    } catch (e: any) {
      err = e.message;
    } finally {
      checking = false;
    }
  }
  const manual = [
    { id: 'audio', label: 'Zvuk do plátna: loopback (BlackHole / VB-Cable) nebo mikrofon, plátno s ?audio=1' },
    { id: 'kiosk', label: 'Plátno v kiosku na celou obrazovku, spořič a uspávání vypnuté' },
    { id: 'qr', label: 'QR na plátně vyzkoušené vlastním telefonem přes Wi-Fi hostů' },
    { id: 'tablet', label: 'Cockpit na tabletu, nabitý, PANIC vyzkoušený' },
    { id: 'gpu', label: 'GPU okno domluvené (L40S ComfyUI na animace, Restyler na fotky)' },
    { id: 'djfy', label: 'DJfy hraje z hostitelského telefonu, kód zadaný v Cockpitu' },
  ];
  const runKey = `pc.runbook.${code}`;
  let done = $state<Record<string, boolean>>(JSON.parse(store.get(runKey) ?? '{}'));
  function toggle(id: string) {
    done = { ...done, [id]: !done[id] };
    store.set(runKey, JSON.stringify(done));
  }
  const problems = $derived(checks.filter((c) => c.status === 'fail').length);
  const warnings = $derived(checks.filter((c) => c.status === 'warn').length);

  // ── moderation
  const pending = $derived((st?.assets ?? []).filter((a) => a.status === 'pending'));
  const processing = $derived((st?.assets ?? []).filter((a) => a.status === 'processing' && a.kind !== 'live'));
  const animating = $derived((st?.assets ?? []).filter((a) => a.status === 'processing' && a.kind === 'live'));
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
          plátno {st?.clients?.stage ?? 0} · hosté {st?.guests.length ?? 0} (online {st?.clients?.guest ?? 0}) · restyle: {st?.config.restyle} · animace: {st?.config.animate}
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
        <label>shader
          <select bind:value={shader}>
            <option value="">(střídat)</option>
            <option value="plasma">Plazma</option>
            <option value="tunnel">Tunel</option>
            <option value="kaleido">Kaleidoskop</option>
            <option value="bars">Ekvalizér</option>
            <option value="aurora">Polární záře</option>
          </select>
        </label>
        {#if packs.length}
          <label>StyleMap
            <select bind:value={pack}>
              {#each packs as x (x.id)}<option value={x.id}>{x.title} ({x.n})</option>{/each}
            </select>
          </label>
          <label><input type="checkbox" bind:checked={sweepTransition} /> přechod přes StyleMap</label>
        {/if}
        <a href={recapUrl(party.joinUrl)} target="_blank">recap pro hosty ↗</a>
        <label>epizoda
          <select bind:value={episode}>
            {#each episodes as e (e.id)}<option value={e.id}>{e.title}</option>{/each}
            <option value="party">🎉 Party epizoda (z galerie)</option>
          </select>
        </label>
      </div>
    </section>

    <details class="preflight" ontoggle={(e) => (e.currentTarget as HTMLDetailsElement).open && !checks.length && preflight()}>
      <summary>
        ✅ Před party
        {#if checks.length}
          <span class:bad={problems > 0} class:warn={!problems && warnings > 0}>
            {problems ? `${problems} ${plural(problems, 'problém', 'problémy', 'problémů')}` : warnings ? `${warnings} ${plural(warnings, 'upozornění', 'upozornění', 'upozornění')}` : 'v pořádku'}
          </span>
        {/if}
      </summary>
      <div class="pf">
        <div>
          <h3>Automatické kontroly <button onclick={preflight} disabled={checking}>{checking ? '…' : '↻'}</button></h3>
          <ul>
            {#each checks as c (c.id)}
              <li class={c.status}><b>{c.label}</b> {c.detail}</li>
            {/each}
          </ul>
        </div>
        <div>
          <h3>Ručně</h3>
          <ul>
            {#each manual as m (m.id)}
              <li><label><input type="checkbox" checked={!!done[m.id]} onchange={() => toggle(m.id)} /> {m.label}</label></li>
            {/each}
          </ul>
        </div>
      </div>
    </details>

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
                <Clip src={fileUrl(code, a.file)} />
              {/if}
              <div class="meta">
                <strong>{a.meta.nickname}</strong>
                {#if a.kind === 'dance'}<span>💃 tanec {a.meta.motionLabel}</span>{/if}
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
          <label><input type="checkbox" checked={party.autoLive} onchange={(e) => patch({ autoLive: e.currentTarget.checked })} /> oživovat schválené fotky</label>
          <button onclick={() => act(async () => { const r = await api.post<{ queued: number }>(p('/animate'), {}, token); err = r.queued ? '' : 'všechny fotky už živé jsou (nebo se oživují)'; })}>✨ Oživit galerii</button>
          {#if animating.length}<span class="muted">oživuje se {animating.length}</span>{/if}
        </div>
        <div class="row small">
          <span class="muted">po doběhnutí:</span>
          <button class:on={party.loopMode === 'loop'} onclick={() => patch({ loopMode: 'loop' })}>od začátku</button>
          <button class:on={party.loopMode === 'shuffle'} onclick={() => patch({ loopMode: 'shuffle' })}>shuffle</button>
        </div>
        <ol class="queue">
          {#each queue as a, i (a.id)}
            <li>
              {#if a.kind === 'sticker'}<span class="qs" style:background={a.meta.color}>✍️</span>{:else}<Clip src={fileUrl(code, a.live ?? a.file)} />{/if}
              <span class="qn">{a.meta.nickname} · {a.kind === 'sticker' ? a.meta.text : a.kind === 'dance' ? `💃 ${a.meta.motionLabel}` : a.meta.styleLabel}{a.live ? ' · ✨ živá' : ''}</span>
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
        <h2>
          Živé streamy ({liveCount}/9 na plátně)
          <label class="small muted"><input type="checkbox" bind:checked={previews} /> náhledy</label>
        </h2>
        <div class="cards">
          {#each streams as s (s.id)}
            <div class="card stream" class:live={s.status === 'live'}>
              {#if streamMedia.get(s.id)}
                <!-- svelte-ignore a11y_media_has_caption -->
                <video use:srcObject={streamMedia.get(s.id)} autoplay playsinline muted></video>
              {:else}
                <div class="novid">{s.online ? (previews ? 'připojuji…' : 'bez náhledu') : 'offline'}</div>
              {/if}
              <div class="meta">
                <strong>{s.name}</strong>
                <span>{s.source === 'mirrorbooth' ? '🪞 MirrorBooth' : '📡 prohlížeč'} · {s.status === 'live' ? '🔴 na plátně' : 'čeká'}</span>
              </div>
              <div class="row">
                {#if s.status === 'live'}
                  <button onclick={() => act(() => api.post(p(`/streams/${s.id}/hold`), {}, token))}>stáhnout</button>
                {:else}
                  <button class="ok" onclick={() => act(() => api.post(p(`/streams/${s.id}/approve`), {}, token))}>✓ pustit</button>
                {/if}
                <button class="bad" onclick={() => act(() => api.del(p(`/streams/${s.id}`), token))}>✕</button>
              </div>
            </div>
          {:else}
            <p class="muted">nikdo nevysílá — hosté: záložka „Živě“, nebo MirrorBooth</p>
          {/each}
        </div>
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
        <h2>
          🎤 Titulky na plátno
          {#if subs.length}<button onclick={() => act(() => api.del(p('/subtitle'), token))}>smazat</button>{/if}
        </h2>
        {#if st?.config.subtitles}
          <div class="row small">
            <label>mluvím
              <select bind:value={langA} disabled={micState !== 'off' && micState !== 'error'}>
                {#each langs as [id, label] (id)}<option value={id}>{label}</option>{/each}
              </select>
            </label>
            <label>překlad do
              <select bind:value={langB} disabled={micState !== 'off' && micState !== 'error'}>
                <option value={langA}>(bez překladu)</option>
                {#each langs.filter(([id]) => id !== langA) as [id, label] (id)}<option value={id}>{label}</option>{/each}
              </select>
            </label>
          </div>
          <div class="row">
            <button class:primary={micState === 'off' || micState === 'error'} class:bad={micState !== 'off' && micState !== 'error'} onclick={toggleMic} disabled={!window.isSecureContext}>
              {micState === 'off' || micState === 'error' ? '🎤 Zapnout mikrofon' : '⏹ Vypnout'}
            </button>
            {#if micState !== 'off' && micState !== 'error'}
              <span class="meter"><span style:width={`${Math.round(micLevel * 100)}%`} class:on={micState === 'speaking'}></span></span>
              <span class="muted">{micState === 'starting' ? 'připojuji…' : micState === 'speaking' ? 'mluvíš' : 'poslouchám'}</span>
            {/if}
          </div>
          {#if !window.isSecureContext}<p class="muted small">Mikrofon v prohlížeči jde jen přes https (nebo localhost).</p>{/if}
          {#if micState === 'error'}<p class="err">{micError}</p>{/if}
        {:else}
          <p class="muted small">Mikrofon potřebuje Translator (TRANSLATOR_URL). Psát titulky jde i bez něj.</p>
        {/if}
        <form class="row" onsubmit={sendTyped}>
          <input placeholder="napsat na plátno (vzkaz, oznámení)" maxlength="160" bind:value={typed} />
          <button disabled={!typed.trim()}>Na plátno</button>
        </form>
        {#if subs.length}
          <ul class="subs">
            {#each subs as l (l.id)}
              <li class:partial={!l.final}>{l.text}{#if l.translation && l.translation !== l.text}<br /><span>{l.translation}</span>{/if}</li>
            {/each}
          </ul>
        {/if}
      </section>

      <section>
        <h2>Losování hráčů</h2>
        <p class="muted">chtějí hrát: {players.length ? players.map((g) => g.nickname).join(', ') : 'nikdo (losuje se ze všech)'}</p>
        <form class="row" onsubmit={(e) => { e.preventDefault(); act(() => api.post(p('/lottery'), { count: lotCount, game: lotGame }, token)); }}>
          <input placeholder="hra (např. Kiriana co-op)" bind:value={lotGame} />
          <input type="number" min="1" max="8" bind:value={lotCount} style="max-width: 70px" />
          <button class="primary">Losovat</button>
        </form>
        <div class="row">
          <button onclick={() => act(() => api.post(p('/game'), {}, token))}>⭕ Spustit Kruh s vylosovanými</button>
          {#if party.scene.kind === 'game'}
            <button onclick={() => act(async () => { await api.del(p('/game'), token); setScene('blank'); })}>Konec hry</button>
          {/if}
        </div>
        <p class="muted small">Kruh: každý hráč brání svůj oblouk pálkou z telefonu, 3 životy, vyhrává poslední. Když telefon zmlkne, hraje za něj autopilot.</p>
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
  .preflight {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 10px 14px;
    margin-bottom: 14px;
  }
  .preflight summary {
    cursor: pointer;
    font-weight: 700;
  }
  .preflight summary span {
    margin-left: 8px;
    font-weight: 400;
    color: var(--green);
  }
  .preflight summary span.bad {
    color: var(--red);
  }
  .preflight summary span.warn {
    color: var(--yellow);
  }
  .pf label {
    display: flex;
    gap: 8px;
    align-items: baseline;
    cursor: pointer;
  }
  .pf {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 16px;
  }
  .pf h3 {
    font-size: 15px;
    margin: 12px 0 6px;
  }
  .pf ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 6px;
    font-size: 14px;
  }
  .pf li {
    border-left: 4px solid var(--line);
    padding: 2px 8px;
  }
  .pf li.ok {
    border-color: var(--green);
  }
  .pf li.warn {
    border-color: var(--yellow);
  }
  .pf li.fail {
    border-color: var(--red);
  }
  .pf li.info {
    border-color: var(--cyan);
  }
  .pf b {
    margin-right: 4px;
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
  .row label input:not([type='checkbox']) {
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
  .card :global(img),
  .card :global(video) {
    width: 100%;
    aspect-ratio: 1;
    object-fit: cover;
    border-radius: 8px;
  }
  .card.mini :global(img) {
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
  .queue :global(img),
  .queue :global(video),
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
  .meter {
    width: 120px;
    height: 8px;
    border-radius: 4px;
    background: var(--line);
    overflow: hidden;
  }
  .meter span {
    display: block;
    height: 100%;
    background: var(--muted);
    transition: width 0.1s;
  }
  .meter span.on {
    background: var(--green);
  }
  .subs {
    list-style: none;
    padding: 0;
    margin: 8px 0 0;
    display: grid;
    gap: 4px;
    font-size: 14px;
  }
  .subs li.partial {
    color: var(--muted);
  }
  .subs span {
    color: var(--yellow);
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
  .card.stream video,
  .card.stream .novid {
    width: 100%;
    aspect-ratio: 3 / 4;
    object-fit: cover;
    border-radius: 8px;
    background: #000;
    display: grid;
    place-items: center;
    font-size: 0.8rem;
    opacity: 0.9;
  }
  .card.stream.live {
    outline: 2px solid var(--red);
  }
</style>
