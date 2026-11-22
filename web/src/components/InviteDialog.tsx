import { useEffect, useRef, useState } from 'react'
import { ApiError, joinLobby, messageOf, type Session } from '../api'
import { parseInviteLink } from '../lib/invite'

type Props = {
  invite: string
  session: Session
  onClose: () => void
  onJoined: (lobbyId: string) => void
}

function inviteError(reason: unknown): string {
  if (reason instanceof ApiError) {
    if (reason.status === 410) return 'This invite link has expired. Ask for a new link.'
    if (reason.status === 400) {
      return reason.message.toLowerCase().includes('different lobby')
        ? 'This invite is for a different lobby. Nothing was joined.'
        : 'This invite link is not valid. Ask for a new link.'
    }
    if (reason.status === 403) return 'You cannot join this lobby.'
    if (reason.status === 404) return 'That lobby no longer exists. Ask for a new link.'
    if (reason.status === 409) return 'That lobby is not available right now.'
  }
  return messageOf(reason)
}

export default function InviteDialog({ invite, session, onClose, onJoined }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const joinRef = useRef<HTMLButtonElement>(null)
  const returnFocus = useRef<HTMLElement | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const parsed = parseInviteLink(invite)

  useEffect(() => {
    returnFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const dialog = ref.current
    if (dialog && !dialog.open) {
      try {
        dialog.showModal()
      } catch {
        dialog.setAttribute('open', '')
      }
    }
    if (parsed) joinRef.current?.focus()
    else closeRef.current?.focus()
    return () => {
      if (returnFocus.current?.isConnected) returnFocus.current.focus()
    }
  }, [])

  async function handleJoin() {
    if (!parsed || busy) return
    setBusy(true)
    setError(null)
    try {
      await joinLobby(session.server, session.token, parsed.lobbyId, parsed.url)
      onJoined(parsed.lobbyId)
    } catch (reason) {
      setError(inviteError(reason))
    } finally {
      setBusy(false)
    }
  }

  return (
    <dialog
      ref={ref}
      className="confirm"
      aria-labelledby="invite-title"
      aria-describedby="invite-description"
      onCancel={(event) => {
        event.preventDefault()
        onClose()
      }}
    >
      <h2 id="invite-title">{parsed ? `Join lobby ${parsed.lobbyId}` : 'Invite link not valid'}</h2>
      <p id="invite-description">
        {parsed
          ? `This link points to lobby ${parsed.lobbyId}. Joining checks the link with your server.`
          : 'This invite link is not valid. Ask for a new link.'}
      </p>
      {error ? <p className="field-error" role="alert">{error}</p> : null}
      <div className="confirm__actions">
        <button
          ref={closeRef}
          type="button"
          className="btn btn--quiet"
          disabled={busy}
          onClick={onClose}
        >
          Not now
        </button>
        {parsed ? (
          <button
            ref={joinRef}
            type="button"
            className="btn btn--primary"
            disabled={busy}
            onClick={() => void handleJoin()}
          >
            {busy ? 'Joining…' : `Join lobby ${parsed.lobbyId}`}
          </button>
        ) : null}
      </div>
    </dialog>
  )
}
