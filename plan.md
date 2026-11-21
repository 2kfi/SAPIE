# Steam API Emulator (SAPIE) — Build Plan

> Current plan: section 13 (0.5). Section 12 is the 0.4 plan and stays as history. Sections 1 to 11
> are the 0.1 and 0.2 history and remain as a decision record.

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

### Current milestone status (2026-11-20 documentation refresh)

This is a status readout of the tree at `b36e7ec` on `dev-client`, not a new decision. The stage map
and phase specs below remain the historical plan.

- M13: headless two-client C++ to Go evidence is recorded in `docs/verification/0.4-two-client.md`.
  Real MW2 and two-network P2P/NAT runs are UNVERIFIED. The harness is not deterministic.
- M14 to M18: the Tauri launcher, product UI, 0600 token handoff, social API and web UI are present
  in the tree. Native window, IPC, keychain and browser click-through were not exercised headless.
- M19: security guide and hardening are in `docs/SECURITY.md`; the container serves the web UI on
  8081. Residual risks and dependency advisories are reported, not silently ignored.
- M20: `ci.yml` and `build.yml` exist. `build.yml` fires only on plain numeric `0.*` tags. No
  tag-triggered release run or Windows bundle has been confirmed here.
- M21: docs refresh and the draft release-candidate checklist are in progress. No `0.4` tag or
  publish has happened.

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

---

## 13. 0.5 plan (next release)

### 13.0 What 0.5 is

0.4 proved the core, built a Tauri launcher and produced installers. 0.5 is the release where
SAPIE gets a face, two homes and a launcher that is a real desktop application.

The 0.4 launcher is a webview app. 0.5 removes the webview entirely. Both launchers are Qt 6 + Qt
Quick, rendered by the Qt scene graph, with a Rust core behind a C ABI. No WebView2, no WebKitGTK,
no embedded browser, no remote content.

Outcomes:

1. **Identity.** One visual and verbal system for the launcher, the web UI, the docs and the
   installers. Not a GOG clone. A recognisable SAPIE.
2. **Two launchers.** `SAPIE for Windows` and `SAPIE for Linux`. Same Qt Quick UI, same Rust core,
   separate platform code, installers and update channels.
3. **A finished library.** Covers, collections, favorites, recently played, playtime, a game page
   with achievements, full friend management and one-click play.
4. **Distribution.** Signed Windows installers, an AppImage, a deb, a Flatpak, a working server
   container, auto-update with rollback, and a release a normal person can install.

0.5 is scoped to identity, the two launchers, the product features around them, and the 0.4
security debts. It is not a store, a chat client or a cloud platform. Those stay out. The Qt
rewrite replaces the 0.4 Tauri shell; it is not a third launcher on top of it.

### 13.1 Inputs from 0.4

The `0.4` tag exists and produced Windows NSIS/MSI and Linux AppImage installers. The tree records
that the installers were built, not executed. The following 0.4 limits are 0.5 must-fix items:

- Real MW2 run is UNVERIFIED.
- The 0.4 launcher is a Tauri webview shell. Its native window, real IPC, keychain access and
  webview navigation are UNVERIFIED. 0.5 replaces it with Qt rather than shipping it further.
- Keyboard-only and screen-reader passes are UNVERIFIED.
- NAT, direct UDP and relay with two real networks are UNVERIFIED.
- Installer execution on clean machines is UNVERIFIED.
- Session tokens are replayable for up to 24 hours; no rotation, no revoke-on-password-change.
- Login and social limiters are in memory and reset on restart.
- Blocked peers stay in lobby membership and can receive lobby frames.
- Windows has no explicit ACL on the token file; it inherits the directory ACL.
- The web UI is plain HTTP unless an operator adds TLS.
- The launcher has no auto-update path and no rollback.

These are not 0.5 features. They are the price of calling 0.5 done.

### 13.2 Project identity

Identity is not a logo. It is the set of decisions a user can recognise without seeing the name:
the mark, the palette, the type, the voice, the motion and the way a game tile behaves.

#### 13.2.1 Name and positioning

- The name is **SAPIE**, always uppercase in product surfaces. Do not expand the acronym in the UI.
  The README may explain it once.
- Product family: `SAPIE for Windows`, `SAPIE for Linux`, `SAPIE Server`, `SAPIE Web`.
- Public tagline: **Your games. Your server.**
- Descriptor: **Old Steam games, no Steam client.** This is the honest one-line explanation.
- The promise: you own the game, you own the machine, you own the server. SAPIE removes the
  dependency on Valve's client, not the game.
- The tone is quiet and factual. No hype, no "next generation", no "seamless", no fake scale.

#### 13.2.2 Mark and wordmark

The current ember `S` square is a placeholder. 0.5 replaces it with a mark that means something:

- Concept: a **keyhole with a play triangle in the negative space**. It joins "unlock" and "play",
  which is the product. It is not a gamepad, a robot, a sparkle or a diamond.
- Construction: one rounded square field, one keyhole cut, one triangle cut. Works at 16px as a
  solid silhouette and at 512px as a detailed icon.
- Wordmark: `SAPIE` set in Manrope ExtraBold, tight tracking, with the mark to the left. The wordmark
  is text, not custom lettering, until a type designer says otherwise.
- App icon: the mark on the brand background, one accent, no gradient, no glow.
- Tray icon: the silhouette at 16/22px, monochrome on Linux where the desktop asks for it.
- Monogram fallback for game covers: a deterministic geometric tile derived from the appid, not a
  fake cover. The same appid always produces the same tile.

The mark is a G1 artifact. Do not generate final art before G1 approves the direction. R-23 applies.

#### 13.2.3 Palette

The 0.5 palette follows `AGENTS-UI.md`, with the ember retired as the brand accent.

| Token | Value | Job |
| --- | --- | --- |
| `--bg-app` | `#17191D` | Application background, never pure black |
| `--bg-sidebar` | `#14161A` | Persistent rail |
| `--bg-panel` | `#1D2025` | Panels, cards, menus |
| `--bg-panel-hover` | `#252930` | Hover and quiet selection |
| `--bg-elevated` | `#292D34` | Dialogs and popovers |
| `--border-subtle` | `#2A2E35` | Hairline separators |
| `--border-strong` | `#383E47` | Strong boundaries and focus containers |
| `--text-primary` | `#E6E8EB` | Main text |
| `--text-secondary` | `#A0A5AE` | Secondary text |
| `--text-muted` | `#737984` | Metadata |
| `--accent` | `#C5A4FF` | Brand accent, active nav, focus, primary action |
| `--accent-soft` | `#342A46` | Selected background |
| `--success` | `#73C991` | Online and success |
| `--warning` | `#E6B86C` | Away and warning |
| `--danger` | `#E17B7B` | Errors and destructive actions |
| `--info` | `#82B8E8` | Informational status |

Rules: one accent, used for active navigation, focus, selection and the primary action. Green, amber,
red and blue are status only. No gradients as a default. No glow. No glass. The ember is retired
from brand use; if a warm tone survives it is a status warning, not identity.

#### 13.2.4 Type

- UI: **Manrope**, with the platform stack as fallback. This is a deliberate override of the
  `AGENTS-UI.md` suggestion of Inter. Inter is the default AI choice and would erase the one type
  decision SAPIE already owns. Manrope is compact in dense rows and already in the product.
- Machine values: **IBM Plex Mono** for paths, appids, SteamIDs, lobby IDs and logs.
- Qt cannot load woff2. Ship Manrope and IBM Plex Mono as TTF or OTF inside the Qt resource file,
  and woff2 in the web build. No remote font import. Both apps must render the intended face on a
  clean machine.
- Scale: 12, 13, 14, 16, 20, 24. Page titles 20 to 24, card titles 13 to 14, metadata 11 to 12.
  No oversized marketing headings.
