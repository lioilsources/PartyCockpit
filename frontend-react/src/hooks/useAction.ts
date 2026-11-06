import { useCallback, useState } from 'react';
import { errorMessage } from '../lib/format';

export type Act = (fn: () => Promise<unknown>) => Promise<void>;

/** One error line for the whole Cockpit: every action clears it first. */
export function useAction() {
  const [err, setErr] = useState('');
  const act = useCallback<Act>(async (fn) => {
    try {
      setErr('');
      await fn();
    } catch (e) {
      setErr(errorMessage(e));
    }
  }, []);
  return { err, setErr, act };
}
