import type { ErrorKind } from './api'

const KINDS: ErrorKind[] = [
  'invalid_path',
  'not_found',
  'permission_denied',
  'backup_conflict',
  'invalid_server',
  'unreachable',
  'bad_credentials',
  'server_too_old',
  'server_error',
  'token_expired',
  'keychain_error',
  'io',
  'process',
]

const COPY: Record<ErrorKind | 'unknown', string> = {
  invalid_path: 'Enter an absolute path to the executable.',
  not_found: 'That file or game was not found on disk.',
  permission_denied: 'The launcher cannot write there. Check the folder permissions.',
  backup_conflict: 'An original library backup exists and does not belong to SAPIE. No file was replaced.',
  invalid_server: 'Enter the server as host:port, for example 127.0.0.1:8081.',
  unreachable: 'Could not reach the SAPIE server. Check the host and that the server is running.',
  bad_credentials: 'Wrong username or password.',
  server_too_old: 'This server is too old. Update the SAPIE server to the 0.4 API.',
  server_error: 'The SAPIE server returned an error. Check the server log.',
  token_expired: 'The session has ended. Sign in again.',
  keychain_error: 'The OS keychain rejected the session token. Check the keychain and try again.',
  io: 'A local file could not be read or written.',
  process: 'The game process could not be started.',
  unknown: 'The launcher could not complete that action.',
}

export type ParsedError = { kind: ErrorKind | 'unknown'; message: string }

export function readError(error: unknown): ParsedError {
  const candidate = error as { kind?: unknown; message?: unknown } | null
  const kind = KINDS.find((value) => value === candidate?.kind) ?? 'unknown'
  const message = typeof candidate?.message === 'string' ? candidate.message.trim() : ''
  return { kind, message: message || COPY[kind] }
}

export function explain(error: unknown): string {
  const parsed = readError(error)
  return parsed.kind === 'unknown' && parsed.message ? parsed.message : COPY[parsed.kind]
}

export function validServer(value: string): boolean {
  const server = value.trim()
  const hostPort = /^[^/\s:]+:(\d{1,5})$/.test(server) || /^\[[0-9a-fA-F:]+\]:(\d{1,5})$/.test(server)
  if (!hostPort) return false
  const port = Number(server.slice(server.lastIndexOf(':') + 1))
  return port >= 1 && port <= 65535
}
