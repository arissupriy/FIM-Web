package main

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

func StartWorker() {
	log.Println("Starting background worker...")

	// Resuscitate crashed jobs (if server restarted while a job was 'running')
	res, err := db.Exec("UPDATE jobs SET status = 'queued' WHERE status = 'running'")
	if err == nil {
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Resuscitated %d crashed jobs back to queued state.\n", rowsAffected)
		}
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
		projects, err := getProjects()
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
					StartFIMWatcher(p.ID, watchPaths)
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

// TriggerWorker signals the worker to process jobs immediately
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
	rows, err := db.QueryContext(ctx, "SELECT id FROM projects WHERE status = 'active' AND integrity_scan_enabled = 1 AND baseline_at > 0")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var projectID int
		if err := rows.Scan(&projectID); err != nil {
			continue
		}

		// Check if there's already a running integrity scan
		var runningCount int
		db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE project_id = ? AND type = 'integrity_scan' AND status IN ('queued', 'running')", projectID).Scan(&runningCount)

		if runningCount == 0 {
			// Queue integrity scan job
			db.Exec("INSERT INTO jobs (project_id, type, status) VALUES (?, 'integrity_scan', 'queued')", projectID)
			log.Printf("Queued integrity scan for project %d\n", projectID)
		}
	}
}

func processNextJob() {
	var job Job

	// SQLite-compatible: Try to claim a job atomically
	// First, get a queued job ID
	err := db.QueryRow(`
		SELECT id, project_id, type FROM jobs
		WHERE status = 'queued'
		ORDER BY id ASC LIMIT 1
	`).Scan(&job.ID, &job.ProjectID, &job.Type)

	if err != nil {
		return // No queued jobs available
	}

	// Try to claim it with UPDATE (only if still queued - prevents race)
	result, err := db.Exec("UPDATE jobs SET status = 'running', started_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'queued'", job.ID)
	if err != nil {
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return // Another worker already took it
	}

	if job.Type == "initial_baseline" {
		db.Exec("UPDATE projects SET status = 'counting' WHERE id = ?", job.ProjectID)
	}

	p, err := getProjectByID(job.ProjectID)
	if err != nil {
		failJob(job.ID, job.ProjectID, err.Error())
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
		failJob(job.ID, p.ID, "scan timeout exceeded 2 hours during quick count")
		return
	}

	if job.Type == "initial_baseline" {
		db.Exec("UPDATE projects SET status = 'scanning', baseline_total = ?, baseline_processed = 0 WHERE id = ?", total, p.ID)
	}

	// 2. Deep Walk & Hash (Baseline Processed)
	existingFiles, err := getProjectFiles(p.ID)
	if err != nil {
		failJob(job.ID, p.ID, "failed to get project files: "+err.Error())
		return
	}

	var addedFiles []ProjectFile
	var modifiedFiles []ProjectFile
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
				addedFiles = append(addedFiles, ProjectFile{
					ProjectID: p.ID,
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
				db.Exec("UPDATE projects SET baseline_processed = ? WHERE id = ?", processed, p.ID)
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
		failJob(job.ID, p.ID, "scan timeout exceeded 2 hours, partial data only")
		return
	}

	if job.Type == "initial_baseline" {
		// Update to reconciling state
		db.Exec("UPDATE projects SET status = 'reconciling', baseline_processed = ? WHERE id = ?", total, p.ID)
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
				addedFiles = append(addedFiles, ProjectFile{
					ProjectID: p.ID,
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
		failJob(job.ID, p.ID, "scan timeout exceeded 2 hours during reconciling")
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
	if err := batchUpsertProjectFiles(append(addedFiles, modifiedFiles...)); err != nil {
		failJob(job.ID, p.ID, "failed to persist files: "+err.Error())
		return
	}
	if err := batchDeleteProjectFiles(deletedIDs); err != nil {
		failJob(job.ID, p.ID, "failed to persist deletions: "+err.Error())
		return
	}

	// OJS Super Smart Checker (Reconciliation)
	if p.Template == "OJS 3.x" || p.Template == "OJS 2.x" {
		orphans, err := reconcileOJSFiles(ctx, p, addedFiles, modifiedFiles)
		if err == nil && len(orphans) > 0 {
			if err := batchUpsertProjectFiles(orphans); err != nil {
				log.Printf("Warning: failed to persist orphan findings for project %d: %v\n", p.ID, err)
			}
		}
	}

	finalStatus := "active"
	if job.Type == "initial_baseline" {
		if total > 0 && float64(filesSuccess) < float64(total)*0.9 {
			finalStatus = "completed_with_warnings"
		}
		// Set baseline timestamp
		db.Exec("UPDATE projects SET status = ?, baseline_at = strftime('%s', 'now'), baseline_total = ?, baseline_processed = ? WHERE id = ?", finalStatus, total, filesSuccess, p.ID)
	} else if job.Type == "integrity_scan" {
		// Update last integrity scan timestamp
		db.Exec("UPDATE projects SET status = ?, last_integrity_scan = strftime('%s', 'now') WHERE id = ?", finalStatus, p.ID)
	} else {
		// Retain previous status for rescan
		db.QueryRow("SELECT status FROM projects WHERE id = ?", p.ID).Scan(&finalStatus)
		db.Exec("UPDATE projects SET status = ? WHERE id = ?", finalStatus, p.ID)
	}

	db.Exec("UPDATE jobs SET status = 'done', finished_at = CURRENT_TIMESTAMP, files_success=?, files_skipped=?, files_error=? WHERE id = ?", filesSuccess, filesSkipped, filesError, job.ID)
	log.Printf("Job %d completed for project %d with %d successes, %d skipped, %d errors\n", job.ID, p.ID, filesSuccess, filesSkipped, filesError)
}

func failJob(jobID int, projectID int, errMsg string) {
	log.Printf("Job %d failed: %s\n", jobID, errMsg)
	db.Exec("UPDATE jobs SET status = 'failed', error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?", errMsg, jobID)
	db.Exec("UPDATE projects SET status = 'error', error_message = ? WHERE id = ?", errMsg, projectID)
}
