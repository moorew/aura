package db

import (
	"database/sql"
	"time"
)

// PushSubscription is a W3C Web Push (VAPID) endpoint registered by a browser /
// PWA. The encryption keys (p256dh, auth) are base64url strings exactly as the
// browser's PushSubscription.toJSON() reports them.
type PushSubscription struct {
	ID        string `json:"id"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
	Platform  string `json:"platform"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type PushSubStore struct {
	db *sql.DB
}

func NewPushSubStore(database *sql.DB) *PushSubStore {
	return &PushSubStore{db: database}
}

// Upsert registers (or refreshes) a subscription keyed by its endpoint.
func (s *PushSubStore) Upsert(id, endpoint, p256dh, auth, platform string) (*PushSubscription, error) {
	now := time.Now().UTC().Format(time.DateTime)
	if platform == "" {
		platform = "web"
	}
	_, err := s.db.Exec(`
		INSERT INTO push_subscriptions (id, endpoint, p256dh, auth, platform, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			p256dh = excluded.p256dh,
			auth = excluded.auth,
			platform = excluded.platform,
			updated_at = excluded.updated_at`,
		id, endpoint, p256dh, auth, platform, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetByEndpoint(endpoint)
}

func (s *PushSubStore) GetByEndpoint(endpoint string) (*PushSubscription, error) {
	row := s.db.QueryRow(
		`SELECT id, endpoint, p256dh, auth, platform, created_at, updated_at
		   FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return scanPushSub(row)
}

func (s *PushSubStore) DeleteByEndpoint(endpoint string) error {
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

func (s *PushSubStore) ListAll() ([]PushSubscription, error) {
	rows, err := s.db.Query(
		`SELECT id, endpoint, p256dh, auth, platform, created_at, updated_at
		   FROM push_subscriptions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushSubscription
	for rows.Next() {
		sub, err := scanPushSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sub)
	}
	return out, rows.Err()
}

func scanPushSub(s scanner) (*PushSubscription, error) {
	var p PushSubscription
	if err := s.Scan(&p.ID, &p.Endpoint, &p.P256dh, &p.Auth, &p.Platform, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}
