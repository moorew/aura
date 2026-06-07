package db

import (
	"context"
	"database/sql"
	"errors"
)

type Attachment struct {
	ID        string `json:"id"`
	OwnerType string `json:"owner_type"` // 'task' | 'objective'
	OwnerID   string `json:"owner_id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

const attachmentCols = `id, owner_type, owner_id, filename, mime_type, size_bytes, created_at`

func scanAttachment(s scanner) (Attachment, error) {
	var a Attachment
	err := s.Scan(&a.ID, &a.OwnerType, &a.OwnerID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.CreatedAt)
	return a, err
}

type AttachmentStore struct{ db *sql.DB }

func NewAttachmentStore(db *sql.DB) *AttachmentStore { return &AttachmentStore{db: db} }

type CreateAttachmentParams struct {
	ID        string
	OwnerType string
	OwnerID   string
	Filename  string
	MimeType  string
	SizeBytes int64
}

func (s *AttachmentStore) Create(ctx context.Context, p CreateAttachmentParams) (Attachment, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO attachments (id, owner_type, owner_id, filename, mime_type, size_bytes)
		VALUES (?,?,?,?,?,?)
		RETURNING `+attachmentCols,
		p.ID, p.OwnerType, p.OwnerID, p.Filename, p.MimeType, p.SizeBytes,
	)
	return scanAttachment(row)
}

func (s *AttachmentStore) Get(ctx context.Context, id string) (Attachment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+attachmentCols+` FROM attachments WHERE id = ?`, id)
	a, err := scanAttachment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	return a, err
}

func (s *AttachmentStore) ListByOwner(ctx context.Context, ownerType, ownerID string) ([]Attachment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+attachmentCols+` FROM attachments
		 WHERE owner_type = ? AND owner_id = ? ORDER BY created_at`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []Attachment{}
	}
	return out, rows.Err()
}

// ListAll returns every attachment row — used by the backup engine.
func (s *AttachmentStore) ListAll(ctx context.Context) ([]Attachment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+attachmentCols+` FROM attachments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []Attachment{}
	}
	return out, rows.Err()
}

func (s *AttachmentStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByOwner removes all attachment rows for an owner and returns the deleted
// IDs so the caller can remove the corresponding blob files.
func (s *AttachmentStore) DeleteByOwner(ctx context.Context, ownerType, ownerID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM attachments WHERE owner_type = ? AND owner_id = ?`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM attachments WHERE owner_type = ? AND owner_id = ?`, ownerType, ownerID); err != nil {
		return nil, err
	}
	return ids, nil
}
