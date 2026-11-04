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
  // a function is called before every (re)connect, e.g. for a fresh single-use ticket
  params: Record<string, string> | (() => Promise<Record<string, string>>) = {},
): Socket {
  let ws: WebSocket | null = null;
  let closed = false;
  let retry = 1000;
  let timer: ReturnType<typeof setTimeout>;

  const retryLater = () => {
    onStatus(false);
    if (closed) return;
    timer = setTimeout(open, retry);
    retry = Math.min(retry * 2, 15000);
  };
  const open = async () => {
    let extra: Record<string, string>;
    try {
      extra = typeof params === 'function' ? await params() : params;
    } catch {
      retryLater();
      return;
    }
    if (closed) return;
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const q = new URLSearchParams({ role, ...extra });
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
    ws.onclose = retryLater;
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
