import { useState, type FormEvent } from 'react'
import { EyeIcon, EyeOffIcon } from '../components/Icons'

type Props = {
  onPreview: () => void
}

export default function Login({ onPreview }: Props) {
  const [showPassword, setShowPassword] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setNotice('No server is connected in this scaffold. Login will be wired up with the Go server.')
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
            <p className="muted">Connect to a SAPIE server to continue.</p>
          </div>
        </div>

        <form onSubmit={handleSubmit}>
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

          <div className="checkbox-field">
            <input id="remember" name="remember" type="checkbox" />
            <label htmlFor="remember">Remember this server</label>
          </div>

          <button type="submit" className="btn btn--primary btn--wide">
            Connect
          </button>
        </form>

        <div className="login-card__foot">
          <p role="status">{notice}</p>
          <button type="button" className="link-button" onClick={onPreview}>
            Open UI preview
          </button>
          <p className="muted">Preview skips sign-in and opens the screens without a server.</p>
        </div>
      </section>
    </main>
  )
}
