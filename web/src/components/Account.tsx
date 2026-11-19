import { useEffect, useState } from 'react'
import { ApiError, getStats, messageOf, type Achievement, type Session } from '../api'
import { EmptyState, ErrorState, LoadingState } from './States'
import { LockIcon } from './Icons'
import AchievementList from './AchievementList'

export default function Account({ session }: { session: Session | null }) {
  const [achievements, setAchievements] = useState<Achievement[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [reload, setReload] = useState(0)

  useEffect(() => {
    if (!session) return
    let cancelled = false
    setLoading(true)
    setError(null)
    getStats(session.server, session.token, session.steamId)
      .then((data) => {
        if (!cancelled) setAchievements(Array.isArray(data?.achievements) ? data.achievements : [])
      })
      .catch((reason) => {
        if (cancelled) return
        setAchievements([])
        setError(reason instanceof ApiError && reason.status === 401
          ? 'Your session was rejected. Sign in again to view achievements.'
          : messageOf(reason))
      })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [session, reload])

  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Your account</h1>
          <p className="muted">Saved games and achievements for this account.</p>
        </div>
        <span className="badge">
          <LockIcon className="badge__icon" />
          Read-only
        </span>
      </header>

      <div className="account-grid">
        <section className="panel" aria-labelledby="account-achievements">
          <h2 id="account-achievements">Achievements</h2>
          {!session ? (
            <EmptyState>Connect to a SAPIE server to view your achievements.</EmptyState>
          ) : error ? (
            <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => setReload((n) => n + 1)}>Retry</button>}>
              {error}
            </ErrorState>
          ) : loading ? (
            <LoadingState label="Loading achievements…" />
          ) : (
            <AchievementList achievements={achievements} />
          )}
        </section>
        <section className="panel" aria-labelledby="account-games">
          <h2 id="account-games">Games</h2>
          <EmptyState>No games are listed by the server.</EmptyState>
        </section>
      </div>
    </section>
  )
}
