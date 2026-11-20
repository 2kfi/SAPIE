# SAPIE wire protocol

The game client talks to `cmd/sapie-server` over TCP (default `:47584`, `--listen`). This is a
binary protobuf protocol, not HTTP; a browser pointed at that port shows a connection reset,
which is expected. Optional TLS is enabled by passing both `--tls-cert` and `--tls-key`.

0.1 is published and frozen. 0.2 adds P2P signaling, direct DATA and relay fallback in code;
0.2 is not tagged or released and G0 is deferred to the user.

## Framing

| Field | Size | Meaning |
| --- | --- | --- |
| length | 4 bytes | big-endian `uint32`, length of the protobuf payload |
| payload | `length` bytes | one `sapie.Frame` |

Maximum frame size is **1 MiB**. Oversized frames are rejected before decoding;
a zero length is invalid. A truncated header or body closes the connection.

## Login handshake

1. Client opens TCP (or TLS) and sends its **first** frame as `LoginRequest`
   (`username`, `password`, `appid`, `client_version`). Any other first payload
   closes the connection without a response.
2. Server verifies the password against the stored argon2id hash. Failed logins
   return `LoginResponse{status: BAD_CREDENTIALS}`; five failures per source IP
   within one minute trigger a temporary block.
3. On success the server creates a 32-byte session token (stored hashed, 24 h
   lifetime) and replies `LoginResponse{status: OK, steam_id, persona_name,
   session_token, friends}`. `friends` lists the account's friends with their
   stored name, appid, lobby and rich presence (offline friends included), plus
   the `persona_state` key (`1` online, `0` offline). `connect_str` is stripped
   from the rich-presence map.

`BANNED`, `SERVER_FULL` and `VERSION_MISMATCH` statuses exist in the contract but
are never emitted.

## Identity rule

`Common_Message.source_id` is **always overwritten** by the server with the
authenticated connection's server-assigned SteamID. Client-supplied identity is
never trusted. SteamIDs are `76561197960265728 + monotonic counter`.

For `Networking_Sockets` P2P frames the server also stamps a direct-connect hint:
`source_ip` (field 128) is set from the authenticated TCP connection's IPv4
address, never from the client. `source_port` (field 129) is taken from the
client-declared real bound UDP port, falling back to the TCP port when the
declaration is missing or out of range. IPv6 peers get no hint (no IPv4 address
is available). The hint is advisory; the server never trusts a client-supplied
`source_ip`.

## App tickets

After login a client may send `TicketRequest{appid, data}`; the server replies
`TicketResponse{ticket, number}`. `ticket` is 32 opaque bytes derived from a
fresh random nonce plus the authenticated SteamID and requested appid; `number`
is a per-session counter starting at 1. Each issued ticket is recorded in the
existing `auth_tickets` table with its nonce, SteamID, appid, number and a
10-minute expiry. `POST /api/v1/tickets/verify` lets an admin check that a
presented ticket was issued by this server to the given SteamID/appid/number
and has not expired. That proves issuance only: it does not check game ownership
with Steam, does not redeem or consume tickets, and game clients do not call it.

The shipped C++ client does **not** send `TicketRequest` and has no branch that
reads a server `TicketResponse`; it still synthesizes `encrypted_app_ticket`
locally. The server-side ticket path is therefore available but unused by the
current game client.

## Supported frames

