-- Reserve this migration for the application's first durable data structures.
-- Add subsequent migrations instead of editing applied migration files.

CREATE TABLE IF NOT EXISTS bot_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
