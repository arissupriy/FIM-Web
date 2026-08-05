// Package ojs provides OJS-specific HTTP handlers.
package ojs

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
	"ojs-monitor/backend/internal/infrastructure/http/handlers"
	"ojs-monitor/backend/internal/templates/ojs/mysql"
)

// Handler handles OJS-specific HTTP requests.
type Handler struct {
	projectRepo repository.ProjectRepository
	fileRepo   repository.FileRepository
}

// NewHandler creates a new OJS handler.
func NewHandler(projectRepo repository.ProjectRepository, fileRepo repository.FileRepository) *Handler {
	return &Handler{
		projectRepo: projectRepo,
		fileRepo:    fileRepo,
	}
}

// connectOJS connects to OJS database for a project.
func connectOJS(p *models.Project) (*mysql.Connection, error) {
	cfg := mysql.Config{
		Host:     p.DBHost,
		User:     p.DBUser,
		Password: p.DBPass,
		DBName:   p.DBName,
		Timeout:  10 * time.Second,
	}
	return mysql.Connect(cfg)
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
	db, err := connectOJS(p)
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database: "+err.Error())
		return
	}
	defer db.Close()

	// Get details
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	details, err := db.GetSystemDetails(ctx)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to get OJS details: "+err.Error())
		return
	}

	// Detect version from filesystem
	details.Version = DetectVersion(p.AppPaths)

	handlers.OK(w, details)
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
	db, err := connectOJS(p)
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get metrics
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	metrics, err := db.GetMetrics(ctx)
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
	db, err := connectOJS(p)
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get warnings
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	warnings, err := db.GetIntegrityWarnings(ctx)
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
	Project *models.Project        `json:"project"`
	OJS     *mysql.SystemDetails   `json:"ojs"`
	Version string                 `json:"version"`
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
	db, err := connectOJS(p)
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get details
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	details, err := db.GetSystemDetails(ctx)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, "failed to get OJS details: "+err.Error())
		return
	}

	version := DetectVersion(p.AppPaths)

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
	db, err := connectOJS(p)
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
	BaselineTotal      int    `json:"baseline_total"`
	BaselineProcessed  int    `json:"baseline_processed"`
	ExecFilesCount     int    `json:"exec_files_count"`
	NewFilesCount      int    `json:"new_files_count"`
	ModifiedFilesCount int    `json:"modified_files_count"`
	DeletedFilesCount  int    `json:"deleted_files_count"`
	OrphanFilesCount   int    `json:"orphan_files_count"`
	NewUsers           int    `json:"new_users"`
	ValidatedUsers     int    `json:"validated_users"`
	UnvalidatedDisabled int   `json:"unvalidated_disabled"`
	UploadsByNewUsers int     `json:"uploads_by_new_users"`
	ActiveAdmins      int      `json:"active_admins"`
	BadSelfReg        int      `json:"bad_self_reg"`
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
	db, err := connectOJS(p)
	if err == nil {
		defer db.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ojsMetrics, err := db.GetMetrics(ctx)
		if err == nil {
			metrics.ActiveAdmins = ojsMetrics.ActiveAdmins
			metrics.NewUsers = ojsMetrics.NewUsers
			metrics.ValidatedUsers = ojsMetrics.ValidatedUsers
			metrics.UnvalidatedDisabled = ojsMetrics.UnvalidatedDisabled
			metrics.UploadsByNewUsers = ojsMetrics.UploadsByNewUsers
			metrics.BadSelfReg = ojsMetrics.BadSelfReg
		}
	}

	handlers.OK(w, metrics)
}

// GetFileRelations returns the OJS database records for a specific file.
func (h *Handler) GetFileRelations(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}

	fileID, err := strconv.Atoi(chi.URLParam(r, "fileId"))
	if err != nil || fileID == 0 {
		handlers.Error(w, http.StatusBadRequest, "invalid file id")
		return
	}

	// Get project
	p, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		handlers.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get file from database
	file, err := h.fileRepo.GetByID(r.Context(), id, fileID)
	if err != nil {
		handlers.Error(w, http.StatusNotFound, "file not found")
		return
	}

	// Connect to OJS database
	db, err := connectOJS(p)
	if err != nil {
		handlers.Error(w, http.StatusServiceUnavailable, "failed to connect to OJS database")
		return
	}
	defer db.Close()

	// Get file info from OJS database
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	baseName := filepath.Base(file.FilePath)
	info, err := db.GetFileInfo(ctx, baseName)
	if err != nil {
		handlers.OK(w, map[string]interface{}{
			"file":           file,
			"ojs_relation":   nil,
			"in_database":    false,
			"classification": file.Status,
		})
		return
	}

	handlers.OK(w, map[string]interface{}{
		"file":         file,
		"ojs_relation": info,
		"in_database":  true,
		"uploader": map[string]interface{}{
			"user_id":    info.UserID,
			"username":   info.Username,
			"email":      info.Email,
			"full_name": info.FullName,
		},
		"submission": map[string]interface{}{
			"submission_id": info.SubmissionID,
			"stage":        info.Stage,
		},
	})
}
