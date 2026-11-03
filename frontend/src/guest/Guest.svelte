<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, store } from '../lib/api';
  import { connect } from '../lib/ws';
  import type { Asset, Guest, MusicSnapshot, Party } from '../lib/types';
  import Photo from './Photo.svelte';
  import Music from './Music.svelte';
  import Live from './Live.svelte';
  import Dance from './Dance.svelte';
  import Clip from '../lib/Clip.svelte';

  let { code }: { code: string } = $props();

  const key = `pc.guest.${code}`;
  let guest = $state<Guest | null>(JSON.parse(store.get(key) ?? 'null'));
  let party = $state<Party | null>(null);
  let music = $state<MusicSnapshot | null>(null);
  let mine = $state<Asset[]>([]);
  let nick = $state(store.get('pc.nick') ?? '');
  let err = $state('');
  let notFound = $state(false);
  let tab = $state<'photo' | 'live' | 'music' | 'note' | 'idea' | 'play'>('photo');
  let toast = $state('');
  let won = $state('');

  api.get<Party>(`/api/parties/${code}`).then((p) => (party = p)).catch((e) => (notFound = e.status === 404, err = e.message));

  async function join() {
    err = '';
    try {
      guest = await api.post<Guest>(`/api/parties/${code}/guests`, { nickname: nick });
      store.set(key, JSON.stringify(guest));
      store.set('pc.nick', nick);
      refresh();
    } catch (e: any) {
      err = e.message;
    }
  }

  const g = (path: string) => `/api/parties/${code}/guests/${guest!.id}${path}`;

  async function refresh() {
    if (!guest) return;
    try {
      const r = await api.get<{ guest: Guest; assets: Asset[] }>(g('/assets'));
      mine = r.assets;
      guest = r.guest;
    } catch (e: any) {
      if (e.status === 404) {
        // party was recreated or guest purged: start over
        store.del(key);
        guest = null;
      }
    }
  }
  refresh();

  const sock = connect(
    code,
    'guest',
    (m) => {
      if (m.type === 'party') party = m.data;
      else if (m.type === 'music') music = m.data;
      else if (m.type === 'asset.updated') refresh();
      else if (m.type === 'lottery.won' && m.data.guestId === guest?.id) won = m.data.game || 'hru';
    },
    () => {},
  );
  // also poll while something is being painted (the WS ping may be lost on mobile)
  const poll = setInterval(() => {
    if (mine.some((a) => a.status === 'processing')) refresh();
  }, 5000);
  onDestroy(() => {
    sock.close();
    clearInterval(poll);
  });

  function say(t: string) {
    toast = t;
    setTimeout(() => (toast = ''), 3000);
  }

  async function react(emoji: string) {
    try {
      await api.post(g('/reactions'), { emoji });
      navigator.vibrate?.(15);
    } catch {
      /* rate limited, ignore */
    }
  }

  let note = $state('');
  let noteColor = $state('#ffd84d');
  const noteColors = ['#ffd84d', '#ff8fb8', '#8fe3ff', '#a6ff8f', '#ffb36b'];
  async function sendNote() {
    try {
      await api.post(g('/stickers'), { text: note, color: noteColor });
      note = '';
      say('Vzkaz odeslán, objeví se po schválení ✍️');
      refresh();
    } catch (e: any) {
      say(e.message);
    }
  }

  let idea = $state('');
  async function sendIdea() {
    try {
      await api.post(g('/ideas'), { text: idea });
      idea = '';
      say('Díky! Z námětů vzniknou další díly 💡');
    } catch (e: any) {
      say(e.message);
    }
  }

  async function setPlay(wants: boolean) {
    await api.put(g('/play'), { wants });
    guest = { ...guest!, wantsToPlay: wants };
    store.set(key, JSON.stringify(guest));
  }

  const statusLabel: Record<string, string> = {
    processing: '🎨 pracuje se…',
    pending: '⏳ čeká na schválení',
    approved: '✅ na plátně',
    rejected: '— nepoužito',
    failed: '⚠️ nepovedlo se',
  };
  const emojis = ['❤️', '🔥', '😂', '🎉', '👏', '😍', '🕺', '💃', '🍍', '🍌'];
