export type Session = {
  server: string
  username: string
  token: string
  steamId: string
  personaName: string
}

export type PresenceValues = Record<string, string | null>

export type Friend = {
  steam_id: string | number
  persona_name: string
  persona_state: string | number
  appid: string | number | null
  rich_presence: PresenceValues | string | null
  lobby_id: string | number | null
  connect_str: string | null
}

export type FriendRequest = {
  id: number
  from_steam_id: string | number
  to_steam_id: string | number
  persona_name: string
  message: string
  created_at: number
  status: 'pending' | 'accepted' | 'declined' | string
}

export type BlockedUser = {
  steam_id: string | number
  persona_name: string
  created_at: number
}

export type SearchUser = {
  steam_id: string | number
  persona_name: string
}

export type SessionInfo = {
  id: string
  created_at: number
  expires_at: number
  current: boolean
}

export type Lobby = {
  lobby_id: string | number
  owner: string | number
  appid: string | number
  member_limit: number
  type: string | number
  joinable: boolean
  members: (string | number)[]
}

export type LobbyInvite = {
  url: string
  expires_at: number
}

export type Achievement = {
  steam_id: string | number
  appid: string | number
  id: string
  display_name: string
  description: string
  hidden: boolean
  icon: string
  achieved: boolean
  unlock_time_unix?: number
  unlock_time?: number
}

export type UserStats = {
  steam_id: string | number
  stats: { name: string; value: string }[]
  achievements: Achievement[]
}

export const SESSION_EXPIRED_MESSAGE = 'Your session has expired. Sign in again.'

export class ApiError extends Error {
  constructor(message: string, readonly status?: number) {
    super(message)
    this.name = 'ApiError'
  }
}

export function isUnauthorized(reason: unknown): boolean {
  return reason instanceof ApiError && reason.status === 401
}

export function messageOf(reason: unknown): string {
  return reason instanceof Error ? reason.message : String(reason)
}

let unauthorizedHandler: (() => void) | null = null
let unauthorizedHandled = false

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
  unauthorizedHandled = false
}

function baseUrl(_server: string): string {
  return '/api/v1'
}

let idemCounter = 0
function idempotencyKey(): string {
  const random = globalThis.crypto?.randomUUID?.()
  return random ?? `web-${Date.now()}-${idemCounter++}`
}

async function errorDetail(response: Response): Promise<string> {
  try {
    const text = await response.text()
    if (!text) return ''
    try {
      const parsed = JSON.parse(text) as { error?: unknown; message?: unknown }
      if (typeof parsed.error === 'string') return parsed.error
      if (typeof parsed.message === 'string') return parsed.message
    } catch {
      // Body is not JSON, use the raw text.
    }
    return text.length > 200 ? `${text.slice(0, 200)}…` : text
  } catch {
    return ''
  }
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'DELETE'
  body?: unknown
  token?: string
  idempotent?: boolean
  timeoutMs?: number
}

async function request<T>(
  server: string,
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = 'GET', body, token, idempotent = false, timeoutMs = 8000 } = options
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), timeoutMs)
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`
  if (idempotent) headers['Idempotency-Key'] = idempotencyKey()

  try {
    const response = await fetch(`${baseUrl(server)}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    })
    if (!response.ok) {
      const detail = await errorDetail(response)
      if (response.status === 401 && token) {
        if (!unauthorizedHandled) {
          unauthorizedHandled = true
          unauthorizedHandler?.()
        }
        throw new ApiError(SESSION_EXPIRED_MESSAGE, 401)
      }
      throw new ApiError(detail || `The server returned ${response.status}.`, response.status)
    }
    if (response.status === 202 || response.status === 204) return undefined as T
    const bodyText = await response.text()
    if (!bodyText.trim()) return undefined as T
    try {
      return JSON.parse(bodyText) as T
    } catch {
      throw new ApiError('The server returned an unexpected response.')
    }
  } catch (reason) {
    if (reason instanceof ApiError) throw reason
    if (controller.signal.aborted) throw new ApiError('The server did not respond in time.')
    throw new ApiError('Could not reach the server. Check your connection and try again.')
  } finally {
    window.clearTimeout(timer)
  }
}

export type LoginResult = {
  token: string
  steamId: string
  personaName: string
}

export function getConfig(server: string): Promise<{ allow_register?: unknown }> {
  return request(server, '/config')
}

export function register(
  server: string,
  username: string,
  password: string,
  personaName?: string,
): Promise<unknown> {
  return request(server, '/register', {
    method: 'POST',
    body: {
      username,
      password,
      ...(personaName ? { persona_name: personaName } : {}),
    },
  })
}

