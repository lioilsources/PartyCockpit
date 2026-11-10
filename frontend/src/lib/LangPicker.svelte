<script lang="ts">
  import { LANGS, lang, setLang, t } from './i18n.svelte';

  // Language switcher: the choice is remembered on this device.
  let { compact = false }: { compact?: boolean } = $props();
</script>

<label class="lp" class:compact>
  {#if !compact}<span>🌐 {t('lang.label')}</span>{/if}
  <select value={lang()} onchange={(e) => setLang(e.currentTarget.value)} aria-label={t('lang.label')}>
    {#each LANGS as l (l.code)}
      <option value={l.code}>{compact ? l.code : l.name}</option>
    {/each}
  </select>
</label>

<style>
  .lp {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    justify-content: center;
    color: var(--muted);
    font-size: 14px;
  }
  select {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    padding: 0.3em 0.5em;
    max-width: 14em;
  }
  .compact select {
    max-width: 6.5em;
    font-size: 13px;
  }
</style>
