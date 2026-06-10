package db

import (
	"context"
	"database/sql"
	"errors"
)

type ICalSubscription struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	URL          string  `json:"url"`
	Color        string  `json:"color"`
	LastSyncedAt *string `json:"last_synced_at"`
	ErrorMsg     *string `json:"error_msg,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type ICalEvent struct {
	ID             string `json:"id"`
	SubscriptionID string `json:"subscription_id"`    // stable per-calendar key (used for show/hide)
	Calendar       string `json:"calendar,omitempty"` // human display name of the source calendar
	UID            string `json:"uid"`
	Summary        string `json:"summary"`
	Description    string `json:"description,omitempty"`
	Location       string `json:"location,omitempty"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	AllDay         bool   `json:"all_day"`
	Color          string `json:"color,omitempty"` // inherited from subscription
}

type ICalStore struct{ db *sql.DB }

func NewICalStore(db *sql.DB) *ICalStore { return &ICalStore{db: db} }

func (s *ICalStore) ListSubscriptions(ctx context.Context) ([]ICalSubscription, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,url,color,last_synced_at,error_msg,created_at,updated_at
		 FROM ical_subscriptions ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ICalSubscription
	for rows.Next() {
		var sub ICalSubscription
		var ls, em sql.NullString
		if err := rows.Scan(&sub.ID, &sub.Name, &sub.URL, &sub.Color, &ls, &em, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		sub.LastSyncedAt = nullStr(ls)
		sub.ErrorMsg = nullStr(em)
		out = append(out, sub)
	}
	return out, nil
}

func (s *ICalStore) CreateSubscription(ctx context.Context, id, name, url, color string) (ICalSubscription, error) {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO ical_subscriptions (id,name,url,color)
		 VALUES (?,?,?,?)
		 RETURNING id,name,url,color,last_synced_at,error_msg,created_at,updated_at`,
		id, name, url, color)
	return scanSubscription(row)
}

func (s *ICalStore) DeleteSubscription(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ical_subscriptions WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ICalStore) UpsertEvents(ctx context.Context, subID string, events []ICalEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, ev := range events {
		allDay := 0
		if ev.AllDay {
			allDay = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ical_events (id,subscription_id,uid,summary,description,location,start_time,end_time,all_day)
			VALUES (?,?,?,?,?,?,?,?,?)
			ON CONFLICT(subscription_id,uid) DO UPDATE SET
				summary=excluded.summary, description=excluded.description,
				location=excluded.location, start_time=excluded.start_time,
				end_time=excluded.end_time, all_day=excluded.all_day,
				updated_at=datetime('now')`,
			ev.ID, subID, ev.UID, ev.Summary, ev.Description, ev.Location,
			ev.StartTime, ev.EndTime, allDay,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE ical_subscriptions SET last_synced_at=datetime('now'), error_msg=NULL, updated_at=datetime('now') WHERE id=?`, subID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *ICalStore) SetError(ctx context.Context, subID, msg string) {
	_, _ = s.db.ExecContext(ctx,
		`UPDATE ical_subscriptions SET error_msg=?,updated_at=datetime('now') WHERE id=?`, msg, subID)
}

func (s *ICalStore) ListEventsForDate(ctx context.Context, date string) ([]ICalEvent, error) {
	// Events where start_time date <= date < end_time date
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.subscription_id, e.uid, e.summary, e.description, e.location,
		       e.start_time, e.end_time, e.all_day, s.color
		FROM ical_events e
		JOIN ical_subscriptions s ON s.id = e.subscription_id
		WHERE substr(e.start_time,1,10) <= ? AND substr(e.end_time,1,10) >= ?
		ORDER BY e.start_time`, date, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ICalEvent
	for rows.Next() {
		var ev ICalEvent
		var desc, loc sql.NullString
		var allDay int
		if err := rows.Scan(&ev.ID, &ev.SubscriptionID, &ev.UID, &ev.Summary,
			&desc, &loc, &ev.StartTime, &ev.EndTime, &allDay, &ev.Color); err != nil {
			return nil, err
		}
		ev.Description = desc.String
		ev.Location = loc.String
		ev.AllDay = allDay == 1
		out = append(out, ev)
	}
	return out, nil
}

func scanSubscription(s scanner) (ICalSubscription, error) {
	var sub ICalSubscription
	var ls, em sql.NullString
	err := s.Scan(&sub.ID, &sub.Name, &sub.URL, &sub.Color, &ls, &em, &sub.CreatedAt, &sub.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ICalSubscription{}, ErrNotFound
	}
	if err != nil {
		return ICalSubscription{}, err
	}
	sub.LastSyncedAt = nullStr(ls)
	sub.ErrorMsg = nullStr(em)
	return sub, nil
}
