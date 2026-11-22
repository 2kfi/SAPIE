import { useCallback, useEffect, useState } from 'react'
import { getLobbies, messageOf, type Lobby, type Session } from '../api'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

export default function LiveAdmin({ session }: { session: Session }) {
  const [lobbies, setLobbies] = useState<Lobby[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getLobbies(session.server, session.token)
      setLobbies(Array.isArray(data?.lobbies) ? data.lobbies : [])
    } catch (reason) {
      setLobbies([])
      setError(messageOf(reason))
    } finally {
      setLoading(false)
    }
  }, [session.server, session.token])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => { void load() }, 15000)
    return () => window.clearInterval(timer)
  }, [load])

  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Live sessions</h1>
          <p className="muted">Lobbies reported by the server.</p>
        </div>
      </header>

      <div className="list-tools">
        <button type="button" className="btn btn--primary" onClick={() => void load()} disabled={loading}>
          Refresh
        </button>
        <p className="muted">Lobbies reload every 15 seconds.</p>
      </div>

      <section className="panel" aria-labelledby="live-lobbies-title">
        <h2 id="live-lobbies-title">Lobbies</h2>
        {error ? (
          <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void load()}>Retry</button>}>
            {error}
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
                    <td>
                      <span className="status-pill">
                        <span
                          className={`dot dot--${lobby.joinable && lobby.members.length < lobby.member_limit ? 'online' : 'offline'}`}
                          aria-hidden="true"
                        />
                        {!lobby.joinable
                          ? 'Closed'
                          : lobby.members.length >= lobby.member_limit
                            ? 'Full'
                            : 'Joinable'}
                      </span>
                    </td>
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
