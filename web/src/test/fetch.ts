import { vi } from 'vitest'

export type Reply = { status?: number; json?: unknown; text?: string }

export function mockFetch(
  handler: (url: string, init?: RequestInit) => Reply | undefined,
) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const reply = handler(url, init)
    if (!reply) return new Response('unexpected request', { status: 500 })
    const body = reply.text ?? (reply.json !== undefined ? JSON.stringify(reply.json) : '')
    return new Response(body, {
      status: reply.status ?? 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

export function bodyOf(init?: RequestInit): Record<string, unknown> {
  if (typeof init?.body !== 'string') return {}
  try {
    return JSON.parse(init.body) as Record<string, unknown>
  } catch {
    return {}
  }
}

export function headerOf(init: RequestInit | undefined, name: string): string | undefined {
  const headers = init?.headers as Record<string, string> | undefined
  if (!headers) return undefined
  const key = Object.keys(headers).find((item) => item.toLowerCase() === name.toLowerCase())
  return key ? headers[key] : undefined
}
