import type { ReactNode } from 'react'

type IconProps = { className?: string }

function Icon({ className, children }: IconProps & { children: ReactNode }) {
  return (
    <svg
      className={className}
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {children}
    </svg>
  )
}

export function FriendsIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="9" cy="8" r="3" />
      <path d="M3.5 19c.4-3 2.6-4.8 5.5-4.8s5.1 1.8 5.5 4.8" />
      <path d="M16 5.5a3 3 0 0 1 0 5.8M17 14.6c2.2.4 3.5 2 3.8 4.4" />
    </Icon>
  )
}

export function GamesIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <rect x="2.5" y="6.5" width="19" height="11" rx="3" />
      <path d="M6.5 10v4M4.5 12h4M15.5 11h.01M18 13.5h.01" />
    </Icon>
  )
}

export function ProfileIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="8.5" r="3.5" />
      <path d="M5 20c.6-3.4 3.3-5.3 7-5.3s6.4 1.9 7 5.3" />
    </Icon>
  )
}

export function LiveIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M3 12h3.5l2.2-5 3.1 10 2.2-5H21" />
    </Icon>
  )
}

export function LockIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <rect x="5" y="10.5" width="14" height="9.5" rx="2" />
      <path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5" />
    </Icon>
  )
}

export function EyeIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M2.5 12S6 6.5 12 6.5 21.5 12 21.5 12 18 17.5 12 17.5 2.5 12 2.5 12Z" />
      <circle cx="12" cy="12" r="2.5" />
    </Icon>
  )
}

export function EyeOffIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M4 4l16 16" />
      <path d="M9.6 6.9A9.8 9.8 0 0 1 12 6.5c6 0 9.5 5.5 9.5 5.5a17 17 0 0 1-2.7 3.1M6.3 8.4A17 17 0 0 0 2.5 12S6 17.5 12 17.5c.9 0 1.8-.1 2.6-.4" />
      <path d="M10 10a2.5 2.5 0 0 0 3.6 3.5" />
    </Icon>
  )
}
