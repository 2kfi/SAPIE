import { useCallback, useEffect, useState, type FormEvent } from 'react'
import {
  ApiError,
  getAdminUsers,
  getLobbies,
  messageOf,
  type Lobby,
  type PresenceUser,
  type Session,
} from '../api'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { gameLabel, personaLabel, personaTone } from '../components/FriendList'

type Props = {
  session: Session
  onNotice: (message: string) => void
}

function clockTime(date: Date): string {
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${hours}:${minutes}`
}

export default function LiveAdmin({ session, onNotice }: Props) {
  const [tokenDraft, setTokenDraft] = useState('')
  const [adminToken, setAdminToken] = useState('')
  const [users, setUsers] = useState<PresenceUser[]>([])
  const [lobbies, setLobbies] = useState<Lobby[]>([])
  const [loading, setLoading] = useState(false)
  const [usersError, setUsersError] = useState<string | null>(null)
  const [lobbiesError, setLobbiesError] = useState<string | null>(null)
  const [asOf, setAsOf] = useState<Date | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    let failed = false

    if (adminToken) {
      setUsersError(null)
      try {
        const data = await getAdminUsers(session.server, adminToken)
        setUsers(Array.isArray(data?.users) ? data.users : [])
      } catch (reason) {
        setUsers([])
        setUsersError(reason instanceof ApiError && reason.status === 401
          ? 'The admin token was rejected.'
          : messageOf(reason))
        failed = true
      }
    } else {
      setUsers([])
      setUsersError(null)
    }

    setLobbiesError(null)
    try {
      const data = await getLobbies(session.server, session.token)
      setLobbies(Array.isArray(data?.lobbies) ? data.lobbies : [])
    } catch (reason) {
      setLobbies([])
      setLobbiesError(messageOf(reason))
      failed = true
    }

    if (!failed) setAsOf(new Date())
    setLoading(false)
  }, [adminToken, session.server, session.token])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => { void load() }, 15000)
    return () => window.clearInterval(timer)
  }, [load])

  function applyToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const token = tokenDraft.trim()
    if (!token) {
      onNotice('Enter an admin token to read the live user list.')
      return
    }
    setAdminToken(token)
    setTokenDraft('')
  }

  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Live sessions</h1>
          <p className="muted">Presence and lobbies reported by the server.</p>
        </div>
        {asOf ? <p className="as-of">as of {clockTime(asOf)}</p> : null}
      </header>

      <form className="admin-token" onSubmit={applyToken}>
        <label htmlFor="admin-token">Admin token</label>
        <div className="inline-field">
          <input
            id="admin-token"
            type="password"
            value={tokenDraft}
            onChange={(event) => setTokenDraft(event.target.value)}
            placeholder="Admin token"
            autoComplete="off"
            spellCheck={false}
          />
          <button type="submit" className="btn btn--quiet">Apply</button>
        </div>
        <p className="muted">Required for the user list. Held in memory only.</p>
      </form>

      <div className="list-tools">
        <button type="button" className="btn btn--quiet" onClick={() => void load()} disabled={loading}>
          Refresh
        </button>
        <p className="muted">Both lists reload every 15 seconds.</p>
      </div>

      <section className="panel" aria-labelledby="live-users-title">
        <h2 id="live-users-title">Live users</h2>
        {!adminToken ? (
          <EmptyState>Enter an admin token to view live users.</EmptyState>
        ) : usersError ? (
          <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void load()}>Retry</button>}>
            {usersError}
          </ErrorState>
        ) : loading && users.length === 0 ? (
          <LoadingState label="Loading live users…" />
        ) : users.length === 0 ? (
          <EmptyState>No users are connected right now.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table>
              <caption className="visually-hidden">Live users with presence</caption>
              <thead>
                <tr>
                  <th scope="col">User</th>
                  <th scope="col">State</th>
                  <th scope="col">Game</th>
                  <th scope="col">Lobby</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.steam_id}>
                    <td>{user.persona_name}</td>
                    <td>
                      <span className={`badge badge--state badge--tone-${personaTone(user.persona_state ?? 0)}`}>
                        <span className={`dot dot--${personaTone(user.persona_state ?? 0)}`} aria-hidden="true" />
                        {personaLabel(user.persona_state ?? 0)}
                      </span>
                    </td>
                    <td>{gameLabel({
                      steam_id: user.steam_id,
                      persona_name: user.persona_name,
                      persona_state: user.persona_state ?? 0,
                      appid: user.appid ?? null,
                      rich_presence: user.rich_presence ?? null,
                      lobby_id: user.lobby_id ?? null,
                      connect_str: user.connect_str ?? null,
                    })}</td>
                    <td className="mono">{user.lobby_id ?? 'None'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="panel" aria-labelledby="live-lobbies-title">
        <h2 id="live-lobbies-title">Lobbies</h2>
        {lobbiesError ? (
          <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void load()}>Retry</button>}>
            {lobbiesError}
          </ErrorState>
        ) : loading && lobbies.length === 0 ? (
          <LoadingState label="Loading lobbies…" />
        ) : lobbies.length === 0 ? (
          <EmptyState>No lobbies are open right now.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table>
              <caption className="visually-hidden">Open lobbies</caption>
              <thead>
                <tr>
                  <th scope="col">Lobby</th>
                  <th scope="col">Owner</th>
                  <th scope="col">App</th>
                  <th scope="col">Members</th>
                  <th scope="col">State</th>
                </tr>
              </thead>
              <tbody>
                {lobbies.map((lobby) => (
                  <tr key={lobby.lobby_id}>
                    <td className="mono">{lobby.lobby_id}</td>
                    <td>{lobby.owner}</td>
                    <td>{lobby.appid}</td>
                    <td>{lobby.members.length} of {lobby.member_limit}</td>
                    <td>{lobby.joinable ? 'Joinable' : 'Closed'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  )
}
