import { useState, type SubmitEvent } from 'react';
import styles from './Login.module.css';

type Props = { code: string; checking: boolean; err: string; onSubmit?: (token: string) => void };

/** Token form, or "připojuji…" while the token is checked or the first state arrives. */
export function Login({ code, checking, err, onSubmit }: Props) {
  const [input, setInput] = useState('');
  const submit = (e: SubmitEvent) => {
    e.preventDefault();
    onSubmit?.(input);
  };
  return (
    <main className={styles.login}>
      <h1>
        Cockpit <code>{code}</code>
        <span className={styles.badge}>React</span>
      </h1>
      {checking || !onSubmit ? (
        <p className="muted">připojuji…</p>
      ) : (
        <form onSubmit={submit}>
          <input
            placeholder="Cockpit token"
            type="password"
            value={input}
            onChange={(e) => {
              setInput(e.target.value);
            }}
          />
          <button className="primary">Přihlásit</button>
        </form>
      )}
      {err && <p className="err">{err}</p>}
    </main>
  );
}
