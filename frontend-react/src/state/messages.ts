import type { WsMsg } from '@pc/lib/ws';
import type { RtcMsg } from '@pc/lib/rtc';
import type { Asset, CockpitState, LiveStream, MusicSnapshot, Party, ShowState, SubtitleLine, Tempo } from '@pc/lib/types';

/**
 * What actually arrives: Go encodes an empty (nil) slice as null, so every
 * list in a payload may be null on the wire even though the app types say
 * array. The reducer turns this into CockpitState.
 */
export type Wire<T> = { [K in keyof T]: T[K] extends readonly unknown[] ? T[K] | null : T[K] };

/** Everything the server sends a Cockpit socket (backend/internal/api). */
export type CockpitMsg =
  | { type: 'party'; data: Party }
  | { type: 'cockpit'; data: Wire<CockpitState> }
  | { type: 'queue'; data: Asset[] | null }
  | { type: 'music'; data: MusicSnapshot | null }
  | { type: 'tempo'; data: Tempo }
  | { type: 'reaction'; data: { emoji: string } }
  | { type: 'streams'; data: LiveStream[] | null }
  | { type: 'rtc'; data: RtcMsg }
  | { type: 'hello'; data: unknown }
  | { type: 'subtitle'; data: SubtitleLine }
  | { type: 'subtitle.clear'; data: unknown }
  | { type: 'show'; data: Wire<ShowState> };

export type CockpitMsgType = CockpitMsg['type'];

// A Record keyed by the union: adding a message type above without listing it
// here is a compile error, so the boundary below can never drift.
const KNOWN: Record<CockpitMsgType, true> = {
  party: true,
  cockpit: true,
  queue: true,
  music: true,
  tempo: true,
  reaction: true,
  streams: true,
  rtc: true,
  hello: true,
  subtitle: true,
  'subtitle.clear': true,
  show: true,
};

/**
 * The socket boundary. The server is trusted for shapes; what it sends to other
 * roles too (stage-only `pairing`, `game.*`, …) is dropped here.
 */
export function asCockpitMsg(m: WsMsg): CockpitMsg | null {
  return Object.hasOwn(KNOWN, m.type) ? (m as CockpitMsg) : null;
}