- Weights: 400, 500, 600, 700. No 800 or 900.

#### 13.2.5 Voice

Short, factual, second person. The user is "you". The server is "your server". The product does not
brag and does not apologize.

| Instead of | Write |
| --- | --- |
| Get Started | Add a game |
| Seamless experience | Your server, your games |
| Oops! Something went wrong | The server did not answer. Check the address and try again. |
| Unlock your gaming potential | Play the games you already own |
| AI-powered | (delete) |

No em dash. No exclamation marks in system messages. No fake numbers. No fake testimonials. No fake
players. The full copy rules live in `design/identity.md`.

#### 13.2.6 Motion and sound

- Motion: 120 to 180ms on hover, 140 to 200ms on menus and dialogs, ease-out entering, ease-in
  leaving. No bounce, no spring, no looping background.
- One first-mount entrance in the launcher. No per-card cascade.
- Respect the OS reduced-motion setting. Qt does not expose it as a QML property, so the platform
  layer reads it (Windows `SPI_GETCLIENTAREAANIMATION`, the desktop portal or the GTK setting on
  Linux) and exposes `Platform.reducedMotion`. Fall back to an in-app toggle when the OS value is
  unavailable.
- Sound: none. A launcher that beeps is a toy.

#### 13.2.7 Art and covers

- Cover art is the main colour source. Portrait 2:3, `object-fit: cover`.
- Missing art uses the monogram tile, never a fake store cover, never a generic gradient blob.
- The user can set a cover from a local image. Store the path and a cached thumbnail, not a remote
  URL.
- No invented game metadata. If the server does not know the game name, show the executable name.

#### 13.2.8 Platform expression

Same identity, different manners:

- **Windows:** native title bar in 0.5, Windows system dialogs, Segoe UI fallback, taskbar progress,
  jump list, per-user install by default. No fake Mica, no custom chrome that breaks accessibility.
- **Linux:** native title bar, GTK file picker, desktop entry, AppStream metadata, AppIndicator tray
  where available. No fake macOS traffic lights.
- The in-app surfaces are identical. The difference is integration, not branding.

#### 13.2.9 Surfaces that carry the identity

- Launcher: icon, tray, About, installer, empty states, monogram tiles.
- Web UI: same tokens and components, no separate theme.
- Docs: README banner, screenshots with real data only, a short identity page.
- Releases: one release-notes template, one artifact naming scheme, one checksum format.
- GitHub: social preview, issue templates with the same voice.
- Server: startup banner and admin page use the wordmark, not ASCII art.

#### 13.2.10 Identity governance

- `design/identity.md` is the source of truth for name, mark, palette, type, voice and motion.
- `design/tokens.css` is the only place colour, type, spacing and radius values live.
- `design/DESIGN.md` holds the screen-level direction and the one-line reason for each major choice.
- Any change to the mark, accent or type needs a G1 review. Do not drift one screen at a time.
- The clone test: swap the logo and the product name. If it could be GOG, Steam or Heroic, it is too
  close. The layout language may be inspired by them. The mark, type, voice and monogram system must
  be SAPIE's.

### 13.3 UI direction

`AGENTS-UI.md` is the visual base for 0.5. It replaces the glass instrument panel in
`web/DESIGN.md` and `design/DESIGN.md`. The launcher implements it in QML. The web UI implements it
in React. They share tokens and behaviour, not component code.

Adopt from it:

- Dark, low-glare, desktop-first frame. Background `#17191D`, panels `#1D2025`.
- Persistent sidebar, 164px at a 1440px baseline, 1px divider, quiet active state.
- Compact 42 to 48px toolbar. Small 28 to 32px icon buttons.
- Cover-art-first library, 5 columns at desktop, 2:3 portrait, 12 to 16px gaps.
- 4 to 6px radius on cards and controls. One level of shadow, only on dialogs and menus.
- Quiet violet accent, used sparingly.
- Real empty, loading and error states. Skeletons match the final layout.
- Content dominates, chrome stays quiet.

Deliberate overrides:

- **Type stays Manrope**, not Inter, for the reason in 13.2.4.
- **No glass** anywhere, even though the old web UI uses it.
- **Dark only** in 0.5. The reason is recorded in `design/DESIGN.md`. A light theme is a separate
  decision with its own contrast evidence.
- **No store, download or install UI** unless the feature exists. The launcher adds games from
  disk; it does not download them.
- **No fake art** and no invented game metadata.

#### 13.3.1 Screens

| Screen | Content | Primary action |
| --- | --- | --- |
| Sign in | Server, username, password, connection status | Sign in |
| Library | Recently played, all games, favorites, search, filters, sort | Play |
| Game | Cover, title, status, Play, details, achievements, recent launches | Play |
| Add game | Pick, review, install, advanced options | Add to library |
| Friends | Online, away, offline, search | Add friend |
| Requests | Incoming, outgoing, accept, decline, cancel | Accept |
| Blocked | Blocked accounts, unblock | Unblock |
| Lobbies | Joinable lobbies, member count, join | Join |
| Settings | Account, server, updates, appearance, diagnostics | Save |
| About | Version, license, notices, log folder | Copy diagnostics |
| Logs | Activity, levels, copy | Copy log |

No screen exists in the rail until it exists as a surface. No disabled fake destinations.

#### 13.3.2 Component primitives

Build these once as QML components in `launcher/ui` and use them in both launchers. The web UI
keeps its React components and takes only the tokens and the behaviour rules.

- `AppShell`, `Sidebar`, `SidebarGroup`, `SidebarItem`, `Toolbar`, `PageHeader`.
- `GameCard`, `GameGrid`, `GameArt`, `MonogramTile`, `PlayButton`, `StatusPill`.
- `FriendRow`, `FriendGroup`, `RequestRow`, `BlockedRow`, `Avatar`, `PresenceDot`.
- `LobbyRow`, `LobbyBadge`, `MemberCount`.
- `Button` (primary, quiet, danger), `IconButton`, `Input`, `SearchField`, `Select`, `Checkbox`.
- `Dialog`, `Drawer`, `Toast`, `Tooltip`, `Menu`.
- `EmptyState`, `LoadingSkeleton`, `ErrorState`, `OfflineBanner`.
- `Section`, `Divider`, `KeyValue`.

Every primitive ships with default, hover, active, focus, disabled and loading where relevant. Each
one sets `Accessible.role`, `Accessible.name` and the focus policy, because a QML control without
them is invisible to a screen reader.

#### 13.3.3 Accessibility

- Keyboard reachable in visual order. Focus ring is a 2px accent outline at 2px offset.
- 44px minimum targets on touch-capable surfaces, 32px minimum on desktop toolbars.
- Status is never colour alone: dot plus text.
- Dialog focus trap, Escape closes, focus returns to the trigger.
- Screen-reader names on every icon button. In QML this means explicit `Accessible` properties,
  not the text label alone.
- QML accessibility is opt-in. A custom `Item` or a styled `Control` is silent to the screen reader
  until it declares a role, a name and its state. Treat that as part of the component, not a later
  audit.
- Verify through the platform bridge: Windows UI Automation, and AT-SPI with Orca on Linux.
- Contrast AA at every text size in the dark palette.
- Reduced motion respected in both shells.
- The web UI must work at 360px. The launcher must work at 860x560.

#### 13.3.4 The G1 prototype

Before production UI work, build a clickable QML prototype with the real Qt Quick shell, the real
component primitives and real empty states. Use one real local game record if a developer has one.
Do not invent games, players, stats or covers. G1 signs off the prototype, then M23 to M28 build it.

### 13.4 The two launchers

#### 13.4.1 Decision

Build **two launcher products on one Rust core and one Qt Quick UI**:

