package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultJQL = `assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC`

type transitionsResponse struct {
	Transitions []jiraTransition `json:"transitions"`
}

type jiraTransition struct {
	ID   string           `json:"id"`
	Name string           `json:"name"`
	To   transitionTarget `json:"to"`
}

type transitionTarget struct {
	StatusCategory StatusCategory `json:"statusCategory"`
}

type Config struct {
	Host     string `json:"host"`
	Email    string `json:"email"`
	APIToken string `json:"api_token"`
	JQL      string `json:"jql"`
}

type IssueFields struct {
	Summary   string     `json:"summary"`
	Status    Status     `json:"status"`
	Priority  *Priority  `json:"priority"`
	IssueType IssueType  `json:"issuetype"`
	Assignee  *User      `json:"assignee"`
	Parent    *Parent    `json:"parent"`
}

// Parent carries the epic/parent link (team-managed projects and sub-tasks).
// Company-managed "Epic Link" lives in a per-instance custom field and is not
// covered here; the sidebar filters degrade gracefully when it's absent.
type Parent struct {
	Key    string `json:"key"`
	Fields struct {
		Summary   string    `json:"summary"`
		IssueType IssueType `json:"issuetype"`
	} `json:"fields"`
}

type Status struct {
	Name           string         `json:"name"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

type StatusCategory struct {
	Key string `json:"key"` // "new", "indeterminate", "done"
}

type Priority struct {
	Name string `json:"name"`
}

type IssueType struct {
	Name string `json:"name"`
}

type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

type SearchResult struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken"`
}

// ── Detailed issue view ───────────────────────────────────────────────────────

type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

type Comment struct {
	Author  User   `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

type IssueDetailFields struct {
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    *Priority `json:"priority"`
	IssueType   IssueType `json:"issuetype"`
	Assignee    *User     `json:"assignee"`
	Reporter    *User     `json:"reporter"`
	Labels      []string  `json:"labels"`
	Created     string    `json:"created"`
	Updated     string    `json:"updated"`
	Comments    struct {
		Comments []Comment `json:"comments"`
		Total    int       `json:"total"`
	} `json:"comment"`
}

type IssueDetail struct {
	ID     string            `json:"id"`
	Key    string            `json:"key"`
	Fields IssueDetailFields `json:"fields"`
}

// ── Jira statuses ─────────────────────────────────────────────────────────────

type JiraStatus struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

type Client struct {
	cfg  Config
	auth string
	http *http.Client
}

func NewClient(cfg Config) *Client {
	token := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	return &Client{
		cfg:  cfg,
		auth: "Basic " + token,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Search(ctx context.Context, nextPageToken string, maxResults int) (SearchResult, error) {
	jql := c.cfg.JQL
	if jql == "" {
		jql = DefaultJQL
	}

	reqBody := map[string]any{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     []string{"summary", "status", "priority", "issuetype", "assignee", "parent"},
	}
	if nextPageToken != "" {
		reqBody["nextPageToken"] = nextPageToken
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.Host+"/rest/api/3/search/jql", bytes.NewReader(bodyBytes))
	if err != nil {
		return SearchResult{}, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return SearchResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return SearchResult{}, fmt.Errorf("unauthorized — check your email and API token")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return SearchResult{}, fmt.Errorf("jira returned HTTP %d: %s", resp.StatusCode, body)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SearchResult{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.Search(ctx, "", 1)
	return err
}

// Myself returns the authenticated account, used to flag issues assigned to the
// connected user (accountId comparison is reliable even when emails are hidden).
func (c *Client) Myself(ctx context.Context) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.Host+"/rest/api/2/myself", nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return User{}, fmt.Errorf("jira returned HTTP %d: %s", resp.StatusCode, b)
	}

	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return User{}, fmt.Errorf("decode: %w", err)
	}
	return u, nil
}

// GetIssue fetches full details for a single Jira issue.
func (c *Client) GetIssue(ctx context.Context, issueKey string) (IssueDetail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.Host+"/rest/api/2/issue/"+issueKey+
			"?fields=summary,description,status,priority,issuetype,assignee,reporter,labels,created,updated,comment",
		nil)
	if err != nil {
		return IssueDetail{}, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return IssueDetail{}, fmt.Errorf("jira returned HTTP %d: %s", resp.StatusCode, b)
	}

	var detail IssueDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return IssueDetail{}, fmt.Errorf("decode: %w", err)
	}
	return detail, nil
}

// GetAllStatuses returns all workflow statuses defined in the Jira instance.
func (c *Client) GetAllStatuses(ctx context.Context) ([]JiraStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.Host+"/rest/api/2/status", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira returned HTTP %d: %s", resp.StatusCode, b)
	}

	var statuses []JiraStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return statuses, nil
}

// GetTransitions returns the available workflow transitions for a Jira issue.
func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]jiraTransition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.Host+"/rest/api/2/issue/"+issueKey+"/transitions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch transitions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transitions: HTTP %d: %s", resp.StatusCode, b)
	}

	var tr transitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("decode transitions: %w", err)
	}
	return tr.Transitions, nil
}

// Transition moves a Jira issue to the given transition ID.
func (c *Client) Transition(ctx context.Context, issueKey, transitionID string) error {
	bodyBytes, _ := json.Marshal(map[string]any{
		"transition": map[string]string{"id": transitionID},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.Host+"/rest/api/2/issue/"+issueKey+"/transitions",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post transition: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post transition: HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

// TransitionToDone moves a Jira issue to whichever transition leads to a "done" status category.
// Silently returns nil if no such transition exists (e.g. issue is already done).
func (c *Client) TransitionToDone(ctx context.Context, issueKey string) error {
	transitions, err := c.GetTransitions(ctx, issueKey)
	if err != nil {
		return err
	}

	var doneID string
	for _, t := range transitions {
		if t.To.StatusCategory.Key == "done" {
			doneID = t.ID
			break
		}
	}
	if doneID == "" {
		return nil // no "done" transition available, skip silently
	}
	return c.Transition(ctx, issueKey, doneID)
}
