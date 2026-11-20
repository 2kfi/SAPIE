import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import AddGameModal from '../screens/AddGameModal'
import { mockTauri } from './tauri'

const detected = {
  exe_path: '/games/iw4mp.exe',
  arch: 'x86',
  era: 'old',
  interfaces: ['ISteamUser', 'ISteamFriends'],
  original_library: 'steam_api_orig.dll',
  warning: null,
}

function renderModal(overrides: Record<string, (args?: Record<string, unknown>) => unknown> = {}) {
  const onAdded = vi.fn()
  const onClose = vi.fn()
  const onEvent = vi.fn()
  mockTauri({ detect_game: async () => detected, ...overrides })
  render(
    <AddGameModal
      progress={null}
      onClose={onClose}
      onAdded={onAdded}
      onEvent={onEvent}
    />,
  )
  return { onAdded, onClose, onEvent }
}

function setPath(value: string) {
  fireEvent.change(screen.getByLabelText('Executable path'), { target: { value } })
}

describe('add game modal', () => {
  it('refuses a relative path', async () => {
    renderModal()
    setPath('games/iw4mp.exe')
    fireEvent.click(screen.getByRole('button', { name: 'Inspect' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('absolute path')
    expect(screen.getByRole('button', { name: 'Add to library' })).toBeDisabled()
  })

  it('prefills an app ID only for a known executable', async () => {
    renderModal()
    setPath('/games/iw4mp.exe')
    fireEvent.click(screen.getByRole('button', { name: 'Inspect' }))

    expect(await screen.findByLabelText('App ID')).toHaveValue('10190')
    expect(screen.getByText('2 found')).toBeInTheDocument()
  })

  it('leaves the app ID empty for an unknown executable', async () => {
    mockTauri({
      detect_game: async () => ({ ...detected, exe_path: '/games/other.exe' }),
    })
    render(
      <AddGameModal progress={null} onClose={vi.fn()} onAdded={vi.fn()} onEvent={vi.fn()} />,
    )
    setPath('/games/other.exe')
    fireEvent.click(screen.getByRole('button', { name: 'Inspect' }))

    await waitFor(() => expect(screen.getByLabelText('App ID')).toHaveValue(''))
    expect(screen.getByText(/No known app ID/)).toBeInTheDocument()
  })

  it('rejects an invalid app ID before writing anything', async () => {
    const { invoke } = mockTauri({ detect_game: async () => detected })
    render(
      <AddGameModal progress={null} onClose={vi.fn()} onAdded={vi.fn()} onEvent={vi.fn()} />,
    )
    setPath('/games/iw4mp.exe')
    fireEvent.click(screen.getByRole('button', { name: 'Inspect' }))
    await waitFor(() => expect(screen.getByLabelText('App ID')).toHaveValue('10190'))
    fireEvent.change(screen.getByLabelText('App ID'), { target: { value: 'abc' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('whole app ID')
    expect(invoke).not.toHaveBeenCalledWith('install_library', expect.anything())
  })

  it('requires a fresh inspection when the path changes', async () => {
    renderModal()
    setPath('/games/iw4mp.exe')
    fireEvent.click(screen.getByRole('button', { name: 'Inspect' }))
    await waitFor(() => expect(screen.getByLabelText('App ID')).toHaveValue('10190'))
    setPath('/games/changed.exe')

    expect(screen.getByRole('button', { name: 'Add to library' })).toBeDisabled()
  })

  it('installs and configures in order, then reports the added path', async () => {
    const calls: string[] = []
    const { onAdded } = renderModal({
      library_add: async () => {
        calls.push('library_add')
        return {}
      },
      install_library: async () => {
        calls.push('install_library')
        return {}
      },
      configure_game: async () => {
        calls.push('configure_game')
        return {}
      },
    })
    setPath('/games/iw4mp.exe')
    fireEvent.click(screen.getByRole('button', { name: 'Inspect' }))
    await waitFor(() => expect(screen.getByLabelText('App ID')).toHaveValue('10190'))
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith('/games/iw4mp.exe'))
    expect(calls).toEqual(['library_add', 'install_library', 'configure_game'])
  })
})
