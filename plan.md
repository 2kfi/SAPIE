# Steam API Emulator (SAPIE) — Build Plan

## 1. Goal

Run old Steam games without Steam. A Go server acts like Steam (accounts, presence, lobbies, achievements, stats). A forked Goldberg emulator DLL runs inside the game, emulates the Steam API, and forwards calls to the server. A Tauri launcher and a browser web UI share one React SPA.

V1 scope: identity → friends/presence/invites → lobbies/matchmaking → achievements/stats. 0.1 shipped with no P2P game traffic of any kind. **No P2P game-traffic relay in v1** is SUPERSEDED for 0.2: 0.2 adds direct client-to-client P2P first, with a relay fallback for NAT-blocked peers. No DRM checks — cracked games are not our problem.

## 2. Fixed decisions

| Item | Decision |
| --- | --- |
| Name | Steam API Emulator (SAPIE) |
| License | Goldberg fork stays LGPLv3; notices preserved, modifications published |
| Branches | `dev-client` (default), `dev-server`, `main-client`, `main-server`; identical tree on all four |
| Integration trunk | `dev-client`. Sync `dev-client → dev-server`; release PRs `dev-client → main-client`, `dev-server → main-server` |
| Client base | Goldberg emulator fork |
| Server | Go + SQLite (`modernc.org/sqlite`, pure Go → no cgo) |
| Login | username + password, argon2id server-side, server-assigned SteamID |
| Client artifacts | Windows x86/x64 + Linux x86/x64/aarch64 × {legacy pre-2016, modern} = 10 |
| Server artifacts | `linux/amd64`, `linux/arm64`, `windows/amd64`, `windows/arm64` |
| Launcher | Tauri shell + shared React/TS/plain-CSS SPA |
| Server web UI | Same SPA; live users/games + statistics; admin can open any account read-only |
| Account page | Achievements + games, read-only. Owner sees own; admin can open any |
| Game launch | Windows: copy matching era/arch DLL next to exe. Linux: `LD_PRELOAD` |
| Design | Light liquid glass / Windows Auro, ember accent, Manrope + IBM Plex Mono, solid fallback when blur unsupported |
| Test games | MW2 (legacy x86, no SteamStub per steamless), Stardew Valley (era detected first) |

## 3. Packages (Arch)

```bash
# official repos
sudo pacman -S --needed \
  base-devel cmake ninja git go python \
  protobuf \
  mingw-w64-gcc \
  aarch64-linux-gnu-gcc \
  lib32-glibc lib32-gcc-libs \
  rust nodejs npm webkit2gtk-4.1 gtk3 libayatana-appindicator

# AUR (yay/paru) — cross-built protobuf for Windows targets
yay -S mingw-w64-protobuf
```

- `mingw-w64-gcc` provides both `i686-w64-mingw32-*` and `x86_64-w64-mingw32-*`.
- `aarch64-linux-gnu-gcc` pulls binutils/glibc/headers.
- `lib32-*` may require uncommenting `[multilib]` in `/etc/pacman.conf`.
- Host `protoc` from `protobuf`; ARM64 protobuf-lite built from source by `tools/build-protobuf.sh`.
- Protobuf 36 requires C++17 → fork moves C++14 → C++17.
- CI uses an `archlinux` container with the same list; Tauri builds on native runners (`windows-latest`, `ubuntu-24.04`, `ubuntu-24.04-arm`).

### `note.md` — first file created

```md
# SAPIE — Developer Note

Project: Steam API Emulator (SAPIE)

## Python rule

ALWAYS use a Python virtual environment for any Python tooling in this repo.
NEVER use system Python or install packages system-wide.

    python -m venv .venv
    source .venv/bin/activate

Use only the activated interpreter's `python`/`pip`. `.venv/` is gitignored.
```

## 4. Repository tree

```
SAPIE/
├─ plan.md
├─ note.md
├─ README.md
├─ LICENSE                       # LGPLv3 + notes for Go server license
├─ .gitignore                    # .venv/, build/, out/, *.log, .sapie-state/, intial-repo/
├─ .github/workflows/build.yml   # client matrix, server matrix, launcher jobs
├─ proto/
│  ├─ net.proto                  # vendored from Goldberg, unchanged
│  └─ sapie.proto                # envelope: login, stats, tickets, ping
├─ web/                          # shared React + TS + plain CSS SPA
│  └─ src/
├─ client/
│  ├─ src-tauri/                 # Rust shell + native commands
│  ├─ CMakeLists.txt
│  ├─ cmake/toolchains/          # windows-x86/x64, linux-x86/x64/arm64
│  ├─ dll/                       # Goldberg fork, minimal edits
│  ├─ sdk_includes/              # vendored Steamworks headers
│  └─ sapie/                     # new client code
│     ├─ era_tables.h
│     ├─ transport.h / transport.cpp
│     ├─ local_mirror.h
│     └─ stats_client.cpp
├─ server/
│  ├─ go.mod / go.sum
│  ├─ cmd/sapie-server/main.go
│  ├─ internal/{protocol,auth,session,state,router,api}/
│  └─ migrations/001_init.sql
├─ tools/
│  ├─ harness/                   # Go fake client
│  ├─ detect-era/                # interface-string extractor wrapper
│  └─ build-protobuf.sh <target>
└─ docs/{PROTOCOL,BUILD,QUICKSTART,UI}.md
```

