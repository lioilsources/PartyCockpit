<script lang="ts">
  import { api } from '../lib/api';
  import { t } from '../lib/i18n.svelte';

  // Photo → Restyler style → Spark. The photo is scaled to ≤ 1536 px JPEG here:
  // the browser applies EXIF orientation when drawing, and iPhone HEIC decodes too.
  let { code, guestId, busy, onsent }: { code: string; guestId: string; busy: boolean; onsent: () => void } = $props();

  type Style = { id: string; label: string; group: string };
  let groups = $state<string[]>([]);
  let styles = $state<Style[]>([]);
  let group = $state('Popular');
  let style = $state('');
  let medium = $state<'illustration' | 'photo'>('illustration');
  let preview = $state('');
  let blob = $state<Blob | null>(null);
  let sending = $state(false);
  let err = $state('');

  api.get<{ groups: string[]; styles: Style[] }>('/api/styles').then((r) => {
    groups = r.groups;
    styles = r.styles;
    style = r.styles.find((s) => s.group === 'Popular')?.id ?? r.styles[0]?.id ?? '';
  });

  async function pick(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    err = '';
    try {
      const bmp = await createImageBitmap(f);
      const s = Math.min(1, 1536 / Math.max(bmp.width, bmp.height));
      const c = document.createElement('canvas');
      c.width = Math.round(bmp.width * s);
      c.height = Math.round(bmp.height * s);
      c.getContext('2d')!.drawImage(bmp, 0, 0, c.width, c.height);
      blob = await new Promise<Blob | null>((r) => c.toBlob(r, 'image/jpeg', 0.9));
      preview = c.toDataURL('image/jpeg', 0.6);
    } catch {
      err = t('photo.openFail');
    }
  }

  async function send() {
    if (!blob || !style) return;
    sending = true;
    err = '';
    const fd = new FormData();
    fd.append('photo', blob, 'photo.jpg');
    fd.append('style', style);
    fd.append('medium', medium);
    try {
      await api.post(`/api/parties/${code}/guests/${guestId}/photos`, fd);
      preview = '';
      blob = null;
      onsent();
    } catch (e: any) {
      err = e.message;
    } finally {
      sending = false;
    }
  }

  const inGroup = $derived(styles.filter((s) => s.group === group));
</script>

<h2>{t('photo.title')}</h2>
<label class="drop" class:has={preview}>
  {#if preview}<img src={preview} alt="" />{:else}<span>{t('photo.pick')}</span>{/if}
  <input type="file" accept="image/*" onchange={pick} />
</label>

<div class="seg">
  <button class:on={medium === 'illustration'} onclick={() => (medium = 'illustration')}>{t('photo.illustration')}</button>
  <button class:on={medium === 'photo'} onclick={() => (medium = 'photo')}>{t('photo.photo')}</button>
</div>

<div class="groups">
  {#each groups as gname (gname)}
    <button class:on={group === gname} onclick={() => (group = gname)}>{gname}</button>
  {/each}
</div>
<div class="styles">
  {#each inGroup as s (s.id)}
    <button class:on={style === s.id} onclick={() => (style = s.id)}>{s.label}</button>
  {/each}
</div>

<button class="primary send" disabled={!blob || !style || sending || busy} onclick={send}>
  {busy ? t('photo.busy') : sending ? t('photo.sending') : t('photo.send')}
</button>
{#if err}<p class="err">{err}</p>{/if}

<style>
  .drop {
    display: grid;
    place-items: center;
    min-height: 180px;
    border: 2px dashed var(--line);
    border-radius: 14px;
    overflow: hidden;
    position: relative;
    cursor: pointer;
  }
  .drop.has {
    border-style: solid;
  }
  .drop img {
    width: 100%;
    max-height: 360px;
    object-fit: contain;
  }
  .drop input {
    position: absolute;
    inset: 0;
    opacity: 0;
  }
  .seg,
  .groups,
  .styles {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .groups {
    overflow-x: auto;
    flex-wrap: nowrap;
    padding-bottom: 4px;
  }
  .groups button {
    white-space: nowrap;
    font-size: 0.85rem;
  }
  .styles {
    max-height: 220px;
    overflow-y: auto;
  }
  .styles button {
    font-size: 0.9rem;
    padding: 0.45em 0.75em;
  }
  .send {
    padding: 0.95em;
  }
</style>
