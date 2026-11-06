import { useEffect, useState } from 'react';

/** The current time, refreshed every `ms`; keeps Date.now() out of render. */
export function useNow(ms: number) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => {
      setNow(Date.now());
    }, ms);
    return () => {
      clearInterval(t);
    };
  }, [ms]);
  return now;
}