</script>

{#if notFound}
  <main class="center"><h1>Party nenalezena</h1><p class="muted">Zkontroluj kód na plátně.</p></main>
{:else if !guest}
  <main class="center">
    <h1>{party?.name ?? 'Party'}</h1>
    <p class="muted">Jak ti máme říkat? Jméno se ukáže na plátně u tvých fotek.</p>
    <form onsubmit={(e) => { e.preventDefault(); join(); }}>
      <input placeholder="Přezdívka" bind:value={nick} maxlength="24" autocomplete="nickname" />
      <button class="primary" disabled={!nick.trim()}>Jdu na to</button>
    </form>
    {#if err}<p class="err">{err}</p>{/if}
  </main>
{:else}
  <main class="g">
    <header>
      <strong>{party?.name}</strong>
      <span class="muted">ahoj {guest.nickname}</span>
    </header>

    {#if won}
      <div class="won">🎉 Vylosovali tě do: <strong>{won}</strong>! Běž k plátnu.<button onclick={() => (won = '')}>ok</button></div>
    {/if}

    <nav>
      <button class:on={tab === 'photo'} onclick={() => (tab = 'photo')}>📸<span>Fotka</span></button>
      <button class:on={tab === 'live'} onclick={() => (tab = 'live')}>📡<span>Živě</span></button>
      <button class:on={tab === 'music'} onclick={() => (tab = 'music')}>🎵<span>Hudba</span></button>
      <button class:on={tab === 'note'} onclick={() => (tab = 'note')}>✍️<span>Vzkaz</span></button>
      <button class:on={tab === 'idea'} onclick={() => (tab = 'idea')}>💡<span>Námět</span></button>
      <button class:on={tab === 'play'} onclick={() => (tab = 'play')}>🎮<span>Hra</span></button>
    </nav>

    <!-- kept mounted: switching tabs must not cut the stream -->
    <section hidden={tab !== 'live'}>
      <Live {code} guestId={guest.id} iceServers={party?.iceServers ?? []} onmsg={say} />
    </section>
    <section hidden={tab === 'live'}>
      {#if tab === 'photo'}
        <Photo {code} guestId={guest.id} busy={mine.some((a) => a.status === 'processing' && a.kind !== 'live')} onsent={() => { say('Fotka se maluje na GPU 🎨'); refresh(); }} />
        <Dance {code} guestId={guest.id} {mine} busy={mine.some((a) => a.status === 'processing' && a.kind !== 'live')} onmsg={say} onsent={refresh} />
      {:else if tab === 'music'}
        <Music {code} guestId={guest.id} {music} djfyCode={party?.djfyCode ?? ''} onmsg={say} />
      {:else if tab === 'note'}
        <h2>Vzkaz na zeď</h2>
        <textarea bind:value={note} maxlength="140" rows="3" placeholder="Všechno nejlepší! 🎂"></textarea>
        <div class="colors">
          {#each noteColors as c (c)}
            <button class="sw" class:sel={noteColor === c} style:background={c} onclick={() => (noteColor = c)} aria-label={c}></button>
          {/each}
        </div>
        <button class="primary wide" disabled={!note.trim()} onclick={sendNote}>Nalepit na zeď</button>
      {:else if tab === 'idea'}
        <h2>Námět na webtoon</h2>
        <p class="muted">Na plátně hrajeme hotové příběhy. Napiš, o čem by měl být další díl — kdo v něm hraje, co se stane.</p>
        <textarea bind:value={idea} maxlength="1000" rows="5" placeholder="Olin omylem pozve na party mimozemšťany…"></textarea>
        <button class="primary wide" disabled={idea.trim().length < 5} onclick={sendIdea}>Poslat námět</button>
      {:else if tab === 'play'}
        <h2>Hry o přestávce</h2>
        <p class="muted">Kdo bude hrát, se losuje. Přihlas se do osudí.</p>
        {#if guest.wantsToPlay}
          <button class="on wide" onclick={() => setPlay(false)}>✓ Jsem v osudí (odhlásit)</button>
        {:else}
          <button class="primary wide" onclick={() => setPlay(true)}>Chci hrát</button>
        {/if}
      {/if}
    </section>

    {#if mine.length}
      <section>
        <h2>Moje</h2>
        <ul class="mine">
          {#each mine as a (a.id)}
            <li>
              {#if a.kind === 'sticker'}
                <span class="mini" style:background={a.meta.color}>✍️</span>
                <span class="t">{a.meta.text}</span>
              {:else if a.kind === 'live' || a.kind === 'dance'}
                {#if a.file}<Clip src={`/files/${code}/${a.file}`} cls="mini" />{:else}<span class="mini">{a.kind === 'dance' ? '💃' : '✨'}</span>{/if}
                <span class="t">{a.kind === 'dance' ? `tanec ${a.meta.motionLabel ?? ''}` : 'živý portrét'}</span>
              {:else}
                {#if a.file}<img src={`/files/${code}/${a.file}`} alt="" />{:else}<span class="mini">🎨</span>{/if}
                <span class="t">{a.meta.styleLabel}</span>
              {/if}
              <span class="st">{statusLabel[a.status] ?? a.status}</span>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    <div class="emoji">
      {#each emojis as e (e)}<button onclick={() => react(e)}>{e}</button>{/each}
    </div>
    {#if toast}<div class="toast">{toast}</div>{/if}
  </main>
{/if}

<style>
  .center {
    max-width: 420px;
    margin: 0 auto;
    padding: 18vh 16px;
    display: grid;
    gap: 10px;
    text-align: center;
  }
  .center form {
    display: grid;
    gap: 10px;
  }
  .g {
    max-width: 520px;
    margin: 0 auto;
    padding: 12px 16px 96px;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding: 6px 0 12px;
  }
  nav {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 6px;
    position: sticky;
    top: 0;
    background: var(--bg);
    padding: 6px 0;
    z-index: 2;
  }
  nav button {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-size: 1.3rem;
    padding: 8px 2px;
  }
  nav button span {
    font-size: 0.7rem;
  }
  section[hidden] {
    display: none;
  }
  section {
    margin-top: 14px;
    display: grid;
    gap: 10px;
  }
  h2 {
    margin: 4px 0 0;
    font-size: 1.15rem;
  }
  .wide {
    width: 100%;
    padding: 0.9em;
  }
  .colors {
    display: flex;
    gap: 10px;
  }
  .sw {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    padding: 0;
  }
  .sw.sel {
    outline: 3px solid #fff;
  }
  .mine {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 8px;
  }
  .mine li {
    display: flex;
    gap: 10px;
    align-items: center;
    background: var(--panel);
    padding: 6px;
    border-radius: 10px;
    font-size: 0.9rem;
  }
  .mine img,
  .mine :global(video.mini),
  .mine :global(img.mini),
  .mini {
    width: 48px;
    height: 48px;
    object-fit: cover;
    border-radius: 8px;
    display: grid;
    place-items: center;
    flex: none;
  }
  .t {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .st {
    font-size: 0.8rem;
    white-space: nowrap;
  }
  .emoji {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    gap: 4px;
    overflow-x: auto;
    padding: 10px 12px calc(10px + env(safe-area-inset-bottom));
    background: rgba(11, 6, 19, 0.92);
    backdrop-filter: blur(8px);
    border-top: 1px solid var(--line);
    justify-content: center;
  }
  .emoji button {
    font-size: 1.5rem;
    background: none;
    border: none;
    padding: 4px 6px;
  }
  .toast {
    position: fixed;
    left: 16px;
    right: 16px;
    bottom: 86px;
    background: var(--panel);
    border: 1px solid var(--pink);
    padding: 12px;
    border-radius: 12px;
    text-align: center;
    z-index: 5;
  }
  .won {
    background: var(--pink);
    padding: 12px;
    border-radius: 12px;
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
  }
</style>
