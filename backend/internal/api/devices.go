package api

import (
	"net/http"

	"github.com/clevercode/sempa/internal/db"
)

type deviceHandler struct {
	store *db.DeviceTokenStore
}

type registerDeviceRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (h *deviceHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerDeviceRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.Platform == "" {
		req.Platform = "android"
	}

	device, err := h.store.Upsert(newID(), req.Token, req.Platform)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to register device")
		return
	}

	respond(w, http.StatusOK, device)
}

func (h *deviceHandler) unregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decode(r, &req); err != nil || req.Token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	if err := h.store.Delete(req.Token); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to unregister device")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
