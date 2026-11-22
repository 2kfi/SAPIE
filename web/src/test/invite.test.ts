import { describe, expect, it } from 'vitest'
import { clearInviteParam, parseInviteLink, readInviteParam } from '../lib/invite'

const LINK = 'sapie://join/42?exp=1795199695&sig=abc'

describe('invite links', () => {
  it('parses a real join link and keeps the exact URL', () => {
    expect(parseInviteLink(`  ${LINK}  `)).toEqual({ lobbyId: '42', url: LINK })
  })

  it('rejects links that are not a single numeric join path', () => {
    const bad = [
      'https://example.test/join/42',
      'sapie://join/42#tail',
      'sapie://user@join/42?exp=1',
      'sapie://join:8080/42?exp=1',
      'sapie://join/42/extra?exp=1',
      'sapie://join/not-a-number?exp=1',
      'not a link',
    ]
    for (const value of bad) expect(parseInviteLink(value)).toBeNull()
  })

  it('reads an invite from the query string or the hash query', () => {
    window.history.replaceState(null, '', `/?invite=${encodeURIComponent(LINK)}#/friends`)
    expect(readInviteParam()).toBe(LINK)
    window.history.replaceState(null, '', `/#/friends?invite=${encodeURIComponent(LINK)}`)
    expect(readInviteParam()).toBe(LINK)
  })

  it('clears the invite parameter without dropping the route', () => {
    window.history.replaceState(null, '', `/?invite=${encodeURIComponent(LINK)}#/friends?invite=${encodeURIComponent(LINK)}`)
    clearInviteParam()
    expect(window.location.search).toBe('')
    expect(window.location.hash).toBe('#/friends')
    expect(readInviteParam()).toBeNull()
  })
})
