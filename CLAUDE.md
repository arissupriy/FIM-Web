# CLAUDE.md

## Project Overview

**OJS Monitor** is a **FIM (File Integrity Monitoring) Platform** - not just for OJS.

> OJS is the first **dedicated template**. The platform is generic and can support multiple CMS/systems via template plugins (WordPress, Drupal, custom).

---

## Platform Purpose

```
┌─────────────────────────────────────────────────────────────────┐
│                     FIM MONITOR PLATFORM                       │
│              (Generic File Integrity Monitoring)                  │
├─────────────────────────────────────────────────────────────┤
│  Core Engine (Platform-wide)                                   │
│  ├── File Scanner (Hash, Permission, Metadata)                 │
│  ├── FIM Watcher (fsnotifywait)                                │
│  ├── Background Worker (Job Queue)                             │
│  └── Database (SQLite - Platform Schema)                       │
├─────────────────────────────────────────────────────────────┤
│  Templates/Plugins (CMS-Specific Detection)                     │
│  ├── OJS Template       → submission_files, users, journals   │
│  ├── WordPress Template → wp_posts, wp_uploads (future)        │
│  └── Custom Template   → (user-defined rules)                 │
└─────────────────────────────────────────────────────────────┘
```

---

## Backend Architecture

### Clean Architecture Structure

```
backend/
├── cmd/                        # Entry points (source code)
│   ├── manage/                 # CLI management tool
│   │   ├── main.go
│   │   └── process/           # PID management for server control
│   │       └── process.go
│   ├── server/                 # HTTP API server
│   │   └── main.go
│   └── worker/                 # Background worker
│       └── main.go
│
├── bin/                        # Compiled binaries
│   ├── manage                  # CLI tool
│   ├── fim-server             # HTTP API server
│   └── worker                 # Background worker
│
├── pkg/                        # Shared packages
│   └── response/               # HTTP response helpers
│       └── response.go
│
├── internal/                   # Private application code
│   ├── domain/                 # Enterprise Business Rules (innermost)
│   │   ├── models/             # Domain models
│   │   ├── repository/         # Repository interfaces
│   │   ├── template/           # Template interface
│   │   └── service/            # Service interfaces
│   │
│   ├── application/            # Application Business Rules
│   │   ├── usecase/           # Use cases
│   │   └── dto/                # Data Transfer Objects
│   │
│   ├── infrastructure/         # Frameworks & Drivers (outermost)
│   │   ├── database/sqlite/    # SQLite implementations
│   │   ├── database/mysql/      # MySQL connections
│   │   ├── http/               # HTTP handlers & middleware
│   │   ├── auth/               # Authentication
│   │   ├── scanner/            # File scanning
│   │   ├── watcher/            # FIM watcher (fsnotify)
│   │   ├── worker/             # Background worker
│   │   └── templates/           # Template implementations
│   │       └── ojs/            # OJS template (first template)
│   │
│   └── wire/                   # Dependency injection
│
├── database/                   # SQLite database files
│   └── ojs_monitor.db
│
└── data/                      # Data files
```

### Domain Layer (`internal/domain/`)

Contains **pure business logic** with NO external dependencies:
- `models/` - Data structures (Project, ProjectFile, FIMEvent, etc.)
- `repository/` - Repository interfaces (ProjectRepository, FileRepository, etc.)
- `template/` - Template interface for CMS-specific detection
- `service/` - Service interfaces

### Application Layer (`internal/application/`)

Contains **use cases** that orchestrate domain logic:
- `usecase/` - Business operations (scan, fim, project, job, file, auth)
- `dto/` - Request/Response DTOs

### Infrastructure Layer (`internal/infrastructure/`)

Implements domain interfaces with **concrete technology**:
- `database/sqlite/` - SQLite repositories
- `database/mysql/` - MySQL connections for CMS databases
- `http/` - HTTP handlers and router
- `auth/` - Authentication service
- `scanner/` - Generic file scanning
- `watcher/` - Real-time FIM using fsnotifywait
- `worker/` - Background job processor
- `templates/` - CMS-specific implementations

