// Package ical fetches and parses ICS/iCalendar feeds.
package ical

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Event is a parsed VEVENT from an ICS feed.
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	URL         string // canonical link to the event (htmlLink etc.), if present
	StartTime   string // ISO-8601
	EndTime     string // ISO-8601
	AllDay      bool
}

// isPrivateIP checks if an IP belongs to a private, loopback, or link-local range.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// validateURL ensures the URL is http(s) and does not resolve to a private/internal IP.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q; only http and https are allowed", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to a private/internal IP address; refusing to fetch")
		}
	}
	return nil
}

// Fetch downloads and parses an ICS URL, returning all events.
// It validates the URL to prevent SSRF against private networks.
func Fetch(rawURL string) ([]Event, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, fmt.Errorf("ical: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("ical fetch %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// Limit response body to 10 MB to avoid memory exhaustion
	body := io.LimitReader(resp.Body, 10<<20)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ical fetch: HTTP %d", resp.StatusCode)
	}
	return Parse(body)
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
		case "URL":
			cur.URL = strings.TrimSpace(val)
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

// parseICSTime handles date-only (YYYYMMDD) and datetime (YYYYMMDDTHHMMSS[Z])
// values, honouring the DTSTART/DTEND TZID parameter so events keep their true
// instant. It returns an ISO-8601 string and whether the event is all-day.
//
// Three cases, each emitting a representation the JS client can interpret
// without drifting hours:
//   - UTC ("…Z"):        RFC3339 with a Z → client converts to local correctly.
//   - Zoned (TZID=…):    parsed in that zone, RFC3339 with the real offset.
//   - Floating (neither): emitted WITHOUT a zone designator so the client reads
//     it as local wall-clock time — never coerced to UTC (the old behaviour,
//     which shifted every floating/zoned event by the viewer's offset).
func parseICSTime(rawLine string) (string, bool) {
	// Split "NAME;PARAM=…:VALUE" into its params and value halves.
	head, val, _ := strings.Cut(rawLine, ":")
	val = strings.TrimSpace(val)
	tzid := paramValue(head, "TZID")
	valueType := paramValue(head, "VALUE")

	// All-day: YYYYMMDD (or explicit VALUE=DATE).
	if valueType == "DATE" || (len(val) == 8 && !strings.Contains(val, "T")) {
		if t, err := time.Parse("20060102", val); err == nil {
			return t.Format("2006-01-02"), true
		}
	}

	// UTC datetime: YYYYMMDDTHHMMSSZ.
	if strings.HasSuffix(val, "Z") {
		if t, err := time.Parse("20060102T150405Z", val); err == nil {
			return t.Format(time.RFC3339), false
		}
	}

	// Zoned datetime: parse the wall-clock time *in its named zone* so the
	// resulting RFC3339 carries the correct offset.
	if tzid != "" {
		if loc := loadLocation(tzid); loc != nil {
			if t, err := time.ParseInLocation("20060102T150405", val, loc); err == nil {
				return t.Format(time.RFC3339), false
			}
		}
	}

	// Floating local datetime: YYYYMMDDTHHMMSS with no zone. Emit it without a
	// designator so the client treats it as its own local time.
	if t, err := time.Parse("20060102T150405", val); err == nil {
		return t.Format("2006-01-02T15:04:05"), false
	}
	return val, false
}

// paramValue extracts an iCalendar property parameter (e.g. TZID) from the
// portion of a content line before the ":" — "DTSTART;TZID=America/New_York".
func paramValue(head, name string) string {
	parts := strings.Split(head, ";")
	for _, p := range parts[1:] {
		k, v, ok := strings.Cut(p, "=")
		if ok && strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// loadLocation resolves a TZID to a *time.Location. It tries the IANA name
// directly (covers Google/Apple/Fastmail feeds), then a small map of common
// Windows zone names that Outlook emits. Unknown zones return nil so the caller
// falls back to floating local time rather than guessing wrong.
func loadLocation(tzid string) *time.Location {
	if loc, err := time.LoadLocation(tzid); err == nil {
		return loc
	}
	if iana, ok := windowsToIANA[tzid]; ok {
		if loc, err := time.LoadLocation(iana); err == nil {
			return loc
		}
	}
	return nil
}

// windowsToIANA maps the most common Outlook/Windows TZID strings to IANA zones.
var windowsToIANA = map[string]string{
	"Eastern Standard Time":          "America/New_York",
	"Central Standard Time":          "America/Chicago",
	"Mountain Standard Time":         "America/Denver",
	"Pacific Standard Time":          "America/Los_Angeles",
	"GMT Standard Time":              "Europe/London",
	"Greenwich Standard Time":        "Atlantic/Reykjavik",
	"W. Europe Standard Time":        "Europe/Berlin",
	"Central Europe Standard Time":   "Europe/Budapest",
	"Romance Standard Time":          "Europe/Paris",
	"Central European Standard Time": "Europe/Warsaw",
	"AUS Eastern Standard Time":      "Australia/Sydney",
	"India Standard Time":            "Asia/Kolkata",
	"Tokyo Standard Time":            "Asia/Tokyo",
	"China Standard Time":            "Asia/Shanghai",
}

func unescapeText(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\N", "\n")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
