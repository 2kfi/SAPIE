# SAPIE agent guide

SAPIE is a non-commercial Steam API replacement for old games you already own. A Go server owns
accounts, presence, lobbies, achievements and stats. A forked Goldberg `steam_api` library inside
the game forwards calls to that server. A native launcher sets the game up and starts it.

This file is the entry point for coding agents. Read it before changing code. For UI work also read
`AGENTS-UI.md` (0.5 visual direction), `design/DESIGN.md` (launcher direction), `design/tokens.css`
(shared tokens) and the antislop core rules. When `AGENTS-UI.md` and the older design docs disagree,
`AGENTS-UI.md` wins; the deliberate overrides are listed in `plan.md` section 13.3 and belong in
`design/identity.md`.

## Repository map

| Path | What it is |
| --- | --- |
| `server/` | Go server, SQLite via `modernc.org/sqlite`, no cgo. Entry: `cmd/sapie-server`. |
| `client/dll/` | Goldberg fork, minimal edits. Game-side Steam API emulation. |
| `client/sapie/` | New client transport, mirror and stats code. |
| `client/src-tauri/` | 0.4 Tauri v2 shell in `src/`, React UI in `ui/`, kept as the prior artifact. The 0.5 launcher does not use it. |
| `launcher/` | 0.5 launcher: Qt 6 + Qt Quick UI, Rust core behind a C ABI in `core/`, platform code in `windows/` and `linux/`. |
| `web/` | React 18 + TypeScript + Vite SPA served by the Go server on the LAN. |
| `design/` | `tokens.css` (single token source), `DESIGN.md` (launcher direction), `identity.md` (0.5, M22), fonts and icons. |
| `AGENTS-UI.md` | 0.5 visual direction for the launchers and the web UI. Reference, not code. |
| `proto/` | `net.proto` (vendored) and `sapie.proto` (envelope). Copies in `client/dll` must stay in sync. |
| `tools/` | Harness, protobuf build, release bundling, proto sync check. |
| `docs/` | Protocol, API, security, build, quickstart, UI, contributing, changelog and verification evidence. |
| `plan.md` | Decision log and milestone history. Section 12 is the 0.4 plan, section 13 the live 0.5 plan. |

Branches: `dev-client` is the default and integration trunk. Release PRs go `dev-client` to
`main-client`. Do not push directly to `main-client`.

## Commands

```sh
# server
cd server && go test ./... && go vet ./...
cd server && go run ./cmd/sapie-server

# launcher Rust core
cd launcher && cargo test
cd launcher && cargo clippy --all-targets -- -D warnings

# launcher Qt configure and QML tests
cmake -S launcher -B launcher/build
QT_QPA_PLATFORM=offscreen /usr/lib/qt6/bin/qmltestrunner -input launcher/ui/screens/tst_screens.qml

# web
cd web && npm install && npm run build && npm test

# proto copies must match
bash tools/check-proto-sync.sh

# container (serves the web UI on 127.0.0.1:8081)
docker compose up --build
```

The 0.5 launcher needs Qt 6.8 with CMake and the Rust toolchain. It uses no webview, so
`libwebkit2gtk-4.1-dev` and `libayatana-appindicator3-dev` are not launcher dependencies. See
`docs/BUILD.md` for the build matrix. Python tooling always runs inside a virtual environment
(`note.md`). Never commit build products; CI rejects them.

## Non-negotiable rules

- The client is the trust boundary. The server never trusts a client-supplied SteamID. The server
  overwrites `source_id` from the authenticated connection.
- No fake data in the UI: no invented players, stats, testimonials, game art or claims. A
  placeholder must look like a placeholder.
- Antislop rules apply to every new UI surface. No em dash character in copy. No dead buttons. No
  nav item without a real destination. Complete empty, loading and error states.
- The Goldberg fork stays LGPLv3. Keep notices and publish modifications. Do not modify Steamless.
- Do not add a dependency for a few lines. Use the standard library or an existing dependency.
- Fix the root cause once in shared code; do not patch callers.

---

# Launcher specification (0.4)

