# SAPIE wire protocol (v1, M0)

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
   session_token, friends}`. `friends` is empty in M0.

`BANNED`, `SERVER_FULL` and `VERSION_MISMATCH` statuses exist in the contract but
are never emitted in M0.

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

## Supported frames in M0

| Client → server | Server behaviour |
| --- | --- |
| `LoginRequest` | Authenticate; reply `LoginResponse` |
| `ping` | Reply `pong` |
| `Common_Message` | `source_id` stamped, logged and dropped (routing seam) |
| `TicketRequest` | Reply `TicketResponse` (see App tickets) |
| `StatsGet`, `StatsSet`, other variants | Logged and ignored (no-op acknowledgement in M0) |

The server sends a `ping` every 30 s and closes a connection with no frame read
for 90 s.

## Not supported yet

Friend delivery, presence, lobby create/join, invites, achievements and stats are
acknowledged no-ops until M2–M4. Tickets are issued but not verified or redeemed.
There is no P2P game-traffic relay in v1.

## Admin API

Loopback by default (`127.0.0.1:8080`), Bearer token from `--admin-token`.
`GET /healthz` is public; `/api/v1/accounts` supports list, create (with optional
`Idempotency-Key`) and delete by SteamID. Responses never contain password hashes.
