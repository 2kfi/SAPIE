import { useCallback, useEffect, useRef, useState } from 'react'
import Login from './screens/Login'
import Register from './screens/Register'
import Shell from './screens/Shell'
import InviteDialog from './components/InviteDialog'
import PlainHttpNotice from './components/PlainHttpNotice'
import Toast from './components/Toast'
import { clearInviteParam, readInviteParam } from './lib/invite'
import { navigate, readRoute, type Route } from './routes'
import {
  logout,
  SESSION_EXPIRED_MESSAGE,
  setUnauthorizedHandler,
  type Session,
} from './api'

export default function App() {
  const [route, setRoute] = useState<Route>(readRoute)
  const [notice, setNotice] = useState<string | null>(null)
  const [loginReason, setLoginReason] = useState<string | null>(null)
  const [session, setSession] = useState<Session | null>(null)
  const [invite, setInvite] = useState<string | null>(readInviteParam)
  const sessionRef = useRef<Session | null>(null)
  const inviteRef = useRef(invite)

  useEffect(() => {
    const syncRoute = () => {
      setRoute(readRoute())
      setInvite(readInviteParam())
    }
    window.addEventListener('hashchange', syncRoute)
    window.addEventListener('popstate', syncRoute)
    return () => {
      window.removeEventListener('hashchange', syncRoute)
      window.removeEventListener('popstate', syncRoute)
    }
  }, [])

  useEffect(() => {
    inviteRef.current = invite
  }, [invite])

  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (!sessionRef.current) return
      sessionRef.current = null
      setSession(null)
      setNotice(null)
      setLoginReason(SESSION_EXPIRED_MESSAGE)
      setRoute('login')
      navigate('login')
    })
    return () => setUnauthorizedHandler(null)
  }, [])

  const go = useCallback((next: Route) => {
    setNotice(null)
    setLoginReason(null)
    navigate(next)
    setRoute(next)
  }, [])

  const connected = useCallback((next: Session) => {
    sessionRef.current = next
    setSession(next)
    setLoginReason(null)
    if (!inviteRef.current) go('friends')
  }, [go])

  const closeInvite = useCallback(() => {
    clearInviteParam()
    inviteRef.current = null
    setInvite(null)
    if (route === 'login' || route === 'register') go('friends')
  }, [go, route])

  const joinedInvite = useCallback((lobbyId: string) => {
    clearInviteParam()
    inviteRef.current = null
    setInvite(null)
    go('friends')
    setNotice(`Joined lobby ${lobbyId}.`)
  }, [go])

  const signOut = useCallback(() => {
    const current = sessionRef.current
    sessionRef.current = null
    setSession(null)
    setNotice(null)
    setLoginReason('You have signed out.')
    setRoute('login')
    navigate('login')
    if (current) {
      void logout(current.server, current.token).catch(() => {
        // Local sign out stands; the token is already discarded.
      })
    }
  }, [])

  return (
    <>
      <PlainHttpNotice />
      {route === 'login' ? (
        <Login
          onConnected={connected}
          onNavigate={go}
          reason={loginReason ?? (invite ? 'Sign in to review your lobby invite.' : null)}
        />
      ) : route === 'register' ? (
        <Register onConnected={connected} onBack={() => go('login')} />
      ) : (
        <Shell
          route={route}
          session={session}
          onNavigate={go}
          onNotice={setNotice}
          onSignOut={signOut}
        />
      )}
      <Toast message={notice} onDismiss={() => setNotice(null)} />
      {invite && session ? (
        <InviteDialog
          invite={invite}
          session={session}
          onClose={closeInvite}
          onJoined={joinedInvite}
        />
      ) : null}
    </>
  )
}
