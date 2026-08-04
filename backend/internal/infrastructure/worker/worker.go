// Package worker provides background job processing for file integrity monitoring.
package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/infrastructure/scanner"
	"ojs-monitor/backend/internal/infrastructure/watcher"
	"ojs-monitor/backend/internal/wire"
)

// Global shutdown channel for graceful shutdown
var (
	shutdownCh = make(chan struct{})
	jobTrigger = make(chan struct{}, 1) // Buffered channel to trigger immediate job processing
)

func isUnderPath(path, base string) bool {
	path = filepath.Clean(path)
	base = filepath.Clean(base)
	return path == base || strings.HasPrefix(path, base+string(os.PathSeparator))
}

// StartWorker initializes and runs the background worker.
func StartWorker() {
	log.Println("Starting background worker...")

	// Resuscitate crashed jobs (if server restarted while a job was 'running')
	ctx := context.Background()
	rowsAffected, _ := wire.ResuscitateCrashedJobs(ctx)
	if rowsAffected > 0 {
		log.Printf("Resuscitated %d crashed jobs back to queued state.\n", rowsAffected)
	}

	// Scheduler: Trigger integrity scan jobs daily at 2 AM (if enabled)
	go func() {
		for {
			select {
			case <-shutdownCh:
				log.Println("Scheduler goroutine shutting down...")
				return
			case <-time.After(1 * time.Hour): // Check every hour
				triggerIntegrityScans()
			}
		}
	}()

	// Start watchers for all active projects
	go func() {
		ctx := context.Background()
		projects, err := wire.GetProjects(ctx)
		if err != nil {
			log.Printf("Failed to get projects for watcher startup: %v\n", err)
			return
		}
		for _, p := range projects {
			if p.Status == "active" && p.BaselineAt > 0 {
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
					watcher.StartFIMWatcher(p.ID, watchPaths)
				}
			}
		}
	}()

	// Signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, initiating graceful shutdown...\n", sig)
		close(shutdownCh)
	}()

	for {
		select {
		case <-shutdownCh:
			log.Println("Worker shutting down gracefully...")
			return
		case <-jobTrigger:
			// Triggered by manual scan - process immediately without delay
			processNextJob()
		default:
			processNextJob()
			time.Sleep(2 * time.Second)
		}
	}
}

// TriggerWorker signals the worker to process jobs immediately.
func TriggerWorker() {
	select {
	case jobTrigger <- struct{}{}:
	default:
		// Channel already has a pending signal
	}
}

func triggerIntegrityScans() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find projects with integrity scan enabled
	projects, err := wire.GetProjectsForIntegrityScan(ctx)
	if err != nil {
		return
	}

	for _, p := range projects {
		// Check if there's already a running integrity scan
		runningCount, _ := wire.GetJobRunningCount(ctx, p.ID)

		if runningCount == 0 {
			// Queue integrity scan job
			if err := wire.CreateIntegrityScanJob(ctx, p.ID); err == nil {
				log.Printf("Queued integrity scan for project %d\n", p.ID)
			}
		}
	}
}

