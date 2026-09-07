CREATE TABLE IF NOT EXISTS birthday_settings (
    guild_id TEXT PRIMARY KEY,
    staff_role_id TEXT,
    notification_channel_id TEXT,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    last_notified_date TEXT
);

CREATE TABLE IF NOT EXISTS birthdays (
    guild_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    month INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
    day INTEGER NOT NULL CHECK (day BETWEEN 1 AND 31),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX IF NOT EXISTS birthdays_by_guild_date ON birthdays (guild_id, month, day);
