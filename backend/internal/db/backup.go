package db

import (
	"context"
	"database/sql"
)

// BackupSettings is the single-row backup configuration.
type BackupSettings struct {
	Enabled       bool    `json:"enabled"`
	ScheduleHour  int     `json:"schedule_hour"`
	Retention     int     `json:"retention"`
	SecurityMode  string  `json:"security_mode"` // 'none' | 'encrypt' | 'exclude_secrets'
	Passphrase    string  `json:"-"`             // never serialised to clients
	HasPassphrase bool    `json:"has_passphrase"`
	Destinations  string  `json:"destinations"` // raw JSON array
	LastRunAt     *string `json:"last_run_at"`
	LastStatus    *string `json:"last_status"`
	LastError     *string `json:"last_error"`
	UpdatedAt     string  `json:"updated_at"`
}

type BackupRun struct {
	ID           string  `json:"id"`
	StartedAt    string  `json:"started_at"`
	FinishedAt   *string `json:"finished_at"`
	Trigger      string  `json:"trigger"`
	Status       string  `json:"status"`
	SizeBytes    *int64  `json:"size_bytes"`
	Filename     *string `json:"filename"`
	Destinations *string `json:"destinations"`
	Error        *string `json:"error"`
}

type BackupStore struct{ db *sql.DB }

func NewBackupStore(db *sql.DB) *BackupStore { return &BackupStore{db: db} }

func (s *BackupStore) Get(ctx context.Context) (BackupSettings, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT enabled, schedule_hour, retention, security_mode,
		       COALESCE(passphrase, ''), destinations,
		       last_run_at, last_status, last_error, updated_at
		FROM backup_settings WHERE id = 1`)
	var b BackupSettings
	var enabled int64
	var lastRun, lastStatus, lastErr sql.NullString
	if err := row.Scan(&enabled, &b.ScheduleHour, &b.Retention, &b.SecurityMode,
		&b.Passphrase, &b.Destinations, &lastRun, &lastStatus, &lastErr, &b.UpdatedAt); err != nil {
		return BackupSettings{}, err
	}
	b.Enabled = enabled == 1
	b.HasPassphrase = b.Passphrase != ""
	b.LastRunAt = nullStr(lastRun)
	b.LastStatus = nullStr(lastStatus)
	b.LastError = nullStr(lastErr)
	return b, nil
}

// UpdateSettings updates the configurable fields. passphrase==nil leaves the
// existing passphrase untouched; passphrase pointing at "" clears it.
func (s *BackupStore) UpdateSettings(ctx context.Context, enabled bool, scheduleHour, retention int,
	securityMode, destinations string, passphrase *string) error {
	en := 0
	if enabled {
		en = 1
	}
	if passphrase != nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE backup_settings
			SET enabled=?, schedule_hour=?, retention=?, security_mode=?,
			    destinations=?, passphrase=?, updated_at=datetime('now')
			WHERE id = 1`,
			en, scheduleHour, retention, securityMode, destinations, *passphrase)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE backup_settings
		SET enabled=?, schedule_hour=?, retention=?, security_mode=?,
		    destinations=?, updated_at=datetime('now')
		WHERE id = 1`,
		en, scheduleHour, retention, securityMode, destinations)
	return err
}

func (s *BackupStore) RecordResult(ctx context.Context, status string, errMsg *string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE backup_settings
		SET last_run_at=datetime('now'), last_status=?, last_error=?
		WHERE id = 1`, status, errMsg)
	return err
}

func (s *BackupStore) InsertRun(ctx context.Context, r BackupRun) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO backup_runs (id, started_at, finished_at, trigger, status, size_bytes, filename, destinations, error)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		r.ID, r.StartedAt, r.FinishedAt, r.Trigger, r.Status, r.SizeBytes, r.Filename, r.Destinations, r.Error)
	return err
}

func (s *BackupStore) ListRuns(ctx context.Context, limit int) ([]BackupRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, started_at, finished_at, trigger, status, size_bytes, filename, destinations, error
		FROM backup_runs ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		var r BackupRun
		var finished, filename, dests, errMsg sql.NullString
		var size sql.NullInt64
		if err := rows.Scan(&r.ID, &r.StartedAt, &finished, &r.Trigger, &r.Status,
			&size, &filename, &dests, &errMsg); err != nil {
			return nil, err
		}
		r.FinishedAt = nullStr(finished)
		r.SizeBytes = nullInt(size)
		r.Filename = nullStr(filename)
		r.Destinations = nullStr(dests)
		r.Error = nullStr(errMsg)
		out = append(out, r)
	}
	if out == nil {
		out = []BackupRun{}
	}
	return out, rows.Err()
}

// SchemaVersion returns the latest applied migration version (for backup manifests).
func (s *BackupStore) SchemaVersion(ctx context.Context) string {
	var v sql.NullString
	_ = s.db.QueryRowContext(ctx,
		`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	return v.String
}
