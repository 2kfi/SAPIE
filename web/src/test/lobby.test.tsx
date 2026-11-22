import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LobbyPanel from '../components/LobbyPanel';
import type { Session } from '../api';
import { mockFetch } from './fetch';

const session: Session = {
  server: 'http://localhost:8080',
  username: 'player',
  token: 'token',
  steamId: '76561198000000000',
  personaName: 'player',
};

describe('LobbyPanel', () => {
  it('shows empty and error states', async () => {
    const user = userEvent.setup();
    let fail = false;
    mockFetch((url) => {
      if (url.endsWith('/lobbies') && !fail) return { json: { lobbies: [] } };
      return { status: 500, text: 'lobby service down' };
    });
    render(<LobbyPanel session={session} onNotice={() => {}} />);
    expect(await screen.findByText('No lobbies are open yet. Create one above.')).toBeInTheDocument();
    fail = true;
    await user.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('lobby service down');
  });

  it('creates a lobby and joins one from the list', async () => {
    const user = userEvent.setup();
    let created = false;
    mockFetch((url, init) => {
      if (url.endsWith('/lobbies') && (!init?.method || init.method === 'GET')) {
        return {
          json: {
            lobbies: [
              ...(created ? [{
                lobby_id: 'lobby-1',
                name: 'Created room',
                owner: 'player',
                appid: 123,
                type: 0,
                members: [],
                member_limit: 4,
                joinable: true,
              }] : []),
              {
                lobby_id: 'lobby-2',
                name: 'Open room',
                owner: 'other',
                appid: 123,
                type: 0,
                members: ['other'],
                member_limit: 6,
                joinable: true,
              },
            ],
          },
        };
      }
      if (url.endsWith('/lobbies') && init?.method === 'POST') {
        created = true;
        return { json: { lobby_id: 'lobby-1' } };
      }
      if (url.endsWith('/lobbies/lobby-2/join')) return { json: {} };
      return { status: 404, text: 'not found' };
    });
    const notices: string[] = [];
    render(<LobbyPanel session={session} onNotice={(message) => notices.push(message)} />);
    await screen.findByText('Lobby lobby-2');
    await user.type(screen.getByLabelText('App ID'), '123');
    await user.type(screen.getByLabelText('Lobby type'), '0');
    await user.type(screen.getByLabelText('Member limit'), '4');
    await user.click(screen.getByRole('button', { name: 'Create lobby' }));
    expect(await screen.findByText('Lobby lobby-1')).toBeInTheDocument();
    const joinButtons = screen.getAllByRole('button', { name: 'Join' });
    await user.click(joinButtons[1]);
    await waitFor(() => expect(notices).toContain('Joined lobby lobby-2.'));
    expect(notices).toContain('Lobby created.');
  });

  it('copies an invite link for a lobby the user is in', async () => {
    const user = userEvent.setup();
    const invite = 'sapie://join/42?exp=1795199695&sig=abc';
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    mockFetch((url, init) => {
      if (url.endsWith('/lobbies') && (!init?.method || init.method === 'GET')) {
        return {
          json: {
            lobbies: [
              {
                lobby_id: 42,
                owner: session.steamId,
                appid: 123,
                type: 0,
                members: [session.steamId],
                member_limit: 4,
                joinable: true,
              },
              {
                lobby_id: 43,
                owner: 'other',
                appid: 123,
                type: 0,
                members: ['other'],
                member_limit: 4,
                joinable: true,
              },
            ],
          },
        };
      }
      if (url.endsWith('/lobbies/42/invite')) {
        return { json: { url: invite, expires_at: 1795199695 } };
      }
      return { status: 404, text: 'not found' };
    });
    render(<LobbyPanel session={session} onNotice={() => {}} />);
    await screen.findByText('Lobby 42');
    const copyButtons = screen.getAllByRole('button', { name: 'Copy invite link' });
    expect(copyButtons).toHaveLength(1);
    await user.click(copyButtons[0]);
    expect(writeText).toHaveBeenCalledWith(invite);
    expect(await screen.findByRole('status')).toHaveTextContent('Invite link copied to clipboard.');
  });

  it('shows the link when the clipboard is unavailable', async () => {
    const user = userEvent.setup();
    const invite = 'sapie://join/44?exp=1795199695&sig=def';
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    mockFetch((url, init) => {
      if (url.endsWith('/lobbies') && (!init?.method || init.method === 'GET')) {
        return {
          json: {
            lobbies: [{
              lobby_id: 44,
              owner: session.steamId,
              appid: 123,
              type: 0,
              members: [session.steamId],
              member_limit: 4,
              joinable: true,
            }],
          },
        };
      }
      if (url.endsWith('/lobbies/44/invite')) {
        return { json: { url: invite, expires_at: 1795199695 } };
      }
      return { status: 404, text: 'not found' };
    });
    render(<LobbyPanel session={session} onNotice={() => {}} />);
    await user.click(await screen.findByRole('button', { name: 'Copy invite link' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Clipboard access failed. Select the link below to copy it.');
    expect(screen.getByText(invite)).toBeInTheDocument();
  });
});
