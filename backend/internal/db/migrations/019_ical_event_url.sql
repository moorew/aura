-- Canonical link to the source event (htmlLink / URL property), used by the
-- schedule view's "Open original event in browser" action.
ALTER TABLE ical_events ADD COLUMN url TEXT;
