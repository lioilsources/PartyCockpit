import { useState } from 'react';
import type { LiveStream } from '@pc/lib/types';
import { useCockpit } from '../CockpitContext';
import { useViewerMedia } from '../hooks/useViewer';
import { cx } from '../lib/format';
import { StreamVideo } from './StreamVideo';
import styles from './Streams.module.css';
import ui from './ui.module.css';

const NONE: LiveStream[] = [];
// the server never says the wall's limit (PC_MAX_LIVE); same number as the Svelte Cockpit
const WALL = 9;

/** Phones' cameras: the Cockpit previews pending ones too, so the operator sees who they let on the wall. */
export function Streams() {
  const { party, model, api, act, viewer } = useCockpit();
  const [previews, setPreviews] = useState(true);
  const streams = model.streams;
  const media = useViewerMedia(viewer, previews ? streams : NONE, party.iceServers);
  const live = streams.filter((s) => s.status === 'live').length;

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>
        Živé streamy ({live}/{WALL} na plátně)
        <label className={cx(ui.small, 'muted')}>
          <input
            type="checkbox"
            checked={previews}
            onChange={(e) => {
              setPreviews(e.target.checked);
            }}
          />{' '}
          náhledy
        </label>
      </h2>
      <div className={ui.cards}>
        {streams.length ? (
          streams.map((s) => {
            const m = media.get(s.id);
            return (
              <div key={s.id} className={cx(ui.card, styles.stream, s.status === 'live' && styles.live)}>
                {m ? <StreamVideo media={m} /> : <div className={styles.novid}>{s.online ? (previews ? 'připojuji…' : 'bez náhledu') : 'offline'}</div>}
                <div className={ui.meta}>
                  <strong>{s.name}</strong>
                  <span>
                    {s.source === 'mirrorbooth' ? '🪞 MirrorBooth' : '📡 prohlížeč'} · {s.status === 'live' ? '🔴 na plátně' : 'čeká'}
                  </span>
                </div>
                <div className={ui.row}>
                  {s.status === 'live' ? (
                    <button onClick={() => act(() => api.streamHold(s.id))}>stáhnout</button>
                  ) : (
                    <button className="ok" onClick={() => act(() => api.streamLive(s.id))}>
                      ✓ pustit
                    </button>
                  )}
                  <button className="bad" onClick={() => act(() => api.streamKick(s.id))} aria-label={`ukončit stream ${s.name}`}>
                    ✕
                  </button>
                </div>
              </div>
            );
          })
        ) : (
          <p className="muted">nikdo nevysílá — hosté: záložka „Živě“, nebo MirrorBooth</p>
        )}
      </div>
    </section>
  );
}
