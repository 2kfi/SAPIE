import { useState, type FormEvent } from 'react'

export type SignInStatus = 'idle' | 'checking' | 'signing_in' | 'error'

type Props = {
  initialServer: string
  status: SignInStatus
  error: string | null
  onSignIn: (server: string, username: string, password: string) => void
}

export default function SignIn({ initialServer, status, error, onSignIn }: Props) {
  const [server, setServer] = useState(initialServer)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const busy = status === 'checking' || status === 'signing_in'

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    onSignIn(server.trim(), username.trim(), password)
    setPassword('')
  }

  return (
    <section className="signin" aria-labelledby="signin-title">
      <div className="signin__panel">
        <div className="signin__mark" aria-hidden="true">
          S
        </div>
        <h1 id="signin-title">Sign in to SAPIE</h1>
        <p className="signin__intro">
          Connect to your server on this network. The session token goes into the OS keychain.
          This window never keeps the password.
        </p>
        <form className="form" onSubmit={submit}>
          <div className="field">
            <label htmlFor="signin-server">Server</label>
            <input
              id="signin-server"
              value={server}
              onChange={(event) => setServer(event.target.value)}
              placeholder="192.168.1.10:8081"
              autoComplete="url"
              disabled={busy}
              required
            />
            <p className="field__hint">Host and port, for example 192.168.1.10:8081.</p>
          </div>
          <div className="field">
            <label htmlFor="signin-username">Username</label>
            <input
              id="signin-username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              disabled={busy}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="signin-password">Password</label>
            <input
              id="signin-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              disabled={busy}
              required
            />
          </div>
          {error && (
            <p className="form__error" role="alert">
              {error}
            </p>
          )}
          {busy && (
            <p className="form__status" role="status" aria-live="polite">
              {status === 'checking' ? 'Checking the server.' : 'Signing in.'}
            </p>
          )}
          <button
            type="submit"
            className="btn btn--primary"
            disabled={busy}
            aria-busy={busy}
          >
            {status === 'checking'
              ? 'Checking server'
              : status === 'signing_in'
                ? 'Signing in'
                : 'Sign in'}
          </button>
        </form>
      </div>
    </section>
  )
}
