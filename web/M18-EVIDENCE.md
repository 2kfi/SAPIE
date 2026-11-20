# M18 evidence

Branch: `r21/m18`
Base: `7342d90`
Scope: `web/**` only

## Screens and routes changed

- Sign in (`#/login`): real login call, specific credential and server errors, expiry reason from global 401 handling, no password or token persisted.
- Register (`#/register`): unchanged flow, typed client only.
- Friends (`#/friends`): replaced the placeholder list with Friends, Requests and Blocked tabs. Real send, accept, decline, cancel, remove, block, unblock, invite and join actions. Empty, loading and error states for each list.
- Profile (`#/account`): real stats, achievements and active sessions from the server. Revoke and sign out everywhere are wired. Persona edit is intentionally absent because no update endpoint exists.
- Live sessions (`#/live`): typed lobby list, refresh, empty, loading and error states. Lobby create and join panel remains in the detail pane.
- Shell: rail is Friends, Profile, Live sessions and Sign out. The Games route and both placeholder game screens are gone. Skip link and keyboard focus styles are present.
- Theme: shared tokens come from `design/tokens.css`. Dark-only choice and product reason are recorded in `web/DESIGN.md`. No theme toggle is shown because there is no second theme.

## Commands run

- `npm exec tsc --noEmit`: passed.
- `npm run build`: passed. Vite 6.4.6 transformed 47 modules. CSS 14.48 kB, JS 259.23 kB.
- `npm test`: passed. 25 tests across 5 files: API client, login, friends, lobbies, app session flow.
- `git diff --check`: passed.
- Headless browser click-through: passed. Playwright 1.65.1 with cached Chromium headless shell, built `dist` served on `127.0.0.1:4173`, API replies mocked from `docs/API.md`. 28 assertions passed across 42 intercepted API calls.

## Browser exercise detail

- Signed in and reached Friends. No token or password in local storage, session storage, rendered text or any request URL.
- Sent a friend request by username and saw it listed in Outgoing.
- Moved tabs with the right arrow key and saw Blocked selected.
- Joined a friend lobby from the detail pane.
- Opened the remove confirmation, pressed Escape, confirmed no delete call, then removed with confirmation.
- Viewed real stats and achievements, revoked another session and saw the result.
- Viewed live lobbies, signed out, confirmed `/logout` was called and returned to sign in.
- Repeated login and navigation at 1440px and 360px. No horizontal overflow on login, Friends, Profile or Live sessions. No em dash in rendered text.

## UNVERIFIED

- No live SAPIE server was running. API shapes and status handling were exercised against mocks derived from `docs/API.md`, not the Go server.
- Persona edit is not implemented. The server has no persona update endpoint, and server changes are outside this milestone scope.
- No screen reader was run. Keyboard tab order, arrow-key tabs, Escape on dialogs and skip-link presence were exercised.
- Light theme is not shipped. The reason is documented in `web/DESIGN.md`.
- Windows, macOS and real WebView environments were not tested. Only headless Chromium on Linux.

## Assumptions

- Antislop global settings file was absent. Active session override was assumed because the user was asleep.
- The Games route was removed rather than stubbed because the launcher owns games and there is no server game-library endpoint.
