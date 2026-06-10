-- Track which calendar each Fastmail event belongs to, so the UI can let the
-- user show/hide individual calendars (not just "all Fastmail events").
ALTER TABLE fastmail_cal_events ADD COLUMN calendar_name TEXT NOT NULL DEFAULT '';