## 5. Wire protocol

4-byte big-endian length prefix + one protobuf `Frame`, max 1 MiB. First frame must be `LoginRequest`; anything else closes the connection.

`proto/sapie.proto` imports `net.proto` and defines: `FriendSnapshot`, `LoginRequest`, `LoginResponse` (status, server-assigned `steam_id`, persona, session token, friends), `TicketRequest/Response`, `StatValue`, `AchievementState`, `StatsGet`, `StatsSet`, `StatsResponse`, `Frame` oneof carrying login, `Common_Message steam`, tickets, stats, ping/pong.

**Server overwrites `Common_Message.source_id` with the authenticated connection's SteamID. Client-supplied identity is never trusted.**

## 6. Server

Packages: `cmd/sapie-server` (flags `--listen :47584`, `--admin-listen 127.0.0.1:8080`, `--db`, `--admin-token`, optional TLS), `internal/protocol` (frame codec), `internal/auth` (argon2id m=64MiB/t=3/p=4, 32-byte tokens stored hashed, constant-time compare, rate limits), `internal/session` (accept loop, login state, 30s ping/90s timeout, token reconnect, registry), `internal/state` (Store interface + SQLite + live presence/lobby mirrors), `internal/router` (route by friend/lobby/dest, reject non-friends, lobby list by appid), `internal/api` (HTTP admin API).

SQLite tables: `accounts`, `friendships`, `sessions`, `lobbies`, `lobby_members`, `presence`, `achievements`, `stats`, `auth_tickets`. FKs with cascades, indexes on lookups, UTC timestamps, SteamIDs as INTEGER. Base `76561197960265728` + monotonic counter.

Admin API (loopback by default, Bearer token, idempotency keys on POST): `GET /healthz`, CRUD on `/api/v1/accounts`, friendship management, `GET /api/v1/lobbies`, `GET /api/v1/accounts/{id}/stats`.

Security: client→server is the trust boundary; server owns identity, friend graph, lobby ownership, stat values. Passwords never logged. Tokens 24h, hashed at rest, rotated on reconnect. 5 failed logins/min/IP; frame-size and rate caps. Achievement unlocks monotonic (`MAX(unlock_time)`). Admin API loopback unless explicitly overridden.

## 7. Client

**Critical behavior rule:** synchronous getters must not round-trip. Getters (`GetSteamID`, friend counts, presence, lobby data) read `client/sapie/local_mirror.h`, populated at login and updated by server pushes. Mutations (`SetRichPresence`, `CreateLobby`, `JoinLobby`, `SetAchievement`, invites) send a frame; callbacks fire on reply/push.

File-by-file:
- `common_includes.h`: add `__aarch64__` → `__LINUX_ARM64__` (currently mislabels aarch64 as 32-bit).
- `dll.cpp`: compile-time `SAPIE_LEGACY_INTERFACES` selects legacy vs modern interface strings.
- `settings_parser.cpp`: parse `sapie_server.txt` (host:port); credentials from `SAPIE_USERNAME`/`SAPIE_PASSWORD`. Absent file preserves Goldberg LAN behavior for A/B debugging.
- `base.cpp`: use server-assigned SteamID instead of `generate_steam_id()`.
- `network.h/.cpp`: `Server_Connection` — TCP connect, login, frame pump, heartbeat, reconnect with backoff; driven by `Networking::Run()`; inbound frames go to existing `Callback_Ids` dispatcher.
- `steam_user_stats.h`: replace local JSON with `StatsGet`/`StatsSet` + read-through cache.
- `CMakeLists.txt`: C++17, 10 targets, no detours/overlay on ARM64.

Tauri native commands: pick executable, copy matching DLL, set `LD_PRELOAD`, spawn process. Launcher stores per-game name, appID, exe path, args, era, arch.

## 8. UI design

Direction: quiet glass instrument panel, not a game store. Cool neutral acrylic, hairline light edges, one ember accent marking the active/live thing. Utilities-app density.

