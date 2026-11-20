# Steam API Emulator (SAPIE) — Build Plan

> Current plan: section 12 (0.4). Sections 1 to 11 are the 0.1 and 0.2 history and remain as a
> decision record.

## 1. Goal

Run old Steam games without Steam. A Go server acts like Steam (accounts, presence, lobbies, achievements, stats). A forked Goldberg emulator DLL runs inside the game, emulates the Steam API, and forwards calls to the server. The launcher is a native Rust egui/eframe app; the Go server also hosts a React browser web UI for LAN use.

V1 scope: identity → friends/presence/invites → lobbies/matchmaking → achievements/stats. 0.1 shipped with no P2P game traffic of any kind. **No P2P game-traffic relay in v1** is SUPERSEDED for 0.2: 0.2 code adds direct client-to-client P2P first, with a relay fallback for NAT-blocked peers. 0.2 is tagged `0.2` and published at https://github.com/2kfi/SAPIE/releases/tag/0.2; G0 is deferred to the user. No DRM checks — cracked games are not our problem.

## 2. Fixed decisions

| Item | Decision |
| --- | --- |
| Name | Steam API Emulator (SAPIE) |
| License | Goldberg fork stays LGPLv3; notices preserved, modifications published |
| Branches | `dev-client` (default), `main-client`; identical tree on both |
| Integration trunk | `dev-client`; release PRs `dev-client → main-client` |
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

Packages: `cmd/sapie-server` (all flags defaulted; `--port 47584` for the binary protobuf game protocol, `--admin-listen 127.0.0.1:8080` for the admin API, `--ui-listen 0.0.0.0:8081` for the LAN web UI, `--ui-dir ./web/dist`, `--allow-register` off by default, `--db`, `--admin-token`, optional TLS), `internal/protocol` (frame codec), `internal/auth` (argon2id m=64MiB/t=3/p=4, 32-byte tokens stored hashed, constant-time compare, rate limits), `internal/session` (accept loop, login state, 30s ping/90s timeout, token reconnect, registry), `internal/state` (Store interface + SQLite + live presence/lobby mirrors), `internal/router` (route by friend/lobby/dest, reject non-friends, lobby list by appid, authorize P2P signaling and relay DATA), `internal/api` (HTTP admin API), `internal/ui` (LAN SPA hosting + narrow user-API reverse proxy).

SQLite tables: `accounts`, `friendships`, `sessions`, `lobbies`, `lobby_members`, `presence`, `achievements`, `stats`, `auth_tickets`, `settings` (persisted admin token). FKs with cascades, indexes on lookups, UTC timestamps, SteamIDs as INTEGER. Base `76561197960265728` + monotonic counter.

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
- P2P text: "Friends can play directly with each other, and the server relays traffic when a direct connection is not possible." No dead buttons.
  The old 0.1 line "Peer-to-peer play is not available in this version." is superseded as of M12; G0 no longer gates the wording.

## 9. Agents

Chief = coordinating thread; workers = `general` subagents with isolated worktrees and self-contained specs. Registry in git-excluded `.sapie-state/`. Cap 2–3 live workers. Check-ins are milestone-based via `sessionID` follow-ups (no timer daemon; no supervision across session exit/sleep).

Assignments: W0-a bootstrap → W0-b wire contract → W0-c client matrix, W0-d Go skeleton, W0-e UI/Tauri (parallel). Then W1 identity, W2 friends, W3 lobbies, W4 stats, W5 hardening.

Risk tiers: green (docs/CI/UI polish), yellow (auth-adjacent, transports, UI behavior), red (proto/schema, auth/session tokens, stats schema). Chief auto-merges green/yellow into `dev-client`; red and `main-client` PRs require user approval of the current head. Review chain: `code-review-and-quality`, `ponytail-review`, `security-audit` for auth/proto, `accessibility` + `webapp-testing` for UI, `replica-test` for flows.

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

### 0.2 milestones (tagged `0.2` and released, G0 deferred)

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
| M-INT | CI web build and `web-dist` artifact, release bundles with `web/dist`, this docs refresh | Code-complete locally; see `docs/BUILD.md`. 0.2 is tagged `0.2` and released. |

Every row above shipped in the tagged `0.2` release with local verification only. All G0 and live-game items are PENDING USER TEST: no live two-client C++↔Go session, no real MW2 2009 run, no browser click-through, no cross-compiled Windows execution, no real NAT test. G0 itself is deferred to the user.

## 11. Risks / deferred

- Deferred: UGC/inventory/overlay/controller, Windows ARM64 client, 32-bit ARM.
- 0.2 shipped in the tagged `0.2` release: direct client-to-client P2P with relay fallback for NAT-blocked peers. 0.1 shipped without P2P; the relay is no longer deferred. G0 is deferred to the user.
- The three small 0.1 limits carried into 0.2 (mutable mirror refs, `StatsGet.names` handling, hidden achievements in unlock pushes) are closed in code. The missing live two-client/real-game run is unchanged and is the hard gate; these milestones cannot be called done without it.
- Unverified: protobuf 36 compatibility with older Goldberg headers; `mingw-w64-protobuf` i686 coverage; native egui launcher on cross-compiled Windows binaries; ARM64 on real hardware; browser click-through; screen-reader and keyboard audit; real NAT traversal.
- Direct reliable DATA is best-effort over UDP. The 5s relay window is measured from connection creation, so a slow handshake may pick the relay. There is no relay rate limit beyond existing per-session handling.
- Stardew Valley's interface era unknown until detected; MW2 DLL replacement assumed to work.
- Mutating calls may race games expecting instant local callbacks; optimistic mirror updates may be needed per game.
- Verification is honest: nothing in 0.2 is fully tested, and G0 acceptance remains with the user.

---

## 12. 0.4 plan (next release)

### 12.0 What 0.4 is

0.4 is the release where SAPIE stops being an engineering prototype and starts being a product a
normal person can install. It has three outcomes:

