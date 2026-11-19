import { useEffect, useRef, useState } from 'react'
import { ErrorState, LoadingState } from '../components/States'
import {
  configureServer,
  copyDll,
  detectGame,
  launchGame,
  pickExecutable,
  setLdPreload,
  unpackSteamstub,
  type Connection,
  type DetectResult,
} from '../tauri'
type Props = {
  connection: Connection | null
  onNotice: (message: string) => void
}

type Game = {
  exePath: string
  detected: DetectResult
  dllPath: string
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

function eraLabel(era: string): string {
  if (era === 'legacy') return 'Legacy (pre-2016)'
  if (era === 'modern') return 'Modern'
  return 'Unknown'
}

function fileName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path
}

export default function GameDetail({ connection, onNotice }: Props) {
  const pathRef = useRef<HTMLInputElement>(null)
  const [game, setGame] = useState<Game | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!game || !connection) return
    configureServer(game.exePath, connection)
      .then(() => onNotice(`Server updated to ${connection.server}.`))
      .catch((reason) => setError(message(reason)))
    // Reconfigure only when the connection changes; adding a game configures it already.
  }, [connection])

  async function addGame() {
    if (!connection) {
      setError('Connect to a server on the login screen before adding a game.')
      return
    }
    setError(null)
    setBusy('Choose the game executable…')
    try {
      const picked = await pickExecutable()
      if (!picked) {
        setBusy(null)
        return
      }

      setBusy('Checking for SteamStub…')
      let exePath = picked
      let unpackNote: string
      try {
        const unpacked = await unpackSteamstub(picked)
        if (unpacked) {
          exePath = unpacked
          unpackNote = 'SteamStub removed; using the unpacked executable.'
        } else {
          unpackNote = 'No SteamStub found; using the original executable.'
        }
      } catch (reason) {
        unpackNote = `SteamStub check skipped: ${message(reason)}`
      }

      setBusy('Detecting the game…')
      const detected = await detectGame(exePath)

      setBusy('Installing the SAPIE DLL…')
      const dllPath = await copyDll(exePath, detected.era)

      setBusy('Writing the connection files…')
      await configureServer(exePath, connection)

      try {
        await setLdPreload(exePath, dllPath)
      } catch (reason) {
        unpackNote += ` LD_PRELOAD was not set: ${message(reason)}`
      }

      setGame({ exePath, detected, dllPath })
      onNotice(`${unpackNote} Game is ready to launch.`)
    } catch (reason) {
      setError(message(reason))
    } finally {
      setBusy(null)
    }
  }

  async function launch() {
    if (!game) return
    setError(null)
    setBusy('Launching game…')
    try {
      await launchGame(game.exePath)
      onNotice('Game launched with SAPIE credentials.')
    } catch (reason) {
      setError(message(reason))
    } finally {
      setBusy(null)
    }
  }

  return (
    <section className="view view--detail" aria-labelledby="game-detail-title">
      <header className="view__head">
        <div>
          <h2 id="game-detail-title">Game details</h2>
          <p className="muted">Launch settings for the selected game.</p>
        </div>
      </header>

      {busy && <LoadingState label={busy} />}

      {!busy && error && (
        <ErrorState action={<button type="button" className="btn btn--quiet" onClick={addGame}>Try again</button>}>
          {error}
        </ErrorState>
      )}

      <dl className="fields">
        <div className="fields__row">
          <dt>Name</dt>
          <dd>{game ? fileName(game.exePath) : 'Not set'}</dd>
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
                value={game?.exePath ?? ''}
                readOnly={!!game}
                placeholder="Not set"
                autoComplete="off"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
              />
              <button type="button" className="btn btn--quiet" onClick={addGame} disabled={!!busy}>
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
          <dd>{game ? eraLabel(game.detected.era) : 'Not set'}</dd>
        </div>
        <div className="fields__row">
          <dt>Architecture</dt>
          <dd>{game ? game.detected.arch : 'Not set'}</dd>
        </div>
        <div className="fields__row">
          <dt>Interfaces</dt>
          <dd className="mono">{game ? `${game.detected.interfaces.length} detected` : 'Not set'}</dd>
        </div>
      </dl>

      <div className="detail__actions">
        <button type="button" className="btn btn--primary" disabled={!game || !!busy} onClick={launch}>
          Launch
        </button>
        <p className="muted">
          {game ? 'Ready to launch.' : 'Launch is unavailable until a game is added.'}
        </p>
      </div>
    </section>
  )
}
