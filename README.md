# Steam API Emulator (SAPIE)

SAPIE runs old Steam games without Steam: a Go server emulates Steam's account,
friend, lobby and stats services; a forked Goldberg emulator DLL forwards Steam
API calls to it; a Tauri launcher and the server's web UI share one React app.

Plan: see `plan.md`. Developer rules: see `note.md`.

## Status

0.1 release. Friends, lobbies and achievements are served by the Go server; no live two-client game run has been done yet. See `docs/QUICKSTART.md`.

## Layout

- `proto/` — shared wire contract (`net.proto` from Goldberg, `sapie.proto` envelope)
- `client/` — Goldberg fork and the 10 DLL build targets, plus the Tauri launcher shell
- `server/` — Go server
- `web/` — shared React UI
- `tools/` — harness and build helpers

## License

The client fork is based on the Goldberg Emulator, LGPLv3. See `LICENSE`.
