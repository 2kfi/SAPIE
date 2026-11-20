# Changelog

This is a draft for 0.4. No `0.4` tag, GitHub Release or container publish has happened. Items below
are changes in this tree since the `0.3` tag. "Shipped in code" means the change is present and
covered by the checks that ran locally or in CI; it does not mean a release artifact or a real game
run was verified. The status of each gate is in `docs/verification/0.4-rc-checklist.md`.

## Launcher

- Replaced the egui launcher with a Tauri v2 shell in `client/src-tauri/src/` and a bundled React 18
  UI in `client/src-tauri/ui/`. The directory keeps its historical name. No egui source remains.
- Added the library home: add a game, review detected arch, era and appid, install the matching
  library with an original backup, then press Play from a tile. Game details offer repair, restore
  original and remove, and display launch arguments and a Linux preload override.
- Added real sign in through `POST /api/v1/login`. The session token goes to the OS keychain through
  the `keyring` crate. The account password is never persisted.
- Added a dark-only launcher with an activity drawer, settings and copyable log. There is no theme
  toggle and no light theme; the decision is recorded in `design/DESIGN.md`.

## Game authentication and protocol

- Added `session_token` to `LoginRequest`. The server resolves an unexpired session hash and derives
  identity from it, while `source_id` stamping in `router.Route` stays authoritative.
- The launcher writes `steam_settings/sapie_client.json` with mode 0600 on Unix immediately before
  spawn, holds only the session token, and removes it on spawn failure. The DLL reads it once,
  deletes it and rejects files older than 10 minutes. Fallback order is the token file, then
  `SAPIE_SESSION_TOKEN`, then `SAPIE_USERNAME` and `SAPIE_PASSWORD`. No password is read from the file
  and neither credential is logged. Windows has no explicit ACL change.
- Fixed a two-client harness lobby-join failure and a teardown crash in the C++ client. No server,
  proto or schema source changed for those fixes.

## Social and web UI

- Added the social API: friend requests with accept, decline and cancel, remove, block and unblock,
  blocked list, username search, logout and session revoke. Friend graph changes push a fresh
  snapshot over the game socket. Rate limits and idempotency keys are documented in `docs/API.md`.
- Rebuilt the LAN web UI around those routes with friends, requests and blocked views, sign out and
  session expiry handling. The web UI is dark only and keeps the session in memory, not in
  `localStorage`.
- Kept admin paths on loopback 8080. Port 8081 proxies only user routes and returns 404 for admin
  paths.

## Server and security

- Added per-IP and per-account game-login limits, idle and frame handling, and a narrow allowlist on
  the web UI proxy. Locked game logins return `BAD_CREDENTIALS` because the wire enum has no lockout
  status.
- Added `docs/SECURITY.md` with the trust boundary, LAN cleartext, token-file and env-var exposure,
  admin token handling, replay risk and an operator TLS path with a self-signed example.
- Hardened the launcher webview boundary: bundled UI only, strict CSP, asset protocol disabled, no
  remote URLs and a capability limited to progress events.
- Container now builds `web/dist`, serves it on 8081 as a non-root user, publishes 8081 on host
  loopback and has a healthcheck.

## CI and release

- Split workflows into `ci.yml` for pushes and pull requests and `build.yml` for plain numeric `0.*`
  tags. `build.yml` stamps the tag into the launcher and web manifests.
- Added launcher Rust and launcher UI checks, the 10-target client matrix, proto sync, a tracked
  build-product guard and the `govulncheck`, `cargo audit` and `npm audit` job. Accepted npm advisory
  IDs are listed in the workflow and fail on any new advisory.

## Documentation

- Refreshed `README.md`, `AGENTS.md`, `dev_notes.md`, `docs/QUICKSTART.md`, `docs/BUILD.md`,
  `docs/PROTOCOL.md`, `docs/UI.md` and `docs/SECURITY.md`. `web/DESIGN.md` and `design/DESIGN.md`
  were reviewed and left unchanged.
- Added `docs/CONTRIBUTING.md`, this changelog and the draft `docs/verification/0.4-rc-checklist.md`.
- Added milestone status to `plan.md` without changing any decision.

## UNVERIFIED

These have no passing evidence in this tree. They are not release gates that have been cleared.

- UNVERIFIED: real MW2 2009 run. The Windows runbook `docs/verification/0.4-morning-gate.md` was not
  executed. No in-game persona, achievement or stat was observed.
- UNVERIFIED: native GUI, real IPC, keychain access and webview navigation. No launcher window was
  opened in the headless evidence run.
- UNVERIFIED: NAT traversal, direct UDP and relay path selection with two real networks.
- UNVERIFIED: Windows bundles, cross-compiled execution and a clean Linux launcher build under the
  narrower pull-request dependency set.
- UNVERIFIED: tag-triggered CD. No `0.*` tag run of `build.yml` has been confirmed.
- UNVERIFIED: screen-reader pass and a full keyboard-only click-through on both UIs.
- UNVERIFIED: npm advisory remediation. Both UIs report six dev-tooling advisories each, accepted in
  CI only because the fix is a major Vitest bump. They are reported, not fixed here.
- UNVERIFIED: the `glib` 0.18.5 unsound warning from `cargo audit`. `cargo audit` exits 0 with the
  warning; the upgrade to 0.20.0 is a transitive change not made in this work.
- The two-client harness is not deterministic. Full passes are recorded, but a client-side local
  mirror timing race makes intermittent failures possible.
