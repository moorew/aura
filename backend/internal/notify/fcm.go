package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

// Service handles sending push notifications via FCM HTTP v1 API.
type Service struct {
	devices    *db.DeviceTokenStore
	projectID  string
	keyPath    string
	httpClient *http.Client

	// OAuth2 token cache
	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// New creates a notification service.
// keyPath is the path to the Firebase service account JSON key file.
func New(devices *db.DeviceTokenStore, keyPath string) *Service {
	projectID := extractProjectID(keyPath)
	return &Service{
		devices:    devices,
		projectID:  projectID,
		keyPath:    keyPath,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled returns true if the service is configured.
func (s *Service) Enabled() bool {
	return s.keyPath != "" && s.projectID != ""
}

// SendToAll sends a notification to all registered devices. sound is the chosen
// tone's slug (e.g. "piano"); empty means the default channel with no custom
// sound. The matching res/raw/<sound>.mp3 must be bundled on the Android side.
func (s *Service) SendToAll(title, body string, data map[string]string, sound string) {
	if !s.Enabled() {
		return
	}

	devices, err := s.devices.ListAll()
	if err != nil {
		slog.Error("notify: list devices", "err", err)
		return
	}

	for _, d := range devices {
		if err := s.send(d.Token, title, body, data, sound); err != nil {
			slog.Warn("notify: send failed", "token", d.Token[:12]+"...", "err", err)
			// If the token is invalid, remove it
			if isTokenInvalid(err) {
				_ = s.devices.Delete(d.Token)
				slog.Info("notify: removed invalid token", "id", d.ID)
			}
		}
	}
}

func (s *Service) send(token, title, body string, data map[string]string, sound string) error {
	accessToken, err := s.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	androidNotif := &fcmAndroidNotification{ChannelID: "reminders"}
	if sound != "" {
		// Each sound maps to its own Android channel (sound + importance are
		// immutable once a channel is created), bound to res/raw/<sound>.mp3. The
		// app creates these "rem_<sound>_v2" channels on demand; the version suffix
		// must stay in sync with push.ts / localReminders.ts so a corrected channel
		// replaces the old broken one on existing installs.
		androidNotif.ChannelID = "rem_" + sound + "_v2"
		androidNotif.Sound = sound
	}

	msg := fcmMessage{
		Message: fcmPayload{
			Token: token,
			Notification: &fcmNotification{
				Title: title,
				Body:  body,
			},
			Android: &fcmAndroid{
				Priority:     "high",
				Notification: androidNotif,
			},
			Data: data,
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", s.projectID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FCM %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getAccessToken returns a cached or fresh OAuth2 access token using
// the gcloud CLI as a simple approach (avoids importing the full Google Auth SDK).
// For production, you'd use google.golang.org/api/option, but this keeps deps minimal.
func (s *Service) getAccessToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accessToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.accessToken, nil
	}

	// Use the service account key file with a direct JWT exchange.
	token, expiry, err := serviceAccountToken(s.keyPath)
	if err != nil {
		return "", err
	}

	s.accessToken = token
	s.tokenExpiry = expiry
	return token, nil
}

func extractProjectID(keyPath string) string {
	if keyPath == "" {
		return ""
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ""
	}
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if json.Unmarshal(data, &sa) != nil {
		return ""
	}
	return sa.ProjectID
}

func isTokenInvalid(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return contains(s, "UNREGISTERED") || contains(s, "INVALID_ARGUMENT") || contains(s, "NOT_FOUND")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// FCM HTTP v1 API message types

type fcmMessage struct {
	Message fcmPayload `json:"message"`
}

type fcmPayload struct {
	Token        string            `json:"token"`
	Notification *fcmNotification  `json:"notification,omitempty"`
	Android      *fcmAndroid       `json:"android,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroid struct {
	Priority     string                  `json:"priority,omitempty"`
	Notification *fcmAndroidNotification `json:"notification,omitempty"`
}

type fcmAndroidNotification struct {
	ChannelID string `json:"channel_id,omitempty"`
	Sound     string `json:"sound,omitempty"`
}
