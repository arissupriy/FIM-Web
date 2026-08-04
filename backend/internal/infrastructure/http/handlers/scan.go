// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/application/usecase/scan"
)

// ScanHandler handles scan HTTP requests.
type ScanHandler struct {
	uc *scan.UseCase
}

// NewScanHandler creates a new scan handler.
func NewScanHandler(uc *scan.UseCase) *ScanHandler {
	return &ScanHandler{uc: uc}
}

// StartBaseline starts a baseline scan for a project.
func (h *ScanHandler) StartBaseline(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.uc.StartBaseline(r.Context(), id); err != nil {
		if err == scan.ErrJobExists {
			Error(w, http.StatusConflict, "scan already in progress")
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// StartIntegrity starts an integrity scan for a project.
func (h *ScanHandler) StartIntegrity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.uc.StartIntegrity(r.Context(), id); err != nil {
		if err == scan.ErrJobExists {
			Error(w, http.StatusConflict, "scan already in progress")
			return
		}
		if err == scan.ErrNoBaseline {
			Error(w, http.StatusBadRequest, "baseline not established")
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// Cancel cancels a running scan.
func (h *ScanHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	jobID, err := strconv.Atoi(idStr)
	if err != nil || jobID == 0 {
		Error(w, http.StatusBadRequest, "invalid job id")
		return
	}

	if err := h.uc.Cancel(r.Context(), jobID); err != nil {
		if err == scan.ErrNotCancellable {
			Error(w, http.StatusConflict, "job not cancellable")
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// Reset resets baseline and queues new baseline scan.
func (h *ScanHandler) Reset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.uc.Reset(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusAccepted, map[string]string{"status": "reset and queued"})
}
