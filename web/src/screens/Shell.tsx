import Account from '../components/Account'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import {
  FriendsIcon,
  GamesIcon,
  LiveIcon,
  LockIcon,
  ProfileIcon,
} from '../components/Icons'
import type { Route } from '../routes'
import type { ViewState } from '../state'
import GameDetail from './GameDetail'
import LiveAdmin from './LiveAdmin'

type ShellRoute = Exclude<Route, 'login'>

type Props = {
  route: ShellRoute
  state: ViewState
  onNavigate: (route: ShellRoute) => void
  onNotice: (message: string) => void
}

const primary: { route: ShellRoute; label: string; Icon: typeof FriendsIcon }[] = [
  { route: 'friends', label: 'Friends', Icon: FriendsIcon },
  { route: 'games', label: 'Games', Icon: GamesIcon },
  { route: 'account', label: 'Profile', Icon: ProfileIcon },
]

function FriendsView({ state, onNotice }: Pick<Props, 'state' | 'onNotice'>) {
  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Friends</h1>
          <p className="muted">Your list loads after you connect to a server.</p>
        </div>
      </header>
      {state === 'loading' ? (
        <LoadingState label="Loading friends…" />
      ) : state === 'error' ? (
        <ErrorState
          action={
            <button
              type="button"
              className="btn btn--quiet"
              onClick={() => onNotice('Friend list is not connected in this scaffold.')}
            >
              Retry
            </button>
          }
        >
          Friend list could not be loaded.
        </ErrorState>
      ) : (
        <EmptyState>No friends yet. Add one by username.</EmptyState>
      )}
    </section>
  )
}

function GamesView({ state, onNotice }: Pick<Props, 'state' | 'onNotice'>) {
  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Games</h1>
          <p className="muted">Games configured for this launcher.</p>
        </div>
      </header>
      {state === 'loading' ? (
        <LoadingState label="Loading games…" />
      ) : state === 'error' ? (
        <ErrorState
          action={
            <button
              type="button"
              className="btn btn--quiet"
              onClick={() => onNotice('Game library is not connected in this scaffold.')}
            >
              Retry
            </button>
          }
        >
          Game library could not be loaded.
        </ErrorState>
      ) : (
        <EmptyState>Add a game to get started</EmptyState>
      )}
    </section>
  )
}

function DetailPane({ route, state, onNotice }: Pick<Props, 'route' | 'state' | 'onNotice'>) {
  if (route === 'games') {
    return <GameDetail state={state} onNotice={onNotice} />
  }

  if (route === 'account') {
    return (
      <section className="view view--detail" aria-labelledby="account-note-title">
        <h2 id="account-note-title">Account access</h2>
        <p>Achievements and games are shown as saved by the server.</p>
        <p className="muted">
          <LockIcon className="inline-icon" /> This view cannot change account data.
        </p>
      </section>
    )
  }

  if (route === 'live') {
    return (
      <section className="view view--detail" aria-labelledby="session-note-title">
        <h2 id="session-note-title">Session details</h2>
        <EmptyState>Select a session to inspect it.</EmptyState>
      </section>
    )
  }

  return (
    <section className="view view--detail" aria-labelledby="p2p-title">
      <h2 id="p2p-title">Playing together</h2>
      <p>Peer-to-peer play is not available in this version.</p>
      <p className="muted">Friend details and invites will appear here when a server is connected.</p>
    </section>
  )
}

export default function Shell({ route, state, onNavigate, onNotice }: Props) {
  return (
    <div className="shell">
      <nav className="rail glass" aria-label="Main navigation">
        <span className="rail__mark" aria-label="SAPIE">
          S
        </span>
        <div className="rail__group">
          {primary.map(({ route: item, label, Icon }) => (
            <button
              key={item}
              type="button"
              className={`rail__item${route === item ? ' is-active' : ''}`}
              aria-current={route === item ? 'page' : undefined}
              onClick={() => onNavigate(item)}
            >
              <Icon />
              <span>{label}</span>
            </button>
          ))}
        </div>
        <div className="rail__group rail__group--foot">
          <button
            type="button"
            className={`rail__item${route === 'live' ? ' is-active' : ''}`}
            aria-current={route === 'live' ? 'page' : undefined}
            onClick={() => onNavigate('live')}
          >
            <LiveIcon />
            <span>Live sessions</span>
          </button>
        </div>
      </nav>

      <main className="workspace" id="main">
        {route === 'friends' && <FriendsView state={state} onNotice={onNotice} />}
        {route === 'games' && <GamesView state={state} onNotice={onNotice} />}
        {route === 'account' && (
          <Account state={state} scope="self" onNotice={onNotice} />
        )}
        {route === 'live' && <LiveAdmin state={state} onNotice={onNotice} />}
      </main>

      <aside className="detail glass" aria-label="Details">
        <DetailPane route={route} state={state} onNotice={onNotice} />
      </aside>
    </div>
  )
}
