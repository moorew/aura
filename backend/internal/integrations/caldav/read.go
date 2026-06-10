package caldav

import (
	"context"
	"strings"
	"time"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/ical"
)

// rangeBound formats a YYYY-MM-DD date as a CalDAV UTC time bound
// (YYYYMMDDTHHMMSSZ). When end is true the bound is pushed to the start of the
// following day so the range is inclusive of dateTo.
func rangeBound(date string, end bool) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	if end {
		t = t.AddDate(0, 0, 1)
	}
	return t.UTC().Format("20060102T000000Z")
}

// ReadCalendarEvents discovers every calendar on the account and returns the
// external events overlapping [dateFrom, dateTo]. Events that Sempa itself
// wrote from tasks (recognisable by their UID prefix) are excluded so the
// read-side calendar view doesn't double-show task time-blocks.
//
// This uses CalDAV (Basic auth + app password) rather than JMAP because
// Fastmail's JMAP session endpoint rejects app-password Basic auth ("not
// bearer"); the same app password works fine for CalDAV, which is already used
// to push task time-blocks.
func ReadCalendarEvents(ctx context.Context, c *Client, dateFrom, dateTo string) ([]db.FastmailCalEvent, error) {
	cals, err := c.ListCalendars(ctx)
	if err != nil {
		return nil, err
	}

	start := rangeBound(dateFrom, false)
	end := rangeBound(dateTo, true)

	var out []db.FastmailCalEvent
	seen := make(map[string]bool)

	for _, cal := range cals {
		color := cal.Color
		if color == "" {
			color = "#6b7280"
		}
		raws, err := c.QueryEvents(ctx, cal.Href, start, end)
		if err != nil {
			// One unreadable calendar shouldn't sink the whole sync.
			continue
		}
		for _, raw := range raws {
			evs, perr := ical.Parse(strings.NewReader(raw.ICS))
			if perr != nil {
				continue
			}
			for _, ev := range evs {
				if ev.UID == "" || ev.StartTime == "" {
					continue
				}
				if IsTaskUID(ev.UID) {
					continue // our own task block — already shown as a task
				}
				if seen[ev.UID] {
					continue
				}
				seen[ev.UID] = true
				out = append(out, db.FastmailCalEvent{
					ID:           ev.UID,
					UID:          ev.UID,
					Summary:      ev.Summary,
					Description:  ev.Description,
					Location:     ev.Location,
					StartTime:    ev.StartTime,
					EndTime:      ev.EndTime,
					AllDay:       ev.AllDay,
					Color:        color,
					CalendarName: cal.Name,
				})
			}
		}
	}
	return out, nil
}
