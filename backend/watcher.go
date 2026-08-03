package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/arissupriy/ojs-monitor/backend/alerts"
	"github.com/fsnotify/fsnotify"
	_ "github.com/go-sql-driver/mysql"
)

// FIM Watcher Configuration (defaults, can be overridden via config)
var (
	EventBufferSize       = 1000   // Max events in buffer per project
	BatchProcessInterval = 1 * time.Second // Process events every 1 second
	DebounceWindow       = 500 * time.Millisecond // Ignore duplicate events within this window
	OJSLookupTimeout    = 5 * time.Second // Timeout for OJS database lookup
)

func initWatcherConfig() {
	EventBufferSize = cfg.FIMBufferSize
	BatchProcessInterval = time.Duration(cfg.FIMBatchIntervalMs) * time.Millisecond
	DebounceWindow = time.Duration(cfg.FIMDebounceMs) * time.Millisecond
	OJSLookupTimeout = time.Duration(cfg.FIMOJSLookupTimeoutMs) * time.Millisecond
}

// ProjectWatcher - state for each project's watcher
type ProjectWatcher struct {
	ProjectID    int
	Watcher      *fsnotify.Watcher
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
	projectWatchers = make(map[int]*ProjectWatcher)
	watchersMutex   sync.RWMutex
	seenFilesPerProject = make(map[int]*sync.Map)
)

// FIMEventRaw - raw event from fsnotify
type FIMEventRaw struct {
	Path      string
	EventType string // CREATED, MODIFIED, DELETED
	Timestamp time.Time
	ProjectID int
	WatchPath string
}

// FIMEventActor - actor information (NOTE: inotify-based tracking cannot detect actual actor)
type FIMEventActor struct {
	Type    string `json:"type"`
	Details string `json:"details,omitempty"`
}

// StartFIMWatcher starts the file system watcher for a project using fsnotify
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

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}

	// Create new watcher for this project
	ctx, cancel := context.WithCancel(globalWatcherCtx)
	pw := &ProjectWatcher{
		ProjectID:    projectID,
		Watcher:      watcher,
		Paths:        validPaths,
		EventChannel: make(chan FIMEventRaw, EventBufferSize),
		Ctx:          ctx,
		Cancel:       cancel,
		IsRunning:    true,
	}

	projectWatchers[projectID] = pw
	seenFilesPerProject[projectID] = &sync.Map{}

	// Update database status
	db.Exec("UPDATE projects SET watcher_status = 'running' WHERE id = ?", projectID)

	// Start event processor for this project
	pw.WG.Add(1)
	go processFIMEventsForProject(pw)

	// Start watching paths recursively
	pw.WG.Add(1)
	go watchWithFsnotify(pw)

	log.Printf("FIM Watcher (fsnotify) started for project %d with %d paths\n", projectID, len(validPaths))
	return nil
}

// watchWithFsnotify watches paths using native fsnotify with recursive support
func watchWithFsnotify(pw *ProjectWatcher) {
	defer pw.WG.Done()

	// Recursively add all directories to watch
	for _, rootPath := range pw.Paths {
		if err := addRecursiveWatch(pw.Watcher, rootPath); err != nil {
			log.Printf("Error adding recursive watch for %s: %v\n", rootPath, err)
		}
	}

	// Watch for new directories being created (separate goroutine with its ownWG)
	pw.WG.Add(1)
	go func() {
		defer pw.WG.Done()
		for {
			select {
			case <-pw.Ctx.Done():
				return
			case event := <-pw.Watcher.Events:
				// If a directory is created, add it to watch
				if event.Op&fsnotify.Create == fsnotify.Create {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						if err := pw.Watcher.Add(event.Name); err != nil {
							log.Printf("Failed to watch new directory %s: %v\n", event.Name, err)
						} else {
							log.Printf("Added watch for new directory: %s\n", event.Name)
						}
					}
				}
			case err := <-pw.Watcher.Errors:
				if err != nil {
					log.Printf("Watcher error: %v\n", err)
				}
			}
		}
	}()

	// Main event loop (this is the main watcher goroutine)
	for {
		select {
		case <-pw.Ctx.Done():
			return
		case event := <-pw.Watcher.Events:
			pw.handleFsnotifyEvent(event)
		}
	}
}

