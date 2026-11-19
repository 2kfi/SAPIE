import { useState, type FormEvent } from 'react'
import { ApiError, login, messageOf, type Session } from '../api'
import { EyeIcon, EyeOffIcon } from '../components/Icons'

type Props = {
  onConnected: (session: Session) => void
}

function validateServer(value: string): string | null {
  const server = value.trim()
  const separator = server.indexOf(':')
  if (separator < 1 || server.indexOf(':', separator + 1) !== -1) {
    return 'Enter the server as host:port (for example 127.0.0.1:7777).'
  }
  const port = Number(server.slice(separator + 1))
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return 'Port must be a number between 1 and 65535.'
  }
  return null
}

export default function Login({ onConnected }: Props) {
  const [showPassword, setShowPassword] = useState(false)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(null)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    const form = new FormData(event.currentTarget)
    const server = String(form.get('server') ?? '').trim()
    const username = String(form.get('username') ?? '').trim()
    const password = String(form.get('password') ?? '')

    const invalid = validateServer(server)
    if (invalid) {
      setNotice({ text: invalid, error: true })
      return
    }
    if (!username) {
      setNotice({ text: 'Username is required.', error: true })
      return
    }
    if (!password) {
      setNotice({ text: 'Password is required.', error: true })
      return
    }

    setBusy(true)
    setNotice(null)
    try {
      const result = await login(server, username, password)
      onConnected({ server, username, password, ...result })
    } catch (reason) {
      const text = reason instanceof ApiError && reason.status === 401
        ? 'Username or password is incorrect.'
        : messageOf(reason)
      setNotice({ text, error: true })
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="login-title">
        <div className="login-card__head">
          <span className="mark" aria-hidden="true">
            S
          </span>
          <div>
            <h1 id="login-title">Steam API Emulator</h1>
            <p className="muted">Sign in to a SAPIE server to continue.</p>
          </div>
        </div>

        <form onSubmit={(event) => void handleSubmit(event)}>
          <div className="field">
            <label htmlFor="server">Server address</label>
            <input
              id="server"
              name="server"
              type="text"
              inputMode="url"
              autoComplete="off"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              placeholder="host:port"
              required
            />
          </div>

          <div className="field">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              name="username"
              type="text"
              autoComplete="username"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              required
            />
          </div>

          <div className="field">
            <label htmlFor="password">Password</label>
            <div className="password-field">
              <input
                id="password"
                name="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                required
              />
              <button
                type="button"
                className="icon-button"
                aria-pressed={showPassword}
                aria-label={showPassword ? 'Hide password' : 'Show password'}
                onClick={() => setShowPassword((shown) => !shown)}
              >
                {showPassword ? <EyeOffIcon /> : <EyeIcon />}
              </button>
            </div>
          </div>

          <button type="submit" className="btn btn--primary btn--wide" disabled={busy}>
            {busy ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <div className="login-card__foot">
          <p role={notice?.error ? 'alert' : 'status'}>{notice?.text}</p>
          <p className="muted">The session token stays in memory for this tab.</p>
        </div>
      </section>
    </main>
  )
}