| Client → server | Server behaviour |
| --- | --- |
| `LoginRequest` | Authenticate; reply `LoginResponse` with friends |
| `ping` | Reply `pong` |
| `Common_Message{Friend}` | Store own presence, then fan out to friends |
| `Common_Message{Friend_Messages}` | Record game-invite connect string, then deliver to `dest_id` or all friends |
| `Common_Message{Lobby}`, `{Lobby_Messages}` | Fan out to the lobby's members |
| `Common_Message{Networking_Sockets}`, `{Networking_Messages}` (signaling) | P2P signaling; allowed only to friends/lobby mates, delivered to `dest_id`. `source_ip`/`source_port` hint stamped on `Networking_Sockets`. |
| `Common_Message{Networking_Sockets}`, `{Networking_Messages}` with `DATA` | Relay fallback: forwarded to the explicit `dest_id` peer only (friend/lobby gate, no broadcast). See P2P traffic. |
| Other `Common_Message` variants | `source_id` stamped, logged and dropped |
| `TicketRequest` | Reply `TicketResponse` (see App tickets); not sent by the shipped client |
| `StatsGet` | Reply `StatsResponse` for `steam_id` (0 means self), filtered by `appid` and, when `names` is supplied, to those stat names |
| `AchievementUnlock` | Persist the unlock; push visible unlocks to friends and lobby mates, never push hidden ones |
| `StatsSet`, other variants | Logged and ignored; no response frame is sent |

Server pushes use `Common_Message` frames: presence is a `Friend` with a
`persona_state` rich-presence key (`1` online, `0` offline), invites are
`Friend_Messages{GAME_INVITE}`, and lobby updates are `Lobby` frames. Login and
disconnect push presence to friends. The server sends a `ping` every 30 s and
closes a connection with no frame read for 90 s.

## Achievements and stats

`AchievementUnlock{appid, def}` is sent by the unlocking client. `def` carries
`id`, `display_name`, `description`, `hidden` and `icon`. The server ignores any
client clock: it stamps one canonical `unlock_time` (Unix seconds, `int64`)
from the database when the record is first created. Repeat unlocks keep that
first timestamp and only refresh the definition fields.

Observers are the union of the unlocker's friends and the members of every
lobby the unlocker has joined. Each receives a `Common_Message` with
`achievement_unlock` set; `source_id` is the unlocker and `steam_id` inside the
message is the same value. The `unlock_time` in the push is the exact integer
stored in the database and returned by the admin API. Achievements marked
`hidden` are stored canonically but are never pushed to friends or lobby mates:
the whole push is skipped rather than sending a redacted record, so existing
clients get no malformed or empty definition. The owner still sees the hidden
achievement through `StatsGet{steam_id: 0}`, and the admin HTTP stats endpoint
still returns it.

`StatsGet{appid, names, steam_id}` queries another user by `steam_id`; zero
means the caller. A nonzero `appid` limits both stats and achievements to that
app; zero returns every app. A nonempty `names` list limits the returned stats
to those exact names; an empty list returns all stats, and `names` does not
affect achievements. When the caller is not the owner, achievements marked
`hidden` are omitted entirely, including their names and unlock times.
The reply is `StatsResponse{stats, achievements, steam_id}`. Each achievement
has both the legacy `unlock_time` (`uint32`) and the canonical
`unlock_time_unix` (`int64`). Use the latter for new clients. The admin HTTP
stats endpoint is unaffected by these game-socket filters and returns all apps.

## Appid and ownership

`appid` is a label used to filter stats and achievements and to tag lobbies,
presence and tickets. There is no per-account app-ownership table: any
authenticated account can name any appid in `LoginRequest`, `TicketRequest`,
`StatsGet` or lobby calls. Verifying a ticket confirms only that this server
issued those bytes to that SteamID for that appid; it is not a Steam entitlement
check. A real ownership gate would need a new table and a game-client check,
neither of which is part of this wire contract.

## P2P traffic (0.2 code, unreleased)

Game traffic uses `Common_Message` with `Networking_Sockets` or
`Networking_Messages` payloads. Two paths exist, and the client picks exactly
one per frame so a packet is never sent twice:

- **Direct UDP.** When a peer hint is available (`source_ip`/`source_port` from
  the stamped `Networking_Sockets` signaling frame), the client sends DATA
  straight to the peer via `sendToPeer()`. Direct is preferred. Reliable DATA
  over UDP is best-effort; the server does not guarantee delivery. There is no
  NAT hole punching, and IPv6 peers get no hint.
