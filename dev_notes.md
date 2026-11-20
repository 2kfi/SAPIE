# SAPIE — Development Notes

## Deferred items after MF1

Carried forward from the MR1 review and MF1 fixes at the 0.1 release at `62007f5`.
None of these block the release; they are known limits, not failures found in testing.

### 1. Mutable mirror refs

The C++ client's `friends()` and `lobbies()` return references to internal lists rather than
copies. In theory another thread could change a list while someone reads it. In practice every
caller is serialized behind a global lock, so there is no observed race. Fixing it properly means
rewriting several Steam interface methods to use copies — broad churn for a hypothetical problem.
Left as a labelled follow-up.

### 2. `StatsGet.names` ignored

The wire message has a field where a caller can say "only send me these named stats." The server
ignores that list and returns everything for that user/app. The result is a superset, so nothing
breaks today — but the field is dead plumbing, and asking for one stat still returns all of them.

### 3. Hidden achievements in unlock pushes

We fixed the *query* leak: a friend can no longer ask the server for your hidden achievements. But
when you actually unlock one, the server still pushes that unlock to friends. So a friend can learn
a hidden achievement's name and unlock time through the push, even though they cannot query it.
It is a smaller version of the privacy issue MF1 already closed.

### 4. No live end-to-end run

This is the real gap. Everything so far was proven in pieces: in-process Go harnesses, stubbed C++
smoke messages, isolated builds. We have not launched two real C++ clients against the real Go
server, run an actual old Steam game with the replacement DLL, or clicked through the web UI in a
browser. Any of those could expose an integration bug that none of our tests cover.

This is the first 0.2 gate: a live two-client C++↔Go session and a real MW2 run must happen before
the online surface or either P2P path can be called done.

Items 1–3 are small, understood limitations. Item 4 is the honest confidence gap: "nothing blocks
the merge" is not the same as "proven on a real machine." Main was merged from dev for the 0.1
release; see the release tag for the exact commit.

### Verification caveat

None of the four items were re-verified live; they come from worker reports and the merged-head test
suite. Main risk: the real two-client path stays unproven until item 4 is run.

## Carry-forward into 0.2

The 0.1 release at `62007f5` is published and frozen. Its UI text, including "Peer-to-peer play is
not available in this version.", stays as shipped. 0.2 code adds the following locally, not tagged
or released, with G0 deferred to the user:

- Full online persona surface: a game sees a logged-in account, persona name and online state, friends
  list and invites, achievements and stats, and a ticket issuance check. These are the checks a
  paranoid MW2-2009-style client probes; there is no app-ownership table.
- Deeper lobbies/matchmaking: join-friend and connect flows, implemented in code but not held for
  two real clients.
- P2P: direct client-to-client first, relay fallback for NAT-blocked peers. This reverses the old
  "No P2P game-traffic relay in v1" rule, superseded for 0.2.
- UI text rule: the P2P placeholder stays as shipped 0.1 text until the user accepts G0. P2P ships
  in 0.2 code, but changing that line is gated on G0 acceptance.

Item 4 above is the first gate: run a live two-client C++↔Go session and a real MW2 game with the
replacement DLL before declaring the online surface or either P2P path done. It is PENDING USER TEST;
0.2 code is complete locally and G0 is deferred to the user.

## 0.2 M6 server persona surface

The three small 0.1 limits above are closed on the server, without a wire or schema change:

- `StatsGet.names` now limits the returned stats to the named set; empty names keep the old
  all-stats behaviour. Achievements are unaffected by `names`.
- Hidden achievements are no longer pushed to friends or lobby mates. The unlock is still
  stored canonically and stays visible to the owner through `StatsGet{steam_id: 0}` and to admin
  through the HTTP stats API. The push is skipped rather than redacted so clients never receive
  an empty or malformed definition.
- Tickets are recorded in the existing `auth_tickets` table with their nonce and a 10-minute
  expiry, and `POST /api/v1/tickets/verify` checks issuance for an admin. This is not Steam
  ownership verification, not redemption, and game clients never call it.

Still open: there is no per-account app-ownership table, so `appid` remains a filter/label and
any authenticated account may name any appid. Tickets are not one-time or bound to a peer.
Item 4 (live two-client C++↔Go session and a real MW2 run) is unchanged and remains the gate
before the online surface can be called done. None of these were verified against a real game.

## 0.2 status (local code-complete, G0 deferred)

0.2 is code-complete on `dev-client` but is not tagged or released. 0.1 stays published and
frozen at `62007f5`, with its assets and GHCR image unchanged.

Shipped in code, with local verification only:

- M6 native egui/eframe launcher (`sapie-launcher`), no webview, localhost dependency or terminal
  window; `windows_subsystem` on Windows.
- M7a server LAN web UI on `--ui-listen 0.0.0.0:8081` from `--ui-dir ./web/dist`, with a narrow
  reverse proxy for user API routes; admin paths stay on loopback 8080 and 404 on 8081.
- M7b browser login and opt-in registration gated by `GET /api/v1/config` and `--allow-register`
  (off by default). The SPA uses a relative `/api/v1` base in the browser.
- M8a/M8b persona completeness and safe client mirror. Tickets are recorded with a 10-minute
  expiry and checked by `POST /api/v1/tickets/verify` (admin). The client does not send
  `TicketRequest` or read a server ticket response; `encrypted_app_ticket` is still synthesized
  locally. There is no app-ownership table; appid is a filter/label, and tickets are not one-time
  or peer-bound.
- M9a/M9b lobby rules and client lobby flows.
- M10a/M10b direct P2P. The server stamps `source_ip` from the authenticated connection and
  `source_port` from the client-declared bound UDP port (field 129), never trusting a client IP;
  the client sends direct UDP via `sendToPeer()` when a hint exists. There is no hole punching,
  and IPv6 gets no hint.
- M11a/M11b relay fallback. Authorized `Networking_Sockets`/`Networking_Messages` DATA goes only
  to the explicit `dest_id` (friend/lobby gate, no broadcast). The client uses one path per frame,
  taking the relay when there is no hint or no inbound DATA within 5s. Direct stays preferred.
- M-INT CI builds `web/dist` as the `web-dist` artifact; release bundles place it at `web/dist` in
  the archive root so `--ui-dir ./web/dist` works. This docs refresh is part of it.

Local verification was `go test ./...`, `go vet ./...`, `go build ./cmd/sapie-server`, the Go
harness, client modern+legacy builds and `sapie_login_smoke`, plus the web build. None of that is
a live run.

Limits and risks: no live two-client C++↔Go session, no real MW2 2009 run, no browser click-through,
no screen-reader or keyboard audit, no cross-compiled Windows execution and no real NAT test. Direct
reliable DATA is best-effort over UDP. The 5s relay window is measured from connection creation, so
a slow handshake may pick the relay. There is no relay rate limit beyond existing per-session
handling. Plain HTTP on the LAN means session tokens cross it in cleartext.

The 0.1 P2P notice is unchanged. Changing it is gated on G0 acceptance, which is the user's call.
