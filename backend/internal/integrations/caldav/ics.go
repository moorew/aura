package caldav

import (
	"fmt"
	"strings"
	"time"
)

// TaskUIDPrefix marks calendar events that originate from Sempa tasks, so they
// can be recognised (and filtered out of the read-side display) elsewhere.
const TaskUIDPrefix = "sempa-task-"

// TaskUID returns the deterministic CalDAV UID for a task. Using a stable UID
// makes PUT an idempotent upsert and DELETE addressable without a mapping table.
func TaskUID(taskID string) string {
	return TaskUIDPrefix + taskID
}

// IsTaskUID reports whether a VEVENT UID was created by Sempa from a task.
func IsTaskUID(uid string) bool {
	return strings.HasPrefix(uid, TaskUIDPrefix)
}

// EventInput is the data needed to render a VEVENT.
type EventInput struct {
	UID         string
	Summary     string
	Description string
	URL         string
	Start       time.Time
	End         time.Time
}

// BuildVCALENDAR renders a single-event VCALENDAR document with UTC timestamps.
func BuildVCALENDAR(in EventInput) string {
	now := time.Now().UTC().Format(icalUTC)
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Sempa//Task Calendar//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	writeLine(&b, "UID", in.UID)
	writeLine(&b, "DTSTAMP", now)
	writeLine(&b, "DTSTART", in.Start.UTC().Format(icalUTC))
	writeLine(&b, "DTEND", in.End.UTC().Format(icalUTC))
	writeLine(&b, "SUMMARY", escapeText(in.Summary))
	if in.Description != "" {
		writeLine(&b, "DESCRIPTION", escapeText(in.Description))
	}
	if in.URL != "" {
		writeLine(&b, "URL", escapeText(in.URL))
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

const icalUTC = "20060102T150405Z"

// ParseTime accepts the timestamp formats stored on a task (RFC3339, with or
// without a zone) and returns a time.Time.
func ParseTime(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("caldav: unrecognised time %q", s)
}

// writeLine emits a property, folding lines longer than 75 octets (RFC 5545).
func writeLine(b *strings.Builder, name, value string) {
	line := name + ":" + value
	for len(line) > 75 {
		b.WriteString(line[:75])
		b.WriteString("\r\n ")
		line = line[75:]
	}
	b.WriteString(line)
	b.WriteString("\r\n")
}

// escapeText escapes a value for an iCalendar TEXT property (RFC 5545 §3.3.11).
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, ";", "\\;")
	return s
}
