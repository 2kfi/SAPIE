# SAPIE 0.4 LAN security

This file describes the shipping code paths in this tree. It is documentation only. A requirement
that is not yet implemented is labeled as such.

## Listeners and what can reach them

| Listener | Default bind | Auth | Exposure |
| --- | --- | --- | --- |
| Game protocol | `:47584` (all interfaces) | Username and password in the first protobuf frame | Binary TCP, not HTTP |
| Admin HTTP API | `127.0.0.1:8080` | Bearer admin token | Account CRUD, ticket verify, user list |
| Web UI | `0.0.0.0:8081` | Bearer session token on user routes | SPA plus a narrow user-API proxy |

Source: `server/cmd/sapie-server/main.go`. Defaults are `--port`, `--admin-listen`, `--ui-listen`.
`--tls-cert` and `--tls-key` wrap only the game listener, with TLS 1.2 as the minimum. They do not
give the web UI or the admin API HTTPS.

The game socket caps a frame at 1 MiB, requires login within 10 seconds, pings every 30 seconds and
closes a connection after 90 seconds without a frame (`server/internal/protocol/frame.go`,
`server/internal/session/session.go`). Five failed logins from one source IP or one account within
a minute block further attempts for the rest of that window. The counters persist in
`rate_events` and survive a server restart; expired rows are cleaned at startup and hourly
(`server/internal/auth/limiter.go`, `server/migrations/003_security.sql`). A locked game login
returns `LoginResponse{status: RATE_LIMITED}`.

The admin token is generated on first start, stored in `sapie.db` and printed once in the server
log. An empty configured token disables admin access rather than opening it. The admin API listens
on loopback by default. The web UI proxy in `server/internal/ui/ui.go` forwards only an explicit
allowlist of user routes, so admin paths such as `/api/v1/users` and account CRUD return 404 on
8081. CORS allows only `localhost` and `127.0.0.1` origins over `http` or `https`
(`server/internal/api/api.go`, `allowedOrigin`).

## Trust boundary

The client is not trusted for identity. `router.Route` is the single place that assigns
`Common_Message.source_id`, and it sets it from the authenticated connection
(`server/internal/router/router.go:39`). Every game frame reaches that function through
`session.handleSteam`. For P2P signaling, the server clears a client-supplied IP and port, then
stamps `source_ip` from the authenticated TCP peer when it has an IPv4 address. A client-declared
UDP port is advisory only.

`appid` is a label for stats, presence, lobbies and tickets. There is no ownership table and no
Steam entitlement check. A ticket verifies issuance to a SteamID and appid, not ownership, and is
neither one-time nor bound to a peer (`docs/PROTOCOL.md`).

An attacker on the LAN can:

- Reach the web UI on 8081, try login, and register only when `--allow-register` is on.
- Read a session token from plain HTTP on 8081 and replay it until expiry or revocation.
- Use a valid session token for user routes: friends, invites, lobbies, own stats, sessions.
- Try passwords on the game port, subject to the per-IP failure limiter.
- Observe unencrypted game traffic unless the game listener was started with TLS flags.

An attacker on the LAN cannot:

- Reach admin routes through 8081. They return 404 there.
- Forge `source_id` or `source_ip` for routed frames.
- Read another account's stats with a user token. `userStats` returns 401 for a different SteamID.
- Enumerate accounts through friend requests easily. Unknown username, self, existing friend and
  blocked pairs all return 404. Search takes 2 to 64 characters, returns at most 10 results and
  is limited to 30 calls per hour per account.
- Recover a token from the session list. It shows a 12-character hash prefix only
  (`server/internal/state/sessions.go`). Account list responses omit password hashes
  (`server/internal/state/state.go`, `Account` versus `AuthAccount`).
- Join a lobby containing a blocked peer. Join checks both the owner and existing members
  (`server/internal/state/state.go`). Lobby and signaling fan-out skip peers blocked with the
  sender (`server/internal/router/router.go`).

## TLS for the web UI

The Go server serves the SPA over plain HTTP. On an untrusted LAN, session tokens cross 8081 in
cleartext. Terminate TLS in front of it. The server code does not need a change. The web UI shows a
fixed notice at the top of every page when `window.location.protocol` is `http:`
(`web/src/components/PlainHttpNotice.tsx`). It names cleartext session tokens, has no dismiss control
and does not cover sign in. It warns without blocking HTTP use.

1. Bind the UI to loopback so the plain port is not on the LAN:
   `sapie-server --ui-listen 127.0.0.1:8081`.
