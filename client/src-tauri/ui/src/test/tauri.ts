import { vi } from 'vitest'
import type { GameRecord } from '../api'

type Handler = (args?: Record<string, unknown>) => unknown

export function mockTauri(handlers: Record<string, Handler> = {}) {
  const invoke = vi.fn(async (command: string, args?: Record<string, unknown>) => {
    const handler = handlers[command]
    if (!handler) throw new Error(`Unexpected command: ${command}`)
    return handler(args)
  })
  const listen = vi.fn(async () => () => {})
  Object.defineProperty(window, '__TAURI__', {
    configurable: true,
    writable: true,
    value: { core: { invoke }, event: { listen } },
  })
  return { invoke, listen }
}

export function game(path: string, overrides: Partial<GameRecord> = {}): GameRecord {
  return {
    exe_path: path,
    arch: 'x86',
    era: 'old',
    interfaces: ['ISteamUser'],
    original_library: null,
    library: null,
    appid: null,
    configured: false,
    launch_args: [],
    ld_preload: null,
    last_played: null,
    ...overrides,
  }
}
