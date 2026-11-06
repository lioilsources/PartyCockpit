import { useEffect, useState } from 'react';
import { plural, type Check } from '@pc/lib/types';
import { useCockpit } from '../CockpitContext';
import { asJson, useStoredState } from '../hooks/useStoredState';
import { cx } from '../lib/format';
import styles from './Preflight.module.css';

const MANUAL = [
  { id: 'audio', label: 'Zvuk do plátna: loopback (BlackHole / VB-Cable) nebo mikrofon, plátno s ?audio=1' },
  { id: 'kiosk', label: 'Plátno v kiosku na celou obrazovku, spořič a uspávání vypnuté' },
  { id: 'qr', label: 'QR na plátně vyzkoušené vlastním telefonem přes Wi-Fi hostů' },
  { id: 'tablet', label: 'Cockpit na tabletu, nabitý, PANIC vyzkoušený' },
  { id: 'gpu', label: 'GPU okno domluvené (L40S ComfyUI na animace, Restyler na fotky)' },
  { id: 'djfy', label: 'DJfy hraje z hostitelského telefonu, kód zadaný v Cockpitu' },
] as const;

const runbookCodec = asJson<Partial<Record<string, boolean>>>();

function Summary({ checks }: { checks: Check[] }) {
  const problems = checks.filter((c) => c.status === 'fail').length;
  const warnings = checks.filter((c) => c.status === 'warn').length;
  const text = problems
    ? `${problems} ${plural(problems, 'problém', 'problémy', 'problémů')}`
    : warnings
      ? `${warnings} ${plural(warnings, 'upozornění', 'upozornění', 'upozornění')}`
      : 'v pořádku';
  return <span className={cx(styles.summary, problems > 0 && styles.bad, !problems && warnings > 0 && styles.warn)}>{text}</span>;
}

/** Two clicks instead of confirm(): a modal dialog would block a kiosk and browser automation. */
function RotateToken() {
  const { api, act, replaceToken } = useCockpit();
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => {
      setArmed(false);
    }, 4000);
    return () => {
      clearTimeout(t);
    };
  }, [armed]);
  const click = () => {
    if (!armed) {
      setArmed(true);
      return;
    }
    setArmed(false);
    void act(async () => {
      replaceToken(await api.rotate());
    });
  };
  return (
    <button className={cx(armed && 'bad')} onClick={click} title="Ostatní otevřené Cockpity se odpojí a budou potřebovat nový odkaz.">
      {armed ? 'Opravdu vyměnit? Ostatní Cockpity se odpojí' : 'Vyměnit cockpit token'}
    </button>
  );
}

export function Preflight() {
  const { code, api, act, setErr } = useCockpit();
  const [checks, setChecks] = useState<Check[]>([]);
  const [checking, setChecking] = useState(false);
  const [done, setDone] = useStoredState(`pc.runbook.${code}`, {}, runbookCodec);

  const run = async () => {
    setChecking(true);
    try {
      setChecks(await api.preflight());
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setChecking(false);
    }
  };

  return (
    <details
      className={styles.preflight}
      onToggle={(e) => {
        if (e.currentTarget.open && !checks.length) void run();
      }}
    >
      <summary>
        ✅ Před party
        {checks.length > 0 && <Summary checks={checks} />}
      </summary>
      <div className={styles.pf}>
        <div>
          <h3>
            Automatické kontroly{' '}
            <button onClick={() => void run()} disabled={checking}>
              {checking ? '…' : '↻'}
            </button>
          </h3>
          <ul>
            {checks.map((c) => (
              <li key={c.id} className={styles[c.status]}>
                <b>{c.label}</b> {c.detail}
              </li>
            ))}
          </ul>
        </div>
        <div>
          <h3>Ručně</h3>
          <p className={cx('muted', styles.small)}>
            Token unikl (fotka tabletu)? <RotateToken />
          </p>
          <p className={cx('muted', styles.small)}>
            Cockpit do telefonu? <button onClick={() => act(api.pair)}>Párovací QR na plátno</button> (10 min)
          </p>
          <ul>
            {MANUAL.map((m) => (
              <li key={m.id}>
                <label>
                  <input
                    type="checkbox"
                    checked={!!done[m.id]}
                    onChange={() => {
                      setDone((d) => ({ ...d, [m.id]: !d[m.id] }));
                    }}
                  />{' '}
                  {m.label}
                </label>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </details>
  );
}
