import { useEffect, useSyncExternalStore } from 'react';
import { Viewer, type RtcMsg } from '@pc/lib/rtc';
import type { WsMsg } from '@pc/lib/ws';
import type { LiveStream } from '@pc/lib/types';
import { Signal } from '../state/signal';

type Media = ReadonlyMap<string, MediaStream>;

/**
 * Wraps the shared WebRTC Viewer (lib/rtc.ts) for React. The Viewer starts a
 * timer, so it is created in an effect (attach), not in render; render reads
 * the connected streams through a snapshot.
 */
export class ViewerStore {
  private viewer: Viewer | null = null;
  private ids: string[] = [];
  private wanted: LiveStream[] = [];
  private ice: RTCIceServer[] = [];
  readonly media = new Signal<Media>(() => this.collect());

  attach(send: (m: WsMsg) => boolean): () => void {
    const v = new Viewer(send, this.ice, this.media.emit);
    this.viewer = v;
    v.sync(this.wanted);
    return () => {
      v.destroy();
      if (this.viewer === v) this.viewer = null;
      this.media.emit();
    };
  }

  sync(wanted: LiveStream[], ice: RTCIceServer[]) {
    this.wanted = wanted;
    this.ice = ice;
    this.ids = wanted.map((s) => s.id);
    this.viewer?.setIce(ice);
    this.viewer?.sync(wanted);
    this.media.emit();
  }

  handle = (m: RtcMsg) => {
    this.viewer?.handle(m);
  };

  reset = () => {
    this.viewer?.reset();
  };

  private collect(): Media {
    const out = new Map<string, MediaStream>();
    for (const id of this.ids) {
      const m = this.viewer?.media(id);
      if (m) out.set(id, m);
    }
    return out;
  }
}

/** Ties the Viewer's lifetime to the socket's `send`. */
export function useViewerAttach(store: ViewerStore, send: (m: WsMsg) => boolean) {
  useEffect(() => store.attach(send), [store, send]);
}

/** Receives `wanted` and returns the connected ones by stream id. */
export function useViewerMedia(store: ViewerStore, wanted: LiveStream[], ice: RTCIceServer[] | undefined): Media {
  useEffect(() => {
    store.sync(wanted, ice ?? []);
  }, [store, wanted, ice]);
  return useSyncExternalStore(store.media.subscribe, store.media.getSnapshot);
}
