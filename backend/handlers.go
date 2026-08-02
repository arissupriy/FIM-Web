package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// escapeLike escapes special characters in LIKE patterns to prevent SQL injection
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

func testDBConnection(p Project) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&timeout=10s&readTimeout=15s",
		p.DBUser, p.DBPass, p.DBHost, p.DBName)

	mysqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer mysqlDB.Close()

	done := make(chan error, 1)
	go func() {
		done <- mysqlDB.Ping()
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("connection timeout after 10 seconds")
	case err := <-done:
		return err
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: data})
}

func respondError(w http.ResponseWriter, status int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: false, Error: errMsg})
}

type ProjectResponse struct {
	Project
	Metrics *DashboardMetrics `json:"metrics,omitempty"`
}

func handleGetProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := getProjects()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Create context with timeout for metrics queries
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.DBQueryTimeoutSecs)*time.Second)
	defer cancel()

	var res []ProjectResponse
	for _, p := range projects {
		p.Configured = p.IsConfigured()
		pr := ProjectResponse{Project: p}
		if p.Status == "active" || p.Status == "completed_with_warnings" || p.Status == "error" {
			metrics, err := FastAuditProject(ctx, p)
			if err == nil {
				metrics.Status = p.Status
				pr.Metrics = &metrics
			} else {
				// Log error but don't fail the entire request
				log.Printf("Warning: failed to get metrics for project %d: %v", p.ID, err)
			}
		}
		res = append(res, pr)
	}

	respondJSON(w, http.StatusOK, res)
}



func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	admin, err := GetAdminByUsername(req.Username)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := GenerateToken(admin.ID, admin.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	
	LogActivity(admin.ID, "LOGIN", "System")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"username": admin.Username,
	})
}

func handleAddProject(w http.ResponseWriter, r *http.Request) {
	var p Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validation: only name is required for initial creation
	if strings.TrimSpace(p.Name) == "" {
		respondError(w, http.StatusBadRequest, "Project name is required")
		return
	}

	// Set default template if not specified
	if p.Template == "" {
		p.Template = "OJS 3.x"
	}

	id, err := addProject(p)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create project: "+err.Error())
		return
	}

	p.ID = id
	adminIDVal := r.Context().Value("admin_id")
	adminID, ok := adminIDVal.(int)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid session")
		return
	}
	LogActivity(adminID, "ADD_PROJECT", p.Name)

	respondJSON(w, http.StatusCreated, p)
}

func handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	var p Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	p.ID = id

	if err := testDBConnection(p); err != nil {
		respondError(w, http.StatusBadRequest, "MySQL connection failed: "+err.Error())
		return
	}

	if err := updateProject(p); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update project: "+err.Error())
		return
	}

	adminIDVal := r.Context().Value("admin_id")
	adminID, ok := adminIDVal.(int)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid session")
		return
	}
	LogActivity(adminID, "UPDATE_PROJECT", p.Name)

	respondJSON(w, http.StatusOK, p)
}

func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := GetAuditLogs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch logs: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, logs)
}

func handleAuditProject(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Fetch project by ID from DB (quick manual search from list for simplicity in this MVP)
	projects, err := getProjects()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var targetProject *Project
	for _, p := range projects {
		if p.ID == id {
			targetProject = &p
			break
		}
	}

	if targetProject == nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	if targetProject.Status != "active" {
		// Return status if not active
		respondJSON(w, http.StatusOK, DashboardMetrics{
			Status:            targetProject.Status,
			BaselineTotal:     targetProject.BaselineTotal,
			BaselineProcessed: targetProject.BaselineProcessed,
		})
		return
	}

	// Fetch cached or fast metrics with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.DBQueryTimeoutSecs)*time.Second)
	defer cancel()

	metrics, err := FastAuditProject(ctx, *targetProject)
	if err != nil {
		// Return partial data instead of error
		log.Printf("Warning: failed to get audit metrics for project %d: %v", targetProject.ID, err)
		respondJSON(w, http.StatusOK, DashboardMetrics{
			Status:            targetProject.Status,
			BaselineTotal:     targetProject.BaselineTotal,
			BaselineProcessed: targetProject.BaselineProcessed,
		})
		return
	}
	metrics.Status = targetProject.Status
	metrics.BaselineTotal = targetProject.BaselineTotal
	metrics.BaselineProcessed = targetProject.BaselineProcessed

	respondJSON(w, http.StatusOK, metrics)
}

func handleGetProjectDetails(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Get OJS details with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.DBQueryTimeoutSecs)*time.Second)
	defer cancel()

	details, err := getOJSDetails(ctx, p)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get OJS details: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, details)
}

