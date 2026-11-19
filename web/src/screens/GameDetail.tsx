import { useRef } from 'react'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { callNative } from '../tauri'
import type { ViewState } from '../state'

type Props = {
  state: ViewState
  onNotice: (message: string) => void
}

function report(onNotice: (message: string) => void, error: unknown) {
  onNotice(error instanceof Error ? error.message : String(error))
}

export default function GameDetail({ state, onNotice }: Props) {
  const pathRef = useRef<HTMLInputElement>(null)

  if (state === 'loading') {
    return <LoadingState label="Loading game details…" />
  }

  return (
    <section className="view view--detail" aria-labelledby="game-detail-title">
      <header className="view__head">
        <div>
          <h2 id="game-detail-title">Game details</h2>
          <p className="muted">Launch settings for the selected game.</p>
        </div>
      </header>

      {state === 'error' && (
        <ErrorState
          action={
            <button type="button" className="btn btn--quiet" onClick={() => pathRef.current?.focus()}>
              Edit path
            </button>
          }
        >
          Executable not found at {'<path>'}
        </ErrorState>
      )}

      <dl className="fields">
        <div className="fields__row">
          <dt>Name</dt>
          <dd>Not set</dd>
        </div>
        <div className="fields__row">
          <dt>App ID</dt>
          <dd className="mono">Not set</dd>
        </div>
        <div className="fields__row">
          <dt>
            <label htmlFor="exe-path">Executable path</label>
          </dt>
          <dd>
            <div className="inline-field">
              <input
                ref={pathRef}
                id="exe-path"
                className="mono"
                type="text"
                placeholder="Not set"
                autoComplete="off"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
              />
              <button
                type="button"
                className="btn btn--quiet"
                onClick={() =>
                  callNative('pick_executable').catch((error) => report(onNotice, error))
                }
              >
                Browse
              </button>
            </div>
          </dd>
        </div>
        <div className="fields__row">
          <dt>Arguments</dt>
          <dd className="mono">Not set</dd>
        </div>
        <div className="fields__row">
          <dt>Era</dt>
          <dd>Not set</dd>
        </div>
        <div className="fields__row">
          <dt>Architecture</dt>
          <dd>Not set</dd>
        </div>
      </dl>

      <div className="detail__actions">
        <button type="button" className="btn btn--primary" disabled>
          Launch
        </button>
        <p className="muted">Launch is unavailable until a game is added.</p>
      </div>

      {state === 'empty' && <EmptyState>Add a game to get started</EmptyState>}
    </section>
  )
}
