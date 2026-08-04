// Package ojs provides OJS-specific HTTP handlers.
package ojs

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
	"ojs-monitor/backend/internal/infrastructure/http/handlers"
)

// Handler handles OJS-specific HTTP requests.
type Handler struct {
	projectRepo repository.ProjectRepository
	fileRepo   repository.FileRepository
	ojsService *Service
}

// NewHandler creates a new OJS handler.
func NewHandler(projectRepo repository.ProjectRepository, fileRepo repository.FileRepository) *Handler {
	return &Handler{
		projectRepo: projectRepo,
		fileRepo:    fileRepo,
		ojsService: NewService(),
	}
}

// GetDetails returns OJS site details for a project.
func (h *Handler) GetDetails(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Connect to OJS database
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database: "+err.Error())
		return
	}
	defer db.Close()

	// Get details
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	details, err := h.ojsService.GetDetails(ctx, db)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to get OJS details: "+err.Error())
		return
	}

	// Detect version from filesystem
	details.Version = h.ojsService.DetectVersion(p.AppPaths)

	handlers.OK(w, details)
}

// GetFileRelations returns OJS file relations for a specific file.
func (h *Handler) GetFileRelations(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || projectID == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	fileID, err := strconv.Atoi(chi.URLParam(r, "fileId"))
	if err != nil || fileID == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid file id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), projectID)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get file
	files, _, err := h.fileRepo.ListByProjectID(r.Context(), projectID, repository.FileFilters{FileID: fileID})
	if err != nil || len(files) == 0 {
		handlers.Error(w, http.StatusNotFound, "file not found")
		return
	}
	file := files[0]

	// Connect to OJS database
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get relations
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	relations, err := h.ojsService.GetFileRelations(ctx, db, file.FilePath)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to get file relations: "+err.Error())
		return
	}

	handlers.OK(w, map[string]interface{}{
		"file_path": file.FilePath,
		"is_upload": file.FileType == "uploads",
		"relations": relations,
	})
}

// GetMetrics returns OJS database metrics for a project.
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Connect to OJS database
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get metrics
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	metrics, err := h.ojsService.GetMetrics(ctx, db)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to get metrics: "+err.Error())
		return
	}

	handlers.OK(w, metrics)
}

// ValidateIntegrity runs OJS-specific integrity checks.
func (h *Handler) ValidateIntegrity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Connect to OJS database
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Run validation
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	warnings, err := h.ojsService.ValidateIntegrity(ctx, db)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to validate integrity: "+err.Error())
		return
	}

	handlers.OK(w, map[string]interface{}{
		"project_id": id,
		"template":   "ojs",
		"warnings":   warnings,
		"status":     "ok",
	})
}

// OJSDetailsResponse wraps OJS details with additional info.
type OJSDetailsResponse struct {
	Project *models.Project  `json:"project"`
	OJS     *models.OJSDetails `json:"ojs"`
	Version string            `json:"version"`
}

// GetFullDetails returns complete project details with OJS info.
func (h *Handler) GetFullDetails(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Connect to OJS database
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get details
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	details, err := h.ojsService.GetDetails(ctx, db)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to get OJS details: "+err.Error())
		return
	}

	version := h.ojsService.DetectVersion(p.AppPaths)

	handlers.OK(w, OJSDetailsResponse{
		Project: p,
		OJS:     details,
		Version: version,
	})
}

// TestConnection tests OJS database connection and validates paths.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Validate paths
	for _, ap := range p.AppPaths {
		if ap != "" {
			if _, err := os.Stat(ap); os.IsNotExist(err) {
				handlers.Error(w, http.StatusBadRequest, "app path not found: "+ap)
				return
			}
		}
	}
	for _, fp := range p.FilesPaths {
		if fp != "" {
			if _, err := os.Stat(fp); os.IsNotExist(err) {
				handlers.Error(w, http.StatusBadRequest, "files path not found: "+fp)
				return
			}
		}
	}

	// Test database connection
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err != nil {
		handlers.Error(w, http.StatusBadRequest, "database connection failed: "+err.Error())
		return
	}
	defer db.Close()

	handlers.OK(w, map[string]interface{}{
		"success": true,
		"message": "connection and paths are valid",
	})
}

// DashboardMetrics holds dashboard metrics combining FIM and OJS data.
type DashboardMetrics struct {
	Status              string `json:"status"`
	BaselineTotal       int    `json:"baseline_total"`
	BaselineProcessed   int    `json:"baseline_processed"`
	ExecFilesCount      int    `json:"exec_files_count"`
	NewFilesCount       int    `json:"new_files_count"`
	ModifiedFilesCount  int    `json:"modified_files_count"`
	DeletedFilesCount   int    `json:"deleted_files_count"`
	OrphanFilesCount    int    `json:"orphan_files_count"`
	NewUsers            int    `json:"new_users"`
	ValidatedUsers      int    `json:"validated_users"`
	UnvalidatedDisabled int    `json:"unvalidated_disabled"`
	UploadsByNewUsers   int    `json:"uploads_by_new_users"`
	ActiveAdmins        int    `json:"active_admins"`
	BadSelfReg          int    `json:"bad_self_reg"`
}

// GetAudit returns dashboard metrics for a project.
func (h *Handler) GetAudit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	metrics := DashboardMetrics{
		Status:            p.Status,
		BaselineTotal:     p.BaselineTotal,
		BaselineProcessed: p.BaselineProcessed,
	}

	// Get FIM metrics from SQLite
	added, modified, deleted, orphan, err := h.fileRepo.GetStats(r.Context(), id)
	if err == nil {
		metrics.NewFilesCount = added
		metrics.ModifiedFilesCount = modified
		metrics.DeletedFilesCount = deleted
		metrics.OrphanFilesCount = orphan
	}

	// Get OJS database metrics
	db, err := h.ojsService.Connect(Config{
		DBHost: p.DBHost,
		DBUser: p.DBUser,
		DBPass: p.DBPass,
		DBName: p.DBName,
	})
	if err == nil {
		defer db.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		dbMetrics, err := h.ojsService.GetMetrics(ctx, db)
		if err == nil {
			metrics.ActiveAdmins = dbMetrics.ActiveAdmins
			metrics.NewUsers = dbMetrics.NewUsers
			metrics.ValidatedUsers = dbMetrics.ValidatedUsers
			metrics.UnvalidatedDisabled = dbMetrics.UnvalidatedDisabled
			metrics.UploadsByNewUsers = dbMetrics.UploadsByNewUsers
			metrics.BadSelfReg = dbMetrics.BadSelfReg
		}
	}

	handlers.OK(w, metrics)
}
