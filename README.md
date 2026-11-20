# SAPIE

SAPIE runs old Steam games without the Steam client. A Go server emulates Steam's
account, friend, lobby and stats services; a forked Goldberg emulator DLL forwards
Steam API calls to it; a Tauri launcher and the server's web UI share one React app.

## Status

Release `0.1` contains M1–M4: server-assigned SteamIDs, friends and presence,
game invites and connect strings, lobbies, and achievements/stats. The full build
and acceptance suite is green. A live two-client session against a real game has
**not** been run yet, so treat this as an early release.

Peer-to-peer play is not available in this version.

## Quick start

- Download the archive for your OS from the `0.1` Release and read
  [`docs/QUICKSTART.md`](docs/QUICKSTART.md).
- Or run the server from source with Docker:

  ```sh
  SAPIE_ADMIN_TOKEN=<your-token> docker compose up --build
  ```

The game protocol listens on TCP `47584`. The admin API binds to `127.0.0.1:8080`
by default; Compose publishes it on host loopback only.

## What works

- Login with server-issued SteamIDs and an admin-managed account list.
- Friend list with online/offline presence and game invites carrying a connect string.
- Lobby create/list/join, shared by both clients and visible in the web UI.
- Achievement unlocks pushed to friends and visible to admins with a canonical unlock time.
- One React UI served by the launcher and available in the server web view.

## What does not

- No P2P game-traffic relay. No `?state=` UI hooks or fabricated online states.
- Steamless is an unmodified Windows-only sidecar shipped under its own NOTICE.
- Older interface-era DLLs are supported, but real-game verification is still pending.

## Repository layout

- `proto/` — canonical wire contract (`net.proto` from Goldberg, `sapie.proto` envelope)
- `client/` — Goldberg fork, 10 DLL/SO build targets, and the Tauri launcher
- `server/` — Go server and pure-Go SQLite store
- `web/` — shared React UI
- `tools/` — harness, proto-sync check and release bundle script
- `docs/` — build, protocol and quickstart guides

## Branches

- `dev-client` — default integration branch
- `dev-server` — synced copy of the same tree
- `main-client`, `main-server` — release mirrors, currently at `0.1`

## Build

See [`docs/BUILD.md`](docs/BUILD.md). The server needs Go 1.26; the web app needs
Node 20; the launcher needs Rust and the Tauri system dependencies. CI builds the
client, launcher and server for each tag, assembles OS archives and publishes the
server image to `ghcr.io/2kfi/sapie-server:<tag>`.

## License

The client fork is based on the Goldberg Emulator (LGPLv3). See `LICENSE`.
Steamless is included as an unmodified sidecar under
`client/src-tauri/resources/steamless/NOTICE.md`.
