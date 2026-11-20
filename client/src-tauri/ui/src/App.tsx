import { useCallback, useEffect, useRef, useState } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { api, listenProgress, type GameRecord } from './api'
import { clearCover, saveCover } from './covers'
import { explain, readError, validServer } from './errors'
import { executableName } from './library'
import type { Level } from './types'
import { ActivityIcon, LibraryIcon, SettingsIcon, SignInIcon } from './icons'
import AddGameModal from './screens/AddGameModal'
import GameDetails, { type Presence } from './screens/GameDetails'
import LibraryScreen from './screens/LibraryScreen'
import SettingsScreen from './screens/SettingsScreen'
import SignIn, { type SignInStatus } from './screens/SignIn'

gsap.registerPlugin(useGSAP)

type EventLine = { id: number; at: string; level: Level; text: string }
type Toast = { id: number; level: Level; text: string }
type Session = { personaName: string; steamId: number; server: string }
type View = 'library' | 'settings' | 'signin'

type LibraryStatus = 'loading' | 'ready' | 'error'

const SERVER_KEY = 'sapie.server'

function cachedServer(): string {
  try {
    return localStorage.getItem(SERVER_KEY) ?? ''
  } catch {
    return ''
  }
}

export default function App() {
  const shellRef = useRef<HTMLDivElement>(null)
  const markRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const nextId = useRef(1)

  const [view, setView] = useState<View>(() => (cachedServer() ? 'library' : 'signin'))
  const [server, setServer] = useState(cachedServer)
  const [session, setSession] = useState<Session | null>(null)
  const [auth, setAuth] = useState<{ status: SignInStatus; error: string | null }>({
    status: 'idle',
    error: null,
  })
  const [games, setGames] = useState<GameRecord[]>([])
  const [libraryStatus, setLibraryStatus] = useState<LibraryStatus>('loading')
  const [libraryError, setLibraryError] = useState<string | null>(null)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [presence, setPresence] = useState<Presence>('unchecked')
  const [presenceError, setPresenceError] = useState<string | null>(null)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [progress, setProgress] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  const [coverVersion, setCoverVersion] = useState(0)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [online, setOnline] = useState(() => navigator.onLine)
  const [events, setEvents] = useState<EventLine[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])

  const log = useCallback((level: Level, text: string) => {
    setEvents((current) => [
      { id: nextId.current++, at: new Date().toISOString(), level, text },
      ...current,
    ])
  }, [])

  const notify = useCallback((level: Level, text: string) => {
    setToasts((current) => [...current, { id: nextId.current++, level, text }])
  }, [])

  const loadLibrary = useCallback(async (): Promise<GameRecord[]> => {
    setLibraryStatus('loading')
    setLibraryError(null)
    try {
      const list = await api.libraryList()
      setGames(list)
      setLibraryStatus('ready')
      return list
    } catch (error) {
      const message = explain(error)
      setLibraryError(message)
      setLibraryStatus('error')
      return []
    }
  }, [])

  useEffect(() => {
    let active = true
    let unlisten: (() => void) | undefined
    log('info', 'Launcher opened.')
    listenProgress((event) => {
      if (active) setProgress(event.message)
    })
      .then((stop) => {
        if (active) unlisten = stop
        else stop()
      })
      .catch(() => {})
    void loadLibrary()
    if (server) {
      void api
        .testConnection(server)
        .then(() => {
          if (!active) return
          setOnline(true)
          log('ok', 'SAPIE server is reachable.')
        })
        .catch((error) => {
          if (!active) return
          setOnline(false)
          log('error', `Server check failed: ${explain(error)}`)
        })
    }
    return () => {
      active = false
      unlisten?.()
    }
  }, [log, loadLibrary])

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
      if (event.key === 'Escape' && !addOpen) setDrawerOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [addOpen])

  useGSAP(
    () => {
      if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
      const targets = [markRef.current, headingRef.current].filter(
        (node): node is HTMLDivElement | HTMLHeadingElement => node !== null,
      )
      gsap.fromTo(
        targets,
        { opacity: 0, y: -6 },
        { opacity: 1, y: 0, duration: 0.28, ease: 'power2.out' },
      )
    },
    { dependencies: [view], scope: shellRef, revertOnUpdate: true },
  )

  async function selectGame(game: GameRecord) {
    setSelectedPath(game.exe_path)
    setPresence('checking')
    setPresenceError(null)
    setActionError(null)
    try {
      await api.detectGame(game.exe_path)
      setPresence('present')
    } catch (error) {
      const kind = readError(error).kind
      if (kind === 'not_found' || kind === 'invalid_path') setPresence('missing')
      else {
        setPresence('error')
        setPresenceError(explain(error))
      }
    }
  }

  async function signIn(targetServer: string, username: string, password: string) {
    setAuth({ status: 'checking', error: null })
    if (!validServer(targetServer)) {
      setAuth({ status: 'error', error: explain({ kind: 'invalid_server' }) })
      return
    }
    try {
      await api.testConnection(targetServer)
      setOnline(true)
    } catch (error) {
      const message = explain(error)
      setOnline(false)
      setAuth({ status: 'error', error: message })
      log('error', `Server check failed: ${message}`)
      return
    }
    setAuth({ status: 'signing_in', error: null })
    try {
      const result = await api.login(targetServer, username, password)
      setSession({
        personaName: result.persona_name,
        steamId: result.steam_id,
        server: result.server,
      })
      setServer(result.server)
      try {
        localStorage.setItem(SERVER_KEY, result.server)
      } catch {
        notify('info', 'The server address could not be saved on this device.')
      }
      setAuth({ status: 'idle', error: null })
      setView('library')
      log('ok', `Signed in as ${result.persona_name}.`)
    } catch (error) {
      const message = explain(error)
      setAuth({ status: 'error', error: message })
      log('error', `Sign in failed: ${message}`)
    }
  }

  async function signOut() {
    try {
      await api.signOut()
      localStorage.removeItem(SERVER_KEY)
      setSession(null)
      setServer('')
      setAuth({ status: 'idle', error: null })
      setView('signin')
      log('ok', 'Signed out. The local token was revoked.')
    } catch (error) {
      const message = explain(error)
      notify('error', message)
      log('error', `Sign out failed: ${message}`)
    }
  }

  async function play(game: GameRecord) {
    setBusyAction('play')
    setActionError(null)
    setProgress(null)
    try {
      const pid = await api.launchGame(game.exe_path)
      log('ok', `${executableName(game.exe_path)} started with process ${pid}.`)
      notify('ok', 'Game started.')
      await loadLibrary()
      await selectGame(game)
    } catch (error) {
      const message = explain(error)
      setActionError(message)
      log('error', `Launch failed: ${message}`)
      notify('error', message)
      if (readError(error).kind === 'token_expired') {
        setAuth({ status: 'error', error: message })
        setView('signin')
      }
    } finally {
      setBusyAction(null)
      setProgress(null)
    }
  }

  async function repair(game: GameRecord) {
    setBusyAction('repair')
    setActionError(null)
    setProgress(null)
    try {
      await api.installLibrary(game.exe_path)
      log('ok', 'Steam API library installed.')
      notify('ok', 'Library repaired.')
      await loadLibrary()
      await selectGame(game)
    } catch (error) {
      const message = explain(error)
      setActionError(message)
      log('error', `Repair failed: ${message}`)
      notify('error', message)
    } finally {
      setBusyAction(null)
      setProgress(null)
    }
  }

  async function restore(game: GameRecord) {
    setBusyAction('restore')
    setActionError(null)
    setProgress(null)
    try {
      await api.restoreLibrary(game.exe_path)
      log('ok', 'Original library restored.')
      notify('ok', 'Original library restored.')
      await loadLibrary()
      await selectGame(game)
    } catch (error) {
      const message = explain(error)
      setActionError(message)
      log('error', `Restore failed: ${message}`)
      notify('error', message)
    } finally {
      setBusyAction(null)
      setProgress(null)
    }
  }

  async function remove(game: GameRecord) {
    const confirmed = window.confirm(
      `Remove ${executableName(game.exe_path)} from the library? Any SAPIE library file is restored or removed.`,
    )
    if (!confirmed) return
    setBusyAction('remove')
    setActionError(null)
    setProgress(null)
    try {
      await api.libraryRemove(game.exe_path)
      if (selectedPath === game.exe_path) {
        setSelectedPath(null)
        setPresence('unchecked')
      }
      clearCover(game.exe_path)
      setCoverVersion((version) => version + 1)
      log('ok', 'Game removed from the library.')
      notify('ok', 'Game removed.')
      await loadLibrary()
    } catch (error) {
      const message = explain(error)
      setActionError(message)
      log('error', `Remove failed: ${message}`)
      notify('error', message)
    } finally {
      setBusyAction(null)
      setProgress(null)
    }
  }

  function storeCover(game: GameRecord, dataUrl: string) {
    try {
      saveCover(game.exe_path, dataUrl)
      setCoverVersion((version) => version + 1)
      log('ok', 'Cover saved on this device.')
    } catch (error) {
      const message = explain(error)
      notify('error', `Cover could not be saved: ${message}`)
      log('error', `Cover could not be saved: ${message}`)
    }
  }

  async function added(path: string) {
    setAddOpen(false)
    setProgress(null)
    const list = await loadLibrary()
    const game = list.find((item) => item.exe_path === path)
    if (game) await selectGame(game)
    notify('ok', 'Game is ready to play.')
  }

  function copyLog() {
    if (events.length === 0) {
      notify('info', 'No activity to copy.')
      return
    }
    const text = events
      .map((event) => `[${event.level.toUpperCase()}] ${event.at} ${event.text}`)
      .join('\n')
    void navigator.clipboard
      .writeText(text)
      .then(() => {
        notify('ok', 'Activity copied to clipboard.')
        log('ok', 'Activity copied to clipboard.')
      })
      .catch((error) => notify('error', `Could not copy activity: ${explain(error)}`))
  }

  const selected = games.find((game) => game.exe_path === selectedPath) ?? null
  const title = view === 'settings' ? 'Settings' : 'Library'

  return (
    <>
      <div className={`shell${view === 'library' ? '' : ' shell--wide'}`} ref={shellRef}>
        <nav className="rail" aria-label="Launcher">
          <div className="rail__mark" ref={markRef} aria-hidden="true">
            S
          </div>
          <div className="rail__group">
            <button
              type="button"
              className={`rail__item${view === 'library' ? ' is-active' : ''}`}
              aria-current={view === 'library' ? 'page' : undefined}
              onClick={() => setView('library')}
            >
              <LibraryIcon />
              <span>Library</span>
            </button>
            <button
              type="button"
              className={`rail__item${view === 'settings' ? ' is-active' : ''}`}
              aria-current={view === 'settings' ? 'page' : undefined}
              onClick={() => setView('settings')}
            >
              <SettingsIcon />
              <span>Settings</span>
            </button>
            {!session && (
              <button
                type="button"
                className={`rail__item${view === 'signin' ? ' is-active' : ''}`}
                aria-current={view === 'signin' ? 'page' : undefined}
                aria-label="Sign in view"
                onClick={() => setView('signin')}
              >
                <SignInIcon />
                <span>Sign in</span>
              </button>
            )}
          </div>
          <div className="rail__group rail__group--foot">
            <button
              type="button"
              className={`rail__item${drawerOpen ? ' is-active' : ''}`}
              aria-controls="activity-drawer"
              aria-expanded={drawerOpen}
              onClick={() => {
                setDrawerOpen(true)
                log('info', 'Activity opened.')
              }}
            >
              <ActivityIcon />
              <span>Activity</span>
            </button>
          </div>
        </nav>

        <main className="workspace">
          {view === 'signin' ? (
            <SignIn
              initialServer={server}
              status={auth.status}
              error={auth.error}
              onSignIn={(target, username, password) => void signIn(target, username, password)}
            />
          ) : (
            <>
              <header className="topbar">
                <div className="topbar__title">
                  <h1 ref={headingRef}>{title}</h1>
                  <p>
                    {view === 'library'
                      ? 'Games you add live here.'
                      : 'Account, server and build details.'}
                  </p>
                </div>
                <div className="topbar__tools">
                  {session && (
                    <span className="status" aria-live="polite">
                      Signed in as {session.personaName}
                    </span>
                  )}
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

              {view === 'library' ? (
                <LibraryScreen
                  games={games}
                  status={libraryStatus}
                  error={libraryError}
                  installingPath={busyAction === 'repair' ? selectedPath : null}
                  progress={progress}
                  onRetry={() => void loadLibrary()}
                  onAdd={() => {
                    setDrawerOpen(false)
                    setAddOpen(true)
                    setProgress(null)
                  }}
                  onSelect={(game) => void selectGame(game)}
                  onPlay={(game) => void play(game)}
                />
              ) : (
                <SettingsScreen
                  session={session}
                  cachedServer={server}
                  onSignIn={() => setView('signin')}
                  onSignOut={() => void signOut()}
                />
              )}
            </>
          )}
        </main>

        {view === 'library' && (
          <GameDetails
            key={`${selectedPath ?? 'none'}-${coverVersion}`}
            game={selected}
            presence={presence}
            presenceError={presenceError}
            busyAction={busyAction}
            progress={progress}
            actionError={actionError}
            onPlay={(game) => void play(game)}
            onRepair={(game) => void repair(game)}
            onRestore={(game) => void restore(game)}
            onRemove={(game) => void remove(game)}
            onCover={(game, dataUrl) => storeCover(game, dataUrl)}
            onClearCover={(game) => {
              clearCover(game.exe_path)
              setCoverVersion((version) => version + 1)
              log('ok', 'Cover removed.')
            }}
          />
        )}
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
            <button
              type="button"
              className="btn btn--quiet"
              onClick={() => {
                setEvents([])
                notify('info', 'Activity cleared.')
              }}
            >
              Clear
            </button>
            <button
              type="button"
              className="btn"
              ref={closeRef}
              onClick={() => {
                setDrawerOpen(false)
                log('info', 'Activity closed.')
              }}
            >
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

      {addOpen && (
        <AddGameModal
          progress={progress}
          onClose={() => {
            setAddOpen(false)
            setProgress(null)
          }}
          onAdded={(path) => void added(path)}
          onEvent={log}
        />
      )}

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
