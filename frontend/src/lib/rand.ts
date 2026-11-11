// Seeded randomness (D23): a scene with params.seed plays the same way every
// time (the Studio's saved show); without a seed it is Math.random as before.

/** mulberry32: small, fast, good enough for shuffles and jitter. */
export function mulberry32(seed: number): () => number {
  let a = seed | 0;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** A generator for the scene's seed (plus a salt per use), or Math.random. */
export function rng(seed: unknown, salt = 0): () => number {
  if (seed === undefined || seed === null || seed === '') return Math.random;
  const n = Number(seed);
  return Number.isFinite(n) ? mulberry32(n * 7919 + salt) : Math.random;
}

/** Seconds since the scene started on the server (clamped at 0). */
export const elapsed = (startedAt: number) => Math.max(0, (Date.now() - (startedAt || Date.now())) / 1000);