func handleStartScan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// 1. Validate File System
	for _, ap := range p.AppPaths {
		if ap != "" {
			if _, err := os.Stat(ap); os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("App path not found: %s", ap))
				return
			}
		}
	}
	for _, fp := range p.FilesPaths {
		if fp != "" {
			if _, err := os.Stat(fp); os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Uploads path not found: %s", fp))
				return
			}
		}
	}

	// 2. Validate DB Connection
	if err := testDBConnection(p); err != nil {
		respondError(w, http.StatusBadRequest, "Database connection failed: "+err.Error())
		return
	}

	// 3. Queue the initial baseline job and update status
	var count int
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE project_id = ? AND status IN ('queued', 'running')", p.ID).Scan(&count)
	if count > 0 {
		respondError(w, http.StatusConflict, "A scan is already in progress or queued for this project")
		return
	}

	// Check if baseline already exists
	hasBaseline := p.BaselineAt > 0

	var jobType string
	if hasBaseline {
		// Already has baseline - this is an integrity scan
		jobType = "integrity_scan"
	} else {
		// First time - establish baseline
		jobType = "initial_baseline"
	}

	_, err = db.Exec("INSERT INTO jobs (project_id, type, status) VALUES (?, ?, 'queued')", p.ID, jobType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to queue job: "+err.Error())
		return
	}

	if jobType == "initial_baseline" {
		db.Exec("UPDATE projects SET status='pending_baseline' WHERE id=?", p.ID)
	}

	// Trigger worker to process immediately
	TriggerWorker()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Job queued: %s", jobType),
		"job_type": jobType,
	})
}

// handleForceScan cancels existing jobs and starts a new scan immediately
func handleForceScan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Validate paths
	for _, ap := range p.AppPaths {
		if ap != "" {
			if _, err := os.Stat(ap); os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("App path not found: %s", ap))
				return
			}
		}
	}
	for _, fp := range p.FilesPaths {
		if fp != "" {
			if _, err := os.Stat(fp); os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Uploads path not found: %s", fp))
				return
			}
		}
	}

	// Validate DB connection
	if err := testDBConnection(p); err != nil {
		respondError(w, http.StatusBadRequest, "Database connection failed: "+err.Error())
		return
	}

	// Cancel all existing queued/scheduled jobs for this project
	db.Exec("UPDATE jobs SET status = 'cancelled' WHERE project_id = ? AND status IN ('queued', 'running', 'scheduled')", id)

	// Determine job type
	var jobType string
	if p.BaselineAt > 0 {
		jobType = "integrity_scan"
	} else {
		jobType = "initial_baseline"
	}

	// Insert new job as queued (will run immediately via worker)
	now := time.Now().Unix()
	_, err = db.Exec("INSERT INTO jobs (project_id, type, status, scheduled_at) VALUES (?, ?, 'queued', ?)", id, jobType, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to queue job: "+err.Error())
		return
	}

	// Update project status
	if jobType == "initial_baseline" {
		db.Exec("UPDATE projects SET status='pending_baseline' WHERE id=?", id)
	}

	// Trigger worker immediately
	TriggerWorker()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Force scan started: %s", jobType),
		"job_type": jobType,
	})
}

// handleResetBaseline resets the baseline and starts fresh
func handleResetBaseline(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}
	_ = p // used for potential future validation

	// Cancel any stuck jobs first
	db.Exec("UPDATE jobs SET status = 'cancelled' WHERE project_id = ? AND status IN ('queued', 'running')", id)

	// Clear existing files (start fresh)
	db.Exec("DELETE FROM project_files WHERE project_id = ?", id)

	// Keep FIM events for forensic history, but clear file_id references
	db.Exec("UPDATE fim_events SET file_id = NULL WHERE project_id = ?", id)

	// Reset baseline timestamp
	db.Exec("UPDATE projects SET baseline_at = 0, baseline_total = 0, baseline_processed = 0, watcher_status = 'stopped' WHERE id = ?", id)

	// Queue new baseline job
	_, err = db.Exec("INSERT INTO jobs (project_id, type, status) VALUES (?, 'initial_baseline', 'queued')", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to queue job: "+err.Error())
		return
	}

	db.Exec("UPDATE projects SET status='pending_baseline' WHERE id=?", id)
	TriggerWorker()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Baseline reset. New baseline scan queued.",
	})
}

