import type { SceneKind } from '@pc/lib/types';
import type { SceneParams } from '../api/cockpitApi';

export const SCENES: readonly { kind: SceneKind; label: string }[] = [
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

export const SHADERS = [
  ['', '(střídat)'],
  ['plasma', 'Plazma'],
  ['tunnel', 'Tunel'],
  ['kaleido', 'Kaleidoskop'],
  ['bars', 'Ekvalizér'],
  ['aurora', 'Polární záře'],
] as const;

export type SceneOptions = {
  beats: number;
  slideStyle: string;
  /** Webtoon columns (D23), up to 4; empty = the first episode. */
  episodes: string[];
  wtLang: string;
  wtSpeed: number;
  pack: string;
  sweepTransition: boolean;
  shader: string;
};

/** Bubble languages offered for webtoon packs. */
export const WEBTOON_LANGS = ['cs', 'en', 'de', 'fr', 'it', 'pl', 'uk'] as const;

/** Toggles an episode column; the oldest drops out past 4 (as in Svelte). */
export const toggledEpisode = (list: readonly string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id].slice(-4));

/** Scene parameters exactly as the Svelte Cockpit sends them. */
export function sceneParams(kind: SceneKind, o: SceneOptions): SceneParams {
  const params: SceneParams = { beats: o.beats };
  if (kind === 'slideshow' && o.slideStyle) params.style = o.slideStyle;
  if (kind === 'webtoon') {
    if (o.episodes.length) params.episodes = [...o.episodes];
    params.lang = o.wtLang;
    params.speed = o.wtSpeed;
  }
  if (kind === 'stylesweep' && o.pack) params.pack = o.pack;
  else if (o.sweepTransition && o.pack) params.transition = o.pack;
  if (kind === 'interlude') {
    params.beats = Math.max(o.beats, 16);
    if (o.shader) params.shader = o.shader;
  }
  return params;
}
