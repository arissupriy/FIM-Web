# FIM Monitor - Clean Architecture Refactoring Plan

> **Platform Purpose:** FIM Monitor is a **generic File Integrity Monitoring platform** that can support multiple CMS/Systems via **Template/Plugin System**.
>
> **OJS** is the first dedicated template - not the only one.

---

## Platform Concept

```
┌─────────────────────────────────────────────────────────────────┐
│                     FIM MONITOR PLATFORM                       │
│                 (Generic File Integrity Monitoring)              │
├─────────────────────────────────────────────────────────────┤
│  Core Engine (Platform-wide)                                   │
│  ├── File Scanner (Hash, Permission, Metadata)                 │
│  ├── FIM Watcher (inotifywait)                              │
│  ├── Background Worker (Job Queue)                            │
│  └── Database (SQLite - Platform Schema)                      │
├─────────────────────────────────────────────────────────────┤
│  Templates/Plugins (CMS-Specific Detection)                  │
│  ├── OJS Template        → submission_files, users, journals   │
│  ├── WordPress Template  → wp_posts, wp_uploads (future)       │
│  ├── Drupal Template     → node, files_managed (future)        │
│  └── Custom Template    → (user-defined rules)               │
└─────────────────────────────────────────────────────────────┘
```

### Template System Benefits

1. **Single Platform, Multiple CMS** - One FIM installation monitors different CMS
2. **CMS-Specific Detection** - OJS knows about submissions, WordPress knows about posts
3. **Extensible** - Add new CMS support via template interface
4. **No Code Duplication** - Generic scanner reused across templates

---

## Current State Analysis

### Current File Structure
```
backend/
├── main.go          (107 lines)  ✅ OK
├── auth.go          (55 lines)   ✅ OK
├── models.go        (132 lines)  ✅ OK
├── db.go           (486 lines)  🔴 OVER LIMIT
├── handlers.go     (1322 lines) 🔴 OVER LIMIT
├── scanner.go      (305 lines)  🔴 OVER LIMIT
├── worker.go       (563 lines)  🔴 OVER LIMIT
└── watcher.go      (812 lines)  🔴 OVER LIMIT
```

### Problems with Current Architecture

1. **Monolithic handlers.go (1322 lines)**
   - 29 HTTP handler functions
   - Mixed concerns (auth, projects, scans, files, FIM)
   - Hard to test individual handlers

2. **Database Layer Mixed with Business Logic**
   - db.go contains both SQL migrations AND queries AND business logic
   - Direct SQL in handlers.go
   - No repository pattern

3. **Tight Coupling**
   - All packages in `main` package
   - No interfaces for testability

4. **OJS Hardcoded**
   - OJS logic mixed with generic FIM
   - No template/plugin abstraction

5. **Missing Test Coverage**
   - No test files

---

## Clean Architecture Design

