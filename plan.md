# Steam API Emulator (SAPIE) — Build Plan

## 1. Goal

Run old Steam games without Steam. A Go server acts like Steam (accounts, presence, lobbies, achievements, stats). A forked Goldberg emulator DLL runs inside the game, emulates the Steam API, and forwards calls to the server. The launcher is a native Rust egui/eframe app; the Go server also hosts a React browser web UI for LAN use.

V1 scope: identity → friends/presence/invites → lobbies/matchmaking → achievements/stats. 0.1 shipped with no P2P game traffic of any kind. **No P2P game-traffic relay in v1** is SUPERSEDED for 0.2: 0.2 code adds direct client-to-client P2P first, with a relay fallback for NAT-blocked peers. 0.2 is code-complete locally but not tagged or released; G0 is deferred to the user. No DRM checks — cracked games are not our problem.

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
| Launcher | Native Rust egui/eframe app (`sapie-launcher`); no webview, localhost or terminal window; `windows_subsystem` on Windows |
| Server web UI | React SPA hosted by the Go server on the LAN (default `0.0.0.0:8081`, `--ui-dir ./web/dist`); narrow proxy forwards user API routes only, admin paths stay on loopback 8080 |
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
  rust nodejs npm libxkbcommon libwayland libx11 libxi libxrandr mesa

