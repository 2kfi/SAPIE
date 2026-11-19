-- UTC timestamps are INTEGER Unix seconds. SteamIDs are INTEGER (base + counter).
CREATE TABLE IF NOT EXISTS accounts (
    steam_id     INTEGER PRIMARY KEY,
    username     TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    persona_name TEXT NOT NULL,
    created_at   INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER))
);

CREATE TABLE IF NOT EXISTS friendships (
    steam_id   INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    friend_id  INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    PRIMARY KEY (steam_id, friend_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    steam_id   INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS lobbies (
    lobby_id   INTEGER PRIMARY KEY,
    owner_id   INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    appid      INTEGER NOT NULL,
    type       INTEGER NOT NULL DEFAULT 0,
    member_limit INTEGER NOT NULL DEFAULT 0,
    joinable   INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER))
);

CREATE TABLE IF NOT EXISTS lobby_members (
    lobby_id  INTEGER NOT NULL REFERENCES lobbies(lobby_id) ON DELETE CASCADE,
    steam_id  INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    joined_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    PRIMARY KEY (lobby_id, steam_id)
);

CREATE TABLE IF NOT EXISTS presence (
    steam_id      INTEGER PRIMARY KEY REFERENCES accounts(steam_id) ON DELETE CASCADE,
    appid         INTEGER NOT NULL DEFAULT 0,
    persona_name  TEXT NOT NULL DEFAULT '',
    lobby_id      INTEGER NOT NULL DEFAULT 0,
    rich_presence BLOB,
    updated_at    INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER))
);

CREATE TABLE IF NOT EXISTS achievements (
    steam_id    INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    appid       INTEGER NOT NULL,
    name        TEXT NOT NULL,
    achieved    INTEGER NOT NULL DEFAULT 0,
    unlock_time INTEGER NOT NULL DEFAULT 0,
    display_name TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    hidden       INTEGER NOT NULL DEFAULT 0,
    icon         TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (steam_id, appid, name)
);

CREATE TABLE IF NOT EXISTS stats (
    steam_id  INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    appid     INTEGER NOT NULL,
    name      TEXT NOT NULL,
    value     BLOB NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    PRIMARY KEY (steam_id, appid, name)
);

CREATE TABLE IF NOT EXISTS auth_tickets (
    number     INTEGER PRIMARY KEY AUTOINCREMENT,
    steam_id   INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    appid      INTEGER NOT NULL,
    data       BLOB,
    created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_friendships_friend ON friendships(friend_id);
CREATE INDEX IF NOT EXISTS idx_sessions_steam_id ON sessions(steam_id);
CREATE INDEX IF NOT EXISTS idx_lobbies_owner ON lobbies(owner_id);
CREATE INDEX IF NOT EXISTS idx_lobbies_appid ON lobbies(appid);
CREATE INDEX IF NOT EXISTS idx_lobby_members_steam ON lobby_members(steam_id);
CREATE INDEX IF NOT EXISTS idx_achievements_appid ON achievements(steam_id, appid);
CREATE INDEX IF NOT EXISTS idx_stats_appid ON stats(steam_id, appid);
CREATE INDEX IF NOT EXISTS idx_auth_tickets_steam ON auth_tickets(steam_id);
