package fastmail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

const sessionURL = "https://api.fastmail.com/.well-known/jmap"

type Config struct {
	Email        string `json:"email"`
	AppPassword  string `json:"app_password"`
	InboxAddress string `json:"inbox_address,omitempty"` // e.g. tasks@sempa.ca
}

type Client struct {
	cfg        Config
	auth       string
	apiURL     string
	account    string // mail account ID
	calAccount string // calendar account ID
	username   string // discovered from JMAP session
	http       *http.Client
}

type jmapSession struct {
	APIURL          string            `json:"apiUrl"`
	PrimaryAccounts map[string]string `json:"primaryAccounts"`
	Username        string            `json:"username"`
}

type jmapRequest struct {
	Using       []string        `json:"using"`
	MethodCalls [][]interface{} `json:"methodCalls"`
}

type jmapResponse struct {
	MethodResponses [][]interface{} `json:"methodResponses"`
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		auth: "Bearer " + cfg.AppPassword,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Username returns the account email discovered from the JMAP session.
// Only populated after Discover() has been called.
func (c *Client) Username() string { return c.username }

func (c *Client) Discover(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("JMAP discover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = "no detail from server"
		}
		return fmt.Errorf("401 unauthorized (email: %s) — %s", c.cfg.Email, detail)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("JMAP session HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var session jmapSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("decode JMAP session: %w", err)
	}
	c.apiURL = session.APIURL
	if c.apiURL == "" {
		c.apiURL = "https://api.fastmail.com/jmap/api/"
	}
	c.account    = session.PrimaryAccounts["urn:ietf:params:jmap:mail"]
	c.calAccount = session.PrimaryAccounts["urn:ietf:params:jmap:calendars"]
	c.username   = session.Username
	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	return c.Discover(ctx)
}

type Email struct {
	ID          string                `json:"id"`
	Subject     string                `json:"subject"`
	From        []EmailAddress        `json:"from"`
	ReceivedAt  string                `json:"receivedAt"`
	TextBody    []BodyPart            `json:"textBody"`
	BodyValues  map[string]BodyValue  `json:"bodyValues"`
}

type EmailAddress struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BodyPart struct {
	PartID string `json:"partId"`
}

type BodyValue struct {
	Value string `json:"value"`
}

func (c *Client) GetFlaggedEmails(ctx context.Context) ([]Email, error) {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return nil, err
		}
	}

	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/query", map[string]interface{}{
				"accountId": c.account,
				"filter":    map[string]interface{}{"hasKeyword": "$flagged"},
				"limit":     100,
				"sort":      []map[string]interface{}{{"property": "receivedAt", "isAscending": false}},
			}, "0"},
			{"Email/get", map[string]interface{}{
				"accountId": c.account,
				"#ids": map[string]interface{}{
					"resultOf": "0",
					"name":     "Email/query",
					"path":     "/ids/*",
				},
				"properties": []string{"id", "subject", "from", "receivedAt"},
			}, "1"},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JMAP request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JMAP returned HTTP %d", resp.StatusCode)
	}

	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}

	// Extract emails from Email/get response (second method call result)
	for _, mc := range jr.MethodResponses {
		if len(mc) < 2 { continue }
		name, ok := mc[0].(string)
		if !ok || name != "Email/get" { continue }

		data, _ := json.Marshal(mc[1])
		var result struct {
			List []Email `json:"list"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result.List, nil
	}
	return nil, nil
}

// Sync fetches starred/flagged Fastmail emails and creates tasks via IMAP.
func Sync(ctx context.Context, cfg Config, tasks *db.TaskStore) (db.SyncResult, error) {
	emails, err := GetIMAPFlaggedEmails(cfg.Email, cfg.AppPassword)
	if err != nil {
		return db.SyncResult{}, err
	}
	var result db.SyncResult
	for _, em := range emails {
		if err := syncEmail(ctx, em, cfg.Email, tasks, &result); err != nil {
			result.Errors++
		}
	}
	return result, nil
}

func syncEmail(ctx context.Context, em Email, accountEmail string, tasks *db.TaskStore, result *db.SyncResult) error {
	result.Total++

	_, err := tasks.FindBySource(ctx, "fastmail", em.ID)
	if err == nil {
		return nil // already imported
	}
	if !errors.Is(err, db.ErrNotFound) {
		return err
	}

	subject := em.Subject
	if subject == "" {
		subject = "(no subject)"
	}

	fromStr := ""
	if len(em.From) > 0 {
		f := em.From[0]
		if f.Name != "" {
			fromStr = f.Name + " <" + f.Email + ">"
		} else {
			fromStr = f.Email
		}
	}

	meta, _ := json.Marshal(map[string]string{
		"from": fromStr,
		"date": em.ReceivedAt,
	})
	metaStr := string(meta)
	source := "fastmail"
	sourceURL := "https://app.fastmail.com/mail/"

	_, createErr := tasks.Create(ctx, db.CreateTaskParams{
		ID:             uuid.New().String(),
		Title:          subject,
		Status:         "backlog",
		Position:       float64(time.Now().UnixMilli()),
		Source:         &source,
		SourceID:       &em.ID,
		SourceURL:      &sourceURL,
		SourceMetadata: &metaStr,
	})
	if createErr != nil {
		return createErr
	}
	result.New++
	return nil
}

// GetEmailsTo fetches unread emails sent to the given address (e.g. tasks@sempa.ca).
func (c *Client) GetEmailsTo(ctx context.Context, toAddress string) ([]Email, error) {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return nil, err
		}
	}

	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/query", map[string]interface{}{
				"accountId": c.account,
				"filter": map[string]interface{}{
					"operator": "AND",
					"conditions": []interface{}{
						map[string]interface{}{"to": toAddress},
						map[string]interface{}{"notKeyword": "$seen"},
					},
				},
				"limit": 50,
				"sort": []map[string]interface{}{{"property": "receivedAt", "isAscending": true}},
			}, "0"},
			{"Email/get", map[string]interface{}{
				"accountId": c.account,
				"#ids": map[string]interface{}{
					"resultOf": "0",
					"name":     "Email/query",
					"path":     "/ids/*",
				},
				"properties":           []string{"id", "subject", "from", "receivedAt", "textBody", "bodyValues"},
				"fetchTextBodyValues":   true,
				"maxBodyValueBytes":     10240,
			}, "1"},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JMAP request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JMAP returned HTTP %d", resp.StatusCode)
	}

	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}
	for _, mc := range jr.MethodResponses {
		if len(mc) < 2 {
			continue
		}
		if name, _ := mc[0].(string); name != "Email/get" {
			continue
		}
		data, _ := json.Marshal(mc[1])
		var result struct {
			List []Email `json:"list"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result.List, nil
	}
	return nil, nil
}

