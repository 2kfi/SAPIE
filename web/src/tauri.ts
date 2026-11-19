import { invoke } from '@tauri-apps/api/core'

export type DetectResult = {
  arch: string
  era: string
  interfaces: string[]
  steam_api_path: string | null
}

export type Connection = {
  server: string
  username: string
  password: string
}

function call<T>(command: string, args?: Record<string, unknown>): Promise<T> {
  if (!('__TAURI_INTERNALS__' in window)) {
    return Promise.reject(new Error('Native bridge unavailable outside the Tauri launcher.'))
  }
  return invoke<T>(command, args)
}

export function pickExecutable(): Promise<string | null> {
  return call<string | null>('pick_executable')
}

export function detectGame(exePath: string): Promise<DetectResult> {
  return call<DetectResult>('detect_game', { exe_path: exePath })
}

export function unpackSteamstub(exePath: string): Promise<string | null> {
  return call<string | null>('unpack_steamstub', { exe_path: exePath })
}

export function copyDll(exePath: string, era: string): Promise<string> {
  return call<string>('copy_dll', { exe_path: exePath, era })
}

export function setLdPreload(exePath: string, libPath: string): Promise<void> {
  return call<void>('set_ld_preload', { exe_path: exePath, lib_path: libPath })
}

export function configureServer(exePath: string, connection: Connection): Promise<void> {
  return call<void>('configure_server', {
    exe_path: exePath,
    server: connection.server,
    username: connection.username,
    password: connection.password,
  })
}

export function launchGame(exePath: string): Promise<void> {
  return call<void>('launch_game', { exe_path: exePath })
}
