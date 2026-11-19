import type { ViewState } from '../state'
import { EmptyState, ErrorState, LoadingState } from './States'
import { LockIcon } from './Icons'

type Props = {
  state: ViewState
  scope: 'self' | 'admin'
  onNotice: (message: string) => void
}

export default function Account({ state, scope, onNotice }: Props) {
  const heading = scope === 'self' ? 'Your account' : 'Account'

  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>{heading}</h1>
          <p className="muted">
            {scope === 'self'
              ? 'Saved games and achievements for this account.'
              : 'Opened by an administrator. Nothing here can be changed.'}
          </p>
        </div>
        <span className="badge">
          <LockIcon className="badge__icon" />
          Read-only
        </span>
      </header>

      {state === 'loading' ? (
        <LoadingState label="Loading account…" />
      ) : state === 'error' ? (
        <ErrorState
          action={
            <button
              type="button"
              className="btn btn--quiet"
              onClick={() => onNotice('Account data is not connected in this scaffold.')}
            >
              Retry
            </button>
          }
        >
          Account details could not be loaded.
        </ErrorState>
      ) : (
        <div className="account-grid">
          <section className="panel" aria-labelledby="account-achievements">
            <h2 id="account-achievements">Achievements</h2>
            <EmptyState>No achievements unlocked yet.</EmptyState>
          </section>
          <section className="panel" aria-labelledby="account-games">
            <h2 id="account-games">Games</h2>
            <EmptyState>No games on this account.</EmptyState>
          </section>
        </div>
      )}
    </section>
  )
}
