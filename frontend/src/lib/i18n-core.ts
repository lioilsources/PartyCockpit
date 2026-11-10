// UI strings for guests, the stage, the recap and the start page. Catalogs live in
// src/i18n/<lang>.json (flat keys); Czech is the source, English the fallback.
// Plural forms are separate keys: key.zero|one|two|few|many|other, picked with Intl.PluralRules.
// The operator Cockpit is not translated (it stays Czech).
// Framework-free on purpose: the React Cockpit shares lib/api.ts and must not pull in Svelte.
import cs from '../i18n/cs.json';
import en from '../i18n/en.json';

export type Dict = Record<string, string>;

export const LANGS: { code: string; name: string }[] = [
  { code: 'ar', name: 'العربية' },
  { code: 'bn', name: 'বাংলা' },
  { code: 'ca', name: 'Català' },
  { code: 'cs', name: 'Čeština' },
  { code: 'da', name: 'Dansk' },
  { code: 'de', name: 'Deutsch' },
  { code: 'el', name: 'Ελληνικά' },
  { code: 'en', name: 'English' },
  { code: 'es', name: 'Español' },
  { code: 'fi', name: 'Suomi' },
  { code: 'fr', name: 'Français' },
  { code: 'gu', name: 'ગુજરાતી' },
  { code: 'he', name: 'עברית' },
  { code: 'hi', name: 'हिन्दी' },
  { code: 'hr', name: 'Hrvatski' },
  { code: 'hu', name: 'Magyar' },
  { code: 'id', name: 'Bahasa Indonesia' },
  { code: 'it', name: 'Italiano' },
  { code: 'ja', name: '日本語' },
  { code: 'kn', name: 'ಕನ್ನಡ' },
  { code: 'ko', name: '한국어' },
  { code: 'ml', name: 'മലയാളം' },
  { code: 'mr', name: 'मराठी' },
  { code: 'ms', name: 'Bahasa Melayu' },
  { code: 'nb', name: 'Norsk bokmål' },
  { code: 'nl', name: 'Nederlands' },
  { code: 'or', name: 'ଓଡ଼ିଆ' },
  { code: 'pa', name: 'ਪੰਜਾਬੀ' },
  { code: 'pl', name: 'Polski' },
  { code: 'pt-BR', name: 'Português (Brasil)' },
  { code: 'pt-PT', name: 'Português (Portugal)' },
  { code: 'ro', name: 'Română' },
  { code: 'ru', name: 'Русский' },
  { code: 'sk', name: 'Slovenčina' },
  { code: 'sl', name: 'Slovenščina' },
  { code: 'sv', name: 'Svenska' },
  { code: 'ta', name: 'தமிழ்' },
  { code: 'te', name: 'తెలుగు' },
  { code: 'th', name: 'ไทย' },
  { code: 'tr', name: 'Türkçe' },
  { code: 'uk', name: 'Українська' },
  { code: 'ur', name: 'اردو' },
  { code: 'vi', name: 'Tiếng Việt' },
  { code: 'zh-Hans', name: '简体中文' },
  { code: 'zh-Hant', name: '繁體中文' },
];

const RTL = new Set(['ar', 'he', 'ur']);
export const KEY = 'pc.lang';

export const ui: { lang: string; dict: Dict } = { lang: 'cs', dict: cs };
const listeners = new Set<() => void>();
export const subscribe = (fn: () => void) => (listeners.add(fn), () => listeners.delete(fn));
export { cs, en };

function read(): string | null {
  try {
    return localStorage.getItem(KEY);
  } catch {
    return null;
  }
}

/** Maps a browser language tag (BCP 47) to one of the 45 catalog codes. */
export function matchLang(tag: string): string | null {
  const t = tag.replace('_', '-');
  const [base, ...rest] = t.split('-');
  const l = base.toLowerCase();
  const has = (c: string) => LANGS.some((x) => x.code === c);
  if (l === 'zh') {
    const r = rest.join('-').toLowerCase();
    return /hant|tw|hk|mo/.test(r) ? 'zh-Hant' : 'zh-Hans';
  }
  if (l === 'pt') return /pt$/i.test(rest[rest.length - 1] ?? '') ? 'pt-PT' : 'pt-BR';
  if (l === 'no' || l === 'nn') return 'nb';
  if (l === 'iw') return 'he';
  if (l === 'in') return 'id';
  return has(l) ? l : null;
}

/** ?lang=, the saved choice, then the browser languages; English when nothing matches. */
export function pickLang(): string {
  // the operator Cockpit is Czech only
  if (/^\/(react\/)?cockpit\//.test(location.pathname)) return 'cs';
  const wanted = new URLSearchParams(location.search).get('lang');
  const saved = read();
  for (const tag of [wanted, saved, ...(navigator.languages ?? [navigator.language])]) {
    if (!tag) continue;
    const m = LANGS.some((x) => x.code === tag) ? tag : matchLang(tag);
    if (m) return m;
  }
  return 'en';
}

export function apply(lang: string, dict: Dict) {
  ui.lang = lang;
  ui.dict = dict;
  document.documentElement.lang = lang;
  document.documentElement.dir = RTL.has(lang) ? 'rtl' : 'ltr';
  listeners.forEach((f) => f());
}

export function remember(lang: string) {
  try {
    localStorage.setItem(KEY, lang);
  } catch {
    /* private mode */
  }
}

export const isRtl = () => RTL.has(ui.lang);

function fill(s: string, p?: Record<string, string | number>): string {
  return p ? s.replace(/\{(\w+)\}/g, (m, k) => (k in p ? String(p[k]) : m)) : s;
}

/** Looks a key up: current language, then English, then Czech, then the key itself. */
export function t(key: string, p?: Record<string, string | number>): string {
  return fill(ui.dict[key] ?? (en as Dict)[key] ?? (cs as Dict)[key] ?? key, p);
}

/** Plural-aware lookup: key.<category> for n in the current language, falling back to key.other. */
export function tn(key: string, n: number, p?: Record<string, string | number>): string {
  let cat = 'other';
  try {
    cat = new Intl.PluralRules(ui.lang).select(n);
  } catch {
    /* unknown locale */
  }
  const k = ui.dict[`${key}.${cat}`] !== undefined ? `${key}.${cat}` : `${key}.other`;
  return t(k, { n, ...p });
}

export const has = (key: string) => key in ui.dict || key in (en as Dict) || key in (cs as Dict);

/** Dance names come from the backend with an emoji; the move id picks the translation. */
export const moveLabel = (id: string | undefined, fallback = '') => (id && has(`move.${id}`) ? t(`move.${id}`) : fallback);
