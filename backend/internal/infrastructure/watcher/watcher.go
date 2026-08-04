// Package watcher provides real-time file integrity monitoring using inotifywait.
package watcher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/wire"
)

// FIM Watcher Configuration
const (
	EventBufferSize       = 1000   // Max events in buffer per project
	BatchProcessInterval  = 1 * time.Second // Process events every 1 second
	DebounceWindow        = 500 * time.Millisecond // Ignore duplicate events within this window
	OJSLookupTimeout      = 5 * time.Second // Timeout for OJS database lookup
)

// ProjectWatcher - state for each project's watcher
type ProjectWatcher struct {
	ProjectID    int
	Paths        []string
	EventChannel chan FIMEventRaw
	Ctx          context.Context
	Cancel       context.CancelFunc
	WG           sync.WaitGroup
	IsRunning    bool
	Mu           sync.RWMutex
}

// FIM Watcher Global State
var (
	globalWatcherCtx, globalWatcherCancel = context.WithCancel(context.Background())
	projectWatchers = make(map[int]*ProjectWatcher) // map[projectID] -> watcher
	watchersMutex  sync.RWMutex
	seenFilesPerProject = make(map[int]*sync.Map) // map[projectID] -> seenFiles
)

// FIMEventRaw - raw event from inotifywait
type FIMEventRaw struct {
	Path      string
	EventType string // CREATE, MODIFY, DELETE, MOVED_TO, MOVED_FROM
	Timestamp time.Time
	ProjectID int
	WatchPath string
}

// FIMEventActor - actor information
type FIMEventActor struct {
	Type      string `json:"type"` // OJS_USER, SYSTEM_USER, PROCESS, UNKNOWN
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Details   string `json:"details,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	Process   string `json:"process,omitempty"`
	UID       string `json:"uid,omitempty"`
}

// OJSInfo - OJS correlation result
type OJSInfo struct {
	Found          bool   `json:"found"`
	UserID         int    `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	FullName       string `json:"full_name,omitempty"`
	Email          string `json:"email,omitempty"`
	Role           string `json:"role,omitempty"`
	SubmissionID   int    `json:"submission_id,omitempty"`
	Classification string `json:"classification"`
	RiskLevel      string `json:"risk_level"`
	Reason         string `json:"reason,omitempty"`
}

// FileMetadata holds file hash and permission information
type FileMetadata struct {
	Hash    string
	Size    int64
	Mode    string  // Octal like "0644"
	UID     uint32
	GID     uint32
}

// FIMEvent - internal event structure for processing
type FIMEvent struct {
	ProjectID      int
	EventType      string
	FilePath       string
	FileHash       string
	ActorType      string
	ActorID        string
	ActorName      string
	ActorDetails   string
	RiskLevel      string
	Classification string
	Source         string
	Details        string
	Timestamp      string
}

// StartFIMWatcher starts the file system watcher for a project
func StartFIMWatcher(projectID int, paths []string) error {
	watchersMutex.Lock()
	defer watchersMutex.Unlock()

	// Check if already running for this project
	if pw, exists := projectWatchers[projectID]; exists {
		pw.Mu.Lock()
		if pw.IsRunning {
			pw.Mu.Unlock()
			log.Printf("Watcher already running for project %d\n", projectID)
			return nil
		}
		pw.Mu.Unlock()
	}

	if len(paths) == 0 {
		log.Printf("No paths configured for project %d\n", projectID)
		return fmt.Errorf("no paths configured")
	}

	// Validate paths exist
	validPaths := []string{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("Watch path does not exist: %s\n", path)
			continue
		}
		validPaths = append(validPaths, path)
	}

	if len(validPaths) == 0 {
		return fmt.Errorf("no valid paths to watch")
	}

	// Create new watcher for this project
	ctx, cancel := context.WithCancel(globalWatcherCtx)
	pw := &ProjectWatcher{
		ProjectID:    projectID,
		Paths:        validPaths,
		EventChannel: make(chan FIMEventRaw, EventBufferSize),
		Ctx:          ctx,
		Cancel:       cancel,
		IsRunning:    true,
	}

	projectWatchers[projectID] = pw
	seenFilesPerProject[projectID] = &sync.Map{}

	// Update database status
	dbCtx := context.Background()
	wire.UpdateWatcherStatus(dbCtx, projectID, "running")

	// Start event processor for this project
	pw.WG.Add(1)
	go processFIMEventsForProject(pw)

	// Start inotify watcher for each path
	for _, path := range validPaths {
		pw.WG.Add(1)
		go watchPathForProject(pw, path)
	}

	log.Printf("FIM Watcher started for project %d with %d paths\n", projectID, len(validPaths))
	return nil
}

