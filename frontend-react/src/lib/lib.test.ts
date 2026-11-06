import { describe, expect, it } from 'vitest';
import { readUrlToken } from '../hooks/useCockpitAuth';
import { parseRoute } from '../route';
import { ago, cx, extendedEnd } from './format';
import { moved } from './queue';
import { sceneParams } from './scene';

const opts = { beats: 8, slideStyle: '', episode: '', pack: '', sweepTransition: false, shader: '' };

describe('sceneParams', () => {
  it('matches what the Svelte Cockpit sends', () => {
    expect(sceneParams('slideshow', { ...opts, slideStyle: 'ghibli' })).toEqual({ beats: 8, style: 'ghibli' });
    expect(sceneParams('interlude', { ...opts, shader: 'tunnel' })).toEqual({ beats: 16, shader: 'tunnel' });
    expect(sceneParams('stylesweep', { ...opts, pack: 'cat' })).toEqual({ beats: 8, pack: 'cat' });
    expect(sceneParams('gallery', { ...opts, pack: 'cat', sweepTransition: true })).toEqual({ beats: 8, transition: 'cat' });
    expect(sceneParams('webtoon', { ...opts, episode: 'party' })).toEqual({ beats: 8, episode: 'party' });
  });
});

describe('helpers', () => {
  it('moves queue items within bounds', () => {
    expect(moved(['a', 'b', 'c'], 1, -1)).toEqual(['b', 'a', 'c']);
    expect(moved(['a', 'b'], 0, -1)).toBeNull();
    expect(moved(['a', 'b'], 1, 1)).toBeNull();
  });

  it('extends the party by 2 h, at most 24 h from now', () => {
    const h = 3600e3;
    expect(extendedEnd(10 * h, 9 * h)).toBe(12 * h);
    expect(extendedEnd(5 * h, 9 * h)).toBe(11 * h); // ended already: from now
    expect(extendedEnd(40 * h, 18 * h)).toBe(42 * h - 60e3);
  });

  it('formats', () => {
    expect(ago(0, 42_000)).toBe('42 s');
    expect(ago(0, 180_000)).toBe('3 min');
    expect(cx('a', false, 'b', undefined)).toBe('a b');
  });

  it('reads the token from the fragment first', () => {
    expect(readUrlToken({ hash: '#token=frag', search: '?token=query' })).toBe('frag');
    expect(readUrlToken({ hash: '', search: '?token=query' })).toBe('query');
    expect(readUrlToken({ hash: '', search: '' })).toBeNull();
  });

  it('parses the route under the Vite base', () => {
    expect(parseRoute('/react/cockpit/abcd', '/react/')).toEqual({ code: 'ABCD' });
    expect(parseRoute('/react/', '/react/')).toBeNull();
    expect(parseRoute('/cockpit/ABCD', '/react/')).toBeNull();
  });
});
