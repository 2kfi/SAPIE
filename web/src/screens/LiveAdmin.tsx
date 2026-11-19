import { useState } from 'react'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import type { ViewState } from '../state'

type Props = {
  state: ViewState
  onNotice: (message: string) => void
}

function clockTime(date: Date): string {
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${hours}:${minutes}`
}

export default function LiveAdmin({ state, onNotice }: Props) {
  const [asOf, setAsOf] = useState(() => new Date())

  function retry() {
    setAsOf(new Date())
    onNotice('Presence feed is not connected in this scaffold.')
  }

  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Live sessions</h1>
          <p className="muted">Who is playing, and where.</p>
        </div>
        <p className="as-of">as of {clockTime(asOf)}</p>
      </header>

      {state === 'loading' ? (
        <LoadingState label="Loading presence…" />
      ) : state === 'error' ? (
        <ErrorState
          action={
            <button type="button" className="btn btn--quiet" onClick={retry}>
              Retry
            </button>
          }
        >
          Presence feed disconnected. Retry.
        </ErrorState>
      ) : (
        <>
          <div className="table-wrap">
            <table>
              <caption className="visually-hidden">Online players</caption>
              <thead>
                <tr>
                  <th scope="col">User</th>
                  <th scope="col">Game</th>
                  <th scope="col">Lobby</th>
                  <th scope="col">Since</th>
                </tr>
              </thead>
              <tbody />
            </table>
          </div>
          <EmptyState>Nobody is online right now.</EmptyState>
          <p className="legend">
            <span className="dot dot--online" aria-hidden="true" /> Online
            <span className="dot dot--away" aria-hidden="true" /> Away
          </p>
        </>
      )}
    </section>
  )
}
