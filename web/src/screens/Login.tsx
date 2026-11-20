import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, getConfig, login, messageOf, type Session } from '../api'
import { EyeIcon, EyeOffIcon } from '../components/Icons'
import type { Route } from '../routes'

type Props = {
  onConnected: (session: Session) => void
  onNavigate: (route: Route) => void
}

type FieldErrors = { username?: string; password?: string }

function currentServer(): string {
  return window.location.host
}

export default function Login({ onConnected, onNavigate }: Props) {
  const [showPassword, setShowPassword] = useState(false)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(null)
  const [errors, setErrors] = useState<FieldErrors>({})
  const [config, setConfig] = useState<{ allowed: boolean | null; error: string | null }>({
    allowed: null,
    error: null,
  })

  useEffect(() => {
    let cancelled = false
    getConfig(currentServer())
      .then((data) => {
        if (!cancelled) setConfig({ allowed: data?.allow_register === true, error: null })
      })
      .catch((reason) => {
        if (!cancelled) setConfig({ allowed: null, error: messageOf(reason) })
      })
    return () => { cancelled = true }
  }, [])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    const values = new FormData(event.currentTarget)
    const username = String(values.get('username') ?? '').trim()
    const password = String(values.get('password') ?? '')

    const nextErrors: FieldErrors = {}
    if (!username) nextErrors.username = 'Enter your username.'
    if (!password) nextErrors.password = 'Enter your password.'
    setErrors(nextErrors)
    if (nextErrors.username || nextErrors.password) {
      setNotice({ text: 'Check the highlighted fields.', error: true })
      document.getElementById(nextErrors.username ? 'username' : 'password')?.focus()
      return
    }

    setBusy(true)
    setNotice(null)
    let signedIn = false
    try {
      const result = await login(currentServer(), username, password)
      signedIn = true
      setNotice({ text: `Signed in as ${username}.`, error: false })
      window.setTimeout(() => onConnected({
        server: currentServer(),
        username,
        password,
        ...result,
      }), 400)
    } catch (reason) {
      const text = reason instanceof ApiError && reason.status === 401
        ? 'Username or password is incorrect.'
        : messageOf(reason)
      setNotice({ text, error: true })
    } finally {
      if (!signedIn) setBusy(false)
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
            <p className="muted">Sign in to continue.</p>
          </div>
        </div>

        <form onSubmit={(event) => void handleSubmit(event)} noValidate>
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
              aria-invalid={Boolean(errors.username)}
              aria-describedby={errors.username ? 'username-error' : undefined}
            />
            {errors.username && (
              <p id="username-error" className="field-error" role="alert">{errors.username}</p>
            )}
          </div>

          <div className="field">
            <label htmlFor="password">Password</label>
            <div className="password-field">
              <input
                id="password"
                name="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                aria-invalid={Boolean(errors.password)}
                aria-describedby={errors.password ? 'password-error' : undefined}
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
            {errors.password && (
              <p id="password-error" className="field-error" role="alert">{errors.password}</p>
            )}
          </div>

          <button type="submit" className="btn btn--primary btn--wide" disabled={busy}>
            {busy ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <div className="login-card__foot">
          {config.error ? (
            <p className="field-error" role="alert">
              Registration availability could not be checked: {config.error}
            </p>
          ) : config.allowed === null ? (
            <p className="muted" role="status">Checking registration availability…</p>
          ) : config.allowed ? (
            <button type="button" className="link-button" disabled={busy} onClick={() => onNavigate('register')}>
              Create an account
            </button>
          ) : (
            <p className="muted">Registration is disabled on this server.</p>
          )}
          <p role={notice?.error ? 'alert' : 'status'}>{notice?.text}</p>
          <p className="muted">The session token stays in memory for this tab.</p>
        </div>
      </section>
    </main>
  )
}
