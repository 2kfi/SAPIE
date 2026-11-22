-- Sliding-window counters for login, game-login and social limits. One row per
-- event; a bucket is a namespace plus an IP or SteamID. Rows expire with the
-- longest window and are deleted on startup and hourly.
CREATE TABLE IF NOT EXISTS rate_events (
    bucket TEXT NOT NULL,
    at     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rate_events_bucket ON rate_events(bucket, at);
