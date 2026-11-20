# SAPIE

**Old games. No Steam client. Your own server.**

SAPIE is a self-hosted Steam API replacement for the games you already own. A Go
server answers the calls those games make, a forked Goldberg DLL forwards them
from inside the game, and one React UI runs in both the launcher and the server
web view.

## Why

Some old games still play fine. Their launcher does not. SAPIE gives those games
the small set of Steam services they actually ask for: identity, friends, lobbies
and achievements. You host it. You keep the saves and the unlock times.

## What 0.1 does

- Issues SteamIDs from your own account list, not from Steam.
- Shows friends online or offline and carries invites with a real connect string.
- Creates and joins lobbies that both clients and the web UI can see.
- Unlocks achievements once, keeps the first unlock time, and shows it to admins.
- Ships one UI for the launcher and the server.

## The fine print

- Peer-to-peer play is not available in this version.
- Non-commercial, for gamers by gamers.
- CI is green across all targets, but no live two-client game run has been done yet.
- Steamless stays an unmodified Windows-only sidecar under its own NOTICE.

## Start in a minute

```sh
SAPIE_ADMIN_TOKEN=<your-token> docker compose up --build
```

The game protocol listens on TCP `47584`. The admin API stays on host loopback at
`127.0.0.1:8080`. For the release archives and game-side config, read
[`docs/QUICKSTART.md`](docs/QUICKSTART.md).

## Layout

- `proto/` — canonical wire contract
- `client/` — Goldberg fork, 10 DLL/SO targets, Tauri launcher
- `server/` — Go server and pure-Go SQLite store
- `web/` — shared React UI
- `tools/` — harness, proto-sync check, release bundler
- `docs/` — build, protocol and quickstart guides

## Branches

`dev-client` is the default integration branch. `dev-server` mirrors it.
`main-client` and `main-server` are the release mirrors, currently at `0.1`.

## Build

See [`docs/BUILD.md`](docs/BUILD.md). Server: Go 1.26. Web: Node 20. Launcher:
Rust plus Tauri system libraries. Every tag builds client, launcher and server,
assembles OS archives, and pushes `ghcr.io/2kfi/sapie-server:<tag>`.

## License

Client fork based on the Goldberg Emulator (LGPLv3). See `LICENSE`.
Steamless is included unmodified under
`client/src-tauri/resources/steamless/NOTICE.md`.
