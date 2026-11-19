import { invoke } from '@tauri-apps/api/core'

export type NativeCommand =
  | 'pick_executable'
  | 'copy_dll'
  | 'set_ld_preload'
  | 'launch_game'

export function callNative(command: NativeCommand): Promise<unknown> {
  if (!('__TAURI_INTERNALS__' in window)) {
    return Promise.reject(new Error('Native bridge unavailable outside the Tauri launcher.'))
  }
  return invoke(command)
}
