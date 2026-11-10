// Svelte face of i18n-core: the same t()/tn(), but reading them inside a template or
// $derived re-runs it when the language changes.
import * as core from './i18n-core';
import type { Dict } from './i18n-core';

export { LANGS, matchLang, has } from './i18n-core';

const loaders = import.meta.glob<Dict>('../i18n/*.json', { import: 'default' });
const rev = $state({ n: 0 });
core.subscribe(() => rev.n++);

async function load(lang: string): Promise<Dict> {
  if (lang === 'cs') return core.cs;
  if (lang === 'en') return core.en;
  const f = loaders[`../i18n/${lang}.json`];
  return f ? await f() : core.en;
}

/** Picks the language (?lang=, saved choice, browser) and loads its catalog. */
export async function initI18n(): Promise<void> {
  const lang = core.pickLang();
  core.apply(lang, await load(lang));
}

/** The user's explicit choice; remembered on this device. */
export async function setLang(lang: string): Promise<void> {
  core.apply(lang, await load(lang));
  core.remember(lang);
}

export const lang = () => (rev.n, core.ui.lang);
export const isRtl = () => (rev.n, core.isRtl());
export const t = (key: string, p?: Record<string, string | number>) => (rev.n, core.t(key, p));
export const tn = (key: string, n: number, p?: Record<string, string | number>) => (rev.n, core.tn(key, n, p));
export const moveLabel = (id: string | undefined, fallback = '') => (rev.n, core.moveLabel(id, fallback));
