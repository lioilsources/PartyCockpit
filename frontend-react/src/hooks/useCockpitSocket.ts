import { useCallback, useEffect, useEffectEvent, useReducer, useRef, useState } from 'react';
import { connect, type Socket, type WsMsg } from '@pc/lib/ws';
import type { RtcMsg } from '@pc/lib/rtc';
import { asCockpitMsg } from '../state/messages';
import { cockpitReducer, initialModel } from '../state/reducer';

type Options = {
  code: string;
  token: string;
  /** a fresh single-use ticket before every (re)connect (D19) */
  ticket: () => Promise<string>;
  onRtc: (m: RtcMsg) => void;
  /** the socket (re)connected: WebRTC needs fresh offers */
  onHello: () => void;
};

/**
 * The Cockpit's party socket: state through the reducer, WebRTC signalling out
 * to the callbacks. One socket per code and token; a rotated token reconnects.
 */
export function useCockpitSocket({ code, token, ticket, onRtc, onHello }: Options) {
  const [model, dispatch] = useReducer(cockpitReducer, initialModel);
  const [online, setOnline] = useState(false);
  const sock = useRef<Socket | null>(null);

  // effect events read the latest callbacks without reopening the socket
  const onMessage = useEffectEvent((raw: WsMsg) => {
    const m = asCockpitMsg(raw);
    if (!m) return;
    if (m.type === 'rtc') onRtc(m.data);
    else if (m.type === 'hello') onHello();
    else dispatch(m);
  });
  const nextTicket = useEffectEvent(() => ticket());

  useEffect(() => {
    if (!token) return;
    const s = connect(
      code,
      'cockpit',
      (m) => {
        onMessage(m);
      },
      setOnline,
      async () => ({ ticket: await nextTicket() }),
    );
    sock.current = s;
    return () => {
      s.close();
      if (sock.current === s) sock.current = null;
    };
  }, [code, token]);

  const send = useCallback((m: WsMsg) => sock.current?.send(m) ?? false, []);
  return { model, online, send };
}
