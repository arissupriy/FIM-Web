// Package models contains all domain models for OJS Monitor.
// These models are pure data structures with no external dependencies.
package models

import (
	"encoding/json"
	"strings"
)

// Project represents an OJS monitoring project
type Project struct {
	ID                      int      `json:"id"`
	Name                    string   `json:"name"`
	Description             string   `json:"description"`
	Template                string   `json:"template"`
	AppPaths                []string `json:"app_paths"`
	FilesPaths              []string `json:"files_paths"`
	BlacklistExts           []string `json:"blacklist_exts"`
	WhitelistPaths          []string `json:"whitelist_paths"`
	DBHost                 string   `json:"db_host"`
	DBUser                 string   `json:"db_user"`
	DBPass                 string   `json:"db_pass"`
	DBName                 string   `json:"db_name"`
	Status                 string   `json:"status"`
	BaselineTotal          int      `json:"baseline_total"`
	BaselineProcessed      int      `json:"baseline_processed"`
	BaselineAt             int64    `json:"baseline_at"`
	WatcherStatus          string   `json:"watcher_status"`
	IntegrityScanEnabled  int      `json:"integrity_scan_enabled"`
	LastIntegrityScan      int64    `json:"last_integrity_scan"`
	ErrorMessage           string   `json:"error_message"`
	Configured             bool     `json:"configured"`
	RescanInterval         int      `json:"rescan_interval"`
}

// IsConfigured checks if project has all required fields filled
func (p *Project) IsConfigured() bool {
	if p.DBHost == "" || p.DBUser == "" || p.DBName == "" {
		return false
	}
	if len(p.AppPaths) == 0 || p.AppPaths[0] == "" {
		return false
	}
	return true
}

// ToJSON serializes project to JSON
func (p *Project) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// FromJSON deserializes project from JSON
func (p *Project) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

