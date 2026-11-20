const keyFor = (path: string) => `sapie.cover.${path}`

export function readCover(path: string): string | null {
  try {
    return localStorage.getItem(keyFor(path))
  } catch {
    return null
  }
}

export function saveCover(path: string, dataUrl: string): void {
  localStorage.setItem(keyFor(path), dataUrl)
}

export function clearCover(path: string): void {
  localStorage.removeItem(keyFor(path))
}
