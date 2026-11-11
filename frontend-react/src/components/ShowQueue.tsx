import { fileUrl, type Asset } from '@pc/lib/types';
import { useCockpit } from '../CockpitContext';
import { cx } from '../lib/format';
import { moved } from '../lib/queue';
import { Clip } from './Clip';
import styles from './ShowQueue.module.css';
import ui from './ui.module.css';

function label(a: Asset) {
  const what = a.kind === 'sticker' ? a.meta.text : a.kind === 'dance' ? `💃 ${a.meta.motionLabel ?? ''}` : a.meta.styleLabel;
  return `${a.meta.nickname ?? ''} · ${what ?? ''}${a.live ? ' · ✨ živá' : ''}`;
}

export function ShowQueue() {
  const { code, party, st, model, api, act, setErr } = useCockpit();
  const queue = model.queue;
  const animating = (st?.assets ?? []).filter((a) => a.status === 'processing' && (a.kind === 'live' || a.kind === 'breeze')).length;

  const move = (i: number, d: number) => {
    const ids = moved(
      queue.map((a) => a.id),
      i,
      d,
    );
    if (ids) void act(() => api.reorder(ids));
  };
  const animateAll = () =>
    act(async () => {
      const r = await api.animate();
      if (!r.queued) setErr('všechny fotky už živé jsou (nebo se oživují)');
    });
  const breezeAll = () =>
    act(async () => {
      const r = await api.animate('breeze');
      if (!r.queued) setErr('všechny fotky už vlasy ve větru mají (nebo se dělají)');
    });

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>Show fronta ({queue.length})</h2>
      <div className={cx(ui.row, ui.small)}>
        <label>
          <input type="checkbox" checked={party.autoLive} onChange={(e) => act(() => api.patch({ autoLive: e.target.checked }))} /> oživovat schválené fotky
        </label>
        <button onClick={() => void animateAll()}>✨ Oživit galerii</button>
        <button title="Wan na GPU, asi minuta na fotku; klip čeká na schválení" onClick={() => void breezeAll()}>
          💨 Rozfoukat vlasy
        </button>
        {animating > 0 && <span className="muted">oživuje se {animating}</span>}
      </div>
      <div className={cx(ui.row, ui.small)}>
        <span className="muted">po doběhnutí:</span>
        <button className={cx(party.loopMode === 'loop' && 'on')} onClick={() => act(() => api.patch({ loopMode: 'loop' }))}>
          od začátku
        </button>
        <button className={cx(party.loopMode === 'shuffle' && 'on')} onClick={() => act(() => api.patch({ loopMode: 'shuffle' }))}>
          shuffle
        </button>
      </div>
      <ol className={styles.queue}>
        {queue.length ? (
          queue.map((a, i) => (
            <li key={a.id}>
              {a.kind === 'sticker' ? (
                <span className={styles.thumb} style={{ background: a.meta.color }}>
                  ✍️
                </span>
              ) : (
                <Clip src={fileUrl(code, a.live ?? a.file)} />
              )}
              <span className={styles.name}>{label(a)}</span>
              <button
                onClick={() => {
                  move(i, -1);
                }}
                aria-label="nahoru"
              >
                ↑
              </button>
              <button
                onClick={() => {
                  move(i, 1);
                }}
                aria-label="dolů"
              >
                ↓
              </button>
              <button className="bad" onClick={() => act(() => api.unqueue(a.id))} aria-label="vyřadit z fronty">
                ✕
              </button>
            </li>
          ))
        ) : (
          <li className="muted">prázdná — schválený obsah se sem řadí</li>
        )}
      </ol>
    </section>
  );
}
