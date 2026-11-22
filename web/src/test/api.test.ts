import { describe, expect, it, vi } from 'vitest'
import {
  acceptFriendRequest,
  ApiError,
  blockFriend,
  createLobbyInvite,
  cancelFriendRequest,
  declineFriendRequest,
  getFriends,
  getLobbies,
  getStats,
  joinLobby,
  listBlocked,
  listFriendRequests,
  listSessions,
  login,
  logout,
  logoutAll,
  removeFriend,
  revokeSession,
  SESSION_EXPIRED_MESSAGE,
  searchUsers,
  sendFriendRequest,
  setUnauthorizedHandler,
  unblockFriend,
} from '../api'
import { bodyOf, headerOf, mockFetch } from './fetch'

const TOKEN = 'tok_live_only'
const INVITE = 'sapie://join/42?exp=1795199695&sig=abc'

describe('typed API client', () => {
  it('returns a login result with no password field', async () => {
    mockFetch(() => ({
      json: { session_token: TOKEN, steam_id: '76561197960265729', persona_name: 'Alice' },
    }))
    const result = await login('host', 'alice', 'pw')
    expect(result).toEqual({
      token: TOKEN,
      steamId: '76561197960265729',
      personaName: 'Alice',
    })
    expect(Object.keys(result)).not.toContain('password')
  })

  it('sends credentials only in the request body, never in the URL', async () => {
    const fetchMock = mockFetch(() => ({ json: { session_token: TOKEN } }))
    await login('host', 'alice', 'secret-value')
    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).toBe('/api/v1/login')
    expect(String(url)).not.toContain('secret-value')
    expect(bodyOf(init)).toEqual({ username: 'alice', password: 'secret-value' })
    expect(localStorage.length).toBe(0)
  })

  it('uses the right method and path for every social and session call', async () => {
    const fetchMock = mockFetch(() => ({ json: {} }))
    const calls: [Promise<unknown>, string, string][] = [
      [getFriends('h', TOKEN), 'GET', '/api/v1/friends'],
      [listFriendRequests('h', TOKEN), 'GET', '/api/v1/friends/requests'],
      [sendFriendRequest('h', TOKEN, 'bob', 'hi'), 'POST', '/api/v1/friends/requests'],
      [acceptFriendRequest('h', TOKEN, 7), 'POST', '/api/v1/friends/requests/7/accept'],
      [declineFriendRequest('h', TOKEN, 7), 'POST', '/api/v1/friends/requests/7/decline'],
      [cancelFriendRequest('h', TOKEN, 7), 'DELETE', '/api/v1/friends/requests/7'],
      [removeFriend('h', TOKEN, 42), 'DELETE', '/api/v1/friends/42'],
      [blockFriend('h', TOKEN, 42), 'POST', '/api/v1/friends/42/block'],
      [unblockFriend('h', TOKEN, 42), 'DELETE', '/api/v1/friends/42/block'],
      [listBlocked('h', TOKEN), 'GET', '/api/v1/friends/blocked'],
      [searchUsers('h', TOKEN, 'bo'), 'GET', '/api/v1/users/search?q=bo'],
      [logout('h', TOKEN), 'POST', '/api/v1/logout'],
      [logoutAll('h', TOKEN), 'POST', '/api/v1/logout-all'],
      [listSessions('h', TOKEN), 'GET', '/api/v1/sessions'],
      [revokeSession('h', TOKEN, 'abc'), 'DELETE', '/api/v1/sessions/abc'],
      [getStats('h', TOKEN, 42), 'GET', '/api/v1/users/42/stats'],
      [getLobbies('h', TOKEN), 'GET', '/api/v1/lobbies'],
      [joinLobby('h', TOKEN, 42), 'POST', '/api/v1/lobbies/42/join'],
      [joinLobby('h', TOKEN, 42, INVITE), 'POST', '/api/v1/lobbies/42/join'],
      [createLobbyInvite('h', TOKEN, 42), 'POST', '/api/v1/lobbies/42/invite'],
    ]
    await Promise.all(calls.map(([promise]) => promise))
    const seen = calls.map((_call, index) => {
      const [url, init] = fetchMock.mock.calls[index]
      return [init?.method ?? 'GET', String(url)] as const
    })
    expect(seen).toEqual(calls.map(([, method, path]) => [method, path]))
    expect(bodyOf(fetchMock.mock.calls[17][1])).toEqual({})
    expect(bodyOf(fetchMock.mock.calls[18][1])).toEqual({ invite: INVITE })
    expect(bodyOf(fetchMock.mock.calls[19][1])).toEqual({})
    expect(localStorage.length).toBe(0)
  })

  it('keeps the token out of URLs and in the authorization header only', async () => {
    const fetchMock = mockFetch(() => ({ json: { friends: [] } }))
    await getFriends('h', TOKEN)
    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).not.toContain(TOKEN)
    expect(headerOf(init, 'Authorization')).toBe(`Bearer ${TOKEN}`)
  })

  it('fires the unauthorized handler once for concurrent authenticated 401s', async () => {
    mockFetch(() => ({ status: 401, text: 'no' }))
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    const results = await Promise.allSettled([
      getFriends('h', TOKEN),
      listBlocked('h', TOKEN),
    ])
    for (const result of results) {
      expect(result.status).toBe('rejected')
      if (result.status === 'rejected') {
        expect(result.reason).toBeInstanceOf(ApiError)
        expect((result.reason as ApiError).message).toBe(SESSION_EXPIRED_MESSAGE)
      }
    }
    expect(handler).toHaveBeenCalledTimes(1)
    setUnauthorizedHandler(null)
  })

  it('does not treat a bad login as session expiry', async () => {
    mockFetch(() => ({ status: 401, text: 'unauthorized' }))
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    await expect(login('h', 'alice', 'wrong')).rejects.toBeInstanceOf(ApiError)
    expect(handler).not.toHaveBeenCalled()
    setUnauthorizedHandler(null)
  })
})
