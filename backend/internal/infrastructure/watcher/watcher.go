// Package watcher provides real-time file integrity monitoring using fsnotify.
package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/wire"
)

// Global alert dispatcher
var globalAlertDispatcher interface {
	Dispatch(event *models.FIMEvent)
}

// SetAlertDispatcher sets the global alert dispatcher.
func SetAlertDispatcher(dispatcher interface{ Dispatch(event *models.FIMEvent) }) {
	globalAlertDispatcher = dispatcher
}

// FIM Watcher Configuration
const (
	EventBufferSize      = 1000    // Max events in buffer per project
	BatchProcessInterval = 1 * time.Second // Process events every 1 second
	DebounceWindow      = 500 * time.Millisecond // Ignore duplicate events within this window
	OJSLookupTimeout   = 5 * time.Second // Timeout for OJS database lookup
)

// ProjectWatcher - state for each project's watcher
type ProjectWatcher struct {
	ProjectID    int
	Paths       []string
	Watcher     *fsnotify.Watcher
	EventChannel chan FIMEvent
	Ctx         context.Context
	Cancel      context.CancelFunc
	WG          sync.WaitGroup
	IsRunning   bool
	Mu          sync.RWMutex
}

// FIM Watcher Global State
var (
	globalWatcherCtx, globalWatcherCancel = context.WithCancel(context.Background())
	projectWatchers = make(map[int]*ProjectWatcher) // map[projectID] -> watcher
	watchersMutex sync.RWMutex
	seenFilesPerProject = make(map[int]*sync.Map) // map[projectID] -> seenFiles
)

