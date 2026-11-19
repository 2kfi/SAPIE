# SAPIE UI (M0 scaffold)

One React 18 + TypeScript + Vite SPA in `web/`. The same `web/dist` is embedded by the
Tauri launcher (`client/src-tauri/`) and can be served by the Go server later. There is no
backend in M0: every data view starts in its empty state, with loading/error states available
for wiring and review.

## Routes

Hash routing keeps the SPA servable from any path without server rewrites.

| Route | Screen |
| --- | --- |
| `#/login` | Server address, username, password, remember server |
| `#/friends` | Main shell: nav rail, friends list, detail pane with the P2P notice |
| `#/games` | Game list and game detail/launcher in the detail pane |
| `#/account` | Shared account view: achievements and games, read-only |
| `#/live` | Live sessions admin table and presence status |

`?state=loading` or `?state=error` before the hash previews a data view's non-empty states.
Example: `index.html?state=error#/games` shows the missing-executable error and **Edit path**.
This URL hook is a scaffold shortcut; remove it when the API is connected.

Form submission on Login and the Retry actions report that no server is connected instead of
pretending to succeed. **Browse** calls the Tauri `pick_executable` stub. **Launch** stays
disabled until a game is configured, with the reason shown next to it.

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

`client/src-tauri/` registers four no-op commands. Each returns a `Not implemented in M0`
error so the JS bridge fails loudly instead of silently succeeding:

- `pick_executable`
- `copy_dll`
- `set_ld_preload`
- `launch_game`

`tauri.conf.json` points `frontendDist` at `../../web/dist` and `devUrl` at
`http://localhost:5173`. `capabilities/default.json` grants the four commands through
`permissions/launcher.toml`. No game launching, DLL copying or preload setup is implemented.

## Build

```sh
cd web && npm install && npm run build   # tsc --noEmit + vite build -> web/dist
cd web && npx tsc --noEmit
cd client/src-tauri && cargo check       # needs webkit2gtk-4.1 dev libs on Linux
```
