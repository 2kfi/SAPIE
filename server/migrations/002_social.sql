-- Social graph additions. friend_requests.status is pending, accepted or declined.
CREATE TABLE IF NOT EXISTS friend_requests (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id     INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    to_id       INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    message     TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined')),
    CHECK (from_id <> to_id),
    UNIQUE (from_id, to_id)
);

CREATE TABLE IF NOT EXISTS blocks (
    steam_id   INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    blocked_id INTEGER NOT NULL REFERENCES accounts(steam_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    PRIMARY KEY (steam_id, blocked_id),
    CHECK (steam_id <> blocked_id)
);

CREATE INDEX IF NOT EXISTS idx_friend_requests_from ON friend_requests(from_id, status);
CREATE INDEX IF NOT EXISTS idx_friend_requests_to ON friend_requests(to_id, status);
CREATE INDEX IF NOT EXISTS idx_blocks_blocked ON blocks(blocked_id);
