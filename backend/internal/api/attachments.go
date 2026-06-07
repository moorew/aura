package api

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/blob"
	"github.com/clevercode/sempa/internal/db"
)

// maxAttachmentBytes caps a single uploaded file at 500 MB.
const maxAttachmentBytes = 500 << 20

type attachmentHandler struct {
	store      *db.AttachmentStore
	blobs      *blob.Store
	tasks      *db.TaskStore
	objectives *db.ObjectiveStore
	hub        *EventHub
}

// ownerExists checks the referenced task/objective is real before accepting an upload.
func (h *attachmentHandler) ownerExists(r *http.Request, ownerType, ownerID string) bool {
	switch ownerType {
	case "task":
		_, err := h.tasks.Get(r.Context(), ownerID)
		return err == nil
	case "objective":
		_, err := h.objectives.Get(r.Context(), ownerID)
		return err == nil
	}
	return false
}

func (h *attachmentHandler) list(ownerType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.store.ListByOwner(r.Context(), ownerType, chi.URLParam(r, "id"))
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to list attachments")
			return
		}
		respond(w, http.StatusOK, items)
	}
}

func (h *attachmentHandler) upload(ownerType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := chi.URLParam(r, "id")
		if !h.ownerExists(r, ownerType, ownerID) {
			respondError(w, http.StatusNotFound, ownerType+" not found")
			return
		}

		// Cap the whole request body. +1 MB slack for multipart framing.
		r.Body = http.MaxBytesReader(w, r.Body, maxAttachmentBytes+(1<<20))

		file, header, err := r.FormFile("file")
		if err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				respondError(w, http.StatusRequestEntityTooLarge, "file exceeds 500 MB limit")
				return
			}
			respondError(w, http.StatusBadRequest, "missing file field")
			return
		}
		defer file.Close()

		if header.Size > maxAttachmentBytes {
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds 500 MB limit")
			return
		}

		filename := sanitizeFilename(header.Filename)
		mimeType := detectMime(header, filename)

		id := uuid.New().String()
		written, err := h.blobs.Create(id, file)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to store file")
			return
		}
		if written > maxAttachmentBytes {
			_ = h.blobs.Remove(id)
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds 500 MB limit")
			return
		}

		att, err := h.store.Create(r.Context(), db.CreateAttachmentParams{
			ID:        id,
			OwnerType: ownerType,
			OwnerID:   ownerID,
			Filename:  filename,
			MimeType:  mimeType,
			SizeBytes: written,
		})
		if err != nil {
			_ = h.blobs.Remove(id)
			respondError(w, http.StatusInternalServerError, "failed to save attachment")
			return
		}

		h.hub.Broadcast("attachment:change", map[string]string{
			"entity": "attachment", "owner_type": ownerType, "owner_id": ownerID,
		})
		respond(w, http.StatusCreated, att)
	}
}

func (h *attachmentHandler) download(w http.ResponseWriter, r *http.Request) {
	att, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get attachment")
		return
	}

	f, err := h.blobs.Open(att.ID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file missing")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", att.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(att.SizeBytes, 10))
	// inline so images/PDFs preview in the browser; quote the filename for the rest.
	disposition := "inline"
	if !isInlineMime(att.MimeType) {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", disposition+`; filename="`+strings.ReplaceAll(att.Filename, `"`, "")+`"`)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = io.Copy(w, f)
}

func (h *attachmentHandler) delete(w http.ResponseWriter, r *http.Request) {
	att, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get attachment")
		return
	}
	if err := h.store.Delete(r.Context(), att.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete attachment")
		return
	}
	_ = h.blobs.Remove(att.ID)
	h.hub.Broadcast("attachment:change", map[string]string{
		"entity": "attachment", "owner_type": att.OwnerType, "owner_id": att.OwnerID,
	})
	respond(w, http.StatusNoContent, nil)
}

// removeForOwner deletes all attachment rows + blobs for an owner. Called when a
// task/objective is deleted so files don't leak. Best-effort; errors are ignored.
func (h *attachmentHandler) removeForOwner(r *http.Request, ownerType, ownerID string) {
	ids, err := h.store.DeleteByOwner(r.Context(), ownerType, ownerID)
	if err != nil {
		return
	}
	for _, id := range ids {
		_ = h.blobs.Remove(id)
	}
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}

// detectMime prefers the browser-supplied Content-Type, falling back to the
// extension and finally a generic binary type.
func detectMime(header *multipart.FileHeader, filename string) string {
	if ct := header.Header.Get("Content-Type"); ct != "" && ct != "application/octet-stream" {
		return ct
	}
	if ext := filepath.Ext(filename); ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return strings.SplitN(byExt, ";", 2)[0]
		}
	}
	return "application/octet-stream"
}

func isInlineMime(m string) bool {
	return strings.HasPrefix(m, "image/") ||
		strings.HasPrefix(m, "video/") ||
		strings.HasPrefix(m, "audio/") ||
		m == "application/pdf" ||
		strings.HasPrefix(m, "text/")
}