1. **Proven.** A real two-client game session and a real MW2 run happened and are recorded. This is
   the gate carried since 0.1.
2. **Usable.** The launcher is a game library with one-click play, not a six-step setup wizard. The
   password is never written to disk. The web UI can add and remove friends without SQLite.
3. **Shippable.** One CI path, versioned artifacts, a working container, signed Windows builds where
   a certificate exists, and docs that match the binaries.

The reference product surface is Steam, Lutris, Heroic and GOG Galaxy. We take the library-first
model, not the store, the DRM, the chat or the cloud.

### 12.1 How M stages and phases work

- An **M stage** is a merge stage into `dev-client`. It ships as one coherent change set.
- A **phase** is one agent assignment: one worktree, one self-contained spec, one artifact, one
  review. Phases are named `M13-a`, `M13-b`, and so on.
- Phases inside an M run in the order listed unless the stage says they are parallel. Two phases
  must not edit the same files concurrently.
- Each phase carries a risk tier. **Green** merges without human review. **Yellow** merges after a
  review of the diff and evidence. **Red** needs the user's approval of the current head (schema,
  auth, tokens, protocol, release).
- A phase is done only when its acceptance line is backed by evidence: a test run, a log, a
  screenshot, or a recorded live run. "Code-complete" is not done.
- The chief thread owns the registry in `.sapie-state/`, caps live workers at 2 to 3, and checks in
  on milestones, not timers.

Stage map:

| Stage | Theme | Merge gate |
| --- | --- | --- |
| M13 | Prove the core | Two-client and MW2 evidence recorded |
| M14 | Launcher foundation | Tauri app builds on Windows and Linux, real login works |
| M15 | Launcher product | A user can add a game and press Play once |
| M16 | DLL and protocol | Game authenticates with a token file, friend changes reach the game |
| M17 | Social server | Add, remove, block and requests work through the API |
| M18 | Web UI | Social flows, logout, theme and a11y pass in a browser |
| M19 | Security | No plaintext secrets, TLS path, hardened sessions, working container |
| M20 | CI and release | Tag produces signed, versioned artifacts |
| M21 | Docs and cleanup | Egui launcher removed, docs match 0.4, tag pushed |

### 12.2 The launcher we are building

#### 12.2.1 Product sentence

SAPIE Launcher is a local game library for games you already own. It detects a game, installs the
right Steam emulation library, remembers the server and account, and starts the game in one click.
It is not a store, not a social network, and not a setup checklist.

#### 12.2.2 What we take from each reference

| Reference | What we take | What we leave |
| --- | --- | --- |
| Steam | Library-first home, large Play action, cover tiles, playtime, friends rail | Store, DRM, chat, marketplace |
| Lutris | Visible runner/config details, per-game settings, repair and restore | Install scripts, wine/runners, emulator sprawl |
| Heroic | Simple grid, platform filter, offline-friendly behavior, clean empty states | Store integrations, cloud saves |
| GOG Galaxy | Tags and collections, recently played, a calm detail page | Integrations platform, achievements sync |

The result is a quiet utility: a library you can scan in two seconds and a Play button that always
works.

#### 12.2.3 Screen map

```
+--------------------------------------------------------------------------+
|  SAPIE        [ Search games ]                 [ status ]  [ settings ]  |
+----------+------------------------------------------+--------------------+
| Library  |  Recently played                          |  Game detail       |
| Favorites|  +--------+ +--------+ +--------+         |  cover             |
| Installed|  | tile   | | tile   | | tile   |         |  title             |
| Recent   |  |  Play  | |  Play  | |  Play  |         |  Play              |
|          |  +--------+ +--------+ +--------+         |  status            |
| Friends  |                                           |  path / appid      |
|          |  All games                                |  repair / restore  |
|          |  +--------+ +--------+ +--------+         |  open folder       |
|          |  | tile   | | tile   | | tile   |         |                    |
|          |  +--------+ +--------+ +--------+         |                    |
+----------+------------------------------------------+--------------------+
|  Activity drawer: last action, errors, copy log                          |
+--------------------------------------------------------------------------+
```

Routes: `library`, `game/:id`, `friends`, `settings`. Add game is a modal, not a route. Sign in is a
full-window state, not a route.

#### 12.2.4 Library home

- Left rail: Library, Favorites, Installed, Recent, Friends, Settings.
- Top bar: search, server status pill, settings button. No fake notifications.
- Sections: Recently played (horizontal, max 6), All games (grid), Favorites (if any).
- Tile anatomy: cover image or monogram tile, title, one line of metadata (appid or last played),
  install state, and a Play button on hover or focus. The tile is a button; the Play action is a
  second control, not a nested button inside a button.
- Search filters by title and path. Filter chips: All, Installed, Not installed, Favorites.
- Sort: name, last played, recently added. Persist the choice.
- Empty library: one headline, one sentence, one primary action, "Add a game".
- Server offline: a thin banner, not a modal. Play still works from cached config.
- If a game path is missing: the tile stays, marked "Missing", with Locate and Remove actions.

#### 12.2.5 Game page

- Hero block with cover, title, Play, and a status line (Ready, Missing library, Game missing,
  Server offline).
- Details: executable path, arch, era, appid, interface count, installed library file, original
  backup state.
- Actions: Play, Repair library, Restore original, Open folder, Remove from library.
- Per-game settings: launch arguments, Linux preload override, appid override, SteamStub unpack.
- Achievements: read-only list from the server for this account and appid. Hidden achievements are
  hidden until unlocked.
- Activity: the last 10 launches with exit code and duration.

#### 12.2.6 Add game flow

One modal, three states, not six pages:

1. **Pick:** file picker, or drag a file onto the window.
2. **Review:** detected arch, era, appid guess, interface count, original library found. Every field
   is editable. Appid is prefilled only for known executables and is shown as a suggestion.
3. **Install:** one primary action, "Add to library", that backs up the original library, copies the
   matching SAPIE library, writes the config, and saves the record. A progress row shows each step.
   On failure, the previous state is restored.

