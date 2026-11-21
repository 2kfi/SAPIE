import { useCallback, useEffect, useState } from 'react'
import {
  getStats,
  isUnauthorized,
  listSessions,
  logoutAll,
  messageOf,
  revokeSession,
  type Achievement,
  type Session,
  type SessionInfo,
  type UserStats,
} from '../api'
import AchievementList from './AchievementList'
import Avatar from './Avatar'
import { EmptyState, ErrorState, LoadingState } from './States'

type Props = {
  session: Session
  onNotice: (message: string) => void
  onSignedOut: () => void
}

export default function Account({ session, onNotice, onSignedOut }: Props) {
  const [stats, setStats] = useState<UserStats | null>(null)
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sessionError, setSessionError] = useState<string | null>(null)
  const [reload, setReload] = useState(0)
  const [busy, setBusy] = useState<string | null>(null)

  const loadSessions = useCallback(async () => {
    setSessionError(null)
    try {
      const data = await listSessions(session.server, session.token)
      setSessions(Array.isArray(data?.sessions) ? data.sessions : [])
    } catch (reason) {
      if (!isUnauthorized(reason)) setSessionError(messageOf(reason))
    }
  }, [session.server, session.token])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    getStats(session.server, session.token, session.steamId)
      .then((data) => { if (!cancelled) setStats(data) })
      .catch((reason) => {
        if (cancelled || isUnauthorized(reason)) return
        setError(messageOf(reason))
      })
      .finally(() => { if (!cancelled) setLoading(false) })
    void loadSessions()
    return () => { cancelled = true }
  }, [session, reload, loadSessions])

  function retry() {
    setReload((value) => value + 1)
  }

  async function revoke(item: SessionInfo) {
    if (busy) return
    setBusy(`revoke:${item.id}`)
    try {
      await revokeSession(session.server, session.token, item.id)
      onNotice('Session revoked.')
      await loadSessions()
    } catch (reason) {
      if (!isUnauthorized(reason)) onNotice(`Could not revoke session: ${messageOf(reason)}`)
    } finally {
      setBusy(null)
    }
  }

  async function signOutEverywhere() {
    if (busy) return
    setBusy('logout-all')
    try {
      await logoutAll(session.server, session.token)
    } catch {
      // Local sign out still stands if the server cannot be reached.
    } finally {
      setBusy(null)
      onSignedOut()
    }
  }

  const achievements: Achievement[] = Array.isArray(stats?.achievements) ? stats.achievements : []
  const values = Array.isArray(stats?.stats) ? stats.stats : []

  return (
    <section className="view">
      <header className="view__head">
        <div className="account__identity">
          <Avatar name={session.personaName || session.username} />
          <div>
            <h1>Your account</h1>
            <p className="muted">{session.personaName || session.username}</p>
          </div>
        </div>
        <button type="button" className="btn btn--quiet" onClick={onSignedOut}>
          Sign out
        </button>
      </header>

      <div className="account-grid">
        <section className="panel" aria-labelledby="account-stats">
          <h2 id="account-stats">Stats</h2>
          {loading ? <LoadingState label="Loading stats…" /> : error ? (
            <ErrorState action={<button type="button" className="btn btn--quiet" onClick={retry}>Retry</button>}>
              {error}
            </ErrorState>
          ) : values.length === 0 ? (
            <EmptyState>The server listed no stats for this account.</EmptyState>
          ) : (
            <dl className="fields">
              {values.map((entry) => (
                <div className="fields__row" key={entry.name}>
                  <dt>{entry.name}</dt>
                  <dd className="mono">{entry.value}</dd>
                </div>
              ))}
            </dl>
          )}
        </section>

        <section className="panel" aria-labelledby="account-achievements">
          <h2 id="account-achievements">Achievements</h2>
          {loading ? <LoadingState label="Loading achievements…" /> : error ? (
            <ErrorState action={<button type="button" className="btn btn--quiet" onClick={retry}>Retry</button>}>
              {error}
            </ErrorState>
          ) : (
            <AchievementList achievements={achievements} />
          )}
        </section>

        <section className="panel" aria-labelledby="account-sessions">
          <h2 id="account-sessions">Active sessions</h2>
          <p className="muted">Session IDs are digest prefixes, not usable tokens.</p>
          {sessionError ? (
            <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void loadSessions()}>Retry</button>}>
              {sessionError}
            </ErrorState>
          ) : sessions.length === 0 ? (
            <EmptyState>No active sessions listed.</EmptyState>
          ) : (
            <ul className="friend-list">
              {sessions.map((item) => (
                <li className="friend" key={item.id}>
                  <div className="friend__head">
                    <span className="mono">{item.id}</span>
                    {item.current ? <span className="badge">This device</span> : null}
                  </div>
                  <p className="muted">
                    Expires {new Date(item.expires_at * 1000).toLocaleString()}
                  </p>
                  {!item.current ? (
                    <div className="friend__actions">
                      <button
                        type="button"
                        className="btn btn--quiet"
                        disabled={busy !== null}
                        onClick={() => void revoke(item)}
                      >
                        {busy === `revoke:${item.id}` ? 'Revoking…' : 'Revoke'}
                      </button>
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
          <div className="detail__actions">
            <button
              type="button"
              className="btn btn--quiet"
              disabled={busy !== null}
              onClick={() => void signOutEverywhere()}
            >
              {busy === 'logout-all' ? 'Signing out…' : 'Sign out everywhere'}
            </button>
          </div>
        </section>
      </div>
    </section>
  )
}
