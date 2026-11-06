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

export type SceneOptions = { beats: number; slideStyle: string; episode: string; pack: string; sweepTransition: boolean; shader: string };

/** Scene parameters exactly as the Svelte Cockpit sends them. */
export function sceneParams(kind: SceneKind, o: SceneOptions): SceneParams {
  const params: SceneParams = { beats: o.beats };
  if (kind === 'slideshow' && o.slideStyle) params.style = o.slideStyle;
  if (kind === 'webtoon' && o.episode) params.episode = o.episode;
  if (kind === 'stylesweep' && o.pack) params.pack = o.pack;
  else if (o.sweepTransition && o.pack) params.transition = o.pack;
  if (kind === 'interlude') {
    params.beats = Math.max(o.beats, 16);
    if (o.shader) params.shader = o.shader;
  }
  return params;
}
