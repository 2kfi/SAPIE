# SAPIE UI

One React 18 + TypeScript + Vite SPA in `web/`. The launcher is now a native Rust egui/eframe app
(`sapie-launcher`); it does not embed `web/dist` or run a webview. The Go server hosts the SPA on
the LAN with `--ui-listen 0.0.0.0:8081` (default) from `--ui-dir ./web/dist`. Port 8081 proxies
only user API routes; admin paths return 404 there and still work on the loopback admin listener
at 8080. The views are wired to that API: login, friends, lobbies, live sessions and account stats.

The frozen 0.1 text below stays until the user accepts G0. Local verification is the web build
only; there has been no browser click-through.

## Routes

Hash routing keeps the SPA servable from any path without server rewrites.

| Route | Screen |
| --- | --- |
| `#/login` | Server address, username, password, remember server |
| `#/register` | Opt-in account creation, shown only when `GET /api/v1/config` reports `allow_register: true` |
| `#/friends` | Main shell: nav rail, friends list, detail pane with the P2P notice |
| `#/games` | Game list and game detail/launcher in the detail pane |
| `#/account` | Shared account view: achievements and games, read-only |
| `#/live` | Live sessions admin table and presence status |

The friends detail pane states the release limit:

Peer-to-peer play is not available in this version.

This line is frozen 0.1 text. 0.2 ships P2P in code, but its change is gated on G0 acceptance.

Loading, empty and error states now come from live requests. The old `?state=loading` and
`?state=error` URL hook was a preview/debug hook from the M0 scaffold; it was removed when the
views were wired to the API and is not present in 0.1.

Login submits to `POST /api/v1/login` and keeps the returned session token. In a browser the SPA
uses a relative `/api/v1` base served by the Go server, so no server address is needed. Registration
uses `POST /api/v1/register` and is offered only when `GET /api/v1/config` allows it. Friends,
lobbies, live sessions and account stats read from the HTTP API; failures surface with retry
actions. **Browse** and **Launch** in the game detail pane still call the old Tauri native bridge;
the native egui launcher does not expose that bridge, so those controls are unavailable in the
browser web UI. Plain HTTP on the LAN means the session token crosses it in cleartext.

## Design tokens

Tokens live at the top of `web/src/styles.css` in `:root`, with dark values in the
`prefers-color-scheme: dark` block.

| Role | Light | Dark |
| --- | --- | --- |
| Background | `#E6EBF2` | `#0E1420` |
| Surface | `#F7F9FC` | `#1B2432` |
| Text | `#16202E` | `#E9EFF7` |
| Muted | `#3F4C60` | `#B3C3D6` |
| Accent | `#A63A16` | `#FF9B6A` |
| Status | `#146C43` | `#8FE3B0` |

Type: Manrope for UI, IBM Plex Mono for appIDs, paths, timestamps and logs. Both are loaded
from Google Fonts with local system fallbacks; no licensed font files ship. Scale
12/13/14/16/20/28. Spacing 4/8/12/16/24/32. Radius 6/10/16. Blur 20px panels / 28px popovers;
glass alpha 0.68. Glass appears only on the nav rail and detail pane; `@supports not
(backdrop-filter)` swaps those panels to the solid surface.

Measured contrast on the light surface is 15.55:1 (text) and 8.25:1 (muted); white on the
light ember is 6.49:1. Dark values are about 13.5:1 / 8.7:1 on the dark surface. Re-check over
bright wallpapers when the webview is made transparent.

Accessibility: every control has a visible label, focus is a 2px ember ring at 2px offset,
interactive targets are at least 44px, presence dots are always paired with text labels,
toasts use `aria-live="polite"` and errors use `role="alert"`. Motion is limited to rail
selection (120ms) and toast entry (200ms), both opt-in under
`prefers-reduced-motion: no-preference`.

## Native launcher

`client/src-tauri/` is a native Rust egui/eframe app built as `sapie-launcher`
(`sapie-launcher.exe` on Windows, with `windows_subsystem`). It has a native file picker, copies
the matching `steam_api` DLL or shared object, sets `LD_PRELOAD`, detects the Steam interface era,
unpacks SteamStub executables, writes the connection files and starts the game with SAPIE
credentials. It stores the server/username/password connection and per-game settings locally.

There is no Tauri shell, webview, WebKit, Electron, embedded SPA or localhost page. The old
`web/src/tauri.ts` bridge remains in the SPA source; its calls fail outside a Tauri host, which is
the case for the native launcher. No cross-compiled Windows build has been executed locally.

## Build

```sh
cd web && npm install && npm run build   # tsc --noEmit + vite build -> web/dist
cd web && npx tsc --noEmit
cd client/src-tauri && cargo check       # needs libxkbcommon/wayland/x11/xi/xrandr/gl1-mesa dev libs on Linux
```