2. Run a reverse proxy on the LAN address, forwarding to `127.0.0.1:8081` and passing the
   `Authorization` header through. Tokens travel in headers, never in URLs
   (`web/src/api.ts`, `request`).
3. Keep the admin listener on `127.0.0.1:8080`. Do not proxy admin paths.

A self-signed certificate for a home LAN, using OpenSSL:

```sh
mkdir -p /etc/sapie/tls
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout /etc/sapie/tls/key.pem -out /etc/sapie/tls/cert.pem \
  -subj "/CN=sapie-home" \
  -addext "subjectAltName=DNS:sapie-home,IP:192.168.1.10"
```

A matching nginx server block:

```nginx
server {
    listen 443 ssl;
    server_name sapie-home;
    ssl_certificate /etc/sapie/tls/cert.pem;
    ssl_certificate_key /etc/sapie/tls/key.pem;
    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

Warning: a self-signed certificate is not trusted by browsers until it is installed on each
client. Until then the browser shows a warning. Even after trust is installed, it proves only what
the person who distributed the certificate chose. Keep it on a home LAN, put the hostname or IP
actually used in the SAN, and protect the private key. Do not expose this setup to the internet.

## Secrets

Passwords are stored as self-describing argon2id hashes with `m=65536,t=3,p=4`
(`server/internal/auth/auth.go`). No read endpoint returns a hash. Session tokens are 32 random
bytes, stored as SHA-256 digests and expire after 24 hours. A game or HTTP login that presents a
token rotates it: the old token is deleted, the replacement keeps the original expiry cap, and a
replay fails immediately. A password change revokes every other session. Expired rows are deleted
hourly (`server/internal/api/api.go`, `server/internal/state/sessions.go`,
`server/cmd/sapie-server/main.go`). Server log lines contain no session token or password. The
exception is the admin token, printed once when generated.

The web UI keeps the session in React state (`web/src/App.tsx`, `session` and `sessionRef`). Tests
assert that `localStorage` stays empty (`web/src/test/login.test.tsx`). Sign out calls
`POST /api/v1/logout`, which revokes the token server side, and clears local state. A 401 clears
local state and returns to sign in. `POST /logout-all` and `DELETE /sessions/{id}` revoke other
sessions.

Launcher status: the launcher has a real `login` command that calls `POST /api/v1/login`.
`KeychainTokenStore` stores the session token in the OS keychain through the `keyring` crate, and a
mockable `TokenStore` trait keeps the storage boundary testable (`client/src-tauri/src/auth.rs`).
The password is never persisted. `LauncherState` holds only non-secret metadata and scrubs legacy
`password`, `token`, `session_token` and `secret` keys when it loads a state file
(`client/src-tauri/src/state.rs`). `sign_out` clears the keychain entry
(`client/src-tauri/src/commands.rs`). Real keychain access is still UNVERIFIED headless. The `login`
command returns the token over Tauri IPC to the UI by the command contract (`SessionInfo.token`,
`client/src-tauri/ui/src/api.ts`). That crossing into UI memory is an accepted 0.4 boundary, not a
disk write.

Game handoff status (M16, shipped): `launch_game` writes `steam_settings/sapie_client.json` mode
`0600` on Unix immediately before spawn and passes no credential env vars
(`client/src-tauri/src/game.rs`). The
file holds only `session_token`; the launcher removes it if spawn fails. Windows has no explicit
ACL change, so the file inherits the game directory's default ACL. The DLL reads the file once,
removes it after the read attempt, rejects files older than 10 minutes, and sends the token in
`LoginRequest.session_token` (`client/dll/settings_parser.cpp`, `client/dll/network.cpp`). The
server resolves the token hash against unexpired sessions and rotates it
(`server/internal/session/session.go`). The shipped DLL does not adopt `LoginResponse.session_token`,
so its next reconnect needs a fresh token file from the launcher. When no token file exists, the DLL falls back to `SAPIE_SESSION_TOKEN`, then to
`SAPIE_USERNAME`/`SAPIE_PASSWORD` for compatibility. The launcher does not set those variables; the
fallback serves older setups that still do. Env values remain visible under the same OS account and
in `/proc` on Linux; no password is read from the token file, and neither credential is logged.

## Webview boundary

`client/src-tauri/tauri.conf.json` bundles the UI from `../ui/dist` and sets a strict CSP with
`default-src 'self'`, no `object-src`, no `frame-src`, no `base-uri` and no `form-action`. The asset
protocol is disabled. The `devUrl` of `http://127.0.0.1:1420` is for development only. The updater
has no endpoints. The `main` capability grants only `core:event:allow-listen` and
`core:event:allow-unlisten` so the UI can receive the M15 `sapie-progress` events
(`client/src-tauri/capabilities/default.json`, `client/src-tauri/ui/src/api.ts`). It declares no other
native permission. There is no remote URL in the production bundle.

