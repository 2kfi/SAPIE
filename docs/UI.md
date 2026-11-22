# SAPIE UI

One React 18 + TypeScript + Vite SPA in `web/`, plus the 0.5 native launcher in `launcher/`: a Qt 6
Quick UI with a Rust core behind a C ABI and no webview. The launcher does not use `web/dist`. The Go
server hosts the
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
| `#/friends` | Friends list and detail pane with the P2P explanation and lobbies |
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

Tokens live in `design/tokens.css` and are imported by the web build. `tools/gen-qml-tokens` derives
the launcher's `Tokens.qml` from the same file. There is one dark theme.

| Role | Value |
| --- | --- |
| App background | `#17191D` |
| Sidebar | `#14161A` |
| Panel | `#1D2025` |
| Text | `#E6E8EB` |
| Secondary text | `#A0A5AE` |
| Accent | `#C5A4FF` |
| Success | `#73C991` |

Type: Manrope for UI, IBM Plex Mono for app IDs, lobby IDs, Steam IDs and other machine values.
Fonts are named with system fallbacks. No font files ship and no remote font is imported. Scale
12/13/14/16/20/24. Spacing 4/8/12/16/20/24/32. Radius 4 and 6 only. Glass blur and the ember
accent are retired from this surface.

Contrast computed from those tokens: text on panel 13.31:1, secondary text on panel 6.6:1, dark
ink on accent 8.49:1, success on panel 8.16:1. Focus is a 2px accent ring at 2px offset,
interactive targets are at least 44px, and presence dots are paired with text labels. Toasts use
`aria-live="polite"` and errors use `role="alert"`.

Motion is one GSAP timeline on first authenticated shell mount, scoped to the shell and skipped
when `prefers-reduced-motion: reduce` is set. Nav icon feedback is a short `contextSafe` tween.
There are no per-card entrances, no delayed content and no fake progress states.

The reasoning behind the shared palette, type and motion is in `design/identity.md`. Dark only
stays, with no light theme or theme toggle on either surface. Copy for the shared surfaces is in
[identity surface copy](SURFACES.md).

## Native launcher

`launcher/` holds the 0.5 launcher. `launcher/core` is a Rust core behind a versioned C ABI,
`launcher/ui` is the shared Qt Quick UI, and `launcher/windows` and `launcher/linux` hold platform
integration and packaging. The 0.4 Tauri app stays in `client/src-tauri` as the prior artifact; no
Tauri code is used by the 0.5 launcher. The shell is a local game library: sign in, add an
executable, review detected arch, era and appid, then press Play. It uses native file dialogs, backs
up the original library, copies the matching `steam_api` file, writes the connection files and starts
the game. Game details offer repair library, restore original, remove from library, and per-game
arguments and a Linux preload override. Friends, activity and settings are reachable from the rail;
there is no chat or store.

The core calls `POST /api/v1/login` and keeps only the session token in the OS keychain. It passes
the token to the game through a short-lived 0600 token file, with an owner-only ACL on Windows in the
0.5 path. It does not persist the account password. The web SPA makes no native calls and has no
Tauri bridge.

Four Windows DLL variants (windows-x86/x64 × legacy/modern) were cross-built and inspected; all
export the five new `SteamContentServer*` symbols and import no `libwinpthread-1.dll`. Native GUI
click-through was not exercised headless.

## Build

```sh
cd web && npm install && npm run build        # tsc --noEmit + vite build -> web/dist
cd web && npx tsc --noEmit
cd launcher && cargo test                     # Rust core
cmake -S launcher -B launcher/build && cmake --build launcher/build
QT_QPA_PLATFORM=offscreen /usr/lib/qt6/bin/qmltestrunner -input launcher/ui/screens/tst_screens.qml
```

The CMake target requires Qt 6.8. Local Qt is 6.12.0, so a local configure is a compatibility check
only. No packaged 0.5 launcher build was verified for this record.

The launcher is dark only. `design/DESIGN.md` records the decision; there is no light theme or
theme toggle on either surface.
