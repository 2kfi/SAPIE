export type Route = 'login' | 'register' | 'friends' | 'account' | 'live'

const routes: Route[] = ['login', 'register', 'friends', 'account', 'live']

export function readRoute(): Route {
  const path = window.location.hash.replace(/^#\/?/, '').split('?')[0]
  return routes.find((route) => route === path) ?? 'login'
}

export function navigate(route: Route): void {
  window.location.hash = `#/${route}`
}
