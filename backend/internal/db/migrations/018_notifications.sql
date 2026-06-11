-- Per-task hard reminders + Web Push subscriptions.

-- remind_at: an exact timestamp (RFC3339 / "2006-01-02 15:04:05" UTC) at which a
-- hard reminder fires. NULL = no reminder. Syncs to local-first clients via taskCols.
ALTER TABLE tasks ADD COLUMN remind_at TEXT;

-- reminder_sent_at: dedupe guard set once a reminder has been dispatched. Snoozing
-- a reminder sets remind_at = now+1h and clears this back to NULL so it fires again.
-- Server-only: NOT part of taskCols, never synced to clients.
ALTER TABLE tasks ADD COLUMN reminder_sent_at TEXT;

CREATE INDEX IF NOT EXISTS idx_tasks_remind_at ON tasks(remind_at) WHERE remind_at IS NOT NULL;

-- W3C Web Push (VAPID) subscriptions. Distinct from device_tokens, which holds
-- Firebase FCM registration tokens for the native Capacitor Android app.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id         TEXT PRIMARY KEY,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,           -- client public key (base64url)
    auth       TEXT NOT NULL,           -- client auth secret (base64url)
    platform   TEXT NOT NULL DEFAULT 'web',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
