package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

type calendarEventsResponse struct {
	Items []calendarEvent `json:"items"`
}

type calendarEvent struct {
	ID      string       `json:"id"`
	Summary string       `json:"summary"`
	Start   calEventTime `json:"start"`
	End     calEventTime `json:"end"`
	HTMLURL string       `json:"htmlLink"`
}

type calEventTime struct {
	DateTime string `json:"dateTime"`
	DateOnly string `json:"date"`
}

func (ct calEventTime) AsDate() string {
	if ct.DateTime != "" {
		t, err := time.Parse(time.RFC3339, ct.DateTime)
		if err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ct.DateOnly
}

// AsISO returns a full ISO-8601 datetime string, or a date-only string if no time component.
func (ct calEventTime) AsISO() string {
	if ct.DateTime != "" {
		return ct.DateTime
	}
	return ct.DateOnly + "T00:00:00Z"
}

// SyncCalendar imports a day's Google Calendar events as tasks.
func SyncCalendar(ctx context.Context, clientID, clientSecret string, stored *StoredToken, tasks *db.TaskStore, targetDate string) (db.SyncResult, error) {
	if err := RefreshAccessToken(ctx, clientID, clientSecret, stored); err != nil {
		return db.SyncResult{}, fmt.Errorf("refresh token: %w", err)
	}

	calIDs := stored.CalendarIDs
	if len(calIDs) == 0 {
		calIDs = []string{"primary"}
	}

	var result db.SyncResult
	for _, calID := range calIDs {
		if err := syncCalendarID(ctx, calID, stored.AccessToken, tasks, targetDate, &result); err != nil {
			result.Errors++
		}
	}
	return result, nil
}

func syncCalendarID(ctx context.Context, calendarID, accessToken string, tasks *db.TaskStore, date string, result *db.SyncResult) error {
	dayStart, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date: %w", err)
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	params := url.Values{}
	params.Set("timeMin", dayStart.UTC().Format(time.RFC3339))
	params.Set("timeMax", dayEnd.UTC().Format(time.RFC3339))
	params.Set("singleEvents", "true")
	params.Set("orderBy", "startTime")
	params.Set("maxResults", "50")

	reqURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?%s",
		url.PathEscape(calendarID), params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calendar API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("calendar API returned HTTP %d", resp.StatusCode)
	}

	var cr calendarEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return err
	}
	for _, ev := range cr.Items {
		if err := upsertCalEvent(ctx, ev, tasks, date, result); err != nil {
			result.Errors++
		}
	}
	return nil
}

func upsertCalEvent(ctx context.Context, ev calendarEvent, tasks *db.TaskStore, date string, result *db.SyncResult) error {
	result.Total++
	if ev.Summary == "" {
		ev.Summary = "(no title)"
	}
	sourceID := "cal_" + ev.ID
	source := "google_calendar"

	scheduledStart := ev.Start.AsISO()
	scheduledEnd   := ev.End.AsISO()

	existing, err := tasks.FindBySource(ctx, source, sourceID)
	if err == nil {
		// Update times on existing tasks (user edits to title/desc are preserved)
		existing.ScheduledStart = &scheduledStart
		existing.ScheduledEnd   = &scheduledEnd
		if _, updateErr := tasks.Update(ctx, existing); updateErr == nil {
			result.Updated++
		}
		return nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return err
	}

	meta, _ := json.Marshal(map[string]string{"date": date, "type": "calendar"})
	metaStr := string(meta)
	status := "planned"
	title := "📅 " + ev.Summary

	_, createErr := tasks.Create(ctx, db.CreateTaskParams{
		ID:             uuid.New().String(),
		Title:          title,
		PlannedDate:    &date,
		Status:         status,
		Position:       float64(time.Now().UnixMilli()),
		Source:         &source,
		SourceID:       &sourceID,
		SourceURL:      &ev.HTMLURL,
		SourceMetadata: &metaStr,
		ScheduledStart: &scheduledStart,
		ScheduledEnd:   &scheduledEnd,
	})
	if createErr != nil {
		return createErr
	}
	result.New++
	return nil
}

// WriteFocusBlock creates a Google Calendar event for a scheduled task.
// Fails gracefully if the token lacks write scope.
func WriteFocusBlock(ctx context.Context, accessToken, calendarID, title, scheduledStart, scheduledEnd, taskURL string) (string, error) {
	if calendarID == "" {
		calendarID = "primary"
	}
	body := map[string]any{
		"summary":     title,
		"description": "Focus work block\n" + taskURL,
		"start":       map[string]string{"dateTime": scheduledStart},
		"end":         map[string]string{"dateTime": scheduledEnd},
	}
	data, _ := json.Marshal(body)

	reqURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events",
		url.PathEscape(calendarID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", nil // no write scope — skip silently
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("calendar write: HTTP %d", resp.StatusCode)
	}
	var created struct{ ID string `json:"id"` }
	_ = json.NewDecoder(resp.Body).Decode(&created)
	return created.ID, nil
}
