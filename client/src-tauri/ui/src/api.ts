export type ErrorKind =
  | 'invalid_path'
  | 'not_found'
  | 'permission_denied'
  | 'backup_conflict'
  | 'invalid_server'
  | 'unreachable'
  | 'bad_credentials'
  | 'server_too_old'
  | 'server_error'
  | 'token_expired'
  | 'keychain_error'
  | 'io'
  | 'process'

export type CommandError = { kind: ErrorKind; message: string }

export type GameRecord = {
  exe_path: string
  arch: string
  era: string
  interfaces: string[]
  original_library: string | null
  library: string | null
  appid: number | null
  configured: boolean
  launch_args: string[]
  ld_preload: string | null
  last_played: number | null
}

export type DetectResult = {
  exe_path: string
  arch: string
  era: string
  interfaces: string[]
  original_library: string | null
  warning: string | null
}

export type InstallResult = {
  arch: string
  era: string
  interfaces: string[]
  original_library: string | null
  library: string
  warning: string | null
}

export type ConfigureResult = {
  appid: number | null
  game_server: string
  note: string
}

export type SessionInfo = {
  token: string
  steam_id: number
  persona_name: string
  server: string
  expires_at: number
}

export type ProgressEvent = { step: string; message: string }

export const PROGRESS_EVENT = 'sapie-progress'

type Invoke = <T>(command: string, args?: Record<string, unknown>) => Promise<T>

function invoke<T>(command: string, args?: Record<string, unknown>): Promise<T> {
  const core = (
    window as Window & { __TAURI__?: { core?: { invoke?: Invoke } } }
  ).__TAURI__?.core
  if (!core?.invoke) {
    return Promise.reject(new Error('Tauri IPC is unavailable in this window.'))
  }
  return core.invoke<T>(command, args)
}

export const api = {
  detectGame: (path: string) => invoke<DetectResult>('detect_game', { path }),
  installLibrary: (path: string) => invoke<InstallResult>('install_library', { path }),
  restoreLibrary: (path: string) => invoke<string | null>('restore_library', { path }),
  configureGame: (path: string, appid: number) =>
    invoke<ConfigureResult>('configure_game', { path, appid }),
  launchGame: (path: string) => invoke<number>('launch_game', { path }),
  unpackSteamstub: (path: string) => invoke<string | null>('unpack_steamstub', { path }),
  login: (server: string, username: string, password: string) =>
    invoke<SessionInfo>('login', { server, username, password }),
  signOut: () => invoke<void>('sign_out'),
  testConnection: (server: string) => invoke<void>('test_connection', { server }),
  libraryList: () => invoke<GameRecord[]>('library_list'),
  libraryAdd: (path: string) => invoke<GameRecord>('library_add', { path }),
  libraryRemove: (path: string) => invoke<void>('library_remove', { path }),
}
