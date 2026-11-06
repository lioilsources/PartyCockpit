/** `cx('a', on && 'on')`: class names without the falsy ones. */
export const cx = (...names: (string | false | null | undefined)[]) => names.filter(Boolean).join(' ');

export const errorMessage = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** "12 s" / "3 min" since t. */
export function ago(t: number, now: number) {
  const s = Math.round((now - t) / 1000);
  return s < 60 ? `${s} s` : `${Math.round(s / 60)} min`;
}

const HOUR = 3600e3;

/** +2 h from the later of now and the current end, capped at 24 h from now (server limit). */
export const extendedEnd = (endsAt: number, now: number) => Math.min(now + 24 * HOUR - 60e3, Math.max(endsAt, now) + 2 * HOUR);

export const clockTime = (t: number) => new Date(t).toLocaleTimeString('cs-CZ', { hour: '2-digit', minute: '2-digit' });
