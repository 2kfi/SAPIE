# SAPIE 0.1

Old games, no Steam client, your own server. SAPIE is a non-commercial Steam API replacement for games you already own, for gamers by gamers.

## Get 0.1

Download one asset from the [SAPIE 0.1 release](https://github.com/2kfi/SAPIE/releases/tag/0.1):

- `SAPIE-0.1-linux-x64.tar.gz`
- `SAPIE-0.1-linux-arm64.tar.gz`
- `SAPIE-0.1-windows-x64.zip`

Or pull the container: `ghcr.io/2kfi/sapie-server:0.1` or `ghcr.io/2kfi/sapie-server:latest`. Both tags currently point to the same digest.

Each archive contains the launcher `sapie` (`sapie.exe` on Windows), the server `sapie-server` (`sapie-server.exe`), `steam_api/<target>-<era>/` with all 10 DLL/SO variants, and `QUICKSTART.md` at the root. `samples/sapie_server.txt` and `samples/steam_appid.txt` are included. The Windows zip also has `steamless/` with its own NOTICE.

## Quick start

Unpack an archive and start the server:

```sh
./sapie-server --listen=:47584 --admin-listen=127.0.0.1:8080 --db=sapie.db --admin-token=<your-token>
```

On Windows, run `sapie-server.exe` with the same flags. The game protocol listens on TCP port `47584`. The admin API stays on loopback at `127.0.0.1:8080`; send your token as a `Bearer` header.

Docker alternative, from the repo root where `compose.yaml` lives:

```sh
SAPIE_ADMIN_TOKEN=<your-token> docker compose up --build
```

## Configure your game

`QUICKSTART.md` ships at the root of every archive and is also at [`docs/QUICKSTART.md`](docs/QUICKSTART.md). It covers `sapie_server.txt` (use `127.0.0.1:47584` for a local server), `steam_appid.txt`, and setting `SAPIE_USERNAME` and `SAPIE_PASSWORD` in the game's environment. The launcher copies the right `steam_api` variant next to the game executable, or you can copy it yourself.

## What 0.1 does

- Issues SteamIDs from your own account list.
- Shows friends online or offline and carries invites with a connect string.
- Creates and joins lobbies visible to clients and the web UI.
- Unlocks achievements once, keeps the first unlock time, and shows it to admins.

## Fine print (0.1)

Peer-to-peer play is not available in this version.

Non-commercial, for gamers by gamers. CI is green across all targets and release assets were verified, but no live two-client C++↔Go session, no real-game run and no browser click-through has been done yet. Steamless is an unmodified Windows-only sidecar under its own NOTICE.

## Branches

`dev-client` is the default; `dev-server`, `main-client` and `main-server` are the others. All currently at 0.1.

## Building / CI

See [`docs/BUILD.md`](docs/BUILD.md) for toolchains, the client matrix and release plumbing.

## License

Goldberg fork, LGPLv3 (`LICENSE`). Steamless NOTICE: `client/src-tauri/resources/steamless/NOTICE.md`.
