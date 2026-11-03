export type WsMsg = { type: string; data: any };
export type Role = 'stage' | 'cockpit' | 'guest' | 'streamer';

export type Socket = {
  /** Sends if connected; signalling is retried by its owner, so drops are fine. */
  send: (m: WsMsg) => boolean;
  close: () => void;
};

/** Reconnecting party socket. The server sends the full state on every connect. */
export function connect(
  code: string,
  role: Role,
  onMsg: (m: WsMsg) => void,
  onStatus: (online: boolean) => void,
  params: Record<string, string> = {},
): Socket {
  let ws: WebSocket | null = null;
  let closed = false;
  let retry = 1000;
  let timer: ReturnType<typeof setTimeout>;

  const open = () => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const q = new URLSearchParams({ role, ...params });
    ws = new WebSocket(`${proto}://${location.host}/ws/parties/${code}?${q}`);
    ws.onopen = () => {
      retry = 1000;
      onStatus(true);
    };
    ws.onmessage = (e) => {
      try {
        onMsg(JSON.parse(e.data));
      } catch {
        /* ignore */
      }
    };
    ws.onclose = () => {
      onStatus(false);
      if (closed) return;
      timer = setTimeout(open, retry);
      retry = Math.min(retry * 2, 15000);
    };
  };
  open();
  return {
    send(m) {
      if (ws?.readyState !== WebSocket.OPEN) return false;
      ws.send(JSON.stringify(m));
      return true;
    },
    close() {
      closed = true;
      clearTimeout(timer);
      ws?.close();
    },
  };
}
