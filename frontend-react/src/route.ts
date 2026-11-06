/** `/react/cockpit/<CODE>` → the party code; the base path comes from Vite. */
export function parseRoute(pathname: string, base: string = import.meta.env.BASE_URL): { code: string } | null {
  if (!pathname.startsWith(base)) return null;
  const [section, raw = ''] = pathname.slice(base.length).split('/');
  return section === 'cockpit' && raw ? { code: raw.toUpperCase() } : null;
}