// FIMEvent - event structure for processing
type FIMEvent struct {
	Path      string
	Op        fsnotify.Op // CREATE, WRITE, REMOVE, CHMOD, etc
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

// correlateWithTemplate correlates a file event using the template system.
// This is the generic version that works with any registered template.
// Core (infrastructure) uses templates through interfaces - no direct CMS imports.
func correlateWithTemplate(projectID int, filePath string, eventType string) (*template.CorrelationResult, error) {
	result := template.NewCorrelationResult(filePath, eventType)

	// Get project
	ctx, cancel := context.WithTimeout(context.Background(), OJSLookupTimeout)
	defer cancel()

	p, err := wire.GetProjectByID(ctx, projectID)
	if err != nil {
		result.Reason = "Project not found"
		return result, nil
	}

	// Handle DELETED events specially
	if eventType == "DELETED" {
		result.Classification = "DELETED"
		result.RiskLevel = "MEDIUM"
		result.Reason = "File deletion detected"
		return result, nil
	}

	// Get template for this project
	t, ok := template.Get(p.Template)
	if !ok {
		result.Reason = fmt.Sprintf("Template '%s' not registered", p.Template)
		return result, nil
	}

	// Create CMS database connection using template (NOT infrastructure)
	dbCfg := template.DBConnectionConfig{
		Host:     p.DBHost,
		User:     p.DBUser,
		Password: p.DBPass,
		DBName:   p.DBName,
		Timeout:  10,
	}

	cmsDB, err := t.CreateDBConnection(ctx, dbCfg)
	if err != nil {
		result.Reason = "Failed to connect to CMS database"
		return result, err
	}
	defer cmsDB.Close()

	// Use template to correlate file
	return t.CorrelateFile(ctx, cmsDB, filePath, eventType)
}

// FileMetadata holds file hash and permission information
type FileMetadata struct {
	Hash    string
	Size    int64
	Mode    string  // Octal like "0644"
	UID     uint32
	GID     uint32
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

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Create new watcher for this project
	ctx, cancel := context.WithCancel(globalWatcherCtx)
	pw := &ProjectWatcher{
		ProjectID:    projectID,
		Paths:       validPaths,
		Watcher:     watcher,
		EventChannel: make(chan FIMEvent, EventBufferSize),
		Ctx:         ctx,
		Cancel:       cancel,
		IsRunning:    true,
	}

	projectWatchers[projectID] = pw
	seenFilesPerProject[projectID] = &sync.Map{}

	// Update database status
	dbCtx := context.Background()
	wire.UpdateWatcherStatus(dbCtx, projectID, "running")

	// Start event processor
	pw.WG.Add(1)
	go processFIMEventsForProject(pw)

	// Add paths to watcher
	for _, path := range validPaths {
		if err := watcher.Add(path); err != nil {
			log.Printf("Failed to watch path %s: %v\n", path, err)
		}
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

	// Close watcher
	if pw.Watcher != nil {
		pw.Watcher.Close()
	}

	// Close channel to signal processors
	if pw.EventChannel != nil {
		close(pw.EventChannel)
	}

	// Wait for all goroutines to finish
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
		if pw.Watcher != nil {
			pw.Watcher.Close()
		}
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

// processFIMEventsForProject processes events in batches for a specific project
func processFIMEventsForProject(pw *ProjectWatcher) {
	defer pw.WG.Done()
	defer pw.Watcher.Close()

	// Start event reader goroutine
	pw.WG.Add(1)
	go func() {
		defer pw.WG.Done()
		for {
			select {
			case <-pw.Ctx.Done():
				return
			case event, ok := <-pw.Watcher.Events:
				if !ok {
					return
				}
				// Send to channel (non-blocking)
				select {
				case pw.EventChannel <- FIMEvent{
					Path:      event.Name,
					Op:        event.Op,
					Timestamp: time.Now(),
					ProjectID: pw.ProjectID,
				}:
				default:
					log.Printf("Warning: event channel full for project %d, dropping event for %s\n", pw.ProjectID, event.Name)
				}
			case err, ok := <-pw.Watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error for project %d: %v\n", pw.ProjectID, err)
			}
		}
	}()

	ticker := time.NewTicker(BatchProcessInterval)
	defer ticker.Stop()

	var events []FIMEvent

	for {
		select {
		case <-pw.Ctx.Done():
			// Process remaining events before exit
			if len(events) > 0 {
				persistFIMEvents(pw.ProjectID, events)
			}
			return
		case event := <-pw.EventChannel:
			// Debounce
			if shouldDebounce(pw.ProjectID, event.Path, event.Op.String()) {
				continue
			}
			events = append(events, event)
		case <-ticker.C:
			if len(events) > 0 {
				persistFIMEvents(pw.ProjectID, events)
				events = events[:0]
			}
		}
	}
}

// shouldDebounce checks if event should be ignored due to duplicate within window
func shouldDebounce(projectID int, path, op string) bool {
	seenFiles, exists := seenFilesPerProject[projectID]
	if !exists {
		seenFiles = &sync.Map{}
		seenFilesPerProject[projectID] = seenFiles
	}

	key := path + "|" + op

	if _, exists := seenFiles.Load(key); exists {
		return true
	}

	seenFiles.Store(key, time.Now())
	seenFilesPerProject[projectID] = seenFiles

	return false
}

// cleanupDebounceCache removes old entries from debounce cache
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
func persistFIMEvents(projectID int, events []FIMEvent) {
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

		// Convert fsnotify.Op to string event type
		eventType := convertFSOp(event.Op)

		fimEvent := FIMEventDB{
			ProjectID:   projectID,
			EventType:   eventType,
			FilePath:    event.Path,
			Source:     "WATCHER",
			Timestamp:   event.Timestamp.Format(time.RFC3339),
		}

		// Get actor information
		actor := getActorInfo(event.Path, eventType)
		fimEvent.ActorType = actor.Type
		fimEvent.ActorID = actor.ID
		fimEvent.ActorName = actor.Name
		if actorJSON, err := json.Marshal(actor); err == nil {
			fimEvent.ActorDetails = string(actorJSON)
		}

		// Correlate with CMS database using template system
		correlation, err := correlateWithTemplate(projectID, event.Path, eventType)
		if err != nil {
			log.Printf("Failed to correlate file: %v\n", err)
		} else if correlation != nil {
			fimEvent.Classification = correlation.Classification
			fimEvent.RiskLevel = correlation.RiskLevel

			if correlation.Found {
				fimEvent.ActorType = correlation.ActorType
				if correlation.ActorID != "" {
					fimEvent.ActorID = correlation.ActorID
				}
				if correlation.ActorName != "" {
					fimEvent.ActorName = correlation.ActorName
				}
				if actorJSON, err := json.Marshal(correlation); err == nil {
					fimEvent.ActorDetails = string(actorJSON)
				}
			}
		}

		// Get file metadata (hash and permissions) if not deleted
		if event.Op != fsnotify.Remove {
			if meta, err := getFileMetadata(event.Path); err == nil && meta != nil {
				fimEvent.FileHash = meta.Hash
				fimEvent.Details = fmt.Sprintf(`{"size": %d, "hash": "%s", "mode": "%s", "uid": %d, "gid": %d}`, meta.Size, meta.Hash, meta.Mode, meta.UID, meta.GID)

				// Check for permission changes
				baselineFile := getBaselineFile(projectID, event.Path)
				if baselineFile != nil {
					if baselineFile.FileMode != meta.Mode || baselineFile.FileUID != meta.UID || baselineFile.FileGID != meta.GID {
						fimEvent.RiskLevel = "HIGH"
						fimEvent.Classification = "PERMISSION_CHANGE"
						fimEvent.Details = fmt.Sprintf(`{"size": %d, "hash": "%s", "mode": "%s", "uid": %d, "gid": %d, "baseline_mode": "%s", "baseline_uid": %d, "baseline_gid": %d, "permission_change": true}`, meta.Size, meta.Hash, meta.Mode, meta.UID, meta.GID, baselineFile.FileMode, baselineFile.FileUID, baselineFile.FileGID)
						updatePermissionChanges(projectID, baselineFile.ID)
					}
				}
			}
		}

		// Store event
		storeFIMEvent(fimEvent)
	}
}

// FIMEventDB - event structure for database
type FIMEventDB struct {
	ProjectID     int
	EventType     string
	FilePath      string
	FileHash      string
	ActorType     string
	ActorID       string
	ActorName     string
	ActorDetails  string
	RiskLevel     string
	Classification string
	Source       string
	Details      string
	Timestamp     string
}

// convertFSOp converts fsnotify.Op to string event type
func convertFSOp(op fsnotify.Op) string {
	switch op {
	case fsnotify.Create:
		return "CREATED"
	case fsnotify.Write, fsnotify.Write|fsnotify.Create:
		return "MODIFIED"
	case fsnotify.Remove:
		return "DELETED"
	case fsnotify.Rename:
		return "RENAMED"
	case fsnotify.Chmod:
		return "PERMISSION_CHANGED"
	default:
		return "MODIFIED"
	}
}

// getActorInfo gets actor information from system
func getActorInfo(filePath string, eventType string) FIMEventActor {
	actor := FIMEventActor{Type: "UNKNOWN"}

	if stat, err := os.Stat(filePath); err == nil {
		if sys := stat.Sys(); sys != nil {
			if stat_t, ok := sys.(*syscall.Stat_t); ok {
				actor.UID = fmt.Sprintf("%d", stat_t.Uid)
			}
		}
	}

	if output, err := exec.Command("whoami").Output(); err == nil {
		actor.Type = "SYSTEM_USER"
		actor.Name = strings.TrimSpace(string(output))
	}

	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", os.Getpid())); err == nil {
		actor.Process = strings.ReplaceAll(string(cmdline), "\x00", " ")
	}

	return actor
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
		ID:       pf.ID,
		FileMode: pf.FileMode,
		FileUID:  pf.FileUID,
		FileGID: pf.FileGID,
	}
}

// updatePermissionChanges increments permission change counter
func updatePermissionChanges(projectID int, fileID int) {
	wire.IncrementPermissionChanges(context.Background(), fileID, projectID)
}

// getFileMetadata calculates SHA256 hash and captures file metadata
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
		Mode: fmt.Sprintf("%04o", stat.Mode().Perm()),
	}

	if sys := stat.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			meta.UID = stat.Uid
			meta.GID = stat.Gid
		}
	}

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
func storeFIMEvent(event FIMEventDB) {
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
		Source:       event.Source,
		Details:      details,
		Timestamp:     fmt.Sprintf("%d", ts),
	}

	if err := wire.CreateFIMEvent(context.Background(), fimEvent); err != nil {
		log.Printf("Failed to store FIM event: %v\n", err)
		return
	}

	// Dispatch alert for FIM event
	if globalAlertDispatcher != nil {
		globalAlertDispatcher.Dispatch(fimEvent)
	}
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
