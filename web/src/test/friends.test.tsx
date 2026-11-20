import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import Friends from '../screens/Friends'
import type { Friend, Session } from '../api'
import { bodyOf, mockFetch, type Reply } from './fetch'

const session: Session = {
  server: 'host',
  token: 'tok',
  username: 'alice',
  steamId: '1',
  personaName: 'Alice',
}

const aliceFriend: Friend = {
  steam_id: 42,
  persona_name: 'Bob',
  persona_state: 1,
  appid: 480,
  rich_presence: null,
  lobby_id: 7,
  connect_str: null,
}

type Responder = (url: string, init?: RequestInit) => Reply | undefined

function responder(overrides: Partial<{
  friends: Friend[]
  incoming: unknown[]
  outgoing: unknown[]
  blocked: unknown[]
  failFriends: boolean
}> = {}): Responder {
  return (url, init) => {
    const method = init?.method ?? 'GET'
    if (url.endsWith('/friends/requests') && method === 'GET') {
      return { json: { incoming: overrides.incoming ?? [], outgoing: overrides.outgoing ?? [] } }
    }
    if (url.endsWith('/friends/requests') && method === 'POST') {
      return { status: 201, json: { id: 9, status: 'pending', created_at: 1 } }
    }
    if (url.includes('/friends/requests/') && method === 'POST') return { json: { status: 'accepted' } }
    if (url.includes('/friends/requests/') && method === 'DELETE') return { status: 204 }
    if (url.endsWith('/friends/blocked')) {
      return { json: { blocked: overrides.blocked ?? [] } }
    }
    if (url.endsWith('/friends/42/block')) return { status: 204 }
    if (url.endsWith('/friends/42/invite')) return { status: 202 }
    if (url.endsWith('/friends/42') && method === 'DELETE') return { status: 204 }
    if (url.endsWith('/friends')) {
      if (overrides.failFriends) return { status: 500, text: 'boom' }
      return { json: { friends: overrides.friends ?? [] } }
    }
    return undefined
  }
}

function renderFriends(handler: Responder = responder()) {
  const fetchMock = mockFetch(handler)
  const onNotice = vi.fn()
  const onFriendsChange = vi.fn()
  render(
    <Friends
      session={session}
      onNotice={onNotice}
      onSelectFriend={vi.fn()}
      onJoinFriend={vi.fn()}
      onFriendsChange={onFriendsChange}
    />,
  )
  return { fetchMock, onNotice, onFriendsChange }
}

describe('friends screen', () => {
  it('shows an empty friends state and loads all three lists', async () => {
    renderFriends()
    expect(await screen.findByText('No friends yet. Open Requests to send one an invitation.')).toBeInTheDocument()
    const urls = screen.getByRole('tab', { name: 'Friends' })
    expect(urls).toHaveAttribute('aria-selected', 'true')
  })

  it('shows a friend with real presence and calls remove after confirmation', async () => {
    const { fetchMock } = renderFriends(responder({ friends: [aliceFriend] }))
    expect(await screen.findByRole('button', { name: 'Bob' })).toBeInTheDocument()
    expect(screen.getByText('App 480')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/Remove Bob from your friends/)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) =>
        String(url) === '/api/v1/friends/42' && init?.method === 'DELETE')).toBe(true)
    })
  })

  it('cancels a removal without calling the endpoint', async () => {
    const { fetchMock } = renderFriends(responder({ friends: [aliceFriend] }))
    await screen.findByRole('button', { name: 'Bob' })
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(fetchMock.mock.calls.some(([url, init]) =>
      String(url) === '/api/v1/friends/42' && init?.method === 'DELETE')).toBe(false)
  })

  it('escapes a confirmation dialog without removing', async () => {
    const { fetchMock } = renderFriends(responder({ friends: [aliceFriend] }))
    await screen.findByRole('button', { name: 'Bob' })
    fireEvent.click(screen.getByRole('button', { name: 'Block' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent(dialog, new Event('cancel', { bubbles: true }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(fetchMock.mock.calls.some(([url]) => String(url).endsWith('/friends/42/block'))).toBe(false)
  })

  it('sends a request by username from the Requests tab', async () => {
    const { fetchMock, onNotice } = renderFriends()
    fireEvent.click(screen.getByRole('tab', { name: 'Requests' }))
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'bob' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send request' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) =>
        String(url) === '/api/v1/friends/requests' && init?.method === 'POST')).toBe(true)
    })
    const call = fetchMock.mock.calls.find(([url, init]) =>
      String(url) === '/api/v1/friends/requests' && init?.method === 'POST')
    expect(bodyOf(call?.[1])).toEqual({ username: 'bob', message: '' })
    expect(onNotice).toHaveBeenCalledWith('Request sent to bob.')
  })

  it('accepts an incoming request and cancels an outgoing one', async () => {
    const { fetchMock } = renderFriends(responder({
      incoming: [{ id: 1, from_steam_id: 5, to_steam_id: 1, persona_name: 'Carol', message: 'hi', created_at: 1, status: 'pending' }],
      outgoing: [{ id: 2, from_steam_id: 1, to_steam_id: 6, persona_name: 'Dave', message: '', created_at: 1, status: 'pending' }],
    }))
    fireEvent.click(screen.getByRole('tab', { name: 'Requests' }))
    expect(await screen.findByText('Carol')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Accept' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url]) =>
        String(url) === '/api/v1/friends/requests/1/accept')).toBe(true)
    })
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) =>
        String(url) === '/api/v1/friends/requests/2' && init?.method === 'DELETE')).toBe(true)
    })
  })

  it('unblocks an account from the Blocked tab', async () => {
    const { fetchMock } = renderFriends(responder({
      blocked: [{ steam_id: 8, persona_name: 'Eve', created_at: 1 }],
    }))
    fireEvent.click(screen.getByRole('tab', { name: 'Blocked' }))
    expect(await screen.findByText('Eve')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Unblock' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) =>
        String(url) === '/api/v1/friends/8/block' && init?.method === 'DELETE')).toBe(true)
    })
  })

  it('shows a retryable error state when friends cannot load', async () => {
    renderFriends(responder({ failFriends: true }))
    expect(await screen.findByText('boom')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('moves between tabs with the arrow keys', async () => {
    renderFriends()
    const friendsTab = screen.getByRole('tab', { name: 'Friends' })
    await waitFor(() => expect(friendsTab).toHaveAttribute('tabindex', '0'))
    friendsTab.focus()
    fireEvent.keyDown(friendsTab, { key: 'ArrowRight' })
    expect(screen.getByRole('tab', { name: 'Requests' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Requests' })).toHaveFocus()
  })
})
