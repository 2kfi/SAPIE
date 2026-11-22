# SAPIE

![SAPIE wordmark, provisional](design/wordmark.svg)

**Your games. Your server.**

Old Steam games, no Steam client, your own server. SAPIE is a non-commercial Steam API replacement
for games you already own, for gamers by gamers.

The wordmark is provisional and pending G1. See `design/identity.md`.

## Quick start

Download the newest archive from the [releases page](https://github.com/2kfi/SAPIE/releases), unpack
it, and start the server from the unpacked directory:

```sh
./sapie-server
```

No flags needed. Game protocol: port 47584. Admin API: 127.0.0.1:8080. Web UI: 0.0.0.0:8081.
The admin token is generated on first start, printed once, and saved in `sapie.db` so it stays the
same across restarts. Pass `--admin-token <token>` to override it. Registration is off by default;
start once with `--allow-register` to create accounts from the web UI, then restart without it.

After enabling registration as above, open `http://<server-ip>:8081`, register two accounts, and log
in. Add friends from the Friends screen by entering a username and sending a request. No database
editing is needed.

The SAPIE Launcher handles game setup. Add an executable, confirm the detected arch, era and appid,
and the launcher copies the matching `steam_api` library, writes `sapie_server.txt` and
`steam_appid.txt`, and starts the game in one click. It passes the session through a short-lived
`steam_settings/sapie_client.json` file (mode 0600 on Unix). Older setups can still set
`SAPIE_USERNAME` and `SAPIE_PASSWORD` in the game environment; that path is a fallback, not the
preferred handoff. See [Quickstart](docs/QUICKSTART.md).

Port 47584 is the binary game protocol, not HTTP. A browser pointed at it showing "connection reset"
is normal.

## Downloads

Release archives are attached to numeric `0.x` tags on the
[releases page](https://github.com/2kfi/SAPIE/releases). The `0.4` tag and its assets are published.
The 0.5 work in this tree is code-complete but unreleased; no 0.5 tag or artifacts are recorded. See
[Upgrading from 0.4 to 0.5](docs/UPGRADE.md) for the documented path.

| Release | Notes |
| --- | --- |
| `0.3` | Tagged at a docs-only commit on `dev-client`. Check the release page for any attached assets. |
| `0.2` | Prior published release with archives and a container image. |
| `0.1` | Frozen, no P2P or web UI. |

Published archives contain the launcher, the server, all ten `steam_api/<target>-<era>` variants,
`QUICKSTART.md`, and sample config files. The Windows zip also includes Steamless and its NOTICE.

Container images follow `ghcr.io/2kfi/sapie-server:<tag>` plus `:latest`. With Docker Compose, run
`docker compose up --build` from the repo root; the generated token appears in `docker compose logs`.
The Compose file serves the built web UI on host loopback port 8081.

## What you get

- Accounts with server-assigned SteamIDs, friends, presence, and invites with connect strings.
- Add, remove, block and unblock friends through requests in the web UI.
- Lobbies you can create, list, and join.
- Achievements and stats, with hidden achievements kept from non-owners.
- A LAN web UI on 8081 for login, registration, friends, and lobbies.
- Direct UDP peer-to-peer game traffic when a peer hint is available, with a server relay fallback.

## Server options

Every flag has a default; pass one only to change it. `--port` sets the game protocol port (default
`47584`), `--admin-listen` the admin API (default `127.0.0.1:8080`, requires a `Bearer` token),
`--ui-listen` the web UI (default `0.0.0.0:8081`), `--ui-dir` the built SPA (default `./web/dist`),
and `--db` the database (default `sapie.db`). `--allow-register` is off by default;
`--admin-token` overrides the stored token for that run. `--tls-cert` and `--tls-key` together enable
TLS on the game listener.

Only user API routes are proxied through 8081; admin paths return 404 there and still work on 8080.
The UI is plain HTTP, so session tokens cross the LAN in cleartext.

## Friends

Friend management is an API, not a database edit. In the web UI, open Friends, enter a username, send
a request, and accept an incoming one. Accepting writes both friendship directions and pushes a fresh
friend snapshot to the game socket. Invites and friend-gated P2P use that graph. Request, remove,
block and unblock routes are listed in [API](docs/API.md).

## Steamless

The launcher can unpack SteamStub-packed Windows executables. Steamless is an unmodified Windows-only
sidecar with its own NOTICE in the Windows archive. Do not modify it.

## More

[Game setup and flags](docs/QUICKSTART.md) · [Building and CI](docs/BUILD.md) ·
[Protocol and API](docs/PROTOCOL.md) · [HTTP API](docs/API.md) · [Security](docs/SECURITY.md) ·
[Contributing](docs/CONTRIBUTING.md) · [Changelog](docs/CHANGELOG.md) · [Upgrade](docs/UPGRADE.md) ·
[Known limits](dev_notes.md)

A headless two-client C++ to Go session is recorded in
[0.4 two-client verification](docs/verification/0.4-two-client.md). A real MW2 run, native GUI
click-through, NAT and relay path selection are UNVERIFIED. Development happens on `dev-client`
(default), released from `main-client`.

## License

Goldberg fork, LGPLv3 ([LICENSE](LICENSE)). Steamless NOTICE:
`client/src-tauri/resources/steamless/NOTICE.md`.
