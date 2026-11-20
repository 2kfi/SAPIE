import { useCallback, useRef, useState } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import Account from '../components/Account'
import { EmptyState } from '../components/States'
import LobbyPanel from '../components/LobbyPanel'
import {
  FriendsIcon,
  LiveIcon,
  LockIcon,
  ProfileIcon,
  SignOutIcon,
} from '../components/Icons'
import { joinLobby, messageOf, type Friend, type Session } from '../api'
import { gameLabel, personaLabel } from '../lib/presence'
import type { Route } from '../routes'
import Friends from './Friends'
import LiveAdmin from './LiveAdmin'

gsap.registerPlugin(useGSAP)

type ShellRoute = Exclude<Route, 'login' | 'register'>

type Props = {
  route: ShellRoute
  session: Session | null
  onNavigate: (route: Route) => void
  onNotice: (message: string) => void
  onSignOut: () => void
}

const primary: { route: ShellRoute; label: string; Icon: typeof FriendsIcon }[] = [
  { route: 'friends', label: 'Friends', Icon: FriendsIcon },
  { route: 'account', label: 'Profile', Icon: ProfileIcon },
]

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
  if (route === 'account') {
    return (
      <section className="view view--detail" aria-labelledby="account-note-title">
        <h2 id="account-note-title">Account access</h2>
        <p>Stats and achievements are read from the server as they were saved.</p>
        <p className="muted">
          <LockIcon className="inline-icon" /> Your persona name comes from the server. Other sessions can be revoked on the account screen.
        </p>
      </section>
    )
  }

  if (route === 'live') {
    return (
      <section className="view view--detail" aria-labelledby="session-note-title">
        <h2 id="session-note-title">Session details</h2>
        <p className="muted">Live lobbies are listed in the main pane.</p>
      </section>
    )
  }

  return (
    <section className="view view--detail" aria-labelledby="p2p-title">
      <h2 id="p2p-title">Playing together</h2>
      <p>Friends can play directly with each other, and the server relays traffic when a direct connection is not possible.</p>

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

export default function Shell({ route, session, onNavigate, onNotice, onSignOut }: Props) {
  const shellRef = useRef<HTMLDivElement>(null)
  const pulseIcon = useRef<(target: Element | null) => void>(() => {})
  const [friends, setFriends] = useState<Friend[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const selectedFriend = friends.find((friend) => String(friend.steam_id) === selectedId) ?? null

  useGSAP((_, contextSafe) => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches || !contextSafe) return
    gsap.timeline({ defaults: { duration: 0.32, ease: 'power2.out' } })
      .from('.rail__mark', { opacity: 0, scale: 0.88 }, 0)
      .from('.workspace .view__head', { opacity: 0, y: -8 }, 0.06)
      .from('.detail .view__head', { opacity: 0, y: -8 }, 0.12)

    pulseIcon.current = contextSafe((target: Element | null) => {
      if (!target) return
      gsap.fromTo(target, { scale: 0.85 }, { scale: 1, duration: 0.18, ease: 'power2.out' })
    })
  }, { scope: shellRef })

  const connectFriend = useCallback(async (friend: Friend) => {
    if (!session || !friend.lobby_id) return
    try {
      await joinLobby(session.server, session.token, friend.lobby_id)
      onNotice(`Joined ${friend.persona_name}'s lobby.`)
    } catch (reason) {
      onNotice(`Could not join ${friend.persona_name}: ${messageOf(reason)}`)
    }
  }, [session, onNotice])

  const needsSession = !session && (route === 'friends' || route === 'account' || route === 'live')

  return (
    <div className="shell" ref={shellRef}>
      <a className="skip-link" href="#main">Skip to content</a>
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
              onClick={(event) => {
                pulseIcon.current(event.currentTarget.querySelector('svg'))
                onNavigate(item)
              }}
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
            onClick={(event) => {
              pulseIcon.current(event.currentTarget.querySelector('svg'))
              onNavigate('live')
            }}
          >
            <LiveIcon />
            <span>Live sessions</span>
          </button>
          <button type="button" className="rail__item" onClick={onSignOut}>
            <SignOutIcon />
            <span>Sign out</span>
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
              <Friends
                session={session}
                onNotice={onNotice}
                onJoinFriend={(friend) => void connectFriend(friend)}
                onSelectFriend={(friend) => setSelectedId(String(friend.steam_id))}
                onFriendsChange={setFriends}
              />
            )}
            {route === 'account' && session && (
              <Account session={session} onNotice={onNotice} onSignedOut={onSignOut} />
            )}
            {route === 'live' && session && <LiveAdmin session={session} />}
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
