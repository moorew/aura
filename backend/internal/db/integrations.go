package db

import (
	"context"
	"database/sql"
	"errors"
)

type IntegrationConfig struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Enabled      bool    `json:"enabled"`
	Config       string  `json:"config"` // raw JSON
	LastSyncedAt *string `json:"last_synced_at"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

const cfgCols = `id, type, enabled, config, last_synced_at, created_at, updated_at`

func scanIntegration(s scanner) (IntegrationConfig, error) {
	var c IntegrationConfig
	var enabled int64
	var lastSynced sql.NullString
	err := s.Scan(&c.ID, &c.Type, &enabled, &c.Config, &lastSynced, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return IntegrationConfig{}, err
	}
	c.Enabled = enabled == 1
	c.LastSyncedAt = nullStr(lastSynced)
	return c, nil
}

type IntegrationConfigStore struct{ db *sql.DB }

func NewIntegrationConfigStore(db *sql.DB) *IntegrationConfigStore {
	return &IntegrationConfigStore{db: db}
}

func (s *IntegrationConfigStore) Get(ctx context.Context, typ string) (IntegrationConfig, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+cfgCols+` FROM integration_configs WHERE type = ?`, typ)
	c, err := scanIntegration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationConfig{}, ErrNotFound
	}
	return c, err
}

func (s *IntegrationConfigStore) Upsert(ctx context.Context, id, typ, configJSON string) (IntegrationConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO integration_configs (id, type, config)
		VALUES (?,?,?)
		ON CONFLICT(type) DO UPDATE SET
			config     = excluded.config,
			enabled    = 1,
			updated_at = datetime('now')
		RETURNING `+cfgCols,
		id, typ, configJSON,
	)
	return scanIntegration(row)
}

func (s *IntegrationConfigStore) UpdateConfig(ctx context.Context, typ, configJSON string) (IntegrationConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE integration_configs SET config = ?, updated_at = datetime('now')
		WHERE type = ?
		RETURNING `+cfgCols,
		configJSON, typ,
	)
	c, err := scanIntegration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationConfig{}, ErrNotFound
	}
	return c, err
}

func (s *IntegrationConfigStore) TouchSyncTime(ctx context.Context, typ string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE integration_configs SET last_synced_at = datetime('now') WHERE type = ?`, typ)
	return err
}

func (s *IntegrationConfigStore) SetEnabled(ctx context.Context, typ string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE integration_configs SET enabled=?, updated_at=datetime('now') WHERE type=?`, v, typ)
	return err
}

func (s *IntegrationConfigStore) Delete(ctx context.Context, typ string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM integration_configs WHERE type = ?`, typ)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
