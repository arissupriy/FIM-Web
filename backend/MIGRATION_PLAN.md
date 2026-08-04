# Clean Architecture Migration Plan

## Current Status

### Already Migrated ✅
- `internal/domain/models/` - Domain models
- `internal/domain/repository/` - Repository interfaces
- `internal/infrastructure/database/sqlite/` - SQLite implementations
- `internal/infrastructure/http/handlers/` - HTTP handlers (auth, project, scan, fim, response)
- `internal/infrastructure/http/middleware/` - Middleware (auth)
- `internal/infrastructure/http/router.go` - Router
- `internal/infrastructure/auth/` - JWT auth service
- `internal/application/usecase/` - Use cases (project, scan, fim)
- `internal/templates/ojs/` - OJS template (detector, service, handlers)

### Legacy Files Remaining ⚠️

| File | Lines | Status |
|------|-------|--------|
| handlers.go | 1316 | Partially migrated, legacy handlers still used |
| db.go | 483 | Repository layer exists, not wired up |
| scanner.go | 302 | NOT migrated |
| watcher.go | 810 | NOT migrated |
| worker.go | 561 | NOT migrated |
| main.go | 183 | Partially wired |

---

## Migration Plan (Step by Step)

### Step 1: Complete db.go → Repository Migration
**Goal:** Wire up existing repositories, remove db.go functions

**Files to modify:**
- `main.go` - Use new repositories instead of db.go functions

**Changes:**
1. Replace `getProjects()` with `projectRepo.List()`
2. Replace `getProjectByID()` with `projectRepo.GetByID()`
3. Replace `addProject()` with `projectRepo.Create()`
4. Replace `updateProject()` with `projectRepo.Update()`
5. Add auth repository usage for `CreateAdmin`, `GetAdminByUsername`
6. Add audit repository usage for `LogActivity`, `GetAuditLogs`
7. Add file repository usage for `getProjectFiles`, `batchUpsertProjectFiles`

**Complexity:** Medium - many call sites to update

---

### Step 2: Migrate scanner.go → Use Cases + OJS Service
**Goal:** Extract scanner logic to proper layers

**New files:**
- `internal/application/usecase/scan/scanner.go` - File scanning logic
- `internal/infrastructure/scanner/scanner.go` - OS-level file operations

**Move functions:**
| From | To | Reason |
|------|----|--------|
| `FastAuditProject()` | Use case | Business logic |
| `reconcileOJSFiles()` | OJS template | CMS-specific |
| `queryDBMetrics()` | OJS service | CMS-specific |
| `getOJSDetails()` | OJS service | Already done in service.go |
| `detectOJSVersion()` | OJS service | Already done in service.go |

**Status in service.go:** Already implemented (GetDetails, DetectVersion, GetMetrics)

**Complexity:** Medium - need to refactor scanner logic

---

### Step 3: Migrate watcher.go → Infrastructure Layer
**Goal:** Move watcher logic to proper infrastructure

**New files:**
- `internal/infrastructure/watcher/watcher.go` - inotifywait management
- `internal/infrastructure/watcher/actor.go` - Actor detection
- `internal/infrastructure/watcher/debounce.go` - Event debouncing

**Move functions:**
| From | To | Reason |
|------|----|--------|
| `StartFIMWatcher()` | watcher.go | Infrastructure |
| `StopFIMWatcherForProject()` | watcher.go | Infrastructure |
| `StopAllFIMWatchers()` | watcher.go | Infrastructure |
| `watchPathForProject()` | watcher.go | Infrastructure |
| `processFIMEventsForProject()` | watcher.go | Infrastructure |
| `shouldDebounce()` | debounce.go | Infrastructure |
| `cleanupDebounceCache()` | debounce.go | Infrastructure |
| `getActorInfo()` | actor.go | Infrastructure |
| `correlateOJS()` | OJS template | CMS-specific |
| `getFileMetadata()` | scanner | Infrastructure |
| `storeFIMEvent()` | fim_repo | Already exists |
| `connectOJS()` | OJS service | Already exists |
| `RestoreWatchersOnStartup()` | watcher.go | Infrastructure |

**Complexity:** High - complex goroutine management

---

### Step 4: Migrate worker.go → Infrastructure Layer
**Goal:** Move worker logic to proper infrastructure

**New files:**
- `internal/infrastructure/worker/worker.go` - Worker implementation
- `internal/infrastructure/worker/queue.go` - Job queue

**Move functions:**
| From | To | Reason |
|------|----|--------|
| `StartWorker()` | worker.go | Infrastructure |
| `TriggerWorker()` | queue.go | Infrastructure |
| `triggerIntegrityScans()` | worker.go | Infrastructure |
| `processNextJob()` | worker.go | Infrastructure |
| `failJob()` | worker.go | Infrastructure |
| `isUnderPath()` | scanner | Utility |

**Dependencies:** Requires scanner.go migration first

**Complexity:** Medium - goroutine and queue management

---

### Step 5: Complete handlers.go Migration
**Goal:** Add missing handlers, remove legacy routes

**New handlers needed:**
- `TestConnection` handler

**Options:**
1. **Option A (Recommended):** Keep legacy handlers for backward compatibility
2. **Option B:** Migrate all 25 handlers to clean architecture

**Recommendation:** Option A - legacy handlers work, new API uses clean architecture

---

## Execution Order

```
1. Step 1: Wire up repositories in main.go
   ↓
2. Step 2: Migrate scanner.go logic
   ↓
3. Step 3: Migrate watcher.go (depends on scanner)
   ↓
4. Step 4: Migrate worker.go (depends on scanner)
   ↓
5. Step 5: Clean up handlers (optional)
   ↓
6. Delete legacy files: db.go, scanner.go, watcher.go, worker.go, handlers.go
```

---

## Files to Delete After Migration

After all steps complete:
- `backend/db.go` - Replaced by repositories
- `backend/scanner.go` - Replaced by use cases + OJS service
- `backend/watcher.go` - Replaced by infrastructure
- `backend/worker.go` - Replaced by infrastructure
- `backend/handlers.go` - Replaced by new handlers (if migrated)
- `backend/models.go` - Replaced by internal/domain/models

---

## Verification After Each Step

1. `go build ./...` - Must compile
2. `go test ./...` - Must pass
3. `go build -o fim-server .` - Must create binary
4. Manual test endpoints