// handleStartIntegrityScan starts a manual integrity scan
// Mode: "now" (force immediate) or "later" (schedule for next interval)
func handleStartIntegrityScan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	if p.BaselineAt == 0 {
		respondError(w, http.StatusBadRequest, "No baseline established. Run baseline scan first.")
		return
	}

	// Parse mode from query/body (default: "now")
	mode := "now"
	if r.URL.Query().Get("mode") == "later" {
		mode = "later"
	}

	// Check if scan is already in progress (only for "now" mode)
	if mode == "now" {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM jobs WHERE project_id = ? AND status IN ('queued', 'running')", p.ID).Scan(&count)
		if count > 0 {
			respondError(w, http.StatusConflict, "A scan is already in progress or queued")
			return
		}
	}

	now := time.Now().Unix()

	if mode == "later" {
		// Schedule for next interval
		intervalHours := 24
		if p.IntegrityScanEnabled == 1 {
			db.QueryRow("SELECT COALESCE(integrity_scan_interval_hours, 24) FROM projects WHERE id = ?", id).Scan(&intervalHours)
		}
		nextScheduled := now + int64(intervalHours*3600)

		// Check if already scheduled
		var existingCount int
		db.QueryRow("SELECT COUNT(*) FROM jobs WHERE project_id = ? AND type = 'integrity_scan' AND status = 'scheduled'", id).Scan(&existingCount)
		if existingCount > 0 {
			respondError(w, http.StatusConflict, "An integrity scan is already scheduled")
			return
		}

		db.Exec("INSERT INTO jobs (project_id, type, status, scheduled_at) VALUES (?, 'integrity_scan', 'scheduled', ?)", id, nextScheduled)
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Integrity scan scheduled for %s", time.Unix(nextScheduled, 0).Format("Jan 2, 15:04")),
			"scheduled_at": nextScheduled,
		})
		return
	}

	// Force run now
	_, err = db.Exec("INSERT INTO jobs (project_id, type, status, scheduled_at) VALUES (?, 'integrity_scan', 'queued', ?)", id, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to queue job: "+err.Error())
		return
	}

	TriggerWorker()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Integrity scan queued",
	})
}

func handleGetProject(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}
	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}
	p.Configured = p.IsConfigured()
	respondJSON(w, http.StatusOK, p)
}

