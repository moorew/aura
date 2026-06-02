package jira

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)


func Sync(ctx context.Context, cfg Config, tasks *db.TaskStore) (db.SyncResult, error) {
	client := NewClient(cfg)
	var result db.SyncResult
	startAt := 0

	for {
		sr, err := client.Search(ctx, startAt, 50)
		if err != nil {
			return result, err
		}

		for i := range sr.Issues {
			if err := syncIssue(ctx, &sr.Issues[i], cfg.Host, tasks, &result); err != nil {
				result.Errors++
			}
		}

		startAt += len(sr.Issues)
		if startAt >= sr.Total || len(sr.Issues) == 0 {
			break
		}
	}

	return result, nil
}

func syncIssue(ctx context.Context, issue *Issue, host string, tasks *db.TaskStore, result *db.SyncResult) error {
	result.Total++

	metaBytes, _ := json.Marshal(map[string]any{
		"key":       issue.Key,
		"status":    issue.Fields.Status.Name,
		"issueType": issue.Fields.IssueType.Name,
		"priority":  priorityName(issue.Fields.Priority),
	})
	meta     := string(metaBytes)
	source   := "jira"
	sourceID := issue.Key
	sourceURL := host + "/browse/" + issue.Key

	existing, err := tasks.FindBySource(ctx, "jira", issue.Key)
	if errors.Is(err, db.ErrNotFound) {
		_, createErr := tasks.Create(ctx, db.CreateTaskParams{
			ID:             uuid.New().String(),
			Title:          issue.Fields.Summary,
			Status:         "backlog",
			Position:       float64(result.Total) * 1000,
			Source:         &source,
			SourceID:       &sourceID,
			SourceURL:      &sourceURL,
			SourceMetadata: &meta,
		})
		if createErr != nil {
			return createErr
		}
		result.New++
		return nil
	}
	if err != nil {
		return err
	}

	// Update if the title or metadata changed
	titleChanged := existing.Title != issue.Fields.Summary
	if titleChanged {
		existing.Title = issue.Fields.Summary
	}
	existing.SourceMetadata = &meta
	existing.SourceURL = &sourceURL

	if titleChanged {
		if _, updateErr := tasks.Update(ctx, existing); updateErr != nil {
			return updateErr
		}
		result.Updated++
	}

	return nil
}

func priorityName(p *Priority) string {
	if p == nil {
		return "None"
	}
	return p.Name
}
