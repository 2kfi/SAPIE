# SAPIE HTTP API

Base path: `/api/v1`. Requests and responses use JSON. The admin API listens on
`127.0.0.1:8080` by default. The LAN web UI listens on `0.0.0.0:8081` and exposes
only the user routes listed in the proxy section.

## Authentication

| Credential | Header | Used for |
| --- | --- | --- |
| Session token | `Authorization: Bearer <session_token>` | User routes |
| Admin token | `Authorization: Bearer <admin_token>` | Admin routes and account CRUD |

`POST /login` returns a fresh session token on every call. Old tokens stay valid
until they expire or are revoked. Session rows store a SHA-256 digest of the
token, never the token itself. `GET /sessions` shows a 12-character digest
prefix as the session ID, so the list cannot be used to recover a token.

Missing or invalid credentials return `401` with `WWW-Authenticate: Bearer`.
Revoked tokens fail immediately on the next request.

## Error codes

| Status | Meaning |
| --- | --- |
| `400` | Invalid JSON, missing query, bad ID or invalid field length |
| `401` | Missing, expired or revoked credential |
| `403` | Public registration is disabled |
| `404` | Missing resource, wrong account, or a lookup that must not disclose existence |
| `429` | Social rate limit exceeded |
| `500` | Server-side failure |

For friend requests, an unknown username, a self request, an existing friend,
a blocked pair and a request owned by another account all return `404`. This is
deliberate: the response does not let a caller enumerate accounts or relationships.

## Idempotency and rate limits

`POST /api/v1/accounts`, `POST /api/v1/friends/requests`, accept, decline and
block accept an optional `Idempotency-Key` header. Keys are scoped to the
authenticated account, so the same key on another account is a separate call.
Only successful responses are cached. A failed call can be retried with the same
key and will run again.

`sendFriendRequest`, `searchUsers` and `blockFriend` share a per-account limit
of 30 calls per hour. The counter is in memory and resets on server restart.

## Public routes

| Method and path | Handler | Request | Response |
| --- | --- | --- | --- |
| `GET /healthz` | anonymous | none | `{"status":"ok"}` |
| `GET /config` | anonymous | none | `{"allow_register":true}` |
| `POST /register` | `register` | `{"username":"alice","password":"pw","persona_name":"Alice"}` | `201` account object, or `403` when registration is off |
| `POST /login` | `login` | `{"username":"alice","password":"pw"}` | `200 {"session_token":"...","steam_id":76561197960265729,"persona_name":"Alice"}` |

## User routes

All routes below require a session token.

| Method and path | Handler | Request | Response |
| --- | --- | --- | --- |
| `GET /friends` | `listFriends` | none | `{"friends":[{steam_id,persona_name,persona_state,appid,rich_presence,lobby_id,connect_str}]}` |
| `POST /friends/{steam_id}/invite` | `inviteFriend` | `{"connect_str":"connect=127.0.0.1:27015"}` | `202`; target must remain an unblocked friend |
| `GET /friends/requests` | `listFriendRequests` | none | `{"incoming":[...],"outgoing":[...]}` |
| `POST /friends/requests` | `sendFriendRequest` | `{"username":"bob","message":"play?"}` | `201 {"id":1,"status":"pending","created_at":1795199695}` |
| `POST /friends/requests/{id}/accept` | `acceptFriendRequest` | none | `200 {"status":"accepted"}` |
| `POST /friends/requests/{id}/decline` | `declineFriendRequest` | none | `200 {"status":"declined"}` |
| `DELETE /friends/requests/{id}` | `deleteFriendRoute` to `cancelFriendRequest` | none | `204`; sender only |
| `DELETE /friends/{steam_id}` | `removeFriend` | none | `204` |
| `POST /friends/{steam_id}/block` | `blockFriend` | none | `204` |
| `DELETE /friends/{steam_id}/block` | `deleteFriendRoute` to `unblockFriend` | none | `204` |
| `GET /friends/blocked` | `listBlocked` | none | `{"blocked":[{steam_id,persona_name,created_at}]}` |
| `GET /users/search?q=bo` | `searchUsers` | none | `{"users":[{steam_id,persona_name}]}` |
| `GET /users/{steam_id}/stats` | `userStats` | none | Own stats, or `401` for another account |
| `GET /lobbies` | `listLobbies` | none | `{"lobbies":[{lobby_id,owner,appid,member_limit,type,joinable,members}]}` |
| `POST /lobbies` | `createLobby` | `{"appid":480,"type":0,"member_limit":4}` | `201 {"lobby_id":1}` |
| `POST /lobbies/{lobby_id}/join` | `joinLobby` | none | `200` and a `Lobby` push to members |
| `POST /logout` | `logout` | none | `204`; revokes the presented token |
| `POST /logout-all` | `logoutAll` | none | `204`; revokes every session for the account |
| `GET /sessions` | `listSessions` | none | `{"sessions":[{id,created_at,expires_at,current}]}` |
| `DELETE /sessions/{id}` | `revokeSession` | none | `204`, or `404` for another account's session |

A request item has this shape:

```json
{
  "id": 1,
  "from_steam_id": 76561197960265729,
  "to_steam_id": 76561197960265730,
  "persona_name": "Bob",
  "message": "play?",
  "created_at": 1795199695,
  "status": "pending"
}
```

`persona_name` is the other participant. Status is `pending`, `accepted` or
`declined`; only pending requests appear in the list.

Search `q` must be 2 to 64 characters. It matches an exact username or a
leading prefix and returns at most 10 results. It returns only `steam_id` and
`persona_name`, never password hashes, full account rows or tokens. A blocked
account can still appear as a search hit; requests, presence and invites to it
are rejected.

Removing a friend deletes both friendship directions and cancels pending
invites between that pair. Blocking hides presence in both directions and
cancels pending invites between that pair, but keeps the friendship rows so
unblocking restores the graph. Accept, remove, block and unblock push a fresh
friend snapshot over the game socket.

## Admin routes

| Method and path | Handler | Request | Response |
| --- | --- | --- | --- |
| `GET /accounts` | `listAccounts` | none | All accounts with presence |
| `POST /accounts` | `createAccount` | Same body as `/register` | `201` account object; `Idempotency-Key` supported |
| `DELETE /accounts/{id}` | `deleteAccount` | none | `204` |
| `POST /tickets/verify` | `verifyTicket` | `{"ticket":"...","steam_id":76561197960265729,"appid":480,"number":1}` | `{"valid":true}` |
| `GET /users` | `listUsers` | none | All accounts with presence; no hashes or tokens |

## LAN proxy

Port 8081 forwards only these user routes to the admin listener:

- `GET /config`, `POST /register`, `POST /login`
- `GET /friends`, `POST /friends/{id}/invite`
- `GET /friends/requests`, `POST /friends/requests`
- `POST /friends/requests/{id}/accept`, `POST /friends/requests/{id}/decline`
- `DELETE /friends/{a}/{b}`, `DELETE /friends/{steam_id}`
- `POST /friends/{steam_id}/block`, `GET /friends/blocked`
- `GET /users/search`, `GET /users/{steam_id}/stats`
- `GET /lobbies`, `POST /lobbies`, `POST /lobbies/{id}/join`
- `POST /logout`, `POST /logout-all`, `GET /sessions`, `DELETE /sessions/{id}`

Admin-only paths such as `/api/v1/users` and account CRUD return `404` on 8081.
Because 8081 is plain HTTP on the LAN, session tokens cross it in cleartext.
Use a reverse proxy with TLS when the LAN is not trusted.
