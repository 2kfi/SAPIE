import { useCallback, useEffect, useState } from 'react'
import Login from './screens/Login'
import Register from './screens/Register'
import Shell from './screens/Shell'
import Toast from './components/Toast'
import { navigate, readRoute, type Route } from './routes'
import type { Session } from './api'

export default function App() {
  const [route, setRoute] = useState<Route>(readRoute)
  const [notice, setNotice] = useState<string | null>(null)
  const [session, setSession] = useState<Session | null>(null)

  useEffect(() => {
    const syncRoute = () => setRoute(readRoute())
    window.addEventListener('hashchange', syncRoute)
    return () => window.removeEventListener('hashchange', syncRoute)
  }, [])

  const go = useCallback((next: Route) => {
    setNotice(null)
    navigate(next)
  }, [])

  const connected = useCallback((next: Session) => {
    setSession(next)
    go('friends')
  }, [go])

  return (
    <>
      {route === 'login' ? (
        <Login onConnected={connected} onNavigate={go} />
      ) : route === 'register' ? (
        <Register onConnected={connected} onBack={() => go('login')} />
      ) : (
        <Shell
          route={route}
          session={session}
          onNavigate={go}
          onNotice={setNotice}
        />
      )}
      <Toast message={notice} onDismiss={() => setNotice(null)} />
    </>
  )
}
