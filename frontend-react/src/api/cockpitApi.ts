import { api as http } from '@pc/lib/api';
import type { Check, CockpitState, Episode, Party, SceneKind } from '@pc/lib/types';

export type PartyPatch = Partial<Pick<Party, 'mode' | 'loopMode' | 'djfyCode' | 'endsAt' | 'autoLive'>>;
export type SceneParams = {
  beats: number;
  style?: string;
  episodes?: string[];
  lang?: string;
  speed?: number;
  pack?: string;
  transition?: string;
  shader?: string;
};
export type TempoBody = { bpm: number; anchor?: number };

/**
 * Every Cockpit endpoint (backend/internal/api/server.go), bound to one party
 * and its token. Methods are arrow properties, so they can be passed around
 * (`onClick={api.panic}`) without losing `this`.
 */
export function cockpitApi(code: string, token: string) {
  const p = (path = '') => `/api/parties/${code}${path}`;
  const post = <T = undefined>(path: string, body: unknown = {}) => http.post<T>(p(path), body, token);
  const del = (path: string) => http.del<undefined>(p(path), token);

  return {
    verify: () => http.get<CockpitState>(p('/cockpit'), token),
    ticket: async () => (await post<{ ticket: string }>('/cockpit/ticket')).ticket,
    rotate: async () => (await post<{ cockpitToken: string }>('/cockpit/rotate')).cockpitToken,
    patch: (body: PartyPatch) => http.patch<Party>(p(), body, token),
    scene: (kind: SceneKind, params: SceneParams) => post('/scene', { kind, params }),
    panic: () => post('/panic'),
    approve: (id: string) => post(`/assets/${id}/approve`),
    reject: (id: string) => post(`/assets/${id}/reject`),
    reorder: (ids: string[]) => http.put<undefined>(p('/queue'), { ids }, token),
    unqueue: (id: string) => del(`/queue/${id}`),
    animate: () => post<{ queued: number }>('/animate'),
    tempo: (body: TempoBody) => post('/tempo', body),
    lottery: (count: number, game: string) => post('/lottery', { count, game }),
    startGame: () => post('/game'),
    stopGame: () => del('/game'),
    preflight: async () => (await http.get<{ checks: Check[] }>(p('/preflight'), token)).checks,
    subtitle: (text: string) => post('/subtitle', { text }),
    clearSubtitles: () => del('/subtitle'),
    streamLive: (id: string) => post(`/streams/${id}/approve`),
    streamHold: (id: string) => post(`/streams/${id}/hold`),
    streamKick: (id: string) => del(`/streams/${id}`),
    pair: () => post('/pair'),
    /** Webtoon ideas as the JSON file the operator downloads. */
    ideasJson: async () => {
      const r = await fetch(p('/ideas'), { headers: { Authorization: `Bearer ${token}` } });
      if (!r.ok) throw new Error(`Chyba ${r.status}`);
      return r.text();
    },
    episodes: () => http.get<Episode[]>('/api/webtoon'),
  };
}

export type CockpitApi = ReturnType<typeof cockpitApi>;
