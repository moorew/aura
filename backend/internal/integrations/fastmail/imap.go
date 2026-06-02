package fastmail

// IMAP-based Fastmail client.
// Fastmail's JMAP requires OAuth; app passwords work with IMAP/TLS directly.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

const imapAddr = "imap.fastmail.com:993"

func dial(email, password string) (*imapclient.Client, error) {
	c, err := imapclient.DialTLS(imapAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("IMAP connect: %w", err)
	}
	if err := c.Login(email, password).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("unauthorized — check your email and app password")
	}
	return c, nil
}

// TestConnectionIMAP verifies credentials.
func TestConnectionIMAP(email, password string) error {
	c, err := dial(email, password)
	if err != nil {
		return err
	}
	c.Logout()
	return nil
}

// ── Inbox panel ──────────────────────────────────────────────────────────────

// GetIMAPInboxEmails returns the N most recent INBOX messages.
func GetIMAPInboxEmails(email, password string, limit int) ([]PanelEmail, error) {
	c, err := dial(email, password)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	mbox, err := c.Select("INBOX", nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("SELECT INBOX: %w", err)
	}
	if mbox.NumMessages == 0 {
		return nil, nil
	}

	total := mbox.NumMessages
	start := uint32(1)
	if total > uint32(limit) {
		start = total - uint32(limit) + 1
	}
	seqSet := imap.SeqSet{}
	seqSet.AddRange(start, total)

	msgs, err := c.Fetch(seqSet, &imap.FetchOptions{
		Envelope: true,
		Flags:    true,
		UID:      true,
	}).Collect()
	if err != nil {
		return nil, fmt.Errorf("FETCH: %w", err)
	}

	emails := make([]PanelEmail, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		emails = append(emails, bufToPanelEmail(msgs[i]))
	}
	return emails, nil
}

// ArchiveIMAPEmail moves a message (by UID) from INBOX to Archive.
func ArchiveIMAPEmail(email, password string, uid uint32) error {
	c, err := dial(email, password)
	if err != nil {
		return err
	}
	defer c.Logout()

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		return fmt.Errorf("SELECT INBOX: %w", err)
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	for _, folder := range []string{"Archive", "Archived", "ARCHIVE"} {
		if _, err := c.Move(uidSet, folder).Wait(); err == nil {
			return nil
		}
	}
	// Fall back: just mark deleted
	return c.Store(uidSet, &imap.StoreFlags{
		Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagDeleted},
	}, nil).Close()
}

// GetIMAPEmailBody fetches the full body of a message by UID.
func GetIMAPEmailBody(email, password string, uid uint32) (string, error) {
	c, err := dial(email, password)
	if err != nil {
		return "", err
	}
	defer c.Logout()

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		return "", fmt.Errorf("SELECT INBOX: %w", err)
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	msgs, err := c.Fetch(uidSet, &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{
			{Peek: true},
		},
	}).Collect()
	if err != nil || len(msgs) == 0 {
		return "", err
	}
	for _, sec := range msgs[0].BodySection {
		if parsed, err := mail.ReadMessage(bytes.NewReader(sec.Bytes)); err == nil {
			if text := imapExtractText(parsed); text != "" {
				return text, nil
			}
		}
	}
	return "", nil
}

// ── Starred email sync ───────────────────────────────────────────────────────

// GetIMAPFlaggedEmails returns all flagged/starred messages from INBOX.
func GetIMAPFlaggedEmails(email, password string) ([]Email, error) {
	c, err := dial(email, password)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		return nil, fmt.Errorf("SELECT INBOX: %w", err)
	}

	searchData, err := c.Search(&imap.SearchCriteria{
		Flag: []imap.Flag{imap.FlagFlagged},
	}, nil).Wait()
	if err != nil {
		return nil, err
	}
	nums := searchData.AllSeqNums()
	if len(nums) == 0 {
		return nil, nil
	}

	seqSet := imap.SeqSetNum(nums...)
	msgs, err := c.Fetch(seqSet, &imap.FetchOptions{
		Envelope: true,
		UID:      true,
	}).Collect()
	if err != nil {
		return nil, err
	}

	result := make([]Email, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, bufToEmail(msg))
	}
	return result, nil
}

// ── Task inbox polling ───────────────────────────────────────────────────────

