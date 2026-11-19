import { useCallback, useEffect, useState, type FormEvent } from 'react'
import {
  ApiError,
  getFriends,
  inviteFriend,
  messageOf,
  type Friend,
  type Session,
} from '../api'
import { EmptyState, ErrorState, LoadingState } from './States'

const REFRESH_MS = 15000

const personaLabels: Record<number, string> = {
  0: 'Offline',
  1: 'Online',
  2: 'Busy',
  3: 'Away',
  4: 'Snooze',
  5: 'Looking to trade',
  6: 'Looking to play',
}

export function personaLabel(value: string | number): string {
  const numeric = Number(value)
  if (Number.isInteger(numeric) && numeric in personaLabels) return personaLabels[numeric]
  return String(value)
}

export function personaTone(value: string | number): 'online' | 'away' | 'offline' {
  const numeric = Number(value)
  if (numeric === 1 || numeric === 5 || numeric === 6) return 'online'
  if (numeric === 2 || numeric === 3 || numeric === 4) return 'away'
  const text = String(value).toLowerCase()
  if (text === 'online') return 'online'
  if (text === 'away' || text === 'busy' || text === 'snooze') return 'away'
  return 'offline'
}

function decodePresenceValue(raw: string): string {
  if (typeof atob !== 'function') return raw
  try {
    const bytes = Uint8Array.from(atob(raw), (char) => char.charCodeAt(0))
    return new TextDecoder().decode(bytes)
  } catch {
    return raw
  }
}

function presenceSummary(value: Friend['rich_presence']): string | null {
  if (typeof value === 'string') return value.trim() || null
  if (!value || typeof value !== 'object') return null
  const pairs = Object.entries(value)
    .filter((entry): entry is [string, string] => typeof entry[1] === 'string' && entry[1].length > 0)
    .map(([key, raw]) => `${key}: ${decodePresenceValue(raw)}`)
  return pairs.length > 0 ? pairs.join(' · ') : null
}

export function gameLabel(friend: Friend): string {
  const presence = presenceSummary(friend.rich_presence)
  if (presence) return presence
  if (friend.appid != null && friend.appid !== '') return `App ${friend.appid}`
  return 'Not in a game'
}

type Props = {
  session: Session
  onNotice: (message: string) => void
  onJoinFriend: (friend: Friend) => void
  onSelectFriend: (friend: Friend) => void
  onFriendsChange: (friends: Friend[]) => void
}

export default function FriendList({
  session,
  onNotice,
  onJoinFriend,
  onSelectFriend,
  onFriendsChange,
}: Props) {
  const [friends, setFriends] = useState<Friend[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [busyId, setBusyId] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getFriends(session.server, session.token)
      const list = Array.isArray(data?.friends) ? data.friends : []
      setFriends(list)
      onFriendsChange(list)
    } catch (reason) {
      setError(messageOf(reason))
    } finally {
      setLoading(false)
    }
  }, [session.server, session.token, onFriendsChange])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => { void load() }, REFRESH_MS)
    return () => window.clearInterval(timer)
  }, [load])

  async function sendInvite(event: FormEvent<HTMLFormElement>, friend: Friend) {
    event.preventDefault()
    if (busyId) return
    const id = String(friend.steam_id)
    const connectStr = (drafts[id] ?? '').trim()
    setBusyId(id)
    try {
      await inviteFriend(session.server, session.token, id, connectStr)
      onNotice(`Invite sent to ${friend.persona_name}.`)
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 401) {
        onNotice('The session expired. Sign in again to send invites.')
      } else {
        onNotice(`Invite failed: ${messageOf(reason)}`)
      }
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className="stack">
      <div className="list-tools">
        <button type="button" className="btn btn--quiet" onClick={() => void load()} disabled={loading}>
          Refresh
        </button>
        <p className="muted">Presence reloads every 15 seconds.</p>
      </div>

      {loading && friends.length === 0 && <LoadingState label="Loading friends…" />}

      {error && (
        <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void load()}>Retry</button>}>
          {error}
        </ErrorState>
      )}

      {!loading && !error && friends.length === 0 && (
        <EmptyState>No friends are connected right now. Refresh to check again.</EmptyState>
      )}

      {friends.length > 0 && (
        <ul className="friend-list">
          {friends.map((friend) => {
            const tone = personaTone(friend.persona_state)
            const id = String(friend.steam_id)
            return (
              <li className="friend" key={id}>
                <div className="friend__head">
                  <button
                    type="button"
                    className="friend__name"
                    onClick={() => onSelectFriend(friend)}
                  >
                    {friend.persona_name}
                  </button>
                  <span className="badge badge--state">
                    <span className={`dot dot--${tone}`} aria-hidden="true" />
                    {personaLabel(friend.persona_state)}
                  </span>
                </div>

                <p>{gameLabel(friend)}</p>

                <div className="friend__meta">
                  <span className="mono">ID {friend.steam_id}</span>
                  {friend.lobby_id ? <span className="badge">Lobby {friend.lobby_id}</span> : null}
                </div>

                {!friend.lobby_id && friend.connect_str ? (
                  <p className="muted mono">Connect string: {friend.connect_str}</p>
                ) : null}

                <div className="friend__actions">
                  <button
                    type="button"
                    className="btn btn--quiet"
                    disabled={!friend.lobby_id || busyId === id}
                    onClick={() => onJoinFriend(friend)}
                  >
                    Join Friend
                  </button>
                  <form className="inline-form" onSubmit={(event) => void sendInvite(event, friend)}>
                    <label className="visually-hidden" htmlFor={`invite-${id}`}>
                      Connect string for {friend.persona_name}
                    </label>
                    <input
                      id={`invite-${id}`}
                      className="mono"
                      type="text"
                      placeholder="Connect string"
                      autoComplete="off"
                      spellCheck={false}
                      value={drafts[id] ?? ''}
                      onChange={(event) => setDrafts((current) => ({
                        ...current,
                        [id]: event.target.value,
                      }))}
                      required
                    />
                    <button type="submit" className="btn btn--quiet" disabled={busyId === id}>
                      Invite
                    </button>
                  </form>
                </div>
                {!friend.lobby_id ? <p className="muted">No lobby to join right now.</p> : null}
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
