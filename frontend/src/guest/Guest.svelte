<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, store } from '../lib/api';
  import { connect } from '../lib/ws';
  import { ownFileUrl, type Asset, type Guest, type MusicSnapshot, type Party } from '../lib/types';
  import Photo from './Photo.svelte';
  import Music from './Music.svelte';
  import Live from './Live.svelte';
  import Dance from './Dance.svelte';
  import Clip from '../lib/Clip.svelte';
  import Handwritten from '../lib/Handwritten.svelte';
  import Pad from './Pad.svelte';
  import type { Socket } from '../lib/ws';
  import { listSheets, type SheetInfo } from '../lib/handwriting';
  import { moveLabel, t } from '../lib/i18n.svelte';
  import LangPicker from '../lib/LangPicker.svelte';

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
  let slot = $state<{ slot: number; color: string } | null>(null); // playing the intermission game (D17)

  api.get<Party>(`/api/parties/${code}`).then((p) => (party = p)).catch((e) => (notFound = e.status === 404, err = e.message));

  async function join() {
    err = '';
    try {
      guest = await api.post<Guest>(`/api/parties/${code}/guests`, { nickname: nick });
      store.set(key, JSON.stringify(guest));
      store.set('pc.nick', nick);
      refresh();
      sock.close();
      sock = openSock();
    } catch (e: any) {
      err = e.message;
      // nickname taken: offer the free variant the server found
      if (e.status === 409 && e.data?.suggest) {
        nick = e.data.suggest;
        err = t('guest.nickTaken', { suggest: e.data.suggest });
      }
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

  // the socket says who the phone is (lottery win, game slot), so it reopens after joining
  let sock: Socket = openSock();
  function openSock() {
    return connect(
      code,
      'guest',
      (m) => {
        if (m.type === 'party') party = m.data;
        else if (m.type === 'music') music = m.data;
        else if (m.type === 'asset.updated') refresh();
        else if (m.type === 'lottery.won') won = m.data.game || 'hru'; // sent to the winner's phone only
        else if (m.type === 'game.slot') {
          slot = m.data;
          tab = 'play';
        } else if (m.type === 'game.end') slot = null;
      },
      () => {},
      guest ? { guest: guest.id } : {},
    );
  }
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
  // handwriting (D16): a few sheets to pick from, previewed live on the note
  let hands = $state<SheetInfo[]>([]);
  let hand = $state(store.get('pc.hand') ?? '');
  listSheets().then((list) => {
    // a short, varied choice: the default and the rest of the handwriting first
    hands = [...list.filter((x) => x.category === 'handwriting'), ...list.filter((x) => x.category === 'kids')].slice(0, 10);
    if (hands.length && !hands.some((x) => x.id === hand)) hand = (hands.find((x) => x.default) ?? hands[0]).id;
  });
  $effect(() => {
    if (hand) store.set('pc.hand', hand);
  });
  async function sendNote() {
    try {
      await api.post(g('/stickers'), { text: note, color: noteColor, sheet: hand });
      note = '';
      say(t('guest.toast.note'));
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
      say(t('guest.toast.idea'));
    } catch (e: any) {
      say(e.message);
    }
  }

  async function setPlay(wants: boolean) {
    await api.put(g('/play'), { wants });
    guest = { ...guest!, wantsToPlay: wants };
    store.set(key, JSON.stringify(guest));
  }

  const statusLabel = (s: string) => (['processing', 'pending', 'approved', 'rejected', 'failed'].includes(s) ? t(`status.${s}`) : s);
  const emojis = ['❤️', '🔥', '😂', '🎉', '👏', '😍', '🕺', '💃', '🍍', '🍌'];
</script>

{#if notFound}
  <main class="center"><h1>{t('guest.notFound')}</h1><p class="muted">{t('guest.notFoundHint')}</p></main>
{:else if !guest}
  <main class="center">
    <h1>{party?.name ?? t('guest.party')}</h1>
    <p class="muted">{t('guest.joinHint')}</p>
    <form onsubmit={(e) => { e.preventDefault(); join(); }}>
      <input placeholder={t('guest.nick')} bind:value={nick} maxlength="24" autocomplete="nickname" />
      <button class="primary" disabled={!nick.trim()}>{t('guest.go')}</button>
    </form>
    {#if err}<p class="err">{err}</p>{/if}
    <LangPicker />
  </main>
{:else}
  <main class="g">
    <header>
      <strong>{party?.name}</strong>
      <span class="muted">{t('guest.hello', { nick: guest.nickname })}</span>
      <LangPicker compact />
      <a class="recap" href={`/recap/${code}`}>🎞 {t('guest.recap')}</a>
    </header>

    {#if won}
      <div class="won">🎉 {t('guest.won', { game: won })}<button onclick={() => (won = '')}>{t('guest.ok')}</button></div>
    {/if}

    <nav>
      <button class:on={tab === 'photo'} onclick={() => (tab = 'photo')}>📸<span>{t('guest.tab.photo')}</span></button>
      <button class:on={tab === 'live'} onclick={() => (tab = 'live')}>📡<span>{t('guest.tab.live')}</span></button>
      <button class:on={tab === 'music'} onclick={() => (tab = 'music')}>🎵<span>{t('guest.tab.music')}</span></button>
      <button class:on={tab === 'note'} onclick={() => (tab = 'note')}>✍️<span>{t('guest.tab.note')}</span></button>
      <button class:on={tab === 'idea'} onclick={() => (tab = 'idea')}>💡<span>{t('guest.tab.idea')}</span></button>
      <button class:on={tab === 'play'} onclick={() => (tab = 'play')}>🎮<span>{t('guest.tab.play')}</span></button>
    </nav>

    <!-- kept mounted: switching tabs must not cut the stream -->
    <section hidden={tab !== 'live'}>
      <Live {code} guestId={guest.id} iceServers={party?.iceServers ?? []} onmsg={say} />
    </section>
    <section hidden={tab === 'live'}>
      {#if tab === 'photo'}
        <Photo {code} guestId={guest.id} busy={mine.some((a) => a.status === 'processing' && a.kind !== 'live')} onsent={() => { say(t('guest.toast.photo')); refresh(); }} />
        <Dance {code} guestId={guest.id} {mine} busy={mine.some((a) => a.status === 'processing' && a.kind !== 'live')} onmsg={say} onsent={refresh} />
      {:else if tab === 'music'}
        <Music {code} guestId={guest.id} {music} djfyCode={party?.djfyCode ?? ''} onmsg={say} />
      {:else if tab === 'note'}
        <h2>{t('guest.noteTitle')}</h2>
        <textarea bind:value={note} maxlength="140" rows="3" placeholder={t('guest.notePlaceholder')}></textarea>
        <div class="colors">
          {#each noteColors as c (c)}
            <button class="sw" class:sel={noteColor === c} style:background={c} onclick={() => (noteColor = c)} aria-label={c}></button>
          {/each}
        </div>
        {#if hands.length}
          <div class="hands">
            {#each hands as h (h.id)}
              <button class:sel={hand === h.id} onclick={() => (hand = h.id)}>{h.name}</button>
            {/each}
          </div>
          <div class="preview" style:background={noteColor}>
            <Handwritten text={note.trim() || t('guest.notePlaceholder')} sheet={hand} cell={30} width={260} density={2} />
          </div>
        {/if}
        <button class="primary wide" disabled={!note.trim()} onclick={sendNote}>{t('guest.noteStick')}</button>
      {:else if tab === 'idea'}
        <h2>{t('guest.ideaTitle')}</h2>
        <p class="muted">{t('guest.ideaHint')}</p>
        <textarea bind:value={idea} maxlength="1000" rows="5" placeholder={t('guest.ideaPlaceholder')}></textarea>
        <button class="primary wide" disabled={idea.trim().length < 5} onclick={sendIdea}>{t('guest.ideaSend')}</button>
      {:else if tab === 'play'}
        {#if slot}
          <h2>{t('guest.playing')} <span style:color={slot.color}>●</span></h2>
          <p class="muted">{t('guest.padHint')}</p>
          <Pad color={slot.color} send={(x) => sock.send({ type: 'game.input', data: { x } })} />
        {/if}
        <h2>{t('guest.gamesTitle')}</h2>
        <p class="muted">{t('guest.gamesHint')}</p>
        {#if guest.wantsToPlay}
          <button class="on wide" onclick={() => setPlay(false)}>{t('guest.inLottery')}</button>
        {:else}
          <button class="primary wide" onclick={() => setPlay(true)}>{t('guest.wantPlay')}</button>
        {/if}
      {/if}
    </section>

    {#if mine.length}
      <section>
        <h2>{t('guest.mine')}</h2>
        <ul class="mine">
          {#each mine as a (a.id)}
            <li>
              {#if a.kind === 'sticker'}
                <span class="mini" style:background={a.meta.color}>✍️</span>
                <span class="t">{a.meta.text}</span>
              {:else if a.kind === 'live' || a.kind === 'dance'}
                {#if a.file}<Clip src={ownFileUrl(code, guest.id, a.file)} cls="mini" />{:else}<span class="mini">{a.kind === 'dance' ? '💃' : '✨'}</span>{/if}
                <span class="t">{a.kind === 'dance' ? t('guest.mineDance', { label: moveLabel(a.meta.motion, a.meta.motionLabel ?? '') }) : t('guest.mineLive')}</span>
              {:else}
                {#if a.file}<img src={ownFileUrl(code, guest.id, a.file)} alt="" />{:else}<span class="mini">🎨</span>{/if}
                <span class="t">{a.meta.styleLabel}</span>
              {/if}
              <span class="st">{statusLabel(a.status)}</span>
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
  .recap {
    color: var(--muted);
    font-size: 14px;
    text-decoration: none;
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
  .hands {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    padding: 4px 0 8px;
  }
  .hands button {
    flex: none;
    padding: 6px 12px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background: var(--panel);
    font-size: 14px;
  }
  .hands button.sel {
    border-color: var(--yellow);
    color: var(--yellow);
  }
  .preview {
    color: #1d1208;
    border-radius: 6px;
    padding: 12px 14px;
    margin-bottom: 12px;
    min-height: 60px;
    box-shadow: 0 6px 14px rgba(0, 0, 0, 0.35);
    transform: rotate(-1.5deg);
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
