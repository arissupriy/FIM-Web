# Clean Architecture Migration Plan - DETAILED

## Dependency Map

```
LEGEND:
  [A] → [B]  means: A calls B
  db.go       = database functions
  scanner.go  = file scanning
  worker.go   = background jobs
  watcher.go  = inotifywait
  handlers.go = HTTP handlers
  main.go     = entry point

CALL GRAPH:
─────────────────────────────────────────────────────────────

main.go
├── initDB()          → db.go
├── SeedDefaultAdmin() → db.go.CreateAdmin()
├── StartWorker()      → worker.go
├── RestoreWatchers()  → watcher.go, db.go.getProjects()
└── handleLogin()      → handlers.go
    └── GetAdminByUsername()   → db.go
    └── LogActivity()          → db.go

worker.go
├── getProjects()           → db.go
├── getProjectByID()        → db.go
├── getProjectFiles()       → db.go
└── batchUpsertProjectFiles() → db.go

watcher.go
├── getProjects()           → db.go
└── (many FIM operations)

handlers.go (25 functions)
├── getProjects()           → db.go [USED 4x]
├── getProjectByID()         → db.go [USED 8x]
├── addProject()             → db.go [USED 1x]
├── updateProject()          → db.go [USED 1x]
├── CreateAdmin()            → db.go [NOT USED directly]
├── GetAdminByUsername()     → db.go [USED 1x]
├── LogActivity()            → db.go [USED 3x]
├── GetAuditLogs()           → db.go [USED 1x]
├── batchUpsertProjectFiles() → db.go [NOT USED in handlers]
├── testDBConnection()       → MySQL connection
├── getOJSDetails()         → scanner.go [USED 1x]
├── FastAuditProject()       → scanner.go [USED 2x]
└── StartFIMWatcher()        → watcher.go [USED 1x]
    └── StopFIMWatcherForProject() → watcher.go
    └── IsWatcherRunningForProject() → watcher.go
```

---

## Systematic Migration Order

### Phase 1: Repository Layer (BLOCKING)
**Status:** Repository interfaces exist, but not wired up

```
db.go functions          →  Repository methods
─────────────────────────────────────────────────────
getProjects()           →  projectRepo.List()
getProjectByID()        →  projectRepo.GetByID()
addProject()             →  projectRepo.Create()
updateProject()          →  projectRepo.Update()
CreateAdmin()            →  adminRepo.Create()
GetAdminByUsername()     →  adminRepo.GetByUsername()
LogActivity()            →  auditRepo.Create()
GetAuditLogs()          →  auditRepo.List()
getProjectFiles()        →  fileRepo.GetByProjectID()
batchUpsertProjectFiles() →  fileRepo.BatchUpsert()
```

**Files to modify:**
1. `main.go` - Replace db.go calls with repository calls
2. `worker.go` - Replace db.go calls with repository calls
3. `watcher.go` - Replace db.go calls with repository calls
4. `handlers.go` - Replace db.go calls with repository calls

**Blockers:** None
**Dependencies:** None

---

### Phase 2: Scanner Layer
**Status:** Functions exist in scanner.go, partially migrated

```
scanner.go functions          →  New location
─────────────────────────────────────────────────────
FastAuditProject()            →  already in scanner.go (TODO: move to usecase)
queryDBMetrics()             →  internal/templates/ojs/service.go (DONE)
getOJSDetails()              →  internal/templates/ojs/service.go (DONE)
detectOJSVersion()           →  internal/templates/ojs/service.go (DONE)
reconcileOJSFiles()          →  internal/templates/ojs/service.go
connectMySQL()               →  internal/templates/ojs/service.go (reuse)
```

**Files to create/modify:**
1. Create `internal/infrastructure/scanner/` if needed
2. Modify scanner.go → extract to proper layers
3. Wire OJS service into scanner

**Blockers:** None
**Dependencies:** Repository layer

---

### Phase 3: Worker Layer
**Status:** NOT migrated

