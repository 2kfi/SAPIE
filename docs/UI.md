# SAPIE UI (0.1)

One React 18 + TypeScript + Vite SPA in `web/`. The same `web/dist` is embedded by the
Tauri launcher (`client/src-tauri/`). The Go server serves the HTTP API the SPA calls; in 0.1 it
does not host the SPA's static files. The views are wired to that API: login, friends, lobbies,
live sessions and account stats.

## Routes

Hash routing keeps the SPA servable from any path without server rewrites.

| Route | Screen |
| --- | --- |
| `#/login` | Server address, username, password, remember server |
| `#/friends` | Main shell: nav rail, friends list, detail pane with the P2P notice |
| `#/games` | Game list and game detail/launcher in the detail pane |
| `#/account` | Shared account view: achievements and games, read-only |
| `#/live` | Live sessions admin table and presence status |

The friends detail pane states the release limit:

Peer-to-peer play is not available in this version.

Loading, empty and error states now come from live requests. The old `?state=loading` and
`?state=error` URL hook was a preview/debug hook from the M0 scaffold; it was removed when the
views were wired to the API and is not present in 0.1.

Login submits to `POST /api/v1/login` and keeps the returned session token. Friends, lobbies,
live sessions and account stats read from the HTTP API; failures surface with retry actions.
**Browse** calls the Tauri `pick_executable` command. **Launch** stays disabled until a game is
configured, with the reason shown next to it.

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

## Tauri shell

`client/src-tauri/` registers implemented commands for the game flow. The four named here
handle the native half of it:

- `pick_executable`: opens the native executable picker
- `copy_dll`: copies the matching `steam_api` DLL or shared object next to the game executable
- `set_ld_preload`: records the Linux `LD_PRELOAD` library for the game
- `launch_game`: starts the configured game with SAPIE credentials

The same flow also calls `detect_game`, `unpack_steamstub` and `configure_server` to detect the
Steam interface era, unpack SteamStub executables and write the connection files.

`tauri.conf.json` points `frontendDist` at `../../web/dist` and `devUrl` at
`http://localhost:5173`. `capabilities/default.json` grants the launcher commands through
`permissions/launcher.toml`.

## Build

```sh
cd web && npm install && npm run build   # tsc --noEmit + vite build -> web/dist
cd web && npx tsc --noEmit
cd client/src-tauri && cargo check       # needs webkit2gtk-4.1 dev libs on Linux
```
