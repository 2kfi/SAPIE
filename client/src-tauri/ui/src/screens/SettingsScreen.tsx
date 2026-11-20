type SessionView = {
  personaName: string
  steamId: number
  server: string
}

type Props = {
  session: SessionView | null
  cachedServer: string
  onSignIn: () => void
  onSignOut: () => void
}

export default function SettingsScreen({ session, cachedServer, onSignIn, onSignOut }: Props) {
  return (
    <section className="settings" aria-label="Settings">
      <div className="settings__group">
        <h2>Account</h2>
        {session ? (
          <dl className="detail__meta">
            <dt>Name</dt>
            <dd>{session.personaName}</dd>
            <dt>Steam ID</dt>
            <dd className="mono">{session.steamId}</dd>
            <dt>Server</dt>
            <dd className="mono">{session.server}</dd>
          </dl>
        ) : (
          <div className="state state--compact">
            <p className="state__title">No account in this window.</p>
            <p>The session token may still be in the OS keychain. Sign in to use it.</p>
            <button type="button" className="btn btn--primary" onClick={onSignIn}>
              Sign in
            </button>
          </div>
        )}
        {session && (
          <button type="button" className="btn" onClick={onSignOut}>
            Sign out and revoke local token
          </button>
        )}
      </div>

      <div className="settings__group">
        <h2>Server</h2>
        <dl className="detail__meta">
          <dt>Last used</dt>
          <dd className="mono">{cachedServer || 'None saved'}</dd>
        </dl>
      </div>

      <div className="settings__group">
        <h2>Launcher</h2>
        <dl className="detail__meta">
          <dt>Build version</dt>
          <dd>Not stamped in this build</dd>
          <dt>Theme</dt>
          <dd>Dark only</dd>
          <dt>Log location</dt>
          <dd>Activity is kept in this window. No log file is written.</dd>
        </dl>
      </div>
    </section>
  )
}
