import { api } from '../lib/api';
import type { SceneKind, ShowEntry } from '../lib/types';

// Scenes a Studio show can hold (D23), with what a new entry starts with.
// "game" is left out: it needs a lottery draw first.
export const KINDS: { kind: SceneKind; label: string; seconds: number; params: Record<string, unknown> }[] = [
  { kind: 'webtoon', label: '📖 Webtoon', seconds: 90, params: { episodes: [], lang: 'cs', speed: 60 } },
  { kind: 'strips', label: '🎞 Pásy', seconds: 60, params: { cols: 3, speed: 40, labels: true, assets: [] } },
  { kind: 'tributes', label: '🎨 Tributy', seconds: 90, params: { layout: 'grid', cols: 2, speed: 40, labels: true, tributes: [] } },
  { kind: 'stylesweep', label: '🎨 StyleMap', seconds: 45, params: { packs: [] } },
  { kind: 'slideshow', label: '🖼 Slideshow', seconds: 60, params: { beats: 8, images: [] } },
  { kind: 'gallery', label: '📸 Galerie hostů', seconds: 60, params: { beats: 8 } },
  { kind: 'dance', label: '💃 Tančírna', seconds: 45, params: { beats: 8 } },
  { kind: 'stickers', label: '✍️ Vzkazy', seconds: 45, params: {} },
  { kind: 'interlude', label: '🌀 Shader přestávka', seconds: 30, params: { beats: 16 } },
  { kind: 'nowplaying', label: '🎵 Now playing', seconds: 30, params: {} },
  { kind: 'recap', label: '🎞 Recap', seconds: 60, params: {} },
  { kind: 'blank', label: '✨ Mezihra', seconds: 15, params: {} },
  { kind: 'pipwall', label: '📡 Živě (streamy)', seconds: 60, params: {} },
];

export const kindLabel = (k: SceneKind) => KINDS.find((x) => x.kind === k)?.label ?? k;

export const newSeed = () => Math.floor(Math.random() * 1e6);

/** A fresh entry; the id is short lowercase as the server wants it. */
export function newEntry(kind: SceneKind): ShowEntry {
  const k = KINDS.find((x) => x.kind === kind) ?? KINDS[0];
  return {
    id: Math.random().toString(36).slice(2, 10),
    kind,
    title: '',
    seconds: k.seconds,
    params: { ...structuredClone(k.params), seed: newSeed() },
  };
}

/** Cockpit endpoints of one party, with the token read at call time. */
export function studioApi(code: string, token: () => string) {
  const p = (path: string) => `/api/parties/${code}${path}`;
  return {
    get: <T>(path: string) => api.get<T>(p(path), token()),
    post: <T>(path: string, body?: unknown) => api.post<T>(p(path), body, token()),
    put: <T>(path: string, body?: unknown) => api.put<T>(p(path), body, token()),
    del: <T>(path: string) => api.del<T>(p(path), token()),
  };
}
export type StudioApi = ReturnType<typeof studioApi>;

/** Toggles an id in a capped list; the oldest drops out. */
export const toggle = (list: string[], id: string, cap: number) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id].slice(-cap));

export const asList = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []);
