// Package handlers provides HTTP request handlers.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/application/usecase/project"
	"ojs-monitor/backend/internal/domain/models"
)

// ProjectHandler handles project HTTP requests.
type ProjectHandler struct {
	uc *project.UseCase
}

// NewProjectHandler creates a new project handler.
func NewProjectHandler(uc *project.UseCase) *ProjectHandler {
	return &ProjectHandler{uc: uc}
}

// List returns all projects.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.uc.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, projects)
}

// Get returns a project by ID.
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		if err == project.ErrNotFound {
			Error(w, http.StatusNotFound, "not found")
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, p)
}

// Create creates a new project.
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	id, err := h.uc.Create(r.Context(), &p)
	if err != nil {
		if err == project.ErrValidation {
			Error(w, http.StatusBadRequest, "name is required")
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	p.ID = id
	Respond(w, http.StatusCreated, p)
}

// Update updates an existing project.
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id

	if err := h.uc.Update(r.Context(), &p); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	OK(w, p)
}

// Delete removes a project.
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := parseIDStr(r, "id")
	if id == 0 {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.uc.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Respond(w, http.StatusNoContent, nil)
}

// parseIDStr extracts int ID from URL parameter using strconv.
func parseIDStr(r *http.Request, param string) int {
	idStr := chi.URLParam(r, param)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0
	}
	return id
}