// addRecursiveWatch adds all subdirectories to the watcher
func addRecursiveWatch(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		if info.IsDir() {
			if err := watcher.Add(path); err != nil {
				log.Printf("Failed to watch %s: %v\n", path, err)
			}
		}
		return nil
	})
}

// handleFsnotifyEvent converts fsnotify event to FIM event
func (pw *ProjectWatcher) handleFsnotifyEvent(event fsnotify.Event) {
	// Skip if not relevant event type
	if event.Op&fsnotify.Create == 0 &&
		event.Op&fsnotify.Write == 0 &&
		event.Op&fsnotify.Remove == 0 &&
		event.Op&fsnotify.Rename == 0 {
		return
	}

	// Determine FIM event type
	var fimEventType string
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		// Check if it's a directory (will be handled separately)
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			return // Directory creation handled in watch loop
		}
		fimEventType = "CREATED"
	case event.Op&fsnotify.Write == fsnotify.Write:
		fimEventType = "MODIFIED"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		fimEventType = "DELETED"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		// Rename is essentially a delete of old + create of new
		fimEventType = "DELETED"
	default:
		return
	}

	// Debounce check
	if shouldDebounce(pw.ProjectID, event.Name, fimEventType) {
		return
	}

	rawEvent := FIMEventRaw{
		Path:      event.Name,
		EventType: fimEventType,
		Timestamp: time.Now(), // fsnotify doesn't provide event time, use current
		ProjectID: pw.ProjectID,
	}

	// Non-blocking send to channel
	select {
	case pw.EventChannel <- rawEvent:
	default:
		log.Printf("Warning: FIM event buffer full for project %d, dropping event for %s\n", pw.ProjectID, event.Name)
	}
}

// StopFIMWatcherForProject stops the watcher for a specific project
func StopFIMWatcherForProject(projectID int) error {
	watchersMutex.Lock()
	defer watchersMutex.Unlock()

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

	// Close watcher
	if pw.Watcher != nil {
		pw.Watcher.Close()
	}

	// Close channel
	if pw.EventChannel != nil {
		close(pw.EventChannel)
	}

	// Wait for goroutines
	pw.WG.Wait()

	// Cleanup
	delete(projectWatchers, projectID)
	delete(seenFilesPerProject, projectID)

	// Update database status
	db.Exec("UPDATE projects SET watcher_status = 'stopped' WHERE id = ?", projectID)

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
		if pw.Watcher != nil {
			pw.Watcher.Close()
		}
		if pw.EventChannel != nil {
			close(pw.EventChannel)
		}
		pw.WG.Wait()

		db.Exec("UPDATE projects SET watcher_status = 'stopped' WHERE id = ?", projectID)
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

// seenFilesCleanup tracks keys for debounce cleanup using a single timer
var seenFilesCleanup = make(chan string, 1000) // Buffered channel for cleanup requests

func init() {
	// Start single cleanup goroutine for all debounce entries
	go func() {
		ticker := time.NewTicker(DebounceWindow / 2) // Check every half window
		defer ticker.Stop()

		cleanupMap := make(map[string]time.Time)

		for {
			select {
			case key := <-seenFilesCleanup:
				cleanupMap[key] = time.Now().Add(DebounceWindow)
			case <-ticker.C:
				now := time.Now()
				for key, expiry := range cleanupMap {
					if now.After(expiry) {
						// Parse key to extract projectID
						parts := strings.SplitN(key, "|", 2)
						if len(parts) == 2 {
							deleteDebounceKey(key)
						}
						delete(cleanupMap, key)
					}
				}
			}
		}
	}()
}

// shouldDebounce checks if event should be ignored due to duplicate within window
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

	// Store with TTL - request cleanup
	seenFiles.Store(key, true)

	// Request cleanup (non-blocking)
	select {
	case seenFilesCleanup <- key:
		// Cleanup requested
	default:
		// Buffer full - cleanup will happen on next tick anyway
	}

	return false
}

