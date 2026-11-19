import { EmptyState } from './States'
import { LockIcon } from './Icons'

export default function Account() {
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

      <p className="muted">Account details are not part of this version of the server API.</p>

      <div className="account-grid">
        <section className="panel" aria-labelledby="account-achievements">
          <h2 id="account-achievements">Achievements</h2>
          <EmptyState>No achievements are listed by the server.</EmptyState>
        </section>
        <section className="panel" aria-labelledby="account-games">
          <h2 id="account-games">Games</h2>
          <EmptyState>No games are listed by the server.</EmptyState>
        </section>
      </div>
    </section>
  )
}