Status: this section describes the published 0.4 artifact, not the 0.5 tree. The 0.4 launcher was a
Tauri v2 shell in `client/src-tauri` with a React UI and no egui source. It was dark only, with the
reason in `design/DESIGN.md`, and used a 0600 session-token file with environment fallback described
in `docs/PROTOCOL.md` and `docs/SECURITY.md`. The `0.4` tag exists and installer builds were recorded;
execution on clean machines is UNVERIFIED. The verification status is tracked in
`docs/verification/0.4-rc-checklist.md` (draft). The 0.5 direction is in the section below.

## Diagnosis

The 0.2 launcher is a configuration wizard: six linear steps, one game at a time, no library, no
real server connection, and it keeps the user's password in a plaintext JSON file. The config work
is correct, but the product is not a launcher yet. It is a setup tool wearing launcher clothes.

0.4 rebuilds it as a game library. Configuration becomes a one-time modal inside that library, not
the main navigation.

## Tech stack decision

**Use Tauri v2 with a React 18 + TypeScript + Vite frontend and a Rust backend.**

The current `egui/eframe` app is replaced. The directory `client/src-tauri` is already named for
Tauri and still contains Tauri scaffold leftovers, so it is reused as the new app root.

Why:

- The project already has a polished React design system in `web/`. A Tauri surface can share its
  tokens, components and icons instead of maintaining a second visual language in immediate-mode
  widgets.
- A game library needs cover tiles, grids, hover states, search, filtering and transitions. These
  are cheap in CSS and expensive in egui.
- Tauri still gives one small native binary per OS, native file dialogs and direct process control.
  It is not Electron.
- egui's look is the reason the current launcher feels unfinished. Restyling it further fights the
  toolkit.

Costs and risks:

- This reverses the 0.2 decision "no webview". That decision is superseded here, on purpose.
- Windows uses the built-in WebView2 runtime (present on Windows 10/11; ship the bootstrapper for
  older installs). Linux needs `webkit2gtk-4.1`. macOS would need WKWebView if added later.
- The launcher must not load remote content. Bundle the UI in the binary and keep the CSP locked.

If a single dependency-free binary becomes a hard requirement, egui is the only realistic
alternative, and the visual bar must be accepted as lower. Do not mix both.

### Frontend

- React 18, TypeScript, Vite 6.
- Plain CSS with tokens shared with `web/` (`design/tokens.css` as the single source). No Tailwind,
  no component library that brings its own look.
- GSAP only where motion has a reason. Respect `prefers-reduced-motion`.
- State: local React state plus the Rust command layer. No global store until a second screen needs
  it.

### Rust backend (Tauri commands)

Keep the existing detection logic and tests; move it behind commands:

- `detect_game(path)` returns arch, era, interfaces, original library path.
- `install_library(path)` copies the matching `steam_api` file and preserves the original backup.
- `restore_library(path)` puts the original back and removes SAPIE's copy.
- `configure_game(path, appid)` writes `sapie_server.txt`, `steam_appid.txt` and launch settings.
- `launch_game(path)` spawns the process with the connection config.
- `unpack_steamstub(path)` runs the Windows Steamless sidecar.
- `login(server, username, password)` performs a real `POST /api/v1/login` and returns the session
  token. The launcher can then show real connection state instead of a green step that lies.
- `library_list`, `library_add`, `library_remove` manage the local game list.

Tauri plugins: `dialog` (file picker), `opener` (open folder), `single-instance`, `updater`,
`log`, and `store` for non-secret preferences. Secrets go to the OS keychain via the `keyring`
crate, never to JSON.

## Security requirements

- Never persist the account password. Store the session token in the OS keychain and re-login when
  it expires. "Remember me" means the token, not the password.
- Do not pass credentials to the game through world-readable environment variables if a safer
  channel can be added. 0.4 minimum: keep env vars for the current DLL, document the exposure, and
  add a short-lived per-launch token file (mode 0600) as the preferred path in 0.5.
- Validate and canonicalize every path before writing. Never write outside the game directory.
- Back up the original library before replacing it. Refuse to overwrite a backup that is not ours.
- Disable remote URLs in the webview, set a strict CSP, and declare only the capabilities each
  window needs.

## Screens

Library is the home screen. Everything else is reachable from it.

