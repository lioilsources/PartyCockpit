// Peer-to-peer WebRTC over the party socket (backend/internal/api/streams.go).
// The viewer (stage, cockpit) offers to every stream it shows; the phone answers.
// LAN only: no SFU, host candidates are enough (DECISIONS.md D12).

import type { WsMsg } from './ws';
import type { LiveStream } from './types';

export type RtcMsg = {
  kind: 'offer' | 'answer' | 'ice' | 'bye';
  stream?: string;
  to?: string;
  from?: string;
  sdp?: string;
  candidate?: RTCIceCandidateInit;
};

type Send = (m: WsMsg) => boolean;
const rtc = (data: RtcMsg): WsMsg => ({ type: 'rtc', data });

// ── viewer ──────────────────────────────────────────────────────────────────

type Incoming = {
  pc: RTCPeerConnection;
  media: MediaStream;
  op: Promise<unknown>; // serialises SDP/ICE handling for this connection
  started: number;
  connected: boolean;
};

/** Receives the given streams; call sync() whenever the list changes. */
export class Viewer {
  private conns = new Map<string, Incoming>();
  private wanted: LiveStream[] = [];
  private timer: ReturnType<typeof setInterval>;

  constructor(
    private send: Send,
    private ice: RTCIceServer[],
    private onChange: () => void,
  ) {
    // watchdog: re-offer to streams that never connected or dropped
    this.timer = setInterval(() => this.heal(), 3000);
  }

  setIce(ice: RTCIceServer[] | null | undefined) {
    this.ice = ice ?? [];
  }

  media(id: string): MediaStream | undefined {
    const c = this.conns.get(id);
    return c?.connected ? c.media : undefined;
  }

  sync(streams: LiveStream[]) {
    this.wanted = streams.filter((s) => s.online);
    const ids = new Set(this.wanted.map((s) => s.id));
    for (const id of [...this.conns.keys()]) if (!ids.has(id)) this.drop(id, true);
    for (const s of this.wanted) if (!this.conns.has(s.id)) this.open(s.id);
  }

  handle(m: RtcMsg) {
    const c = m.stream ? this.conns.get(m.stream) : undefined;
    if (!c) return;
    if (m.kind === 'answer' && m.sdp) {
      c.op = c.op.then(() => c.pc.setRemoteDescription({ type: 'answer', sdp: m.sdp })).catch(() => {});
    } else if (m.kind === 'ice' && m.candidate) {
      c.op = c.op.then(() => c.pc.addIceCandidate(m.candidate)).catch(() => {});
    } else if (m.kind === 'bye') {
      this.drop(m.stream!, false);
    }
  }

  /** The socket reconnected: every connection needs a fresh offer. */
  reset() {
    for (const id of [...this.conns.keys()]) this.drop(id, false);
    this.sync(this.wanted);
  }

  destroy() {
    clearInterval(this.timer);
    for (const id of [...this.conns.keys()]) this.drop(id, true);
  }

  private open(id: string) {
    const pc = new RTCPeerConnection({ iceServers: this.ice });
    const c: Incoming = { pc, media: new MediaStream(), op: Promise.resolve(), started: Date.now(), connected: false };
    this.conns.set(id, c);
    pc.addTransceiver('video', { direction: 'recvonly' }); // video only: the music is on the speakers
    pc.ontrack = (e) => {
      c.media.addTrack(e.track);
      this.onChange();
    };
    pc.onicecandidate = (e) => {
      if (e.candidate) this.send(rtc({ kind: 'ice', stream: id, candidate: e.candidate.toJSON() }));
    };
    pc.onconnectionstatechange = () => {
      const was = c.connected;
      c.connected = pc.connectionState === 'connected';
      if (was !== c.connected) this.onChange();
    };
    c.op = pc
      .createOffer()
      .then((o) => pc.setLocalDescription(o))
      .then(() => this.send(rtc({ kind: 'offer', stream: id, sdp: pc.localDescription!.sdp })))
      .catch(() => {});
  }

