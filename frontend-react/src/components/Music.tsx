import { useState, type SubmitEvent } from 'react';
import { TapTempo } from '@pc/lib/beat';
import { useCockpit } from '../CockpitContext';
import styles from './Music.module.css';
import ui from './ui.module.css';

const SOURCE = { tap: 'tap', djfy: 'z DJfy', none: 'plátno poslouchá audio / idle' } as const;

function TapButton() {
  const { api, act } = useCockpit();
  const [tapper] = useState(() => new TapTempo());
  const [taps, setTaps] = useState(0);
  const tap = () => {
    const r = tapper.tap();
    setTaps(tapper.count);
    if (r) void act(() => api.tempo(r));
  };
  return (
    <button className={styles.tap} onClick={tap}>
      TAP {taps > 0 && taps < 4 ? `(${taps}/4)` : ''}
    </button>
  );
}

export function Music() {
  const { party, model, api, act } = useCockpit();
  const { music, tempo, reactions } = model;
  // seeded once from the party; the operator edits it before connecting
  const [djfyCode, setDjfyCode] = useState(party.djfyCode);
  const connect = (e: SubmitEvent) => {
    e.preventDefault();
    void act(() => api.patch({ djfyCode }));
  };
  const disconnect = () => {
    setDjfyCode('');
    void act(() => api.patch({ djfyCode: '' }));
  };

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>Hudba (DJfy)</h2>
      <form className={ui.row} onSubmit={connect}>
        <input
          className={styles.code}
          placeholder="kód DJfy party"
          value={djfyCode}
          onChange={(e) => {
            setDjfyCode(e.target.value);
          }}
        />
        <button>Připojit</button>
        {party.djfyCode && (
          <button type="button" onClick={disconnect}>
            Odpojit
          </button>
        )}
      </form>
      {music?.nowPlaying ? (
        <p>
          ▶ <strong>{music.nowPlaying.title}</strong> — {music.nowPlaying.artist}{' '}
          <span className="muted">
            · fronta {music.queue.length} · hosté DJfy {music.guests}
          </span>
        </p>
      ) : (
        party.djfyCode && <p className="muted">čekám na snapshot z telefonu s DJfy…</p>
      )}
      <div className={ui.row}>
        <TapButton />
        <span>
          {tempo.bpm > 0 ? `${tempo.bpm} BPM` : 'bez tempa'} <span className="muted">({SOURCE[tempo.source]})</span>
        </span>
        {tempo.source === 'tap' && <button onClick={() => act(() => api.tempo({ bpm: 0 }))}>zrušit tap</button>}
      </div>
      {reactions.length > 0 && <p className={styles.react}>{reactions.join(' ')}</p>}
    </section>
  );
}