1. **Sign in** (first run and when the token is invalid)
   - Server `host:port`, username, password. Real login call with a visible pending state.
   - Errors are specific: bad host, unreachable, wrong credentials, server too old.
   - Remember me stores the token in the keychain.
2. **Library** (home)
   - Grid of game tiles. Each tile: name, a user-chosen image or an honest monogram tile derived
     from the appid, install state, and a Play action.
   - Search, filter by installed/not installed, sort by name or last played.
   - Empty state: one clear action, "Add a game".
   - Offline banner when the server is unreachable. Play can still work from cached config.
3. **Add game** (modal, not a route)
   - Pick executable, then show detected arch, era, appid and interface count before writing.
   - Appid is editable and prefilled only for known executables. Do not guess silently.
   - One primary action, "Add to library", that installs the library and writes config in one
     transaction with progress. Advanced options (LD_PRELOAD, arguments, SteamStub unpack) are
     collapsed by default.
4. **Game details** (side panel or route from a tile)
   - Path, arch, era, appid, interface list, installed library, original backup state.
   - Actions: Play, Repair (reinstall library), Restore original, Remove from library, Open folder.
   - Per-game launch arguments and Linux preload override.
5. **Settings**
   - Account, server, theme, update channel, launcher version, log location.
   - Sign out and revoke the local token.
6. **Logs** (drawer)
   - Human-readable activity with levels. Copy-to-clipboard. No raw password, ever.

Do not add friends, chat or a store to the launcher in 0.4. The LAN web UI owns the social surface.

## Interaction rules

- One primary action per view. Play is the primary action on a configured game tile.
- No forced linear wizard. A returning user launches in one click.
- Every long operation shows progress and can be cancelled where safe.
- No dead controls. If a feature is not built, it is not shown. "Coming soon" is not allowed on a
  button that looks enabled.
- Keyboard first: tab order follows visual order, Esc closes modals, Enter activates the focused
  control.
- Fail loudly and specifically. "Something went wrong" is not an error message.

## Visual direction

Reading this as: a native game library for players who already own their games, in a quiet dark
instrument-panel language, dial ENERGY 2 / RHYTHM 2 / MOTION 2. Supersedes the 0.2 launcher look.

- Reuse the shared tokens in `design/tokens.css`. The launcher is the same product as the web UI.
- Superseded: the spec asked for light and dark with OS following. The shipped decision is dark only,
  with the reason in `design/DESIGN.md`. There is no theme toggle and no light-token set.
- The ember accent marks play actions, focus and live state. It is not a background wash.
- Cover images are user-provided or an honest monogram tile. Never ship invented game art.
- Glass is not used. A native app with a crisp surface reads better than a fake acrylic panel.
- Motion: library enter, modal open, tile hover. No looping pulse, no per-card cascade.
- Identity: the monogram tile and the ember play affordance are the recurring motif.

## Required states

Every data surface ships with empty, loading and error. The library also needs: server offline,
game missing from disk, library not installed, install in progress, launch failed. The sign-in
screen needs: checking, bad host, unreachable, unauthorized, success.

## Acceptance criteria for 0.4 launcher

- A first-run user can sign in, add one game and launch it without reading docs.
- A returning user launches a configured game in one click from the library.
- The password is never written to disk or to the launcher log.
- Removing the launcher's library file restores the original game library.
- The shipped dark theme must pass WCAG AA contrast at every text size, and keyboard-only use must
  work. Neither check is signed off here; screen-reader verification is also UNVERIFIED headless.
- `cargo test`, `cargo clippy`, `tsc --noEmit` and the Vitest suite pass in CI.
- A Windows build and a Linux AppImage must be produced by CI and attached to the release. This
  acceptance criterion is UNVERIFIED; no release run has been confirmed.

## Migration

1. Add the Tauri app alongside the egui app and reach parity behind a branch.
2. Keep `cargo test` for the detection logic; reuse those tests.
3. Ship the Tauri launcher in 0.4, keep the egui binary out of the release.
4. Delete the egui source and rename `client/src-tauri` to `launcher/` only after 0.4 is tagged and
   the release bundles are verified.

---

# 0.4 priorities

The executable plan, with M stages, per-phase assignments, risk tiers and the full change inventory,
is in `plan.md` section 12. This section is the summary, not the source of truth.

