package notify

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/clevercode/sempa/internal/db"
)

// settingsType is the integration_configs key under which notification settings
// are stored as a single JSON document.
const settingsType = "notifications"

// Settings is the user-tunable notification configuration. It is persisted as
// JSON in integration_configs(type='notifications') and edited from the
// Notifications settings screen.
type Settings struct {
	MasterEnabled  bool          `json:"master_enabled"`
	WebPushEnabled bool          `json:"webpush_enabled"`
	FCMEnabled     bool          `json:"fcm_enabled"`
	WebhookEnabled bool          `json:"webhook_enabled"`
	SoundEnabled   bool          `json:"sound_enabled"`
	SoundID        string        `json:"sound_id"`
	MorningDigest  bool          `json:"morning_digest"`
	DigestHour     int           `json:"digest_hour"`
	Webhook        WebhookConfig `json:"webhook"`
	Routines       RoutineConfig `json:"routines"`
}

// RoutineConfig drives the in-app scheduled workflows (rendered client-side, not
// as OS notifications). Days are 1=Mon … 7=Sun; times are "HH:MM".
type RoutineConfig struct {
	WeeklyPlanDay     int    `json:"weekly_plan_day"`
	WeeklyPlanTime    string `json:"weekly_plan_time"`
	DailyShutdownTime string `json:"daily_shutdown_time"`
	Workdays          []int  `json:"workdays"`
}

// DefaultSettings is what a fresh install behaves like before the user visits
// the settings screen.
func DefaultSettings() Settings {
	return Settings{
		MasterEnabled:  true,
		WebPushEnabled: true,
		FCMEnabled:     true,
		WebhookEnabled: false,
		SoundEnabled:   true,
		SoundID:        "piano",
		MorningDigest:  true,
		DigestHour:     8,
		Routines: RoutineConfig{
			WeeklyPlanDay:     1, // Monday
			WeeklyPlanTime:    "08:30",
			DailyShutdownTime: "17:00",
			Workdays:          []int{1, 2, 3, 4, 5},
		},
	}
}

// LoadSettings reads stored settings over the defaults. Absent JSON fields keep
// their default, so older/partial documents stay valid.
func LoadSettings(ctx context.Context, store *db.IntegrationConfigStore) Settings {
	s := DefaultSettings()
	cfg, err := store.Get(ctx, settingsType)
	if err == nil && strings.TrimSpace(cfg.Config) != "" {
		_ = json.Unmarshal([]byte(cfg.Config), &s)
	}
	return s
}

// Notification is a single message to fan out across the enabled channels.
type Notification struct {
	Title  string
	Body   string
	URL    string // app-relative deep link, e.g. "/focus/<taskId>"
	TaskID string
	Tag    string // collapse key, e.g. "reminder-<taskId>"
	Type   string // "reminder" | "morning_digest" | ...
}

// pushPayload is the JSON the service worker (sw.js) parses on a 'push' event.
type pushPayload struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	URL     string `json:"url,omitempty"`
	TaskID  string `json:"taskId,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Type    string `json:"type,omitempty"`
	Sound   bool   `json:"sound"`
	SoundID string `json:"soundId,omitempty"`
}

// Dispatcher fans a Notification out to Web Push, FCM, and the generic webhook,
// honoring the master + per-channel toggles in Settings.
type Dispatcher struct {
	configs  *db.IntegrationConfigStore
	pushSubs *db.PushSubStore
	webPush  *WebPushSender
	fcm      *Service
	appURL   string
}

func NewDispatcher(configs *db.IntegrationConfigStore, pushSubs *db.PushSubStore, webPush *WebPushSender, fcm *Service, appURL string) *Dispatcher {
	return &Dispatcher{configs: configs, pushSubs: pushSubs, webPush: webPush, fcm: fcm, appURL: appURL}
}

// Send dispatches n across every enabled channel. Failures are logged, never
// fatal — one dead channel must not block the others.
func (d *Dispatcher) Send(ctx context.Context, n Notification) {
	st := LoadSettings(ctx, d.configs)
	if !st.MasterEnabled {
		return
	}

	// Resolve the chosen sound slug; empty disables custom sound entirely.
	soundID := ""
	if st.SoundEnabled {
		soundID = st.SoundID
		if soundID == "" {
			soundID = "piano"
		}
	}

	if st.WebPushEnabled && d.webPush != nil {
		d.sendWebPush(n, st.SoundEnabled, soundID)
	}
	if st.FCMEnabled && d.fcm != nil && d.fcm.Enabled() {
		data := map[string]string{"url": n.URL, "taskId": n.TaskID, "type": n.Type, "tag": n.Tag}
		d.fcm.SendToAll(n.Title, n.Body, data, soundID)
	}
	if st.WebhookEnabled && st.Webhook.configured() {
		click := ""
		if n.URL != "" {
			click = strings.TrimRight(d.appURL, "/") + n.URL
		}
		if err := SendWebhook(st.Webhook, n.Title, n.Body, click); err != nil {
			slog.Warn("notify: webhook send failed", "err", err)
		}
	}
}

func (d *Dispatcher) sendWebPush(n Notification, sound bool, soundID string) {
	subs, err := d.pushSubs.ListAll()
	if err != nil {
		slog.Error("notify: list push subscriptions", "err", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload, _ := json.Marshal(pushPayload{
		Title:   n.Title,
		Body:    n.Body,
		URL:     n.URL,
		TaskID:  n.TaskID,
		Tag:     n.Tag,
		Type:    n.Type,
		Sound:   sound,
		SoundID: soundID,
	})
	for _, sub := range subs {
		if err := d.webPush.Send(sub, payload, 3600); err != nil {
			if isSubscriptionGone(err) {
				_ = d.pushSubs.DeleteByEndpoint(sub.Endpoint)
				slog.Info("notify: removed expired push subscription", "id", sub.ID)
			} else {
				slog.Warn("notify: web push send failed", "err", err)
			}
		}
	}
}