// Job represents a background scan job
type Job struct {
	ID           int    `json:"id"`
	ProjectID   int    `json:"project_id"`
	Type        string `json:"type"` // initial_baseline, integrity_scan
	Status      string `json:"status"` // queued, running, done, failed
	ErrorMsg    string `json:"error_message,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	FinishedAt  string `json:"finished_at,omitempty"`
	FilesSuccess int   `json:"files_success,omitempty"`
	FilesSkipped  int  `json:"files_skipped,omitempty"`
	FilesError    int  `json:"files_error,omitempty"`
}

// IsQueued returns true if job is in queued status
func (j *Job) IsQueued() bool {
	return j.Status == "queued"
}

// IsRunning returns true if job is currently running
func (j *Job) IsRunning() bool {
	return j.Status == "running"
}

// IsFinished returns true if job is done or failed
func (j *Job) IsFinished() bool {
	return j.Status == "done" || j.Status == "failed"
}

// ProjectFile represents a tracked file in the filesystem
type ProjectFile struct {
	ID                 int    `json:"id"`
	ProjectID          int    `json:"project_id"`
	FilePath           string `json:"file_path"`
	Hash              string `json:"hash"`
	FileSize          int64  `json:"file_size"`
	ModTime           int64  `json:"mod_time"`
	Status            string `json:"status"` // ADDED, MODIFIED, DELETED, ORPHAN
	FileType          string `json:"file_type"` // project, uploads
	FileMode          string `json:"file_mode"` // Octal like "0644"
	FileUID           uint32 `json:"file_uid"`
	FileGID           uint32 `json:"file_gid"`
	PermissionChanges int    `json:"permission_changes"`
	CreatedAt         int64  `json:"created_at,omitempty"`
	UpdatedAt         int64  `json:"updated_at,omitempty"`
}

// IsOrphan returns true if file is flagged as orphan
func (f *ProjectFile) IsOrphan() bool {
	return f.Status == "ORPHAN"
}

// IsModified returns true if file has been modified
func (f *ProjectFile) IsModified() bool {
	return f.Status == "MODIFIED"
}

// FIMEvent represents a file integrity monitoring event
type FIMEvent struct {
	ID             int    `json:"id"`
	ProjectID     int    `json:"project_id"`
	EventType     string `json:"event_type"` // CREATED, MODIFIED, DELETED
	FilePath      string `json:"file_path"`
	FileHash      string `json:"file_hash,omitempty"`
	ActorType     string `json:"actor_type,omitempty"` // OJS_USER, SYSTEM_USER, PROCESS
	ActorID       string `json:"actor_id,omitempty"`
	ActorName     string `json:"actor_name,omitempty"`
	ActorDetails  string `json:"actor_details,omitempty"`
	RiskLevel     string `json:"risk_level"` // LOW, MEDIUM, HIGH, CRITICAL
	Classification string `json:"classification"` // TRUSTED, UNKNOWN_SOURCE, MODIFIED, DELETED
	Source        string `json:"source"` // WATCHER, RESCAN
	Details       string `json:"details,omitempty"`
	AlertSent     bool   `json:"alert_sent"`
	Timestamp     string `json:"timestamp"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// IsHighRisk returns true if event has HIGH or CRITICAL risk
func (e *FIMEvent) IsHighRisk() bool {
	return e.RiskLevel == "HIGH" || e.RiskLevel == "CRITICAL"
}

// IsUnknownSource returns true if file source is unknown
func (e *FIMEvent) IsUnknownSource() bool {
	return e.Classification == "UNKNOWN_SOURCE"
}

// FIMWatchPath represents a watched path configuration
type FIMWatchPath struct {
	ID            int    `json:"id"`
	ProjectID     int    `json:"project_id"`
	Path         string `json:"path"`
	WatchType    string `json:"watch_type"` // OJS_WORKFLOW, SYSTEM, NONE
	Enabled      bool   `json:"enabled"`
	AlertOnUnknown bool `json:"alert_on_unknown"`
	AlertLevel    string `json:"alert_level"` // LOW, MEDIUM, HIGH, CRITICAL
}

// Admin represents an admin user
type Admin struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        int    `json:"id"`
	AdminID   int    `json:"admin_id"`
	AdminName string `json:"admin_name"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Timestamp string `json:"timestamp"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewSuccessResponse creates a successful API response
func NewSuccessResponse(data interface{}) APIResponse {
	return APIResponse{Success: true, Data: data}
}

// NewErrorResponse creates an error API response
func NewErrorResponse(err string) APIResponse {
	return APIResponse{Success: false, Error: err}
}

// DashboardMetrics holds audit metrics for a project
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
	UploadsByNewUsers  int    `json:"uploads_by_new_users"`
	ActiveAdmins       int    `json:"active_admins"`
	BadSelfReg         int    `json:"bad_self_reg"`
}

// OJSDetails holds OJS-specific information
type OJSDetails struct {
	Version           string `json:"version"`
	Jurournals        int    `json:"journals"`
	Users            int    `json:"users"`
	Submissions      int    `json:"submissions"`
	Articles         int    `json:"articles"`
	ReviewAssignments int    `json:"review_assignments"`
	PrimaryLocale    string `json:"primary_locale"`
	InstalledLocales string `json:"installed_locales"`
	MinPasswordLen   int    `json:"min_password_len"`
}

// FileMetadata holds file hash and permission information
type FileMetadata struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	Mode string `json:"mode"` // Octal like "0644"
	UID  uint32 `json:"uid"`
	GID  uint32 `json:"gid"`
}

// ScanResult holds the result of a scan operation
type ScanResult struct {
	TotalFiles     int
	ProcessedFiles int
	AddedFiles     int
	ModifiedFiles  int
	DeletedFiles   int
	OrphanFiles    int
	SkippedFiles   int
	ErrorFiles     int
	Duration       string
	Status         string
	Error          error
}

// HasErrors returns true if scan had any errors
func (s *ScanResult) HasErrors() bool {
	return s.ErrorFiles > 0 || s.Error != nil
}

// SuccessRate calculates the success rate of the scan
func (s *ScanResult) SuccessRate() float64 {
	if s.TotalFiles == 0 {
		return 100.0
	}
	return float64(s.ProcessedFiles) / float64(s.TotalFiles) * 100
}

// NormalizePath cleans and normalizes a file path
func NormalizePath(path string) string {
	return strings.TrimSpace(path)
}

// SafePath checks if a path is safe (no traversal)
func SafePath(path string) bool {
	return !strings.Contains(path, "..")
}