Advanced options are collapsed: preload path, launch arguments, SteamStub unpack, force era. The
user can open them, but does not have to.

#### 12.2.7 Friends panel

- Read-only presence from the server: name, state, game, lobby, connect string.
- Actions: Join lobby when the friend is in one, Invite when a connect string is available.
- Incoming friend requests show as a badge and a compact list. Accept and decline call the API.
- Full management in the launcher: search by username, send request, cancel, accept, decline, remove
  friend, block and unblock. The web UI has the same actions; neither surface is a second-class path.
- No chat. No avatars unless the user sets one. No invented presence.

#### 12.2.8 Settings

- Account: persona name, SteamID, sign out, revoke local token.
- Server: host:port, TLS indicator, test connection.
- Appearance: system, light, dark.
- Updates: channel, check now, current version.
- Diagnostics: log location, copy log, open data folder.
- About: version, license, third-party notices.

#### 12.2.9 First run

1. Welcome, one sentence, "Get started".
2. Sign in. Real login call. Specific errors.
3. Empty library with "Add a game".
4. Add game modal.
5. Ready to play.

No account creation in the launcher unless the server allows registration. If registration is
closed, the screen says so and links to the web UI.

#### 12.2.10 Interaction and state rules

- One primary action per view.
- Returning users never re-enter a wizard. Play is one click from the library.
- Every long operation is async, shows progress, and can be cancelled where safe.
- No dead controls. A disabled control explains why next to it.
- Keyboard: tab order follows visual order, Esc closes modals, Enter activates the focused control,
  focus is always visible.
- Errors are specific: bad host, unreachable, wrong credentials, server too old, library missing,
  permission denied.
- Every surface ships empty, loading and error states. The library adds: missing game, library not
  installed, server offline, installing, launch failed.
- No password in any log, error string or crash report.

#### 12.2.11 Visual direction

Reading this as: a native game library for players who already own their games, in the same quiet
instrument-panel language as the web UI, dial ENERGY 2 / RHYTHM 2 / MOTION 2.

- Reuse `web/DESIGN.md` tokens through a shared `design/tokens.css`. One source of colour, type,
  spacing and radius.
- Default follows the OS theme. Ship light and dark, both checked for contrast.
- Ember marks Play, focus and live state. It is not a background wash.
- Cover art is user-provided or an honest monogram tile. Never invent game art.
- No fake glass. Native surfaces, hairline edges, one elevation level for modals.
- Motion: library enter, modal open, tile hover. No looping pulse, no per-card cascade.
- Identity motif: the monogram tile plus the ember Play affordance.
- The launcher has its own `design/DESIGN.md` with one-line reasons for each choice. It must cite
  and resolve the open findings in `anti-slop/audit-001-2026-11-20.md`.

#### 12.2.12 Launcher non-goals for 0.4

- Store, purchasing, downloads, DRM.
- Chat, voice, streaming.
- Cloud saves, workshop, inventory, overlay.
- macOS and Windows ARM64.
- Importing a Steam library or scanning all disks for games.

### 12.3 M13: Prove the core (gate)

This stage produces evidence, not features. It can run in parallel with M14, but no later stage may
be called done until M13 passes.

#### M13-a Real two-client C++ to Go session
- **Agent:** one worker, two machines or two VMs.
- **Risk:** yellow. **Priority:** must. **Depends:** none.
- **Scope:** build the modern and legacy DLLs, run two real game-side clients against one Go server.
  Exercise login, SteamID assignment, friends from the DB, presence, invite, lobby create and join,
  stats and achievement push.
- **Acceptance:** `docs/verification/0.4-two-client.md` with commands, client logs, server logs,
  distinct SteamIDs, matching presence, and one joined lobby. No crash, no silent fallback.

#### M13-b Real MW2 2009 run
- **Agent:** one worker, Windows machine.
- **Risk:** yellow. **Priority:** must. **Depends:** M13-a.
- **Scope:** install the DLL through the launcher (or manually if the launcher is not ready), unpack
  SteamStub if needed, set appid 10190, launch, sign in, verify SteamID and persona in the game and
  on the server, unlock one achievement, write one stat.
- **Acceptance:** evidence doc with the exact build, the DLL variant, the server log line, a game
  screenshot, and the persisted rows read back from SQLite.

#### M13-c P2P direct and relay
- **Agent:** one worker, two networks.
- **Risk:** yellow. **Priority:** must. **Depends:** M13-a.
- **Scope:** run a direct UDP session between two peers with a server hint. Then block UDP so the
  relay is chosen. Confirm one path per frame and that the relay only targets `dest_id`.
- **Acceptance:** logs showing the direct endpoint, then the relay path, with no broadcast and no
  cross-lobby delivery. Known limits recorded.

#### M13-d Regression and decision record
- **Agent:** one worker.
- **Risk:** green. **Priority:** must. **Depends:** M13-a, M13-b, M13-c.
- **Scope:** re-run `go test ./...`, `cargo test`, the 10 DLL builds, the web build, and the harness.
  Update `dev_notes.md` and section 11 of this plan with what was proven and what remains.
- **Acceptance:** green CI run linked, local results recorded, G0 language removed only where
  evidence exists.

### 12.4 M14: Launcher foundation (Tauri)

This stage replaces the egui shell. The detection logic and tests move, they are not rewritten.

#### M14-a Tauri v2 scaffold
- **Agent:** one worker.
- **Risk:** yellow. **Priority:** must. **Depends:** none.
- **Scope:** replace `client/src-tauri` egui code with Tauri v2, React 18, TypeScript, Vite 6. Add
  `tauri.conf.json`, capabilities, icons, CSP, no remote URLs. Keep the directory name until M21.
- **Acceptance:** `cargo tauri dev` opens a window on Linux. CI builds Windows x64 and Linux x64.
  `cargo test` still runs the detection tests. No remote content is loadable.

