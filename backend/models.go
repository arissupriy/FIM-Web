package main

type Project struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Template       string   `json:"template"`
	AppPaths       []string `json:"app_paths"`
	FilesPaths     []string `json:"files_paths"`
	BlacklistExts  []string `json:"blacklist_exts"`
	WhitelistPaths []string `json:"whitelist_paths"`
	DBHost         string   `json:"db_host"`
	DBUser         string   `json:"db_user"`
	DBPass         string   `json:"db_pass"`
	DBName         string   `json:"db_name"`
	Status         string   `json:"status"`
	BaselineTotal     int      `json:"baseline_total"`
	BaselineProcessed int      `json:"baseline_processed"`
	BaselineAt        int64    `json:"baseline_at"`        // Timestamp when baseline was established
	WatcherStatus     string   `json:"watcher_status"`    // stopped, running, error
	IntegrityScanEnabled int   `json:"integrity_scan_enabled"` // Daily integrity scan
	LastIntegrityScan  int64    `json:"last_integrity_scan"`  // Last integrity scan timestamp
	ErrorMessage      string   `json:"error_message"`
	Configured     bool      `json:"configured"`        // Computed: true if DB and paths are set
	RescanInterval int      `json:"rescan_interval"`   // Auto-rescan interval in minutes (DEPRECATED: use integrity_scan_enabled)
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

type Job struct {
	ID           int
	ProjectID    int
	Type         string
	Status       string
	ErrorMessage string
	CreatedAt    string
	StartedAt    string
	FinishedAt   string
}

type DashboardMetrics struct {
	Status              string `json:"status"`
	BaselineTotal       int    `json:"baseline_total"`
	BaselineProcessed   int    `json:"baseline_processed"`
	ExecFilesCount      int    `json:"exec_files_count"`
	NewFilesCount       int `json:"new_files_count"`
	ModifiedFilesCount  int `json:"modified_files_count"`
	DeletedFilesCount   int `json:"deleted_files_count"`
	OrphanFilesCount    int `json:"orphan_files_count"`
	NewUsers            int `json:"new_users"`
	ValidatedUsers      int `json:"validated_users"`
	UnvalidatedDisabled int `json:"unvalidated_disabled"`
	UploadsByNewUsers   int `json:"uploads_by_new_users"`
	ActiveAdmins        int `json:"active_admins"`
	BadSelfReg          int `json:"bad_self_reg"`
}

type ProjectFile struct {
	ID        int
	ProjectID int
	FilePath  string
	Hash      string
	FileSize  int64
	ModTime   int64
	Status    string
	FileType  string // "project" or "uploads"
}

type FIMEvent struct {
	ID            int    `json:"id"`
	ProjectID     int    `json:"project_id"`
	EventType     string `json:"event_type"` // CREATED, MODIFIED, DELETED
	FilePath      string `json:"file_path"`
	FileHash      string `json:"file_hash,omitempty"`
	ActorType     string `json:"actor_type,omitempty"` // OJS_USER, SYSTEM_USER, PROCESS
	ActorID       string `json:"actor_id,omitempty"`
	ActorName     string `json:"actor_name,omitempty"`
	ActorDetails  string `json:"actor_details,omitempty"`
	RiskLevel     string `json:"risk_level"`    // LOW, MEDIUM, HIGH, CRITICAL
	Classification string `json:"classification"` // TRUSTED, UNKNOWN_SOURCE, MODIFIED, DELETED
	Source        string `json:"source"`        // WATCHER, RESCAN
	Details       string `json:"details,omitempty"`
	AlertSent     bool   `json:"alert_sent"`
	Timestamp     string `json:"timestamp"`
	CreatedAt     string `json:"created_at,omitempty"`
}

type FIMWatchPath struct {
	ID              int    `json:"id"`
	ProjectID       int    `json:"project_id"`
	Path            string `json:"path"`
	WatchType       string `json:"watch_type"` // OJS_WORKFLOW, SYSTEM, NONE
	Enabled         bool   `json:"enabled"`
	AlertOnUnknown  bool   `json:"alert_on_unknown"`
	AlertLevel      string `json:"alert_level"` // LOW, MEDIUM, HIGH, CRITICAL
}

type Admin struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

type AuditLog struct {
	ID        int    `json:"id"`
	AdminID   int    `json:"admin_id"`
	AdminName string `json:"admin_name"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Timestamp string `json:"timestamp"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