- `SAPIE for Windows`
- `SAPIE for Linux`

They share the Rust core, the QML UI, the design tokens and the update manifest format. They do not
share installers, platform integration or release gates.

**The webview is gone in both.** The 0.4 launcher embedded a browser. 0.5 runs Qt 6 + Qt Quick and
draws its own scene. There is no WebView2, no WebKitGTK, no HTML, no CSS and no browser JavaScript
runtime in the launcher. QML is a declarative UI language, not a web page. The only web surface left
in the project is the LAN web UI, which is a browser page on purpose.

Why drop it:

- The user wants a real launcher, not a browser wrapped in a window frame.
- It removes a runtime dependency on Windows and a system library stack on Linux.
- It removes the whole HTML, CSS and JS layer from the desktop trust boundary.
- Native file dialogs, tray, accessibility bridge, credential storage and installers stop being
  adapters around a webview and become the app.
- Qt Quick is GPU rendered and handles a cover grid and animated states directly.

What it costs, stated honestly: Qt adds roughly 40 to 60 MB installed, a C++ build step and LGPLv3
obligations. Section 13.4.6 covers the licence work. That is the price of a real app.

Why not WinUI 3 on Windows plus GTK4 on Linux: that doubles the UI code, the design system, the
tests and the maintenance. One Qt Quick codebase already reaches both desktops. Native feel comes
from platform integration, not from a second toolkit. A per-platform toolkit rewrite is a 0.6
track, only if Qt proves insufficient.

Why not keep Tauri on Windows and Qt on Linux: that is two UI codebases and two design systems for
one product. Rejected.

What "native" means in this plan: Qt Quick scene graph rendering, a native window frame, native file
dialogs, a native tray, the native accessibility bridge, the OS credential store and a native
installer. It does not mean a stock widget look; the SAPIE design system owns the visuals.

#### 13.4.2 Stack

| Layer | Windows | Linux | Shared |
| --- | --- | --- | --- |
| App shell | Qt 6.8 LTS + Qt Quick, MSVC 2022 | Qt 6.8 LTS + Qt Quick, GCC or Clang | Qt 6 |
| UI | QML + Qt Quick Controls | same | `launcher/ui` |
| Core | Rust `sapie-launcher-core` | same | `launcher/core` |
| Bridge | C ABI plus a C++ `CoreBridge : QObject` | same | `launcher/bridge` |
| Rendering | Qt Quick scene graph, GPU with software fallback | same | Qt |
| Secrets | Windows Credential Manager | Secret Service (GNOME Keyring or KWallet) | `keyring` crate |
| Library store | JSON with atomic write and schema version | same | `launcher/core` |
| Packaging | `windeployqt`, NSIS, MSI, portable zip, MSIX later | `linuxdeploy` + Qt plugin, AppImage, deb, Flatpak | CI |
| Updates | Signed manifest, installer apply | Signed manifest, AppImage replace, Flatpak | shared manifest format |
| Native bits | Steamless, jump list, taskbar progress, ACL, `sapie://` | LD_PRELOAD, desktop entry, MIME, AppStream | platform interface |
| Signing | Authenticode (OV/EV or Azure Trusted Signing) | GPG or AppImage signature, Flatpak | CI secrets |
| Build | CMake 3.24+, MSVC, Rust, `windeployqt` | CMake 3.24+, Ninja, Rust, `linuxdeploy` | CMake |
| Minimum OS | Windows 10 1809 and 11 | Ubuntu 22.04+, Fedora 40+, Arch | document per release |

The bridge is deliberately small. The Rust core exposes a versioned C ABI (`sapie_core.h`) with
command functions, an event callback and opaque handles. One hand-written `CoreBridge : QObject`
wraps it for QML with `Q_INVOKABLE` methods and signals. No cxx-qt, no generated bindings, no IPC
socket and no JSON command channel on the hot path. The network protocol to the SAPIE server is
unchanged; this is only the UI-to-core boundary. If the surface ever outgrows one header, cxx-qt is
the upgrade path, not the starting point.

#### 13.4.3 Directory layout

```
launcher/
├─ CMakeLists.txt             # top-level build, Qt 6 and Rust core
├─ Cargo.toml                 # Rust workspace
├─ core/                      # Rust: auth, detect, install, config, launch, library, updates
│  ├─ include/sapie_core.h    # the C ABI, the only contract QML sees
│  └─ src/
├─ bridge/                    # C++ CoreBridge QObject over the C ABI
├─ ui/                        # QML: Tokens.qml, components/, screens/, icons/, fonts/, qrc
├─ app/                       # shared main.cpp, application setup, platform interface
├─ windows/                   # Win32 integration, .rc, NSIS/MSI scripts, windeployqt
├─ linux/                     # XDG integration, desktop entry, metainfo, AppImage/Flatpak
└─ tests/                     # fake platform, fixtures, QML tests, screenshots
```

The Rust core inside `client/src-tauri` migrates into `launcher/core`. The Tauri shell and its React
UI are deleted, not ported. The DLL, server, web and proto keep their current paths. Design lives in
the top-level `design/` directory, not inside `launcher/`, so the web UI and the docs consume the
same tokens and icon sources.

`design/tokens.css` stays the single source for the web. A small generator in `tools/` emits
`launcher/ui/Tokens.qml` from it. Font sources live in `design/fonts/`; the build copies the TTF or
OTF files into `launcher/ui/fonts/` and compiles them into the resource file. Do not hand-maintain
two palettes or two font sets.

#### 13.4.4 Platform differences

| Concern | Windows | Linux |
| --- | --- | --- |
| File picker | `QFileDialog`, which uses `IFileDialog` | `QFileDialog` through the XDG portal |
| Tray | `QSystemTrayIcon`, jump list, taskbar progress via Win32 | `QSystemTrayIcon` / StatusNotifierItem, desktop entry |
| Single instance | `QLocalServer` named pipe | `QLocalServer` Unix socket |
| Token ACL | Owner-only DACL via the Windows API | `0600` |
| Library install | Copy DLL, Steamless sidecar | Copy `.so`, `LD_PRELOAD` |
| Protocol handler | `HKCU\Software\Classes\sapie` | `x-scheme-handler/sapie` desktop entry |
| Updates | Signed manifest, silent NSIS apply | Signed manifest, AppImage replace, Flatpak |
| Elevation | Per-user by default, per-machine MSI | Never root; user install |
| Sandbox | None beyond the OS | Flatpak sandbox with documented permissions |
| High DPI | Qt 6 automatic scaling | Qt 6 automatic scaling, 1x and 2x tested |
| Antivirus | Sign binaries, avoid packers | Not applicable |

#### 13.4.5 What both launchers must do

- Real login against the server. No fake "connected" state.
- Token in the OS credential store. No password on disk.
- Detect arch, era, interfaces and appid.
- Install, repair and restore the game library.
- Launch the game in one click with a 0600 token file.
- Full friends, requests, blocked, lobby join and invite.
- Library with covers, monograms, favorites, recent and playtime.
- Auto-update with a visible version and a rollback path.
- Empty, loading, error, offline and missing-game states.
- Keyboard and screen-reader use through the Qt accessibility bridge.
- Start without a browser runtime. If WebView2 or WebKitGTK is missing, nothing changes, because
  neither is used.

#### 13.4.6 Qt licensing and runtime

- Pin Qt 6.8 LTS for the whole 0.5 cycle. Do not float to a new minor inside a release.
- Qt is used under LGPLv3 with dynamic linking. Ship the Qt libraries, the licence texts and the
  third-party notices. Provide the relink path LGPLv3 requires: the app must be relinkable against
  a modified Qt, with instructions in `docs/BUILD.md`.