// MarkRead marks the given email IDs as read ($seen).
func (c *Client) MarkRead(ctx context.Context, emailIDs []string) error {
	if len(emailIDs) == 0 {
		return nil
	}
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return err
		}
	}

	update := make(map[string]interface{}, len(emailIDs))
	for _, id := range emailIDs {
		update[id] = map[string]interface{}{"keywords/$seen": true}
	}

	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/set", map[string]interface{}{
				"accountId": c.account,
				"update":    update,
			}, "0"},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("JMAP mark-read: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JMAP mark-read returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// SyncInbox is kept for backward compat; delegates to SyncIMAPTaskInbox.
func SyncInbox(ctx context.Context, cfg Config, tasks *db.TaskStore) (db.SyncResult, error) {
	if cfg.InboxAddress == "" {
		return db.SyncResult{}, fmt.Errorf("no inbox address configured")
	}
	client := NewClient(cfg)
	emails, err := client.GetEmailsTo(ctx, cfg.InboxAddress)
	if err != nil {
		return db.SyncResult{}, err
	}

	var result db.SyncResult
	var readIDs []string

	today := time.Now().Format("2006-01-02")
	ws := mondayOf(today)

	for _, em := range emails {
		result.Total++

		// Idempotency: skip if already imported.
		if _, err := tasks.FindBySource(ctx, "fastmail", "inbox_"+em.ID); err == nil {
			readIDs = append(readIDs, em.ID)
			continue
		}

		subject := em.Subject
		if subject == "" {
			subject = "(no subject)"
		}

		// Extract plain text body.
		var desc *string
		if len(em.TextBody) > 0 && em.BodyValues != nil {
			if bv, ok := em.BodyValues[em.TextBody[0].PartID]; ok && bv.Value != "" {
				v := strings.TrimSpace(bv.Value)
				if len(v) > 4000 {
					v = v[:4000] + "…"
				}
				desc = &v
			}
		}

		source := "fastmail"
		sourceID := "inbox_" + em.ID
		sourceURL := "https://app.fastmail.com/mail/"

		_, createErr := tasks.Create(ctx, db.CreateTaskParams{
			ID:          uuid.New().String(),
			Title:       subject,
			Description: desc,
			Status:      "planned",
			PlannedDate: &today,
			WeekStart:   &ws,
			Position:    float64(time.Now().UnixMilli()),
			Source:      &source,
			SourceID:    &sourceID,
			SourceURL:   &sourceURL,
			Tags:        []string{},
		})
		if createErr != nil {
			result.Errors++
		} else {
			result.New++
			readIDs = append(readIDs, em.ID)
		}
	}

	// Mark processed emails as read so they don't reappear.
	if err := client.MarkRead(ctx, readIDs); err != nil {
		// Non-fatal: tasks were created, just log the mark-read failure.
		_ = err
	}

	return result, nil
}

// mondayOf returns the ISO week Monday for a YYYY-MM-DD date string.
func mondayOf(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return t.AddDate(0, 0, -(wd - 1)).Format("2006-01-02")
}

// ── Task inbox (standalone forwarding integration) ─────────────────────────

// InboxConfig holds credentials and settings for the standalone email inbox feature.
type InboxConfig struct {
	Email            string   `json:"email"`
	AppPassword      string   `json:"app_password"`
	InboxAddress     string   `json:"inbox_address"`
	AllowedSenders   []string `json:"allowed_senders,omitempty"`
	AnthropicAPIKey  string   `json:"anthropic_api_key,omitempty"` // injected at runtime, not stored
}

// SyncTaskInbox fetches unread emails to InboxAddress, filters by AllowedSenders,
// creates planned tasks, and marks emails as read. Uses IMAP.
func SyncTaskInbox(ctx context.Context, cfg InboxConfig, tasks *db.TaskStore) (db.SyncResult, error) {
	if cfg.InboxAddress == "" {
		return db.SyncResult{}, fmt.Errorf("no inbox address configured")
	}
	return SyncIMAPTaskInbox(ctx, cfg, tasks)
}

// syncTaskInboxJMAP is the old JMAP-based implementation, kept for reference.
// nolint:unused
func syncTaskInboxJMAPLegacy(ctx context.Context, cfg InboxConfig, tasks *db.TaskStore) (db.SyncResult, error) {
	fmCfg := Config{Email: cfg.Email, AppPassword: cfg.AppPassword}
	client := NewClient(fmCfg)

	emails, err := client.GetEmailsTo(ctx, cfg.InboxAddress)
	if err != nil {
		return db.SyncResult{}, err
	}

	var result db.SyncResult
	var readIDs []string
	today := time.Now().Format("2006-01-02")
	ws := mondayOf(today)

	for _, em := range emails {
		result.Total++

		if !senderAllowed(em.From, cfg.AllowedSenders) {
			readIDs = append(readIDs, em.ID) // mark read so it doesn't linger
			continue
		}

		sourceID := "taskinbox_" + em.ID
		if _, err := tasks.FindBySource(ctx, "fastmail", sourceID); err == nil {
			readIDs = append(readIDs, em.ID)
			continue
		}

		subject := em.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		var desc *string
		if len(em.TextBody) > 0 && em.BodyValues != nil {
			if bv, ok := em.BodyValues[em.TextBody[0].PartID]; ok && bv.Value != "" {
				v := strings.TrimSpace(bv.Value)
				if len(v) > 4000 {
					v = v[:4000] + "…"
				}
				desc = &v
			}
		}

		source := "fastmail"
		srcURL := "https://app.fastmail.com/mail/"

		if _, createErr := tasks.Create(ctx, db.CreateTaskParams{
			ID:          uuid.New().String(),
			Title:       subject,
			Description: desc,
			Status:      "planned",
			PlannedDate: &today,
			WeekStart:   &ws,
			Position:    float64(time.Now().UnixMilli()),
			Source:      &source,
			SourceID:    &sourceID,
			SourceURL:   &srcURL,
			Tags:        []string{},
		}); createErr != nil {
			result.Errors++
		} else {
			result.New++
			readIDs = append(readIDs, em.ID)
		}
	}

	_ = client.MarkRead(ctx, readIDs)
	return result, nil
}

// senderAllowed returns true when the email's From matches an allowed sender.
// If allowed is empty, all senders are permitted.
func senderAllowed(from []EmailAddress, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, addr := range from {
		email := strings.ToLower(strings.TrimSpace(addr.Email))
		for _, a := range allowed {
			a = strings.ToLower(strings.TrimSpace(a))
			if strings.HasPrefix(a, "@") {
				if strings.HasSuffix(email, a) {
					return true
				}
			} else if email == a {
				return true
			}
		}
	}
	return false
}

// ── Inbox panel (email view + convert-to-task) ─────────────────────────────

type Mailbox struct {
	ID   string  `json:"id"`
	Role *string `json:"role"`
}

type PanelEmail struct {
	ID         string         `json:"id"`
	Subject    string         `json:"subject"`
	From       []EmailAddress `json:"from"`
	ReceivedAt string         `json:"receivedAt"`
	Preview    string         `json:"preview"`
	Keywords   map[string]bool `json:"keywords"`
}

func (e PanelEmail) IsUnread() bool { return !e.Keywords["$seen"] }

// GetMailboxRoles returns the JMAP IDs for the inbox and archive mailboxes.
func (c *Client) GetMailboxRoles(ctx context.Context) (inboxID, archiveID string, err error) {
	if c.apiURL == "" {
		if err = c.Discover(ctx); err != nil {
			return
		}
	}
	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Mailbox/get", map[string]interface{}{
				"accountId":  c.account,
				"ids":        nil,
				"properties": []string{"id", "role"},
			}, "0"},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("GetMailboxRoles: %w", err)
	}
	defer resp.Body.Close()
	var jr jmapResponse
	if err = json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return
	}
	for _, mc := range jr.MethodResponses {
		if name, _ := mc[0].(string); name != "Mailbox/get" {
			continue
		}
		data, _ := json.Marshal(mc[1])
		var result struct{ List []Mailbox `json:"list"` }
		if err = json.Unmarshal(data, &result); err != nil {
			return
		}
		for _, m := range result.List {
			if m.Role == nil {
				continue
			}
			switch *m.Role {
			case "inbox":
				inboxID = m.ID
			case "archive":
				archiveID = m.ID
			}
		}
		return
	}
	return "", "", fmt.Errorf("no mailbox data in response")
}

