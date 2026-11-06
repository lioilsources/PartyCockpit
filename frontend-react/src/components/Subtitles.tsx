import { useState, type SubmitEvent } from 'react';
import { useCockpit } from '../CockpitContext';
import { useMic } from '../hooks/useMic';
import { asString, useStoredState } from '../hooks/useStoredState';
import { cx } from '../lib/format';
import styles from './Subtitles.module.css';
import ui from './ui.module.css';

const LANGS = [
  ['cs', 'čeština'],
  ['sk', 'slovenčina'],
  ['en', 'angličtina'],
  ['de', 'němčina'],
  ['uk', 'ukrajinština'],
  ['pl', 'polština'],
  ['es', 'španělština'],
] as const;

/** MC microphone → Translator → caption bar on the stage (M7), plus typed lines. */
export function Subtitles() {
  const { code, st, model, api, act, setErr } = useCockpit();
  const mic = useMic();
  const [langA, setLangA] = useStoredState('pc.sub.langA', 'cs', asString);
  const [langB, setLangB] = useStoredState('pc.sub.langB', 'en', asString);
  const [typed, setTyped] = useState('');
  const secure = window.isSecureContext;
  const subs = model.subs;

  const toggleMic = () => {
    if (mic.active) {
      mic.mic.stop();
      return;
    }
    api
      .ticket()
      .then((t) => mic.mic.start(code, t, langA, langB))
      .catch((e: unknown) => {
        setErr(e instanceof Error ? e.message : String(e));
      });
  };
  const sendTyped = (e: SubmitEvent) => {
    e.preventDefault();
    const text = typed.trim();
    if (!text) return;
    void act(async () => {
      await api.subtitle(text);
      setTyped('');
    });
  };

  return (
    <section className={ui.section}>
      <h2 className={ui.h2}>
        🎤 Titulky na plátno
        {subs.length > 0 && <button onClick={() => act(api.clearSubtitles)}>smazat</button>}
      </h2>
      {st?.config.subtitles ? (
        <>
          <div className={cx(ui.row, ui.small)}>
            <label>
              mluvím
              <select
                value={langA}
                disabled={mic.active}
                onChange={(e) => {
                  setLangA(e.target.value);
                }}
              >
                {LANGS.map(([id, label]) => (
                  <option key={id} value={id}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              překlad do
              <select
                value={langB}
                disabled={mic.active}
                onChange={(e) => {
                  setLangB(e.target.value);
                }}
              >
                <option value={langA}>(bez překladu)</option>
                {LANGS.filter(([id]) => id !== langA).map(([id, label]) => (
                  <option key={id} value={id}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className={ui.row}>
            <button className={mic.active ? 'bad' : 'primary'} onClick={toggleMic} disabled={!secure}>
              {mic.active ? '⏹ Vypnout' : '🎤 Zapnout mikrofon'}
            </button>
            {mic.active && (
              <>
                <span className={styles.meter}>
                  <span style={{ width: `${Math.round(mic.level * 100)}%` }} className={cx(mic.state === 'speaking' && styles.on)}></span>
                </span>
                <span className="muted">{mic.state === 'starting' ? 'připojuji…' : mic.state === 'speaking' ? 'mluvíš' : 'poslouchám'}</span>
              </>
            )}
          </div>
          {!secure && <p className={cx('muted', ui.small)}>Mikrofon v prohlížeči jde jen přes https (nebo localhost).</p>}
          {mic.state === 'error' && <p className="err">{mic.error}</p>}
        </>
      ) : (
        <p className={cx('muted', ui.small)}>Mikrofon potřebuje Translator (TRANSLATOR_URL). Psát titulky jde i bez něj.</p>
      )}
      <form className={ui.row} onSubmit={sendTyped}>
        <input
          placeholder="napsat na plátno (vzkaz, oznámení)"
          maxLength={160}
          value={typed}
          onChange={(e) => {
            setTyped(e.target.value);
          }}
        />
        <button disabled={!typed.trim()}>Na plátno</button>
      </form>
      {subs.length > 0 && (
        <ul className={styles.subs}>
          {subs.map((l) => (
            <li key={l.id} className={cx(!l.final && styles.partial)}>
              {l.text}
              {l.translation && l.translation !== l.text && (
                <>
                  <br />
                  <span>{l.translation}</span>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
