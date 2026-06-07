package caldav

import (
	"context"
	"fmt"

	"github.com/clevercode/sempa/internal/db"
)

// Schedulable reports whether a task currently has a complete time block that
// should be mirrored to the calendar.
func Schedulable(t db.Task) bool {
	return t.ScheduledStart != nil && *t.ScheduledStart != "" &&
		t.ScheduledEnd != nil && *t.ScheduledEnd != "" &&
		t.ArchivedAt == nil
}

// eventForTask renders the VCALENDAR body for a schedulable task.
func eventForTask(t db.Task, appURL string) (string, error) {
	start, err := ParseTime(*t.ScheduledStart)
	if err != nil {
		return "", err
	}
	end, err := ParseTime(*t.ScheduledEnd)
	if err != nil {
		return "", err
	}
	desc := ""
	if t.Description != nil {
		desc = *t.Description
	}
	url := ""
	if appURL != "" {
		url = appURL + "/task/" + t.ID
	}
	return BuildVCALENDAR(EventInput{
		UID:         TaskUID(t.ID),
		Summary:     t.Title,
		Description: desc,
		URL:         url,
		Start:       start,
		End:         end,
	}), nil
}

// PushTask mirrors a single task to the calendar: it PUTs the event when the
// task is schedulable, or DELETEs it otherwise (e.g. unscheduled or archived).
func PushTask(ctx context.Context, c *Client, calendarHref string, t db.Task, appURL string) error {
	uid := TaskUID(t.ID)
	if !Schedulable(t) {
		return c.DeleteEvent(ctx, calendarHref, uid)
	}
	ics, err := eventForTask(t, appURL)
	if err != nil {
		return err
	}
	return c.PutEvent(ctx, calendarHref, uid, ics)
}

// DeleteTask removes a task's event from the calendar (used when a task is
// deleted entirely).
func DeleteTask(ctx context.Context, c *Client, calendarHref, taskID string) error {
	return c.DeleteEvent(ctx, calendarHref, TaskUID(taskID))
}

// SyncAll pushes every scheduled task to the calendar. Returns the number of
// events written. Individual failures are collected but do not abort the run.
func SyncAll(ctx context.Context, c *Client, calendarHref string, tasks *db.TaskStore, appURL string) (int, error) {
	list, err := tasks.ListScheduled(ctx)
	if err != nil {
		return 0, err
	}
	written := 0
	var firstErr error
	for _, t := range list {
		ics, err := eventForTask(t, appURL)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := c.PutEvent(ctx, calendarHref, TaskUID(t.ID), ics); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		written++
	}
	if written == 0 && firstErr != nil {
		return 0, fmt.Errorf("caldav sync: %w", firstErr)
	}
	return written, nil
}
