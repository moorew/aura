package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebhookConfig describes a generic HTTP POST target for self-hosted push
// services such as ntfy or Gotify. The body sent is a superset JSON that both
// services understand (ntfy reads topic/title/message; Gotify reads
// title/message/priority), so one shape works for either.
type WebhookConfig struct {
	Endpoint   string `json:"endpoint"`    // full URL to POST to
	Method     string `json:"method"`      // defaults to POST
	AuthHeader string `json:"auth_header"` // e.g. "Authorization" or "X-Api-Key"
	AuthValue  string `json:"auth_value"`  // e.g. "Bearer tk_..." or a token
	Topic      string `json:"topic"`       // ntfy topic (optional)
}

func (c WebhookConfig) configured() bool { return strings.TrimSpace(c.Endpoint) != "" }

type webhookPayload struct {
	Topic    string `json:"topic,omitempty"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority,omitempty"`
	Click    string `json:"click,omitempty"` // ntfy deep-link
	Tags     []string `json:"tags,omitempty"`
}

var webhookClient = &http.Client{Timeout: 10 * time.Second}

// SendWebhook posts a notification to the configured generic webhook endpoint.
func SendWebhook(cfg WebhookConfig, title, body, clickURL string) error {
	if !cfg.configured() {
		return fmt.Errorf("webhook endpoint not configured")
	}
	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}

	payload, _ := json.Marshal(webhookPayload{
		Topic:    cfg.Topic,
		Title:    title,
		Message:  body,
		Priority: 5,
		Click:    clickURL,
		Tags:     []string{"bell"},
	})

	req, err := http.NewRequest(method, cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.AuthHeader != "" && cfg.AuthValue != "" {
		req.Header.Set(cfg.AuthHeader, cfg.AuthValue)
	}

	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webhook %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// TestWebhook sends a canned message to validate the handshake from the
// Settings "Send Test Notification" button.
func TestWebhook(cfg WebhookConfig) error {
	return SendWebhook(cfg, "Sempa test notification",
		"If you can read this, your webhook is wired up correctly. 🎉", "")
}
