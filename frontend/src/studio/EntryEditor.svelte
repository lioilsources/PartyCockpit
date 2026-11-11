<script lang="ts">
  import { api } from '../lib/api';
  import { fileUrl, type Asset, type Episode, type GalleryImage, type ShowEntry } from '../lib/types';
  import type { PackInfo } from '../lib/stylemap';
  import { asList, kindLabel, newSeed, toggle } from './kinds';

  // The parameters and items of one show entry (D23). Every change goes up
  // through onchange; the parent saves and the preview restarts the scene.
  let {
    entry = $bindable(),
    code,
    fileKey,
    episodes,
    packs,
    photos,
    onchange,
  }: { entry: ShowEntry; code: string; fileKey: string; episodes: Episode[]; packs: PackInfo[]; photos: Asset[]; onchange: () => void } = $props();

  function set(key: string, value: unknown) {
    entry.params = { ...entry.params, [key]: value };
    onchange();
  }
  const num = (key: string, fallback: number) => Number(entry.params[key] ?? fallback);
  const list = (key: string) => asList(entry.params[key]);

  // slideshow: images picked from FineTuneGallery (cached on disk for the show)
  let gallery = $state<GalleryImage[]>([]);
  let galleryStyle = $state('');
  let galleryErr = $state('');
  async function loadGallery() {
    galleryErr = '';
    try {
      const q = new URLSearchParams({ limit: '150' });
      if (galleryStyle) q.set('style', galleryStyle);
      gallery = await api.get<GalleryImage[]>(`/api/gallery?${q}`);
      if (!gallery.length) galleryErr = 'galerie nevrátila žádné obrázky';
    } catch (e: any) {
      galleryErr = e.message;
    }
  }
  const shaders = [
    ['', '(střídat)'],
    ['plasma', 'Plazma'],
    ['tunnel', 'Tunel'],
    ['kaleido', 'Kaleidoskop'],
    ['bars', 'Ekvalizér'],
    ['aurora', 'Polární záře'],
  ];
</script>