// StopFIMWatcherForProject stops the watcher for a specific project
func StopFIMWatcherForProject(projectID int) error {
	watchersMutex.Lock()
	defer watchersMutex.Unlock()

	dbCtx := context.Background()

	pw, exists := projectWatchers[projectID]
	if !exists || !pw.IsRunning {
		log.Printf("Watcher not running for project %d\n", projectID)
		return nil
	}

	log.Printf("Stopping FIM Watcher for project %d...\n", projectID)
	pw.Mu.Lock()
	pw.IsRunning = false
	pw.Mu.Unlock()

	pw.Cancel()

	// Close channel to signal processors
	if pw.EventChannel != nil {
		close(pw.EventChannel)
	}

	// Wait for all watchers to finish
	pw.WG.Wait()

	// Cleanup
	delete(projectWatchers, projectID)
	delete(seenFilesPerProject, projectID)

	// Update database status
	wire.UpdateWatcherStatus(dbCtx, projectID, "stopped")

	log.Printf("FIM Watcher stopped for project %d\n", projectID)
	return nil
}

// StopAllFIMWatchers stops all file watchers
func StopAllFIMWatchers() {
	watchersMutex.Lock()
	defer watchersMutex.Unlock()

	log.Println("Stopping all FIM Watchers...")
	globalWatcherCancel()

	for projectID, pw := range projectWatchers {
		pw.Mu.Lock()
		pw.IsRunning = false
		pw.Mu.Unlock()

		pw.Cancel()
		if pw.EventChannel != nil {
			close(pw.EventChannel)
		}
		pw.WG.Wait()

		wire.UpdateWatcherStatus(context.Background(), projectID, "stopped")
		delete(projectWatchers, projectID)
		delete(seenFilesPerProject, projectID)
	}

	log.Println("All FIM Watchers stopped")
}

// GetWatcherStatus returns status for all watchers
func GetWatcherStatus() map[int]bool {
	watchersMutex.RLock()
	defer watchersMutex.RUnlock()

	status := make(map[int]bool)
	for projectID, pw := range projectWatchers {
		pw.Mu.RLock()
		status[projectID] = pw.IsRunning
		pw.Mu.RUnlock()
	}
	return status
}

// IsWatcherRunningForProject returns true if watcher is running for project
func IsWatcherRunningForProject(projectID int) bool {
	watchersMutex.RLock()
	defer watchersMutex.RUnlock()

	if pw, exists := projectWatchers[projectID]; exists {
		pw.Mu.RLock()
		defer pw.Mu.RUnlock()
		return pw.IsRunning
	}
	return false
}

