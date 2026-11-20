import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlainHttpNotice from '../components/PlainHttpNotice'

describe('PlainHttpNotice', () => {
  it('names the cleartext risk on HTTP without a dismiss control', () => {
    render(<PlainHttpNotice protocol="http:" />)
    const notice = screen.getByRole('note', { name: 'Connection security' })
    expect(notice).toHaveTextContent('This page was loaded over HTTP.')
    expect(notice).toHaveTextContent('Session tokens are sent without encryption.')
    expect(notice.querySelector('button')).toBeNull()
  })

  it('renders nothing on HTTPS', () => {
    render(<PlainHttpNotice protocol="https:" />)
    expect(screen.queryByRole('note')).not.toBeInTheDocument()
  })
})
