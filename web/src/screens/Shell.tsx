import { useCallback, useState } from 'react'
import Account from '../components/Account'
import { EmptyState } from '../components/States'
import FriendList, { gameLabel, personaLabel } from '../components/FriendList'
import LobbyPanel from '../components/LobbyPanel'
import {
  FriendsIcon,
  GamesIcon,
  LiveIcon,
  LockIcon,
  ProfileIcon,
} from '../components/Icons'
import { joinLobby, messageOf, type Friend, type Session } from '../api'
import type { Route } from '../routes'
import GameDetail from './GameDetail'
import LiveAdmin from './LiveAdmin'

type ShellRoute = Exclude<Route, 'login'>

type Props = {
  route: ShellRoute
  session: Session | null
  onNavigate: (route: Route) => void
  onNotice: (message: string) => void
}

const primary: { route: ShellRoute; label: string; Icon: typeof FriendsIcon }[] = [
  { route: 'friends', label: 'Friends', Icon: FriendsIcon },
  { route: 'games', label: 'Games', Icon: GamesIcon },
  { route: 'account', label: 'Profile', Icon: ProfileIcon },
]

function FriendsView({
  session,
  onNotice,
  onJoinFriend,
  onSelectFriend,
  onFriendsChange,
}: {
  session: Session
  onNotice: Props['onNotice']
  onJoinFriend: (friend: Friend) => void
  onSelectFriend: (friend: Friend) => void
  onFriendsChange: (friends: Friend[]) => void
}) {
  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Friends</h1>
          <p className="muted">Presence reported by the connected server.</p>
        </div>
      </header>
      <FriendList
        session={session}
        onNotice={onNotice}
        onJoinFriend={onJoinFriend}
        onSelectFriend={onSelectFriend}
        onFriendsChange={onFriendsChange}
      />
    </section>
  )
}

function GamesView() {
  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Games</h1>
          <p className="muted">Games configured for this launcher.</p>
        </div>
      </header>
      <EmptyState>Add a game from the details pane to configure it for this server.</EmptyState>
    </section>
  )
}

function DetailPane({
  route,
  session,
  selectedFriend,
  onConnectFriend,
  onNotice,
}: Pick<Props, 'route' | 'session' | 'onNotice'> & {
  selectedFriend: Friend | null
  onConnectFriend: (friend: Friend) => void
}) {
  if (route === 'games') {
    return <GameDetail connection={session} onNotice={onNotice} />
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
        <p className="muted">Live users and lobbies are listed in the main pane.</p>
      </section>
    )
  }

  return (
    <section className="view view--detail" aria-labelledby="p2p-title">
      <h2 id="p2p-title">Playing together</h2>
      <p>Peer-to-peer play is not available in this version.</p>

      {selectedFriend ? (
        <div className="detail-card">
          <h3>{selectedFriend.persona_name}</h3>
          <dl className="fields">
            <div className="fields__row">
              <dt>Presence</dt>
              <dd>{personaLabel(selectedFriend.persona_state)}</dd>
            </div>
            <div className="fields__row">
              <dt>Game</dt>
              <dd>{gameLabel(selectedFriend)}</dd>
            </div>
            <div className="fields__row">
              <dt>Lobby</dt>
              <dd className="mono">{selectedFriend.lobby_id ?? 'None'}</dd>
            </div>
          </dl>
          <div className="detail__actions">
            <button
              type="button"
              className="btn btn--primary"
              disabled={!selectedFriend.lobby_id}
              onClick={() => onConnectFriend(selectedFriend)}
            >
              Connect to friend
            </button>
            <p className="muted">
              {selectedFriend.lobby_id
                ? `Joins lobby ${selectedFriend.lobby_id}.`
                : 'This friend is not in a lobby.'}
            </p>
          </div>
        </div>
      ) : (
        <EmptyState>Select a friend to see their details.</EmptyState>
      )}

      {session ? <LobbyPanel session={session} onNotice={onNotice} /> : null}
    </section>
  )
}

export default function Shell({ route, session, onNavigate, onNotice }: Props) {
  const [friends, setFriends] = useState<Friend[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const selectedFriend = friends.find((friend) => String(friend.steam_id) === selectedId) ?? null

  const connectFriend = useCallback(async (friend: Friend) => {
    if (!session || !friend.lobby_id) return
    try {
      await joinLobby(session.server, session.token, friend.lobby_id)
      onNotice(`Joined ${friend.persona_name}'s lobby.`)
    } catch (reason) {
      onNotice(`Could not join ${friend.persona_name}: ${messageOf(reason)}`)
    }
  }, [session, onNotice])

  const needsSession = !session && (route === 'friends' || route === 'games' || route === 'live')

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
        {needsSession ? (
          <section className="view">
            <header className="view__head">
              <h1>Sign in required</h1>
            </header>
            <EmptyState>Connect to a SAPIE server to view this screen.</EmptyState>
            <div className="detail__actions">
              <button type="button" className="btn btn--primary" onClick={() => onNavigate('login')}>
                Go to sign in
              </button>
            </div>
          </section>
        ) : (
          <>
            {route === 'friends' && session && (
              <FriendsView
                session={session}
                onNotice={onNotice}
                onJoinFriend={(friend) => void connectFriend(friend)}
                onSelectFriend={(friend) => setSelectedId(String(friend.steam_id))}
                onFriendsChange={setFriends}
              />
            )}
            {route === 'games' && <GamesView />}
            {route === 'account' && <Account />}
            {route === 'live' && session && <LiveAdmin session={session} onNotice={onNotice} />}
          </>
        )}
      </main>

      <aside className="detail glass" aria-label="Details">
        <DetailPane
          route={route}
          session={session}
          selectedFriend={selectedFriend}
          onConnectFriend={(friend) => void connectFriend(friend)}
          onNotice={onNotice}
        />
      </aside>
    </div>
  )
}
