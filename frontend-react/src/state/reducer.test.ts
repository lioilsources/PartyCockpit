import { describe, expect, it } from 'vitest';
import type { SubtitleLine } from '@pc/lib/types';
import { asCockpitMsg } from './messages';
import { cockpitReducer, initialModel, MAX_REACTIONS, MAX_SUBS } from './reducer';
import { cockpitState, party } from '../test/fixtures';

const line = (id: number, text: string, final = false): SubtitleLine => ({ id, text, final, done: final });

describe('cockpitReducer', () => {
  it('replaces whole slices, as the server sends them', () => {
    const s = cockpitReducer(initialModel, { type: 'party', data: party() });
    expect(s.party?.code).toBe('ABCD');
    expect(cockpitReducer(s, { type: 'queue', data: null }).queue).toEqual([]);
    expect(cockpitReducer(s, { type: 'streams', data: null }).streams).toEqual([]);
  });

  it('reads null lists from the wire as empty', () => {
    const s = cockpitReducer(initialModel, { type: 'cockpit', data: { ...cockpitState(), guests: null, jobs: null } });
    expect(s.st?.guests).toEqual([]);
    expect(s.st?.jobs).toEqual([]);
  });

  it('keeps the newest reactions first, capped', () => {
    let s = initialModel;
    for (let i = 0; i < MAX_REACTIONS + 3; i++) s = cockpitReducer(s, { type: 'reaction', data: { emoji: String(i) } });
    expect(s.reactions).toHaveLength(MAX_REACTIONS);
    expect(s.reactions[0]).toBe(String(MAX_REACTIONS + 2));
  });

  it('updates a subtitle line in place and caps new ones', () => {
    let s = cockpitReducer(initialModel, { type: 'subtitle', data: line(1, 'ahoj') });
    s = cockpitReducer(s, { type: 'subtitle', data: line(2, 'druhý') });
    s = cockpitReducer(s, { type: 'subtitle', data: line(1, 'ahoj všichni', true) });
    expect(s.subs.map((l) => l.text)).toEqual(['druhý', 'ahoj všichni']);
    for (let i = 3; i < 3 + MAX_SUBS; i++) s = cockpitReducer(s, { type: 'subtitle', data: line(i, `${i}`) });
    expect(s.subs).toHaveLength(MAX_SUBS);
    expect(cockpitReducer(s, { type: 'subtitle.clear', data: null }).subs).toEqual([]);
  });

  it('keeps the show, its entries never null', () => {
    const s = cockpitReducer(initialModel, {
      type: 'show',
      data: { name: 'Večer', entries: null, loop: true, updatedAt: 1, playing: false, index: 0, entryId: '', startedAt: 0, endsAt: 0 },
    });
    expect(s.show?.entries).toEqual([]);
    expect(s.show?.name).toBe('Večer');
  });

  it('leaves state alone for signalling', () => {
    expect(cockpitReducer(initialModel, { type: 'hello', data: null })).toBe(initialModel);
    expect(cockpitReducer(initialModel, { type: 'rtc', data: { kind: 'bye', stream: 's1' } })).toBe(initialModel);
  });
});

describe('asCockpitMsg', () => {
  it('drops messages meant for other roles', () => {
    expect(asCockpitMsg({ type: 'pairing', data: null })).toBeNull();
    expect(asCockpitMsg({ type: 'game.slot', data: {} })).toBeNull();
    expect(asCockpitMsg({ type: 'tempo', data: { bpm: 120, source: 'tap', anchor: 1 } })?.type).toBe('tempo');
  });
});
