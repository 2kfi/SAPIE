import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App from '../App'
import SignIn from '../screens/SignIn'
import { mockTauri } from './tauri'

function fillSignIn() {
  fireEvent.change(screen.getByLabelText('Server'), {
    target: { value: '127.0.0.1:8081' },
  })
  fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'hunter2' } })
}

describe('sign in', () => {
  it('submits typed credentials and clears the password field', () => {
    const onSignIn = vi.fn()
    render(
      <SignIn initialServer="" status="idle" error={null} onSignIn={onSignIn} />,
    )
    fillSignIn()
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(onSignIn).toHaveBeenCalledWith('127.0.0.1:8081', 'alice', 'hunter2')
    expect(screen.getByLabelText('Password')).toHaveValue('')
  })

  it('shows a specific error for rejected credentials and keeps the secret out of the DOM', async () => {
    const { invoke } = mockTauri({
      test_connection: async () => undefined,
      library_list: async () => [],
      login: async () => {
        throw { kind: 'bad_credentials', message: 'Wrong username or password.' }
      },
    })
    render(<App />)
    fillSignIn()
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong username or password.')
    expect(invoke).toHaveBeenCalledWith(
      'login',
      expect.objectContaining({ password: 'hunter2' }),
    )
    expect(document.body.textContent).not.toContain('hunter2')
  })

  it('rejects a bad host without calling login', async () => {
    const { invoke } = mockTauri({ library_list: async () => [] })
    render(<App />)
    fireEvent.change(screen.getByLabelText('Server'), { target: { value: 'localhost' } })
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'hunter2' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('host:port')
    expect(invoke).not.toHaveBeenCalledWith('login', expect.anything())
  })

  it('reports an unreachable server', async () => {
    mockTauri({
      library_list: async () => [],
      test_connection: async () => {
        throw { kind: 'unreachable', message: 'timed out' }
      },
    })
    render(<App />)
    fillSignIn()
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not reach the SAPIE server')
  })

  it('reports an old server', async () => {
    mockTauri({
      library_list: async () => [],
      test_connection: async () => {
        throw { kind: 'server_too_old', message: 'no config route' }
      },
    })
    render(<App />)
    fillSignIn()
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('server is too old')
  })

  it('shows a pending state while checking, then opens the library without rendering the token', async () => {
    mockTauri({
      library_list: async () => [],
      test_connection: async () => undefined,
      login: async () => ({
        token: 'session-token-must-not-render',
        steam_id: 76561198000000000,
        persona_name: 'alice',
        server: '127.0.0.1:8081',
        expires_at: 2_000_000_000,
      }),
    })
    render(<App />)
    fillSignIn()
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(screen.getByRole('button', { name: 'Checking server' })).toBeDisabled()
    expect(await screen.findByRole('button', { name: 'Add a game' })).toBeInTheDocument()
    await waitFor(() => {
      expect(document.body.textContent).not.toContain('session-token-must-not-render')
      expect(document.body.textContent).not.toContain('hunter2')
    })
  })
})
