import { LockIcon } from '../components/Icons'

export default function GameDetail() {
  return (
    <section className="view view--detail" aria-labelledby="game-detail-title">
      <header className="view__head">
        <div>
          <h2 id="game-detail-title">Game details</h2>
          <p>Game setup runs in the native launcher.</p>
        </div>
      </header>

      <div className="panel panel--native">
        <h3>Managed by native launcher</h3>
        <p className="muted">
          The launcher chooses the executable, installs the library, writes the connection files and
          starts the game.
        </p>
        <p className="muted">
          <LockIcon className="inline-icon" /> This pane cannot change game files.
        </p>
      </div>
    </section>
  )
}
