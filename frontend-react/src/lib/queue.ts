/** The queue with item i moved by d places, or null at either end. */
export function moved(ids: string[], i: number, d: number): string[] | null {
  const j = i + d;
  if (j < 0 || j >= ids.length) return null;
  const out = [...ids];
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}
