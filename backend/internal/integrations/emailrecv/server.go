package emailrecv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/google/uuid"

	"github.com/clevercode/aura/internal/db"
)

type Server struct {
	smtp  *gosmtp.Server
	tasks *db.TaskStore
}

func New(addr string, tasks *db.TaskStore) *Server {
	s := &Server{tasks: tasks}
	srv := gosmtp.NewServer(s)
	srv.Addr = addr
	srv.Domain = "aura"
	srv.EnableSMTPUTF8 = true
	s.smtp = srv
	return s
}

func (s *Server) ListenAndServe() error { return s.smtp.ListenAndServe() }
func (s *Server) Close() error          { return s.smtp.Close() }

func (s *Server) NewSession(_ *gosmtp.Conn) (gosmtp.Session, error) {
	return &session{srv: s}, nil
}

type session struct {
	srv *Server
}

func (s *session) Mail(_ string, _ *gosmtp.MailOptions) error { return nil }
func (s *session) Rcpt(_ string, _ *gosmtp.RcptOptions) error { return nil }
func (s *session) Reset()                                      {}
func (s *session) Logout() error                               { return nil }

func (s *session) Data(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		slog.Warn("emailrecv: failed to parse message", "err", err)
		return nil
	}

	subject := decodeHeader(msg.Header.Get("Subject"))
	if subject == "" {
		subject = "(no subject)"
	}

	body := extractText(msg)

	// Use Message-ID for idempotency; fall back to hash of raw bytes.
	msgID := strings.Trim(msg.Header.Get("Message-Id"), "<> \t")
	if msgID == "" {
		h := sha256.Sum256(raw)
		msgID = fmt.Sprintf("%x", h[:8])
	}

	// Avoid duplicate imports.
	if _, err := s.srv.tasks.FindBySource(context.Background(), "email_forward", msgID); err == nil {
		slog.Info("emailrecv: duplicate message, skipping", "message_id", msgID)
		return nil
	}

	today := time.Now().Format("2006-01-02")
	ws := mondayOf(today)
	src := "email_forward"
	pos := float64(time.Now().UnixMilli())

	var desc *string
	if body != "" {
		d := truncate(body, 4000)
		desc = &d
	}

	_, err = s.srv.tasks.Create(context.Background(), db.CreateTaskParams{
		ID:          uuid.New().String(),
		Title:       subject,
		Description: desc,
		Status:      "planned",
		PlannedDate: &today,
		WeekStart:   &ws,
		Position:    pos,
		Source:      &src,
		SourceID:    &msgID,
		Tags:        []string{},
	})
	if err != nil {
		slog.Error("emailrecv: create task", "err", err)
	} else {
		slog.Info("emailrecv: task created", "subject", subject)
	}
	return nil
}

// ── Email parsing helpers ────────────────────────────────────────────────────

func extractText(msg *mail.Message) string {
	ct := msg.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	text, _ := readPart(ct, msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
	return strings.TrimSpace(text)
}

func readPart(contentType, transferEncoding string, r io.Reader) (string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Unparseable content type — try reading as plain text.
		body, _ := io.ReadAll(decode(transferEncoding, r))
		return string(body), nil
	}

	switch {
	case mediaType == "text/plain":
		body, err := io.ReadAll(decode(transferEncoding, r))
		return string(body), err

	case mediaType == "text/html":
		body, err := io.ReadAll(decode(transferEncoding, r))
		if err != nil {
			return "", err
		}
		return stripHTML(string(body)), nil

	case strings.HasPrefix(mediaType, "multipart/"):
		boundary := params["boundary"]
		if boundary == "" {
			return "", nil
		}
		mr := multipart.NewReader(r, boundary)
		var plainText, htmlText string
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			pct := p.Header.Get("Content-Type")
			if pct == "" {
				pct = "text/plain"
			}
			pte := p.Header.Get("Content-Transfer-Encoding")
			text, _ := readPart(pct, pte, p)
			pMedia, _, _ := mime.ParseMediaType(pct)
			if pMedia == "text/plain" && plainText == "" {
				plainText = text
			} else if pMedia == "text/html" && htmlText == "" {
				htmlText = text
			}
			// If we found plain text in a multipart/alternative, prefer it.
			if plainText != "" && strings.HasPrefix(mediaType, "multipart/alternative") {
				break
			}
		}
		if plainText != "" {
			return plainText, nil
		}
		return htmlText, nil
	}
	return "", nil
}

func decode(encoding string, r io.Reader) io.Reader {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, &whitespaceStripper{r: r})
	default:
		return r
	}
}

// whitespaceStripper strips whitespace so base64.NewDecoder doesn't choke on line breaks.
type whitespaceStripper struct{ r io.Reader }

func (w *whitespaceStripper) Read(p []byte) (int, error) {
	n, err := w.r.Read(p)
	j := 0
	for i := 0; i < n; i++ {
		if p[i] != '\n' && p[i] != '\r' && p[i] != ' ' && p[i] != '\t' {
			p[j] = p[i]
			j++
		}
	}
	return j, err
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)
var multiSpaceRe = regexp.MustCompile(`[ \t]+`)
var multiNLRe = regexp.MustCompile(`\n{3,}`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	s = multiNLRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	out, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// mondayOf returns the ISO week Monday (YYYY-MM-DD) for a given date string.
func mondayOf(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := t.AddDate(0, 0, -(wd - 1))
	return monday.Format("2006-01-02")
}
