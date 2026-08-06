// Package handlers contains HTTP handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/application/dto/response"
	"ojs-monitor/backend/internal/application/usecase/alert"
)

// AlertHandler handles alert-related HTTP requests.
type AlertHandler struct {
	uc *alert.Usecase
}

// NewAlertHandler creates a new alert handler.
func NewAlertHandler(uc *alert.Usecase) *AlertHandler {
	return &AlertHandler{uc: uc}
}

// CreateAlertConfig handles POST /api/v1/alerts
func (h *AlertHandler) CreateAlertConfig(w http.ResponseWriter, r *http.Request) {
	var input alert.CreateAlertConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	config, err := h.uc.CreateAlertConfig(r.Context(), &input)
	if err != nil {
		switch err {
		case alert.ErrInvalidInput:
			BadRequest(w, "invalid input")
		case alert.ErrChannelNotSupported:
			BadRequest(w, "unsupported channel")
		default:
			InternalError(w, "failed to create alert config")
		}
		return
	}

	Created(w, response.ToAlertConfigResponse(config))
}

// GetAlertConfig handles GET /api/v1/alerts/:id
func (h *AlertHandler) GetAlertConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	config, err := h.uc.GetAlertConfig(r.Context(), id)
	if err != nil {
		if err == alert.ErrAlertConfigNotFound {
			NotFound(w, "alert config not found")
			return
		}
		InternalError(w, "failed to get alert config")
		return
	}

	OK(w, response.ToAlertConfigResponse(config))
}

// ListAlertConfigs handles GET /api/v1/projects/:project_id/alerts
func (h *AlertHandler) ListAlertConfigs(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "project_id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		BadRequest(w, "invalid project ID")
		return
	}

	configs, err := h.uc.ListAlertConfigs(r.Context(), projectID)
	if err != nil {
		InternalError(w, "failed to list alert configs")
		return
	}

	OK(w, response.ToAlertConfigResponseList(configs))
}

// UpdateAlertConfig handles PUT /api/v1/alerts/:id
func (h *AlertHandler) UpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	var input alert.UpdateAlertConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	config, err := h.uc.UpdateAlertConfig(r.Context(), id, &input)
	if err != nil {
		switch err {
		case alert.ErrAlertConfigNotFound:
			NotFound(w, "alert config not found")
		case alert.ErrInvalidInput:
			BadRequest(w, "invalid input")
		case alert.ErrChannelNotSupported:
			BadRequest(w, "unsupported channel")
		case alert.ErrProjectMismatch:
			BadRequest(w, "project ID mismatch")
		default:
			InternalError(w, "failed to update alert config")
		}
		return
	}

	OK(w, response.ToAlertConfigResponse(config))
}

// DeleteAlertConfig handles DELETE /api/v1/alerts/:id
func (h *AlertHandler) DeleteAlertConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	err = h.uc.DeleteAlertConfig(r.Context(), id)
	if err != nil {
		if err == alert.ErrAlertConfigNotFound {
			NotFound(w, "alert config not found")
			return
		}
		InternalError(w, "failed to delete alert config")
		return
	}

	NoContent(w)
}

// EnableAlertConfig handles POST /api/v1/alerts/:id/enable
func (h *AlertHandler) EnableAlertConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	err = h.uc.EnableAlertConfig(r.Context(), id)
	if err != nil {
		if err == alert.ErrAlertConfigNotFound {
			NotFound(w, "alert config not found")
			return
		}
		InternalError(w, "failed to enable alert config")
		return
	}

	OK(w, map[string]string{"message": "alert config enabled"})
}

// DisableAlertConfig handles POST /api/v1/alerts/:id/disable
func (h *AlertHandler) DisableAlertConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	err = h.uc.DisableAlertConfig(r.Context(), id)
	if err != nil {
		if err == alert.ErrAlertConfigNotFound {
			NotFound(w, "alert config not found")
			return
		}
		InternalError(w, "failed to disable alert config")
		return
	}

	OK(w, map[string]string{"message": "alert config disabled"})
}

// GetAlertHistory handles GET /api/v1/alerts/:id/history
func (h *AlertHandler) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := h.uc.GetAlertHistory(r.Context(), id, limit)
	if err != nil {
		InternalError(w, "failed to get alert history")
		return
	}

	OK(w, response.ToAlertHistoryResponseList(history))
}

// ListProjectAlertHistory handles GET /api/v1/projects/:project_id/alerts/history
func (h *AlertHandler) ListProjectAlertHistory(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "project_id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		BadRequest(w, "invalid project ID")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := h.uc.ListProjectAlertHistory(r.Context(), projectID, limit)
	if err != nil {
		InternalError(w, "failed to list alert history")
		return
	}

	OK(w, response.ToAlertHistoryResponseList(history))
}

// GetAlertStats handles GET /api/v1/projects/:project_id/alerts/stats
func (h *AlertHandler) GetAlertStats(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "project_id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		BadRequest(w, "invalid project ID")
		return
	}

	stats, err := h.uc.GetAlertStats(r.Context(), projectID)
	if err != nil {
		InternalError(w, "failed to get alert stats")
		return
	}

	OK(w, response.NewAlertStatsResponse(
		stats.ProjectID,
		stats.TotalSent+stats.TotalFailed+stats.TotalPending,
		stats.TotalSent,
		stats.TotalFailed,
		stats.TotalPending,
		stats.ByChannel,
	))
}

// TestAlertConfig handles POST /api/v1/alerts/:id/test
func (h *AlertHandler) TestAlertConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(w, "invalid alert config ID")
		return
	}

	err = h.uc.TestAlertConfig(r.Context(), id)
	if err != nil {
		if err == alert.ErrAlertConfigNotFound {
			NotFound(w, "alert config not found")
			return
		}
		InternalError(w, "failed to test alert config")
		return
	}

	OK(w, map[string]string{"message": "test alert queued"})
}
