import { vi } from 'vitest';

/** A WebSocket the test drives: `FakeSocket.last.receive(msg)` delivers a server message. */
export class FakeSocket {
  static OPEN = 1;
  static all: FakeSocket[] = [];
  static get last(): FakeSocket {
    const s = FakeSocket.all.at(-1);
    if (!s) throw new Error('no socket opened');
    return s;
  }
  readyState = 0;
  sent: unknown[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(readonly url: string) {
    FakeSocket.all.push(this);
    queueMicrotask(() => {
      this.readyState = FakeSocket.OPEN;
      this.onopen?.();
    });
  }
  send(data: string) {
    this.sent.push(JSON.parse(data));
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
  receive(type: string, data: unknown) {
    this.onmessage?.({ data: JSON.stringify({ type, data }) });
  }
}

type Route = (init: RequestInit | undefined) => unknown;
const href = (input: string | URL | Request) => (typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);

/** fetch answering JSON per "METHOD /path"; anything else is a 404. Calls are recorded. */
export function fakeFetch(routes: Partial<Record<string, Route>>) {
  const fn = vi.fn((input: string | URL | Request, init?: RequestInit) => {
    const url = new URL(href(input), 'http://party.test');
    const key = `${init?.method ?? 'GET'} ${url.pathname}`;
    const route = routes[key];
    if (!route) return Promise.resolve(new Response(JSON.stringify({ error: 'nenalezeno' }), { status: 404 }));
    const body = route(init);
    return Promise.resolve(body instanceof Response ? body : body === undefined ? new Response(null, { status: 204 }) : new Response(JSON.stringify(body)));
  });
  vi.stubGlobal('fetch', fn);
  return fn;
}

export const authHeader = (init: RequestInit | undefined) => (init?.headers as Record<string, string> | undefined)?.Authorization;
