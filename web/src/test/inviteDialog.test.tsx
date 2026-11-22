import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import InviteDialog from '../components/InviteDialog'
import type { Session } from '../api'
import { bodyOf, mockFetch } from './fetch'

const LINK = 'sapie://join/42?exp=1795199695&sig=abc'
const session: Session = {
  server: 'http://localhost:8080',
  username: 'player',
  token: 'token',
  steamId: '76561198000000000',
  personaName: 'player',
}

function renderDialog(invite: string, onJoined = vi.fn()) {
  const onClose = vi.fn()
  const view = render(
    <InviteDialog invite={invite} session={session} onClose={onClose} onJoined={onJoined} />,
  )
  return { onClose, onJoined, view }
}

describe('web invite fallback', () => {
  it('joins only the lobby named in the link', async () => {
    const fetchMock = mockFetch(() => ({ json: { lobby_id: 42 } }))
    const { onJoined } = renderDialog(LINK)
    await screen.findByRole('dialog')
    fireEvent.click(screen.getByRole('button', { name: 'Join lobby 42' }))
    await waitFor(() => expect(onJoined).toHaveBeenCalledWith('42'))
    const call = fetchMock.mock.calls.find(([url]) => String(url).endsWith('/lobbies/42/join'))
    expect(call).toBeDefined()
    expect(bodyOf(call?.[1])).toEqual({ invite: LINK })
  })

  it('names expired and wrong-lobby links without joining', async () => {
    mockFetch((url) => {
      if (url.endsWith('/lobbies/42/join')) return { status: 410, text: 'invite link expired' }
      return undefined
    })
    const { onJoined } = renderDialog(LINK)
    fireEvent.click(screen.getByRole('button', { name: 'Join lobby 42' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('This invite link has expired. Ask for a new link.')
    expect(onJoined).not.toHaveBeenCalled()
  })

  it('names a link for a different lobby', async () => {
    mockFetch((url) => {
      if (url.endsWith('/lobbies/42/join')) {
        return { status: 400, text: 'invite link is for a different lobby' }
      }
      return undefined
    })
    const { onJoined } = renderDialog(LINK)
    fireEvent.click(screen.getByRole('button', { name: 'Join lobby 42' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('This invite is for a different lobby. Nothing was joined.')
    expect(onJoined).not.toHaveBeenCalled()
  })

  it('shows a bad link without calling the join API', () => {
    const fetchMock = mockFetch(() => ({ json: {} }))
    renderDialog('https://example.test/join/42')
    expect(screen.getByText('Invite link not valid')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Join lobby/ })).not.toBeInTheDocument()
    expect(fetchMock.mock.calls).toHaveLength(0)
  })

  it('closes with Escape and returns focus', async () => {
    const opener = document.createElement('button')
    opener.textContent = 'Open invite'
    document.body.append(opener)
    opener.focus()
    const { onClose, view } = renderDialog(LINK)
    const dialog = await screen.findByRole('dialog')
    fireEvent(dialog, new Event('cancel', { bubbles: true }))
    expect(onClose).toHaveBeenCalled()
    view.unmount()
    expect(opener).toHaveFocus()
    opener.remove()
  })
})