#### M14-b Rust command layer
- **Agent:** one worker.
- **Risk:** yellow. **Priority:** must. **Depends:** M14-a.
- **Scope:** port `detect_arch`, `scan_interfaces`, `detect_game`, `find_original_steam_api`,
  `copy_dll`, `configure_server`, `launch_game`, `unpack_steamstub`. Make them async where they do
  IO, emit progress events, and return typed errors. Add `restore_library` and `library_*` commands.
- **Acceptance:** unit tests cover detection, backup preference, arch parsing, appid, restore. A
  100 MB file detect does not block the UI thread.

#### M14-c Secure auth and token storage
- **Agent:** one worker.
- **Risk:** red. **Priority:** must. **Depends:** M14-b.
- **Scope:** `login` command calls `POST /api/v1/login`. Store the token in the OS keychain via the
  `keyring` crate. Never persist the password. Sign out clears the keychain entry. Handle expired
  tokens by returning to sign in. Add a real connection test.
- **Acceptance:** after login, `launcher-state.json` contains no password and no token. A token
  expiry returns a specific error and the sign-in screen. Tests cover the storage boundary.

#### M14-d Shared tokens and app shell
- **Agent:** one worker.
- **Risk:** green. **Priority:** must. **Depends:** M14-a.
- **Scope:** create `design/tokens.css` from `web/src/styles.css`, consume it in both web and
  launcher. Build the shell: rail, top bar, content, detail panel, activity drawer, toasts, offline
  banner. Add `design/DESIGN.md` with the launcher dials and reasons.
- **Acceptance:** no duplicated colour constants. Light and dark tokens resolve. Shell renders
  empty, loading and error states without data.

#### M14-e Packaging skeleton
- **Agent:** one worker.
- **Risk:** yellow. **Priority:** should. **Depends:** M14-a.
- **Scope:** Tauri bundler config for NSIS and MSI on Windows, AppImage and deb on Linux, icons,
  portable zip, updater signing keys placeholder. Do not ship the egui launcher in the bundle.
- **Acceptance:** CI produces one installable artifact per OS. A clean Linux container installs and
  starts the AppImage.

### 12.5 M15: Launcher product

#### M15-a Library grid
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M14-d.
- **Scope:** recently played, all games, favorites, search, filters, sort, tile states, monogram
  tile, user-chosen image, missing-path state.
- **Acceptance:** 0 games, 1 game and 40 games all render without overflow. Search and sort persist
  across restart. Keyboard reaches every tile and action.

#### M15-b Add game modal
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M14-b.
- **Scope:** pick, review, install as one modal. Editable arch, era, appid, preload. Collapsed
  advanced options. Per-step progress. Backup and rollback on failure. Duplicate detection.
- **Acceptance:** add a real game end to end. A failed copy leaves the original library in place. A
  duplicate path focuses the existing record instead of adding a second.

#### M15-c Game page and actions
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M15-a, M14-b.
- **Scope:** hero, status, details, Play, Repair, Restore original, Open folder, Remove, launch
  arguments, per-game settings, recent launches.
- **Acceptance:** restore puts the original file back and removes SAPIE's copy. Remove deletes the
  record but leaves the game files. Every action has a confirmation or an undo.

#### M15-d Launch pipeline and playtime
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M15-c.
- **Scope:** write config, spawn the process, watch for exit, record duration and exit code, handle
  spawn failure, keep the UI responsive. One click from a ready tile.
- **Acceptance:** launching a stub process records duration. A missing executable produces a
  specific error, not a crash. The launcher can launch twice without state corruption.

#### M15-e Friends panel
- **Agent:** one worker. **Risk:** yellow. **Priority:** should. **Depends:** M17-b, M17-d.
- **Scope:** friends list with presence, game, lobby, join and invite. Requests tab with send,
  cancel, accept and decline. Blocked tab with block and unblock. Remove friend with confirmation.
  Add friend by username search. All actions call the same API as the web UI.
- **Acceptance:** presence matches the web UI. A request sent from the launcher can be accepted in
  the browser, and the reverse. Remove and block take effect on both surfaces. No action requires
  the web UI.

#### M15-f Settings and logs
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M15-a.
- **Scope:** account, server, appearance, updates, diagnostics, about, sign out. Activity drawer
  with levels, copy, open log folder. Redact secrets.
- **Acceptance:** the log never contains a password or token. Sign out returns to the sign-in state
  and clears the keychain token.

#### M15-g Themes and accessibility
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M14-d.
- **Scope:** system, light, dark. Contrast measurement per theme. Keyboard-only pass. Reduced
  motion. Focus rings. 44px targets. Screen-reader labels on icon buttons.
- **Acceptance:** a keyboard-only user can sign in, add a game and launch it. Both themes pass AA at
  every text size. No focus trap.

#### M15-h Achievements and UI tests
- **Agent:** one worker. **Risk:** yellow. **Priority:** should. **Depends:** M15-c.
- **Scope:** read-only achievements and stats per game from the API. Vitest for components, a
  browser-mode harness with a mock API for flows. No Tauri driver in CI.
- **Acceptance:** hidden achievements stay hidden until unlocked. Tests cover sign in, add game,
  launch failure, empty library and offline banner.

### 12.6 M16: DLL and protocol

#### M16-a Token handoff
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M14-c, M17-e.
- **Scope:** add `string session_token = 5` to `LoginRequest` in `proto/sapie.proto` and the client
  copy. The server accepts a token instead of a password for the game protocol. The launcher writes
  a short-lived token file (`sapie_client.json`, mode 0600) next to the game. The DLL reads the file,
  falls back to `SAPIE_USERNAME`/`SAPIE_PASSWORD` only when the file is absent, and never logs either.
- **Acceptance:** the game logs in with the token file and no password env vars. An expired or
  revoked token gets a clear rejection. The proto copies pass `tools/check-proto-sync.sh`.

