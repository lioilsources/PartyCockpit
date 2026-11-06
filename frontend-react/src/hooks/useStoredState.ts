import { useEffect, useState, type Dispatch, type SetStateAction } from 'react';
import { store } from '@pc/lib/api';

type Codec<T> = { parse: (raw: string) => T; serialize: (v: T) => string };

export const asString: Codec<string> = { parse: (raw) => raw, serialize: (v) => v };
export const asFlag: Codec<boolean> = { parse: (raw) => raw === '1', serialize: (v) => (v ? '1' : '0') };
export function asJson<T>(): Codec<T> {
  return { parse: (raw) => JSON.parse(raw) as T, serialize: (v) => JSON.stringify(v) };
}

/**
 * useState mirrored to localStorage under the same keys as the Svelte Cockpit,
 * so switching UIs keeps the operator's choices. Storage can be missing
 * (private mode) or hold garbage; both fall back to `initial`.
 */
export function useStoredState<T>(key: string, initial: T, codec: Codec<T>): [T, Dispatch<SetStateAction<T>>] {
  const [value, setValue] = useState<T>(() => {
    const raw = store.get(key);
    if (raw === null) return initial;
    try {
      return codec.parse(raw);
    } catch {
      return initial;
    }
  });
  useEffect(() => {
    store.set(key, codec.serialize(value));
  }, [key, value, codec]);
  return [value, setValue];
}
