package fastmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ImproveTitle uses Claude Haiku to turn an email subject into a concise
// action-oriented task title. Falls back to the stripped subject on any error.
func ImproveTitle(ctx context.Context, apiKey, subject string) string {
	if apiKey == "" || subject == "" {
		return subject
	}

	prompt := fmt.Sprintf(
		"Convert this email subject into a brief, action-oriented task title. "+
			"Maximum 8 words. Start with a verb. Remove any newsletter boilerplate, "+
			"company names, or urgency language. Return ONLY the task title, nothing else.\n\n"+
			"Subject: %q\n\nTask title:", subject)

	body, _ := json.Marshal(map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 60,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return subject
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return subject
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return subject
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return subject
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Content) == 0 {
		return subject
	}

	title := strings.TrimSpace(result.Content[0].Text)
	// Strip any surrounding quotes the model might add
	title = strings.Trim(title, `"'`)
	if title == "" {
		return subject
	}
	return title
}