// deleteDebounceKey deletes the key from seenFiles (called by cleanup goroutine)
// Note: This iterates through all project maps - acceptable since cleanup is infrequent
func deleteDebounceKey(key string) {
	seenFilesCleanup <- key // Put it back for the next cycle
	watchersMutex.RLock()
	defer watchersMutex.RUnlock()
	for _, seenFiles := range seenFilesPerProject {
		seenFiles.Delete(key)
	}
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

		// Get file metadata (hash and permissions) if it's a file (not directory)
		var fileHash string
		var fileSize int64
		var fileMode string
		var fileUID, fileGID int
		if event.EventType != "DELETED" {
			if meta, err := getFileMetadata(event.Path); err == nil {
				fileHash = meta.Hash
				fileSize = meta.Size
				fileMode = meta.Mode
				fileUID = meta.UID
				fileGID = meta.GID
			}
		}

		// Determine file_type based on path
		fileType := getFileType(event.Path, projectID)

		// Use single writer function
		update := FileStateUpdate{
			ProjectID: projectID,
			EventType: event.EventType,
			FilePath:  event.Path,
			Hash:     fileHash,
			FileSize: fileSize,
			ModTime:  event.Timestamp.Unix(),
			FileMode: fileMode,
			FileUID:  fileUID,
			FileGID:  fileGID,
			FileType: fileType,
			Timestamp: event.Timestamp,
			Source:   "WATCHER",
		}

		if err := updateFileState(update); err != nil {
			log.Printf("Failed to update file state: %v\n", err)
		} else {
			log.Printf("Watcher: %s %s\n", event.EventType, event.Path)
		}
	}
}

// getFileType determines file_type based on path
func getFileType(filePath string, projectID int) string {
	project, err := getProjectByID(projectID)
	if err != nil {
		return "project"
	}

	// Check if file is in files_path (uploads) or app_path (project)
	for _, fp := range project.FilesPaths {
		if fp != "" && strings.HasPrefix(filePath, fp) {
			return "uploads"
		}
	}
	return "project"
}

// FileStateUpdate represents a file state change for atomic processing
type FileStateUpdate struct {
	ProjectID int
	EventType string // CREATED, MODIFIED, DELETED, PERMISSION_CHANGED
	FilePath  string
	Hash      string
	FileSize  int64
	ModTime   int64
	// Permission tracking fields
	FileMode string
	FileUID  int
	FileGID  int
	// For permission change events
	OldFileMode string
	OldFileUID  int
	OldFileGID  int
	FileType  string
	Timestamp time.Time
	Source    string // "WATCHER", "BASELINE", "INTEGRITY_SCAN"
}

