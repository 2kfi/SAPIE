import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, type DetectResult } from '../api'
import { explain } from '../errors'
import { knownAppid } from '../library'
import type { Level } from '../types'

type Props = {
  progress: string | null
  onClose: () => void
  onAdded: (path: string) => void
  onEvent: (level: Level, text: string) => void
}

function absolutePath(value: string): boolean {
  const path = value.trim()
  return path.startsWith('/') || /^[A-Za-z]:[\\/]/.test(path) || path.startsWith('\\\\')
}

export default function AddGameModal({ progress, onClose, onAdded, onEvent }: Props) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const pathRef = useRef<HTMLInputElement>(null)
  const [path, setPath] = useState('')
  const [detection, setDetection] = useState<DetectResult | null>(null)
  const [inspecting, setInspecting] = useState(false)
  const [inspectError, setInspectError] = useState<string | null>(null)
  const [appid, setAppid] = useState('')
  const [appidError, setAppidError] = useState<string | null>(null)
  const [unpack, setUnpack] = useState(false)
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  useEffect(() => {
    pathRef.current?.focus()
  }, [])

  useEffect(() => {
    function onKeyDown(event: globalThis.KeyboardEvent) {
      if (event.key === 'Escape' && !adding) onClose()
      if (event.key !== 'Tab' || !dialogRef.current) return
      const focusable = Array.from(
        dialogRef.current.querySelectorAll<HTMLElement>(
          'button:not([disabled]), input:not([disabled]), select:not([disabled]), summary',
        ),
      )
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [adding, onClose])

  async function inspect() {
    const target = path.trim()
    if (!absolutePath(target)) {
      setInspectError('Enter an absolute path to the executable.')
      setDetection(null)
      return
    }
    setInspecting(true)
    setInspectError(null)
    setDetection(null)
    try {
      const result = await api.detectGame(target)
      setDetection(result)
      setAppid(knownAppid(target) === null ? '' : String(knownAppid(target)))
      onEvent('ok', 'Executable inspected.')
    } catch (error) {
      setInspectError(explain(error))
      onEvent('error', `Inspection failed: ${explain(error)}`)
    } finally {
      setInspecting(false)
    }
  }

  async function add(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const target = path.trim()
    if (!detection || detection.exe_path !== target) {
      setAddError('Inspect this path before adding it.')
      return
    }
    const parsed = Number(appid)
    if (!Number.isInteger(parsed) || parsed < 1) {
      setAppidError('Enter a whole app ID greater than zero.')
      return
    }
    setAddError(null)
    setAppidError(null)
    setAdding(true)
    try {
      await api.libraryAdd(target)
      onEvent('ok', 'Game added to the library.')
      await api.installLibrary(target)
      onEvent('ok', 'Steam API library installed.')
      if (unpack) {
        const result = await api.unpackSteamstub(target)
        onEvent('ok', result ?? 'SteamStub unpack finished.')
      }
      await api.configureGame(target, parsed)
      onEvent('ok', 'Game configuration written.')
      onAdded(target)
    } catch (error) {
      const message = explain(error)
      setAddError(message)
      onEvent('error', `Add game failed: ${message}`)
    } finally {
      setAdding(false)
    }
  }

  return (
    <div className="dialog-backdrop">
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="add-title"
        ref={dialogRef}
      >
        <div className="modal__head">
          <h2 id="add-title">Add a game</h2>
          <button type="button" className="btn btn--quiet" onClick={onClose} disabled={adding}>
            Close
          </button>
        </div>
        <form onSubmit={add} className="form">
          <div className="field">
            <label htmlFor="game-path">Executable path</label>
            <div className="field__row">
              <input
                id="game-path"
                ref={pathRef}
                value={path}
                onChange={(event) => {
                  setPath(event.target.value)
                  setDetection(null)
                }}
                placeholder="/games/my-game/game.exe"
                disabled={adding}
                required
              />
              <button type="button" className="btn" onClick={inspect} disabled={inspecting || adding}>
                {inspecting ? 'Inspecting' : 'Inspect'}
              </button>
            </div>
            <p className="field__hint">
              Paste the absolute path. This build has no native file picker command.
            </p>
            {inspectError && (
              <p className="field__error" role="alert">
                {inspectError}
              </p>
            )}
          </div>

          {detection && (
            <dl className="detail__meta detail__meta--modal">
              <dt>Architecture</dt>
              <dd>{detection.arch}</dd>
              <dt>Era</dt>
              <dd>{detection.era}</dd>
              <dt>Interfaces</dt>
              <dd>{detection.interfaces.length} found</dd>
              <dt>Original library</dt>
              <dd>{detection.original_library ?? 'None found'}</dd>
            </dl>
          )}
          {detection?.warning && <p className="field__hint">{detection.warning}</p>}

          <div className="field">
            <label htmlFor="game-appid">App ID</label>
            <input
              id="game-appid"
              inputMode="numeric"
              value={appid}
              onChange={(event) => {
                setAppid(event.target.value)
                setAppidError(null)
              }}
              disabled={adding}
              required
            />
            <p className="field__hint">
              {detection && knownAppid(path) !== null
                ? 'Prefilled from the executable name. Change it if this is not that game.'
                : 'No known app ID for this executable. Enter the one you own.'}
            </p>
            {appidError && (
              <p className="field__error" role="alert">
                {appidError}
              </p>
            )}
          </div>

          <details className="advanced">
            <summary>Advanced options</summary>
            <label className="check">
              <input
                type="checkbox"
                checked={unpack}
                onChange={(event) => setUnpack(event.target.checked)}
                disabled={adding}
              />
              Unpack SteamStub before configuring
            </label>
            <p className="field__hint">
              Runs the bundled Windows sidecar on this executable. Leave this off unless the game
              needs it.
            </p>
            <p className="field__hint">
              Launch arguments and the Linux LD_PRELOAD override live in launcher state. This build
              cannot edit them, so they stay read-only in game details.
            </p>
          </details>

          {adding && progress && (
            <p className="progress" role="status" aria-live="polite">
              {progress}
            </p>
          )}
          {adding && !progress && (
            <p className="progress" role="status" aria-live="polite">
              Working on this game.
            </p>
          )}
          {addError && (
            <p className="form__error" role="alert">
              {addError}
            </p>
          )}
          <div className="modal__actions">
            <button type="button" className="btn" onClick={onClose} disabled={adding}>
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn--primary"
              disabled={adding || detection === null}
              aria-busy={adding}
            >
              {adding ? 'Adding to library' : 'Add to library'}
            </button>
          </div>
          {adding && (
            <p className="field__hint">This step cannot be cancelled safely once it starts.</p>
          )}
        </form>
      </div>
    </div>
  )
}