// SyncIMAPTaskInbox fetches unread messages TO inboxAddress, creates tasks, marks read.
func SyncIMAPTaskInbox(ctx context.Context, cfg InboxConfig, tasks *db.TaskStore) (db.SyncResult, error) {
	c, err := dial(cfg.Email, cfg.AppPassword)
	if err != nil {
		return db.SyncResult{}, err
	}
	defer c.Logout()

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		return db.SyncResult{}, fmt.Errorf("SELECT INBOX: %w", err)
	}

	searchData, err := c.Search(&imap.SearchCriteria{
		Header:  []imap.SearchCriteriaHeaderField{{Key: "To", Value: cfg.InboxAddress}},
		NotFlag: []imap.Flag{imap.FlagSeen},
	}, nil).Wait()
	if err != nil {
		return db.SyncResult{}, err
	}
	nums := searchData.AllSeqNums()
	if len(nums) == 0 {
		return db.SyncResult{}, nil
	}

	seqSet := imap.SeqSetNum(nums...)
	msgs, err := c.Fetch(seqSet, &imap.FetchOptions{
		Envelope: true,
		UID:      true,
		BodySection: []*imap.FetchItemBodySection{
			{Peek: true},
		},
	}).Collect()
	if err != nil {
		return db.SyncResult{}, err
	}

	var result db.SyncResult
	today := time.Now().Format("2006-01-02")
	ws := mondayOf(today)

	var seenSeqs []uint32

	for _, msg := range msgs {
		result.Total++
		em := bufToEmail(msg)

		if !senderAllowed(em.From, cfg.AllowedSenders) {
			seenSeqs = append(seenSeqs, msg.SeqNum)
			continue
		}

		sourceID := "taskinbox_" + em.ID
		if _, findErr := tasks.FindBySource(ctx, "fastmail", sourceID); findErr == nil {
			seenSeqs = append(seenSeqs, msg.SeqNum)
			continue
		}

		subject := em.Subject
		if subject == "" {
			subject = "(no subject)"
		}

		var desc *string
		for _, sec := range msg.BodySection {
			if parsed, err := mail.ReadMessage(bytes.NewReader(sec.Bytes)); err == nil {
				if text := imapExtractText(parsed); text != "" {
					if len(text) > 4000 {
						text = text[:4000] + "…"
					}
					desc = &text
					break
				}
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
		}
		seenSeqs = append(seenSeqs, msg.SeqNum)
	}

	if len(seenSeqs) > 0 {
		_ = c.Store(imap.SeqSetNum(seenSeqs...), &imap.StoreFlags{
			Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagSeen},
		}, nil).Close()
	}
	return result, nil
}

// ── Conversion helpers ───────────────────────────────────────────────────────

func bufToPanelEmail(msg *imapclient.FetchMessageBuffer) PanelEmail {
	pe := PanelEmail{
		ID:       fmt.Sprintf("%d", uint32(msg.UID)),
		Keywords: make(map[string]bool),
	}
	if msg.Envelope != nil {
		pe.Subject = msg.Envelope.Subject
		pe.ReceivedAt = msg.Envelope.Date.UTC().Format(time.RFC3339)
		for _, addr := range msg.Envelope.From {
			pe.From = append(pe.From, EmailAddress{
				Name:  addr.Name,
				Email: addr.Mailbox + "@" + addr.Host,
			})
		}
	}
	for _, f := range msg.Flags {
		pe.Keywords[strings.ToLower(string(f))] = true
	}
	return pe
}

func bufToEmail(msg *imapclient.FetchMessageBuffer) Email {
	em := Email{ID: fmt.Sprintf("%d", uint32(msg.UID))}
	if msg.Envelope != nil {
		em.Subject = msg.Envelope.Subject
		em.ReceivedAt = msg.Envelope.Date.UTC().Format(time.RFC3339)
		for _, addr := range msg.Envelope.From {
			em.From = append(em.From, EmailAddress{
				Name:  addr.Name,
				Email: addr.Mailbox + "@" + addr.Host,
			})
		}
	}
	return em
}

func imapExtractText(msg *mail.Message) string {
	ct := msg.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		b, _ := io.ReadAll(msg.Body)
		return strings.TrimSpace(string(b))
	}
	switch {
	case mediaType == "text/plain":
		enc := msg.Header.Get("Content-Transfer-Encoding")
		var r io.Reader = msg.Body
		if strings.EqualFold(enc, "quoted-printable") {
			r = quotedprintable.NewReader(msg.Body)
		}
		b, _ := io.ReadAll(r)
		return strings.TrimSpace(string(b))
	case strings.HasPrefix(mediaType, "multipart/"):
		mr := multipart.NewReader(msg.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			pct := p.Header.Get("Content-Type")
			if pct == "" {
				pct = "text/plain"
			}
			if pm, _, _ := mime.ParseMediaType(pct); pm == "text/plain" {
				enc := p.Header.Get("Content-Transfer-Encoding")
				var r io.Reader = p
				if strings.EqualFold(enc, "quoted-printable") {
					r = quotedprintable.NewReader(p)
				}
				b, _ := io.ReadAll(r)
				return strings.TrimSpace(string(b))
			}
		}
	}
	return ""
}
