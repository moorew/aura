package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/clevercode/sempa/internal/db"
	"github.com/google/uuid"
)

// destResult is the per-destination outcome recorded for a run.
type destResult struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Status string `json:"status"` // 'success' | 'error'
	Error  string `json:"error,omitempty"`
	Pruned int    `json:"pruned,omitempty"`
}

// Run builds a bundle and pushes it to every enabled destination, then prunes
// old backups to the retention limit. The result is recorded in backup_runs.
// driveToken resolves a Google Drive access token (may be nil if Drive unused).
func (s *Service) Run(ctx context.Context, trigger string, driveToken DriveTokenFunc) (db.BackupRun, error) {
	settings, err := s.store.Get(ctx)
	if err != nil {
		return db.BackupRun{}, err
	}

	run := db.BackupRun{
		ID:        uuid.New().String(),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Trigger:   trigger,
	}

	finish := func(status string, size *int64, filename string, results []destResult, runErr error) (db.BackupRun, error) {
		fin := time.Now().UTC().Format(time.RFC3339)
		run.FinishedAt = &fin
		run.Status = status
		run.SizeBytes = size
		if filename != "" {
			run.Filename = &filename
		}
		if len(results) > 0 {
			if b, e := json.Marshal(results); e == nil {
				rs := string(b)
				run.Destinations = &rs
			}
		}
		if runErr != nil {
			es := runErr.Error()
			run.Error = &es
		}
		_ = s.store.InsertRun(ctx, run)
		var errMsg *string
		if runErr != nil {
			es := runErr.Error()
			errMsg = &es
		}
		_ = s.store.RecordResult(ctx, status, errMsg)
		return run, runErr
	}

	dests, err := ParseDestinations(settings.Destinations)
	if err != nil {
		return finish("error", nil, "", nil, fmt.Errorf("bad destination config: %w", err))
	}
	enabled := make([]DestConfig, 0, len(dests))
	for _, d := range dests {
		if d.Enabled {
			enabled = append(enabled, d)
		}
	}
	if len(enabled) == 0 {
		return finish("error", nil, "", nil, fmt.Errorf("no enabled backup destinations"))
	}

	// Build once, push everywhere.
	result, err := s.Build(ctx, settings.SecurityMode, settings.Passphrase)
	if err != nil {
		return finish("error", nil, "", nil, fmt.Errorf("build backup: %w", err))
	}
	defer result.Cleanup()
	size := result.Size

	var results []destResult
	var firstErr error
	for _, dc := range enabled {
		res := destResult{Type: dc.Type, Name: dc.Name, Status: "success"}
		dest, err := NewDestination(dc, driveToken)
		if err != nil {
			res.Status, res.Error = "error", err.Error()
			results = append(results, res)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := dest.Put(ctx, result.Filename, result.Path); err != nil {
			res.Status, res.Error = "error", err.Error()
			results = append(results, res)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		res.Pruned = s.prune(ctx, dest, settings.Retention)
		results = append(results, res)
	}

	status := "success"
	if firstErr != nil {
		status = "error"
	}
	return finish(status, &size, result.Filename, results, firstErr)
}

// prune deletes the oldest backups beyond the retention count. Best-effort.
func (s *Service) prune(ctx context.Context, dest Destination, retention int) int {
	if retention < 1 {
		retention = 1
	}
	files, err := dest.List(ctx)
	if err != nil || len(files) <= retention {
		return 0
	}
	removed := 0
	for _, f := range files[retention:] {
		if err := dest.Delete(ctx, f.ID); err == nil {
			removed++
		}
	}
	return removed
}