func processNextJob() {
	// SQLite-compatible: Try to claim a job atomically
	ctx := context.Background()
	jobID, projectID, jobType, success, err := wire.ClaimNextQueued(ctx)
	if err != nil || !success {
		return // No queued jobs available or another worker got it
	}

	if jobType == "initial_baseline" {
		wire.UpdateProjectStatus(ctx, projectID, "counting")
	}

	p, err := wire.GetProjectByID(ctx, projectID)
	if err != nil {
		wire.FailJob(ctx, jobID, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	// Check for shutdown signal during work
	go func() {
		select {
		case <-shutdownCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// 1. Quick Count (Baseline Total)
	total := 0
	countFunc := func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Check whitelist
		for _, wPath := range p.WhitelistPaths {
			if wPath != "" && isUnderPath(path, wPath) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			return nil
		}
		total++
		return nil
	}
	timedOutCount := false
	for _, ap := range p.AppPaths {
		if err := filepath.Walk(ap, countFunc); err == context.DeadlineExceeded {
			timedOutCount = true
			break
		}
	}
	if !timedOutCount {
		for _, fp := range p.FilesPaths {
			if err := filepath.Walk(fp, countFunc); err == context.DeadlineExceeded {
				timedOutCount = true
				break
			}
		}
	}

	if timedOutCount {
		wire.FailJob(ctx, jobID, "scan timeout exceeded 2 hours during quick count")
		return
	}

	if jobType == "initial_baseline" {
		wire.UpdateBaseline(ctx, projectID, "scanning", total, 0)
	}

	// 2. Deep Walk & Hash (Baseline Processed)
	existingFiles, err := wire.GetProjectFiles(ctx, projectID)
	if err != nil {
		wire.FailJob(ctx, jobID, "failed to get project files: "+err.Error())
		return
	}

	var addedFiles []*models.ProjectFile
	var modifiedFiles []*models.ProjectFile
	var deletedIDs []int

	// Thread-safe seenFiles map using sync.Map
	var seenFiles sync.Map

	processed := 0
	var filesSuccess, filesSkipped, filesError int

	// Mutex for thread-safe access to addedFiles/modifiedFiles/deletedIDs
	var filesMutex sync.Mutex

	processPath := func(root string, fileType string) error {
		if root == "" {
			return nil
		}
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil {
				if os.IsPermission(err) {
					log.Printf("Warning: permission denied %s\n", path)
					filesSkipped++
					if info != nil && info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				filesError++
				return nil
			}

			// Symlink Guard
			if info.Mode()&os.ModeSymlink != 0 {
				filesSkipped++
				return nil
			}

			// Check whitelist
			for _, wPath := range p.WhitelistPaths {
				if wPath != "" && isUnderPath(path, wPath) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if info.IsDir() {
				return nil
			}

			seenFiles.Store(path, true)
			size := info.Size()
			modTime := info.ModTime().Unix()

			// [P1-04] Capture file permissions
			fileMode := info.Mode().Perm().String()
			var fileUID, fileGID uint32
			if sys := info.Sys(); sys != nil {
				if stat, ok := sys.(*syscall.Stat_t); ok {
					fileUID = stat.Uid
					fileGID = stat.Gid
				}
			}

			hashStr := ""
			if size < 10*1024*1024 { // Hash if < 10MB
				f, err := os.Open(path)
				if err != nil {
					if os.IsNotExist(err) {
						filesSkipped++
						return nil // Graceful skip (file deleted during scan)
					}
					if os.IsPermission(err) {
						filesSkipped++
						return nil
					}
					filesError++
					log.Printf("Warning: failed to open %s: %v\n", path, err)
				} else {
					h := sha256.New()
					io.Copy(h, f)
					hashStr = hex.EncodeToString(h.Sum(nil))
					f.Close()
				}
			}

			filesMutex.Lock()
			if existing, ok := existingFiles[path]; ok {
				// [P1-04] Check for permission changes
				permChanged := existing.FileMode != fileMode || existing.FileUID != fileUID || existing.FileGID != fileGID
				if existing.FileSize != size || existing.ModTime != modTime || permChanged || (hashStr != "" && existing.Hash != hashStr) {
					existing.FileSize = size
					existing.ModTime = modTime
					existing.Hash = hashStr
					// [P1-04] Update permission fields
					existing.FileMode = fileMode
					existing.FileUID = fileUID
					existing.FileGID = fileGID
					if permChanged {
						existing.PermissionChanges++
						existing.Status = "MODIFIED"
					}
					modifiedFiles = append(modifiedFiles, existing)
				}
			} else {
				addedFiles = append(addedFiles, &models.ProjectFile{
					ProjectID: projectID,
					FilePath:  path,
					Hash:      hashStr,
					FileSize:  size,
					ModTime:   modTime,
					Status:    "ADDED",
					FileType:  fileType,
					// [P1-04] Store permission fields
					FileMode: fileMode,
					FileUID:  fileUID,
					FileGID:  fileGID,
				})
			}
			filesMutex.Unlock()

			filesSuccess++
			processed++
			if processed%50 == 0 { // Batch progress updates
				wire.UpdateProjectStatus(ctx, projectID, "scanning")
				time.Sleep(1 * time.Millisecond) // Micro-Yielding to reduce I/O spike
			}
			return nil
		})
	}

	timedOut := false
	for _, ap := range p.AppPaths {
		if err := processPath(ap, "project"); err == context.DeadlineExceeded {
			timedOut = true
			break
		}
	}
	if !timedOut {
		for _, fp := range p.FilesPaths {
			if err := processPath(fp, "uploads"); err == context.DeadlineExceeded {
				timedOut = true
				break
			}
		}
	}

	if timedOut {
		wire.FailJob(ctx, jobID, "scan timeout exceeded 2 hours, partial data only")
		return
	}

	if jobType == "initial_baseline" {
		// Update to reconciling state
		wire.UpdateBaseline(ctx, projectID, "reconciling", total, processed)
	}

	reconcileProcessed := 0
	// Reconciling Second-Pass (catch files missed during walk)
	reconcileMissedFiles := func(root string, fileType string) error {
		if root == "" {
			return nil
		}
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil || info.IsDir() {
				return nil
			}

			reconcileProcessed++
			if reconcileProcessed%50 == 0 {
				time.Sleep(1 * time.Millisecond) // Just yield, don't update progress
			}

			// Use LoadOrStore for thread-safe check and update
			if _, loaded := seenFiles.LoadOrStore(path, true); !loaded {
				size := info.Size()
				modTime := info.ModTime().Unix()
				// [P1-04] Capture file permissions
				fileMode := info.Mode().Perm().String()
				var fileUID, fileGID uint32
				if sys := info.Sys(); sys != nil {
					if stat, ok := sys.(*syscall.Stat_t); ok {
						fileUID = stat.Uid
						fileGID = stat.Gid
					}
				}
				hashStr := ""
				if size < 10*1024*1024 {
					f, err := os.Open(path)
					if err == nil {
						h := sha256.New()
						io.Copy(h, f)
						hashStr = hex.EncodeToString(h.Sum(nil))
						f.Close()
					}
				}
				filesMutex.Lock()
				addedFiles = append(addedFiles, &models.ProjectFile{
					ProjectID: projectID,
					FilePath:  path,
					Hash:      hashStr,
					FileSize:  size,
					ModTime:   modTime,
					Status:    "ADDED",
					FileType:  fileType,
					// [P1-04] Store permission fields
					FileMode: fileMode,
					FileUID:  fileUID,
					FileGID:  fileGID,
				})
				filesMutex.Unlock()
			}
			return nil
		})
	}

	for _, ap := range p.AppPaths {
		if err := reconcileMissedFiles(ap, "project"); err == context.DeadlineExceeded {
			timedOut = true
			break
		}
	}
	if !timedOut {
		for _, fp := range p.FilesPaths {
			if err := reconcileMissedFiles(fp, "uploads"); err == context.DeadlineExceeded {
				timedOut = true
				break
			}
		}
	}

	if timedOut {
		wire.FailJob(ctx, jobID, "scan timeout exceeded 2 hours during reconciling")
		return
	}

	// Check deleted files: files in existingFiles but NOT in seenFiles
	filesMutex.Lock()
	for path, existing := range existingFiles {
		if _, seen := seenFiles.Load(path); !seen {
			deletedIDs = append(deletedIDs, existing.ID)
		}
	}
	filesMutex.Unlock()

	// Persist FIM changes
	allFiles := make([]*models.ProjectFile, 0, len(addedFiles)+len(modifiedFiles))
	allFiles = append(allFiles, addedFiles...)
	allFiles = append(allFiles, modifiedFiles...)
	if err := wire.BatchUpsertFiles(ctx, allFiles); err != nil {
		wire.FailJob(ctx, jobID, "failed to persist files: "+err.Error())
		return
	}
	if err := wire.BatchDeleteFiles(ctx, deletedIDs); err != nil {
		wire.FailJob(ctx, jobID, "failed to persist deletions: "+err.Error())
		return
	}

	// OJS Super Smart Checker (Reconciliation)
	// Note: scanner.ReconcileOJSFiles uses wire.LegacyProject types, so we convert for the call
	if p.Template == "OJS 3.x" || p.Template == "OJS 2.x" {
		// Convert domain files to legacy for scanner.ReconcileOJSFiles
		legacyAdded := make([]wire.LegacyProjectFile, len(addedFiles))
		for i, f := range addedFiles {
			legacyAdded[i] = wire.LegacyProjectFile{
				ID: f.ID, ProjectID: f.ProjectID, FilePath: f.FilePath,
				Hash: f.Hash, FileSize: f.FileSize, ModTime: f.ModTime,
				Status: f.Status, FileType: f.FileType,
				FileMode: f.FileMode, FileUID: f.FileUID, FileGID: f.FileGID,
				PermissionChanges: f.PermissionChanges,
			}
		}
		legacyModified := make([]wire.LegacyProjectFile, len(modifiedFiles))
		for i, f := range modifiedFiles {
			legacyModified[i] = wire.LegacyProjectFile{
				ID: f.ID, ProjectID: f.ProjectID, FilePath: f.FilePath,
				Hash: f.Hash, FileSize: f.FileSize, ModTime: f.ModTime,
				Status: f.Status, FileType: f.FileType,
				FileMode: f.FileMode, FileUID: f.FileUID, FileGID: f.FileGID,
				PermissionChanges: f.PermissionChanges,
			}
		}
		legacyP := wire.LegacyProject{
			ID: p.ID, DBHost: p.DBHost, DBUser: p.DBUser, DBPass: p.DBPass, DBName: p.DBName,
			AppPaths: p.AppPaths, FilesPaths: p.FilesPaths, Template: p.Template,
		}
		orphans, err := scanner.ReconcileOJSFiles(ctx, legacyP, legacyAdded, legacyModified)
		if err == nil && len(orphans) > 0 {
			orphanFiles := make([]*models.ProjectFile, len(orphans))
			for i, o := range orphans {
				orphanFiles[i] = &models.ProjectFile{
					ProjectID: o.ProjectID, FilePath: o.FilePath,
					Hash: o.Hash, FileSize: o.FileSize, ModTime: o.ModTime,
					Status: o.Status, FileType: o.FileType,
					FileMode: o.FileMode, FileUID: o.FileUID, FileGID: o.FileGID,
					PermissionChanges: o.PermissionChanges,
				}
			}
			if err := wire.BatchUpsertFiles(ctx, orphanFiles); err != nil {
				log.Printf("Warning: failed to persist orphan findings for project %d: %v\n", projectID, err)
			}
		}
	}

	finalStatus := "active"
	if jobType == "initial_baseline" {
		if total > 0 && float64(filesSuccess) < float64(total)*0.9 {
			finalStatus = "completed_with_warnings"
		}
		// Set baseline timestamp
		wire.UpdateBaseline(ctx, projectID, finalStatus, total, filesSuccess)
	} else if jobType == "integrity_scan" {
		// Update last integrity scan timestamp
		wire.UpdateIntegrityScan(ctx, projectID, finalStatus)
	} else {
		// Retain previous status for rescan - get current status
		currentStatus := wire.GetProjectStatus(ctx, projectID)
		wire.UpdateProjectStatus(ctx, projectID, currentStatus)
	}

	wire.CompleteJob(ctx, jobID, filesSuccess, filesSkipped, filesError)
	log.Printf("Job %d completed for project %d with %d successes, %d skipped, %d errors\n", jobID, projectID, filesSuccess, filesSkipped, filesError)
}

func failJob(jobID int, projectID int, errMsg string) {
	ctx := context.Background()
	log.Printf("Job %d failed: %s\n", jobID, errMsg)
	wire.FailJob(ctx, jobID, errMsg)
	wire.UpdateProjectStatusWithError(ctx, projectID, errMsg)
}
