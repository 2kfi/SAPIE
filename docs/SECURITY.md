# SAPIE 0.4 LAN security

This file describes what the code in this tree does at commit `3362471`. It is documentation only.
It adds no code, TLS, token file or container changes. Those are separate work lanes. A requirement
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
`server/internal/session/session.go`). Five failed logins from one source IP within a minute block
further attempts from that IP for the rest of that window (`server/internal/auth/limiter.go`).

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

## TLS for the web UI

The Go server serves the SPA over plain HTTP. On an untrusted LAN, session tokens cross 8081 in
cleartext. Terminate TLS in front of it. The server code does not need a change.

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
bytes, stored as SHA-256 digests, expire after 24 hours, and expired rows are deleted hourly
(`server/internal/api/api.go`, `server/cmd/sapie-server/main.go`). Server log lines contain no
session token or password. The exception is the admin token, printed once when generated.

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

Game handoff status: `launch_game` passes `SAPIE_USERNAME` and `SAPIE_SESSION_TOKEN` to the game and
never passes a password env var (`client/src-tauri/src/game.rs`, `launch`). The values are still
visible to other processes under the same OS account and in `/proc` on Linux. The DLL parser still
reads `SAPIE_USERNAME` and `SAPIE_PASSWORD` from the environment
(`client/dll/settings_parser.cpp:119`) and does not consume the token yet. M16 token consumption and
the short-lived per-launch token file `sapie_client.json`, mode `0600`, remain future work and are
not implemented in this tree.

## Webview boundary

`client/src-tauri/tauri.conf.json` bundles the UI from `../ui/dist` and sets a strict CSP with
`default-src 'self'`, no `object-src`, no `frame-src`, no `base-uri` and no `form-action`. The asset
protocol is disabled. The `devUrl` of `http://127.0.0.1:1420` is for development only. The updater
has no endpoints. The `main` capability declares no permissions
(`client/src-tauri/capabilities/default.json`). There is no remote URL in the production bundle.

The launcher is dark only. `design/DESIGN.md` records a user decision with no theme toggle and no
system-theme following. Do not claim a light theme or a second contrast pass.

## Docker web UI

Current tree: `Dockerfile` builds only the Go server, exposes 47584 and 8080 and does not copy
`web/dist`. `compose.yaml` publishes `47584` and `127.0.0.1:8080` and runs the admin listener on
`:8080` inside the container. Port 8081 is not published today. A separate lane owns the container
change.

Operator guidance for when the UI is containerized:

- Bind the UI listener inside the container to `0.0.0.0:8081` so a published port works. Publish it
  to `127.0.0.1:8081` on the host if a reverse proxy terminates TLS there.
- Keep admin routes off 8081. The allowlist in `server/internal/ui/ui.go` must stay narrow.
- Keep the admin API unpublished, or published only on host loopback. It carries account CRUD.
- TLS termination is the operator's job, using the reverse proxy described above. The Go server
  does not terminate TLS on 8081.

## Residual risks and unverified items

- No live two-client C++ to Go session and no real MW2 run has been performed. `docs/QUICKSTART.md`
  and `docs/PROTOCOL.md` state this. P2P, NAT traversal and the relay path are therefore unverified
  in game. Direct UDP is best effort and the relay has no dedicated rate limit.
- The native Tauri window, real IPC and real keychain access were not exercised headless. Webview
  navigation and keyboard-only paths were not exercised either.
- The login limiter and the social limiter are in memory and reset on server restart
  (`server/internal/auth/limiter.go`, `server/internal/api/social.go`). There is no per-account
  lockout.
- Blocked peers remain in lobby membership. `router.lobbyRecipients` returns all lobby members
  without a block filter, so a blocked peer can still receive lobby frames.
- A session token remains replayable for up to 24 hours after it leaks. There is no token rotation
  and no revoke-on-password-change path in this tree.
- The admin token is written to the server log once at first start. Treat first-run logs as
  sensitive.
- Dependency audits (`govulncheck`, `npm audit`, `cargo audit`) were not run for this document.
