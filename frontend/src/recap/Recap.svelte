<script lang="ts">
  import { api, store } from '../lib/api';
  import Clip from '../lib/Clip.svelte';
  import { fileUrl, type Guest, type Recap, type RecapItem } from '../lib/types';
  import { lang, t, tn } from '../lib/i18n.svelte';
  import LangPicker from '../lib/LangPicker.svelte';

  // /recap/<code> (M7): what was on the screen tonight, to take home. The phone
  // that joined the party sees its own pictures first.
  let { code }: { code: string } = $props();

  let recap = $state<Recap | null>(null);
  let err = $state('');
  let open = $state<RecapItem | null>(null);
  const me = (() => {
    try {
      return (JSON.parse(store.get(`pc.guest.${code}`) ?? 'null') as Guest | null)?.nickname ?? '';
    } catch {
      return '';
    }
  })();

  api
    .get<Recap>(`/api/parties/${code}/recap`)
    .then((r) => (recap = r))
    .catch((e) => (err = e.status === 404 ? t('recap.gone') : e.message));

  const media = $derived((recap?.items ?? []).filter((i) => i.kind !== 'sticker'));
  const notes = $derived((recap?.items ?? []).filter((i) => i.kind === 'sticker'));
  // nicknames are not unique, so "mine" is a convenience sort, not access control
  const sorted = $derived(me ? [...media.filter((i) => i.nickname === me), ...media.filter((i) => i.nickname !== me)] : media);
  const mineCount = $derived(me ? media.filter((i) => i.nickname === me).length : 0);
  const purge = $derived(recap ? new Date(recap.purgeAt).toLocaleDateString(lang(), { day: 'numeric', month: 'long' }) : '');

  const dl = (i: RecapItem, file = i.file) => `${fileUrl(code, file)}`;
  const name = (i: RecapItem, file = i.file ?? '') => `${(i.nickname ?? 'party').replace(/[^\p{L}\p{N}]+/gu, '-')}-${i.kind}${file.slice(file.lastIndexOf('.'))}`;
</script>

<main class="rc">
  {#if err}
    <p class="err">{err}</p>
  {:else if !recap}
    <p class="muted">{t('recap.loading')}</p>
  {:else}
    <header>
      <h1>{recap.name}</h1>
      <p class="muted">
        {mineCount
          ? t('recap.summaryMine', { images: tn('recap.images', media.length), mine: tn('recap.mine', mineCount), date: purge })
          : t('recap.summary', { images: tn('recap.images', media.length), date: purge })}
      </p>
      <LangPicker />
      {#if media.length}<a class="all" href={`/api/parties/${code}/recap.zip`} download>{t('recap.all')}</a>{/if}
    </header>

    <div class="grid">
      {#each sorted as i (i.id)}
        <button class="tile" class:mine={me && i.nickname === me} onclick={() => (open = i)}>
          <!-- stills in the grid (a phone can't play 30 loops); the loop opens on tap -->
          <Clip src={fileUrl(code, i.file)} />
          <span>{i.nickname ?? t('recap.guest')}{#if i.kind === 'dance'} · 💃{/if}</span>
        </button>
      {:else}
        <p class="muted">{t('recap.empty')}</p>
      {/each}
    </div>

    {#if notes.length}
      <h2>{t('recap.notesTitle')}</h2>
      <ul class="notes">
        {#each notes as n (n.id)}<li style:--c={n.color ?? 'var(--yellow)'}><strong>{n.nickname ?? t('recap.guest')}:</strong> {n.text}</li>{/each}
      </ul>
    {/if}
  {/if}
</main>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div class="sheet" onclick={() => (open = null)}>
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div class="card" onclick={(e) => e.stopPropagation()}>
      <Clip src={fileUrl(code, open.live ?? open.file)} />
      <p><strong>{open.nickname ?? t('recap.guest')}</strong>{#if open.label} · {open.label}{/if}</p>
      <div class="row">
        <a class="primary" href={dl(open)} download={name(open)}>{open.kind === 'dance' ? t('recap.dlDance') : t('recap.dlPhoto')}</a>
        {#if open.live}<a href={dl(open, open.live)} download={name(open, open.live)}>{t('recap.dlLive')}</a>{/if}
        <button onclick={() => (open = null)}>{t('recap.close')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .rc {
    max-width: 960px;
    margin: 0 auto;
    padding: 20px 16px 48px;
  }
  header h1 {
    margin: 0 0 4px;
    font-size: 30px;
  }
  .muted {
    color: var(--muted);
  }
  .err {
    color: var(--red);
  }
  .all,
  .row a,
  .row button {
    display: inline-block;
    padding: 10px 16px;
    border-radius: var(--radius);
    background: var(--panel);
    border: 1px solid var(--line);
    color: var(--text);
    text-decoration: none;
    cursor: pointer;
  }
  .all,
  .row a.primary {
    background: var(--pink);
    border-color: var(--pink);
    color: #fff;
    font-weight: 700;
  }
  .grid {
    margin-top: 18px;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 8px;
  }
  .tile {
    position: relative;
    padding: 0;
    border: 0;
    border-radius: 10px;
    overflow: hidden;
    background: var(--panel);
    aspect-ratio: 3 / 4;
    cursor: pointer;
  }
  .tile.mine {
    outline: 3px solid var(--yellow);
  }
  .tile :global(img),
  .tile :global(video) {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .tile span {
    position: absolute;
    left: 6px;
    bottom: 6px;
    background: rgba(0, 0, 0, 0.6);
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 13px;
  }
  .notes {
    list-style: none;
    padding: 0;
    display: grid;
    gap: 8px;
  }
  .notes li {
    border-left: 4px solid var(--c);
    background: var(--panel);
    padding: 8px 12px;
    border-radius: 8px;
  }
  .sheet {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.8);
    display: grid;
    place-items: center;
    padding: 16px;
    z-index: 10;
  }
  .card {
    max-width: 520px;
    width: 100%;
    text-align: center;
  }
  .card :global(img),
  .card :global(video) {
    max-width: 100%;
    max-height: 70vh;
    border-radius: 12px;
  }
  .row {
    display: flex;
    gap: 8px;
    justify-content: center;
    flex-wrap: wrap;
  }
</style>
