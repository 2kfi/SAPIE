import type { Connection } from './tauri'

export type Session = Connection & {
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

export type Lobby = {
  lobby_id: string | number
  owner: string | number
  appid: string | number
  member_limit: number
  type: string | number
  joinable: boolean
  members: (string | number)[]
}

export type PresenceUser = {
  steam_id: string | number
  persona_name: string
  persona_state?: string | number
  appid?: string | number | null
  rich_presence?: PresenceValues | string | null
  lobby_id?: string | number | null
  connect_str?: string | null
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
  unlock_time: number
}

export type UserStats = {
  steam_id: string | number
  stats: { name: string; value: string }[]
  achievements: Achievement[]
}

export class ApiError extends Error {
  constructor(message: string, readonly status?: number) {
    super(message)
    this.name = 'ApiError'
  }
}

export function messageOf(reason: unknown): string {
  return reason instanceof Error ? reason.message : String(reason)
}

function baseUrl(server: string): string {
  return `http://${server}/api/v1`
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
  method?: 'GET' | 'POST'
  body?: unknown
  token?: string
  timeoutMs?: number
}

async function request<T>(
  server: string,
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = 'GET', body, token, timeoutMs = 8000 } = options
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), timeoutMs)
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`

  try {
    const response = await fetch(`${baseUrl(server)}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    })
    if (!response.ok) {
      const detail = await errorDetail(response)
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
    throw new ApiError(`Could not reach the server at ${server}. Check the address and try again.`)
  } finally {
    window.clearTimeout(timer)
  }
}

export type LoginResult = {
  token: string
  steamId: string
  personaName: string
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
): Promise<void> {
  return request(server, `/lobbies/${encodeURIComponent(String(lobbyId))}/join`, {
    method: 'POST',
    token,
  })
}

export async function getAdminUsers(
  server: string,
  adminToken: string,
): Promise<PresenceUser[]> {
  const data = await request<PresenceUser[] | { users?: PresenceUser[] }>(server, '/users', {
    token: adminToken,
  })
  return Array.isArray(data) ? data : Array.isArray(data?.users) ? data.users : []
}
