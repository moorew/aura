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

// ImproveTitle uses a local Ollama model to turn an email subject into a
// concise action-oriented task title. Falls back to the stripped subject on
// any error or if Ollama is not configured.
func ImproveTitle(ctx context.Context, ollamaBaseURL, model, subject string) string {
	if ollamaBaseURL == "" || subject == "" {
		return subject
	}
	if model == "" {
		model = "qwen2.5:1.5b"
	}

	prompt := fmt.Sprintf(
		"Convert this email subject into a brief, action-oriented task title. "+
			"Maximum 8 words. Start with a verb. Remove newsletter boilerplate, "+
			"company names, and urgency language. Return ONLY the task title.\n\n"+
			"Subject: %q\n\nTask title:", subject)

	body, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ollamaBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return subject
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
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
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return subject
	}

	title := strings.TrimSpace(result.Response)
	title = strings.Trim(title, `"'`)
	if title == "" {
		return subject
	}
	return title
}
