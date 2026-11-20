import { useCallback, useEffect, useRef, useState } from 'react'
import Login from './screens/Login'
import Register from './screens/Register'
import Shell from './screens/Shell'
import Toast from './components/Toast'
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
  const sessionRef = useRef<Session | null>(null)

  useEffect(() => {
    const syncRoute = () => setRoute(readRoute())
    window.addEventListener('hashchange', syncRoute)
    return () => window.removeEventListener('hashchange', syncRoute)
  }, [])

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
    go('friends')
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
      {route === 'login' ? (
        <Login onConnected={connected} onNavigate={go} reason={loginReason} />
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
    </>
  )
}
