export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function req<T>(method: string, path: string, body?: unknown, token?: string): Promise<T> {
  const headers: Record<string, string> = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  let payload: BodyInit | undefined;
  if (body instanceof FormData) payload = body;
  else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  const r = await fetch(path, { method, headers, body: payload });
  if (r.status === 204) return undefined as T;
  const text = await r.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    /* not json */
  }
  if (!r.ok) throw new ApiError(r.status, data?.error ?? `Chyba ${r.status}`);
  return data as T;
}

export const api = {
  get: <T>(p: string, token?: string) => req<T>('GET', p, undefined, token),
  post: <T>(p: string, body?: unknown, token?: string) => req<T>('POST', p, body ?? {}, token),
  put: <T>(p: string, body?: unknown, token?: string) => req<T>('PUT', p, body ?? {}, token),
  patch: <T>(p: string, body?: unknown, token?: string) => req<T>('PATCH', p, body ?? {}, token),
  del: <T>(p: string, token?: string) => req<T>('DELETE', p, undefined, token),
  adminPost: async <T>(p: string, body: unknown, adminToken: string) => {
    const r = await fetch(p, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Admin-Token': adminToken },
      body: JSON.stringify(body),
    });
    const data = await r.json().catch(() => null);
    if (!r.ok) throw new ApiError(r.status, data?.error ?? `Chyba ${r.status}`);
    return data as T;
  },
  adminGet: async <T>(p: string, adminToken: string) => {
    const r = await fetch(p, { headers: { 'X-Admin-Token': adminToken } });
    const data = await r.json().catch(() => null);
    if (!r.ok) throw new ApiError(r.status, data?.error ?? `Chyba ${r.status}`);
    return data as T;
  },
};

export const store = {
  get(key: string): string | null {
    try {
      return localStorage.getItem(key);
    } catch {
      return null;
    }
  },
  set(key: string, v: string) {
    try {
      localStorage.setItem(key, v);
    } catch {
      /* private mode */
    }
  },
  del(key: string) {
    try {
      localStorage.removeItem(key);
    } catch {
      /* private mode */
    }
  },
};
