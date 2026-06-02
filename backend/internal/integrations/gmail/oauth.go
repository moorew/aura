package gmail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	authURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL = "https://oauth2.googleapis.com/token"

	scopeGmail           = "https://www.googleapis.com/auth/gmail.readonly"
	scopeCalendarRead    = "https://www.googleapis.com/auth/calendar.readonly"
	scopeCalendarEvents  = "https://www.googleapis.com/auth/calendar.events"
	scopeCalendarFreeBusy = "https://www.googleapis.com/auth/calendar.events.freebusy"

	// Full calendar scope set (read + write events + free/busy)
	scopeCalendarFull = scopeCalendarRead + " " + scopeCalendarEvents + " " + scopeCalendarFreeBusy
)

type StoredToken struct {
	Email           string   `json:"email"`
	AccessToken     string   `json:"access_token"`
	RefreshToken    string   `json:"refresh_token"`
	Expiry          string   `json:"expiry"`
	Labels          []string `json:"labels"`
	CalendarEnabled bool     `json:"calendar_enabled"`
	CalendarIDs     []string `json:"calendar_ids"`
}

func DefaultLabels() []string { return []string{"STARRED"} }

var (
	stateMu  sync.Mutex
	stateMap = map[string]time.Time{}
)

func GenerateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	stateMu.Lock()
	stateMap[state] = time.Now().Add(10 * time.Minute)
	stateMu.Unlock()
	return state
}

func ConsumeState(state string) bool {
	stateMu.Lock()
	defer stateMu.Unlock()
	exp, ok := stateMap[state]
	if !ok { return false }
	delete(stateMap, state)
	return time.Now().Before(exp)
}

func AuthURL(clientID, redirectURI, state string, includeCalendar bool) string {
	scopes := scopeGmail
	if includeCalendar {
		scopes = scopeGmail + " " + scopeCalendarFull
	}
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scopes)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("state", state)
	return authURL + "?" + params.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func ExchangeCode(ctx context.Context, clientID, clientSecret, redirectURI, code string) (StoredToken, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil { return StoredToken{}, err }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return StoredToken{}, fmt.Errorf("token exchange: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return StoredToken{}, fmt.Errorf("token exchange returned HTTP %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return StoredToken{}, fmt.Errorf("decode token: %w", err)
	}
	expiry := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return StoredToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		Expiry:       expiry,
		Labels:       DefaultLabels(),
	}, nil
}

func RefreshAccessToken(ctx context.Context, clientID, clientSecret string, stored *StoredToken) error {
	if stored.RefreshToken == "" { return fmt.Errorf("no refresh token stored") }
	if stored.Expiry != "" {
		exp, err := time.Parse(time.RFC3339, stored.Expiry)
		if err == nil && time.Now().Add(60*time.Second).Before(exp) { return nil }
	}
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", stored.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return fmt.Errorf("refresh token: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh returned HTTP %d", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil { return err }
	stored.AccessToken = tr.AccessToken
	stored.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return nil
}

func FetchEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://gmail.googleapis.com/gmail/v1/users/me/profile", nil)
	if err != nil { return "", err }
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", fmt.Errorf("fetch profile: %w", err) }
	defer resp.Body.Close()
	var profile struct { EmailAddress string `json:"emailAddress"` }
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil { return "", err }
	return profile.EmailAddress, nil
}
