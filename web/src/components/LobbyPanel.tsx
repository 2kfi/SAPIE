import { useCallback, useEffect, useState, type FormEvent } from 'react'
import {
  ApiError,
  createLobby,
  getLobbies,
  joinLobby,
  messageOf,
  type Lobby,
  type Session,
} from '../api'
import { EmptyState, ErrorState, LoadingState } from './States'

const REFRESH_MS = 15000

type Props = {
  session: Session
  onNotice: (message: string) => void
}

export default function LobbyPanel({ session, onNotice }: Props) {
  const [lobbies, setLobbies] = useState<Lobby[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [joiningId, setJoiningId] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getLobbies(session.server, session.token)
      setLobbies(Array.isArray(data?.lobbies) ? data.lobbies : [])
    } catch (reason) {
      setError(messageOf(reason))
    } finally {
      setLoading(false)
    }
  }, [session.server, session.token])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => { void load() }, REFRESH_MS)
    return () => window.clearInterval(timer)
  }, [load])

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (creating) return
    const form = event.currentTarget
    const data = new FormData(form)
    const appid = Number(String(data.get('appid') ?? '').trim())
    const typeText = String(data.get('type') ?? '').trim()
    const type = typeText === '' ? NaN : Number(typeText)
    const memberLimit = Number(String(data.get('member_limit') ?? '').trim())

    if (!Number.isInteger(appid) || appid < 1) {
      setFormError('App ID must be a whole number above zero.')
      return
    }
    if (!Number.isInteger(type) || type < 0) {
      setFormError('Lobby type must be a whole number of 0 or more.')
      return
    }
    if (!Number.isInteger(memberLimit) || memberLimit < 1) {
      setFormError('Member limit must be a whole number above zero.')
      return
    }

    setCreating(true)
    setFormError(null)
    try {
      await createLobby(session.server, session.token, {
        appid,
        type,
        member_limit: memberLimit,
      })
      form.reset()
      onNotice('Lobby created.')
      await load()
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 401) {
        setFormError('The session expired. Sign in again to create a lobby.')
      } else {
        setFormError(messageOf(reason))
      }
    } finally {
      setCreating(false)
    }
  }

  async function handleJoin(lobby: Lobby) {
    if (joiningId) return
    const id = String(lobby.lobby_id)
    setJoiningId(id)
    try {
      await joinLobby(session.server, session.token, lobby.lobby_id)
      onNotice(`Joined lobby ${id}.`)
      await load()
    } catch (reason) {
      onNotice(`Could not join lobby ${id}: ${messageOf(reason)}`)
    } finally {
      setJoiningId(null)
    }
  }

  return (
    <section className="stack" aria-labelledby="lobby-panel-title">
      <div className="section-head">
        <h3 id="lobby-panel-title">Lobbies</h3>
        <div className="list-tools">
          <button type="button" className="btn btn--quiet" onClick={() => void load()} disabled={loading}>
            Refresh
          </button>
        </div>
      </div>

      <form className="create-form" onSubmit={(event) => void handleCreate(event)}>
        <div className="field">
          <label htmlFor="lobby-appid">App ID</label>
          <input id="lobby-appid" name="appid" type="number" min="1" step="1" placeholder="App ID" required />
        </div>
        <div className="field">
          <label htmlFor="lobby-type">Lobby type</label>
          <input
            id="lobby-type"
            name="type"
            type="number"
            min="0"
            step="1"
            placeholder="Lobby type"
            required
          />
        </div>
        <div className="field">
          <label htmlFor="lobby-limit">Member limit</label>
          <input id="lobby-limit" name="member_limit" type="number" min="1" step="1" placeholder="Limit" required />
        </div>
        <div className="create-form__actions">
          <button type="submit" className="btn btn--quiet" disabled={creating}>
            {creating ? 'Creating…' : 'Create lobby'}
          </button>
          {formError ? <p role="alert">{formError}</p> : null}
        </div>
      </form>

      {loading && lobbies.length === 0 && <LoadingState label="Loading lobbies…" />}

      {error && (
        <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void load()}>Retry</button>}>
          {error}
        </ErrorState>
      )}

      {!loading && !error && lobbies.length === 0 && (
        <EmptyState>No lobbies are open yet. Create one above.</EmptyState>
      )}

      {lobbies.length > 0 && (
        <ul className="lobby-list">
          {lobbies.map((lobby) => {
            const id = String(lobby.lobby_id)
            return (
              <li className="lobby" key={id}>
                <div className="friend__head">
                  <span className="mono">Lobby {id}</span>
                  <span className="badge">{lobby.joinable ? 'Joinable' : 'Closed'}</span>
                </div>
                <div className="lobby__meta">
                  <span>Owner {lobby.owner}</span>
                  <span>App {lobby.appid}</span>
                  <span>Type {lobby.type}</span>
                  <span>{lobby.members.length} of {lobby.member_limit} members</span>
                </div>
                <ul className="member-list" aria-label={`Members of lobby ${id}`}>
                  {lobby.members.length > 0 ? (
                    lobby.members.map((member) => <li className="mono" key={String(member)}>{member}</li>)
                  ) : (
                    <li className="muted">No members reported.</li>
                  )}
                </ul>
                <div className="detail__actions">
                  <button
                    type="button"
                    className="btn btn--quiet"
                    disabled={!lobby.joinable || joiningId === id}
                    onClick={() => void handleJoin(lobby)}
                  >
                    Join
                  </button>
                  {!lobby.joinable ? <p className="muted">This lobby is closed to new members.</p> : null}
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
