// Package ical fetches and parses ICS/iCalendar feeds.
package ical

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Event is a parsed VEVENT from an ICS feed.
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	StartTime   string // ISO-8601
	EndTime     string // ISO-8601
	AllDay      bool
}

// Fetch downloads and parses an ICS URL, returning all events.
func Fetch(url string) ([]Event, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("ical fetch %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ical fetch: HTTP %d", resp.StatusCode)
	}
	return Parse(resp.Body)
}

// Parse reads an ICS stream and returns parsed events.
func Parse(r io.Reader) ([]Event, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}

	var events []Event
	var cur *Event

	for _, line := range lines {
		prop, val, _ := strings.Cut(line, ":")
		// Strip parameters (e.g. DTSTART;TZID=America/Toronto → DTSTART)
		prop = strings.ToUpper(strings.SplitN(prop, ";", 2)[0])

		switch prop {
		case "BEGIN":
			if strings.EqualFold(val, "VEVENT") {
				cur = &Event{}
			}
		case "END":
			if strings.EqualFold(val, "VEVENT") && cur != nil {
				events = append(events, *cur)
				cur = nil
			}
		}

		if cur == nil {
			continue
		}

		switch prop {
		case "UID":
			cur.UID = val
		case "SUMMARY":
			cur.Summary = unescapeText(val)
		case "DESCRIPTION":
			cur.Description = unescapeText(val)
		case "LOCATION":
			cur.Location = unescapeText(val)
		case "DTSTART":
			cur.StartTime, cur.AllDay = parseICSTime(line)
		case "DTEND", "DTEND;VALUE=DATE":
			cur.EndTime, _ = parseICSTime(line)
		}
	}
	return events, nil
}

// unfold joins continuation lines (RFC 5545 §3.1).
func unfold(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	var cur strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			cur.WriteString(strings.TrimLeft(line, " \t"))
		} else {
			if cur.Len() > 0 {
				lines = append(lines, cur.String())
			}
			cur.Reset()
			cur.WriteString(line)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines, scanner.Err()
}

// parseICSTime handles date-only (YYYYMMDD) and datetime (YYYYMMDDTHHMMSSZ) values.
// It returns an ISO-8601 string and whether the event is all-day.
func parseICSTime(rawLine string) (string, bool) {
	// Extract value after last ":"
	_, val, _ := strings.Cut(rawLine, ":")
	val = strings.TrimSpace(val)

	// All-day: YYYYMMDD
	if len(val) == 8 {
		if t, err := time.Parse("20060102", val); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	// UTC datetime: YYYYMMDDTHHMMSSZ
	if strings.HasSuffix(val, "Z") {
		if t, err := time.Parse("20060102T150405Z", val); err == nil {
			return t.Format(time.RFC3339), false
		}
	}
	// Local datetime: YYYYMMDDTHHMMSS (treat as UTC for simplicity)
	if t, err := time.Parse("20060102T150405", val); err == nil {
		return t.UTC().Format(time.RFC3339), false
	}
	return val, false
}

func unescapeText(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\N", "\n")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