#### M16-b Appid and config from the launcher
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M16-a.
- **Scope:** read appid, era and preload from the launcher config and `steam_appid.txt`. Remove the
  MW2-only hardcode from the launcher. Add an explicit appid override. Log the chosen value.
- **Acceptance:** a non-MW2 game launches with its own appid. No file is written outside the game
  directory. Invalid input is rejected before the game starts.

#### M16-c Friends and request state in the mirror
- **Agent:** one worker. **Risk:** red. **Priority:** should. **Depends:** M17-b, M17-d.
- **Scope:** handle friend add, remove, block and request pushes. Update `local_mirror.h` and fire the
  existing friend callbacks. Do not block getters on the network.
- **Acceptance:** a friend added on the web appears in the game within one push. A removal clears
  presence and lobby access. No deadlock under the global lock.

#### M16-d Reconnect, status and errors
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M16-a.
- **Scope:** reconnect with backoff, heartbeat, token refresh, a status line in the launcher, and a
  clean fallback when the server is unreachable. Never spin on a hard failure.
- **Acceptance:** killing the server reconnects after restart. A revoked token stops retrying and
  surfaces an error. Logs show each attempt with a bounded count.

#### M16-e P2P hardening
- **Agent:** one worker. **Risk:** yellow. **Priority:** should. **Depends:** M13-c.
- **Scope:** measure the relay window from first inbound DATA, not connection creation. Add a relay
  rate limit and a per-frame path log. Decide and document IPv6 behavior.
- **Acceptance:** a slow handshake still prefers direct when DATA arrives. Relay abuse is capped.
  Path choice is visible in logs for both peers.

#### M16-f Interface coverage and client tests
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M16-a.
- **Scope:** run the interface generator for the test games, add any missing interfaces they need,
  keep ABI and era selection intact. Extend `sapie_login_smoke` with login, friend, request, lobby,
  stats and token cases.
- **Acceptance:** modern and legacy builds pass the smoke test. No interface regression for MW2 or
  Stardew.

### 12.7 M17: Social server

#### M17-a Schema migration 002
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** none.
- **Scope:** add `friend_requests` (id, from_id, to_id, message, created_at, status), `blocks`
  (steam_id, blocked_id, created_at), and indexes. Keep `friendships` as the accepted graph. Add a
  migration runner that is idempotent.
- **Acceptance:** a fresh DB and a 0.2 DB both migrate. FKs cascade. Tests cover duplicate
  requests, self requests, blocked pairs and accepted requests.

#### M17-b Friend request API
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M17-a.
- **Scope:** `GET /api/v1/friends/requests`, `POST /api/v1/friends/requests`,
  `POST /api/v1/friends/requests/{id}/accept`, `POST /api/v1/friends/requests/{id}/decline`,
  `DELETE /api/v1/friends/requests/{id}`. Idempotency keys, rate limits, no enumeration leak.
- **Acceptance:** send, accept and decline work for two accounts. Accept creates both friendship
  directions once. Duplicate sends are idempotent. A blocked pair cannot send.

#### M17-c Remove, block and search
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M17-b.
- **Scope:** `DELETE /api/v1/friends/{steam_id}`, `POST /api/v1/friends/{steam_id}/block`,
  `DELETE /api/v1/friends/{steam_id}/block`, `GET /api/v1/friends/blocked`,
  `GET /api/v1/users/search?q=`. Search is exact username or a short prefix, never a full dump.
- **Acceptance:** remove deletes both directions and ends invites. Block hides presence and rejects
  requests. Search does not reveal password hashes or full account rows.

#### M17-d Presence and invite gating
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M17-b.
- **Scope:** push a friend snapshot when the graph changes. Reject invites and P2P signaling between
  blocked or non-friend accounts. Keep the existing lobby gate.
- **Acceptance:** a new friend appears without a restart. A blocked peer cannot invite, join or
  relay. Tests cover each rejection path.

#### M17-e Logout, revoke and session hygiene
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M17-a.
- **Scope:** `POST /api/v1/logout`, `POST /api/v1/logout-all`, `GET /api/v1/sessions`, revoke one
  session, cleanup expired rows on a timer, rotate on login. Keep tokens hashed at rest.
- **Acceptance:** logout invalidates the token immediately. Logout-all invalidates every device.
  Expired rows do not accumulate. Tests cover replay of a revoked token.

#### M17-f Social tests and API docs
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M17-b to M17-e.
- **Scope:** table-driven tests for every endpoint, error code and idempotency path. Update
  `docs/PROTOCOL.md` and add `docs/API.md` with requests, responses and auth.
- **Acceptance:** `go test ./...` covers every new route. Docs match the handler names and payloads.

### 12.8 M18: Web UI

#### M18-a API client
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M17-b, M17-c.
- **Scope:** typed functions and errors for requests, accept, decline, remove, block, unblock,
  search, logout and sessions. Remove `password` from `Session`. Store the token in memory only.
- **Acceptance:** `tsc --noEmit` passes. A 401 anywhere returns the user to sign in. No password is
  kept after login.

#### M18-b Friends and requests screen
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M18-a.
- **Scope:** tabs Friends, Requests, Blocked. Send request by username, incoming and outgoing lists,
  accept, decline, cancel, remove, block, unblock. Empty, loading, error and busy states. Confirmation
  for remove and block.
- **Acceptance:** two browser sessions can send, accept and remove a friend. Blocked users disappear
  from presence. No control is enabled without a behavior.

#### M18-c Logout and session expiry
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M18-a.
- **Scope:** visible sign out in the rail or account menu. Handle 401 globally. Clear state, return to
  login, no token in the URL or localStorage.
- **Acceptance:** sign out clears the screen and the next request fails cleanly. An expired token
  shows one message, not a loop.

#### M18-d Games route decision
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** none.
- **Scope:** remove the Games rail item for 0.4 because the launcher owns games. If a server library
  endpoint lands, replace it with real read-only data instead. Do not keep the placeholder.
- **Acceptance:** no rail item leads to a page whose only content is "handled by the launcher".

