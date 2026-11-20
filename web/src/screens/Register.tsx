import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, getConfig, login, messageOf, register, type Session } from '../api'
import { EyeIcon, EyeOffIcon } from '../components/Icons'

type Props = {
  onConnected: (session: Session) => void
  onBack: () => void
}

type FieldErrors = { username?: string; password?: string }

function currentServer(): string {
  return window.location.host
}

export default function Register({ onConnected, onBack }: Props) {
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
    const personaName = String(values.get('persona_name') ?? '').trim()

    const nextErrors: FieldErrors = {}
    if (!username) nextErrors.username = 'Enter a username.'
    if (!password) nextErrors.password = 'Enter a password.'
    setErrors(nextErrors)
    if (nextErrors.username || nextErrors.password) {
      setNotice({ text: 'Check the highlighted fields.', error: true })
      document.getElementById(nextErrors.username ? 'register-username' : 'register-password')?.focus()
      return
    }

    setBusy(true)
    setNotice(null)
    let signedIn = false
    try {
      await register(currentServer(), username, password, personaName)
      const result = await login(currentServer(), username, password)
      signedIn = true
      setNotice({ text: `Account created. Signed in as ${username}.`, error: false })
      window.setTimeout(() => onConnected({
        server: currentServer(),
        username,
        password,
        ...result,
      }), 400)
    } catch (reason) {
      const text = reason instanceof ApiError && reason.status === 403
        ? 'Registration is disabled on this server.'
        : reason instanceof ApiError && reason.status === 409
          ? 'That username is already taken.'
          : messageOf(reason)
      setNotice({ text, error: true })
    } finally {
      if (!signedIn) setBusy(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="register-title">
        <div className="login-card__head">
          <span className="mark" aria-hidden="true">
            S
          </span>
          <div>
            <h1 id="register-title">Create an account</h1>
            <p className="muted">Choose a username and password for this server.</p>
          </div>
        </div>

        {config.allowed === true ? (
          <form onSubmit={(event) => void handleSubmit(event)} noValidate>
            <div className="field">
              <label htmlFor="register-username">Username</label>
              <input
                id="register-username"
                name="username"
                type="text"
                autoComplete="username"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                aria-invalid={Boolean(errors.username)}
                aria-describedby={errors.username ? 'register-username-error' : undefined}
              />
              {errors.username && (
                <p id="register-username-error" className="field-error" role="alert">{errors.username}</p>
              )}
            </div>

            <div className="field">
              <label htmlFor="register-persona">Persona name (optional)</label>
              <input
                id="register-persona"
                name="persona_name"
                type="text"
                autoComplete="nickname"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
              />
            </div>

            <div className="field">
              <label htmlFor="register-password">Password</label>
              <div className="password-field">
                <input
                  id="register-password"
                  name="password"
                  type={showPassword ? 'text' : 'password'}
                  autoComplete="new-password"
                  aria-invalid={Boolean(errors.password)}
                  aria-describedby={errors.password ? 'register-password-error' : undefined}
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
                <p id="register-password-error" className="field-error" role="alert">{errors.password}</p>
              )}
            </div>

            <button type="submit" className="btn btn--primary btn--wide" disabled={busy}>
              {busy ? 'Creating account…' : 'Create account'}
            </button>
          </form>
        ) : (
          <p className="muted" role={config.error ? 'alert' : 'status'}>
            {config.error
              ? `Registration availability could not be checked: ${config.error}`
              : config.allowed === null
                ? 'Checking registration availability…'
                : 'Registration is disabled on this server.'}
          </p>
        )}

        <div className="login-card__foot">
          <button type="button" className="link-button" disabled={busy} onClick={onBack}>
            Back to sign in
          </button>
          <p role={notice?.error ? 'alert' : 'status'}>{notice?.text}</p>
          <p className="muted">The session token stays in memory for this tab.</p>
        </div>
      </section>
    </main>
  )
}