export async function login(
  server: string,
  username: string,
  password: string,
): Promise<LoginResult> {
  const data = await request<{
    session_token?: unknown
    steam_id?: unknown
    persona_name?: unknown
  }>(server, '/login', { method: 'POST', body: { username, password } })
  if (typeof data?.session_token !== 'string' || !data.session_token) {
    throw new ApiError('The server did not return a session token.')
  }
  return {
    token: data.session_token,
    steamId: data.steam_id == null ? '' : String(data.steam_id),
    personaName: typeof data.persona_name === 'string' ? data.persona_name : username,
  }
}

export function getFriends(server: string, token: string): Promise<{ friends: Friend[] }> {
  return request(server, '/friends', { token })
}

export function inviteFriend(
  server: string,
  token: string,
  steamId: string | number,
  connectStr: string,
): Promise<void> {
  return request(server, `/friends/${encodeURIComponent(String(steamId))}/invite`, {
    method: 'POST',
    token,
    body: { connect_str: connectStr },
  })
}

export function listFriendRequests(
  server: string,
  token: string,
): Promise<{ incoming: FriendRequest[]; outgoing: FriendRequest[] }> {
  return request(server, '/friends/requests', { token })
}

export function sendFriendRequest(
  server: string,
  token: string,
  username: string,
  message = '',
): Promise<{ id: number; status: string; created_at: number }> {
  return request(server, '/friends/requests', {
    method: 'POST',
    token,
    idempotent: true,
    body: { username, message },
  })
}

export function acceptFriendRequest(
  server: string,
  token: string,
  id: number,
): Promise<{ status: string }> {
  return request(server, `/friends/requests/${encodeURIComponent(String(id))}/accept`, {
    method: 'POST',
    token,
    idempotent: true,
  })
}

export function declineFriendRequest(
  server: string,
  token: string,
  id: number,
): Promise<{ status: string }> {
  return request(server, `/friends/requests/${encodeURIComponent(String(id))}/decline`, {
    method: 'POST',
    token,
    idempotent: true,
  })
}

export function cancelFriendRequest(
  server: string,
  token: string,
  id: number,
): Promise<void> {
  return request(server, `/friends/requests/${encodeURIComponent(String(id))}`, {
    method: 'DELETE',
    token,
  })
}

export function removeFriend(
  server: string,
  token: string,
  steamId: string | number,
): Promise<void> {
  return request(server, `/friends/${encodeURIComponent(String(steamId))}`, {
    method: 'DELETE',
    token,
  })
}

export function blockFriend(
  server: string,
  token: string,
  steamId: string | number,
): Promise<void> {
  return request(server, `/friends/${encodeURIComponent(String(steamId))}/block`, {
    method: 'POST',
    token,
    idempotent: true,
  })
}

export function unblockFriend(
  server: string,
  token: string,
  steamId: string | number,
): Promise<void> {
  return request(server, `/friends/${encodeURIComponent(String(steamId))}/block`, {
    method: 'DELETE',
    token,
  })
}

export function listBlocked(
  server: string,
  token: string,
): Promise<{ blocked: BlockedUser[] }> {
  return request(server, '/friends/blocked', { token })
}

export function searchUsers(
  server: string,
  token: string,
  query: string,
): Promise<{ users: SearchUser[] }> {
  return request(server, `/users/search?q=${encodeURIComponent(query)}`, { token })
}

export function logout(server: string, token: string): Promise<void> {
  return request(server, '/logout', { method: 'POST', token })
}

export function logoutAll(server: string, token: string): Promise<void> {
  return request(server, '/logout-all', { method: 'POST', token })
}

export function listSessions(
  server: string,
  token: string,
): Promise<{ sessions: SessionInfo[] }> {
  return request(server, '/sessions', { token })
}

export function revokeSession(
  server: string,
  token: string,
  id: string,
): Promise<void> {
  return request(server, `/sessions/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    token,
  })
}

export function getLobbies(server: string, token: string): Promise<{ lobbies: Lobby[] }> {
  return request(server, '/lobbies', { token })
}

export function getStats(
  server: string,
  token: string,
  steamId: string | number,
): Promise<UserStats> {
  return request(server, `/users/${encodeURIComponent(String(steamId))}/stats`, { token })
}

export type NewLobby = {
  appid: number | string
  type: number
  member_limit: number
}

export function createLobby(
  server: string,
  token: string,
  lobby: NewLobby,
): Promise<{ lobby_id: string | number }> {
  return request(server, '/lobbies', { method: 'POST', token, body: lobby })
}

export function joinLobby(
  server: string,
  token: string,
  lobbyId: string | number,
  invite?: string,
): Promise<void> {
  return request(server, `/lobbies/${encodeURIComponent(String(lobbyId))}/join`, {
    method: 'POST',
    token,
    ...(invite ? { body: { invite } } : {}),
  })
}

export function createLobbyInvite(
  server: string,
  token: string,
  lobbyId: string | number,
): Promise<LobbyInvite> {
  return request(server, `/lobbies/${encodeURIComponent(String(lobbyId))}/invite`, {
    method: 'POST',
    token,
  })
}
