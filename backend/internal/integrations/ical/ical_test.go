package ical

import (
	"strings"
	"testing"
	"time"
)

func TestParseICSTime(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    string // exact for fixed cases; "" means use wantInstant
		allDay  bool
		instant string // RFC3339 instant the value must equal (zone-independent check)
	}{
		{
			name:   "all-day date",
			line:   "DTSTART;VALUE=DATE:20260610",
			want:   "2026-06-10",
			allDay: true,
		},
		{
			name:   "bare 8-digit date is all-day",
			line:   "DTSTART:20260610",
			want:   "2026-06-10",
			allDay: true,
		},
		{
			name:    "UTC datetime keeps the instant",
			line:    "DTSTART:20260610T140000Z",
			instant: "2026-06-10T14:00:00Z",
		},
		{
			name:    "zoned datetime resolves via TZID",
			line:    "DTSTART;TZID=America/New_York:20260610T140000",
			instant: "2026-06-10T18:00:00Z", // EDT is UTC-4 in June
		},
		{
			name: "floating datetime emitted without a zone",
			line: "DTSTART:20260610T140000",
			want: "2026-06-10T14:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, allDay := parseICSTime(tt.line)
			if allDay != tt.allDay {
				t.Fatalf("allDay = %v, want %v", allDay, tt.allDay)
			}
			if tt.want != "" {
				if got != tt.want {
					t.Fatalf("got %q, want %q", got, tt.want)
				}
				return
			}
			// Compare as instants so an equivalent offset still passes.
			parsed, err := time.Parse(time.RFC3339, got)
			if err != nil {
				t.Fatalf("output %q is not RFC3339: %v", got, err)
			}
			want, _ := time.Parse(time.RFC3339, tt.instant)
			if !parsed.Equal(want) {
				t.Fatalf("instant %s != want %s (raw %q)", parsed.UTC(), want.UTC(), got)
			}
		})
	}
}

func TestParseEventURL(t *testing.T) {
	ics := strings.Join([]string{
		"BEGIN:VEVENT",
		"UID:abc123",
		"SUMMARY:Standup",
		"URL:https://calendar.example.com/event/abc123",
		"DTSTART;TZID=America/New_York:20260610T090000",
		"DTEND;TZID=America/New_York:20260610T093000",
		"END:VEVENT",
	}, "\r\n")

	events, err := Parse(strings.NewReader(ics))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].URL != "https://calendar.example.com/event/abc123" {
		t.Fatalf("URL = %q", events[0].URL)
	}
}
