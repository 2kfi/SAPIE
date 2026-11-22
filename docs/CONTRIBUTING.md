# Contributing to SAPIE

SAPIE is a non-commercial Steam API replacement for old games you already own. This file covers the
branch model, the commands that run in this tree and the standard for calling work done.

## Branch model

- `dev-client` is the default branch and the integration trunk. Open pull requests against it.
- Release pull requests go from `dev-client` to `main-client`. Do not push directly to
  `main-client`.
- Do not create or push tags as part of ordinary work. Tag pushes start the release workflow.
- Work that changes protobuf, schema, auth, tokens or stats is red tier and needs the user's approval
  of the current head. Docs, CI and UI polish are green tier.

Python tooling runs inside a virtual environment; see `note.md`. Never commit build products. CI
rejects tracked files under `out/`, `build/`, `web/dist/`, `third_party/` and
`client/src-tauri/target/`, and rejects tracked binaries and object files outside the Steamless
sidecar.

## Commands

Run these from the repository root unless a line says otherwise.

```sh
# Go server
cd server && go test ./... && go vet ./...
cd server && go run ./cmd/sapie-server

# Launcher Rust core
cd launcher && cargo test
cd launcher && cargo clippy --all-targets -- -D warnings

# Launcher Qt configure and QML tests
cmake -S launcher -B launcher/build
QT_QPA_PLATFORM=offscreen /usr/lib/qt6/bin/qmltestrunner -input launcher/ui/screens/tst_screens.qml

# LAN web UI
cd web && npm install && npm run build && npm test

# Protobuf copies must match
bash tools/check-proto-sync.sh

# Container (publishes the web UI on 127.0.0.1:8081)
docker compose up --build
```

`npm run build` runs `tsc --noEmit` then `vite build` for the web SPA. The 0.5 launcher loads its Qt
Quick UI from `launcher/ui` and needs Qt 6.8, CMake, MSVC or GCC and the Rust toolchain. The 0.4
Tauri webview dependencies (`libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev`) are not used by
the 0.5 launcher. See `docs/BUILD.md` for the build matrix.

## What CI runs

- `.github/workflows/ci.yml` on pushes to `dev-client`, pull requests to `dev-client` and
  `main-client`, and manual dispatch: proto sync, tracked-build-product checks, the 10-target client
  matrix, server tests and builds, launcher Rust checks and build, launcher UI and web builds and
  tests, and the dependency audits.
- `.github/workflows/build.yml` on pushed tags matching plain numeric `0.*`: version stamping,
  release builds, archives, GitHub Release and the server container. No tag-triggered run has been
  confirmed in this tree.

## Standards for changes

- The client is the trust boundary. The server never trusts a client-supplied SteamID; it overwrites
  `source_id` from the authenticated connection.
- No fake data in a UI: no invented players, stats, testimonials, game art or claims. A placeholder
  must look like a placeholder.
- Antislop rules apply to every UI surface. No em dash character in copy, no dead buttons, no nav
  item without a real destination, and complete empty, loading and error states.
- The Goldberg fork stays LGPLv3. Keep notices and publish modifications. Do not modify Steamless.
- Do not add a dependency for a few lines. Use the standard library or an existing dependency.
- Fix the root cause once in shared code; do not patch callers.
- Never persist an account password. The launcher keeps a session token in the OS keychain and hands
  it to the game through a 0600 token file with environment fallback.

## Definition of done

A change is done when the code builds, tests pass, the affected UI state was exercised and not just
compiled, the docs that mention the change are updated, and no known security or data-loss issue is
left unstated. State what was not verified. "Code-complete" and "verified" are different words.

Evidence for 0.4 and 0.5 lives in `docs/verification/`. The headless two-client record is
`0.4-two-client.md`. The 0.5 records are `0.5-identity-gate.md`, `0.5-platform-gate.md`,
`0.5-core-gate.md` and `0.5-rc-checklist.md`. None of the 0.5 gates is signed off. The upgrade path
is in `docs/UPGRADE.md`.