// updateFileState is the SINGLE WRITER for project_files table
// All file state changes (from watcher, baseline, integrity scan) go through here
// Uses a transaction to ensure atomicity with fim_events
func updateFileState(update FileStateUpdate) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	ts := update.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	modTime := update.ModTime
	if modTime == 0 {
		modTime = ts.Unix()
	}

	// Default risk level for permission changes is HIGH
	permRiskLevel := "HIGH"

	switch update.EventType {
	case "CREATED":
		// INSERT OR IGNORE - handle race condition with watcher + baseline
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO project_files
			(project_id, file_path, hash, file_size, mod_time, status, file_type, file_mode, file_uid, file_gid, permission_changes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'ADDED', ?, ?, ?, ?, 0, strftime('%s', 'now'), strftime('%s', 'now'))
		`, update.ProjectID, update.FilePath, update.Hash, update.FileSize, modTime, update.FileType, update.FileMode, update.FileUID, update.FileGID)
		if err != nil {
			return fmt.Errorf("failed to insert file: %v", err)
		}
		// Get file_id for fim_events
		var fileID int
		tx.QueryRow("SELECT id FROM project_files WHERE project_id = ? AND file_path = ?",
			update.ProjectID, update.FilePath).Scan(&fileID)

		// Insert into fim_events with file_id
		_, err = tx.Exec(`
			INSERT INTO fim_events
			(project_id, file_id, event_type, file_path, file_hash, file_mode, file_uid, file_gid, actor_type, actor_id, actor_name,
			 actor_details, risk_level, classification, source, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, update.ProjectID, fileID, update.EventType, update.FilePath, update.Hash, update.FileMode, update.FileUID, update.FileGID,
			"UNKNOWN", "", "system", "actor attribution not available via inotify",
			classifyRisk(update.EventType, update.FilePath), classifyClassification(update.FilePath),
			update.Source, fmt.Sprintf(`{"size": %d}`, update.FileSize), ts.Unix())
		if err != nil {
			return fmt.Errorf("failed to insert fim_event: %v", err)
		}

	case "MODIFIED":
		// First check if there are permission changes
		var oldMode, oldUID, oldGID string
		var existingID int
		var permissionChanges int
		var hasPermChange bool

		tx.QueryRow("SELECT id, file_mode, file_uid, file_gid, COALESCE(permission_changes, 0) FROM project_files WHERE project_id = ? AND file_path = ?",
			update.ProjectID, update.FilePath).Scan(&existingID, &oldMode, &oldUID, &oldGID, &permissionChanges)

		// Check for permission changes
		if update.FileMode != "" && oldMode != "" && update.FileMode != oldMode {
			hasPermChange = true
		}
		if update.FileUID > 0 && oldUID != "" {
			var oldUIDInt int
			fmt.Sscanf(oldUID, "%d", &oldUIDInt)
			if update.FileUID != oldUIDInt {
				hasPermChange = true
			}
		}
		if update.FileGID > 0 && oldGID != "" {
			var oldGIDInt int
			fmt.Sscanf(oldGID, "%d", &oldGIDInt)
			if update.FileGID != oldGIDInt {
				hasPermChange = true
			}
		}

		// UPDATE only if exists (has baseline)
		result, err := tx.Exec(`
			UPDATE project_files
			SET hash = ?, file_size = ?, mod_time = ?, file_mode = ?, file_uid = ?, file_gid = ?,
			    status = 'MODIFIED', updated_at = strftime('%s', 'now')
			WHERE project_id = ? AND file_path = ?
		`, update.Hash, update.FileSize, modTime, update.FileMode, update.FileUID, update.FileGID, update.ProjectID, update.FilePath)
		if err != nil {
			return fmt.Errorf("failed to update file: %v", err)
		}
		rowsAffected, _ := result.RowsAffected()
		var fileID int
		if rowsAffected == 0 {
			// File not in baseline - insert as ADDED
			res, err := tx.Exec(`
				INSERT INTO project_files
				(project_id, file_path, hash, file_size, mod_time, status, file_type, file_mode, file_uid, file_gid, permission_changes, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, 'ADDED', ?, ?, ?, ?, 0, strftime('%s', 'now'), strftime('%s', 'now'))
			`, update.ProjectID, update.FilePath, update.Hash, update.FileSize, modTime, update.FileType, update.FileMode, update.FileUID, update.FileGID)
			if err != nil {
				return fmt.Errorf("failed to insert new file from modification: %v", err)
			}
			id, _ := res.LastInsertId()
			fileID = int(id)
		} else {
			fileID = existingID
		}

		// If permission changed, create a separate permission change event
		if hasPermChange {
			// Increment permission changes counter
			tx.Exec("UPDATE project_files SET permission_changes = permission_changes + 1 WHERE id = ?", fileID)

			// Insert permission change event
			_, err = tx.Exec(`
				INSERT INTO fim_events
				(project_id, file_id, event_type, file_path, file_mode, file_uid, file_gid,
				 old_file_mode, old_file_uid, old_file_gid, actor_type, actor_id, actor_name,
				 actor_details, risk_level, classification, source, details, timestamp)
				VALUES (?, ?, 'PERMISSION_CHANGED', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, update.ProjectID, fileID, update.FilePath, update.FileMode, update.FileUID, update.FileGID,
				oldMode, oldUID, oldGID,
				"UNKNOWN", "", "system", "actor attribution not available via inotify",
				permRiskLevel, classifyClassification(update.FilePath),
				update.Source, fmt.Sprintf(`{"size": %d, "reason": "permission change detected"}`, update.FileSize), ts.Unix())
			if err != nil {
				return fmt.Errorf("failed to insert permission change event: %v", err)
			}
		}

		// Insert into fim_events with file_id
		_, err = tx.Exec(`
			INSERT INTO fim_events
			(project_id, file_id, event_type, file_path, file_hash, file_mode, file_uid, file_gid, actor_type, actor_id, actor_name,
			 actor_details, risk_level, classification, source, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, update.ProjectID, fileID, update.EventType, update.FilePath, update.Hash, update.FileMode, update.FileUID, update.FileGID,
			"UNKNOWN", "", "system", "actor attribution not available via inotify",
			classifyRisk(update.EventType, update.FilePath), classifyClassification(update.FilePath),
			update.Source, fmt.Sprintf(`{"size": %d}`, update.FileSize), ts.Unix())
		if err != nil {
			return fmt.Errorf("failed to insert fim_event: %v", err)
		}

	case "DELETED":
		// Check if file was in baseline (only mark as DELETED if it was tracked)
		var exists int
		var fileID int
		tx.QueryRow("SELECT COUNT(*), id FROM project_files WHERE project_id = ? AND file_path = ?",
			update.ProjectID, update.FilePath).Scan(&exists, &fileID)
		if exists > 0 {
			// Update status to DELETED instead of hard delete (preserve history)
			_, err = tx.Exec(`
				UPDATE project_files
				SET status = 'DELETED', updated_at = strftime('%s', 'now')
				WHERE project_id = ? AND file_path = ?
			`, update.ProjectID, update.FilePath)
			if err != nil {
				return fmt.Errorf("failed to mark file as deleted: %v", err)
			}

			// Insert into fim_events with file_id
			_, err = tx.Exec(`
				INSERT INTO fim_events
				(project_id, file_id, event_type, file_path, file_hash, file_mode, file_uid, file_gid, actor_type, actor_id, actor_name,
				 actor_details, risk_level, classification, source, details, timestamp)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, update.ProjectID, fileID, update.EventType, update.FilePath, update.Hash, update.FileMode, update.FileUID, update.FileGID,
				"UNKNOWN", "", "system", "actor attribution not available via inotify",
				classifyRisk(update.EventType, update.FilePath), classifyClassification(update.FilePath),
				update.Source, fmt.Sprintf(`{"size": %d}`, update.FileSize), ts.Unix())
			if err != nil {
				return fmt.Errorf("failed to insert fim_event: %v", err)
			}
		}
		// If not exists, just ignore (file was never in baseline)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Dispatch alerts for HIGH and CRITICAL risk events
	dispatchAlertIfNeeded(update)

	return nil
}

// dispatchAlertIfNeeded dispatches alerts for significant FIM events
func dispatchAlertIfNeeded(update FileStateUpdate) {
	riskLevel := classifyRisk(update.EventType, update.FilePath)

	// Only alert for HIGH or CRITICAL risk events
	if riskLevel != "HIGH" && riskLevel != "CRITICAL" {
		return
	}

	// Get project name
	projectName := ""
	if p, err := getProjectByID(update.ProjectID); err == nil {
		projectName = p.Name
	}

	// Get event ID from database (last inserted)
	var eventID int
	db.QueryRow("SELECT last_insert_rowid()").Scan(&eventID)

	alerts.DispatchAlert(
		update.ProjectID,
		projectName,
		eventID,
		update.EventType,
		update.FilePath,
		update.Hash,
		update.FileMode,
		update.OldFileMode,
		riskLevel,
		classifyClassification(update.FilePath),
		"system",
		"actor attribution not available via inotify",
		update.Source,
		update.Timestamp,
	)
}

// classifyRisk determines risk level based on event type and path
func classifyRisk(eventType, filePath string) string {
	if eventType == "DELETED" {
		return "MEDIUM"
	}
	if eventType == "PERMISSION_CHANGED" {
		return "HIGH"
	}
	if strings.Contains(filePath, "/usageStats/") ||
		strings.Contains(filePath, "/cache/") ||
		strings.Contains(filePath, "/temp/") {
		return "LOW"
	}
	return "HIGH"
}

// classifyClassification determines classification based on path
func classifyClassification(filePath string) string {
	if strings.Contains(filePath, "/usageStats/") {
		return "SYSTEM_GENERATED"
	}
	if strings.Contains(filePath, "/cache/") {
		return "SYSTEM_GENERATED"
	}
	if strings.Contains(filePath, "/temp/") {
		return "SYSTEM_GENERATED"
	}
	return "UNKNOWN_SOURCE"
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

// correlateOJS checks if file is from OJS workflow using CORRECT project ID
func correlateOJS(filePath string, eventType string, projectID int) OJSInfo {
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

	// Try to correlate with OJS database using CORRECT project ID
	ctx, cancel := context.WithTimeout(context.Background(), OJSLookupTimeout)
	defer cancel()

	// Get project config for DB connection using the CORRECT project ID
	project, err := getProjectByID(projectID)
	if err != nil {
		log.Printf("Failed to get project %d config: %v\n", projectID, err)
		result.Reason = "Project not found"
		return result
	}

	if project.DBHost == "" || project.DBUser == "" || project.DBName == "" {
		result.Reason = "OJS database not configured for this project"
		return result
	}

	// Connect to OJS database
	ojsDB, err := connectMySQLWithContext(ctx, project.DBUser, project.DBPass, project.DBHost, project.DBName)
	if err != nil {
		log.Printf("Failed to connect to OJS DB for project %d: %v\n", projectID, err)
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

// FileMetadata contains file hash and permission information
type FileMetadata struct {
	Hash   string
	Size   int64
	Mode   string // Octal permission string (e.g., "0644")
	UID    int    // Owner user ID
	GID    int    // Owner group ID
}

// getFileMetadata calculates SHA256 hash and captures permission info of a file
func getFileMetadata(filePath string) (FileMetadata, error) {
	meta := FileMetadata{}

	file, err := os.Open(filePath)
	if err != nil {
		return meta, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return meta, err
	}

	// Get permission info from syscall.Stat_t
	if stat.Sys() != nil {
		if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
			meta.UID = int(sysStat.Uid)
			meta.GID = int(sysStat.Gid)
			meta.Mode = fmt.Sprintf("%04o", stat.Mode().Perm())
		}
	}

	// If syscall didn't work, fall back to os.FileMode
	if meta.Mode == "" {
		meta.Mode = fmt.Sprintf("%04o", stat.Mode().Perm())
	}

	// Skip large files (> 10MB) for hashing, but still capture metadata
	if stat.Size() > 10*1024*1024 {
		return meta, nil
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return meta, err
	}

	meta.Hash = hex.EncodeToString(hash.Sum(nil))
	meta.Size = stat.Size()

	return meta, nil
}

// getFileHash calculates SHA256 hash of file (legacy function for compatibility)
func getFileHash(filePath string) (string, int64, error) {
	meta, err := getFileMetadata(filePath)
	return meta.Hash, meta.Size, err
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

	_, err := db.Exec(`
		INSERT INTO fim_events
		(project_id, event_type, file_path, file_hash, actor_type, actor_id, actor_name,
		 actor_details, risk_level, classification, source, details, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ProjectID, event.EventType, event.FilePath, event.FileHash,
		event.ActorType, event.ActorID, event.ActorName, event.ActorDetails,
		event.RiskLevel, event.Classification, event.Source, details, ts)

	if err != nil {
		log.Printf("Failed to store FIM event: %v\n", err)
	}
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

	projects, err := getProjects()
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
				// Start watcher without holding the lock
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