- Do not static-link Qt without a legal review.
- Do not add a Qt module with a different licence (Charts and Virtual Keyboard are the common
  traps) unless the licence is checked and recorded. Draw charts in QML instead.
- `windeployqt` and `linuxdeploy-plugin-qt` decide which plugins ship. A missing `platforms`,
  `imageformats` or `styles` plugin is a blank window, so the packaging test asserts that the app
  starts on a clean machine.
- Compile QML ahead of time (`qmlcachegen`/`qmlsc`) and run `qmllint` in CI. A QML file that only
  fails at load time is a blank window on a user's machine.
- The Rust core stays under the repository licence. Qt stays a separate, replaceable shared
  library.

### 13.5 What to add in 0.5

#### 13.5.1 Must ship

- The identity system in 13.2 and the UI direction in 13.3.
- Two launchers on the shared Rust core and Qt Quick UI in 13.4. No webview runtime on either OS.
- Full friend management in both launchers, not only the web UI.
- Library: covers, monograms, favorites, recent, playtime, search, filters, sort.
- Game page with achievements, repair, restore and per-game settings.
- Add game flow with detection, review, install, rollback and editable appid.
- Settings, logs, About, diagnostics and sign out.
- System tray, single instance and `sapie://` links.
- Auto-update with signed manifests and rollback.
- The 0.4 security debts in 13.1, including the launcher replacement.
- Windows and Linux installers that a normal person can install and remove.

#### 13.5.2 Should ship

- MSIX for Windows and Flathub listing for Linux.
- A beta update channel next to stable.
- Server-side avatars or deterministic monograms. Start with monograms; upload only if it earns its
  keep.
- `sapie://join/<lobby>` invite links from the web and the launcher.
- A server-side game catalog for names and metadata, if it can be populated without fake data.

#### 13.5.3 Stretch, not 0.5

- Per-platform toolkit shells (WinUI 3 and GTK4) replacing Qt.
- Qt Widgets as a second UI toolkit. One toolkit, one UI language.
- Chat, voice, streaming, cloud saves, workshop, inventory, overlay.
- Store, purchasing, downloads, DRM.
- macOS, Windows ARM64, 32-bit ARM.
- Importing a Steam library or scanning all disks.
- App ownership enforcement.

### 13.6 Bundling and distribution

#### 13.6.1 Windows

Artifacts:

- `SAPIE-0.5.0-windows-x64-setup.exe` (NSIS, per-user, no admin).
- `SAPIE-0.5.0-windows-x64.msi` (per-machine, enterprise).
- `SAPIE-0.5.0-windows-x64-portable.zip`.
- `SAPIE-0.5.0-windows-x64.msix` only if 13.5.2 lands.

Rules:

- Sign every executable and installer with Authenticode and a timestamp. Use Azure Trusted Signing
  or an OV/EV certificate. Document that SmartScreen reputation takes time.
- Deploy Qt with `windeployqt --release --compiler-runtime`: Qt Core, Gui, Quick, QuickControls2,
  Network, Svg, the `platforms/qwindows.dll` plugin, the styles and image format plugins, the MSVC
  runtime, and a `qt.conf` that points at the bundled tree. There is no WebView2 bootstrapper and no
  runtime to detect.
- Test the portable zip on a clean Windows 10 and 11 machine with no Qt, no MSVC and no developer
  tools installed. A missing plugin is a blank window, so this test is mandatory.
- Write to `%LOCALAPPDATA%\SAPIE`, not `Program Files`, for the per-user install. Keep game files
  where the user put them.
- Token file ACL: owner-only, explicit, not inherited.
- Uninstall removes the app and the shortcut. Ask before deleting the library record and never
  delete game files.
- Auto-update: signed JSON manifest on GitHub Releases, stable and beta channels, rollback to the
  previous signed version. Apply through the per-user NSIS installer in silent mode, then verify the
  new binary's signature before restarting. A portable zip is updated by replacing the extracted
  folder, so the app never patches itself in place there.

#### 13.6.2 Linux

Artifacts:

- `SAPIE-0.5.0-linux-x64.AppImage`.
- `SAPIE-0.5.0-linux-amd64.deb`.
- `SAPIE-0.5.0-linux-x64.flatpak` and a Flathub manifest.
- `SAPIE-0.5.0-linux-arm64.AppImage` and deb if the runner holds.
- `SAPIE-0.5.0-linux-x64.rpm` only if a maintainer signs up for it.

Rules:

- AppStream metadata, a `.desktop` entry, `x-scheme-handler/sapie`, and hicolor icons at the sizes
  the desktops ask for.
- Bundle Qt. The AppImage uses `linuxdeploy` with the Qt plugin, the deb installs a private Qt tree
  under `/opt/SAPIE`, and the Flatpak uses `org.kde.Platform` with the matching Qt 6 runtime. Do not
  link against the distro Qt: Ubuntu 22.04 ships 6.2 and 24.04 ships 6.4, both older than the pinned
  6.8. Bundling keeps one build working across the matrix.
- Flatpak documents its filesystem permissions. A game directory outside the sandbox needs a portal
  or an explicit `--filesystem` grant. Say so in the UI.
- Token file mode `0600`. Never write to `/usr` or `/etc` from the app.
- Install the AppImage and the deb user install per user; the system deb goes through the package
  manager. The deb does not require root to run, only to install system-wide.
- Auto-update: AppImage replaces itself after verifying the signed manifest. Flatpak updates through
  its runtime. Debs update through the distro or a signed repository later.
- Wayland and X11 both work. Test Qt's `wayland` and `xcb` platform plugins explicitly. HiDPI works.
  Test at 1x and 2x.

#### 13.6.3 Server, DLL and web

- Server: `linux/amd64`, `linux/arm64`, `windows/amd64`, `windows/arm64` binaries plus a multi-arch
  container with `web/dist`.
- DLL: the 10 target-era variants, unchanged naming.
- Web: `web/dist` inside the container and the release archive.
- Checksums: one `SHA256SUMS` file per release.
- SBOM: one per artifact family.
- Third-party notices: Qt 6 and the Qt modules used (LGPLv3), Manrope and IBM Plex (OFL), Rust
  crates, Steamless, and GSAP for the web UI only.

#### 13.6.4 Release channels and rollback

- `stable` and `beta` channels. The launcher shows the channel and the version.
- Update manifest is signed. The app verifies the signature before applying.
- Keep the previous version downloadable. Rollback is a documented manual step in 0.5, automatic
  only if the updater supports it.
- A failed update leaves the old version runnable and says so.

#### 13.6.5 Artifact naming

`SAPIE-<version>-<os>-<arch>-<kind>.<ext>`, for example
`SAPIE-0.5.0-windows-x64-setup.exe` and `SAPIE-0.5.0-linux-x64.AppImage`. No spaces, no dates in the
name, no `latest` in a file name.

### 13.7 M stages and phases

M22 to M30 are 0.5. M13 to M21 are 0.4 and stay above as history. Each phase is one agent, one
worktree, one artifact. Risk tiers as in 12.1.

Stage map:

| Stage | Theme | Gate |
| --- | --- | --- |
| M22 | Identity foundation | G1 |
| M23 | Shared launcher core | G1 holds |
| M24 | SAPIE for Windows | G2 |
| M25 | SAPIE for Linux | G2 |
| M26 | Launcher product | G3 |
| M27 | Server, protocol and security debts | G3 |
| M28 | Web UI identity | G1 and G3 |
| M29 | Bundling, updates and CI | G4 |
| M30 | Migration, release candidate and tag | G4 |

#### M22: Identity foundation

##### M22-a Identity decision record
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** none.
- **Scope:** read `AGENTS-UI.md`, the existing `design/` and `web/DESIGN.md`. Write
  `design/identity.md` with name, tagline, mark brief, palette, type, voice, motion, art rules and
  the GOG-inspiration boundary. Record every override of `AGENTS-UI.md` with a reason.
