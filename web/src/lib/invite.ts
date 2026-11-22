export type ParsedInvite = {
  lobbyId: string
  url: string
}

export function parseInviteLink(raw: string): ParsedInvite | null {
  const urlText = raw.trim()
  let parsed: URL
  try {
    parsed = new URL(urlText)
  } catch {
    return null
  }
  if (
    parsed.protocol !== 'sapie:'
    || parsed.hostname !== 'join'
    || parsed.username
    || parsed.password
    || parsed.port
    || parsed.hash
  ) {
    return null
  }
  const lobbyId = parsed.pathname.replace(/^\//, '')
  if (!/^\d+$/.test(lobbyId) || parsed.pathname !== `/${lobbyId}`) return null
  return { lobbyId, url: urlText }
}

export function readInviteParam(): string | null {
  const fromSearch = new URLSearchParams(window.location.search).get('invite')?.trim()
  if (fromSearch) return fromSearch
  const hash = window.location.hash.replace(/^#/, '')
  const queryAt = hash.indexOf('?')
  if (queryAt < 0) return null
  return new URLSearchParams(hash.slice(queryAt + 1)).get('invite')?.trim() || null
}

export function clearInviteParam(): void {
  const url = new URL(window.location.href)
  url.searchParams.delete('invite')
  const hash = url.hash.replace(/^#/, '')
  const queryAt = hash.indexOf('?')
  if (queryAt >= 0) {
    const params = new URLSearchParams(hash.slice(queryAt + 1))
    if (params.has('invite')) {
      params.delete('invite')
      const path = hash.slice(0, queryAt)
      const query = params.toString()
      url.hash = query ? `#${path}?${query}` : `#${path}`
    }
  }
  window.history.replaceState(null, '', `${url.pathname}${url.search}${url.hash}`)
}
