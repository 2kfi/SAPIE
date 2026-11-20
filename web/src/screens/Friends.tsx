import { useCallback, useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import {
  ApiError,
  acceptFriendRequest,
  blockFriend,
  cancelFriendRequest,
  declineFriendRequest,
  getFriends,
  inviteFriend,
  isUnauthorized,
  listBlocked,
  listFriendRequests,
  messageOf,
  removeFriend,
  sendFriendRequest,
  unblockFriend,
  type BlockedUser,
  type Friend,
  type FriendRequest,
  type Session,
} from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { gameLabel, personaLabel, personaTone } from '../lib/presence'

const REFRESH_MS = 15000
const TABS = ['friends', 'requests', 'blocked'] as const
type Tab = (typeof TABS)[number]
const TAB_LABELS: Record<Tab, string> = {
  friends: 'Friends',
  requests: 'Requests',
  blocked: 'Blocked',
}

type Props = {
  session: Session
  onNotice: (message: string) => void
  onSelectFriend: (friend: Friend) => void
  onJoinFriend: (friend: Friend) => void
  onFriendsChange: (friends: Friend[]) => void
}

type Confirmation = { kind: 'remove' | 'block'; friend: Friend }

function requestError(reason: unknown): string {
  if (reason instanceof ApiError && reason.status === 404) {
    return 'No account matched that username, or a request already exists.'
  }
  if (reason instanceof ApiError && reason.status === 409) {
    return 'A request already exists for this account.'
  }
  if (reason instanceof ApiError && reason.status === 429) {
    return 'Too many requests. Try again later.'
  }
  return messageOf(reason)
}

export default function Friends({ session, onNotice, onSelectFriend, onJoinFriend, onFriendsChange }: Props) {
  const [tab, setTab] = useState<Tab>('friends')
  const [friends, setFriends] = useState<Friend[]>([])
  const [incoming, setIncoming] = useState<FriendRequest[]>([])
  const [outgoing, setOutgoing] = useState<FriendRequest[]>([])
  const [blocked, setBlocked] = useState<BlockedUser[]>([])
  const [loading, setLoading] = useState<Record<Tab, boolean>>({
    friends: false,
    requests: false,
    blocked: false,
  })
  const [errors, setErrors] = useState<Record<Tab, string | null>>({
    friends: null,
    requests: null,
    blocked: null,
  })
  const [username, setUsername] = useState('')
  const [message, setMessage] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)
  const [invites, setInvites] = useState<Record<string, string>>({})
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([])

  const loadFriends = useCallback(async () => {
    setLoading((state) => ({ ...state, friends: true }))
    setErrors((state) => ({ ...state, friends: null }))
    try {
      const data = await getFriends(session.server, session.token)
      const list = Array.isArray(data?.friends) ? data.friends : []
      setFriends(list)
      onFriendsChange(list)
    } catch (reason) {
      if (!isUnauthorized(reason)) setErrors((state) => ({ ...state, friends: messageOf(reason) }))
    } finally {
      setLoading((state) => ({ ...state, friends: false }))
    }
  }, [session.server, session.token, onFriendsChange])

  const loadRequests = useCallback(async () => {
    setLoading((state) => ({ ...state, requests: true }))
    setErrors((state) => ({ ...state, requests: null }))
    try {
      const data = await listFriendRequests(session.server, session.token)
      setIncoming(Array.isArray(data?.incoming) ? data.incoming : [])
      setOutgoing(Array.isArray(data?.outgoing) ? data.outgoing : [])
    } catch (reason) {
      if (!isUnauthorized(reason)) setErrors((state) => ({ ...state, requests: messageOf(reason) }))
    } finally {
      setLoading((state) => ({ ...state, requests: false }))
    }
  }, [session.server, session.token])

  const loadBlocked = useCallback(async () => {
    setLoading((state) => ({ ...state, blocked: true }))
    setErrors((state) => ({ ...state, blocked: null }))
    try {
      const data = await listBlocked(session.server, session.token)
      setBlocked(Array.isArray(data?.blocked) ? data.blocked : [])
    } catch (reason) {
      if (!isUnauthorized(reason)) setErrors((state) => ({ ...state, blocked: messageOf(reason) }))
    } finally {
      setLoading((state) => ({ ...state, blocked: false }))
    }
  }, [session.server, session.token])

  useEffect(() => {
    void loadFriends()
    void loadRequests()
    void loadBlocked()
    const timer = window.setInterval(() => { void loadFriends() }, REFRESH_MS)
    return () => window.clearInterval(timer)
  }, [loadFriends, loadRequests, loadBlocked])

  function changeTab(next: Tab) {
    setTab(next)
    if (next === 'requests') void loadRequests()
    if (next === 'blocked') void loadBlocked()
  }

  function onTabKey(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next = index
    if (event.key === 'ArrowRight') next = (index + 1) % TABS.length
    else if (event.key === 'ArrowLeft') next = (index - 1 + TABS.length) % TABS.length
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = TABS.length - 1
    else return
    event.preventDefault()
    changeTab(TABS[next])
    tabRefs.current[next]?.focus()
  }

  async function runAction(key: string, action: () => Promise<void>, onDone?: () => void) {
    if (busy) return
    setBusy(key)
    setFormError(null)
    try {
      await action()
      onDone?.()
    } catch (reason) {
      if (!isUnauthorized(reason)) {
        const text = requestError(reason)
        setFormError(text)
        onNotice(text)
      }
    } finally {
      setBusy(null)
    }
  }

  function handleSend(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const name = username.trim()
    const note = message.trim()
    if (!name || name.length > 64 || note.length > 280) {
      setFormError('Enter a username of up to 64 characters and a message of up to 280.')
      return
    }
    void runAction('send', async () => {
      await sendFriendRequest(session.server, session.token, name, note)
      setUsername('')
      setMessage('')
      onNotice(`Request sent to ${name}.`)
    }, () => { void loadRequests() })
  }

  function handleInvite(event: FormEvent<HTMLFormElement>, friend: Friend) {
    event.preventDefault()
    const id = String(friend.steam_id)
    const connectStr = (invites[id] ?? '').trim()
    if (!connectStr) return
    void runAction(`invite:${id}`, async () => {
      await inviteFriend(session.server, session.token, id, connectStr)
      onNotice(`Invite sent to ${friend.persona_name}.`)
    })
  }

  function handleConfirmation() {
    if (!confirmation || busy) return
    const { kind, friend } = confirmation
    const id = String(friend.steam_id)
    void runAction(`${kind}:${id}`, async () => {
      if (kind === 'remove') await removeFriend(session.server, session.token, id)
      else await blockFriend(session.server, session.token, id)
      onNotice(kind === 'remove'
        ? `${friend.persona_name} removed from friends.`
        : `${friend.persona_name} blocked.`)
      setConfirmation(null)
    }, () => {
      void loadFriends()
      void loadBlocked()
      void loadRequests()
    })
  }

  async function simpleAction(
    key: string,
    action: () => Promise<unknown>,
    success: string,
    reload: () => void,
  ) {
    await runAction(key, async () => {
      await action()
      onNotice(success)
    }, reload)
  }

  return (
    <section className="view">
      <header className="view__head">
        <div>
          <h1>Friends</h1>
          <p className="muted">People, requests and blocks on this server.</p>
        </div>
      </header>

      <div className="tabs" role="tablist" aria-label="Friends sections">
        {TABS.map((item, index) => (
          <button
            key={item}
            ref={(node) => { tabRefs.current[index] = node }}
            type="button"
            role="tab"
            id={`tab-${item}`}
            aria-selected={tab === item}
            aria-controls={`panel-${item}`}
            tabIndex={tab === item ? 0 : -1}
            className={`tabs__tab${tab === item ? ' is-active' : ''}`}
            onClick={() => changeTab(item)}
            onKeyDown={(event) => onTabKey(event, index)}
          >
            {TAB_LABELS[item]}
          </button>
        ))}
      </div>

      <div
        role="tabpanel"
        id={`panel-${tab}`}
        aria-labelledby={`tab-${tab}`}
        className="tab-panel"
      >
        {tab === 'friends' && (
          <div className="stack">
            <div className="list-tools">
              <button type="button" className="btn btn--quiet" disabled={busy !== null} onClick={() => void loadFriends()}>
                Refresh
              </button>
              <p className="muted">Presence reloads every 15 seconds.</p>
            </div>
            {loading.friends && friends.length === 0 ? <LoadingState label="Loading friends…" /> : null}
            {errors.friends ? (
              <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void loadFriends()}>Retry</button>}>
                {errors.friends}
              </ErrorState>
            ) : null}
            {!loading.friends && !errors.friends && friends.length === 0 ? (
              <EmptyState>No friends yet. Open Requests to send one an invitation.</EmptyState>
            ) : null}
            {friends.length > 0 && (
              <ul className="friend-list">
                {friends.map((friend) => {
                  const id = String(friend.steam_id)
                  const tone = personaTone(friend.persona_state)
                  return (
                    <li className="friend" key={id}>
                      <div className="friend__head">
                        <button type="button" className="friend__name" onClick={() => onSelectFriend(friend)}>
                          {friend.persona_name}
                        </button>
                        <span className="badge badge--state">
                          <span className={`dot dot--${tone}`} aria-hidden="true" />
                          {personaLabel(friend.persona_state)}
                        </span>
                      </div>
                      <p>{gameLabel(friend)}</p>
                      <div className="friend__meta">
                        <span className="mono">ID {friend.steam_id}</span>
                        {friend.lobby_id ? <span className="badge">Lobby {friend.lobby_id}</span> : null}
                      </div>
                      <div className="friend__actions">
                        <button
                          type="button"
                          className="btn btn--quiet"
                          disabled={!friend.lobby_id || busy !== null}
                          onClick={() => onJoinFriend(friend)}
                        >
                          Join
                        </button>
                        <button
                          type="button"
                          className="btn btn--quiet"
                          disabled={busy !== null}
                          onClick={() => setConfirmation({ kind: 'remove', friend })}
                        >
                          Remove
                        </button>
                        <button
                          type="button"
                          className="btn btn--quiet"
                          disabled={busy !== null}
                          onClick={() => setConfirmation({ kind: 'block', friend })}
                        >
                          Block
                        </button>
                      </div>
                      {!friend.lobby_id && friend.connect_str ? (
                        <p className="muted mono">Connect string: {friend.connect_str}</p>
                      ) : null}
                      <form className="inline-form" onSubmit={(event) => handleInvite(event, friend)}>
                        <label className="visually-hidden" htmlFor={`invite-${id}`}>
                          Connect string for {friend.persona_name}
                        </label>
                        <input
                          id={`invite-${id}`}
                          className="mono"
                          type="text"
                          placeholder="Connect string"
                          autoComplete="off"
                          spellCheck={false}
                          value={invites[id] ?? ''}
                          onChange={(event) => setInvites((current) => ({
                            ...current,
                            [id]: event.target.value,
                          }))}
                          required
                        />
                        <button type="submit" className="btn btn--quiet" disabled={busy !== null}>
                          {busy === `invite:${id}` ? 'Sending…' : 'Invite'}
                        </button>
                      </form>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        )}

        {tab === 'requests' && (
          <div className="stack">
            <form className="panel request-form" onSubmit={handleSend}>
              <h2>Add a friend</h2>
              <div className="field">
                <label htmlFor="friend-username">Username</label>
                <input
                  id="friend-username"
                  name="username"
                  type="text"
                  autoComplete="off"
                  autoCapitalize="none"
                  autoCorrect="off"
                  spellCheck={false}
                  maxLength={64}
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  required
                />
              </div>
              <div className="field">
                <label htmlFor="friend-message">Message (optional)</label>
                <input
                  id="friend-message"
                  name="message"
                  type="text"
                  maxLength={280}
                  value={message}
                  onChange={(event) => setMessage(event.target.value)}
                />
              </div>
              <div>
                <button type="submit" className="btn btn--primary" disabled={busy !== null}>
                  {busy === 'send' ? 'Sending…' : 'Send request'}
                </button>
              </div>
              {formError ? <p className="field-error" role="alert">{formError}</p> : null}
            </form>

            {loading.requests && incoming.length === 0 && outgoing.length === 0 ? (
              <LoadingState label="Loading requests…" />
            ) : null}
            {errors.requests ? (
              <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void loadRequests()}>Retry</button>}>
                {errors.requests}
              </ErrorState>
            ) : null}

            <section className="panel" aria-labelledby="incoming-title">
              <h2 id="incoming-title">Incoming</h2>
              {incoming.length === 0 ? <EmptyState>No incoming requests.</EmptyState> : (
                <ul className="friend-list">
                  {incoming.map((item) => {
                    const key = String(item.id)
                    return (
                      <li className="friend" key={key}>
                        <div className="friend__head">
                          <span className="friend__label">{item.persona_name}</span>
                        </div>
                        {item.message ? <p>{item.message}</p> : null}
                        <div className="friend__actions">
                          <button
                            type="button"
                            className="btn btn--primary"
                            disabled={busy !== null}
                            onClick={() => void simpleAction(
                              `accept:${key}`,
                              () => acceptFriendRequest(session.server, session.token, item.id),
                              `${item.persona_name} is now a friend.`,
                              () => { void loadRequests(); void loadFriends() },
                            )}
                          >
                            {busy === `accept:${key}` ? 'Accepting…' : 'Accept'}
                          </button>
                          <button
                            type="button"
                            className="btn btn--quiet"
                            disabled={busy !== null}
                            onClick={() => void simpleAction(
                              `decline:${key}`,
                              () => declineFriendRequest(session.server, session.token, item.id),
                              `Declined the request from ${item.persona_name}.`,
                              () => { void loadRequests() },
                            )}
                          >
                            {busy === `decline:${key}` ? 'Declining…' : 'Decline'}
                          </button>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>

            <section className="panel" aria-labelledby="outgoing-title">
              <h2 id="outgoing-title">Outgoing</h2>
              {outgoing.length === 0 ? <EmptyState>No outgoing requests.</EmptyState> : (
                <ul className="friend-list">
                  {outgoing.map((item) => {
                    const key = String(item.id)
                    return (
                      <li className="friend" key={key}>
                        <div className="friend__head">
                          <span className="friend__label">{item.persona_name}</span>
                          <span className="badge">Pending</span>
                        </div>
                        {item.message ? <p>{item.message}</p> : null}
                        <div className="friend__actions">
                          <button
                            type="button"
                            className="btn btn--quiet"
                            disabled={busy !== null}
                            onClick={() => void simpleAction(
                              `cancel:${key}`,
                              () => cancelFriendRequest(session.server, session.token, item.id),
                              `Request to ${item.persona_name} cancelled.`,
                              () => { void loadRequests() },
                            )}
                          >
                            {busy === `cancel:${key}` ? 'Cancelling…' : 'Cancel'}
                          </button>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>
          </div>
        )}

        {tab === 'blocked' && (
          <div className="stack">
            <div className="list-tools">
              <button type="button" className="btn btn--quiet" disabled={busy !== null} onClick={() => void loadBlocked()}>
                Refresh
              </button>
            </div>
            {loading.blocked && blocked.length === 0 ? <LoadingState label="Loading blocked accounts…" /> : null}
            {errors.blocked ? (
              <ErrorState action={<button type="button" className="btn btn--quiet" onClick={() => void loadBlocked()}>Retry</button>}>
                {errors.blocked}
              </ErrorState>
            ) : null}
            {!loading.blocked && !errors.blocked && blocked.length === 0 ? (
              <EmptyState>No blocked accounts.</EmptyState>
            ) : null}
            {blocked.length > 0 && (
              <ul className="friend-list">
                {blocked.map((user) => {
                  const id = String(user.steam_id)
                  return (
                    <li className="friend" key={id}>
                      <div className="friend__head">
                        <span className="friend__label">{user.persona_name}</span>
                        <span className="mono">ID {user.steam_id}</span>
                      </div>
                      <div className="friend__actions">
                        <button
                          type="button"
                          className="btn btn--quiet"
                          disabled={busy !== null}
                          onClick={() => void simpleAction(
                            `unblock:${id}`,
                            () => unblockFriend(session.server, session.token, id),
                            `${user.persona_name} unblocked.`,
                            () => { void loadBlocked() },
                          )}
                        >
                          {busy === `unblock:${id}` ? 'Unblocking…' : 'Unblock'}
                        </button>
                      </div>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        )}
      </div>

      <ConfirmDialog
        open={confirmation !== null}
        title={confirmation?.kind === 'remove' ? 'Remove friend' : 'Block account'}
        message={confirmation
          ? confirmation.kind === 'remove'
            ? `Remove ${confirmation.friend.persona_name} from your friends? You can send a new request later.`
            : `Block ${confirmation.friend.persona_name}? Presence and invites are hidden both ways until you unblock them.`
          : ''}
        confirmLabel={confirmation?.kind === 'remove' ? 'Remove' : 'Block'}
        busy={busy !== null}
        onConfirm={handleConfirmation}
        onCancel={() => setConfirmation(null)}
      />
    </section>
  )
}
