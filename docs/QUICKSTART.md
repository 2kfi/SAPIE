# SAPIE Quickstart

SAPIE is a non-commercial project for running old Steam games without the Steam client. A Go server
provides accounts, friends, lobbies and stats; a modified `steam_api` library in the game forwards
calls to it. The launcher is a native Rust egui/eframe app, and the server also hosts a browser UI on
the LAN.

Friends can play directly with each other, and the server relays traffic when a direct connection is not possible.

This describes the direct P2P path with a server relay fallback. It has not been verified in a live two-client game.

## Download

Get a 0.2 archive from the [SAPIE 0.2 release](https://github.com/2kfi/SAPIE/releases/tag/0.2):

- `SAPIE-0.2-linux-x64.tar.gz`
- `SAPIE-0.2-linux-arm64.tar.gz`
- `SAPIE-0.2-windows-x64.zip`

The frozen 0.1 assets remain at https://github.com/2kfi/SAPIE/releases/tag/0.1. Unpack the archive
for your operating system: it contains the launcher `sapie` (`sapie.exe` on Windows), the server
`sapie-server` (`sapie-server.exe` on Windows), `steam_api/<target>-<era>/` libraries, `web/dist/`,
and this file at the archive root.

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

There is no add-friend endpoint yet. Stop the server and link the two accounts in the database, in
both directions:

```sh
sqlite3 sapie.db "INSERT OR IGNORE INTO friendships(steam_id,friend_id) VALUES (<A>,<B>), (<B>,<A>);"
```

Restart the server afterward. Friends lists, invites, and friend-gated P2P need this row.

## Game setup

Set `SAPIE_USERNAME` and `SAPIE_PASSWORD` in the environment where the game runs. Without both, the client falls back to Goldberg LAN mode.

Put `host:port` in `sapie_server.txt` next to the game executable. For a server on the same machine that is `127.0.0.1:47584`. Put the game's appid in `steam_appid.txt` on its own line. Sample files are in the archive's `samples/` directory (and in `docs/samples/` in the source tree).

Use the native launcher's file picker to copy the matching `steam_api` DLL or shared object next to the game executable, or copy the file there manually. The launcher is an egui/eframe app: no webview, localhost page or terminal window is involved.

## Steamless

The launcher can unpack SteamStub-packed Windows executables. Steamless is an unmodified Windows-only sidecar. Its NOTICE is at `steamless/NOTICE.md` inside the Windows archive; in a source checkout the same file is `client/src-tauri/resources/steamless/NOTICE.md`. Do not modify it.
