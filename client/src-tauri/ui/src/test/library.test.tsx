import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { executableName, filterGames, knownAppid } from '../library'
import LibraryScreen from '../screens/LibraryScreen'
import { game } from './tauri'

const games = [
  game('/games/Zeta/zeta.exe', {
    library: 'steam_api.dll',
    configured: true,
    appid: 10190,
    last_played: 100,
  }),
  game('/games/alpha/alpha.exe', { last_played: 300 }),
  game('/games/Beta/iw4mp.exe', { last_played: 200 }),
]

describe('library helpers', () => {
  it('takes the display name from the executable path', () => {
    expect(executableName('/games/My Game\\game.exe')).toBe('game')
    expect(knownAppid('/games/iw4mp.exe')).toBe(10190)
    expect(knownAppid('/games/unknown.exe')).toBeNull()
  })

  it('filters by text and install state without mutating the source', () => {
    expect(filterGames(games, 'beta', 'all', 'name').map((item) => item.exe_path)).toEqual([
      '/games/Beta/iw4mp.exe',
    ])
    expect(filterGames(games, '', 'installed', 'name')).toHaveLength(1)
    expect(filterGames(games, '', 'not-installed', 'name')).toHaveLength(2)
    expect(games[0].exe_path).toBe('/games/Zeta/zeta.exe')
  })

  it('sorts by name or last played', () => {
    expect(filterGames(games, '', 'all', 'name').map((item) => item.exe_path)).toEqual([
      '/games/alpha/alpha.exe',
      '/games/Beta/iw4mp.exe',
      '/games/Zeta/zeta.exe',
    ])
    expect(filterGames(games, '', 'all', 'last_played').map((item) => item.exe_path)).toEqual([
      '/games/alpha/alpha.exe',
      '/games/Beta/iw4mp.exe',
      '/games/Zeta/zeta.exe',
    ])
  })
})

describe('library screen states', () => {
  const props = {
    games,
    status: 'ready' as const,
    error: null,
    installingPath: null,
    progress: null,
    onRetry: vi.fn(),
    onAdd: vi.fn(),
    onSelect: vi.fn(),
    onPlay: vi.fn(),
  }

  it('renders an empty state with one add action', () => {
    render(<LibraryScreen {...props} games={[]} />)
    expect(screen.getByText('No games in the library yet.')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Add a game' })).toHaveLength(1)
  })

  it('renders a loading and an error state', () => {
    const { rerender } = render(<LibraryScreen {...props} status="loading" />)
    expect(screen.getByRole('status')).toHaveTextContent('Reading the local library.')

    rerender(<LibraryScreen {...props} status="error" error="Launcher state is locked" />)
    expect(screen.getByRole('alert')).toHaveTextContent('Launcher state is locked')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(props.onRetry).toHaveBeenCalled()
  })
})