The launcher is dark only. `design/DESIGN.md` records a user decision with no theme toggle and no
system-theme following. Do not claim a light theme or a second contrast pass.

## Docker web UI

Current tree: `Dockerfile` builds `web/dist` in a Node stage, copies it to `/app/web/dist`, runs the
Go server as a non-root user with `--ui-dir /app/web/dist --ui-listen 0.0.0.0:8081`, and exposes
47584 and 8081. `compose.yaml` publishes `47584`, `127.0.0.1:8080` and `127.0.0.1:8081`, keeps the
admin listener on `:8080` inside the container, mounts a data volume, and probes 8081 with a
healthcheck.

Operator guidance:

- Bind the UI listener inside the container to `0.0.0.0:8081` so a published port works. Publish it
  to `127.0.0.1:8081` on the host if a reverse proxy terminates TLS there.
- Keep admin routes off 8081. The allowlist in `server/internal/ui/ui.go` must stay narrow.
- Keep the admin API unpublished, or published only on host loopback. It carries account CRUD.
- TLS termination is the operator's job, using the reverse proxy described above. The Go server
  does not terminate TLS on 8081.

## Residual risks and unverified items

- A live two-client C++ to Go session is recorded in `docs/verification/0.4-two-client.md`. No real
  MW2 run has been performed. P2P, NAT traversal and the relay path are therefore unverified in
  game. Direct UDP is best effort and the relay has no dedicated rate limit. The two-client harness
  had a teardown-order race; it now waits for guest completion and five consecutive runs passed.
- The native Tauri window, real IPC and real keychain access were not exercised headless. Webview
  navigation and keyboard-only paths were not exercised either.
- The shipped DLL does not persist the rotated `LoginResponse.session_token`. A token-file game
  login consumes the file's token; the next reconnect needs a new file from the launcher. Client
  changes are outside this server-only work.
- Lobby membership rows are not rewritten when a block is added. Delivery and join checks filter
  blocked peers, but an existing row remains until the peer leaves or the lobby is deleted.
- The admin token is written to the server log once at first start. Treat first-run logs as
  sensitive.

## Dependency audits

CI runs all three audits in the `dependency-audits` job of `.github/workflows/ci.yml`. Dependabot
opens weekly updates for the Go module, npm in both UIs, Cargo and GitHub Actions
(`.github/dependabot.yml`).

Results from this tree on 2026-11-21:

- `govulncheck ./...` in `server/`: no vulnerabilities found.
- `cargo audit` in `client/src-tauri/`: exit 0, no vulnerabilities across 535 crate dependencies.
  Six warnings, all printed by the tool: `chrono` 0.4.45 (RUSTSEC-2026-0543, unmaintained),
  `proc-macro-error` 1.0.4 (RUSTSEC-2024-0370, unmaintained), `zbus_names` 3.0.0
  (RUSTSEC-2026-0371, unmaintained), `zvariant` 4.2.0 (RUSTSEC-2026-0370, unmaintained),
  `zvariant_derive` 4.2.0 (RUSTSEC-2026-0372, unmaintained) and `glib` 0.18.5
  (RUSTSEC-2024-0429, unsound, fixed in 0.20.0). The `glib` upgrade is a transitive dependency
  change, not made in this lane.
- `npm audit` in `web/` and `client/src-tauri/ui/`: six findings each, all in dev tooling that is not
  bundled into the shipped SPA or launcher UI. One high (`vite`, GHSA-587h-7wmj-h5pm plus seven
  other advisory IDs), two critical (`tinypool` GHSA-5gmw-xhrv-c9v3 and GHSA-85c8-ppgw-ccpr,
  `vitest` GHSA-5xrq-8626-4rwp and GHSA-82fw-gwwq-j7x9), three moderate (`@vitest/mocker`,
  `esbuild`, `vite-node`). npm only offers `vitest@4.1.12`, a major bump of both UI test stacks,
  which is outside this lane. The CI step prints every advisory on each run, accepts only the 14
  IDs listed in the workflow and fails on any new advisory. These findings are reported, not
  silently ignored.