- Accent `#A63A16` light / `#FF9B6A` dark; status green `#146C43` / `#8FE3B0`.
- Light bg `#E6EBF2`, surface `#F7F9FC`, text `#16202E`, muted `#3F4C60`; dark bg `#0E1420`, surface `#1B2432`.
- Manrope (UI), IBM Plex Mono (appIDs/paths/logs). Scale 12/13/14/16/20/28. Spacing 4/8/12/16/24/32. Radius 6/10/16. Blur 20px panels / 28px popovers. Glass alpha ≥ `.52`.
- Contrast: light text/glass 15.55:1, muted 8.25:1, white/ember 6.49:1; dark 14.29:1 / 8.68:1. Re-verify over bright wallpapers.
- Glass limited to nav rail + detail pane; `@supports not (backdrop-filter)` → solid surface.
- Screens: login, main shell (rail/friends/detail), game detail + launch, live sessions admin, account (shared). Each has empty/loading/error states.
- Presence dots always have text labels. Focus = 2px ember ring at 2px offset. Toasts `aria-live="polite"`, errors `role="alert"`. 44×44 targets.
- Motion: only rail selection (120ms), modal (160ms), toast (200ms), focus ring. No pulsing dots, no staggered rows; `prefers-reduced-motion` collapses to opacity.
- P2P placeholder text: "Peer-to-peer play is not available in this version." No dead buttons.
  This is 0.1 text; it changes only when P2P ships in 0.2.

## 9. Agents

Chief = coordinating thread; workers = `general` subagents with isolated worktrees and self-contained specs. Registry in git-excluded `.sapie-state/`. Cap 2–3 live workers. Check-ins are milestone-based via `sessionID` follow-ups (no timer daemon; no supervision across session exit/sleep).

Assignments: W0-a bootstrap → W0-b wire contract → W0-c client matrix, W0-d Go skeleton, W0-e UI/Tauri (parallel). Then W1 identity, W2 friends, W3 lobbies, W4 stats, W5 hardening.

Risk tiers: green (docs/CI/UI polish), yellow (auth-adjacent, transports, UI behavior), red (proto/schema, auth/session tokens, stats schema). Chief auto-merges green/yellow into dev branches; red and all `main-*` PRs require user approval of the current head. Review chain: `code-review-and-quality`, `ponytail-review`, `security-audit` for auth/proto, `accessibility` + `webapp-testing` for UI, `replica-test` for flows.

Sync `dev-client → dev-server` after every integration batch to prevent proto drift.

## 10. Milestones

| # | Scope | Acceptance |
| --- | --- | --- |
| M0 | Repo/branches, `note.md`, fork, proto, 10-DLL matrix, Tauri scaffold, Go login + SQLite, harness | Artifacts link; `go test ./...` green; harness logs in twice with distinct SteamIDs; admin API lists both |
| M1 | Identity in `SteamAPI_Init` + launcher login; server-assigned SteamID | Game boots with DLL replaced; `GetSteamID()` matches admin record (MW2) |
| M2 | Friends/presence/invites; friend-list UI | Two harness clients see each other online with connect string |
| M3 | Lobbies/matchmaking; live-games dashboard | "Join Friend"/"Connect to Friend" puts both clients in one lobby (Stardew) |
| M4 | Achievements/stats; account page | Unlock on A reflected on B; admin API shows same unlock time |
| M5 | Reconnect/backoff, limits, docs, release packaging | Docs complete; failure and rate-limit tests pass |

0.1 is published and frozen. The table above is 0.1 history; 0.2 work starts below.

### 0.2 milestones (planned)

| # | Scope | Acceptance |
| --- | --- | --- |
| M6 | Full online persona surface | A real MW2-2009-style client sees a logged-in account, persona name and online state, friends list and invites, achievements and stats, and app ownership/ticket. Verified with a real client, not just the harness. |
| M7 | Deeper lobbies/matchmaking | "Join Friend"/"Connect to Friend" flows hold for two real clients. |
| M8 | Direct P2P | Two game clients exchange game traffic directly, with no server relaying of game packets. Verified on the same LAN and across NAT where possible. |
| M9 | Relay fallback | NAT-blocked peers fall back to a SAPIE-run relay, with the direct path preferred. A blocked-direct case is verified. |

Order: M6/M7 (real two-client proof) first, then M8 direct P2P, then M9 relay fallback.

## 11. Risks / deferred

- Deferred: UGC/inventory/overlay/controller, Windows ARM64 client, 32-bit ARM.
- 0.2 planned: P2P, direct client-to-client first with relay fallback for NAT-blocked peers. 0.1 shipped without P2P; the relay is no longer deferred.
- Carried from `dev_notes.md` into 0.2: mutable mirror refs, `StatsGet.names` handling, hidden achievements in unlock pushes, and the missing live two-client/real-game run. Only the last is a hard gate (M6 to M9 cannot be called done without it); the first three are known limits to fix or accept.
- Unverified: protobuf 36 compatibility with older Goldberg headers; `mingw-w64-protobuf` i686 coverage; Tauri webkit2gtk blur; ARM64 on real hardware.
- Stardew Valley's interface era unknown until detected; MW2 DLL replacement assumed to work.
- Mutating calls may race games expecting instant local callbacks; optimistic mirror updates may be needed per game.
