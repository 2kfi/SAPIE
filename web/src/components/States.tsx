import type { ReactNode } from 'react'

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="state">
      <p>{children}</p>
    </div>
  )
}

export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="state">
      <p role="status">{label}</p>
    </div>
  )
}

export function ErrorState({
  children,
  action,
}: {
  children: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="state state--error" role="alert">
      <p>{children}</p>
      {action}
    </div>
  )
}
