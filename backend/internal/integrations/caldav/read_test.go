package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockCalDAV serves the minimal PROPFIND/REPORT responses ReadCalendarEvents
// needs: principal → home-set → one calendar → two events (one external, one
// Sempa task block that must be filtered out).
func mockCalDAV(t *testing.T) *httptest.Server {
	t.Helper()
	const principal = `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">
<d:response><d:href>/dav/</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status>
<d:prop><d:current-user-principal><d:href>/dav/principals/user/me/</d:href></d:current-user-principal></d:prop>
</d:propstat></d:response></d:multistatus>`

	const home = `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
<d:response><d:href>/dav/principals/user/me/</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status>
<d:prop><c:calendar-home-set><d:href>/dav/calendars/user/me/</d:href></c:calendar-home-set></d:prop>
</d:propstat></d:response></d:multistatus>`

	const calendars = `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:ic="http://apple.com/ns/ical/">
<d:response><d:href>/dav/calendars/user/me/work/</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status>
<d:prop><d:displayname>Work</d:displayname><d:resourcetype><d:collection/><c:calendar/></d:resourcetype>
<ic:calendar-color>#ff0000</ic:calendar-color>
<c:supported-calendar-component-set><c:comp name="VEVENT"/></c:supported-calendar-component-set></d:prop>
</d:propstat></d:response></d:multistatus>`

	const events = `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
<d:response><d:href>/dav/calendars/user/me/work/ext.ics</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status>
<d:prop><c:calendar-data>BEGIN:VCALENDAR
BEGIN:VEVENT
UID:external-123
SUMMARY:Team standup
DTSTART:20260610T140000Z
DTEND:20260610T143000Z
END:VEVENT
END:VCALENDAR</c:calendar-data></d:prop></d:propstat></d:response>
<d:response><d:href>/dav/calendars/user/me/work/task.ics</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status>
<d:prop><c:calendar-data>BEGIN:VCALENDAR
BEGIN:VEVENT
UID:sempa-task-abc
SUMMARY:My task block
DTSTART:20260610T160000Z
DTEND:20260610T170000Z
END:VEVENT
END:VCALENDAR</c:calendar-data></d:prop></d:propstat></d:response>
</d:multistatus>`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(body)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		switch {
		case r.Method == "REPORT":
			_, _ = w.Write([]byte(events))
		case strings.Contains(string(body), "current-user-principal"):
			_, _ = w.Write([]byte(principal))
		case strings.Contains(string(body), "calendar-home-set"):
			_, _ = w.Write([]byte(home))
		default:
			_, _ = w.Write([]byte(calendars))
		}
	}))
}

func TestReadCalendarEvents(t *testing.T) {
	srv := mockCalDAV(t)
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL, Username: "me@example.com", Password: "app pass word"})
	if err != nil {
		t.Fatal(err)
	}

	evs, err := ReadCalendarEvents(context.Background(), c, "2026-06-08", "2026-06-14")
	if err != nil {
		t.Fatalf("ReadCalendarEvents: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 external event (task block filtered), got %d: %+v", len(evs), evs)
	}
	got := evs[0]
	if got.UID != "external-123" {
		t.Errorf("UID = %q, want external-123", got.UID)
	}
	if got.Summary != "Team standup" {
		t.Errorf("Summary = %q, want Team standup", got.Summary)
	}
	if got.Color != "#ff0000" {
		t.Errorf("Color = %q, want #ff0000 from parent calendar", got.Color)
	}
}
