package db

import (
	"context"
	"database/sql"
	"errors"
)

type WeekReview struct {
	ID         string  `json:"id"`
	WeekStart  string  `json:"week_start"`
	Wins       *string `json:"wins"`       // JSON array of strings
	Challenges *string `json:"challenges"` // JSON array of strings
	NextFocus  *string `json:"next_focus"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type WeekReviewStore struct {
	db *sql.DB
}

func NewWeekReviewStore(db *sql.DB) *WeekReviewStore {
	return &WeekReviewStore{db: db}
}

func (s *WeekReviewStore) Get(ctx context.Context, weekStart string) (WeekReview, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, week_start, wins, challenges, next_focus, created_at, updated_at
		 FROM week_reviews WHERE week_start = ?`, weekStart)
	return scanWeekReview(row)
}

// List returns all week reviews, newest first. limit <= 0 means no limit.
func (s *WeekReviewStore) List(ctx context.Context, limit int) ([]WeekReview, error) {
	q := `SELECT id, week_start, wins, challenges, next_focus, created_at, updated_at
		 FROM week_reviews ORDER BY week_start DESC`
	args := []any{}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := []WeekReview{}
	for rows.Next() {
		r, err := scanWeekReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

func (s *WeekReviewStore) Upsert(ctx context.Context, id, weekStart string, wins, challenges, nextFocus *string) (WeekReview, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO week_reviews (id, week_start, wins, challenges, next_focus)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(week_start) DO UPDATE SET
			wins       = excluded.wins,
			challenges = excluded.challenges,
			next_focus = excluded.next_focus,
			updated_at = datetime('now')
		RETURNING id, week_start, wins, challenges, next_focus, created_at, updated_at`,
		id, weekStart, wins, challenges, nextFocus,
	)
	return scanWeekReview(row)
}

func scanWeekReview(s scanner) (WeekReview, error) {
	var r WeekReview
	var wins, challenges, nextFocus sql.NullString
	err := s.Scan(&r.ID, &r.WeekStart, &wins, &challenges, &nextFocus, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WeekReview{}, ErrNotFound
	}
	if err != nil {
		return WeekReview{}, err
	}
	r.Wins = nullStr(wins)
	r.Challenges = nullStr(challenges)
	r.NextFocus = nullStr(nextFocus)
	return r, nil
}
