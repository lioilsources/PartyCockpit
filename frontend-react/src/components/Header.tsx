import { useCockpit } from '../CockpitContext';
import { clockTime, cx, extendedEnd } from '../lib/format';
import styles from './Header.module.css';

export function Header() {
  const { code, party, st, online, api, act } = useCockpit();
  const stageUrl = `/stage/${code}?key=${st?.config.stageKey ?? ''}`;
  return (
    <header className={styles.header}>
      <div>
        <h1>
          {party.name} <code>{party.code}</code>
          <span className={styles.badge} title="Druhá implementace Cockpitu (D21)">
            React
          </span>
        </h1>
        <span className={cx(styles.dot, online && styles.on)} aria-label={online ? 'online' : 'offline'}></span>
        <span className="muted">
          plátno {st?.clients.stage ?? 0} · hosté {st?.guests.length ?? 0} (online {st?.clients.guest ?? 0}) · restyle: {st?.config.restyle} · animace:{' '}
          {st?.config.animate}
        </span>
      </div>
      <div className={styles.links}>
        <a href={stageUrl} target="_blank" rel="noreferrer">
          Otevřít plátno ↗
        </a>
        <a href={party.joinUrl} target="_blank" rel="noreferrer">
          {party.joinUrl}
        </a>
      </div>
      <span className={styles.ends}>
        končí {clockTime(party.endsAt)}
        <button onClick={() => act(() => api.patch({ endsAt: extendedEnd(party.endsAt, Date.now()) }))}>+2 h</button>
      </span>
      <button className={cx('bad', styles.panic)} onClick={() => act(api.panic)}>
        PANIC
      </button>
    </header>
  );
}