// GetInboxEmails returns recent emails from the inbox mailbox.
func (c *Client) GetInboxEmails(ctx context.Context, inboxID string, limit int) ([]PanelEmail, error) {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return nil, err
		}
	}
	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/query", map[string]interface{}{
				"accountId": c.account,
				"filter":    map[string]interface{}{"inMailbox": inboxID},
				"sort":      []map[string]interface{}{{"property": "receivedAt", "isAscending": false}},
				"limit":     limit,
			}, "0"},
			{"Email/get", map[string]interface{}{
				"accountId": c.account,
				"#ids": map[string]interface{}{
					"resultOf": "0", "name": "Email/query", "path": "/ids/*",
				},
				"properties": []string{"id", "subject", "from", "receivedAt", "preview", "keywords"},
			}, "1"},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetInboxEmails: %w", err)
	}
	defer resp.Body.Close()
	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}
	for _, mc := range jr.MethodResponses {
		if name, _ := mc[0].(string); name != "Email/get" {
			continue
		}
		data, _ := json.Marshal(mc[1])
		var result struct{ List []PanelEmail `json:"list"` }
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result.List, nil
	}
	return nil, nil
}

// GetEmailBody fetches the plain-text body of a single email.
func (c *Client) GetEmailBody(ctx context.Context, emailID string) (string, error) {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return "", err
		}
	}
	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/get", map[string]interface{}{
				"accountId":           c.account,
				"ids":                 []string{emailID},
				"properties":          []string{"id", "textBody", "bodyValues"},
				"fetchTextBodyValues": true,
				"maxBodyValueBytes":   8192,
			}, "0"},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return "", err
	}
	for _, mc := range jr.MethodResponses {
		if name, _ := mc[0].(string); name != "Email/get" {
			continue
		}
		data, _ := json.Marshal(mc[1])
		var result struct {
			List []Email `json:"list"`
		}
		if err := json.Unmarshal(data, &result); err != nil || len(result.List) == 0 {
			return "", err
		}
		em := result.List[0]
		if len(em.TextBody) > 0 && em.BodyValues != nil {
			if bv, ok := em.BodyValues[em.TextBody[0].PartID]; ok {
				return strings.TrimSpace(bv.Value), nil
			}
		}
		return "", nil
	}
	return "", nil
}

// ArchiveEmail moves an email from the inbox to the archive mailbox.
func (c *Client) ArchiveEmail(ctx context.Context, emailID, inboxID, archiveID string) error {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return err
		}
	}
	update := map[string]interface{}{
		"mailboxIds/" + inboxID: nil,
	}
	if archiveID != "" {
		update["mailboxIds/"+archiveID] = true
	}
	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/set", map[string]interface{}{
				"accountId": c.account,
				"update":    map[string]interface{}{emailID: update},
			}, "0"},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ArchiveEmail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ArchiveEmail: HTTP %d", resp.StatusCode)
	}
	return nil
}
