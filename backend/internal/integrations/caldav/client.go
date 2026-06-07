// Package caldav implements a minimal CalDAV client: principal/calendar
// discovery (RFC 4791 §6, RFC 5397) plus event read (calendar-query REPORT)
// and write (PUT/DELETE). It is provider-agnostic but currently used with
// Fastmail (https://caldav.fastmail.com) via Basic auth + app password.
package caldav

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FastmailBaseURL is the CalDAV entry point for Fastmail accounts.
const FastmailBaseURL = "https://caldav.fastmail.com"

// discoveryPath is where principal discovery begins on the base server.
const discoveryPath = "/dav/"

// Config holds the connection details for a CalDAV server.
type Config struct {
	BaseURL  string // e.g. https://caldav.fastmail.com
	Username string // account email
	Password string // app password (whitespace is stripped automatically)
}

// Calendar is a writable calendar collection discovered on the server.
type Calendar struct {
	Href  string `json:"href"`  // server-absolute path, e.g. /dav/calendars/user/x/abc/
	Name  string `json:"name"`  // display name
	Color string `json:"color"` // hex colour, may be empty
}

// Client talks to a single CalDAV server.
type Client struct {
	cfg  Config
	auth string
	base *url.URL
	http *http.Client
}

// NewClient builds a CalDAV client. The app password is sanitized (Fastmail
// shows app passwords in space-separated groups; whitespace is never part of
// the credential).
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = FastmailBaseURL
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("caldav: invalid base URL: %w", err)
	}
	pw := strings.Join(strings.Fields(cfg.Password), "")
	token := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + pw))
	return &Client{
		cfg:  cfg,
		auth: "Basic " + token,
		base: base,
		http: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ── XML response shapes (RFC 4918 multistatus) ────────────────────────────────

type msMultistatus struct {
	XMLName   xml.Name     `xml:"DAV: multistatus"`
	Responses []msResponse `xml:"DAV: response"`
}

type msResponse struct {
	Href      string       `xml:"DAV: href"`
	Propstats []msPropstat `xml:"DAV: propstat"`
}

type msPropstat struct {
	Status string `xml:"DAV: status"`
	Prop   msProp `xml:"DAV: prop"`
}

type msProp struct {
	DisplayName          string           `xml:"DAV: displayname"`
	ResourceType         msResourceType   `xml:"DAV: resourcetype"`
	GetETag              string           `xml:"DAV: getetag"`
	CurrentUserPrincipal msHref           `xml:"DAV: current-user-principal"`
	CalendarHomeSet      msHref           `xml:"urn:ietf:params:xml:ns:caldav calendar-home-set"`
	CalendarColor        string           `xml:"http://apple.com/ns/ical/ calendar-color"`
	SupportedComps       msSupportedComps `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
	CalendarData         string           `xml:"urn:ietf:params:xml:ns:caldav calendar-data"`
}

type msHref struct {
	Href string `xml:"DAV: href"`
}

type msResourceType struct {
	Calendar   *struct{} `xml:"urn:ietf:params:xml:ns:caldav calendar"`
	Collection *struct{} `xml:"DAV: collection"`
}

type msSupportedComps struct {
	Comps []struct {
		Name string `xml:"name,attr"`
	} `xml:"urn:ietf:params:xml:ns:caldav comp"`
}

// ── HTTP plumbing ─────────────────────────────────────────────────────────────

// resolve turns a server href (absolute path or full URL) into a full URL.
func (c *Client) resolve(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return c.base.String() + href
	}
	return c.base.ResolveReference(u).String()
}

func (c *Client) do(ctx context.Context, method, fullURL string, depth string, body string, contentType string) (*http.Response, error) {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	if depth != "" {
		req.Header.Set("Depth", depth)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(req)
}

func (c *Client) propfind(ctx context.Context, fullURL, depth, body string) (*msMultistatus, error) {
	resp, err := c.do(ctx, "PROPFIND", fullURL, depth, body, `application/xml; charset="utf-8"`)
	if err != nil {
		return nil, fmt.Errorf("PROPFIND %s: %w", fullURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("CalDAV auth failed (401) — check email and app password")
	}
	// 207 Multi-Status is the success code for PROPFIND.
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("PROPFIND %s: HTTP %d: %s", fullURL, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var ms msMultistatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("PROPFIND %s: decode: %w", fullURL, err)
	}
	return &ms, nil
}

// ── Discovery ─────────────────────────────────────────────────────────────────

const propPrincipal = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`

const propHomeSet = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-home-set/></d:prop></d:propfind>`

const propCalendars = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:ic="http://apple.com/ns/ical/">
  <d:prop>
    <d:displayname/>
    <d:resourcetype/>
    <c:supported-calendar-component-set/>
    <ic:calendar-color/>
  </d:prop>
</d:propfind>`

// firstProp returns the first propstat prop from a response (servers may split
// props across propstats by status; we only request props that should succeed).
func firstProp(r msResponse) msProp {
	for _, ps := range r.Propstats {
		if strings.Contains(ps.Status, "200") {
			return ps.Prop
		}
	}
	if len(r.Propstats) > 0 {
		return r.Propstats[0].Prop
	}
	return msProp{}
}

// calendarHome discovers the calendar-home-set collection for the account.
func (c *Client) calendarHome(ctx context.Context) (string, error) {
	// 1. current-user-principal
	ms, err := c.propfind(ctx, c.resolve(discoveryPath), "0", propPrincipal)
	if err != nil {
		return "", err
	}
	principal := ""
	for _, r := range ms.Responses {
		if p := firstProp(r).CurrentUserPrincipal.Href; p != "" {
			principal = p
			break
		}
	}
	if principal == "" {
		return "", fmt.Errorf("CalDAV: no current-user-principal returned by server")
	}

	// 2. calendar-home-set on the principal
	ms, err = c.propfind(ctx, c.resolve(principal), "0", propHomeSet)
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		if h := firstProp(r).CalendarHomeSet.Href; h != "" {
			return h, nil
		}
	}
	return "", fmt.Errorf("CalDAV: no calendar-home-set returned by server")
}

// ListCalendars discovers all writable VEVENT calendars on the account.
func (c *Client) ListCalendars(ctx context.Context) ([]Calendar, error) {
	home, err := c.calendarHome(ctx)
	if err != nil {
		return nil, err
	}
	ms, err := c.propfind(ctx, c.resolve(home), "1", propCalendars)
	if err != nil {
		return nil, err
	}
	var cals []Calendar
	for _, r := range ms.Responses {
		prop := firstProp(r)
		// Must be a calendar collection.
		if prop.ResourceType.Calendar == nil {
			continue
		}
		// Must support VEVENT (skip task-only / contact collections).
		if !supportsVEVENT(prop.SupportedComps) {
			continue
		}
		name := prop.DisplayName
		if name == "" {
			name = "Calendar"
		}
		cals = append(cals, Calendar{
			Href:  r.Href,
			Name:  name,
			Color: prop.CalendarColor,
		})
	}
	return cals, nil
}

func supportsVEVENT(s msSupportedComps) bool {
	if len(s.Comps) == 0 {
		return true // server didn't report; assume yes
	}
	for _, comp := range s.Comps {
		if strings.EqualFold(comp.Name, "VEVENT") {
			return true
		}
	}
	return false
}

// ── Event write ───────────────────────────────────────────────────────────────

// eventURL builds the .ics resource URL for a given UID within a calendar.
func (c *Client) eventURL(calendarHref, uid string) string {
	href := strings.TrimRight(calendarHref, "/") + "/" + uid + ".ics"
	return c.resolve(href)
}

// PutEvent creates or replaces the event identified by uid in calendarHref.
// It is idempotent: re-PUTting the same UID updates the existing event.
func (c *Client) PutEvent(ctx context.Context, calendarHref, uid, ics string) error {
	resp, err := c.do(ctx, http.MethodPut, c.eventURL(calendarHref, uid), "", ics, "text/calendar; charset=utf-8")
	if err != nil {
		return fmt.Errorf("PUT event: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
	// 201 Created or 204 No Content (update) are both success.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PUT event: HTTP %d", resp.StatusCode)
	}
	return nil
}

// DeleteEvent removes the event identified by uid. A 404 is treated as success
// (the event is already gone).
func (c *Client) DeleteEvent(ctx context.Context, calendarHref, uid string) error {
	resp, err := c.do(ctx, http.MethodDelete, c.eventURL(calendarHref, uid), "", "", "")
	if err != nil {
		return fmt.Errorf("DELETE event: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("DELETE event: HTTP %d", resp.StatusCode)
	}
	return nil
}

// ── Event read (calendar-query REPORT) ────────────────────────────────────────

// RawEvent is one VEVENT resource returned by a calendar-query.
type RawEvent struct {
	Href string
	ICS  string // the VCALENDAR body
}

// QueryEvents fetches all events overlapping [start, end) in calendarHref.
// start/end are formatted as iCal UTC timestamps (YYYYMMDDTHHMMSSZ).
func (c *Client) QueryEvents(ctx context.Context, calendarHref, start, end string) ([]RawEvent, error) {
	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop><d:getetag/><c:calendar-data/></d:prop>
  <c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT">
    <c:time-range start="%s" end="%s"/>
  </c:comp-filter></c:comp-filter></c:filter>
</c:calendar-query>`, start, end)

	resp, err := c.do(ctx, "REPORT", c.resolve(calendarHref), "1", body, `application/xml; charset="utf-8"`)
	if err != nil {
		return nil, fmt.Errorf("REPORT: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("REPORT: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var ms msMultistatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("REPORT decode: %w", err)
	}
	var out []RawEvent
	for _, r := range ms.Responses {
		if data := firstProp(r).CalendarData; data != "" {
			out = append(out, RawEvent{Href: r.Href, ICS: data})
		}
	}
	return out, nil
}

// TestConnection verifies credentials by discovering the calendar home.
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.calendarHome(ctx)
	return err
}