func handleGetProjectJobs(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Get jobs (exclude cancelled)
	rows, err := db.Query(`
		SELECT id, type, status, error_message,
		       COALESCE(scheduled_at, 0) as scheduled_at,
		       created_at,
		       COALESCE(finished_at, '') as finished_at
		FROM jobs
		WHERE project_id = ? AND status != 'cancelled'
		ORDER BY COALESCE(scheduled_at, 0) DESC, id DESC
		LIMIT 100
	`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var jobs []map[string]interface{}
	for rows.Next() {
		var jID int
		var jType, jStatus, jErr, jCreated, jFinished string
		var jScheduledAt int64
		rows.Scan(&jID, &jType, &jStatus, &jErr, &jScheduledAt, &jCreated, &jFinished)
		jobs = append(jobs, map[string]interface{}{
			"id": jID,
			"type": jType,
			"status": jStatus,
			"error": jErr,
			"scheduled_at": jScheduledAt,
			"created_at": jCreated,
			"finished_at": jFinished,
		})
	}
	if jobs == nil {
		jobs = []map[string]interface{}{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": jobs,
	})
}

func handleGetProjectFiles(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}
	rows, err := db.Query("SELECT id, file_path, hash, status, datetime(mod_time, 'unixepoch') FROM project_files WHERE project_id = ? ORDER BY id DESC LIMIT 500", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var files []map[string]interface{}
	for rows.Next() {
		var fID int
		var fPath, fHash, fStatus, fCreated string
		rows.Scan(&fID, &fPath, &fHash, &fStatus, &fCreated)
		files = append(files, map[string]interface{}{
			"id": fID, "file_path": fPath, "hash": fHash, "status": fStatus, "created_at": fCreated,
		})
	}
	if files == nil {
		files = []map[string]interface{}{}
	}
	respondJSON(w, http.StatusOK, files)
}

func handleGetProjectFilesPaginated(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Get query params
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	typeFilter := r.URL.Query().Get("type") // "project", "uploads", or "all"
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	// Get project to know app_paths and files_paths
	p, err := getProjectByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Build base conditions
	baseConditions := "WHERE project_id = ?"
	args := []interface{}{id}
	countArgs := []interface{}{id}

	if search != "" {
		baseConditions += " AND file_path LIKE ?"
		searchPattern := "%" + escapeLike(search) + "%"
		args = append(args, searchPattern)
		countArgs = append(countArgs, searchPattern)
	}

	if status != "" && status != "all" {
		baseConditions += " AND status = ?"
		args = append(args, status)
		countArgs = append(countArgs, status)
	}

	// Build file type filter
	typeCondition := ""
	if typeFilter != "" && typeFilter != "all" {
		if typeFilter == "project" {
			// Files NOT in files_path = project files
			if len(p.FilesPaths) > 0 && p.FilesPaths[0] != "" {
				typeCondition += " AND ("
				for i, fp := range p.FilesPaths {
					if fp != "" {
						if i > 0 {
							typeCondition += " AND "
						}
						typeCondition += "file_path NOT LIKE ?"
						likePattern := fp + "%"
						args = append(args, likePattern)
						countArgs = append(countArgs, likePattern)
					}
				}
				typeCondition += ")"
			}
		} else if typeFilter == "uploads" {
			// Files in files_path = uploads
			for _, fp := range p.FilesPaths {
				if fp != "" {
					typeCondition += " AND file_path LIKE ?"
					likePattern := fp + "%"
					args = append(args, likePattern)
					countArgs = append(countArgs, likePattern)
					break
				}
			}
		}
	}

	query := "SELECT id, file_path, hash, status, mod_time, COALESCE(created_at, 0), COALESCE(updated_at, 0), COALESCE(file_type, 'project') FROM project_files " + baseConditions + typeCondition
	countQuery := "SELECT COUNT(*) FROM project_files " + baseConditions + typeCondition

	// Get total count
	var total int
	db.QueryRow(countQuery, countArgs...).Scan(&total)

	// Get paginated results
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var files []map[string]interface{}
	for rows.Next() {
		var fID int
		var fPath, fHash, fStatus, fFileType string
		var fModTime, fCreated, fUpdated int64
		rows.Scan(&fID, &fPath, &fHash, &fStatus, &fModTime, &fCreated, &fUpdated, &fFileType)

		files = append(files, map[string]interface{}{
			"id": fID, "file_path": fPath, "hash": fHash, "status": fStatus,
			"file_type": fFileType,
			"mod_time": time.Unix(fModTime, 0).Format(time.RFC3339),
			"created_at": time.Unix(fCreated, 0).Format(time.RFC3339),
			"updated_at": time.Unix(fUpdated, 0).Format(time.RFC3339),
		})
	}
	if files == nil {
		files = []map[string]interface{}{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
		"pagination": map[string]interface{}{
			"page": page,
			"limit": limit,
			"total": total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

func handleGetOrphanFiles(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	// Build query for orphan files only
	query := "SELECT id, file_path, hash, status, datetime(mod_time, 'unixepoch') FROM project_files WHERE project_id = ? AND status = 'ORPHAN'"
	countQuery := "SELECT COUNT(*) FROM project_files WHERE project_id = ? AND status = 'ORPHAN'"
	args := []interface{}{id}
	countArgs := []interface{}{id}

	if search != "" {
		query += " AND file_path LIKE ?"
		countQuery += " AND file_path LIKE ?"
		searchPattern := "%" + escapeLike(search) + "%" // Escape special chars to prevent LIKE injection
		args = append(args, searchPattern)
		countArgs = append(countArgs, searchPattern)
	}

	// Get total count
	var total int
	db.QueryRow(countQuery, countArgs...).Scan(&total)

	// Get paginated results
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var files []map[string]interface{}
	for rows.Next() {
		var fID int
		var fPath, fHash, fStatus, fCreated string
		rows.Scan(&fID, &fPath, &fHash, &fStatus, &fCreated)
		files = append(files, map[string]interface{}{
			"id": fID, "file_path": fPath, "hash": fHash, "status": fStatus, "created_at": fCreated,
		})
	}
	if files == nil {
		files = []map[string]interface{}{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
		"pagination": map[string]interface{}{
			"page": page,
			"limit": limit,
			"total": total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

func handleGetFileDetail(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	fileIDStr := chi.URLParam(r, "fileId")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Get file details
	var fID int
	var fPath, fHash, fStatus string
	var fSize int64
	var fModTime int64
	var fCreated, fUpdated int64
	err = db.QueryRow(`
		SELECT id, file_path, hash, status, file_size, mod_time, COALESCE(created_at, 0), COALESCE(updated_at, 0)
		FROM project_files WHERE id = ? AND project_id = ?
	`, fileID, projectID).Scan(&fID, &fPath, &fHash, &fStatus, &fSize, &fModTime, &fCreated, &fUpdated)
	if err != nil {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// Get project to know paths
	p, err := getProjectByID(projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Classify file type
	fileType := "project"
	for _, fp := range p.FilesPaths {
		if fp != "" && strings.HasPrefix(fPath, fp) {
			fileType = "uploads"
			break
		}
	}

	// Get file history
	var history []map[string]interface{}
	rows, err := db.Query(`
		SELECT id, hash, status, datetime(created_at, 'unixepoch') as created_at_str
		FROM project_files
		WHERE project_id = ? AND file_path = ?
		ORDER BY created_at DESC
	`, projectID, fPath)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var hID int
			var hHash, hStatus, hDate string
			rows.Scan(&hID, &hHash, &hStatus, &hDate)
			history = append(history, map[string]interface{}{
				"id": hID, "hash": hHash, "status": hStatus, "date": hDate,
			})
		}
	}
	if history == nil {
		history = []map[string]interface{}{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"file": map[string]interface{}{
			"id": fID,
			"file_path": fPath,
			"hash": fHash,
			"status": fStatus,
			"file_type": fileType,
			"file_size": fSize,
			"file_mod_time": time.Unix(fModTime, 0).Format(time.RFC3339),
			"created_at": time.Unix(fCreated, 0).Format(time.RFC3339),
			"updated_at": time.Unix(fUpdated, 0).Format(time.RFC3339),
		},
		"history": history,
	})
}

func handleGetFileOJSRelations(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	fileIDStr := chi.URLParam(r, "fileId")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Get file details
	var fPath string
	err = db.QueryRow("SELECT file_path FROM project_files WHERE id = ? AND project_id = ?", fileID, projectID).Scan(&fPath)
	if err != nil {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	p, err := getProjectByID(projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Only process files in files_path
	isUpload := false
	for _, fp := range p.FilesPaths {
		if fp != "" && strings.HasPrefix(fPath, fp) {
			isUpload = true
			break
		}
	}

	result := map[string]interface{}{
		"file_path": fPath,
		"is_upload": isUpload,
		"relations": []map[string]interface{}{},
	}

	if !isUpload {
		respondJSON(w, http.StatusOK, result)
		return
	}

	mysqlDB, err := connectMySQL(p.DBUser, p.DBPass, p.DBHost, p.DBName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to connect to OJS database")
		return
	}
	defer mysqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	basename := filepath.Base(fPath)
	var relations []map[string]interface{}
	sqlQuery := `SELECT sf.file_id, sf.original_file_name, sf.submission_id, sf.uploader_user_id, sf.file_type, sf.date_uploaded, sf.stage_id, sf.revision, COALESCE(u.username, 'unknown') as uploader_name, COALESCE(u.email, '') as uploader_email, COALESCE(sfr.round, 0) as review_round FROM submission_files sf LEFT JOIN users u ON u.user_id = sf.uploader_user_id LEFT JOIN (SELECT file_id, MAX(review_round_id) as round FROM review_round_files GROUP BY file_id) sfr ON sfr.file_id = sf.file_id WHERE sf.original_file_name = ?`
	rows, err := mysqlDB.QueryContext(ctx, sqlQuery, basename)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sfFileID, sfSubmissionID, sfUploaderID int
			var sfOriginalName, sfUploaderName, sfUploaderEmail, sfFileType, sfDateUploaded string
			var sfStageID, sfRevision, sfReviewRound int

			rows.Scan(&sfFileID, &sfOriginalName, &sfSubmissionID, &sfUploaderID, &sfFileType, &sfDateUploaded, &sfStageID, &sfRevision, &sfUploaderName, &sfUploaderEmail, &sfReviewRound)

			// Get article title
			var articleTitle string
			mysqlDB.QueryRowContext(ctx, `SELECT COALESCE(ps.setting_value, CONCAT('Submission #', ?)) FROM submissions s JOIN publications pub ON pub.submission_id = s.submission_id LEFT JOIN publication_settings ps ON ps.publication_id = pub.publication_id AND ps.setting_name = 'title' WHERE s.submission_id = ? LIMIT 1`, sfSubmissionID, sfSubmissionID).Scan(&articleTitle)

			// Get author
			var authorName string
			mysqlDB.QueryRowContext(ctx, `SELECT COALESCE(CONCAT(pa.given_name, ' ', pa.family_name), 'Unknown') FROM publication_authors pa WHERE pa.submission_id = ? LIMIT 1`, sfSubmissionID).Scan(&authorName)

			relations = append(relations, map[string]interface{}{
				"type": "submission_file",
				"file_id": sfFileID,
				"original_name": sfOriginalName,
				"submission_id": sfSubmissionID,
				"article_title": articleTitle,
				"author_name": authorName,
				"uploader_user_id": sfUploaderID,
				"uploader_name": sfUploaderName,
				"uploader_email": sfUploaderEmail,
				"file_type": sfFileType,
				"date_uploaded": sfDateUploaded,
				"stage_id": sfStageID,
				"review_round": sfReviewRound,
				"revision": sfRevision,
			})
		}
	}

	if relations == nil {
		relations = []map[string]interface{}{}
	}

	result["relations"] = relations
	respondJSON(w, http.StatusOK, result)
}

func handleGetFIMStats(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Format("2006-01-02")
	monthStart := now.Format("2006-01") + "-01"

	stats := map[string]interface{}{
		"today":      map[string]interface{}{"added": 0, "modified": 0, "deleted": 0, "total": 0},
		"this_week":  map[string]interface{}{"added": 0, "modified": 0, "deleted": 0, "total": 0},
		"this_month": map[string]interface{}{"added": 0, "modified": 0, "deleted": 0, "total": 0},
		"all_time":   map[string]interface{}{"added": 0, "modified": 0, "deleted": 0, "orphan": 0, "total": 0},
	}

	// Today
	for _, status := range []string{"ADDED", "MODIFIED", "DELETED"} {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND status = ? AND date(created_at, 'unixepoch', 'localtime') >= ?`, id, status, today).Scan(&count)
		stats["today"].(map[string]interface{})[strings.ToLower(status)] = count
	}
	var todayTotal int
	db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND date(created_at, 'unixepoch', 'localtime') >= ?`, id, today).Scan(&todayTotal)
	stats["today"].(map[string]interface{})["total"] = todayTotal

	// This week
	for _, status := range []string{"ADDED", "MODIFIED", "DELETED"} {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND status = ? AND date(created_at, 'unixepoch', 'localtime') >= ?`, id, status, weekStart).Scan(&count)
		stats["this_week"].(map[string]interface{})[strings.ToLower(status)] = count
	}
	var weekTotal int
	db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND date(created_at, 'unixepoch', 'localtime') >= ?`, id, weekStart).Scan(&weekTotal)
	stats["this_week"].(map[string]interface{})["total"] = weekTotal

	// This month
	for _, status := range []string{"ADDED", "MODIFIED", "DELETED"} {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND status = ? AND date(created_at, 'unixepoch', 'localtime') >= ?`, id, status, monthStart).Scan(&count)
		stats["this_month"].(map[string]interface{})[strings.ToLower(status)] = count
	}
	var monthTotal int
	db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND date(created_at, 'unixepoch', 'localtime') >= ?`, id, monthStart).Scan(&monthTotal)
	stats["this_month"].(map[string]interface{})["total"] = monthTotal

	// All time
	for _, status := range []string{"ADDED", "MODIFIED", "DELETED", "ORPHAN"} {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ? AND status = ?`, id, status).Scan(&count)
		stats["all_time"].(map[string]interface{})[strings.ToLower(status)] = count
	}
	var allTotal int
	db.QueryRow(`SELECT COUNT(*) FROM project_files WHERE project_id = ?`, id).Scan(&allTotal)
	stats["all_time"].(map[string]interface{})["total"] = allTotal

	respondJSON(w, http.StatusOK, stats)
}

func handleTestConnection(w http.ResponseWriter, r *http.Request) {
	var p Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	if len(p.AppPaths) == 0 || (len(p.AppPaths) == 1 && p.AppPaths[0] == "") {
		respondError(w, http.StatusBadRequest, "Minimal satu App Path wajib diisi")
		return
	}
	if p.DBHost == "" || p.DBUser == "" || p.DBName == "" {
		respondError(w, http.StatusBadRequest, "Konfigurasi Database tidak lengkap")
		return
	}
	
	// Check Paths
	for _, ap := range p.AppPaths {
		if ap != "" {
			if _, err := os.Stat(ap); os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("App path not found: %s", ap))
				return
			}
		}
	}
	for _, fp := range p.FilesPaths {
		if fp != "" {
			if _, err := os.Stat(fp); os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Files path not found: %s", fp))
				return
			}
		}
	}
	
	// Check DB
	if err := testDBConnection(p); err != nil {
		respondError(w, http.StatusBadRequest, "Database connection failed: " + err.Error())
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Connection and paths are valid"})
}

func handleCancelJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	jobID, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	// NOTE: Currently all authenticated admins can cancel any job.
	// If multi-tenant support is needed, add project ownership check here.

	// Check job status - only allow cancelling queued jobs
	var currentStatus string
	var projectID int
	err = db.QueryRow("SELECT status, project_id FROM jobs WHERE id = ?", jobID).Scan(&currentStatus, &projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	if currentStatus != "queued" {
		respondError(w, http.StatusConflict, "Only queued jobs can be cancelled")
		return
	}

	// Delete the queued job
	result, err := db.Exec("DELETE FROM jobs WHERE id = ? AND status = 'queued'", jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to cancel job: "+err.Error())
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Job not found or already cancelled")
		return
	}

	// Check if there are any remaining queued jobs for this project
	var remainingQueued int
	if err := db.QueryRow("SELECT COUNT(*) FROM jobs WHERE project_id = ? AND status = 'queued'", projectID).Scan(&remainingQueued); err != nil {
		log.Printf("Warning: failed to check remaining queued jobs: %v", err)
	}

	// If no more queued jobs, revert project status back
	if remainingQueued == 0 {
		// Check if there's a running job
		var runningCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM jobs WHERE project_id = ? AND status = 'running'", projectID).Scan(&runningCount); err != nil {
			log.Printf("Warning: failed to check running jobs: %v", err)
		}

		if runningCount == 0 {
			// No running job either, set status back based on configuration
			var p Project
			if err := db.QueryRow("SELECT id, name, description, template, app_path, files_path, db_host, db_user, db_name FROM projects WHERE id = ?", projectID).Scan(
				&p.ID, &p.Name, &p.Description, &p.Template, new(string), new(string), &p.DBHost, &p.DBUser, &p.DBName,
			); err != nil {
				log.Printf("Warning: failed to get project for status revert: %v", err)
			}

			// Get current project status
			var currentProjectStatus string
			db.QueryRow("SELECT status FROM projects WHERE id = ?", projectID).Scan(&currentProjectStatus)

			// If project was in pending/counting/scanning state, revert to pending_baseline
			if currentProjectStatus == "pending_baseline" || currentProjectStatus == "counting" || currentProjectStatus == "scanning" || currentProjectStatus == "reconciling" {
				db.Exec("UPDATE projects SET status = 'pending_baseline' WHERE id = ?", projectID)
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// handleGetFIMEvents returns FIM forensic events with pagination and filters
func handleGetFIMEvents(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Get query params
	eventType := r.URL.Query().Get("type")
	riskLevel := r.URL.Query().Get("risk_level")
	classification := r.URL.Query().Get("classification")
	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	// Build query
	baseConditions := "WHERE project_id = ?"
	args := []interface{}{projectID}
	countArgs := []interface{}{projectID}

	if eventType != "" && eventType != "all" {
		baseConditions += " AND event_type = ?"
		args = append(args, eventType)
		countArgs = append(countArgs, eventType)
	}

	if riskLevel != "" && riskLevel != "all" {
		baseConditions += " AND risk_level = ?"
		args = append(args, riskLevel)
		countArgs = append(countArgs, riskLevel)
	}

	if classification != "" && classification != "all" {
		baseConditions += " AND classification = ?"
		args = append(args, classification)
		countArgs = append(countArgs, classification)
	}

	if search != "" {
		baseConditions += " AND (file_path LIKE ? OR actor_name LIKE ?)"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
		countArgs = append(countArgs, pattern, pattern)
	}

	countQuery := "SELECT COUNT(*) FROM fim_events " + baseConditions
	var total int
	db.QueryRow(countQuery, countArgs...).Scan(&total)

	query := "SELECT id, project_id, COALESCE(file_id, 0), event_type, file_path, file_hash, actor_type, actor_id, actor_name, actor_details, risk_level, classification, source, details, alert_sent, datetime(timestamp, 'unixepoch'), datetime(created_at, 'unixepoch') FROM fim_events " + baseConditions + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var events []FIMEvent
	for rows.Next() {
		var e FIMEvent
		var actorID, actorName, actorDetails, details sql.NullString
		var alertSentInt int
		var timestamp, createdAt sql.NullString
		var fileID int
		rows.Scan(&e.ID, &e.ProjectID, &fileID, &e.EventType, &e.FilePath, &e.FileHash, &e.ActorType, &actorID, &actorName, &actorDetails, &e.RiskLevel, &e.Classification, &e.Source, &details, &alertSentInt, &timestamp, &createdAt)
		if fileID > 0 {
			e.FileID = &fileID
		}

		if actorID.Valid {
			e.ActorID = actorID.String
		}
		if actorName.Valid {
			e.ActorName = actorName.String
		}
		if actorDetails.Valid {
			e.ActorDetails = actorDetails.String
		}
		if details.Valid {
			e.Details = details.String
		}
		e.AlertSent = alertSentInt == 1
		if timestamp.Valid {
			e.Timestamp = timestamp.String
		}
		if createdAt.Valid {
			e.CreatedAt = createdAt.String
		}

		events = append(events, e)
	}
	if events == nil {
		events = []FIMEvent{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"pagination": map[string]interface{}{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

// handleGetFIMEventStats returns FIM event statistics
func handleGetFIMEventStats(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	weekStart := dayStart - int64(now.Weekday())*86400
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()

	type Stats struct {
		Events         int `json:"events"`
		HighRisk       int `json:"high_risk"`
		CriticalRisk   int `json:"critical_risk"`
		UnknownSource  int `json:"unknown_source"`
		AlertsSent     int `json:"alerts_sent"`
	}

	// All time
	var allStats Stats
	db.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN risk_level IN ('HIGH', 'CRITICAL') THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN risk_level = 'CRITICAL' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN classification = 'UNKNOWN_SOURCE' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN alert_sent = 1 THEN 1 ELSE 0 END), 0) FROM fim_events WHERE project_id = ?", projectID).Scan(&allStats.Events, &allStats.HighRisk, &allStats.CriticalRisk, &allStats.UnknownSource, &allStats.AlertsSent)

	// This month
	var monthStats Stats
	db.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN risk_level IN ('HIGH', 'CRITICAL') THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN risk_level = 'CRITICAL' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN classification = 'UNKNOWN_SOURCE' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN alert_sent = 1 THEN 1 ELSE 0 END), 0) FROM fim_events WHERE project_id = ? AND timestamp >= ?", projectID, monthStart).Scan(&monthStats.Events, &monthStats.HighRisk, &monthStats.CriticalRisk, &monthStats.UnknownSource, &monthStats.AlertsSent)

	// This week
	var weekStats Stats
	db.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN risk_level IN ('HIGH', 'CRITICAL') THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN risk_level = 'CRITICAL' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN classification = 'UNKNOWN_SOURCE' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN alert_sent = 1 THEN 1 ELSE 0 END), 0) FROM fim_events WHERE project_id = ? AND timestamp >= ?", projectID, weekStart).Scan(&weekStats.Events, &weekStats.HighRisk, &weekStats.CriticalRisk, &weekStats.UnknownSource, &weekStats.AlertsSent)

	// Today
	var todayStats Stats
	db.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN risk_level IN ('HIGH', 'CRITICAL') THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN risk_level = 'CRITICAL' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN classification = 'UNKNOWN_SOURCE' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN alert_sent = 1 THEN 1 ELSE 0 END), 0) FROM fim_events WHERE project_id = ? AND timestamp >= ?", projectID, dayStart).Scan(&todayStats.Events, &todayStats.HighRisk, &todayStats.CriticalRisk, &todayStats.UnknownSource, &todayStats.AlertsSent)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"all_time":    allStats,
		"this_month":  monthStats,
		"this_week":   weekStats,
		"today":       todayStats,
	})
}

// handleStartFIMWatcher starts the FIM file watcher for a project
func handleStartFIMWatcher(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	// Get project paths
	p, err := getProjectByID(projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Combine app_paths and files_paths for watching
	var watchPaths []string
	for _, ap := range p.AppPaths {
		if ap != "" {
			watchPaths = append(watchPaths, ap)
		}
	}
	for _, fp := range p.FilesPaths {
		if fp != "" {
			watchPaths = append(watchPaths, fp)
		}
	}

	if len(watchPaths) == 0 {
		respondError(w, http.StatusBadRequest, "No paths configured for watching")
		return
	}

	// Check if inotifywait is available
	if _, err := exec.LookPath("inotifywait"); err != nil {
		respondError(w, http.StatusServiceUnavailable, "inotifywait not installed. Install with: apt-get install inotify-tools")
		return
	}

	// Start watcher
	if err := StartFIMWatcher(projectID, watchPaths); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start watcher: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("FIM watcher started for project %d", projectID),
		"paths":   watchPaths,
	})
}

// handleStopFIMWatcher stops the FIM file watcher for a project
func handleStopFIMWatcher(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	if err := StopFIMWatcherForProject(projectID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to stop watcher: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("FIM watcher stopped for project %d", projectID),
	})
}

// handleGetFIMWatcherStatus returns the current watcher status for a project
func handleGetFIMWatcherStatus(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	isRunning := IsWatcherRunningForProject(projectID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"running": isRunning,
		"project_id": projectID,
		"message": map[string]interface{}{
			"active": isRunning,
			"status": map[string]bool{
				"running": isRunning,
				"stopped": !isRunning,
			},
		},
	})
}