#### M18-e Persona and account
- **Agent:** one worker. **Risk:** green. **Priority:** should. **Depends:** M18-a.
- **Scope:** edit persona name, view own stats and achievements, admin view of any account on the
  loopback admin UI. Keep admin paths off 8081.
- **Acceptance:** a persona change appears in the friends list after refresh. Admin routes still
  404 on 8081.

#### M18-f Theme
- **Agent:** one worker. **Risk:** green. **Priority:** should. **Depends:** M14-d.
- **Scope:** working light and dark themes, default follows the OS, toggle in the rail, tokens in one
  place. If 0.4 ships dark-only, write the product reason in `web/DESIGN.md` and remove the claim
  that a toggle is expected.
- **Acceptance:** both themes pass AA. The toggle persists. No component breaks in light mode.

#### M18-g Accessibility and responsive pass
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M18-b.
- **Scope:** keyboard order, focus rings, Esc on modals, 44px targets, labels, live regions, mobile
  layout, no horizontal overflow. Review the open antislop findings.
- **Acceptance:** keyboard-only flow works at 360px and 1440px. No contrast failure. Recorded
  click-through.

#### M18-h Browser tests and brand consistency
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M18-b, M18-c.
- **Scope:** Playwright suite for login, friends, requests, lobby, empty and error states, theme.
  Replace "Steam API Emulator" with SAPIE in titles and headings. Add the logo placeholder rule.
- **Acceptance:** CI runs the suite against a test server. No page title says the old name. No
  fabricated content.

### 12.9 M19: Security

#### M19-a Secret storage
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M14-c, M18-a.
- **Scope:** audit every place a password or token can land: launcher JSON, web state, logs, env
  vars, crash output, URL, error messages. Move the launcher to keychain only. Web keeps the token
  in memory. Redact logs.
- **Acceptance:** grep of the state file and logs finds no password or token. A deliberate error
  path does not print a secret. The token file is 0600.

#### M19-b TLS path
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M17-e.
- **Scope:** add `--tls-auto` self-signed generation, or document a cert path, and serve the web UI
  over HTTPS. Add a clear warning when the UI runs over plain HTTP. Ensure no mixed content. The
  admin API stays loopback.
- **Acceptance:** the browser UI loads over HTTPS without warnings after trusting the cert. Login
  and friends work. The plain HTTP mode is labeled insecure in the UI.

#### M19-c Session and transport hardening
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M17-e.
- **Scope:** per-IP and per-account limits, lockout, constant-time compares, frame caps, idle
  timeout, token rotation, revoke on password change. Review the game listener for unbounded reads.
- **Acceptance:** tests cover brute force, oversized frames, replayed tokens and idle disconnect.
  No secret is stored in plaintext at rest.

#### M19-d Docker that actually serves the UI
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M18-d.
- **Scope:** multi-stage image builds `web/dist`, copies it to `/app/web/dist`, runs the server with
  `--ui-dir /app/web/dist`, exposes 8081, runs non-root, adds a healthcheck, updates compose ports
  and volume. Add a compose profile for TLS if M19-b lands.
- **Acceptance:** `docker compose up` serves login at `http://localhost:8081`. The admin API stays on
  loopback. The container restarts cleanly with the data volume.

#### M19-e Threat model and dependency audit
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M19-a to M19-d.
- **Scope:** `docs/SECURITY.md` with the trust boundary, LAN cleartext, env var exposure, admin token,
  replay, and what is explicitly not protected. Add `govulncheck`, `npm audit`, `cargo audit`, and
  Dependabot.
- **Acceptance:** the doc names each known limit. CI runs all three audits. No high finding is
  silently ignored.

### 12.10 M20: CI and release

#### M20-a Workflow split
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** none.
- **Scope:** `ci.yml` for push and PR (server, client, launcher, web, rust checks, tests, lint).
  `release.yml` only for plain numeric `0.*` tags. Remove duplicated full matrices from `build.yml`. Cache Go,
  Cargo, npm and protobuf.
- **Acceptance:** a PR runs every check once. A tag runs the release path once. No tag other than
  plain numeric `0.*` triggers a release.

#### M20-b Version stamping
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M20-a.
- **Scope:** derive the version from the tag for server ldflags, `tauri.conf.json`, `package.json`
  and the release notes. Fail the build on a mismatch.
- **Acceptance:** the plain numeric `0.4` tag stamps 0.4.0 in the launcher About and the web
  footer. No hardcoded 0.1.0 remains.

#### M20-c Launcher build and signing
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M14-e, M15-h.
- **Scope:** build Tauri on Windows and Linux in CI, upload installers, sign Windows if a cert secret
  exists, generate the updater manifest, keep unsigned builds labeled. Cross-compiled Windows
  execution is not required, but a Windows runner build is.
- **Acceptance:** the tag attaches a Windows installer, a Linux AppImage and a deb. Unsigned builds
  say so. The updater manifest points at the release.

#### M20-d Tests in CI
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M17-f, M18-h, M16-f.
- **Scope:** Go tests and vet, C++ smoke, Rust tests and clippy, TypeScript build, Vitest, Playwright,
  proto sync, tracked build product check. Fail on warnings that are treated as errors by the project.
- **Acceptance:** a red test blocks merge. The suite runs under 20 minutes with caches.

#### M20-e Bundles, container and release checklist
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M20-c, M19-d.
- **Scope:** update `tools/release-bundle.sh` for the Tauri launcher name and new files. Add
  checksums, a third-party notices file, and an SBOM. Publish the container with the web UI. Write
  `docs/RELEASE.md`.
- **Acceptance:** a dry-run tag produces the three archives plus the container. The archive contains
  the launcher, server, 10 libraries, web/dist, docs and notices. The checklist is followed.

### 12.11 M21: Docs, cleanup and tag

#### M21-a Docs
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M20-e.
- **Scope:** update README, QUICKSTART, BUILD, PROTOCOL, UI, AGENTS, `web/DESIGN.md`, add
  CONTRIBUTING, SECURITY, CHANGELOG. Remove claims that are no longer true.