// watchPathForProject watches a single path using inotifywait
func watchPathForProject(pw *ProjectWatcher, path string) {
	defer pw.WG.Done()

	args := []string{
		"-m",       // Monitor mode (continuous)
		"-r",       // Recursive
		"-q",       // Quiet
		"--format", // Output format: path|event|epoch_timestamp
		"%w%f|%e|%W",
		"--timefmt", // Required but not used with %W
		"%s",
		path,
	}

	cmd := exec.CommandContext(pw.Ctx, "inotifywait", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe for %s: %v\n", path, err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start inotifywait for %s: %v\n", path, err)
		return
	}

	reader := io.Reader(stdout)
	buf := make([]byte, 4096)

	for {
		select {
		case <-pw.Ctx.Done():
			cmd.Process.Kill()
			cmd.Wait()
			return
		default:
		}

		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		line := strings.TrimSpace(string(buf[:n]))
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}

		filePath := parts[0]
		eventType := parts[1]
		timestamp := parts[2]

		// Convert inotify event to FIM event type
		fimEventType := convertInotifyEvent(eventType)
		if fimEventType == "" {
			continue
		}

		// Parse timestamp (epoch seconds from inotifywait with %W)
		ts := time.Now()
		tsUnix, err := strconv.ParseInt(timestamp, 10, 64)
		if err == nil {
			ts = time.Unix(tsUnix, 0)
		}

		rawEvent := FIMEventRaw{
			Path:      filePath,
			EventType: fimEventType,
			Timestamp: ts,
			ProjectID: pw.ProjectID,
			WatchPath: path,
		}

		// Non-blocking send to channel
		select {
		case pw.EventChannel <- rawEvent:
		default:
			// Buffer full, log warning
			log.Printf("Warning: FIM event buffer full for project %d, dropping event for %s\n", pw.ProjectID, filePath)
		}
	}
}

// convertInotifyEvent converts inotify event types to FIM event types
func convertInotifyEvent(inotifyEvent string) string {
	switch inotifyEvent {
	case "CREATE":
		return "CREATED"
	case "MODIFY", "CLOSE_WRITE":
		return "MODIFIED"
	case "DELETE", "MOVED_FROM":
		return "DELETED"
	case "MOVED_TO":
		return "CREATED"
	default:
		return ""
	}
}

// processFIMEventsForProject processes events in batches for a specific project
func processFIMEventsForProject(pw *ProjectWatcher) {
	defer pw.WG.Done()

	ticker := time.NewTicker(BatchProcessInterval)
	defer ticker.Stop()

	var eventBatch []FIMEventRaw

	for {
		select {
		case <-pw.Ctx.Done():
			// Process remaining events before exit
			if len(eventBatch) > 0 {
				persistFIMEvents(pw.ProjectID, eventBatch)
			}
			return
		case event := <-pw.EventChannel:
			// Debounce: check if we've seen this file recently
			if shouldDebounce(pw.ProjectID, event.Path, event.EventType) {
				continue
			}
			eventBatch = append(eventBatch, event)
		case <-ticker.C:
			// Process batch
			if len(eventBatch) > 0 {
				persistFIMEvents(pw.ProjectID, eventBatch)
				eventBatch = eventBatch[:0]
			}
		}
	}
}

// shouldDebounce checks if event should be ignored due to duplicate within window
// Uses sync.Map with time-based cleanup to prevent goroutine leaks
func shouldDebounce(projectID int, path, eventType string) bool {
	seenFiles, exists := seenFilesPerProject[projectID]
	if !exists {
		seenFiles = &sync.Map{}
		seenFilesPerProject[projectID] = seenFiles
	}

	key := path + "|" + eventType

	// Check if recently seen
	if _, exists := seenFiles.Load(key); exists {
		return true
	}

	// Store with timestamp for cleanup
	seenFiles.Store(key, time.Now())

	// Periodic cleanup of old entries (every DebounceWindow * 2)
	// This replaces the fire-and-forget goroutine approach
	seenFilesPerProject[projectID] = seenFiles

	return false
}

// cleanupDebounceCache removes old entries from debounce cache
// Called periodically to prevent memory growth
func cleanupDebounceCache(projectID int) {
	seenFiles, exists := seenFilesPerProject[projectID]
	if !exists {
		return
	}

	cutoff := time.Now().Add(-DebounceWindow * 2)
	seenFiles.Range(func(key, value interface{}) bool {
		if ts, ok := value.(time.Time); ok {
			if ts.Before(cutoff) {
				seenFiles.Delete(key)
			}
		}
		return true
	})
}

