package fastmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

type jmapCalEvent struct {
	ID              string                     `json:"id"`
	UID             string                     `json:"uid"`
	Title           string                     `json:"title"`
	Description     interface{}                `json:"description"` // string or {value, type}
	Locations       map[string]jmapLocation    `json:"locations"`
	Start           string                     `json:"start"`
	Duration        string                     `json:"duration"`
	TimeZone        string                     `json:"timeZone"`
	ShowWithoutTime bool                       `json:"showWithoutTime"`
	CalendarIds     map[string]bool            `json:"calendarIds"`
}

type jmapLocation struct {
	Name string `json:"name"`
}

type jmapCalendar struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// GetCalendars returns all calendars for the account with their colors.
func (c *Client) GetCalendars(ctx context.Context) ([]jmapCalendar, error) {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return nil, err
		}
	}
	if c.calAccount == "" {
		return nil, fmt.Errorf("no JMAP Calendars capability — check account permissions")
	}

	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:calendars"},
		MethodCalls: [][]interface{}{
			{"Calendar/get", map[string]interface{}{
				"accountId":  c.calAccount,
				"ids":        nil,
				"properties": []string{"id", "name", "color"},
			}, "0"},
		},
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Calendar/get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Calendar/get: HTTP %d", resp.StatusCode)
	}

	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}
	for _, mc := range jr.MethodResponses {
		if name, _ := mc[0].(string); name != "Calendar/get" {
			continue
		}
		data, _ := json.Marshal(mc[1])
		var result struct {
			List []jmapCalendar `json:"list"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result.List, nil
	}
	return nil, nil
}

// SyncCalendar fetches calendar events for the given date range and stores them.
// dateFrom and dateTo are YYYY-MM-DD strings (inclusive).
func SyncCalendar(ctx context.Context, cfg Config, store *db.FastmailCalStore, dateFrom, dateTo string) (int, error) {
	client := NewClient(cfg)
	if err := client.Discover(ctx); err != nil {
		return 0, fmt.Errorf("discover: %w", err)
	}
	if client.calAccount == "" {
		return 0, fmt.Errorf("this Fastmail account does not have JMAP Calendars access")
	}

	// Get calendar colors
	cals, _ := client.GetCalendars(ctx)
	calColors := make(map[string]string, len(cals))
	for _, cal := range cals {
		if cal.Color != "" {
			calColors[cal.ID] = cal.Color
		}
	}

	// Query events for the date range
	events, err := client.getCalendarEventsForRange(ctx, dateFrom, dateTo)
	if err != nil {
		return 0, err
	}

	dbEvents := make([]db.FastmailCalEvent, 0, len(events))
	for _, ev := range events {
		// Determine color from parent calendar
		color := "#6b7280"
		for calID := range ev.CalendarIds {
			if c, ok := calColors[calID]; ok {
				color = c
			}
			break
		}

		// Parse description (may be string or {value, type} object)
		desc := extractDescription(ev.Description)

		// First location name
		loc := ""
		for _, l := range ev.Locations {
			loc = l.Name
			break
		}

		startTime := ev.Start
		endTime := computeEndTime(ev.Start, ev.Duration)
		if endTime == "" {
			endTime = startTime
		}

		dbEvents = append(dbEvents, db.FastmailCalEvent{
			ID:          ev.ID,
			UID:         coalesceStr(ev.UID, ev.ID),
			Summary:     ev.Title,
			Description: desc,
			Location:    loc,
			StartTime:   startTime,
			EndTime:     endTime,
			AllDay:      ev.ShowWithoutTime,
			Color:       color,
		})
	}

	if err := store.UpsertEvents(ctx, dbEvents); err != nil {
		return 0, err
	}
	return len(dbEvents), nil
}

func (c *Client) getCalendarEventsForRange(ctx context.Context, dateFrom, dateTo string) ([]jmapCalEvent, error) {
	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:calendars"},
		MethodCalls: [][]interface{}{
			{"CalendarEvent/query", map[string]interface{}{
				"accountId": c.calAccount,
				"filter": map[string]interface{}{
					"after":  dateFrom + "T00:00:00",
					"before": dateTo + "T23:59:59",
				},
			}, "0"},
			{"CalendarEvent/get", map[string]interface{}{
				"accountId": c.calAccount,
				"#ids": map[string]interface{}{
					"resultOf": "0",
					"name":     "CalendarEvent/query",
					"path":     "/ids/*",
				},
				"properties": []string{
					"id", "uid", "title", "description", "locations",
					"start", "duration", "timeZone", "showWithoutTime", "calendarIds",
				},
			}, "1"},
		},
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CalendarEvent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CalendarEvent: HTTP %d", resp.StatusCode)
	}

	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}

	for _, mc := range jr.MethodResponses {
		if name, _ := mc[0].(string); name != "CalendarEvent/get" {
			continue
		}
		data, _ := json.Marshal(mc[1])
		var result struct {
			List []jmapCalEvent `json:"list"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result.List, nil
	}
	return nil, nil
}

// extractDescription handles the JMAP description field which may be a string
// or a {value, type} object (JSCalendar contentType).
func extractDescription(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case map[string]interface{}:
		if val, ok := v["value"].(string); ok {
			return val
		}
	}
	return ""
}

// computeEndTime adds a JSCalendar ISO 8601 duration to a local datetime string.
func computeEndTime(start, duration string) string {
	if start == "" {
		return ""
	}
	d := parseISO8601Duration(duration)
	if d == 0 {
		// Default: 1 hour for timed events, 1 day for all-day
		if len(start) == 10 {
			d = 24 * time.Hour
		} else {
			d = time.Hour
		}
	}

	var t time.Time
	var err error
	if len(start) == 10 {
		t, err = time.Parse("2006-01-02", start)
	} else {
		t, err = time.Parse("2006-01-02T15:04:05", start)
	}
	if err != nil {
		return start
	}
	result := t.Add(d)
	if len(start) == 10 {
		return result.Format("2006-01-02")
	}
	return result.Format("2006-01-02T15:04:05")
}

// parseISO8601Duration parses durations like PT1H, P1D, PT30M, PT1H30M.
func parseISO8601Duration(s string) time.Duration {
	if s == "" || s[0] != 'P' {
		return 0
	}
	s = s[1:] // skip 'P'
	var total time.Duration

	tIdx := strings.IndexByte(s, 'T')
	datePart := s
	timePart := ""
	if tIdx >= 0 {
		datePart = s[:tIdx]
		timePart = s[tIdx+1:]
	}

	// Parse days from date part
	if dIdx := strings.IndexByte(datePart, 'D'); dIdx >= 0 {
		n, _ := strconv.Atoi(datePart[:dIdx])
		total += time.Duration(n) * 24 * time.Hour
	}

	// Parse hours, minutes, seconds from time part
	rest := timePart
	for len(rest) > 0 {
		i := strings.IndexAny(rest, "HMS")
		if i < 0 {
			break
		}
		n, _ := strconv.Atoi(rest[:i])
		switch rest[i] {
		case 'H':
			total += time.Duration(n) * time.Hour
		case 'M':
			total += time.Duration(n) * time.Minute
		case 'S':
			total += time.Duration(n) * time.Second
		}
		rest = rest[i+1:]
	}
	return total
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