- **Acceptance:** the doc is approved at G1. No em dash. Every choice has one line of reason.

##### M22-b Tokens v2
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M22-a.
- **Scope:** rewrite `design/tokens.css` with the 0.5 palette, type, spacing, radius, shadows and
  motion. Retire the ember brand accent. Add the generator that emits `launcher/ui/Tokens.qml`, and
  a contrast table in `design/DESIGN.md`.
- **Acceptance:** web and Qt consume the same source. No hardcoded colour outside tokens. AA
  measured for every text pair. Regenerating produces no diff.

##### M22-c Mark, wordmark and monogram
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M22-a, G1 direction.
- **Scope:** three mark directions, user picks one at G1. Produce SVG wordmark, app icon at all
  sizes, tray silhouette, and a deterministic monogram generator. No final art before approval.
- **Acceptance:** mark works at 16px and 512px. Icon set is complete for Windows and Linux. The
  monogram is stable for the same appid. G1 signs off.

##### M22-d Component primitives
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M22-b.
- **Scope:** build the primitives in 13.3.2 as QML components in `launcher/ui`. Every custom control
  declares its `Accessible` role, name and state. Include all states and keyboard behavior.
- **Acceptance:** a gallery screen renders every primitive in every state. Focus order is correct.
  `qmllint` is clean. The accessibility bridge reports each control on Windows and Linux.

##### M22-e Identity prototype
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M22-c, M22-d.
- **Scope:** clickable QML prototype of Sign in, Library, Game, Friends, Settings, plus empty,
  loading and error states. Use a real local record if available. No invented content.
- **Acceptance:** G1 approves the prototype. Swap-the-logo test fails to identify GOG, Steam or
  Heroic. Antislop audit passes.

##### M22-f Identity surfaces
- **Agent:** one worker. **Risk:** green. **Priority:** should. **Depends:** M22-c.
- **Scope:** README banner, social preview, About screen, installer splash, release-notes template,
  startup banner, docs theme.
- **Acceptance:** the same mark, palette and voice across all surfaces. No fake screenshots.

#### M23: Shared launcher core

##### M23-a Extract the core crate
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M22.
- **Scope:** create `launcher/core` and move auth, detection, install, config, launch, library,
  keychain, paths and update logic out of the current Tauri shell. Expose it as a C ABI
  (`sapie_core.h`) with no Qt dependency.
- **Acceptance:** the Rust tests pass. A C smoke test calls every exported function. The crate
  builds without Qt installed.

##### M23-b C ABI and CoreBridge
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M23-a.
- **Scope:** versioned `sapie_core.h`: opaque handles, status codes, owned strings with explicit free
  functions, one event callback. One hand-written `CoreBridge : QObject` in `launcher/bridge` with
  `Q_INVOKABLE` methods and signals for QML. No JSON command channel and no local socket.
- **Acceptance:** a Qt test drives login, detect and launch through the bridge with a fake core.
  Unknown ABI versions fail with a clear message. No leaked strings under ASan.

##### M23-c Platform boundary
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M23-a.
- **Scope:** Rust traits for paths, process, keychain, ACL and updater. A C++ `Platform` interface for
  dialogs, tray, protocol registration and single instance. Fake implementations for tests.
- **Acceptance:** core tests run on Linux and Windows with the fake platform. No platform `cfg`
  leaks into product logic. The C++ side has exactly one implementation per OS.

##### M23-d Library store migration
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M23-a.
- **Scope:** schema v2 for the local library, atomic write, backup, migration from the 0.4 state,
  recovery from a corrupt file.
- **Acceptance:** a 0.4 state file migrates with no data loss. A truncated write leaves the previous
  file usable. Tests cover both.

##### M23-e Core test harness
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M23-c, M23-d.
- **Scope:** fixtures for real game directory layouts, fake filesystem, fake keychain, fake
  process spawn.
- **Acceptance:** the core suite runs on both OSes in CI without opening a window.

##### M23-f Qt application skeleton
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M22-d, M23-b.
- **Scope:** `launcher/app` and the top-level CMake build. QML engine loading from the resource file,
  bundled fonts, the `Tokens.qml` singleton, window state, the dark frame, single instance, logging
  and clean shutdown. No webview and no remote URL.
- **Acceptance:** an empty SAPIE shell opens on Windows and Linux from the same QML. Assets load
  from the binary. A network trace shows no remote request at startup.

#### M24: SAPIE for Windows

##### M24-a Windows shell
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M23.
- **Scope:** `launcher/windows`, Qt 6 app target with MSVC, native frame and dark title bar, Qt Quick
  window, tray, jump list, taskbar progress, `QLocalServer` single instance.
- **Acceptance:** builds on `windows-latest` with Qt 6.8, opens the shared QML, and runs with no
  WebView2 installed.

##### M24-b Windows integration
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M24-a.
- **Scope:** file associations, `sapie://` handler, long paths, token ACL, Steamless, antivirus
  notes, `%LOCALAPPDATA%` paths.
- **Acceptance:** protocol opens the launcher, ACL is owner-only, Steamless unpack works, long path
  test passes.

##### M24-c Windows packaging
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M24-b.
- **Scope:** `windeployqt` with plugins, styles and the MSVC runtime, NSIS per-user, MSI per-machine,
  portable zip, icons, version info, uninstall behavior. No WebView2 bootstrapper.
- **Acceptance:** clean Windows 10 and 11 install, launch, update prompt and uninstall without
  admin for the per-user path. The portable zip runs on a machine with no Qt or MSVC installed.

##### M24-d Windows signing
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M24-c.
- **Scope:** Authenticode signing in CI, timestamping, secret handling, SmartScreen documentation.
- **Acceptance:** every shipped executable and installer is signed. The SmartScreen reputation risk
  is documented, not hidden.

##### M24-e Windows update path
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M24-d.
- **Scope:** update check in the core, signed manifest, download, signature and version verify,
  silent per-user NSIS apply, stable and beta channels, rollback.
- **Acceptance:** 0.4 to 0.5 update and a 0.5 to 0.5.1 dry run succeed. A tampered manifest is
  rejected. A failed update leaves 0.4 runnable.

#### M25: SAPIE for Linux

##### M25-a Linux shell
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M23.
- **Scope:** `launcher/linux`, Qt 6 app target, native frame, Qt Quick window, `QLocalServer` single
  instance, tray through StatusNotifierItem where the desktop provides it.
- **Acceptance:** builds on `ubuntu-24.04` and `ubuntu-24.04-arm`, opens the shared QML, starts
  under Wayland and X11, and needs no WebKitGTK.

##### M25-b Linux integration
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M25-a.
- **Scope:** XDG dirs, desktop entry, MIME, AppStream, LD_PRELOAD, 0600 token file, no Steamless,
  Wayland and X11, HiDPI.
- **Acceptance:** file manager launch, protocol handler, game launch with preload, 1x and 2x.

##### M25-c Linux packaging
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M25-b.
- **Scope:** AppImage through `linuxdeploy` with the Qt plugin, deb with a private Qt tree under
  `/opt/SAPIE`, Flatpak on `org.kde.Platform` with the Qt 6 runtime, icons, desktop file, metainfo,
  sandbox permissions.
- **Acceptance:** install, launch and uninstall on Ubuntu 22.04, Ubuntu 24.04, Fedora and Arch.
  Flatpak runs with a documented filesystem grant. The AppImage runs without Qt installed.

##### M25-d Linux update path
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M25-c.
- **Scope:** signed manifest in the core, AppImage replace with signature check, Flatpak update,
  deb policy, rollback.