// persistFIMEvents processes and stores batch of events
func persistFIMEvents(projectID int, events []FIMEventRaw) {
	if len(events) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, event := range events {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fimEvent := FIMEvent{
			ProjectID: projectID,
			EventType:    event.EventType,
			FilePath:      event.Path,
			Source:        "WATCHER",
			Timestamp:     event.Timestamp.Format(time.RFC3339),
		}

		// Get actor information
		actor := getActorInfo(event.Path, event.EventType)
		fimEvent.ActorType = actor.Type
		fimEvent.ActorID = actor.ID
		fimEvent.ActorName = actor.Name
		if actorJSON, err := json.Marshal(actor); err == nil {
			fimEvent.ActorDetails = string(actorJSON)
		}

		// Correlate with OJS database
		ojsInfo := correlateOJS(projectID, event.Path, event.EventType)
		fimEvent.Classification = ojsInfo.Classification
		fimEvent.RiskLevel = ojsInfo.RiskLevel

		if ojsInfo.Found {
			fimEvent.ActorType = "OJS_USER"
			fimEvent.ActorID = fmt.Sprintf("%d", ojsInfo.UserID)
			fimEvent.ActorName = ojsInfo.Username
			if actorJSON, err := json.Marshal(ojsInfo); err == nil {
				fimEvent.ActorDetails = string(actorJSON)
			}
		}

		// Get file metadata (hash and permissions) if it's a file (not directory)
		if event.EventType != "DELETED" {
			if meta, err := getFileMetadata(event.Path); err == nil && meta != nil {
				fimEvent.FileHash = meta.Hash
				fimEvent.Details = fmt.Sprintf(`{"size": %d, "hash": "%s", "mode": "%s", "uid": %d, "gid": %d}`, meta.Size, meta.Hash, meta.Mode, meta.UID, meta.GID)

				// [P1-05] Check for permission changes from baseline
				baselineFile := getBaselineFile(projectID, event.Path)
				if baselineFile != nil {
					if baselineFile.FileMode != meta.Mode || baselineFile.FileUID != meta.UID || baselineFile.FileGID != meta.GID {
						// Permission change detected
						fimEvent.RiskLevel = "HIGH"
						fimEvent.Classification = "PERMISSION_CHANGE"
						fimEvent.Details = fmt.Sprintf(`{"size": %d, "hash": "%s", "mode": "%s", "uid": %d, "gid": %d, "baseline_mode": "%s", "baseline_uid": %d, "baseline_gid": %d, "permission_change": true}`, meta.Size, meta.Hash, meta.Mode, meta.UID, meta.GID, baselineFile.FileMode, baselineFile.FileUID, baselineFile.FileGID)

						// Update permission_changes counter in baseline
						updatePermissionChanges(projectID, baselineFile.ID)
					}
				}
			}
		}

		// Store event
		storeFIMEvent(fimEvent)
	}
}

// BaselineFile represents file baseline for permission comparison
type BaselineFile struct {
	ID        int
	FileMode  string
	FileUID   uint32
	FileGID   uint32
}

// getBaselineFile retrieves baseline file record for permission comparison
func getBaselineFile(projectID int, filePath string) *BaselineFile {
	ctx := context.Background()
	pf, err := wire.GetBaselineFile(ctx, projectID, filePath)
	if err != nil {
		return nil
	}
	return &BaselineFile{
		ID:        pf.ID,
		FileMode:  pf.FileMode,
		FileUID:   pf.FileUID,
		FileGID:   pf.FileGID,
	}
}

// updatePermissionChanges increments permission change counter
func updatePermissionChanges(projectID int, fileID int) {
	wire.IncrementPermissionChanges(context.Background(), fileID, projectID)
}

