import { describe, expect, it } from 'vitest';
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
});