- **Acceptance:** 0.4 to 0.5 update via AppImage and Flatpak. A tampered manifest is rejected. A
  failed update leaves the old version runnable.

##### M25-e Distro and desktop matrix
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M25-c.
- **Scope:** VM scripts and evidence for GNOME, KDE, Wayland and X11, 1x and 2x.
- **Acceptance:** `docs/verification/0.5-linux-matrix.md` with screenshots and notes.

#### M26: Launcher product

##### M26-a Library
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M22-e, M23.
- **Scope:** grid, covers, monograms, favorites, recent, search, filter, sort, playtime, missing game
  state.
- **Acceptance:** 0, 1 and 100 games render without overflow. Search and sort persist. Keyboard
  reaches every tile and Play action.

##### M26-b Game page
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M26-a.
- **Scope:** cover, Play, status, details, achievements, repair, restore, launch args, preload,
  recent launches.
- **Acceptance:** restore puts the original library back. Hidden achievements stay hidden. Every
  action has a confirmation or undo.

##### M26-c Add game
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M23.
- **Scope:** pick, detect, review, install, rollback, editable appid and era, advanced options.
- **Acceptance:** adding a real game works end to end. A failed copy rolls back. A duplicate focuses
  the existing record.

##### M26-d Friends
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M27-a to M27-c.
- **Scope:** friends, requests, blocked, search, add, remove, block, unblock, lobby join, invite,
  monogram avatars.
- **Acceptance:** cross-surface: a request sent in the launcher is accepted in the web and the
  reverse. Remove and block take effect on both.

##### M26-e Settings, logs and About
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M26-a.
- **Scope:** account, server, updates, appearance, diagnostics, About, sign out, copy log, open data
  folder.
- **Acceptance:** no secret in any log. Sign out clears the keychain token. Update channel is
  visible.

##### M26-f Onboarding and states
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M26-a to M26-e.
- **Scope:** first run, empty, loading, error, offline, missing game, launch failed, update failed.
- **Acceptance:** every state has a specific message and an action. No raw stack trace reaches the
  user.

##### M26-g Accessibility
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M26-a to M26-f.
- **Scope:** keyboard, screen reader through `Accessible` properties, contrast, reduced motion, 44px
  targets, focus return.
- **Acceptance:** a keyboard-only user signs in, adds a game and launches it. A UIA pass on Windows
  and an AT-SPI/Orca pass on Linux are recorded. Both OSes.

##### M26-h Launcher tests
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M26-a to M26-g.
- **Scope:** Qt Quick Test and `qmltestrunner` against a fake `CoreBridge`, screenshot checks with
  `grabToImage`, `qmllint`, and platform integration tests.
- **Acceptance:** CI runs the suite on both OSes. No test needs a real game or a real server to
  pass.

#### M27: Server, protocol and security debts

##### M27-a Token rotation and revoke
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** none.
- **Scope:** rotate on login, revoke on password change, list sessions, cap lifetime, invalidate on
  logout-all, tests for replay.
- **Acceptance:** a revoked or rotated token fails immediately. A password change kills every other
  session.

##### M27-b Limiter persistence
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M27-a.
- **Scope:** persist login, social and game-login limiters. Add a lockout status to the wire enum or
  a clear error path. Cleanup expired rows.
- **Acceptance:** a restart does not reset a lockout. Tests cover the lockout window.

##### M27-c Blocked peers in lobbies
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M27-a.
- **Scope:** filter `router.lobbyRecipients`, invitations and signaling by the block list. Keep the
  owner and member rules intact.
- **Acceptance:** a blocked peer receives no lobby frames and cannot join or relay. Tests for each
  path.

##### M27-d Windows token ACL
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M23.
- **Scope:** write the token file with an explicit owner-only ACL on Windows. Verify on a clean
  profile.
- **Acceptance:** `icacls` shows only the owner. The DLL can still read it. Documented.

##### M27-e Deep links and invites
- **Agent:** one worker. **Risk:** yellow. **Priority:** should. **Depends:** M27-c.
- **Scope:** `sapie://join/<lobby>`, resolve and join, web fallback, invite copy link.
- **Acceptance:** a link from the web opens the launcher and joins. A bad or expired link shows a
  specific error.

##### M27-f Protocol tests and docs
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M27-a to M27-e.
- **Scope:** table-driven tests for every new path, updated `docs/PROTOCOL.md`, `docs/API.md`,
  `docs/SECURITY.md`.
- **Acceptance:** every new route has a test and a doc line. The 0.4 risk list is updated.

#### M28: Web UI identity

##### M28-a Tokens and primitives
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M22-b, M22-d.
- **Scope:** consume the shared tokens and the behaviour rules. Keep the React components; there is
  no QML reuse on the web. Remove the glass panels and the ember accent. Match the Qt launcher frame
  and type.
- **Acceptance:** web and launcher look like one product. No duplicated colour values. The token
  generator is the only path from `design/tokens.css` to either surface.

##### M28-b Friends, requests and lobbies
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M28-a, M27.
- **Scope:** restyle the friends, requests, blocked and lobby surfaces to the 0.5 direction. Keep the
  real states and data.
- **Acceptance:** keyboard and contrast pass. No fake content. Cross-surface actions work.

##### M28-c Account and admin
- **Agent:** one worker. **Risk:** green. **Priority:** should. **Depends:** M28-a.
- **Scope:** account, achievements, admin view, monogram avatar, sign out.
- **Acceptance:** admin routes stay on loopback. No secret leaks.

##### M28-d Docs and surfaces
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M22-f.
- **Scope:** README, `docs/UI.md`, screenshots with real data, social preview, issue templates.
- **Acceptance:** every image is real or clearly a placeholder. The voice matches 13.2.5.

##### M28-e Accessibility and responsive
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M28-b.
- **Scope:** keyboard, focus, 360px, contrast, reduced motion, screen reader.
- **Acceptance:** recorded pass at 360, 768 and 1440. No horizontal overflow.

#### M29: Bundling, updates and CI

##### M29-a Windows CI
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M24.
- **Scope:** Qt 6.8 MSVC build, `windeployqt`, sign, package NSIS/MSI/portable, checksums, SBOM,
  upload.
- **Acceptance:** a tag produces all Windows artifacts. Signing is verified. A portable artifact
  starts on a runner with no Qt installed.

##### M29-b Linux CI
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M25.
- **Scope:** Qt 6.8 build, AppImage through `linuxdeploy` and the Qt plugin, deb with the private Qt
  tree, Flatpak on `org.kde.Platform`, AppStream, checksums, SBOM, upload.
- **Acceptance:** a tag produces all Linux artifacts. Flatpak lint passes. The AppImage starts on a
  runner with no distro Qt.

##### M29-c Update manifests and channels
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M29-a, M29-b.
- **Scope:** signed update manifests, stable and beta, version pinning, rollback notes.
- **Acceptance:** the launcher sees the channel and version. A tampered manifest is rejected.

##### M29-d Release bundles
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M29-a to M29-c.
- **Scope:** update `tools/release-bundle.sh` for two launchers, server, DLLs, web, checksums,
  notices, SBOM. One archive per OS plus the container.
- **Acceptance:** a dry-run tag produces the full set. No artifact is missing or misnamed.

##### M29-e Container and server bundles
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M29-d.
- **Scope:** multi-arch container with `web/dist`, healthcheck, compose, server binaries.
- **Acceptance:** `docker compose up` serves the web on 8081. The admin API stays loopback.

##### M29-f Security and dependency gates
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M29-a to M29-e.
- **Scope:** `govulncheck`, `npm audit`, `cargo audit`, secret scanning, signed release, dependency
  policy.
- **Acceptance:** no new high finding is silently accepted. The allowlist is documented.

