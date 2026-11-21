import { useEffect, useRef } from 'react'

type Props = {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  busy = false,
  onConfirm,
  onCancel,
}: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const returnFocus = useRef<HTMLElement | null>(null)

  useEffect(() => {
    const dialog = ref.current
    if (!open) {
      if (returnFocus.current?.isConnected) returnFocus.current.focus()
      returnFocus.current = null
      return
    }
    returnFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    if (!dialog) return
    if (!dialog.open) {
      try {
        dialog.showModal()
      } catch {
        dialog.setAttribute('open', '')
      }
    }
  }, [open])

  if (!open) return null

  return (
    <dialog
      ref={ref}
      className="confirm"
      aria-labelledby="confirm-title"
      aria-describedby="confirm-message"
      onCancel={(event) => {
        event.preventDefault()
        onCancel()
      }}
    >
      <h2 id="confirm-title">{title}</h2>
      <p id="confirm-message">{message}</p>
      <div className="confirm__actions">
        <button type="button" className="btn btn--quiet" disabled={busy} onClick={onCancel}>
          Cancel
        </button>
        <button type="button" className="btn btn--danger" disabled={busy} onClick={onConfirm}>
          {busy ? 'Working…' : confirmLabel}
        </button>
      </div>
    </dialog>
  )
}
