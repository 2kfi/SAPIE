import { useState } from 'react'
import type { GameRecord } from '../api'
import { readCover } from '../covers'
import { executableName, filterGames, isInstalled, type GameFilter, type GameSort } from '../library'

type Props = {
  games: GameRecord[]
  status: 'loading' | 'ready' | 'error'
  error: string | null
  installingPath: string | null
  progress: string | null
  onRetry: () => void
  onAdd: () => void
  onSelect: (game: GameRecord) => void
  onPlay: (game: GameRecord) => void
}

function TileCover({ game }: { game: GameRecord }) {
  const name = executableName(game.exe_path)
  const cover = readCover(game.exe_path)
  if (cover) {
    return <img className="tile__art" src={cover} alt={`Cover chosen for ${name}`} />
  }
  return (
    <div className="tile__monogram">
      <span aria-hidden="true">{game.appid ?? '?'}</span>
      <span className="tile__monogram-label">
        {game.appid === null ? 'No app ID yet' : 'App ID'}
      </span>
    </div>
  )
}

export default function LibraryScreen({
  games,
  status,
  error,
  installingPath,
  progress,
  onRetry,
  onAdd,
  onSelect,
  onPlay,
}: Props) {
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<GameFilter>('all')
  const [sort, setSort] = useState<GameSort>('name')
  const visible = filterGames(games, query, filter, sort)

  return (
    <section className="library" aria-label="Game library">
      {games.length > 0 && (
        <div className="library__toolbar">
          <label className="search">
            <span className="visually-hidden">Search games</span>
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search games"
            />
          </label>
          <label className="select">
            <span className="visually-hidden">Filter games</span>
            <select
              value={filter}
              onChange={(event) => setFilter(event.target.value as GameFilter)}
            >
              <option value="all">All games</option>
              <option value="installed">Library installed</option>
              <option value="not-installed">Library not installed</option>
            </select>
          </label>
          <label className="select">
            <span className="visually-hidden">Sort games</span>
            <select value={sort} onChange={(event) => setSort(event.target.value as GameSort)}>
              <option value="name">Sort by name</option>
              <option value="last_played">Sort by last played</option>
            </select>
          </label>
          <button type="button" className="btn btn--primary" onClick={onAdd}>
            Add a game
          </button>
        </div>
      )}

      {status === 'loading' && (
        <div className="state" role="status">
          <p className="state__title">Reading the local library.</p>
          <p>Checking the games saved on this device.</p>
        </div>
      )}

      {status === 'error' && (
        <div className="state state--error" role="alert">
          <div>
            <p className="state__title">The library could not be read.</p>
            <p>{error}</p>
          </div>
          <button type="button" className="btn" onClick={onRetry}>
            Retry
          </button>
        </div>
      )}

      {status === 'ready' && games.length === 0 && (
        <div className="state state--empty">
          <p className="state__title">No games in the library yet.</p>
          <p>Add a game you already own to install its Steam API library and write its config.</p>
          <button type="button" className="btn btn--primary" onClick={onAdd}>
            Add a game
          </button>
        </div>
      )}

      {status === 'ready' && games.length > 0 && visible.length === 0 && (
        <div className="state state--empty">
          <p className="state__title">No games match those filters.</p>
          <button
            type="button"
            className="btn"
            onClick={() => {
              setQuery('')
              setFilter('all')
            }}
          >
            Clear filters
          </button>
        </div>
      )}

      {status === 'ready' && visible.length > 0 && (
        <div className="tiles">
          {visible.map((game, index) => {
            const name = executableName(game.exe_path)
            const busy = installingPath === game.exe_path
            const canPlay = game.configured && isInstalled(game)
            return (
              <article className="tile" key={game.exe_path}>
                <button
                  type="button"
                  className="tile__open"
                  onClick={() => onSelect(game)}
                  aria-label={`Details for ${name}`}
                >
                  <TileCover game={game} />
                  <span className="tile__name">{name}</span>
                </button>
                <div className="tile__meta">
                  <span className="chip">
                    {isInstalled(game) ? 'Library installed' : 'Library not installed'}
                  </span>
                  <span className={game.configured ? 'chip chip--quiet' : 'chip chip--warn'}>
                    {game.configured ? `App ID ${game.appid}` : 'App ID not set'}
                  </span>
                </div>
                {busy && progress && (
                  <p className="progress" role="status" aria-live="polite">
                    {progress}
                  </p>
                )}
                {!canPlay && !busy && (
                  <p className="tile__hint">Install the library and set an app ID to enable Play.</p>
                )}
                <button
                  type="button"
                  className="btn btn--primary tile__play"
                  disabled={!canPlay || busy}
                  onClick={() => onPlay(game)}
                  aria-describedby={canPlay ? undefined : `tile-hint-${index}`}
                >
                  {busy ? 'Installing' : 'Play'}
                </button>
                {!canPlay && (
                  <span id={`tile-hint-${index}`} className="visually-hidden">
                    Install the library and set an app ID to enable Play.
                  </span>
                )}
              </article>
            )
          })}
        </div>
      )}
    </section>
  )
}