#### M30: Migration, release candidate and tag

##### M30-a State migration
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M23-d.
- **Scope:** migrate a 0.4 library, server, token and per-game settings into 0.5. Keep the original
  library backup intact.
- **Acceptance:** a 0.4 profile upgrades with no lost game. A failed migration backs up and tells
  the user.

##### M30-b Upgrade path
- **Agent:** one worker. **Risk:** yellow. **Priority:** must. **Depends:** M30-a, M24-e, M25-d.
- **Scope:** install 0.4, then 0.5, on Windows and Linux. Verify library, token, friends and launch.
- **Acceptance:** documented upgrade path. A downgrade does not destroy the 0.5 state.

##### M30-c Release candidate verification
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** all.
- **Scope:** clean VMs, both OSes, install, sign in, add game, launch, friends, requests, block,
  lobby, update, uninstall. Run the M13 core evidence again. Confirm no webview runtime is present
  on either VM.
- **Acceptance:** `docs/verification/0.5-rc-checklist.md` signed off. G2, G3 and G4 pass.

##### M30-d Docs and changelog
- **Agent:** one worker. **Risk:** green. **Priority:** must. **Depends:** M30-c.
- **Scope:** README, QUICKSTART, BUILD, PROTOCOL, API, SECURITY, UI, CONTRIBUTING, CHANGELOG,
  AGENTS, identity docs. Remove 0.4-only claims.
- **Acceptance:** every command runs. No doc references a removed path.

##### M30-e Tag and publish
- **Agent:** one worker. **Risk:** red. **Priority:** must. **Depends:** M30-d.
- **Scope:** bump to 0.5.0, tag `v0.5.0`, publish artifacts, container, notes, known limits.
- **Acceptance:** artifacts exist, checksums verify, container pulls, notes state what was and was
  not verified.

### 13.8 G gates

G stages are acceptance gates, not merge stages. A gate is signed off by the user or the chief with
recorded evidence. A failed gate blocks the next stage.

#### G1: Identity gate
- **When:** after M22-e, before M23 production UI.
- **Owner:** user plus chief.
- **Criteria:** `design/identity.md` approved. Mark and wordmark chosen. Palette, type and voice
  final. QML prototype of Sign in, Library, Game, Friends and Settings approved. Contrast AA.
  Swap-the-logo test passes. No GOG clone. No fake content. Antislop audit passes.
- **Evidence:** `docs/verification/0.5-identity-gate.md` with screenshots and the decisions.

#### G2: Platform gate
- **When:** after M24-c and M25-c, before M29 release work.
- **Owner:** chief, with user sign-off for signing exceptions.
- **Criteria:** clean install, launch and uninstall on Windows 10, Windows 11, Ubuntu 22.04,
  Ubuntu 24.04, Fedora and Arch. File picker, tray, protocol handler, token storage and update check
  work. Per-user Windows install needs no admin. Linux install needs no root. Neither OS has
  WebView2 or WebKitGTK installed during the test.
- **Evidence:** `docs/verification/0.5-platform-gate.md` with VM logs and screenshots.

#### G3: Core gate
- **When:** after M26 and M27, before M30-c.
- **Owner:** user plus chief.
- **Criteria:** live two-client session, a real game run on Windows and Linux, token login with no
  password, friend request, accept, remove, block, lobby join, achievements. No plaintext secret in
  disk or logs. The 0.4 security debts are closed.
- **Evidence:** `docs/verification/0.5-core-gate.md` with logs and the exact builds.

#### G4: Release gate
- **When:** after M30-c, before the tag.
- **Owner:** user.
- **Criteria:** signed Windows and Linux artifacts, checksums, SBOM, signed update manifest, upgrade
  from 0.4, rollback path, container, docs, known limits. Qt libraries are bundled and the LGPLv3
  relink path is documented. No webview is linked. A user can install, add a game, play and
  uninstall without reading source docs.
- **Evidence:** `docs/verification/0.5-rc-checklist.md` signed off.

### 13.9 Full change inventory

#### Design and identity

- `design/identity.md`: new, source of truth for name, mark, palette, type, voice, motion, art.
- `design/DESIGN.md`: rewrite for the 0.5 direction, GOG boundary, dark-only reason, screen reasons.
- `design/tokens.css`: rewrite, violet accent, new neutrals, type, spacing, radius, elevation.
- `tools/gen-qml-tokens`: new, emits `launcher/ui/Tokens.qml` from `design/tokens.css`.
- `design/fonts/`: Manrope and IBM Plex Mono as TTF or OTF for the Qt build, woff2 for the web.
- `design/icons/`: new SVG sources and exported sizes.
- `design/wordmark.svg`, `design/mark.svg`, `design/app-icon.svg`: new.
- `web/DESIGN.md`: replace the glass direction with the shared 0.5 direction.
- `AGENTS-UI.md`: keep as the reference, mark the overrides in `design/identity.md`.

#### Launcher

- `launcher/CMakeLists.txt`: Qt 6 build, Rust core, both platform targets.
- `launcher/Cargo.toml`: workspace.
- `launcher/core/`: auth, detect, install, config, launch, library, keychain, paths, update, errors,
  platform traits and tests. Exposes `include/sapie_core.h`.
- `launcher/bridge/`: `CoreBridge.h` and `CoreBridge.cpp`, the only C++ that touches the C ABI.
- `launcher/ui/`: QML components, screens, `Tokens.qml`, icons, TTF fonts, `qrc`.
- `launcher/app/`: `main.cpp`, QML engine setup, fonts, window state, single instance, platform
  interface.
- `launcher/windows/`: Win32 integration, `.rc`, NSIS/MSI scripts, `windeployqt`.
- `launcher/linux/`: XDG integration, `.desktop`, metainfo, AppImage and Flatpak manifests.
- `launcher/tests/`: fake platform and bridge, fixtures, QML tests, screenshots.
- `client/src-tauri/`: migrate out and remove in M23/M30.
- `client/src-tauri/resources/steamless/`: move to the Windows launcher resources.
- `client/src-tauri/ui/`: deleted with the Tauri shell. The QML UI is new work, not a port.

#### Launcher screens and components

- Sign in, Library, Game, Add game, Friends, Requests, Blocked, Lobbies, Settings, About, Logs.
- QML: `AppShell`, `Sidebar`, `Toolbar`, `GameCard`, `GameGrid`, `MonogramTile`, `PlayButton`,
  `FriendRow`, `RequestRow`, `BlockedRow`, `Avatar`, `PresenceDot`, `LobbyRow`, `Dialog`, `Drawer`,
  `Toast`, `Menu`, `EmptyState`, `LoadingSkeleton`, `ErrorState`, `OfflineBanner`.
- Every custom control sets `Accessible.role`, `Accessible.name` and its state.
- State: library snapshot, session, update state, online state, progress events.

#### Server and protocol

- `server/migrations/003_0_5.sql`: limiter persistence, session rotation metadata, any new tables.
- `server/internal/state/state.go`: session rotation, revoke, limiter storage, block-aware lobby
  queries.
- `server/internal/api/api.go`: rotation, revoke, deep-link resolve, invite link, lockout status.
- `server/internal/session/session.go`: token rotation, revoked token, game login lockout.
- `server/internal/router/router.go`: block-aware lobby recipients.
- `server/internal/ui/ui.go`: deep-link fallback, new user routes.
- `server/cmd/sapie-server/main.go`: version, limiter store, deep-link base, TLS status.
- `proto/sapie.proto`: lockout status if added, any new frame for invites.
- `client/dll/sapie.proto`: mirror.
- `server/internal/protocol/*.pb.go`: regenerate.
- Tests for every new path.

#### DLL and client

