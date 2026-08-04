// Package handlers provides HTTP handlers for the application.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/application/usecase/file"
)

// FileHandler handles file-related HTTP requests.
type FileHandler struct {
	uc *file.UseCase
}

// NewFileHandler creates a new FileHandler.
func NewFileHandler(uc *file.UseCase) *FileHandler {
	return &FileHandler{uc: uc}
}

// List returns files for a project.
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Parse query params
	params := file.ListParams{
		ProjectID: projectID,
		Search:   r.URL.Query().Get("search"),
		Status:   r.URL.Query().Get("status"),
		FileType: r.URL.Query().Get("type"),
		Page:     1,
		Limit:    50,
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			params.Page = parsed
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			params.Limit = parsed
		}
	}

	result, err := h.uc.List(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to list files: "+err.Error())
		return
	}

	OK(w, result)
}

// GetOrphans returns orphan files for a project.
func (h *FileHandler) GetOrphans(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	limit := 500
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	orphans, err := h.uc.GetOrphans(r.Context(), projectID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get orphan files: "+err.Error())
		return
	}

	OK(w, orphans)
}

// Get returns a single file by ID.
func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.Atoi(chi.URLParam(r, "fileId"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid file ID")
		return
	}
	projectID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	f, err := h.uc.GetByID(r.Context(), fileID, projectID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get file: "+err.Error())
		return
	}
	if f == nil || f.ProjectID != projectID {
		Error(w, http.StatusNotFound, "File not found")
		return
	}

	OK(w, f)
}

// GetStats returns file statistics for a project.
func (h *FileHandler) GetStats(w http.ResponseWriter, r *http.Request) {
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

// GetByHash returns files with the same hash.
func (h *FileHandler) GetByHash(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		Error(w, http.StatusBadRequest, "Hash parameter required")
		return
	}

	files, err := h.uc.GetByHash(r.Context(), hash)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get files: "+err.Error())
		return
	}

	OK(w, files)
}
