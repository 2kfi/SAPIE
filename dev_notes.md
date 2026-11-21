# SAPIE Development Notes

## Current known limits (0.4 release candidate, 2026-11-20)

This section is the live known-limits list. The sections below it are historical notes from 0.1 and
0.2 and are kept for context. Items marked UNVERIFIED have no passing evidence in this tree.

### Proven and not proven

- A headless two-client C++ to Go session is recorded in `docs/verification/0.4-two-client.md`. It
  covers login, server-assigned IDs, DB friendships made through the social API, presence, invite,
  a joined lobby with two members on both sides, a stats response and a persisted achievement.
- The two-client harness now waits for the guest to finish before host teardown. Five consecutive
  real-server runs passed with two-member rosters on both sides. The earlier intermittent failure
  was teardown order: the host left the lobby before the guest sampled the roster.
- UNVERIFIED: real MW2 2009 run. The Windows runbook is `docs/verification/0.4-morning-gate.md` and
  was not executed in the headless evidence run.
- UNVERIFIED: native Tauri window, real IPC, keychain access and webview navigation. No window was
  opened in the headless evidence run.
- UNVERIFIED: keyboard-only and screen-reader pass on the launcher and the web UI.
- UNVERIFIED: NAT traversal, direct UDP and relay path selection with two real networks.
- UNVERIFIED: installer execution and cross-compiled execution. Tag `0.4` CD run 44882984036
  produced Windows NSIS/MSI and Linux AppImage installers, but none were executed. Only native
  `linux-x64` modern and legacy DLLs were built and run in the evidence run.

### M19 security findings

The current threat model is `docs/SECURITY.md`. Its residual risks are:

- A session token remains replayable for up to 24 hours after it leaks. There is no token rotation
  and no revoke-on-password-change path.
- The login limiter, social limiter and per-account game-login lockout live in memory and reset on
  server restart. A locked game login returns `BAD_CREDENTIALS` because the wire enum has no lockout
  status.
- Blocked peers remain in lobby membership. `router.lobbyRecipients` does not filter blocks, so a
  blocked peer can still receive lobby frames.
- The admin token is printed once to the server log at first start. Treat first-run logs as
  sensitive.
- The web UI is plain HTTP on the LAN unless an operator terminates TLS in front of it, so session
  tokens cross 8081 in cleartext.
- The game token file is mode 0600 on Unix. Windows has no explicit ACL change, so it inherits the
  game directory default ACL. The DLL falls back to `SAPIE_SESSION_TOKEN`, then to
  `SAPIE_USERNAME`/`SAPIE_PASSWORD` when no file exists; env values are visible under the same OS
  account and in `/proc` on Linux. No password is read from the token file and neither credential is
  logged.
- Real keychain access is still UNVERIFIED headless.
- Dependency audits are reported, not silently ignored: `govulncheck` found no Go vulnerabilities,
  `cargo audit` exited 0 with six unmaintained/unsound warnings including `glib` 0.18.5, and both
  npm UIs report six dev-tooling advisories each. The CI job accepts only the advisory IDs named in
  `.github/workflows/ci.yml` and fails on any new one.

### Build alignment

`Cargo.lock` pulls `webkit2gtk` through Tauri. Both `.github/workflows/build.yml` and
`.github/workflows/ci.yml` now install the same Linux launcher dependency set, including
`libglib2.0-dev`, `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev` and `librsvg2-dev`. The
earlier gap where pull requests installed only the X11/Wayland/GL packages is closed. Green CI on a
head with this dependency set is pending a workflow run.

## Deferred items after MF1

Carried forward from the MR1 review and MF1 fixes at the 0.1 release at `62007f5`.
None of these block the release; they are known limits, not failures found in testing.

### 1. Mutable mirror refs

The C++ client's `friends()` and `lobbies()` return references to internal lists rather than
copies. In theory another thread could change a list while someone reads it. In practice every
caller is serialized behind a global lock, so there is no observed race. Fixing it properly means
rewriting several Steam interface methods to use copies, broad churn for a hypothetical problem.
Left as a labelled follow-up.

### 2. `StatsGet.names` ignored

The wire message has a field where a caller can say "only send me these named stats." The server
ignores that list and returns everything for that user/app. The result is a superset, so nothing
breaks today, but the field is dead plumbing, and asking for one stat still returns all of them.

### 3. Hidden achievements in unlock pushes

We fixed the *query* leak: a friend can no longer ask the server for your hidden achievements. But
when you actually unlock one, the server still pushes that unlock to friends. So a friend can learn
a hidden achievement's name and unlock time through the push, even though they cannot query it.
It is a smaller version of the privacy issue MF1 already closed.

### 4. No live end-to-end run

This is the real gap. Everything so far was proven in pieces: in-process Go harnesses, stubbed C++
smoke messages, isolated builds. We have not launched two real C++ clients against the real Go
server, run an actual old Steam game with the replacement DLL, or clicked through the web UI in a
browser. Any of those could expose an integration bug that none of our tests cover.

This is the first 0.2 gate: a live two-client C++↔Go session and a real MW2 run must happen before
the online surface or either P2P path can be called done.

Items 1–3 are small, understood limitations. Item 4 is the honest confidence gap: "nothing blocks
the merge" is not the same as "proven on a real machine." Main was merged from dev for the 0.1
release; see the release tag for the exact commit.

