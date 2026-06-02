package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)


// messageListResponse from the Gmail messages.list endpoint
type messageListResponse struct {
	Messages           []messageRef `json:"messages"`
	NextPageToken      string       `json:"nextPageToken"`
	ResultSizeEstimate int          `json:"resultSizeEstimate"`
}

type messageRef struct {
	ID string `json:"id"`
}

type messageDetail struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
	Payload struct {
		Headers []header `json:"headers"`
	} `json:"payload"`
}

type header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (m *messageDetail) header(name string) string {
	for _, h := range m.Payload.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

func Sync(ctx context.Context, clientID, clientSecret string, stored *StoredToken, tasks *db.TaskStore) (db.SyncResult, error) {
	if err := RefreshAccessToken(ctx, clientID, clientSecret, stored); err != nil {
		return db.SyncResult{}, fmt.Errorf("refresh token: %w", err)
	}

	var result db.SyncResult
	pageToken := ""

	for {
		msgRefs, next, err := listMessages(ctx, stored.AccessToken, stored.Labels, pageToken)
		if err != nil {
			return result, err
		}

		for _, ref := range msgRefs {
			if err := syncMessage(ctx, ref.ID, stored.AccessToken, tasks, &result); err != nil {
				result.Errors++
			}
		}

		if next == "" {
			break
		}
		pageToken = next
	}

	return result, nil
}

func listMessages(ctx context.Context, accessToken string, labels []string, pageToken string) ([]messageRef, string, error) {
	params := url.Values{}
	for _, l := range labels {
		params.Add("labelIds", l)
	}
	params.Set("maxResults", "100")
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	reqURL := "https://gmail.googleapis.com/gmail/v1/users/me/messages?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("list messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("list messages returned HTTP %d", resp.StatusCode)
	}

	var lr messageListResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, "", err
	}

	return lr.Messages, lr.NextPageToken, nil
}

func syncMessage(ctx context.Context, msgID, accessToken string, tasks *db.TaskStore, result *db.SyncResult) error {
	result.Total++

	// Check if this message is already imported
	_, err := tasks.FindBySource(ctx, "gmail", msgID)
	if err == nil {
		return nil // already exists, skip (subject doesn't change for a message)
	}
	if !errors.Is(err, db.ErrNotFound) {
		return err
	}

	// Fetch message details
	detail, err := fetchMessage(ctx, msgID, accessToken)
	if err != nil {
		return err
	}

	subject := detail.header("Subject")
	if subject == "" {
		subject = "(no subject)"
	}
	from := detail.header("From")
	date := detail.header("Date")

	metaBytes, _ := json.Marshal(map[string]string{
		"from":    from,
		"date":    date,
		"snippet": detail.Snippet,
	})
	meta    := string(metaBytes)
	source  := "gmail"
	sourceURL := "https://mail.google.com/mail/u/0/#inbox/" + msgID

	_, createErr := tasks.Create(ctx, db.CreateTaskParams{
		ID:             uuid.New().String(),
		Title:          subject,
		Status:         "backlog",
		Position:       float64(result.Total) * 1000,
		Source:         &source,
		SourceID:       &msgID,
		SourceURL:      &sourceURL,
		SourceMetadata: &meta,
	})
	if createErr != nil {
		return createErr
	}
	result.New++
	return nil
}

func fetchMessage(ctx context.Context, msgID, accessToken string) (*messageDetail, error) {
	params := url.Values{}
	params.Set("format", "metadata")
	params.Set("metadataHeaders", "Subject")
	params.Set("metadataHeaders", "From")
	params.Set("metadataHeaders", "Date")

	reqURL := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s?%s",
		msgID, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch message %s returned HTTP %d", msgID, resp.StatusCode)
	}

	var detail messageDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}
