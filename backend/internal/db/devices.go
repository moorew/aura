package db

import (
	"database/sql"
	"time"
)

type DeviceToken struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	Platform  string `json:"platform"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type DeviceTokenStore struct {
	db *sql.DB
}

func NewDeviceTokenStore(database *sql.DB) *DeviceTokenStore {
	return &DeviceTokenStore{db: database}
}

func (s *DeviceTokenStore) Upsert(id, token, platform string) (*DeviceToken, error) {
	now := time.Now().UTC().Format(time.DateTime)
	_, err := s.db.Exec(`
		INSERT INTO device_tokens (id, token, platform, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET updated_at = ?`,
		id, token, platform, now, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetByToken(token)
}

func (s *DeviceTokenStore) GetByToken(token string) (*DeviceToken, error) {
	row := s.db.QueryRow(`SELECT id, token, platform, created_at, updated_at FROM device_tokens WHERE token = ?`, token)
	var d DeviceToken
	if err := row.Scan(&d.ID, &d.Token, &d.Platform, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (s *DeviceTokenStore) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM device_tokens WHERE token = ?`, token)
	return err
}

func (s *DeviceTokenStore) ListAll() ([]DeviceToken, error) {
	rows, err := s.db.Query(`SELECT id, token, platform, created_at, updated_at FROM device_tokens ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceToken
	for rows.Next() {
		var d DeviceToken
		if err := rows.Scan(&d.ID, &d.Token, &d.Platform, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
