import { useEffect, useState, useSyncExternalStore } from 'react';
import { Mic, type MicState } from '@pc/lib/mic';
import { Signal } from '../state/signal';

export type MicSnapshot = { state: MicState; level: number; error: string };

class MicStore {
  // the callback is only called later, after `signal` exists
  readonly mic = new Mic(() => {
    this.signal.emit();
  });
  readonly signal = new Signal<MicSnapshot>(() => ({ state: this.mic.state, level: this.mic.level, error: this.mic.error }));
}

/** The MC microphone for subtitles (lib/mic.ts); stopped when the Cockpit unmounts. */
export function useMic() {
  const [store] = useState(() => new MicStore());
  const snap = useSyncExternalStore(store.signal.subscribe, store.signal.getSnapshot);
  useEffect(
    () => () => {
      store.mic.stop();
    },
    [store],
  );
  return { ...snap, mic: store.mic, active: snap.state !== 'off' && snap.state !== 'error' };
}
