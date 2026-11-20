# SAPIE UI

One React 18 + TypeScript + Vite SPA in `web/`, plus a separate React UI bundled into the Tauri v2
launcher at `client/src-tauri/ui/`. The launcher does not use `web/dist`. The Go server hosts the
web SPA on the LAN with `--ui-listen 0.0.0.0:8081` (default) from `--ui-dir ./web/dist`. Port 8081
proxies only user API routes; admin paths return 404 there and still work on the loopback admin
listener at 8080. The views are wired to that API: login, friends, lobbies, live sessions and
account achievements.

The UI is dark only. There is no light theme and no theme toggle. Values shown in the UI come from
API responses, from the browser session, or they are absent. Game setup, admin credentials and
user administration are not available in the browser.

## Routes

Hash routing keeps the SPA servable from any path without server rewrites.

| Route | Screen |
| --- | --- |
| `#/login` | Server sign in, with registration offered only when `GET /api/v1/config` allows it |
| `#/register` | Opt-in account creation |
| `#/friends` | Friends list and detail pane with the P2P explanation |
| `#/games` | Read-only "Managed by native launcher" panel in the detail pane |
| `#/account` | Achievements from `GET /api/v1/users/{steam_id}/stats`, read-only |
| `#/live` | Lobby table from `GET /api/v1/lobbies` |

The friends detail pane says:

Friends can play directly with each other, and the server relays traffic when a direct connection is not possible.

This describes direct peer-to-peer play between friends with a server relay fallback. It has not
been verified in a live two-client game.

Loading, empty and error states come from live requests. The old `?state=loading` and
`?state=error` preview hook is not present. Login and registration navigate as soon as the API
answers; there is no artificial delay.

Game setup is owned by the native launcher. It selects the executable, installs the matching
Steam library, writes the connection files and starts the game. The browser pane does not call the
old Tauri bridge and has no file pickers or launch buttons. The admin token form and the panels
that called `/api/v1/users` and `/api/v1/users/{steam_id}/stats` with that token were removed from
the live sessions screen, because those admin routes return 404 on port 8081 by design.

## Design tokens

Tokens live at the top of `web/src/styles.css` in `:root`. There is only one dark theme.

| Role | Value |
| --- | --- |
| Background | `#0e1420` |
| Surface | `#1b2432` |
| Quiet surface | `#16202e` |
| Text | `#e9eff7` |
| Muted | `#b3c3d6` |
| Ember accent | `#ff9b6a` |
| Status | `#8fe3b0` |

Type: Manrope for UI, IBM Plex Mono for app IDs, lobby IDs, Steam IDs and other machine values.
Both are loaded from Google Fonts with local system fallbacks; no licensed font files ship. Scale
12/13/14/16/20/28. Spacing 4/8/12/16/20/24/32. Radius 4/6/10/16.

Glass blur is limited to the nav rail and the detail pane. `@supports not (backdrop-filter)`
swaps those two panels to the solid surface. The workspace, forms, rows and toasts are solid.

Measured contrast: text on surface 13.5:1, muted on surface 8.69:1, dark ink on ember 9.11:1,
status on surface 10.25:1. Focus is a 2px ember ring at 2px offset, interactive targets are at
least 44px, and presence dots are always paired with text labels. Toasts use `aria-live="polite"`
and errors use `role="alert"`.

Motion is one GSAP timeline on first authenticated shell mount, scoped to the shell and skipped
when `prefers-reduced-motion: reduce` is set. Nav icon feedback is a short `contextSafe` tween.
There are no per-card entrances, no delayed content and no fake progress states.

The reasoning behind each colour, icon, radius and type choice is in `web/DESIGN.md`.

## Native launcher

`client/src-tauri/` is a Tauri v2 app built as `sapie-launcher` (`sapie-launcher.exe` on Windows,
with `windows_subsystem`). Its React UI lives in `ui/` and is bundled into the binary. The shell is
a local game library: sign in, add an executable, review detected arch, era and appid, then press
Play. It uses native file dialogs, backs up the original library, copies the matching `steam_api`
file, writes the connection files and starts the game. Game details offer repair library, restore
original, remove from library, and display per-game arguments and a Linux preload override. Activity
and settings are reachable from the rail. There are no friends, chat or store screens in the launcher
in 0.4.

The launcher calls `POST /api/v1/login` and keeps only the session token in the OS keychain. It
passes the token to the game through a short-lived 0600 token file with environment fallback; it
does not persist the account password. The web SPA makes no native calls and has no Tauri bridge.

Four Windows DLL variants (windows-x86/x64 × legacy/modern) were cross-built and inspected; all
export the five new `SteamContentServer*` symbols and import no `libwinpthread-1.dll`. Native GUI
click-through was not exercised headless.

## Build

```sh
cd web && npm install && npm run build        # tsc --noEmit + vite build -> web/dist
cd web && npx tsc --noEmit
cd client/src-tauri && cargo check             # Tauri shell
cd client/src-tauri/ui && npm ci && npm run build
cd client/src-tauri/ui && npm test             # Vitest
```

The launcher is dark only. `design/DESIGN.md` records the decision; there is no light theme or
theme toggle on either surface.
