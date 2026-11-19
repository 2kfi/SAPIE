import { useCallback, useEffect, useState } from 'react'
import Login from './screens/Login'
import Shell from './screens/Shell'
import Toast from './components/Toast'
import { navigate, readRoute, type Route } from './routes'
import { readPreviewState } from './state'

// shortcut: sign-in is stubbed in M0, so the shell previews the signed-in user's account.

export default function App() {
  const [route, setRoute] = useState<Route>(readRoute)
  const [notice, setNotice] = useState<string | null>(null)
  const previewState = readPreviewState()

  useEffect(() => {
    const syncRoute = () => setRoute(readRoute())
    window.addEventListener('hashchange', syncRoute)
    return () => window.removeEventListener('hashchange', syncRoute)
  }, [])

  const go = useCallback((next: Route) => {
    setNotice(null)
    navigate(next)
  }, [])

  return (
    <>
      {route === 'login' ? (
        <Login onPreview={() => go('friends')} />
      ) : (
        <Shell
          route={route}
          state={previewState}
          onNavigate={go}
          onNotice={setNotice}
        />
      )}
      <Toast message={notice} onDismiss={() => setNotice(null)} />
    </>
  )
}