  private drop(id: string, tell: boolean) {
    const c = this.conns.get(id);
    if (!c) return;
    this.conns.delete(id);
    if (tell) this.send(rtc({ kind: 'bye', stream: id }));
    c.pc.close();
    this.onChange();
  }

  private heal() {
    const now = Date.now();
    for (const [id, c] of this.conns) {
      const st = c.pc.connectionState;
      const stuck = !c.connected && now - c.started > 8000;
      if (st === 'failed' || st === 'closed' || stuck) {
        this.drop(id, true);
        this.open(id);
      }
    }
    // dropped by the phone ("bye") but still listed: try again
    for (const s of this.wanted) if (!this.conns.has(s.id)) this.open(s.id);
  }
}

// ── publisher (phone) ───────────────────────────────────────────────────────

type Outgoing = { pc: RTCPeerConnection; op: Promise<unknown> };

/** Answers every viewer's offer with the phone's (filtered) camera track. */
export class Publisher {
  private conns = new Map<string, Outgoing>();

  constructor(
    private send: Send,
    private ice: RTCIceServer[],
    private media: MediaStream,
    private maxBitrate = 1_200_000,
  ) {}

  get viewers() {
    return [...this.conns.values()].filter((c) => c.pc.connectionState === 'connected').length;
  }

  handle(m: RtcMsg) {
    const from = m.from;
    if (!from) return;
    if (m.kind === 'offer' && m.sdp) {
      this.drop(from);
      this.answer(from, m.sdp);
    } else if (m.kind === 'ice' && m.candidate) {
      const c = this.conns.get(from);
      if (c) c.op = c.op.then(() => c.pc.addIceCandidate(m.candidate)).catch(() => {});
    } else if (m.kind === 'bye') {
      this.drop(from);
    }
  }

  /** Swap the outgoing track (e.g. the camera was restarted) without renegotiating. */
  setMedia(media: MediaStream) {
    this.media = media;
    const track = media.getVideoTracks()[0];
    for (const c of this.conns.values()) {
      for (const s of c.pc.getSenders()) if (s.track?.kind === 'video' || !s.track) s.replaceTrack(track ?? null);
    }
  }

  close() {
    for (const id of [...this.conns.keys()]) {
      this.send(rtc({ kind: 'bye', to: id }));
      this.drop(id);
    }
  }

  private answer(from: string, sdp: string) {
    const pc = new RTCPeerConnection({ iceServers: this.ice });
    const c: Outgoing = { pc, op: Promise.resolve() };
    this.conns.set(from, c);
    for (const t of this.media.getVideoTracks()) pc.addTrack(t, this.media);
    pc.onicecandidate = (e) => {
      if (e.candidate) this.send(rtc({ kind: 'ice', to: from, candidate: e.candidate.toJSON() }));
    };
    pc.onconnectionstatechange = () => {
      if (pc.connectionState === 'failed' || pc.connectionState === 'closed') this.drop(from);
    };
    c.op = pc
      .setRemoteDescription({ type: 'offer', sdp })
      .then(() => pc.createAnswer())
      .then((a) => pc.setLocalDescription(a))
      .then(() => {
        this.send(rtc({ kind: 'answer', to: from, sdp: pc.localDescription!.sdp }));
        return this.capBitrate(pc);
      })
      .catch(() => this.drop(from));
  }

  // every viewer gets its own encode; keep a 3×3 wall within LAN Wi-Fi
  private async capBitrate(pc: RTCPeerConnection) {
    for (const s of pc.getSenders()) {
      const p = s.getParameters();
      if (!p.encodings?.length) continue;
      p.encodings[0].maxBitrate = this.maxBitrate;
      await s.setParameters(p).catch(() => {});
    }
  }

  private drop(id: string) {
    const c = this.conns.get(id);
    if (!c) return;
    this.conns.delete(id);
    c.pc.close();
  }
}

/** Svelte action: <video use:srcObject={media}>. */
export function srcObject(node: HTMLVideoElement, media: MediaStream | undefined | null) {
  const set = (m: MediaStream | undefined | null) => {
    if (node.srcObject !== (m ?? null)) node.srcObject = m ?? null;
    if (m) node.play().catch(() => {});
  };
  set(media);
  return { update: set };
}