// getActorInfo gets actor information from system
func getActorInfo(filePath string, eventType string) FIMEventActor {
	actor := FIMEventActor{Type: "UNKNOWN"}

	// Try to get UID and GID from stat using syscall
	if stat, err := os.Stat(filePath); err == nil {
		if sys := stat.Sys(); sys != nil {
			if stat_t, ok := sys.(*syscall.Stat_t); ok {
				actor.UID = fmt.Sprintf("%d", stat_t.Uid)
				// Also get GID if available
				_ = stat_t.Gid // Can be used later if needed
			}
		}
	}

	// Get current user info
	if output, err := exec.Command("whoami").Output(); err == nil {
		actor.Type = "SYSTEM_USER"
		actor.Name = strings.TrimSpace(string(output))
	}

	// Get current process info
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", os.Getpid())); err == nil {
		actor.Process = strings.ReplaceAll(string(cmdline), "\x00", " ")
	}

	return actor
}

// correlateOJS checks if file is from OJS workflow
// Pass projectID to get correct project config
func correlateOJS(projectID int, filePath string, eventType string) OJSInfo {
	result := OJSInfo{
		Classification: "UNKNOWN_SOURCE",
		RiskLevel:     "HIGH",
		Reason:        "File not found in OJS submission_files",
	}

	if eventType == "DELETED" {
		result.Classification = "DELETED"
		result.RiskLevel = "MEDIUM"
		result.Reason = "File deletion detected"
		return result
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return result
	}

	// Get file name
	fileName := filepath.Base(filePath)
	if fileName == "" {
		return result
	}

	// Try to correlate with OJS database
	ctx, cancel := context.WithTimeout(context.Background(), OJSLookupTimeout)
	defer cancel()

	// Get project config for DB connection
	p, err := wire.GetProjectByID(ctx, projectID)
	if err != nil {
		log.Printf("Failed to get OJS DB config: %v\n", err)
		result.Reason = "OJS database not configured"
		return result
	}

	// Connect to OJS database
	ojsDB, err := connectOJS(ctx, p.DBHost, p.DBUser, p.DBPass, p.DBName)
	if err != nil {
		log.Printf("Failed to connect to OJS DB: %v\n", err)
		result.Reason = "OJS database connection failed"
		return result
	}
	defer ojsDB.Close()

	// Query OJS for file info
	var userID int
	var username, email string
	var submissionID int

	query := `
		SELECT sf.uploader_user_id, COALESCE(u.username, ''),
		       COALESCE(u.email, ''), sf.submission_id
		FROM submission_files sf
		LEFT JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE sf.original_file_name = ?
		LIMIT 1
	`

	err = ojsDB.QueryRowContext(ctx, query, fileName).Scan(&userID, &username, &email, &submissionID)
	if err != nil {
		// File not in OJS - could be system file
		if strings.Contains(filePath, "/usageStats/") {
			result.Classification = "SYSTEM_GENERATED"
			result.RiskLevel = "LOW"
			result.Reason = "System-generated usage stats file"
		} else if strings.Contains(filePath, "/cache/") {
			result.Classification = "SYSTEM_GENERATED"
			result.RiskLevel = "LOW"
			result.Reason = "System cache file"
		} else if strings.Contains(filePath, "/temp/") {
			result.Classification = "SYSTEM_GENERATED"
			result.RiskLevel = "LOW"
			result.Reason = "Temporary file"
		} else {
			result.Classification = "UNKNOWN_SOURCE"
			result.RiskLevel = "HIGH"
			result.Reason = fmt.Sprintf("File '%s' not found in OJS submission_files", fileName)
		}
		return result
	}

	// Found in OJS
	result.Found = true
	result.UserID = userID
	result.Username = username
	result.Email = email
	result.SubmissionID = submissionID
	result.Classification = "OJS_WORKFLOW"
	result.RiskLevel = "LOW"
	result.Reason = "Matched OJS submission_files"

	return result
}