1. **Prove the core.** A live two-client C++ to Go session and a real MW2 run. This is the gate
   carried since 0.1. Nothing else can be called done before it.
2. **Rebuild the launcher** per this file. Real login, library, one-click play, no plaintext
   password, restore flow.
3. **Security pass.** TLS guidance and self-signed setup for the web UI, token handling, keychain,
   Docker web UI, logout and token revoke.
4. **Social completeness.** Add-friend and remove-friend API and UI are present in the tree; the
   LAN web UI owns that surface. The old advice to edit SQLite by hand is stale.
5. **Release hygiene.** One CI workflow, version stamping from the tag, changelog, container with
   `web/dist` and port 8081.

# 0.5 direction

The live 0.5 plan is `plan.md` section 13: M22 to M30, four gates G1 to G4, the full change
inventory and the release definition. This section is the summary, not the source of truth.

0.5 gives SAPIE an identity and two launchers.

- **Identity.** Name, mark, palette, type, voice and motion in `design/identity.md`. Quiet violet
  accent `#C5A4FF`, dark neutrals from `AGENTS-UI.md`, Manrope for UI, IBM Plex Mono for machine
  values. No glass, no ember brand accent, no fake art. The ember is retired as identity and may
  survive only as a warning status.
- **Two products, one core.** `SAPIE for Windows` and `SAPIE for Linux`. Shared `launcher/core`
  (Rust, C ABI), shared `launcher/ui` (Qt Quick QML), separate platform code for integration,
  packaging and updates.
- **No webview.** The 0.4 Tauri shell is replaced. The launchers are Qt 6 applications rendered by
  the Qt scene graph. No WebView2, no WebKitGTK, no HTML or JS in the launcher. The only browser
  surface is the LAN web UI.
- **Why not WinUI 3 plus GTK4.** That doubles the UI, the tests and the maintenance. One Qt Quick
  codebase reaches both desktops. Native feel comes from dialogs, tray, protocol handler and
  installers, not a second toolkit.
- **Windows:** Qt 6.8 + MSVC, Credential Manager, Authenticode, `windeployqt`, NSIS per-user and MSI
  per-machine, portable zip, Steamless sidecar, owner-only token ACL.
- **Linux:** Qt 6.8, bundled Qt in AppImage, deb and Flatpak (`org.kde.Platform`), Secret Service,
  AppStream, desktop entry, `x-scheme-handler/sapie`, `0600` token file, LD_PRELOAD.
- **Licensing:** Qt is LGPLv3 with dynamic linking. Ship the libraries, notices and the relink
  instructions. No static Qt and no non-LGPL Qt module without review.
- **Bundling:** `SAPIE-<version>-<os>-<arch>-<kind>.<ext>`, signed update manifests, stable and beta
  channels, one `SHA256SUMS`, one SBOM per family.
- **Security debts from 0.4 are must-fix,** not features: token rotation and revoke, persistent
  limiters, block list enforced in lobby routing, Windows token ACL, TLS guidance.
- **Gates.** G1 identity, G2 platform, G3 core, G4 release. A gate needs recorded evidence and the
  user's sign-off. Do not start production UI before G1.

`client/src-tauri` is the 0.4 artifact. The 0.5 Rust core and Qt Quick UI are in `launcher/`, and the
launcher commands above target that tree. The Tauri shell is not used by 0.5. No 0.5 tag, release or
artifact exists yet; the documented upgrade path is `docs/UPGRADE.md`.

# Definition of done

A change is done when the code builds, tests pass, the affected UI state was exercised (not just
compiled), the docs that mention the change are updated, and no known security or data-loss issue
is left unstated. State what was not verified. "Code-complete" and "verified" are different words.

## Agent workflow

Chief thread coordinates; worker subagents implement in isolated worktrees. Keep the registry in
`.sapie-state/`, cap 2 to 3 live workers, and check in on milestones, not timers. Risk tiers:
green for docs, CI and UI polish; yellow for transports and UI behavior; red for proto, schema,
auth, tokens and stats. Red work and anything merged to `main-client` needs the user's approval of
the current head. Reviews: `code-review-and-quality`, `ponytail-review`, `security-audit` for auth
and proto, `accessibility` and `webapp-testing` for UI.
