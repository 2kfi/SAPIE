# SAPIE wire protocol (v1, M4)

The game client talks to `cmd/sapie-server` over TCP (default `:47584`). Optional TLS
is enabled by passing both `--tls-cert` and `--tls-key`.

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
   stored name, appid, lobby and rich presence (offline friends included).

`BANNED`, `SERVER_FULL` and `VERSION_MISMATCH` statuses exist in the contract but
are never emitted.

## Identity rule

`Common_Message.source_id` is **always overwritten** by the server with the
authenticated connection's server-assigned SteamID. Client-supplied identity is
never trusted. SteamIDs are `76561197960265728 + monotonic counter`.

## App tickets

After login a client may send `TicketRequest{appid, data}`; the server replies
`TicketResponse{ticket, number}`. `ticket` is 32 opaque bytes derived from a
fresh random nonce plus the authenticated SteamID and requested appid; `number`
is a per-session counter starting at 1. No ticket record is kept, so tickets
cannot be verified or redeemed yet.

## Supported frames

| Client → server | Server behaviour |
| --- | --- |
| `LoginRequest` | Authenticate; reply `LoginResponse` with friends |
| `ping` | Reply `pong` |
| `Common_Message{Friend}` | Store own presence, then fan out to friends |
| `Common_Message{Friend_Messages}` | Record game-invite connect string, then deliver to `dest_id` or all friends |
| `Common_Message{Lobby}`, `{Lobby_Messages}` | Fan out to the lobby's members |
| Other `Common_Message` variants | `source_id` stamped, logged and dropped |
| `TicketRequest` | Reply `TicketResponse` (see App tickets) |
| `StatsGet` | Reply `StatsResponse` for `steam_id` (0 means self) |
| `AchievementUnlock` | Persist the unlock and push it to friends and lobby mates |
| `StatsSet`, other variants | Logged and ignored (no-op acknowledgement) |

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
stored in the database and returned by the admin API.

`StatsGet{appid, names, steam_id}` queries another user by `steam_id`; zero
means the caller. The reply is `StatsResponse{stats, achievements, steam_id}`.
Each achievement has both the legacy `unlock_time` (`uint32`) and the canonical
`unlock_time_unix` (`int64`). Use the latter for new clients.

## Not supported yet

Tickets are issued but not verified or redeemed. There is no P2P game-traffic
relay in v1.

## HTTP API

Base `/api/v1`, JSON bodies, Bearer auth. Admin calls use `--admin-token`; user
calls use the `session_token` from login. The same per-IP login limiter guards
game and HTTP login. CORS allows only localhost, 127.0.0.1 and tauri.localhost
origins over http, https or tauri.

| Method and path | Auth | Result |
| --- | --- | --- |
| `POST /login` | public | `{session_token, steam_id, persona_name}`; 401 on bad credentials |
| `GET /friends` | user | `{friends:[{steam_id, persona_name, persona_state, appid, rich_presence, lobby_id, connect_str}]}` |
| `POST /friends/{steam_id}/invite` | user | 202, pushes `Friend_Messages{GAME_INVITE}` with the body's `connect_str` |
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