- **Acceptance:** every command in the docs runs. No doc references a 0.2-only path or the egui
  launcher.

#### M21-b Remove the egui launcher and rename
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M15-h.
- **Scope:** delete the egui source and its tests after the detection tests are ported. Rename
  `client/src-tauri` to `launcher/`. Update CI, docs, release script and `.gitignore`. Remove
  committed Tauri `gen/schemas` if the build regenerates them.
- **Acceptance:** the repo has one launcher. A clean clone builds it. No dead path references.

#### M21-c Release candidate verification
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M13, M19, M20.
- **Scope:** re-run the M13 evidence against the release candidate. Fresh install on Windows and
  Linux, add game, launch, friend request, accept, remove, block, logout, TLS, container, both
  themes, keyboard pass.
- **Acceptance:** a signed-off checklist with logs and screenshots. Any regression blocks the tag.

#### M21-d Tag and notes
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M21-c.
- **Scope:** bump to 0.4.0, update CHANGELOG, tag, publish, update `dev_notes.md` with known limits,
  move 0.2 details to the history section if needed.
- **Acceptance:** release assets exist, the container is pullable, the notes state what was and was
  not verified.

### 12.12 Full change inventory

This is the checklist of everything 0.4 touches. A phase may not skip an item without writing why.

#### Server files and surfaces

- `server/migrations/002_social.sql`: `friend_requests`, `blocks`, indexes.
- `server/migrations/schema.go`: embed and run migration 002, idempotent.
- `server/internal/state/state.go`: `SendFriendRequest`, `FriendRequestsOf`, `AcceptFriendRequest`,
  `DeclineFriendRequest`, `CancelFriendRequest`, `RemoveFriend`, `BlockAccount`, `UnblockAccount`,
  `BlockedOf`, `IsBlocked`, `SearchAccounts`, `RevokeSession`, `RevokeAllSessions`, `SessionsOf`,
  `CleanupExpiredSessions`.
- `server/internal/api/api.go`: request, accept, decline, cancel, remove, block, unblock, search,
  logout, logout-all, sessions routes and handlers. Idempotency and rate limits. Tests for each.
- `server/internal/session/session.go`: token login for the game protocol, friend snapshot pushes,
  reconnect state, status logging.
- `server/internal/router/router.go`: reject blocked and non-friend invites and signaling; keep the
  lobby gate; relay only `dest_id`.
- `server/internal/ui/ui.go`: allowlist the new user routes on 8081; never expose admin routes.
- `server/cmd/sapie-server/main.go`: `--tls-auto`, session cleanup, version flag, any new defaults.
- `proto/sapie.proto`: `LoginRequest.session_token`, friend request and friend event frames if the
  push is not carried by the existing `Common_Message` path.
- `client/dll/sapie.proto`: mirror of the proto change; `tools/check-proto-sync.sh` stays green.
- `server/internal/protocol/sapie.pb.go`: regenerated.
- `server/internal/**/*_test.go`: new cases for every route, migration and rejection path.

#### Web files and surfaces

- `web/src/api.ts`: remove password from `Session`, add requests, accept, decline, cancel, remove,
  block, unblock, search, logout, sessions. Typed errors.
- `web/src/components/FriendList.tsx`: split into friends, requests, blocked, search and add form.
- `web/src/components/FriendRequests.tsx`: new.
- `web/src/components/BlockedList.tsx`: new.
- `web/src/components/AddFriend.tsx`: new.
- `web/src/components/AccountMenu.tsx`: sign out and session info.
- `web/src/screens/Shell.tsx`: rail, remove or replace Games, account menu, request badge.
- `web/src/screens/Login.tsx` and `Register.tsx`: no password in session, 401 handling.
- `web/src/screens/GameDetail.tsx`: remove if the Games route is removed.
- `web/src/routes.ts`: route list matches the rail.
- `web/src/styles.css`: light theme, new components, request badge, account menu.
- `web/src/components/Icons.tsx`: add only icons with a real meaning.
- `web/DESIGN.md`: dark decision or light theme, new surfaces, one-line reasons.
- `web/package.json`: version 0.4.0, test scripts.
- `web/src/**/*.test.tsx`, `web/e2e/*.spec.ts`: new.

#### Launcher files and surfaces

- `client/src-tauri/tauri.conf.json`: product name, version, window, CSP, bundle targets.
- `client/src-tauri/Cargo.toml`: Tauri v2, commands, keyring, serde, no egui.
- `client/src-tauri/capabilities/*.json`: least privilege.
- `client/src-tauri/src/lib.rs`, `main.rs`: app setup, state, commands.
- `client/src-tauri/src/commands/detect.rs`, `install.rs`, `launch.rs`, `auth.rs`, `library.rs`.
- `client/src-tauri/src/detect.rs`: ported tests from the egui file.
- `client/src-tauri/src/keychain.rs`: token storage and clearing.
- `client/src-tauri/ui/`: React app, screens, components, tokens.
- `client/src-tauri/ui/screens/Library.tsx`, `Game.tsx`, `Friends.tsx`, `Settings.tsx`, `SignIn.tsx`.
- `client/src-tauri/ui/components/GameTile.tsx`, `AddGameModal.tsx`, `FriendList.tsx`,
  `FriendRequests.tsx`, `BlockedList.tsx`, `AddFriend.tsx`, `AccountMenu.tsx`, `StatusPill.tsx`.
- `design/tokens.css`: shared token source.
- `design/DESIGN.md`: launcher direction.
- `client/src-tauri/resources/steamless/`: unchanged, notice kept.
- `client/src-tauri/icons/`: real app icon, not the Tauri default.
- Installer and updater config.

#### DLL files and surfaces

- `client/dll/settings_parser.cpp`: read `sapie_client.json` (token, appid, era, preload), then
  `steam_appid.txt`, then env fallback.