### Verification caveat

None of the four items were re-verified live; they come from worker reports and the merged-head test
suite. Main risk: the real two-client path stays unproven until item 4 is run.

## Carry-forward into 0.2

The 0.1 release at `62007f5` is published and frozen, and did ship the old UI text "Peer-to-peer
play is not available in this version." M12 replaced that line with the approved sentence; G0 no
longer gates the wording. 0.2 adds the following in the tagged `0.2` release, with G0 deferred to
the user:

- Full online persona surface: a game sees a logged-in account, persona name and online state, friends
  list and invites, achievements and stats, and a ticket issuance check. These are the checks a
  paranoid MW2-2009-style client probes; there is no app-ownership table.
- Deeper lobbies/matchmaking: join-friend and connect flows, implemented in code but not held for
  two real clients.
- P2P: direct client-to-client first, relay fallback for NAT-blocked peers. This reverses the old
  "No P2P game-traffic relay in v1" rule, superseded for 0.2.
- UI text rule: superseded in M12. The UI uses "Friends can play directly with each other, and the
  server relays traffic when a direct connection is not possible." The old 0.1 line is no longer
  current and G0 does not gate the wording.

Item 4 above is the first gate: run a live two-client C++↔Go session and a real MW2 game with the
replacement DLL before declaring the online surface or either P2P path done. It is PENDING USER TEST;
0.2 is released and G0 is deferred to the user.

## 0.2 M6 server persona surface

The three small 0.1 limits above are closed on the server, without a wire or schema change:

- `StatsGet.names` now limits the returned stats to the named set; empty names keep the old
  all-stats behaviour. Achievements are unaffected by `names`.
- Hidden achievements are no longer pushed to friends or lobby mates. The unlock is still
  stored canonically and stays visible to the owner through `StatsGet{steam_id: 0}` and to admin
  through the HTTP stats API. The push is skipped rather than redacted so clients never receive
  an empty or malformed definition.
- Tickets are recorded in the existing `auth_tickets` table with their nonce and a 10-minute
  expiry, and `POST /api/v1/tickets/verify` checks issuance for an admin. This is not Steam
  ownership verification, not redemption, and game clients never call it.

Still open: there is no per-account app-ownership table, so `appid` remains a filter/label and
any authenticated account may name any appid. Tickets are not one-time or bound to a peer.
Item 4 (live two-client C++↔Go session and a real MW2 run) is unchanged and remains the gate
before the online surface can be called done. None of these were verified against a real game.

## 0.2 status (tagged and released, G0 deferred)

0.2 is tagged `0.2` and published at https://github.com/2kfi/SAPIE/releases/tag/0.2. 0.1 stays
published and frozen at `62007f5`, with its assets and GHCR image unchanged.

Shipped in code, with local verification only:

- M6 native egui/eframe launcher (`sapie-launcher`), no webview, localhost dependency or terminal
  window; `windows_subsystem` on Windows.
- M7a server LAN web UI on `--ui-listen 0.0.0.0:8081` from `--ui-dir ./web/dist`, with a narrow
  reverse proxy for user API routes; admin paths stay on loopback 8080 and 404 on 8081.
- M7b browser login and opt-in registration gated by `GET /api/v1/config` and `--allow-register`
  (off by default). The SPA uses a relative `/api/v1` base in the browser.
- M8a/M8b persona completeness and safe client mirror. Tickets are recorded with a 10-minute
  expiry and checked by `POST /api/v1/tickets/verify` (admin). The client does not send
  `TicketRequest` or read a server ticket response; `encrypted_app_ticket` is still synthesized
  locally. There is no app-ownership table; appid is a filter/label, and tickets are not one-time
  or peer-bound.
- M9a/M9b lobby rules and client lobby flows.
- M10a/M10b direct P2P. The server stamps `source_ip` from the authenticated connection and
  `source_port` from the client-declared bound UDP port (field 129), never trusting a client IP;
  the client sends direct UDP via `sendToPeer()` when a hint exists. There is no hole punching,
  and IPv6 gets no hint.
- M11a/M11b relay fallback. Authorized `Networking_Sockets`/`Networking_Messages` DATA goes only
  to the explicit `dest_id` (friend/lobby gate, no broadcast). The client uses one path per frame,
  taking the relay when there is no hint or no inbound DATA within 5s. Direct stays preferred.
- M-INT CI builds `web/dist` as the `web-dist` artifact; release bundles place it at `web/dist` in
  the archive root so `--ui-dir ./web/dist` works. This docs refresh is part of it.

Local verification was `go test ./...`, `go vet ./...`, `go build ./cmd/sapie-server`, the Go
harness, client modern+legacy builds and `sapie_login_smoke`, plus the web build. None of that is
a live run.

Limits and risks: no live two-client C++↔Go session, no real MW2 2009 run, no browser click-through,
no screen-reader or keyboard audit, no cross-compiled Windows execution and no real NAT test. Direct
reliable DATA is best-effort over UDP. The 5s relay window is measured from connection creation, so
a slow handshake may pick the relay. There is no relay rate limit beyond existing per-session
handling. Plain HTTP on the LAN means session tokens cross it in cleartext.

The 0.1 P2P notice is superseded: M12 replaced it with the approved direct-P2P plus relay sentence,
and G0 no longer gates the wording. Live two-client verification is still pending.
