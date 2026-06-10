-- Cache of Open Graph / link-preview metadata for URLs found in task notes.
-- Populated on demand by GET /api/v1/unfurl?url=… and reused so we don't
-- re-fetch the same page on every render. Failed fetches are cached too
-- (ok = 0) as a negative cache to avoid hammering dead/blocked links.
CREATE TABLE IF NOT EXISTS link_unfurls (
    url         TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    image_url   TEXT NOT NULL DEFAULT '',
    site_name   TEXT NOT NULL DEFAULT '',
    favicon_url TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,   -- last HTTP status (0 = transport error)
    ok          INTEGER NOT NULL DEFAULT 0,   -- 1 when we got usable metadata
    fetched_at  TEXT NOT NULL                 -- RFC3339 UTC
);