// getFileMetadata calculates SHA256 hash and captures file metadata including permissions
func getFileMetadata(filePath string) (*FileMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	meta := &FileMetadata{
		Size: stat.Size(),
		Mode: fmt.Sprintf("%04o", stat.Mode().Perm()), // Octal permission
	}

	// Get UID and GID from sys if available
	if sys := stat.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			meta.UID = stat.Uid
			meta.GID = stat.Gid
		}
	}

	// Skip large files (> 10MB) for hashing
	if stat.Size() > 10*1024*1024 {
		return meta, nil
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	meta.Hash = hex.EncodeToString(hash.Sum(nil))
	return meta, nil
}

// storeFIMEvent stores event in database
func storeFIMEvent(event FIMEvent) {
	ts := time.Now().Unix()
	if event.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, event.Timestamp); err == nil {
			ts = t.Unix()
		}
	}

	details := event.Details
	if details == "" {
		details = "{}"
	}

	// Convert to models.FIMEvent and store via wire
	fimEvent := &models.FIMEvent{
		ProjectID:     event.ProjectID,
		EventType:     event.EventType,
		FilePath:      event.FilePath,
		FileHash:      event.FileHash,
		ActorType:     event.ActorType,
		ActorID:       event.ActorID,
		ActorName:     event.ActorName,
		ActorDetails:  event.ActorDetails,
		RiskLevel:     event.RiskLevel,
		Classification: event.Classification,
		Source:        event.Source,
		Details:       details,
		Timestamp:     fmt.Sprintf("%d", ts),
	}

	if err := wire.CreateFIMEvent(context.Background(), fimEvent); err != nil {
		log.Printf("Failed to store FIM event: %v\n", err)
	}
}

// connectOJS connects to OJS MySQL database
func connectOJS(ctx context.Context, host, user, pass, dbname string) (*sql.DB, error) {
	return connectMySQLWithContext(ctx, user, pass, host, dbname)
}

// connectMySQLWithContext connects to MySQL with context support
func connectMySQLWithContext(ctx context.Context, user, pass, host, dbName string) (*sql.DB, error) {
	// Extract host and port if host includes port
	hostParts := strings.Split(host, ":")
	actualHost := hostParts[0]
	port := "3306"
	if len(hostParts) > 1 {
		port = hostParts[1]
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s",
		user, pass, actualHost, port, dbName)

	mysqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	mysqlDB.SetMaxOpenConns(2)
	mysqlDB.SetMaxIdleConns(1)
	mysqlDB.SetConnMaxLifetime(30 * time.Second)

	if err := mysqlDB.PingContext(ctx); err != nil {
		mysqlDB.Close()
		return nil, fmt.Errorf("ping failed: %v", err)
	}

	return mysqlDB, nil
}

// RestoreWatchersOnStartup restores watchers for all active projects
func RestoreWatchersOnStartup() {
	watchersMutex.Lock()
	defer watchersMutex.Unlock()

	log.Println("Restoring FIM watchers on startup...")

	ctx := context.Background()
	projects, err := wire.GetProjects(ctx)
	if err != nil {
		log.Printf("Failed to get projects: %v\n", err)
		return
	}

	for _, p := range projects {
		if p.Status == "active" && p.BaselineAt > 0 && p.WatcherStatus == "running" {
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

			if len(watchPaths) > 0 {
				// Create new watcher state (we need to release the mutex first)
				go func(projectID int, paths []string) {
					if err := StartFIMWatcher(projectID, paths); err != nil {
						log.Printf("Failed to restore watcher for project %d: %v\n", projectID, err)
					}
				}(p.ID, watchPaths)
			}
		}
	}

	log.Printf("Watcher restoration initiated for %d projects\n", len(projects))
}
