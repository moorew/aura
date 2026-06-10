package db

import (
	"context"
	"database/sql"
)

type FastmailCalEvent struct {
	ID           string `json:"id"`
	UID          string `json:"uid"`
	Summary      string `json:"summary"`
	Description  string `json:"description,omitempty"`
	Location     string `json:"location,omitempty"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	AllDay       bool   `json:"all_day"`
	Color        string `json:"color"`
	CalendarName string `json:"calendar_name,omitempty"`
}

type FastmailCalStore struct{ db *sql.DB }

func NewFastmailCalStore(db *sql.DB) *FastmailCalStore { return &FastmailCalStore{db: db} }

func (s *FastmailCalStore) UpsertEvents(ctx context.Context, events []FastmailCalEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, ev := range events {
		allDay := 0
		if ev.AllDay {
			allDay = 1
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO fastmail_cal_events (id,uid,summary,description,location,start_time,end_time,all_day,color,calendar_name,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,datetime('now'))
			ON CONFLICT(uid) DO UPDATE SET
			  summary=excluded.summary, description=excluded.description,
			  location=excluded.location, start_time=excluded.start_time,
			  end_time=excluded.end_time, all_day=excluded.all_day,
			  color=excluded.color, calendar_name=excluded.calendar_name,
			  updated_at=excluded.updated_at`,
			ev.ID, ev.UID, ev.Summary, ev.Description, ev.Location,
			ev.StartTime, ev.EndTime, allDay, ev.Color, ev.CalendarName)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *FastmailCalStore) ListEventsForDate(ctx context.Context, date string) ([]FastmailCalEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,uid,summary,COALESCE(description,''),COALESCE(location,''),
		       start_time,end_time,all_day,color,COALESCE(calendar_name,'')
		FROM fastmail_cal_events
		WHERE date(start_time) = ? OR (date(start_time) <= ? AND date(end_time) > ?)
		ORDER BY start_time`, date, date, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FastmailCalEvent
	for rows.Next() {
		var ev FastmailCalEvent
		var allDay int
		if err := rows.Scan(&ev.ID, &ev.UID, &ev.Summary, &ev.Description, &ev.Location,
			&ev.StartTime, &ev.EndTime, &allDay, &ev.Color, &ev.CalendarName); err != nil {
			return nil, err
		}
		ev.AllDay = allDay == 1
		out = append(out, ev)
	}
	return out, nil
}

func (s *FastmailCalStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM fastmail_cal_events`)
	return err
}
