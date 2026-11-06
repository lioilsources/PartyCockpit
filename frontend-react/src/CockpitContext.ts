import { createContext, use } from 'react';
import type { CockpitState, Party } from '@pc/lib/types';
import type { WsMsg } from '@pc/lib/ws';
import type { CockpitApi } from './api/cockpitApi';
import type { Act } from './hooks/useAction';
import type { ViewerStore } from './hooks/useViewer';
import type { CockpitModel } from './state/reducer';

export type CockpitCtx = {
  code: string;
  api: CockpitApi;
  /** present: the Cockpit renders only after the first `party` message */
  party: Party;
  st: CockpitState | null;
  model: CockpitModel;
  online: boolean;
  act: Act;
  setErr: (e: string) => void;
  /** a rotated token reconnects the socket */
  replaceToken: (t: string) => void;
  viewer: ViewerStore;
  send: (m: WsMsg) => boolean;
};

export const CockpitContext = createContext<CockpitCtx | null>(null);

export function useCockpit(): CockpitCtx {
  const c = use(CockpitContext);
  if (!c) throw new Error('useCockpit outside <CockpitContext>');
  return c;
}
