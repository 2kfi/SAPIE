import { useCallback, useEffect, useState } from 'react'
import Login from './screens/Login'
import Shell from './screens/Shell'
import Toast from './components/Toast'
import { navigate, readRoute, type Route } from './routes'
import { readPreviewState } from './state'
import type { Connection } from './tauri'

export default function App() {
  const [route, setRoute] = useState<Route>(readRoute)
  const [notice, setNotice] = useState<string | null>(null)
  const [connection, setConnection] = useState<Connection | null>(null)
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
        <Login onConnect={setConnection} onPreview={() => go('friends')} />
      ) : (
        <Shell
          route={route}
          state={previewState}
          connection={connection}
          onNavigate={go}
          onNotice={setNotice}
        />
      )}
      <Toast message={notice} onDismiss={() => setNotice(null)} />
    </>
  )
}
