package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/ical"
)

type icalHandler struct {
	store *db.ICalStore
}

func (h *icalHandler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := h.store.ListSubscriptions(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list subscriptions")
		return
	}
	respond(w, http.StatusOK, subs)
}

func (h *icalHandler) createSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Color string `json:"color"`
	}
	if err := decode(r, &req); err != nil || req.URL == "" {
		respondError(w, http.StatusBadRequest, "name and url are required")
		return
	}
	if req.Color == "" {
		req.Color = "#6b7280"
	}
	sub, err := h.store.CreateSubscription(r.Context(), uuid.New().String(), req.Name, req.URL, req.Color)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}
	// Sync in background so the HTTP response is fast
	go h.syncSubscriptionByID(context.Background(), sub.ID)
	respond(w, http.StatusCreated, sub)
}

func (h *icalHandler) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteSubscription(r.Context(), chi.URLParam(r, "id")); err != nil {
		respondError(w, http.StatusNotFound, "subscription not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *icalHandler) syncSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.syncSubscriptionByID(r.Context(), id)
	respond(w, http.StatusOK, map[string]string{"status": "synced"})
}

func (h *icalHandler) listEventsForDate(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		respondError(w, http.StatusBadRequest, "date is required")
		return
	}
	events, err := h.store.ListEventsForDate(r.Context(), date)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []db.ICalEvent{}
	}
	respond(w, http.StatusOK, events)
}

func (h *icalHandler) syncSubscriptionByID(ctx context.Context, id string) {
	subs, err := h.store.ListSubscriptions(ctx)
	if err != nil {
		return
	}
	for _, sub := range subs {
		if sub.ID == id {
			h.syncOne(ctx, sub)
			return
		}
	}
}

func (h *icalHandler) syncOne(ctx context.Context, sub db.ICalSubscription) {
	events, err := ical.Fetch(sub.URL)
	if err != nil {
		h.store.SetError(ctx, sub.ID, err.Error())
		return
	}
	dbEvents := make([]db.ICalEvent, 0, len(events))
	for _, ev := range events {
		if ev.UID == "" || ev.StartTime == "" {
			continue
		}
		dbEvents = append(dbEvents, db.ICalEvent{
			ID:             uuid.New().String(),
			SubscriptionID: sub.ID,
			UID:            ev.UID,
			Summary:        ev.Summary,
			Description:    ev.Description,
			Location:       ev.Location,
			StartTime:      ev.StartTime,
			EndTime:        ev.EndTime,
			AllDay:         ev.AllDay,
		})
	}
	_ = h.store.UpsertEvents(ctx, sub.ID, dbEvents)
}