```
worker.go functions       →  New location
─────────────────────────────────────────────────────
StartWorker()            →  internal/infrastructure/worker/worker.go
TriggerWorker()          →  internal/infrastructure/worker/worker.go
triggerIntegrityScans()  →  internal/infrastructure/worker/worker.go
processNextJob()         →  internal/infrastructure/worker/worker.go
failJob()                →  internal/infrastructure/worker/worker.go
isUnderPath()            →  internal/infrastructure/scanner/util.go
```

**Files to create:**
1. `internal/infrastructure/worker/worker.go`
2. `internal/infrastructure/worker/queue.go`

**Blockers:** Scanner layer
**Dependencies:** Repository, Scanner

---

### Phase 4: Watcher Layer
**Status:** NOT migrated

```
watcher.go functions              →  New location
─────────────────────────────────────────────────────────────────
StartFIMWatcher()               →  internal/infrastructure/watcher/watcher.go
StopFIMWatcherForProject()        →  internal/infrastructure/watcher/watcher.go
StopAllFIMWatchers()             →  internal/infrastructure/watcher/watcher.go
watchPathForProject()             →  internal/infrastructure/watcher/watcher.go
processFIMEventsForProject()      →  internal/infrastructure/watcher/processor.go
shouldDebounce()                 →  internal/infrastructure/watcher/debounce.go
cleanupDebounceCache()            →  internal/infrastructure/watcher/debounce.go
getActorInfo()                    →  internal/infrastructure/watcher/actor.go
correlateOJS()                    →  internal/templates/ojs/service.go
getFileMetadata()                  →  scanner layer
storeFIMEvent()                   →  fim_repo (exists)
RestoreWatchersOnStartup()        →  internal/infrastructure/watcher/watcher.go
```

**Files to create:**
1. `internal/infrastructure/watcher/watcher.go`
2. `internal/infrastructure/watcher/processor.go`
3. `internal/infrastructure/watcher/debounce.go`
4. `internal/infrastructure/watcher/actor.go`

**Blockers:** Scanner layer, Repository layer
**Dependencies:** Repository, Scanner, OJS Service

---

### Phase 5: Handlers Layer (OPTIONAL)
**Status:** Handlers exist in handlers.go

```
Migration Options:
─────────────────────────────────────────────────────
Option A: Keep legacy handlers (RECOMMENDED)
  - Legacy routes /api/* use handlers.go
  - New routes /api/v1/* use clean architecture
  - No additional work needed

Option B: Migrate all handlers
  - 25 handlers to migrate
  - High effort, low value
```

**Recommendation:** Option A - backward compatibility

---

## Execution Matrix

| Phase | Tasks | Effort | Blockers | Files Modified |
|-------|-------|--------|----------|---------------|
| 1 | Wire repositories | Medium | None | main.go, worker.go, watcher.go, handlers.go |
| 2 | Scanner layer | Medium | Repos | scanner.go, new files |
| 3 | Worker layer | Medium | Scanner | worker.go, new files |
| 4 | Watcher layer | High | Scanner, Repos | watcher.go, new files |
| 5 | Handlers (optional) | Low | None | handlers.go (optional) |

---

## Verification Checklist Per Phase

### Phase N Verification:
```bash
# 1. Build check
cd backend && go build ./...

# 2. Tests
go test ./...

# 3. Binary
go build -o fim-server .

# 4. Dependencies check
go mod tidy && go build ./...

# 5. Static analysis
go vet ./...
```

---

## Files to DELETE After ALL Phases

```
backend/
├── db.go         # Replaced by repositories
├── scanner.go    # Replaced by use cases + OJS service
├── watcher.go    # Replaced by infrastructure/watcher
├── worker.go     # Replaced by infrastructure/worker
├── handlers.go   # Optional - for backward compatibility
└── models.go     # Replaced by internal/domain/models
```

---

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Breaking existing functionality | Keep legacy routes until new API verified |
| Circular dependencies | Follow execution order strictly |
| Missing test coverage | Add tests after each phase |
| Regression | Run full test suite after each phase |

---

## Timeline Recommendation

```
Day 1: Phase 1 (Repository wiring)
Day 2: Phase 2 (Scanner layer)
Day 3: Phase 3 (Worker layer)
Day 4: Phase 4 (Watcher layer)
Day 5: Phase 5 + Cleanup + Testing
```
