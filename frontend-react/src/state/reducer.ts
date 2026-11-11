import type { Asset, CockpitState, LiveStream, MusicSnapshot, Party, ShowState, SubtitleLine, Tempo } from '@pc/lib/types';
import type { CockpitMsg, Wire } from './messages';

export type CockpitModel = {
  party: Party | null;
  st: CockpitState | null;
  queue: Asset[];
  music: MusicSnapshot | null;
  tempo: Tempo;
  /** newest first */
  reactions: string[];
  streams: LiveStream[];
  /** newest first; one id is updated several times while the MC speaks */
  subs: SubtitleLine[];
  /** the Studio show and where it plays (D23) */
  show: ShowState | null;
};

export const MAX_REACTIONS = 14;
export const MAX_SUBS = 6;

export const initialModel: CockpitModel = {
  party: null,
  st: null,
  queue: [],
  music: null,
  tempo: { bpm: 0, source: 'none', anchor: 0 },
  reactions: [],
  streams: [],
  subs: [],
  show: null,
};

const lists = (st: Wire<CockpitState>): CockpitState => ({
  ...st,
  assets: st.assets ?? [],
  guests: st.guests ?? [],
  ideas: st.ideas ?? [],
  jobs: st.jobs ?? [],
});

/**
 * Pure state of the Cockpit, driven by socket messages only. The server sends
 * the full state on every (re)connect, so there is nothing to merge on resume.
 */
export function cockpitReducer(s: CockpitModel, m: CockpitMsg): CockpitModel {
  switch (m.type) {
    case 'party':
      return { ...s, party: m.data };
    case 'cockpit':
      return { ...s, st: lists(m.data) };
    case 'queue':
      return { ...s, queue: m.data ?? [] };
    case 'music':
      return { ...s, music: m.data };
    case 'tempo':
      return { ...s, tempo: m.data };
    case 'reaction':
      return { ...s, reactions: [m.data.emoji, ...s.reactions].slice(0, MAX_REACTIONS) };
    case 'streams':
      return { ...s, streams: m.data ?? [] };
    case 'subtitle': {
      const i = s.subs.findIndex((x) => x.id === m.data.id);
      return { ...s, subs: i >= 0 ? s.subs.with(i, m.data) : [m.data, ...s.subs].slice(0, MAX_SUBS) };
    }
    case 'subtitle.clear':
      return { ...s, subs: [] };
    case 'show':
      return { ...s, show: { ...m.data, entries: m.data.entries ?? [] } };
    case 'rtc':
    case 'hello':
      // WebRTC signalling is a side effect of the socket hook, not state
      return s;
  }
}