- `client/dll/settings.h`: new fields for token, appid source, config path.
- `client/dll/network.cpp` and `.h`: token login, reconnect, friend and request events, status,
  relay window fix, rate limit.
- `client/dll/steam_friends.h`: apply friend graph changes, presence, blocks.
- `client/sapie/local_mirror.h`: friend and request state, thread safety.
- `client/dll/base.cpp`: server-assigned SteamID stays, no local generation.
- `client/dll/dll.cpp`: unchanged era selection unless a new interface is added.
- `client/dll/CMakeLists.txt`: new sources if any.
- `client/harness/sapie_login_smoke.cpp`: token, friend, request, lobby, stats cases.
- `tools/harness/`: two-client scenario if it can run headless.

#### CI, release and container

- `.github/workflows/ci.yml`: PR and push checks, one matrix, caches.
- `.github/workflows/release.yml`: plain numeric `0.*` tags only, artifacts, signing, release notes.
- Delete or rewrite `.github/workflows/build.yml` to remove duplication.
- `.github/dependabot.yml`: Go, npm, Cargo, Actions.
- `tools/release-bundle.sh`: Tauri launcher name, new files, checksums, notices.
- `Dockerfile`: web build stage, `/app/web/dist`, non-root, healthcheck, 8081.
- `compose.yaml`: 8081, TLS profile, data volume, healthcheck.
- `.gitignore`: `launcher/target`, Tauri generated output, no committed binaries.
- Version files: `client/src-tauri/Cargo.toml`, `web/package.json`, any server version constant.

#### Docs and repo hygiene

- `README.md`: 0.4 install, launcher, social, security limits.
- `docs/QUICKSTART.md`: no SQLite friend editing, no egui.
- `docs/BUILD.md`: Tauri deps, new CI, signing.
- `docs/PROTOCOL.md`: token login, friend events.
- `docs/UI.md`: new routes, theme, launcher split.
- `docs/API.md`: new.
- `docs/SECURITY.md`: new.
- `docs/RELEASE.md`: new.
- `docs/verification/0.4-two-client.md`: new.
- `AGENTS.md`: stack, commands, phase rules, launcher spec.
- `CONTRIBUTING.md`: new.
- `CHANGELOG.md`: new.
- `LICENSE`: add third-party notices for Tauri, WebView2, GSAP, Manrope and IBM Plex.
- `plan.md`: this section is the live plan; older sections are history.
- `dev_notes.md`: verified versus unverified after M13 and M21.
- Repo rename: `client/src-tauri` to `launcher/` in M21.

#### Tests that must exist by the tag

- Go: social routes, migration, token login, block gating, revoke, cleanup.
- C++: token file login, friend event apply, reconnect, smoke.
- Rust: detect, backup, restore, appid, keychain boundary, launch error.
- Web: Vitest components, Playwright flows, theme, 401.
- CI: proto sync, no tracked build products, version match.
- Live: two-client, MW2, direct P2P, relay, Windows install, Linux install.

### 12.13 Dependency lanes

| Lane | Phases | Rule |
| --- | --- | --- |
| Proof | M13-a, M13-b, M13-c, M13-d | Runs in parallel with launcher work, gates every "done" claim |
| Launcher shell | M14-a, M14-b, M14-d, M14-e | Serial on the same files |
| Launcher auth | M14-c then M15-f | Red, needs approval |
| Launcher product | M15-a, M15-b, M15-c, M15-d, M15-g, M15-h | M15-b needs M14-b, the rest can interleave |
| DLL | M16-a, M16-b, M16-d, M16-f | M16-a is the first red gate |
| DLL social | M16-c, M16-e | Needs M17-b and M13-c |
| Social server | M17-a, M17-b, M17-c, M17-d, M17-e, M17-f | Serial on schema then parallel on routes |
| Web | M18-a, M18-b, M18-c, M18-d, M18-e, M18-f, M18-g, M18-h | After M17 routes exist |
| Security | M19-a, M19-b, M19-c, M19-d, M19-e | After auth and web are stable |
| Release | M20-a to M20-e | Last code stage |
| Docs and tag | M21-a to M21-d | After release candidate |

### 12.14 Verification matrix

| Layer | Command or run | Gate |
| --- | --- | --- |
| Server unit | `cd server && go test ./... && go vet ./...` | Every PR |
| Server live | two-client session, MW2, direct P2P, relay | 0.4 tag |
| C++ build | 10 target-era builds | Every PR |
| C++ smoke | `sapie_login_smoke` modern and legacy | Every PR |
| Rust | `cargo test`, `cargo clippy`, `cargo fmt --check` | Every PR |
| Web static | `tsc --noEmit`, `vite build` | Every PR |
| Web browser | Playwright flows, keyboard, both themes | Every PR |
| Container | `docker compose up`, login on 8081 | Release |
| Security | `govulncheck`, `npm audit`, `cargo audit` | Every PR |
| Release | tag dry run, checksums, archive contents | Release |

### 12.15 0.4 release definition

0.4 is shippable when all of these are true:

- M13 evidence exists for two clients, MW2, direct P2P and relay.
- A new user installs the launcher, signs in, adds one game and presses Play without reading docs.
- A returning user presses Play once.
- The password is never written to disk, logged, or placed in a URL.
- Add, accept, remove and block work from the web UI, and the game sees the change.
- The container serves the web UI on 8081 with the admin API still on loopback.
- The tag produces versioned Windows and Linux artifacts plus the container.
- `anti-slop/audit-001-2026-11-20.md` findings 1, 2, 3 and 4 are fixed or explicitly accepted.
- `dev_notes.md` states what was and was not verified.

### 12.16 Out of scope for 0.4

Store, purchasing, cloud saves, chat, voice, streaming, workshop, inventory, overlay, controller,
macOS, Windows ARM64, 32-bit ARM, hole punching, app ownership enforcement, importing a Steam
library, and full friend management inside the launcher. These are 0.5 candidates and are listed
here so a phase cannot quietly grow into them.
