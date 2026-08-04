// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"

	"ojs-monitor/backend/internal/domain/repository"
)

// FIMHandler handles FIM HTTP requests.
type FIMHandler struct {
	projectRepo repository.ProjectRepository
	eventRepo  repository.FIMEventRepository
}

// NewFIMHandler creates a new FIM handler.
func NewFIMHandler(projectRepo repository.ProjectRepository, eventRepo repository.FIMEventRepository) *FIMHandler {
	return &FIMHandler{
		projectRepo: projectRepo,
		eventRepo:  eventRepo,
	}
}

// GetStatus returns FIM status for a project.
func (h *FIMHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	OK(w, map[string]interface{}{
		"project_id":     p.ID,
		"watching":       p.WatcherStatus == "active",
		"last_scan":      p.LastIntegrityScan,
		"baseline_at":    p.BaselineAt,
		"baseline_hash":  "",
	})
}

// GetEvents returns recent FIM events for a project.
func (h *FIMHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	filters := repository.NewFIMEventFilters()
	filters.Limit = 100
	events, _, err := h.eventRepo.GetByProjectID(r.Context(), id, filters)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	OK(w, events)
}

// GetEventStats returns FIM event statistics for a project.
func (h *FIMHandler) GetEventStats(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	stats, err := h.eventRepo.GetStats(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	OK(w, stats)
}

// GetWatcherStatus returns watcher status for a project.
func (h *FIMHandler) GetWatcherStatus(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	OK(w, map[string]interface{}{
		"project_id":     p.ID,
		"watcher_status": p.WatcherStatus,
		"is_active":     p.WatcherStatus == "active",
	})
}

// StartWatcher starts the inotify watcher for a project.
func (h *FIMHandler) StartWatcher(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.projectRepo.UpdateWatcherStatus(r.Context(), id, "active"); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusOK, map[string]string{"status": "watcher started"})
}

// StopWatcher stops the inotify watcher for a project.
func (h *FIMHandler) StopWatcher(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.projectRepo.UpdateWatcherStatus(r.Context(), id, "stopped"); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusOK, map[string]string{"status": "watcher stopped"})
}
