import { describe, expect, it, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from '../App';
import { mockFetch } from './fetch';

const INVITE = 'sapie://join/42?exp=1795199695&sig=abc';

function sessionReplies() {
  return {
    friends: { friends: [] },
    requests: { incoming: [], outgoing: [] },
    blocked: { blocked: [] },
    lobbies: { lobbies: [] },
  };
}

function signIn() {
  fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } });
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'pw' } });
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));
}

function baseHandler(unauthorized = false) {
  const replies = sessionReplies();
  return (url: string) => {
    if (url.endsWith('/config')) return { json: { allow_register: false } };
    if (url.endsWith('/login')) {
      return { json: { session_token: 'session-token-123', steam_id: 99, persona_name: 'Alice' } };
    }
    if (unauthorized && url.includes('/friends')) return { status: 401, text: 'unauthorized' };
    if (url.endsWith('/friends')) return { json: replies.friends };
    if (url.endsWith('/friends/requests')) return { json: replies.requests };
    if (url.endsWith('/friends/blocked')) return { json: replies.blocked };
    if (url.endsWith('/lobbies')) return { json: replies.lobbies };
    if (url.endsWith('/lobbies/42/join')) return { json: { lobby_id: 42 } };
    if (url.endsWith('/logout')) return { json: {} };
    return undefined;
  };
}

describe('App session flow', () => {
  beforeEach(() => {
    window.location.hash = '#/login';
    localStorage.clear();
    sessionStorage.clear();
  });

  it('returns to sign in on an authenticated 401 without storing the token', async () => {
    mockFetch(baseHandler(true));
    render(<App />);
    signIn();
    expect(await screen.findByText('Your session has expired. Sign in again.')).toBeInTheDocument();
    await waitFor(() => expect(window.location.hash).toBe('#/login'));
    expect(localStorage.length).toBe(0);
    expect(document.body.textContent).not.toContain('session-token-123');
  });

  it('keeps a login 401 as a credential error and does not show expiry', async () => {
    mockFetch((url) => {
      if (url.endsWith('/config')) return { json: { allow_register: false } };
      return { status: 401, text: 'unauthorized' };
    });
    render(<App />);
    signIn();
    expect(await screen.findByText('Username or password is incorrect.')).toBeInTheDocument();
    expect(screen.queryByText('Your session has expired. Sign in again.')).not.toBeInTheDocument();
  });

  it('signs out through the server and discards the local session', async () => {
    const fetchMock = mockFetch(baseHandler());
    render(<App />);
    signIn();
    await screen.findByRole('button', { name: 'Sign out' });
    expect(await screen.findByRole('heading', { name: 'Friends' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    const logoutCall = fetchMock.mock.calls.find(([url]) => String(url).endsWith('/logout'));
    expect(logoutCall).toBeDefined();
    const headers = logoutCall?.[1]?.headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer session-token-123');
    expect(await screen.findByText('You have signed out.')).toBeInTheDocument();
    expect(localStorage.length).toBe(0);
    expect(document.body.textContent).not.toContain('session-token-123');
  });

  it('opens the invite panel from a web fallback link and joins that lobby', async () => {
    window.history.replaceState(null, '', `/?invite=${encodeURIComponent(INVITE)}#/login`);
    const fetchMock = mockFetch(baseHandler());
    render(<App />);
    expect(await screen.findByText('Sign in to review your lobby invite.')).toBeInTheDocument();
    signIn();
    expect(await screen.findByRole('dialog')).toHaveTextContent('Join lobby 42');
    fireEvent.click(screen.getByRole('button', { name: 'Join lobby 42' }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) =>
      String(url).endsWith('/lobbies/42/join'))).toBe(true));
    expect(await screen.findByText('Joined lobby 42.')).toBeInTheDocument();
    expect(window.location.search).toBe('');
  });
});
