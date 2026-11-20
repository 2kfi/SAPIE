# SAPIE

Old Steam games, no Steam client, your own server. SAPIE is a non-commercial Steam API replacement
for games you already own, for gamers by gamers.

## Quick start

Download an archive from the [0.2 release](https://github.com/2kfi/SAPIE/releases/tag/0.2), unpack it,
and start the server from the unpacked directory:

```sh
./sapie-server \
  --listen=:47584 \
  --admin-listen=127.0.0.1:8080 \
  --ui-listen=0.0.0.0:8081 \
  --ui-dir=./web/dist \
  --admin-token=<your-token> \
  --allow-register \
  --db=sapie.db
```

Open `http://<server-ip>:8081`, register two accounts, and log in. Then, next to each game executable:

- `sapie_server.txt` containing `<server-ip>:47584`
- `steam_appid.txt` containing the game's appid
- `SAPIE_USERNAME` and `SAPIE_PASSWORD` in the game's environment

Copy the matching library from `steam_api/<target>-<era>/` beside the executable, or use the native
`sapie` launcher (`sapie.exe` on Windows), then start the game.

Port 47584 is the binary game protocol, not HTTP. A browser pointed at it showing "connection reset"
is normal.

## Downloads

| Release | Assets | Notes |
| --- | --- | --- |
| [0.2](https://github.com/2kfi/SAPIE/releases/tag/0.2) | `SAPIE-0.2-linux-x64.tar.gz`, `SAPIE-0.2-linux-arm64.tar.gz`, `SAPIE-0.2-windows-x64.zip` | Current release |
| [0.1](https://github.com/2kfi/SAPIE/releases/tag/0.1) | Same three files with `0.1` in place of `0.2` | Frozen, no P2P or web UI |

Each archive contains the launcher, the server, all ten `steam_api/<target>-<era>` variants,
`QUICKSTART.md`, and sample config files. The Windows zip also includes Steamless and its NOTICE.

Container images: `ghcr.io/2kfi/sapie-server:0.2` (also `:latest`); the frozen 0.1 image is `:0.1`.
With Docker Compose, run `SAPIE_ADMIN_TOKEN=<your-token> docker compose up --build` from the repo root.

## What you get

- Accounts with server-assigned SteamIDs, friends, presence, and invites with connect strings.
- Lobbies you can create, list, and join.
- Achievements and stats, with hidden achievements kept from non-owners.
- A LAN web UI on 8081 for login, registration, friends, and lobbies.
- Direct UDP peer-to-peer game traffic when a peer hint is available, with a server relay fallback.

## Server options

`--listen` is the game protocol (default `:47584`), `--admin-listen` is the admin API
(default `127.0.0.1:8080`, requires a `Bearer` token), `--ui-listen` is the web UI
(default `0.0.0.0:8081`), and `--ui-dir` points at the built SPA (default `./web/dist`).
Registration is off unless `--allow-register` is passed. Only user API routes are proxied through
8081; admin paths return 404 there and still work on 8080. The UI is plain HTTP, so session tokens
cross the LAN in cleartext.

## Friends

There is no add-friend endpoint yet. To link two accounts, stop the server and add both directions
to the database:

```sh
sqlite3 sapie.db "INSERT OR IGNORE INTO friendships(steam_id,friend_id) VALUES (<A>,<B>), (<B>,<A>);"
```

Restart the server afterward. Friends lists, invites, and friend-gated P2P need this row.

## Steamless

The launcher can unpack SteamStub-packed Windows executables. Steamless is an unmodified Windows-only
sidecar with its own NOTICE in the Windows archive. Do not modify it.

## More

[Game setup and flags](docs/QUICKSTART.md) · [Building and CI](docs/BUILD.md) ·
[Protocol](docs/PROTOCOL.md) · [Known limits](dev_notes.md)

Direct P2P is implemented but has not yet been verified in a live two-client game run. Development
happens on `dev-client` (default), released from `main-client`.

## License

Goldberg fork, LGPLv3 ([LICENSE](LICENSE)). Steamless NOTICE:
`client/src-tauri/resources/steamless/NOTICE.md`.