### Proposed Package Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, wire dependencies
│
├── internal/
│   ├── domain/                  # Enterprise Business Rules (innermost)
│   │   ├── models/
│   │   │   └── models.go        # All data structures
│   │   ├── repository/          # Repository interfaces
│   │   │   ├── project.go       # ProjectRepository
│   │   │   ├── job.go          # JobRepository
│   │   │   ├── file.go         # FileRepository
│   │   │   └── fim.go          # FIMRepository
│   │   ├── template/            # Template interfaces (KEY!)
│   │   │   └── template.go     # TemplateDetector interface
│   │   └── service/            # Service interfaces
│   │       ├── scanner.go      # ScannerService
│   │       ├── watcher.go     # WatcherService
│   │       └── worker.go      # WorkerService
│   │
│   ├── templates/               # Template Implementations
│   │   ├── ojs/
│   │   │   ├── detector.go    # OJS orphan detection
│   │   │   ├── metrics.go    # OJS-specific metrics
│   │   │   └── validator.go  # OJS integrity rules
│   │   ├── wordpress/          # (future)
│   │   └── drupal/            # (future)
│   │
│   ├── application/            # Application Business Rules
│   │   ├── usecase/
│   │   │   ├── project/
│   │   │   │   ├── create.go
│   │   │   │   ├── update.go
│   │   │   │   ├── list.go
│   │   │   └── delete.go
│   │   │   ├── scan/
│   │   │   │   ├── start_baseline.go
│   │   │   │   ├── start_integrity.go
│   │   │   │   └── cancel.go
│   │   │   └── fim/
│   │   │       ├── start_watcher.go
│   │   │       ├── stop_watcher.go
│   │   │       └── get_events.go
│   │   └── dto/
│   │       ├── request/
│   │       │   └── project.go
│   │       └── response/
│   │           ├── project.go
│   │           ├── metrics.go
│   │           └── fim.go
│   │
│   ├── infrastructure/          # Frameworks & Drivers (outermost)
│   │   ├── database/
│   │   │   ├── sqlite/
│   │   │   │   ├── sqlite.go       # Connection, migrations
│   │   │   │   ├── project_repo.go
│   │   │   │   ├── job_repo.go
│   │   │   │   ├── file_repo.go
│   │   │   └── fim_repo.go
│   │   ├── mysql/
│   │   │   └── ojs.go           # CMS database connections
│   │   └── http/
│   │       ├── router.go
│   │       ├── middleware/
│   │       │   ├── auth.go
│   │       │   └── logging.go
│   │       └── handlers/
│   │           ├── project.go
│   │           ├── scan.go
│   │           ├── file.go
│   │           └── fim.go
│   ├── worker/
│   │   ├── worker.go            # Background worker
│   │   ├── scanner.go           # Generic file scanning
│   │   └── reconciler.go        # Template-based reconciliation
│   └── watcher/
│       ├── watcher.go            # FIM watcher
│       └── inotify.go           # inotify wrapper
│
├── pkg/
│   └── response/
│       └── response.go
│
└── *_test.go files
```

---

## Template System Design (Key Feature)

### Template Interface

```go
// internal/domain/template/template.go

// Template represents a CMS-specific detection strategy
type Template interface {
    // Name returns the template identifier
    Name() string

    // Priority returns template priority (higher = checked first)
    Priority() int

    // DetectOrphans finds files not in CMS database
    DetectOrphans(ctx context.Context, db *mysql.OJS, files []*models.ProjectFile) ([]*models.ProjectFile, error)

    // GetMetrics returns CMS-specific dashboard metrics
    GetMetrics(ctx context.Context, db *mysql.OJS) (*TemplateMetrics, error)

    // ValidateIntegrity checks CMS-specific integrity rules
    ValidateIntegrity(ctx context.Context, db *mysql.OJS, project *models.Project) error

    // GetDBConfig returns required database config for this template
    RequiredDBConfig() []string
}

// TemplateRegistry manages all available templates
type TemplateRegistry struct {
    templates map[string]Template
    mutex    sync.RWMutex
}

// Register adds a template to the registry
func (r *TemplateRegistry) Register(t Template)

// Get returns template by name
func (r *TemplateRegistry) Get(name string) (Template, error)

// DetectTemplate auto-detects CMS type from database
func (r *TemplateRegistry) DetectTemplate(ctx context.Context, db *mysql.OJS) (Template, error)
```

### Template Metrics

```go
// TemplateMetrics holds CMS-specific metrics
type TemplateMetrics struct {
    TemplateName string
    Generic     *models.DashboardMetrics  // Platform-wide metrics
    Specific    map[string]interface{}   // Template-specific (e.g., "pending_submissions", "unpublished_articles")
}
```

### OJS Template Implementation

```go
// internal/templates/ojs/detector.go

type OJSTemplate struct{}

func (t *OJSTemplate) Name() string { return "ojs" }
func (t *OJSTemplate) Priority() int { return 100 } // High priority for OJS 3.x/2.x

func (t *OJSTemplate) DetectOrphans(ctx context.Context, db *mysql.OJS, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
    var orphans []*models.ProjectFile

    for _, f := range files {
        // Check submission_files table
        var count int
        err := db.QueryRowContext(ctx,
            "SELECT COUNT(*) FROM submission_files WHERE original_file_name = ?",
            filepath.Base(f.FilePath)).Scan(&count)

        if err != nil || count == 0 {
            f.Status = "ORPHAN"
            orphans = append(orphans, f)
        }
    }
    return orphans, nil
}