<div class="editor">
  <div class="row">
    <strong>{kindLabel(entry.kind)}</strong>
    <label class="grow">název <input placeholder="(volitelné)" bind:value={entry.title} oninput={onchange} /></label>
    <label>délka <input type="number" min="5" max="3600" bind:value={entry.seconds} onchange={onchange} /> s</label>
  </div>
  <div class="row small">
    <label>seed <input type="number" value={entry.params.seed ?? ''} onchange={(e) => set('seed', e.currentTarget.value === '' ? undefined : Number(e.currentTarget.value))} /></label>
    <button onclick={() => set('seed', newSeed())} title="jiné pořadí, tempo sloupců a začátky">🎲 nový seed</button>
    <label>přechod
      <select value={entry.params.transition ?? ''} onchange={(e) => set('transition', e.currentTarget.value || undefined)}>
        <option value="">(bez StyleMap přechodu)</option>
        {#each packs as p (p.id)}<option value={p.id}>{p.title}</option>{/each}
      </select>
    </label>
  </div>

  {#if entry.kind === 'webtoon'}
    <div class="row small">
      <label>jazyk bublin
        <select value={entry.params.lang ?? 'cs'} onchange={(e) => set('lang', e.currentTarget.value)}>
          {#each ['cs', 'en', 'de', 'fr', 'it', 'pl', 'uk'] as l (l)}<option value={l}>{l}</option>{/each}
        </select>
      </label>
      <label>rychlost <input type="number" min="10" max="400" step="10" value={num('speed', 60)} onchange={(e) => set('speed', Number(e.currentTarget.value))} /> px/s</label>
      <span class="muted">sloupce: {list('episodes').length || 'první epizoda'} (max 4)</span>
    </div>
    <div class="picks">
      <label class:on={list('episodes').includes('party')}><input type="checkbox" checked={list('episodes').includes('party')} onchange={() => set('episodes', toggle(list('episodes'), 'party', 4))} /> 🎉 Party epizoda</label>
      {#each episodes as ep (ep.id)}
        <label class:on={list('episodes').includes(ep.id)}>
          <input type="checkbox" checked={list('episodes').includes(ep.id)} onchange={() => set('episodes', toggle(list('episodes'), ep.id, 4))} />
          {#if ep.cover}<img src={ep.cover} alt="" />{/if}
          <span>{ep.story ? `${ep.story} · ` : ''}{ep.title} <span class="muted">({ep.n})</span></span>
        </label>
      {:else}
        <p class="muted">Na disku nejsou žádné epizody: stáhni balík v Přípravě → Webtoon knihovna.</p>
      {/each}
    </div>
  {:else if entry.kind === 'strips'}
    <div class="row small">
      <label>sloupce <input type="number" min="2" max="5" value={num('cols', 3)} onchange={(e) => set('cols', Number(e.currentTarget.value))} /></label>
      <label>rychlost <input type="number" min="5" max="300" step="5" value={num('speed', 40)} onchange={(e) => set('speed', Number(e.currentTarget.value))} /> px/s</label>
      <label><input type="checkbox" checked={entry.params.labels !== false} onchange={(e) => set('labels', e.currentTarget.checked)} /> popisky</label>
      <span class="muted">{list('assets').length ? `${list('assets').length} vybraných fotek` : 'všechny fotky z fronty'}</span>
      {#if list('assets').length}<button onclick={() => set('assets', [])}>všechny</button>{/if}
    </div>
    <div class="thumbs">
      {#each photos as a (a.id)}
        <button class="thumb" class:on={list('assets').includes(a.id)} onclick={() => set('assets', toggle(list('assets'), a.id, 200))} title={`${a.meta.nickname ?? ''} ${a.meta.styleLabel ?? ''}`}>
          <img src={fileUrl(code, a.file, fileKey)} alt="" />
          {#if list('assets').includes(a.id)}<span class="n">{list('assets').indexOf(a.id) + 1}</span>{/if}
          {#if a.live}<span class="clip">▶</span>{/if}
        </button>
      {:else}
        <p class="muted">Ve frontě nejsou fotky: nahraj je v Přípravě → Fotky.</p>
      {/each}
    </div>
  {:else if entry.kind === 'stylesweep'}
    <div class="row small">
      <label>snímků/s <input type="number" min="2" max="30" value={num('fps', 12)} onchange={(e) => set('fps', Number(e.currentTarget.value))} /></label>
      <span class="muted">dlaždice: {list('packs').length || 'první pack'} (max 9)</span>
    </div>
    <div class="picks">
      {#each packs as p (p.id)}
        <label class:on={list('packs').includes(p.id)}>
          <input type="checkbox" checked={list('packs').includes(p.id)} onchange={() => set('packs', toggle(list('packs'), p.id, 9))} />
          <span>{p.title} <span class="muted">({p.n}{p.source === 'party' ? ', lokální' : ''})</span></span>
        </label>
      {:else}
        <p class="muted">Žádné packy: vytvoř je v Přípravě → StyleMap ze session.</p>
      {/each}
    </div>
  {:else if entry.kind === 'slideshow'}
    <div class="row small">
      <label>přepnout po <input type="number" min="2" max="64" value={num('beats', 8)} onchange={(e) => set('beats', Number(e.currentTarget.value))} /> dobách</label>
      <label>styl <input placeholder="styleId" bind:value={galleryStyle} /></label>
      <button onclick={loadGallery}>načíst z galerie</button>
      <span class="muted">{list('images').length ? `${list('images').length} vybraných` : 'bez výběru = celá galerie (online)'}</span>
      {#if list('images').length}<button onclick={() => set('images', [])}>zrušit výběr</button>{/if}
    </div>
    {#if galleryErr}<p class="err">{galleryErr}</p>{/if}
    <div class="thumbs">
      {#each list('images').filter((sha) => !gallery.some((g) => g.sha256 === sha)) as sha (sha)}
        <button class="thumb on" onclick={() => set('images', toggle(list('images'), sha, 300))}><img src={`/api/gallery/thumb/${sha}`} alt="" /></button>
      {/each}
      {#each gallery as g (g.sha256)}
        <button class="thumb" class:on={list('images').includes(g.sha256)} onclick={() => set('images', toggle(list('images'), g.sha256, 300))} title={g.styleId}>
          <img src={g.thumb} alt="" loading="lazy" />
          {#if list('images').includes(g.sha256)}<span class="n">{list('images').indexOf(g.sha256) + 1}</span>{/if}
        </button>
      {/each}
    </div>
  {:else if entry.kind === 'gallery' || entry.kind === 'dance'}
    <div class="row small">
      <label>přepnout po <input type="number" min="2" max="64" value={num('beats', 8)} onchange={(e) => set('beats', Number(e.currentTarget.value))} /> dobách</label>
      <span class="muted">hraje show frontu party</span>
    </div>
  {:else if entry.kind === 'interlude'}
    <div class="row small">
      <label>shader
        <select value={entry.params.shader ?? ''} onchange={(e) => set('shader', e.currentTarget.value || undefined)}>
          {#each shaders as [id, label] (id)}<option value={id}>{label}</option>{/each}
        </select>
      </label>
      <label>střídat po <input type="number" min="16" max="128" value={num('beats', 16)} onchange={(e) => set('beats', Math.max(16, Number(e.currentTarget.value)))} /> dobách</label>
    </div>
  {:else}
    <p class="muted small">Tahle scéna nemá položky.</p>
  {/if}
</div>

<style>
  .editor {
    display: grid;
    gap: 8px;
  }
  .row {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    align-items: center;
  }
  .row label {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .row label input:not([type='checkbox']) {
    width: 6em;
  }
  .row .grow {
    flex: 1;
  }
  .row .grow input {
    width: 100% !important;
  }
  .small {
    font-size: 0.88rem;
  }
  .picks {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 4px 10px;
    max-height: 240px;
    overflow: auto;
    font-size: 13px;
  }
  .picks label {
    display: flex;
    gap: 6px;
    align-items: center;
    padding: 2px 4px;
    border-radius: 6px;
  }
  .picks label.on {
    background: var(--panel);
  }
  .picks img {
    width: 28px;
    height: 40px;
    object-fit: cover;
    border-radius: 4px;
  }
  .thumbs {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
    gap: 6px;
    max-height: 260px;
    overflow: auto;
  }
  .thumb {
    position: relative;
    padding: 0;
    border-radius: 8px;
    overflow: hidden;
    aspect-ratio: 3 / 4;
    opacity: 0.6;
  }
  .thumb.on {
    opacity: 1;
    outline: 3px solid var(--pink);
  }
  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .thumb .n,
  .thumb .clip {
    position: absolute;
    top: 3px;
    left: 3px;
    background: var(--pink);
    color: #fff;
    font-size: 11px;
    border-radius: 999px;
    padding: 0 6px;
  }
  .thumb .clip {
    left: auto;
    right: 3px;
    background: rgba(0, 0, 0, 0.6);
  }
</style>
