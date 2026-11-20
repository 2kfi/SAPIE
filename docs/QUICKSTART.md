# SAPIE Quickstart

SAPIE is a non-commercial project for running old Steam games without the Steam client. A Go server provides the account, friend, lobby and stats endpoints, and a modified `steam_api` library in the game forwards calls to it. The launcher is a native Rust egui/eframe app; 0.2 also lets the Go server host a browser web UI on the LAN.

Peer-to-peer play is not available in this version.

This is frozen 0.1 text and stays unchanged until the user accepts G0. P2P exists in 0.2 code but is not released.

## Download

Get a 0.1 archive from the release at https://github.com/2kfi/SAPIE/releases/tag/0.1:

- `SAPIE-0.1-linux-x64.tar.gz`
- `SAPIE-0.1-linux-arm64.tar.gz`
- `SAPIE-0.1-windows-x64.zip`

These are the frozen 0.1 assets. There is no 0.2 archive yet; 0.2 is code-complete on `dev-client` but not tagged or released.

Unpack the archive for your operating system. It contains the launcher `sapie` (`sapie.exe` on
Windows), the server `sapie-server` (`sapie-server.exe` on Windows), `steam_api/<target>-<era>/`
libraries, and `QUICKSTART.md` at the archive root.

## Start the server

Run the server binary from the archive, or run it from a source checkout:

```sh
./sapie-server --listen=:47584 --admin-listen=127.0.0.1:8080 --db=sapie.db --admin-token=<your-token>
```

From source, use `go run ./cmd/sapie-server` with the same flags from the `server/` directory.

With Docker, set the token in the environment and run Compose from the repository root
(`compose.yaml` lives there):

```sh
SAPIE_ADMIN_TOKEN=<your-token> docker compose up --build
```

The admin API is reachable only from the host at `http://127.0.0.1:8080`. Pass the same value as a `Bearer` token. If `--admin-token` is empty, the server generates a token and withholds its value. The game protocol listens on TCP port 47584.

From 0.2 source the server also takes `--ui-listen 0.0.0.0:8081` (default), `--ui-dir ./web/dist` and `--allow-register` (off by default). Port 47584 is the binary protobuf game protocol, not HTTP; a browser pointed at it shows a connection reset, which is expected. The web UI on 8081 serves the SPA and proxies only user API routes (config, register, login, friends, invite, lobbies, join, own stats). Admin-only paths such as `/api/v1/users` and account CRUD return 404 on 8081 and still work on 8080. Plain HTTP on the LAN means session tokens cross it in cleartext.

## Web UI (0.2 source)

With `--ui-listen` enabled, open `http://<server-host>:8081` in a browser. Login uses `POST /api/v1/login`; registration is offered only when `GET /api/v1/config` reports `allow_register: true`, which requires `--allow-register`. The SPA uses a relative `/api/v1` base when served this way. No browser click-through has been done yet.

## Game setup

Set `SAPIE_USERNAME` and `SAPIE_PASSWORD` in the environment where the game runs. Without both, the client falls back to Goldberg LAN mode.

Put `host:port` in `sapie_server.txt` next to the game executable. For a server on the same machine that is `127.0.0.1:47584`. Put the game's appid in `steam_appid.txt` on its own line. Sample files are in the archive's `samples/` directory (and in `docs/samples/` in the source tree).

Use the native launcher's file picker to copy the matching `steam_api` DLL or shared object next to the game executable, or copy the file there manually. The launcher is an egui/eframe app: no webview, localhost page or terminal window is involved.

## Steamless

The launcher can unpack SteamStub-packed Windows executables. Steamless is an unmodified Windows-only sidecar. Its NOTICE is at `steamless/NOTICE.md` inside the Windows archive; in a source checkout the same file is `client/src-tauri/resources/steamless/NOTICE.md`. Do not modify it.