func (t *OJSTemplate) GetMetrics(ctx context.Context, db *mysql.OJS) (*TemplateMetrics, error) {
    // Query OJS-specific metrics: pending_submissions, unvalidated_users, etc.
}

func (t *OJSTemplate) RequiredDBConfig() []string {
    return []string{"db_host", "db_user", "db_pass", "db_name"}
}
```

### WordPress Template (Future)

```go
// internal/templates/wordpress/detector.go

type WordPressTemplate struct{}

func (t *WordPressTemplate) Name() string { return "wordpress" }
func (t *WordPressTemplate) Priority() int { return 90 }

func (t *WordPressTemplate) DetectOrphans(ctx context.Context, db *mysql.OJS, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
    // Check wp_posts, wp_uploads, wp_postmeta tables
}
```

---

## Layer Responsibilities

### 1. Domain Layer (`internal/domain/`)

- **Pure business logic with NO external dependencies**
- Contains: Models, Repository Interfaces, **Template Interface**
- Example: `Template` interface defines detection methods without implementation

### 2. Templates Layer (`internal/templates/`)

- **CMS-specific implementations**
- Implements `Template` interface
- OJS, WordPress, Drupal, etc.

### 3. Application Layer (`internal/application/`)

- **Use cases orchestrate domain logic**
- Uses Template interface (not concrete implementation)
- Generic scanner with injected template

### 4. Infrastructure Layer (`internal/infrastructure/`)

- **Implements domain interfaces**
- Generic scanner, watcher, worker

---

## Refactoring Phases

### Phase 1: Domain Layer ✅ COMPLETED
- Models
- Repository interfaces
- Template interface
- Service interfaces

### Phase 2: Infrastructure Layer ✅ COMPLETED
- SQLite repositories
- MySQL connections
- Database schema

### Phase 3: Template System (PENDING)
- Create `internal/templates/ojs/`
- OJS orphan detection
- OJS metrics
- Template registry

### Phase 4: Application Layer (PENDING)
- Use cases with template injection
- DTOs

### Phase 5: Presentation Layer (PENDING)
- HTTP handlers
- Middleware

### Phase 6: Worker & Watcher (PENDING)
- Generic scanner with template support
- Template-based reconciliation

### Phase 7: Entry Point (PENDING)
- Wire dependencies
- Template registration

### Phase 8: Testing (PENDING)
- Unit tests
- Integration tests
- Template mock tests

---

## File-by-File Plan (Phase 3)

### Create Template System

```
internal/domain/template/
└── template.go          # Template interface + registry

internal/templates/
└── ojs/
    ├── detector.go      # Orphan detection
    ├── metrics.go       # OJS metrics
    └── validator.go      # Integrity rules
```

### Template Registry

```go
// internal/domain/template/registry.go
package template

func NewRegistry() *Registry {
    r := &Registry{
        templates: make(map[string]Template),
    }
    // Register built-in templates
    r.Register(&ojs.OJSTemplate{})
    return r
}
```

---

## Success Criteria

1. **File Size:** All files < 200 lines
2. **Test Coverage:** Minimum 70% coverage
3. **Template Isolation:** Templates don't import each other
4. **Generic Core:** Scanner/watcher/worker work without CMS templates
5. **Plugin Registration:** New templates can be added via registry

---

## Template Extension Guide

### Adding WordPress Support

1. Create `internal/templates/wordpress/detector.go`
2. Implement `Template` interface
3. Register in registry:

```go
// cmd/server/main.go
registry := template.NewRegistry()
registry.Register(&wordpress.WordPressTemplate{})
```

### Adding Custom Detection Rules

```go
type CustomTemplate struct{}

func (t *CustomTemplate) DetectOrphans(...) {
    // User-defined logic
}
```

---

## Notes

- Templates are **plugins** - platform works with or without specific CMS
- Generic scanner is the **core** - templates enhance detection
- Repository pattern enables easy mocking for tests
- Use dependency injection for template registration
