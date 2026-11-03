export type WsMsg = { type: string; data: any };

/** Reconnecting party socket. The server sends the full state on every connect. */
export function connect(
  code: string,
  role: 'stage' | 'cockpit' | 'guest',
  onMsg: (m: WsMsg) => void,
  onStatus: (online: boolean) => void,
  token?: string,
): () => void {
  let ws: WebSocket | null = null;
  let closed = false;
  let retry = 1000;
  let timer: ReturnType<typeof setTimeout>;

  const open = () => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const q = new URLSearchParams({ role });
    if (token) q.set('token', token);
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
  return () => {
    closed = true;
    clearTimeout(timer);
    ws?.close();
  };
}
