import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import Avatar from '../components/Avatar'

describe('Avatar', () => {
  it('shows the first initial of a persona name', () => {
    const { container } = render(<Avatar name="alice" small />)
    const avatar = container.querySelector('.avatar')
    expect(avatar).toHaveClass('avatar--sm')
    expect(avatar).toHaveTextContent('A')
  })

  it('uses a question mark when no persona name is available', () => {
    const { container } = render(<Avatar name="   " />)
    expect(container.querySelector('.avatar')).toHaveTextContent('?')
  })
})
