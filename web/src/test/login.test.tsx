import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import Login from '../screens/Login'
import type { Session } from '../api'
import { bodyOf, mockFetch } from './fetch'

function renderLogin(reason?: string) {
  const onConnected = vi.fn<(session: Session) => void>()
  const onNavigate = vi.fn()
  render(<Login onConnected={onConnected} onNavigate={onNavigate} reason={reason} />)
  return { onConnected, onNavigate }
}

function fillAndSubmit() {
  fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'pw' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
}

describe('sign in screen', () => {
  it('uses the SAPIE name and shows a supplied reason', async () => {
    mockFetch(() => ({ json: { allow_register: true } }))
    renderLogin('Your session has expired. Sign in again.')
    expect(screen.getByRole('heading', { name: 'SAPIE' })).toBeInTheDocument()
    expect(screen.getByText('Your session has expired. Sign in again.')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Create an account' })).toBeInTheDocument()
    expect(screen.queryByText(/Steam API Emulator/)).not.toBeInTheDocument()
  })

  it('signs in with a session that carries no password', async () => {
    mockFetch((url) => {
      if (url.endsWith('/config')) return { json: { allow_register: false } }
      return { json: { session_token: 'tok', steam_id: 99, persona_name: 'Alice' } }
    })
    const { onConnected } = renderLogin()
    await waitFor(() => expect(screen.getByText('Registration is disabled on this server.')).toBeInTheDocument())
    fillAndSubmit()
    await waitFor(() => expect(onConnected).toHaveBeenCalledTimes(1))
    const session = onConnected.mock.calls[0][0]
    expect(session).toMatchObject({
      server: window.location.host,
      username: 'alice',
      token: 'tok',
      steamId: '99',
      personaName: 'Alice',
    })
    expect(Object.keys(session)).not.toContain('password')
    expect(localStorage.length).toBe(0)
  })

  it('sends the password in the login body and nowhere else', async () => {
    const fetchMock = mockFetch(() => ({
      json: { session_token: 'tok', steam_id: 99, persona_name: 'Alice' },
    }))
    renderLogin()
    fillAndSubmit()
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).endsWith('/login'))
    expect(loginCall).toBeDefined()
    expect(String(loginCall?.[0])).not.toContain('pw')
    expect(bodyOf(loginCall?.[1])).toEqual({ username: 'alice', password: 'pw' })
  })

  it('shows a specific message for wrong credentials', async () => {
    mockFetch((url) => {
      if (url.endsWith('/config')) return { json: { allow_register: false } }
      return { status: 401, text: 'unauthorized' }
    })
    renderLogin()
    fillAndSubmit()
    await waitFor(() => {
      expect(screen.getByText('Username or password is incorrect.')).toBeInTheDocument()
    })
  })

  it('blocks an empty submit and focuses the first invalid field', async () => {
    mockFetch(() => ({ json: { allow_register: false } }))
    renderLogin()
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByText('Enter your username.')).toBeInTheDocument()
    expect(screen.getByLabelText('Username')).toHaveFocus()
  })
})
