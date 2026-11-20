import type { GameRecord } from './api'

export type GameFilter = 'all' | 'installed' | 'not-installed'
export type GameSort = 'name' | 'last_played'

// Mirrors the only executable names the Rust layer recognises for an app ID.
const KNOWN_APPIDS: Record<string, number> = {
  'iw4mp.exe': 10190,
  'iw4sp.exe': 10190,
}

export function executableName(path: string): string {
  const name = path.split(/[\\/]/).pop() ?? path
  return name.replace(/\.[^.]+$/, '')
}

export function knownAppid(path: string): number | null {
  const name = path.split(/[\\/]/).pop()?.toLowerCase() ?? ''
  return KNOWN_APPIDS[name] ?? null
}

export function isInstalled(game: GameRecord): boolean {
  return game.library !== null
}

export function filterGames(
  games: GameRecord[],
  query: string,
  filter: GameFilter,
  sort: GameSort,
): GameRecord[] {
  const needle = query.trim().toLowerCase()
  return games
    .filter((game) => {
      if (filter === 'installed' && !isInstalled(game)) return false
      if (filter === 'not-installed' && isInstalled(game)) return false
      if (!needle) return true
      return (
        executableName(game.exe_path).toLowerCase().includes(needle) ||
        game.exe_path.toLowerCase().includes(needle) ||
        String(game.appid ?? '').includes(needle)
      )
    })
    .sort((a, b) => {
      if (sort === 'last_played') {
        return (b.last_played ?? 0) - (a.last_played ?? 0)
      }
      return executableName(a.exe_path).localeCompare(executableName(b.exe_path))
    })
}
