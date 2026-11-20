import { useRef, useState, type ChangeEvent } from 'react'
import type { GameRecord } from '../api'
import { readCover } from '../covers'
import { executableName, isInstalled } from '../library'

export type Presence = 'unchecked' | 'checking' | 'present' | 'missing' | 'error'

type Props = {
  game: GameRecord | null
  presence: Presence
  presenceError: string | null
  busyAction: string | null
  progress: string | null
  actionError: string | null
  onPlay: (game: GameRecord) => void
  onRepair: (game: GameRecord) => void
  onRestore: (game: GameRecord) => void
  onRemove: (game: GameRecord) => void
  onCover: (game: GameRecord, dataUrl: string) => void
  onClearCover: (game: GameRecord) => void
}

export default function GameDetails({
  game,
  presence,
  presenceError,
  busyAction,
  progress,
  actionError,
  onPlay,
  onRepair,
  onRestore,
  onRemove,
  onCover,
  onClearCover,
}: Props) {
  const [coverError, setCoverError] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  if (!game) {
    return (
      <aside className="detail" aria-label="Game details">
        <h2>Game details</h2>
        <div className="detail__monogram" aria-hidden="true">
          ?
        </div>
        <div className="state state--compact">
          <p className="state__title">No game selected.</p>
          <p>Open a game from the library to see its path, architecture, interfaces and install state.</p>
        </div>
      </aside>
    )
  }

  const name = executableName(game.exe_path)
  const cover = readCover(game.exe_path)
  const canPlay = game.configured && isInstalled(game) && presence === 'present'
  const busy = busyAction !== null

  function pickCover(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (file.size > 1_500_000) {
      setCoverError('Choose an image under 1.5 MB.')
      return
    }
    setCoverError(null)
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') onCover(game!, reader.result)
    }
    reader.onerror = () => setCoverError('That image could not be read.')
    reader.readAsDataURL(file)
  }

  return (
    <aside className="detail" aria-label={`Details for ${name}`}>
      <h2>{name}</h2>
      {cover ? (
        <img className="detail__cover" src={cover} alt={`Cover chosen for ${name}`} />
      ) : (
        <div className="detail__monogram">
          <span aria-hidden="true">{game.appid ?? '?'}</span>
          <span className="detail__monogram-label">
            {game.appid === null ? 'No app ID yet' : 'App ID'}
          </span>
        </div>
      )}

      <dl className="detail__meta">
        <dt>Path</dt>
        <dd className="mono">{game.exe_path}</dd>
        <dt>Architecture</dt>
        <dd>{game.arch}</dd>
        <dt>Era</dt>
        <dd>{game.era}</dd>
        <dt>App ID</dt>
        <dd className="mono">{game.appid ?? 'Not set'}</dd>
        <dt>Library</dt>
        <dd>{game.library ?? 'Not installed'}</dd>
        <dt>Original backup</dt>
        <dd>{game.original_library ?? 'None recorded'}</dd>
        <dt>Interfaces</dt>
        <dd>{game.interfaces.length > 0 ? game.interfaces.join(', ') : 'None detected'}</dd>
      </dl>

      <div className="detail__presence" role="status" aria-live="polite">
        {presence === 'checking' && 'Checking the executable on disk.'}
        {presence === 'present' && 'Executable found on disk.'}
        {presence === 'missing' && 'Executable missing from disk. Check the path before playing.'}
        {presence === 'unchecked' && 'Executable presence has not been checked.'}
        {presence === 'error' && (presenceError ?? 'The executable could not be checked.')}
      </div>

      {actionError && (
        <p className="form__error" role="alert">
          {actionError}
        </p>
      )}

      {busy && progress && (
        <p className="progress" role="status" aria-live="polite">
          {progress}
        </p>
      )}

      <div className="detail__actions">
        <button
          type="button"
          className="btn btn--primary"
          disabled={!canPlay || busy}
          onClick={() => onPlay(game)}
        >
          Play
        </button>
        <button
          type="button"
          className="btn"
          disabled={presence !== 'present' || busy}
          onClick={() => onRepair(game)}
        >
          Repair library
        </button>
        <button
          type="button"
          className="btn"
          disabled={!game.original_library || busy}
          onClick={() => onRestore(game)}
        >
          Restore original
        </button>
        <button type="button" className="btn btn--quiet" disabled={busy} onClick={() => onRemove(game)}>
          Remove from library
        </button>
      </div>

      <div className="detail__cover-controls">
        <button type="button" className="btn" onClick={() => fileRef.current?.click()}>
          Choose cover
        </button>
        <input
          ref={fileRef}
          className="visually-hidden"
          type="file"
          accept="image/*"
          tabIndex={-1}
          aria-hidden="true"
          onChange={pickCover}
        />
        {cover && (
          <button type="button" className="btn btn--quiet" onClick={() => onClearCover(game)}>
            Remove cover
          </button>
        )}
        {coverError && (
          <p className="field__error" role="alert">
            {coverError}
          </p>
        )}
      </div>

      <h3>Launch settings</h3>
      <p className="detail__note">
        Arguments: {game.launch_args.length > 0 ? game.launch_args.join(' ') : 'none'}
      </p>
      <p className="detail__note">
        Linux preload: {game.ld_preload ?? 'none'}
      </p>
      <p className="detail__note">
        These values come from launcher state. This build has no command to change them, so they stay
        read-only here.
      </p>
    </aside>
  )
}
