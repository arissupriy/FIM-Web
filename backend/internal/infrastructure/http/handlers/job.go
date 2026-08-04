// Package handlers provides HTTP handlers for the application.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/application/usecase/job"
)

// JobHandler handles job-related HTTP requests.
type JobHandler struct {
	uc *job.UseCase
}

// NewJobHandler creates a new JobHandler.
func NewJobHandler(uc *job.UseCase) *JobHandler {
	return &JobHandler{uc: uc}
}

// List returns all jobs for a project.
func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Parse limit query param (default 50)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	jobs, err := h.uc.GetByProjectID(r.Context(), projectID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get jobs: "+err.Error())
		return
	}

	OK(w, jobs)
}

// Get returns a single job by ID.
func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.Atoi(chi.URLParam(r, "jobID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	j, err := h.uc.GetByID(r.Context(), jobID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get job: "+err.Error())
		return
	}
	if j == nil {
		Error(w, http.StatusNotFound, "Job not found")
		return
	}

	OK(w, j)
}

// Cancel cancels a running job.
func (h *JobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.Atoi(chi.URLParam(r, "jobID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	// Verify job exists and get project info
	j, err := h.uc.GetByID(r.Context(), jobID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get job: "+err.Error())
		return
	}
	if j == nil {
		Error(w, http.StatusNotFound, "Job not found")
		return
	}

	if err := h.uc.Cancel(r.Context(), jobID); err != nil {
		Error(w, http.StatusInternalServerError, "Failed to cancel job: "+err.Error())
		return
	}

	OK(w, map[string]string{"message": "Job cancelled"})
}

// Stats returns job statistics for a project.
func (h *JobHandler) Stats(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	stats, err := h.uc.GetStats(r.Context(), projectID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get stats: "+err.Error())
		return
	}

	OK(w, stats)
}

// ListActive returns all active jobs across all projects.
func (h *JobHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.uc.GetActiveJobs(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get active jobs: "+err.Error())
		return
	}

	OK(w, jobs)
}

// ParseJobID parses job ID from URL parameter.
func ParseJobID(r *http.Request, param string) (int, error) {
	idStr := chi.URLParam(r, param)
	return strconv.Atoi(idStr)
}

// JobResponse represents job API response.
type JobResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// MarshalJSON implements json.Marshaler for JobResponse.
func (r JobResponse) MarshalJSON() ([]byte, error) {
	type alias JobResponse
	return json.Marshal(alias(r))
}
