import { useCallback, useEffect, useRef, useState } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { ActivityIcon, LibraryIcon } from './icons'

gsap.registerPlugin(useGSAP)

type Level = 'info' | 'ok' | 'error'

type EventLine = {
  id: number
  at: string
  level: Level
  text: string
}

type Toast = {
  id: number
  level: Level
  text: string
}

type LibraryState =
  | { status: 'loading' }
  | { status: 'ready' }
  | { status: 'error'; message: string }

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

// M15 replaces this read with the real library loader. Reading now keeps the
// storage failure path real instead of inventing a sample game list.
function readLibraryState(): LibraryState {
  try {
    localStorage.getItem('sapie.library')
    return { status: 'ready' }
  } catch (error) {
    return {
      status: 'error',
      message: `Local storage is unavailable: ${describe(error)}`,
    }
  }
}

export default function App() {
  const shellRef = useRef<HTMLDivElement>(null)
  const markRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const detailRef = useRef<HTMLHeadingElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const nextId = useRef(1)

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [online, setOnline] = useState(() => navigator.onLine)
  const [library, setLibrary] = useState<LibraryState>({ status: 'loading' })
  const [events, setEvents] = useState<EventLine[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])

  const makeEvent = useCallback((level: Level, text: string): EventLine => {
    return { id: nextId.current++, at: new Date().toISOString(), level, text }
  }, [])

  const log = useCallback(
    (level: Level, text: string) => {
      setEvents((current) => [makeEvent(level, text), ...current])
    },
    [makeEvent],
  )

  const notify = useCallback((level: Level, text: string) => {
    setToasts((current) => [
      ...current,
      { id: nextId.current++, level, text },
    ])
  }, [])

  const reload = useCallback(() => {
    setLibrary({ status: 'loading' })
    setLibrary(readLibraryState())
  }, [])

  useEffect(() => {
    log('info', 'Launcher shell opened.')
    reload()
  }, [log, reload])

  useEffect(() => {
    const onOnline = () => {
      setOnline(true)
      log('ok', 'Network available.')
    }
    const onOffline = () => {
      setOnline(false)
      log('error', 'Network unavailable.')
    }
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    return () => {
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
    }
  }, [log])

  useEffect(() => {
    if (!drawerOpen) return
    closeRef.current?.focus()
  }, [drawerOpen])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setDrawerOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  useGSAP(
    () => {
      if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
      const targets = [markRef.current, headingRef.current, detailRef.current].filter(
        (node): node is HTMLDivElement | HTMLHeadingElement => node !== null,
      )
      gsap.fromTo(
        targets,
        { opacity: 0, y: -6 },
        { opacity: 1, y: 0, duration: 0.28, ease: 'power2.out' },
      )
    },
    { scope: shellRef },
  )

  function openDrawer() {
    setDrawerOpen(true)
    log('info', 'Activity opened.')
  }

  function closeDrawer() {
    setDrawerOpen(false)
    log('info', 'Activity closed.')
  }

  async function copyLog() {
    if (events.length === 0) {
      notify('info', 'No activity to copy.')
      return
    }
    const text = events
      .map((event) => `[${event.level.toUpperCase()}] ${event.at} ${event.text}`)
      .join('\n')
    try {
      await navigator.clipboard.writeText(text)
      notify('ok', 'Activity copied to clipboard.')
      log('ok', 'Activity copied to clipboard.')
    } catch (error) {
      notify('error', `Could not copy activity: ${describe(error)}`)
    }
  }

  function clearLog() {
    setEvents([])
    notify('info', 'Activity cleared.')
  }

  const trimmedQuery = query.trim()

  return (
    <>
      <div className="shell" ref={shellRef}>
        <nav className="rail" aria-label="Launcher">
          <div className="rail__mark" ref={markRef} aria-hidden="true">
            S
          </div>
          <div className="rail__group">
            <button
              type="button"
              className={`rail__item${drawerOpen ? '' : ' is-active'}`}
              aria-current={drawerOpen ? undefined : 'page'}
              onClick={() => setDrawerOpen(false)}
            >
              <LibraryIcon />
              <span>Library</span>
            </button>
          </div>
          <div className="rail__group rail__group--foot">
            <button
              type="button"
              className={`rail__item${drawerOpen ? ' is-active' : ''}`}
              aria-controls="activity-drawer"
              aria-expanded={drawerOpen}
              onClick={openDrawer}
            >
              <ActivityIcon />
              <span>Activity</span>
            </button>
          </div>
        </nav>

        <main className="workspace">
          <header className="topbar">
            <div className="topbar__title">
              <h1 ref={headingRef}>Library</h1>
              <p>Games you add live here.</p>
            </div>
            <div className="topbar__tools">
              <label className="search">
                <span className="visually-hidden">Search games</span>
                <input
                  type="search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Search games"
                />
              </label>
              <span
                className={`status ${online ? 'status--online' : 'status--offline'}`}
                aria-live="polite"
              >
                <span className="status__dot" aria-hidden="true" />
                Network {online ? 'available' : 'unavailable'}
              </span>
            </div>
          </header>

          {!online && (
            <div className="offline-banner" role="status">
              Network unavailable. A configured game can still launch from cached settings.
            </div>
          )}

          <section className="content" aria-label="Game library">
            {library.status === 'loading' && (
              <div className="state" role="status">
                <p className="state__title">Reading the local library.</p>
                <p>Checking saved state on this device.</p>
              </div>
            )}
            {library.status === 'error' && (
              <div className="state state--error" role="alert">
                <div>
                  <p className="state__title">The library could not be read.</p>
                  <p>{library.message}</p>
                </div>
                <button type="button" className="btn" onClick={reload}>
                  Retry
                </button>
              </div>
            )}
            {library.status === 'ready' && (
              <div className="library">
                <div className="section-head">
                  <h2>All games</h2>
                </div>
                <div className="state state--empty">
                  <p className="state__title">No games in the library yet.</p>
                  <p>
                    {trimmedQuery
                      ? `No games match "${trimmedQuery}".`
                      : 'A game appears here after you add it to the library.'}
                  </p>
                  {!trimmedQuery && <p>Adding a game arrives with the library flow.</p>}
                </div>
              </div>
            )}
          </section>
        </main>

        <aside className="detail" aria-label="Game details">
          <h2 ref={detailRef}>Game details</h2>
          <div className="detail__monogram" aria-hidden="true">
            ?
          </div>
          <div className="state state--compact">
            <p className="state__title">No game selected.</p>
            <p>Open a game from the library to see its path, architecture, interfaces and install state.</p>
          </div>
        </aside>
      </div>

      <section
        id="activity-drawer"
        className="activity"
        aria-label="Activity"
        hidden={!drawerOpen}
      >
        <div className="activity__head">
          <h2>Activity</h2>
          <div className="activity__tools">
            <button type="button" className="btn btn--quiet" onClick={copyLog}>
              Copy log
            </button>
            <button type="button" className="btn btn--quiet" onClick={clearLog}>
              Clear
            </button>
            <button type="button" className="btn" ref={closeRef} onClick={closeDrawer}>
              Close
            </button>
          </div>
        </div>
        {events.length === 0 ? (
          <p className="activity__empty">No activity recorded.</p>
        ) : (
          <ol className="activity__list">
            {events.map((event) => (
              <li key={event.id} className={`activity__line activity__line--${event.level}`}>
                <span className="activity__level">[{event.level.toUpperCase()}]</span>
                <time dateTime={event.at}>{new Date(event.at).toLocaleTimeString()}</time>
                <span>{event.text}</span>
              </li>
            ))}
          </ol>
        )}
      </section>

      <div className={`toast-region${drawerOpen ? ' toast-region--raised' : ''}`} aria-live="polite">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast toast--${toast.level}`}>
            <span>
              <span className="toast__level">{toast.level.toUpperCase()}</span> {toast.text}
            </span>
            <button
              type="button"
              onClick={() =>
                setToasts((current) => current.filter((item) => item.id !== toast.id))
              }
            >
              Dismiss
            </button>
          </div>
        ))}
      </div>
    </>
  )
}