- **Relay fallback.** When there is no hint, or no inbound DATA has been seen
  within a 5-second window, the client sends the DATA frame to the server with
  an explicit `dest_id`, and the server forwards it to that peer only.
  Authorization is the same friend/lobby gate used for signaling; there is no
  broadcast and a frame addressed to self is dropped. The 5-second window is
  measured from connection creation, so a slow handshake may pick the relay
  even when a direct path would work.

Relaying is rate-limited only by the existing per-session handling; there is no
dedicated relay rate limit. No live two-client C++↔Go session or real NAT test
has been run.

## Not supported yet

Tickets can be checked for issuance by an admin but are not redeemed or
consumed, and no Steam ownership check exists. Tickets are not one-time nor
bound to a peer. P2P game traffic (direct and relay) exists in 0.2 code but is
unreleased; the 0.1 P2P notice stays until G0 acceptance.

Known 0.1 limit, closed in 0.2 code:

- The C++ client's `friends()` and `lobbies()` returned references to internal
  lists rather than copies. In 0.2 they return locked copies, and local lobby
  mutations write back to the mirror explicitly. No live two-client run has
  exercised this.

## HTTP API

Base `/api/v1`, JSON bodies, Bearer auth. Admin calls use `--admin-token`; user
calls use the `session_token` from login. The same per-IP login limiter guards
game and HTTP login. CORS allows only localhost, 127.0.0.1 and tauri.localhost
origins over http, https or tauri.

The admin API listens on `--admin-listen` (default `127.0.0.1:8080`, loopback).
The 0.2 web UI listens separately on `--ui-listen` (default `0.0.0.0:8081`) and
serves the SPA from `--ui-dir` (default `./web/dist`). On 8081 a narrow reverse
proxy forwards only user routes: `config`, `register`, `login`, `friends`,
`invite`, `lobbies`, `join` and one's own stats. Admin-only paths such as
`/api/v1/users` and account CRUD return 404 on 8081 and still work on 8080.
Because 8081 is plain HTTP on the LAN, session tokens cross it in cleartext.

| Method and path | Auth | Result |
| --- | --- | --- |
| `GET /config` | public | `{allow_register}`; drives whether the SPA offers registration |
| `POST /register` | public when `--allow-register` | Creates an account; 403 when registration is off |
| `POST /login` | public | `{session_token, steam_id, persona_name}`; 401 on bad credentials |
| `GET /friends` | user | `{friends:[{steam_id, persona_name, persona_state, appid, rich_presence, lobby_id, connect_str}]}` |
| `POST /friends/{steam_id}/invite` | user | 202, pushes `Friend_Messages{GAME_INVITE}` with the body's `connect_str` |
| `POST /tickets/verify` | admin | 200 `{valid}` for `{ticket, steam_id, appid, number}`; issuance check only |
| `GET /lobbies` | user or admin | `{lobbies:[{lobby_id, owner, appid, member_limit, type, joinable, members}]}` |
| `POST /lobbies` | user | 201 `{lobby_id}`; body `{appid,type,member_limit}` |
| `POST /lobbies/{lobby_id}/join` | user | 200, pushes a `Lobby` update to members |
| `GET /users` | admin | All accounts with presence; no password hashes or tokens |
| `GET /users/{steam_id}/stats` | admin, or the matching user | `{steam_id, stats:[{name,value}], achievements:[{steam_id,appid,id,display_name,description,hidden,icon,achieved,unlock_time}]}` |
| `GET /accounts`, `POST /accounts`, `DELETE /accounts/{id}` | admin | Account management; create accepts `Idempotency-Key` |

`rich_presence` values and stat `value` fields are byte maps, so JSON encodes
them as base64 strings. `unlock_time` is Unix seconds as a JSON number, the
same integer sent in the `AchievementUnlock` push. A user token can only read
its own stats path; any other user returns 401.
`connect_str` is stored in the inviter's presence and exposed to friends.
`persona_state` is `1` when the user holds a live game session, `0` otherwise.
`GET /healthz` is public. All responses omit password hashes and session tokens
except `POST /login`.
