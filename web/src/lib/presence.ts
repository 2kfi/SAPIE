import type { Friend } from '../api'

const personaLabels: Record<number, string> = {
  0: 'Offline',
  1: 'Online',
  2: 'Busy',
  3: 'Away',
  4: 'Snooze',
  5: 'Looking to trade',
  6: 'Looking to play',
}

export function personaLabel(value: string | number): string {
  const numeric = Number(value)
  if (Number.isInteger(numeric) && numeric in personaLabels) return personaLabels[numeric]
  return String(value)
}

export function personaTone(value: string | number): 'online' | 'away' | 'offline' {
  const numeric = Number(value)
  if (numeric === 1 || numeric === 5 || numeric === 6) return 'online'
  if (numeric === 2 || numeric === 3 || numeric === 4) return 'away'
  const text = String(value).toLowerCase()
  if (text === 'online') return 'online'
  if (text === 'away' || text === 'busy' || text === 'snooze') return 'away'
  return 'offline'
}

function decodePresenceValue(raw: string): string {
  if (typeof atob !== 'function') return raw
  try {
    const bytes = Uint8Array.from(atob(raw), (char) => char.charCodeAt(0))
    return new TextDecoder().decode(bytes)
  } catch {
    return raw
  }
}

function presenceSummary(value: Friend['rich_presence']): string | null {
  if (typeof value === 'string') return value.trim() || null
  if (!value || typeof value !== 'object') return null
  const pairs = Object.entries(value)
    .filter((entry): entry is [string, string] => typeof entry[1] === 'string' && entry[1].length > 0)
    .map(([key, raw]) => `${key}: ${decodePresenceValue(raw)}`)
  return pairs.length > 0 ? pairs.join(' · ') : null
}

export function gameLabel(friend: Friend): string {
  const presence = presenceSummary(friend.rich_presence)
  if (presence) return presence
  if (friend.appid != null && friend.appid !== '') return `App ${friend.appid}`
  return 'Not in a game'
}
