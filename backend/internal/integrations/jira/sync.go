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
	nextPageToken := ""

	// Resolve the connected account once so each issue can be flagged "mine".
	// Best-effort: if this fails we simply omit the flag (filter stays inert).
	myAccountID := ""
	if me, err := client.Myself(ctx); err == nil {
		myAccountID = me.AccountID
	}

	for {
		sr, err := client.Search(ctx, nextPageToken, 50)
		if err != nil {
			return result, err
		}

		for i := range sr.Issues {
			if err := syncIssue(ctx, &sr.Issues[i], cfg.Host, myAccountID, tasks, &result); err != nil {
				result.Errors++
			}
		}

		if sr.NextPageToken == "" || len(sr.Issues) == 0 {
			break
		}
		nextPageToken = sr.NextPageToken
	}

	return result, nil
}

func syncIssue(ctx context.Context, issue *Issue, host, myAccountID string, tasks *db.TaskStore, result *db.SyncResult) error {
	result.Total++

	metaMap := map[string]any{
		"key":            issue.Key,
		"status":         issue.Fields.Status.Name,
		"statusCategory": issue.Fields.Status.StatusCategory.Key, // new | indeterminate | done
		"issueType":      issue.Fields.IssueType.Name,
		"priority":       priorityName(issue.Fields.Priority),
	}
	if issue.Fields.Assignee != nil {
		metaMap["assignee"] = issue.Fields.Assignee.DisplayName
		metaMap["mine"] = myAccountID != "" && issue.Fields.Assignee.AccountID == myAccountID
	} else {
		metaMap["mine"] = false
	}
	if issue.Fields.Parent != nil {
		metaMap["epicKey"] = issue.Fields.Parent.Key
		metaMap["epicName"] = issue.Fields.Parent.Fields.Summary
	}
	metaBytes, _ := json.Marshal(metaMap)
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

	// Update if the title OR the synced metadata changed. (Previously only a
	// title change persisted, so status/assignee/priority drift was silently
	// dropped — which also broke any metadata-driven sidebar filtering.)
	titleChanged := existing.Title != issue.Fields.Summary
	metaChanged := existing.SourceMetadata == nil || *existing.SourceMetadata != meta
	if titleChanged {
		existing.Title = issue.Fields.Summary
	}
	existing.SourceMetadata = &meta
	existing.SourceURL = &sourceURL

	if titleChanged || metaChanged {
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
