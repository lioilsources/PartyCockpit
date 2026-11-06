import { useCallback, useEffect, useState } from 'react';
import { ApiError, store } from '@pc/lib/api';
import { cockpitApi } from '../api/cockpitApi';
import { errorMessage } from '../lib/format';

export const tokenKey = (code: string) => `pc.cockpit.${code}`;

/** `#token=` from a pairing QR or Home; `?token=` from older links (D19). */
export function readUrlToken(loc: Pick<Location, 'hash' | 'search'>): string | null {
  return new URLSearchParams(loc.hash.slice(1)).get('token') ?? new URLSearchParams(loc.search).get('token');
}

export type AuthStatus = 'none' | 'checking' | 'ok';

/**
 * The Cockpit token: read once from the URL (then dropped from it) or from
 * localStorage, checked against the server before the socket opens.
 */
export function useCockpitAuth(code: string) {
  // computed once and without side effects: StrictMode may call it twice
  const [initial] = useState(() => {
    const fromUrl = readUrlToken(location);
    return { fromUrl, token: fromUrl ?? store.get(tokenKey(code)) ?? '' };
  });
  const [token, setToken] = useState(initial.token);
  const [status, setStatus] = useState<AuthStatus>(initial.token ? 'checking' : 'none');
  const [err, setErr] = useState('');

  // the token must not stay in the address bar, history or a screenshot
  useEffect(() => {
    if (!initial.fromUrl) return;
    store.set(tokenKey(code), initial.fromUrl);
    history.replaceState(null, '', location.pathname);
  }, [initial, code]);

  useEffect(() => {
    if (status !== 'checking') return;
    let live = true;
    cockpitApi(code, token)
      .verify()
      .then(
        () => {
          if (!live) return;
          store.set(tokenKey(code), token);
          setErr('');
          setStatus('ok');
        },
        (e: unknown) => {
          if (!live) return;
          setErr(e instanceof ApiError && e.status === 401 ? 'Neplatný cockpit token' : errorMessage(e));
          setToken('');
          setStatus('none');
        },
      );
    return () => {
      live = false;
    };
  }, [status, token, code]);

  const submit = useCallback((t: string) => {
    setToken(t.trim());
    setStatus('checking');
  }, []);
  /** After a rotation: the new token is already valid, no second check. */
  const replace = useCallback(
    (t: string) => {
      store.set(tokenKey(code), t);
      setToken(t);
    },
    [code],
  );
  return { token, status, err, submit, replace };
}