# AUR (yay/paru) — cross-built protobuf for Windows targets
yay -S mingw-w64-protobuf
```

- `mingw-w64-gcc` provides both `i686-w64-mingw32-*` and `x86_64-w64-mingw32-*`.
- `aarch64-linux-gnu-gcc` pulls binutils/glibc/headers.
- `lib32-*` may require uncommenting `[multilib]` in `/etc/pacman.conf`.
- Host `protoc` from `protobuf`; ARM64 protobuf-lite built from source by `tools/build-protobuf.sh`.
- Protobuf 36 requires C++17 → fork moves C++14 → C++17.
- CI uses an `archlinux` container with the same list; the native launcher builds on native runners (`windows-latest`, `ubuntu-24.04`, `ubuntu-24.04-arm`) with libxkbcommon/wayland/x11/xi/xrandr/gl1-mesa dev libs. There is no webkit2gtk dependency.

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
├─ .github/workflows/build.yml   # client, server, launcher, web jobs + release bundles
├─ proto/
│  ├─ net.proto                  # vendored from Goldberg, unchanged
│  └─ sapie.proto                # envelope: login, stats, tickets, ping
├─ web/                          # React + TS + plain-CSS SPA hosted by the server
│  └─ src/
├─ client/
│  ├─ src-tauri/                 # native Rust egui/eframe launcher (dir name retained)
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

Packages: `cmd/sapie-server` (flags `--listen :47584` for the binary protobuf game protocol, `--admin-listen 127.0.0.1:8080` for the admin API, `--ui-listen 0.0.0.0:8081` for the LAN web UI, `--ui-dir ./web/dist`, `--allow-register` off by default, `--db`, `--admin-token`, optional TLS), `internal/protocol` (frame codec), `internal/auth` (argon2id m=64MiB/t=3/p=4, 32-byte tokens stored hashed, constant-time compare, rate limits), `internal/session` (accept loop, login state, 30s ping/90s timeout, token reconnect, registry), `internal/state` (Store interface + SQLite + live presence/lobby mirrors), `internal/router` (route by friend/lobby/dest, reject non-friends, lobby list by appid, authorize P2P signaling and relay DATA), `internal/api` (HTTP admin API), `internal/ui` (LAN SPA hosting + narrow user-API reverse proxy).

SQLite tables: `accounts`, `friendships`, `sessions`, `lobbies`, `lobby_members`, `presence`, `achievements`, `stats`, `auth_tickets`. FKs with cascades, indexes on lookups, UTC timestamps, SteamIDs as INTEGER. Base `76561197960265728` + monotonic counter.

Admin API (loopback by default, Bearer token, idempotency keys on POST): `GET /healthz`, CRUD on `/api/v1/accounts`, friendship management, `GET /api/v1/lobbies`, `GET /api/v1/accounts/{id}/stats`. The LAN UI listener on 8081 hosts the SPA and reverse-proxies only user routes (`config`, `register`, `login`, `friends`, `invite`, `lobbies`, `join`, own stats). Admin-only paths (`/api/v1/users`, account CRUD) return 404 on 8081 and still work on 8080. Over plain HTTP the session token crosses the LAN in cleartext.

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

Native launcher (egui/eframe): native file picker, copy matching DLL, set `LD_PRELOAD`, spawn process. It stores the server/username/password connection plus per-game exe path, arch, era, interfaces and DLL/preload settings in a local state file. There is no webview, localhost dependency or terminal window.

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
  This is frozen 0.1 text. 0.2 ships P2P in code, but the line stays until the user accepts G0; changing it is gated on G0 acceptance.

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

### 0.2 milestones (code-complete locally, not tagged or released)

| # | Scope | Status / local verification |
| --- | --- | --- |
| M6 | Native egui/eframe launcher (`sapie-launcher`) | Code-complete; client modern+legacy builds and `sapie_login_smoke` pass locally. No cross-compiled Windows execution. |
| M7a | Server LAN hosting, user-API reverse proxy, register proxy | Code-complete; `go test ./...`, `go build ./cmd/sapie-server`, harness pass locally. No browser click-through. |
| M7b | Web login and opt-in registration behind `GET /api/v1/config` | Code-complete; web build (`tsc --noEmit` + vite) passes locally. No browser click-through. |
| M8a | Server persona completeness (`StatsGet.names`, hidden-push skip, ticket recording and verify) | Code-complete; `go test ./...` and harness pass locally. |
| M8b | Safe client mirror (locked `friends()`/`lobbies()` copies, explicit local lobby write-back) | Code-complete; client modern+legacy builds and `sapie_login_smoke` pass locally. |
| M9a | Server lobby rules (limit, joinable/type, appid, duplicates, owner transfer, disconnect cleanup) | Code-complete; `go test ./...` and harness pass locally. |
| M9b | Client lobby flows (repeated JOIN fix, roster and owner updates) | Code-complete; client modern+legacy builds and `sapie_login_smoke` pass locally. |
| M10a | Server P2P signaling authorization plus source IP/port hint | Code-complete; `go test ./...` and harness pass locally. |
| M10b | Client direct UDP P2P via `sendToPeer()` when a hint exists | Code-complete; client modern+legacy builds and `sapie_login_smoke` pass locally. |
| M11a | Server DATA relay to explicit `dest_id` only (friend/lobby gate, no broadcast) | Code-complete; `go test ./...` and harness pass locally. |
| M11b | Client relay fallback when no hint or no inbound DATA within 5s | Code-complete; client modern+legacy builds and `sapie_login_smoke` pass locally. |
| M-INT | CI web build and `web-dist` artifact, release bundles with `web/dist`, this docs refresh | Code-complete locally; see `docs/BUILD.md`. 0.2 is not tagged or released. |

Every row above is local code-complete only. All G0 and live-game items are PENDING USER TEST: no live two-client C++↔Go session, no real MW2 2009 run, no browser click-through, no cross-compiled Windows execution, no real NAT test. G0 itself is deferred to the user.

## 11. Risks / deferred

- Deferred: UGC/inventory/overlay/controller, Windows ARM64 client, 32-bit ARM.
- 0.2 code-complete locally: direct client-to-client P2P with relay fallback for NAT-blocked peers. 0.1 shipped without P2P; the relay is no longer deferred. 0.2 is not tagged or released and G0 is deferred to the user.
- The three small 0.1 limits carried into 0.2 (mutable mirror refs, `StatsGet.names` handling, hidden achievements in unlock pushes) are closed in code. The missing live two-client/real-game run is unchanged and is the hard gate; these milestones cannot be called done without it.
- Unverified: protobuf 36 compatibility with older Goldberg headers; `mingw-w64-protobuf` i686 coverage; native egui launcher on cross-compiled Windows binaries; ARM64 on real hardware; browser click-through; screen-reader and keyboard audit; real NAT traversal.
- Direct reliable DATA is best-effort over UDP. The 5s relay window is measured from connection creation, so a slow handshake may pick the relay. There is no relay rate limit beyond existing per-session handling.
- Stardew Valley's interface era unknown until detected; MW2 DLL replacement assumed to work.
- Mutating calls may race games expecting instant local callbacks; optimistic mirror updates may be needed per game.
- Verification is honest: nothing in 0.2 is fully tested, and G0 acceptance remains with the user.