- `client/dll/settings_parser.cpp`: token file, appid, era, preload, deep-link join.
- `client/dll/settings.h`: new fields.
- `client/dll/network.cpp` and `.h`: rotation, lockout, friend events, reconnect, path logs.
- `client/dll/steam_friends.h`: block and request state.
- `client/sapie/local_mirror.h`: friend, request, block state.
- `client/harness/sapie_login_smoke.cpp`: token rotation, lockout, friends, lobby.
- `tools/harness/`: two-client scenario for CI if feasible.

#### Web

- `web/src/api.ts`: new endpoints, typed errors, session rotation.
- `web/src/components/`: shared primitives and social rows.
- `web/src/screens/`: Friends, Requests, Blocked, Lobbies, Account, Admin, Sign in.
- `web/src/styles.css`: consume shared tokens, remove glass and ember.
- `web/DESIGN.md`: 0.5 direction.
- `web/package.json`: scripts and version.
- `web/e2e/`: Playwright.
- `web/src/**/*.test.tsx`: Vitest.

#### CI, packaging and release

- `.github/workflows/ci.yml`: core, Qt build, QML lint and tests, web, server, DLL, audits.
- `.github/workflows/release-windows.yml`: Qt build, `windeployqt`, sign, package, sign manifest.
- `.github/workflows/release-linux.yml`: Qt build, AppImage, deb, Flatpak, AppStream, sign.
- `.github/workflows/release.yml`: orchestrate, bundle, container, publish.
- `.github/workflows/build.yml`: remove or fold into the above.
- `.github/dependabot.yml`: Go, npm, Cargo, Actions, and Qt version bumps handled by hand.
- `tools/release-bundle.sh`: two launchers, new names, checksums, notices, SBOM.
- `tools/verify-checksums.sh`: new.
- `Dockerfile`, `compose.yaml`: 0.5 server, web, healthcheck.
- Signing secrets: Authenticode, AppImage GPG, Flatpak.
- `docs/RELEASE.md`: new.

#### Docs

- `README.md`: 0.5 install, identity, two launchers, limits.
- `docs/QUICKSTART.md`: Windows and Linux paths, no SQLite friend editing.
- `docs/BUILD.md`: Qt 6.8, CMake, MSVC and Ninja, Rust, Qt LGPL relink steps, signing, Flatpak.
- `docs/PROTOCOL.md`: token rotation, lockout, deep links.
- `docs/API.md`: new endpoints.
- `docs/SECURITY.md`: 0.5 threat model, Windows ACL, Flatpak sandbox, TLS.
- `docs/UI.md`: 0.5 screens and tokens.
- `docs/IDENTITY.md`: public identity page, or `design/identity.md` linked.
- `docs/CONTRIBUTING.md`: two-launcher workflow.
- `docs/CHANGELOG.md`: 0.5 entries.
- `docs/verification/0.5-identity-gate.md`, `0.5-platform-gate.md`, `0.5-core-gate.md`,
  `0.5-linux-matrix.md`, `0.5-rc-checklist.md`: new.
- `AGENTS.md`: 0.5 stack, launcher split, commands, gates.
- `plan.md`: this section is the live 0.5 plan.
- `dev_notes.md`: 0.5 limits and evidence.

#### Repo hygiene

- Move `client/src-tauri` to `launcher/` with `core`, `bridge`, `ui`, `app`, `windows`, `linux`.
- Remove the Tauri shell, its `gen/schemas`, `tauri.conf.json`, capabilities and React UI. Nothing
  Tauri survives 0.5.
- Keep `client/dll`, `client/sapie`, `server`, `web`, `proto`, `tools` in place.
- Update `.gitignore` for `launcher/build*`, `launcher/core/target`, generated QML caches and
  packaging output.
- Bump versions to 0.5.0 in the Cargo workspace, CMake project, web and docs.
- Add third-party notices for Qt 6 (LGPLv3), Manrope and IBM Plex (OFL), Rust crates, Steamless and
  GSAP for the web UI.
- Archive 0.4 plan detail as history if `plan.md` becomes too long. Do not delete the decision log.

### 13.10 Dependency lanes

| Lane | Phases | Rule |
| --- | --- | --- |
| Identity | M22-a to M22-f | G1 blocks production UI |
| Core | M23-a to M23-f | Must land before both shells |
| Windows | M24-a to M24-e | Parallel with Linux after core |
| Linux | M25-a to M25-e | Parallel with Windows after core |
| Product | M26-a to M26-h | After core, uses shared QML |
| Server and security | M27-a to M27-f | Parallel with shells, blocks friends UI |
| Web | M28-a to M28-e | After tokens and server routes |
| Packaging | M29-a to M29-f | After both shells |
| Release | M30-a to M30-e | Last |

Parallel lanes: Windows and Linux after M23. Server security and identity can run in parallel.
M26-c can start after M23. M26-d waits on M27-a to M27-c.

### 13.11 Verification matrix

| Layer | Command or run | Gate |
| --- | --- | --- |
| Identity | G1 prototype review, contrast, swap test | G1 |
| Core | `cargo test` in `launcher/core`, fake platform, ASan on the C ABI | Every PR |
| Qt build | CMake build on Windows and Linux, `qmllint` clean | Every PR |
| Launcher UI | Qt Quick Test, `qmltestrunner`, fake `CoreBridge`, screenshot checks | Every PR |
| No webview | dependency and import scan, clean machine without WebView2 or WebKitGTK | G2 |
| Accessibility | Windows UIA pass, AT-SPI/Orca pass | G2 and G3 |
| Windows | build on `windows-latest`, install, launch, uninstall | G2 |
| Linux | build on `ubuntu-24.04`, AppImage, deb, Flatpak, Wayland and X11 | G2 |
| Server | `go test ./...`, `go vet ./...` | Every PR |
| Web | `tsc --noEmit`, `vite build`, Vitest, Playwright | Every PR |
| DLL | 10 builds, smoke modern and legacy | Every PR |
| Core live | two-client, real game, friends, lobby, achievements | G3 |
| Security | `govulncheck`, `npm audit`, `cargo audit`, secret scan | Every PR |
| Licensing | Qt LGPL notices, relink instructions, no non-LGPL Qt module | G4 |
| Packaging | tag dry run, checksums, SBOM, signatures | G4 |
| Upgrade | 0.4 to 0.5 on Windows and Linux | G4 |

### 13.12 0.5 release definition

0.5 is shippable when all of these are true:

- G1, G2, G3 and G4 are signed off with evidence.
- `SAPIE for Windows` installs per-user without admin, launches, updates and uninstalls.
- `SAPIE for Linux` installs as AppImage, deb and Flatpak, launches, updates and uninstalls.
- Neither launcher uses a webview. No WebView2, WebKitGTK or embedded browser is linked or loaded.
- The Qt runtime is bundled and the LGPLv3 relink path is documented and tested.
- The identity is applied to launcher, web, docs, installer and release notes.
- Both launchers do full friend management, library, game page, add game and settings.
- The password is never on disk, in a log or in a URL.
- Token rotation and revoke work. Blocked peers cannot join or receive lobby frames.
- The 0.4 security debts in 13.1 are closed or explicitly accepted with a reason.
- The tag produces signed Windows and Linux artifacts, checksums, SBOM and the container.
- `dev_notes.md` states what was and was not verified.

### 13.13 Out of scope for 0.5

Store, purchasing, downloads, DRM, chat, voice, streaming, cloud saves, workshop, inventory, overlay,
controller, macOS, Windows ARM64, 32-bit ARM, hole punching, app ownership enforcement, importing a
Steam library, fully native WinUI 3 or GTK4 shells, user-uploaded avatars, and server-side game
metadata beyond what the API already returns. These are 0.6 candidates and are listed here so a
phase cannot quietly grow into them.
