type Props = {
  message: string | null
  onDismiss: () => void
}

export default function Toast({ message, onDismiss }: Props) {
  return (
    <div className="toast-region" role="region" aria-label="Notifications" aria-live="polite">
      {message && (
        <div className="toast">
          <p>{message}</p>
          <button type="button" onClick={onDismiss}>
            Dismiss
          </button>
        </div>
      )}
    </div>
  )
}
