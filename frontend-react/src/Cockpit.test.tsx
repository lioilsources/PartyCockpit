import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Cockpit } from './Cockpit';
import { authHeader, fakeFetch, FakeSocket } from './test/fakes';
import { cockpitState, party, sticker } from './test/fixtures';

beforeEach(() => {
  FakeSocket.all = [];
  vi.stubGlobal('WebSocket', FakeSocket);
});
afterEach(() => {
  vi.unstubAllGlobals();
  history.replaceState(null, '', '/');
});

const ok = () => cockpitState();

describe('Cockpit', () => {
  it('takes the token from the fragment, drops it from the URL and never puts it in the socket URL', async () => {
    history.replaceState(null, '', '/react/cockpit/ABCD#token=tok-1');
    const fetch = fakeFetch({ 'GET /api/parties/ABCD/cockpit': ok, 'POST /api/parties/ABCD/cockpit/ticket': () => ({ ticket: 'tk-1' }) });
    render(<Cockpit code="ABCD" />);

    await waitFor(() => {
      expect(FakeSocket.all).toHaveLength(1);
    });
    expect(localStorage.getItem('pc.cockpit.ABCD')).toBe('tok-1');
    expect(location.hash).toBe('');
    expect(FakeSocket.last.url).toContain('ticket=tk-1');
    expect(FakeSocket.last.url).not.toContain('tok-1');
    expect(authHeader(fetch.mock.calls[0]?.[1])).toBe('Bearer tok-1');
    expect(screen.getByText('připojuji…')).toBeInTheDocument();
  });

  it('rejects a bad stored token and shows the form', async () => {
    localStorage.setItem('pc.cockpit.ABCD', 'old');
    fakeFetch({ 'GET /api/parties/ABCD/cockpit': () => new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 }) });
    render(<Cockpit code="ABCD" />);
    expect(await screen.findByText('Neplatný cockpit token')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Cockpit token')).toBeInTheDocument();
    expect(FakeSocket.all).toHaveLength(0);
  });

  it('renders the state from the socket and approves through the API', async () => {
    localStorage.setItem('pc.cockpit.ABCD', 'tok-2');
    const approved: string[] = [];
    fakeFetch({
      'GET /api/parties/ABCD/cockpit': ok,
      'POST /api/parties/ABCD/cockpit/ticket': () => ({ ticket: 'tk-2' }),
      'POST /api/parties/ABCD/assets/a1/approve': (init) => {
        approved.push(authHeader(init) ?? '');
        return undefined;
      },
    });
    render(<Cockpit code="ABCD" />);
    await waitFor(() => {
      expect(FakeSocket.all).toHaveLength(1);
    });

    act(() => {
      FakeSocket.last.receive('party', party());
      FakeSocket.last.receive('cockpit', cockpitState([sticker('a1', 'Eva'), sticker('a2', 'Petr', { status: 'approved' })]));
      FakeSocket.last.receive('pairing', { url: 'ignored' });
    });
    expect(screen.getByRole('heading', { name: /Silvestr/ })).toBeInTheDocument();
    expect(screen.getByText('Ke schválení (1)')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'schválit Eva' }));
    expect(approved).toEqual(['Bearer tok-2']);
  });

  it('asks twice before rotating the token, then reconnects with the new one', async () => {
    localStorage.setItem('pc.cockpit.ABCD', 'tok-3');
    fakeFetch({
      'GET /api/parties/ABCD/cockpit': ok,
      'POST /api/parties/ABCD/cockpit/ticket': (init) => ({ ticket: authHeader(init) === 'Bearer tok-4' ? 'tk-new' : 'tk-old' }),
      'POST /api/parties/ABCD/cockpit/rotate': () => ({ cockpitToken: 'tok-4' }),
    });
    render(<Cockpit code="ABCD" />);
    await waitFor(() => {
      expect(FakeSocket.all).toHaveLength(1);
    });
    act(() => {
      FakeSocket.last.receive('party', party());
    });

    const rotate = screen.getByRole('button', { name: 'Vyměnit cockpit token' });
    await userEvent.click(rotate);
    expect(FakeSocket.all).toHaveLength(1);
    await userEvent.click(screen.getByRole('button', { name: /Opravdu vyměnit/ }));

    await waitFor(() => {
      expect(FakeSocket.last.url).toContain('ticket=tk-new');
    });
    expect(localStorage.getItem('pc.cockpit.ABCD')).toBe('tok-4');
  });
});