---

## Binaries

| Binary | Purpose | Entry Point |
|--------|---------|-------------|
| `manage` | CLI tool for DB management & server control | `cmd/manage/` |
| `fim-server` | HTTP API server | `cmd/server/` |
| `worker` | Background job processor | `cmd/worker/` |

---

## CLI Workflow

### 1. Initial Setup

```bash
# Build all binaries
cd backend
make build

# Run migrations
./manage migrate

# Create admin user
./manage add-admin <username> <password>

# Or use default admin
./manage seed  # Creates admin/admin123
```

### 2. Start Services (Daemon Mode)

```bash
# Start server + worker together (daemon mode)
./manage server:start

# Check service status
./manage server:status

# Stop services
./manage server:stop

# Restart services
./manage server:restart
```

### 3. Or Start Separately (Manual Mode)

```bash
# Terminal 1: Start API server
./fim-server

# Terminal 2: Start background worker
./worker
```

### 4. Check Status

```bash
./manage status
```

---

## Server Management

The `manage` CLI controls server and worker processes using **PID files**:

| Command | Description |
|---------|-------------|
| `manage server:start` | Start server + worker (daemon) |
| `manage server:stop` | Stop server + worker gracefully |
| `manage server:restart` | Restart all services |
| `manage server:status` | Show service status |

PID files are stored in the same directory as binaries:
- `.server.pid` - Server process ID
- `.worker.pid` - Worker process ID

---

## Makefile Targets

```bash
make build              # Build all binaries
make clean            # Remove binaries
make test             # Run tests
make test-race        # Run tests with race detector
make status           # Show system status
make migrate          # Run migrations
make server:start     # Start server + worker
make server:stop      # Stop server + worker
make server:restart   # Restart services
make server:status    # Show service status
```

---

## Template System

Templates provide **CMS-specific detection logic**:

### OJS Template (`internal/templates/ojs/`)
- Orphan detection (files not in submission_files)
- User metrics (new users, validated users)
- Journal statistics
- Version detection

### Future Templates
- WordPress Template
- Drupal Template
- Custom Template

### Adding a New Template

1. Create `internal/templates/<name>/`
2. Implement `Template` interface
3. Register in application

---

## Working Directory

Always work from:
```
/home/arissupriy/stai/ojs-monitor/backend
```

---

## Build Rules

All builds from repository root (`/home/arissupriy/stai/ojs-monitor`):

```bash
make build           # Build all binaries to bin/
cd backend
go build ./...      # Build all packages
go test ./...       # Run all tests
go test -race ./... # Race detector
```

Binary outputs (in `backend/bin/`):
- `backend/bin/manage`
- `backend/bin/fim-server`
- `backend/bin/worker`

---

## Database

SQLite database:
```
backend/database/ojs_monitor.db
```

Migrations run automatically on startup via `wire.InitDB()`.

---

## Security Rules

Always consider:
- Path traversal
- Symlink attack
- TOCTOU race conditions
- Permission issues
- Command injection
- Resource exhaustion

Treat all input as untrusted.

---

## Modification Policy

Make **minimal changes**:
- No large refactors
- No mass renaming
- No structural changes
- No API changes

Unless explicitly requested.

---

## Git Policy

Allowed:
- `git diff`
- `git status`
- `git show`
- `git log`

Disallowed (unless requested):
- `git push`
- `git merge`
- `git rebase`
- `git tag`
- `git force push`

---

## Output Format

When done, always report:
1. Summary of changes
2. Files modified
3. Reason for changes
4. Validations run
5. Remaining risks

Don't say something succeeded without verification.

---

## Engineering Mindset

Before changing code:
1. Understand the architecture
2. Find root cause
3. Make minimal changes
4. Verify results
5. Avoid regressions

Prioritize correctness over speed.
