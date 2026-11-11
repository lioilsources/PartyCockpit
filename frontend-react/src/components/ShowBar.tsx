import { useCockpit } from '../CockpitContext';
import { useNow } from '../hooks/useNow';
import ui from './ui.module.css';

/** The Studio show (D23) as one row: where it plays and ⏮ ▶/⏸ ⏭. Editing is in the Studio. */
export function ShowBar() {
  const { code, api, act, model } = useCockpit();
  const show = model.show;
  const now = useNow(1000);
  if (!show?.entries.length) {
    return (
      <div className={ui.row}>
        <span className="muted">🎬 Žádná show.</span>
        <a href={`/studio/${code}`} target="_blank" rel="noreferrer">
          Připravit ve Studiu ↗
        </a>
      </div>
    );
  }
  const cur = show.entries.at(show.index);
  const left = Math.max(0, Math.round((show.endsAt - now) / 1000));
  return (
    <div className={ui.row}>
      <span>
        🎬 {show.name || 'Show'}: {show.playing && cur ? `hraje ${String(show.index + 1)}/${String(show.entries.length)} · ${cur.title || cur.kind} · ${String(left)} s` : 'stojí'}
      </span>
      <button onClick={() => act(() => api.show('prev'))} title="předchozí scéna">
        ⏮
      </button>
      {show.playing ? (
        <button className="on" onClick={() => act(() => api.show('pause'))}>
          ⏸
        </button>
      ) : (
        <button onClick={() => act(() => api.show('play'))}>▶</button>
      )}
      <button onClick={() => act(() => api.show('next'))} title="další scéna">
        ⏭
      </button>
      <a href={`/studio/${code}`} target="_blank" rel="noreferrer">
        Studio ↗
      </a>
    </div>
  );
}
