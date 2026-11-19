export type ViewState = 'empty' | 'loading' | 'error'

// shortcut: URL ?state= previews M0 states without a backend; drop when the API lands.
export function readPreviewState(): ViewState {
  const value = new URLSearchParams(window.location.search).get('state')
  return value === 'loading' || value === 'error' ? value : 'empty'
}
