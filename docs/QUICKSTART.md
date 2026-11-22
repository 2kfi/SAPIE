# SAPIE Quickstart

SAPIE is a non-commercial project for running old Steam games without the Steam client. A Go server
provides accounts, friends, lobbies and stats; a modified `steam_api` library in the game forwards
calls to it. The 0.5 launcher is a Qt 6 app with a Rust core and a Qt Quick UI, and the server also
hosts a browser UI on the LAN. The launcher uses no webview.

Friends can play directly with each other, and the server relays traffic when a direct connection is not possible.

This describes the direct P2P path with a server relay fallback. A headless two-client C++ to Go
session is recorded in `docs/verification/0.4-two-client.md`. A real game run and NAT path selection
are UNVERIFIED.

## Download

Get a release asset from the
[SAPIE releases page](https://github.com/2kfi/SAPIE/releases). Standalone launcher installers are
named `sapie-launcher-*`; the full server bundles and container are named `sapie-server-*`. The `0.4`
archive and tag `0.5` are published. See [Upgrade](UPGRADE.md) for the documented 0.4 to 0.5 path.

Unpack the server bundle for your operating system. It contains the launcher `sapie-launcher`
(`sapie-launcher.exe` on Windows), the server `sapie-server` (`sapie-server.exe` on Windows),
`steam_api/<target>-<era>/` libraries, `web/dist/`, and this file at the archive root. Older `0.1`
and `0.2` assets stay on their tags.

## Start the server

Run the server binary **from the archive root** so `./web/dist` resolves:

```sh
./sapie-server
```

No flags are needed. Defaults: game protocol on port 47584, admin API on 127.0.0.1:8080, web UI on
0.0.0.0:8081, assets from `./web/dist`, database `sapie.db`. Every flag has a default and only
overrides it, for example `--port 47585` moves the game listener, and `--allow-register` enables
public registration (off by default). Run once with `--allow-register` to create accounts, then
restart without it. From source, use `go run ./cmd/sapie-server` with the same flags from the
`server/` directory.

The admin token is generated on first start, printed once, and stored in `sapie.db` so it survives
restarts. Pass `--admin-token <token>` to override it for one run. With Docker, run Compose from the
repository root; the token appears in `docker compose logs`:

```sh
docker compose up --build
```

The admin API is reachable only from the host at `http://127.0.0.1:8080`; pass the token as a `Bearer` token. Port 47584 is the binary protobuf game protocol, not HTTP. A browser pointed at it shows a connection reset.

The web UI on 8081 serves the SPA and proxies only user API routes (config, register, login, friends, invite, lobbies, join, own stats). Admin-only paths such as `/api/v1/users` and account CRUD return 404 on 8081 and still work on 8080. Plain HTTP on the LAN means session tokens cross it in cleartext.

## Web UI

With `--ui-listen` enabled, open `http://<server-host>:8081` in a browser. Login uses `POST /api/v1/login`; registration is offered only when `GET /api/v1/config` reports `allow_register: true`, which requires `--allow-register`. The SPA uses a relative `/api/v1` base when served this way.

## Friends

Friend management is in the web UI. Open Friends, enter a username, send a request, and accept an
incoming one. Accepting writes both friendship directions and pushes a fresh snapshot to the game
socket. There is no SQLite step. Requests, remove, block and unblock are documented in `docs/API.md`.

## Game setup

Use the launcher. Sign in to the server, add the game executable, confirm the detected arch, era and
appid, then press Add to library. The launcher backs up the original library, copies the matching
`steam_api` file, writes the connection files and starts the game. The connection state is real; the
launcher calls `POST /api/v1/login` and stores only the session token in the OS keychain. The
password is never written to disk or to the log.

For a manual setup, put `host:port` in `sapie_server.txt` next to the executable (for a local server,
`127.0.0.1:47584`) and the appid in `steam_appid.txt` on its own line. Sample files are in the
archive's `samples/` directory and in `docs/samples/` in the source tree. Copy the matching library
by hand if needed.

The preferred handoff is a short-lived `steam_settings/sapie_client.json` file (mode 0600 on Unix)
holding only the session token. When no file exists, the client falls back to `SAPIE_SESSION_TOKEN`,
then to `SAPIE_USERNAME` and `SAPIE_PASSWORD` for older setups. Without any credential the client
falls back to Goldberg LAN mode. Environment values are visible to other processes under the same
OS account and in `/proc` on Linux. The DLL deletes the token file after one read and rejects files
older than 10 minutes.

## Two-peer NAT check

This is a manual check for two players on different networks. It needs two machines, two registered
accounts, one server both can reach, and a friend link or the same lobby.

1. Sign in on both machines and add each other as friends, or join the same lobby.
2. Record each peer's network type: full-cone, restricted-cone, symmetric NAT or carrier-grade NAT.
   If you do not know it, write "unknown" rather than guessing.
3. Start game traffic in both directions. Confirm both peers see each other move. That proves the
   connection works, not which path it used.
4. To claim a direct path, capture UDP on the game machine with Wireshark or `tcpdump` and show
   inbound packets from the other peer's public IP and port. A server IP in the capture is the relay
   path. A server hint alone is not proof of direct traffic.
5. Repeat with one peer on a mobile hotspot or another carrier-grade network. Punching is expected to
   fail there, so the server relay is the guaranteed path. Do not record a direct result on that run.

The launcher does not show the chosen path, and release builds do not write a debug log. Use the
capture as evidence. Record the network types, build commit and result for both runs in
`docs/verification/0.5-platform-gate.md`.

## Steamless

The launcher can unpack SteamStub-packed Windows executables. Steamless is an unmodified Windows-only sidecar. Its NOTICE is at `steamless/NOTICE.md` inside the Windows archive; in a source checkout the same file is `client/src-tauri/resources/steamless/NOTICE.md`. Do not modify it.
