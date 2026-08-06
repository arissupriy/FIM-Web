# CLAUDE.md

## AI Efficiency Stack (Auto-Enabled)

```
Caveman → ALREADY ON (65% compression)
ai-local → RUN ON SESSION START
graphify → Use for code navigation
mem0 → Query for context
```

---

### SESSION START SEQUENCE (REQUIRED)

When conversation starts, IMMEDIATELY run:

```bash
ai-local context && ai-local status
```

Then proceed with user requests using local context.

---

### Commands Reference:

| Command | When | API Cost |
|---------|------|----------|
| `ai-local context` | Session start | 0 |
| `ai-local status` | Session start | 0 |
| `graphify query "..."` | When asking about code | 0 |
| `graphify path A B` | Find relationship | 0 |
| `mcp__mem0-memory__search_memories` | Get memories | Minimal |

---

### EFFICIENCY RULES

**ALWAYS:**
- Use `ai-local context` first (zero cost)
- Use `graphify query` before reading files
- Batch reads when possible

**NEVER:**
- Read files without checking graph first
- Multiple small edits (batch instead)
- Re-run same query (results are cached)

GOOD (efficient):
  → graphify query "authentication flow"
  → Read auth.go, helpers.go together
  → Make fixes
  → Test
  → Done (3-4 API calls)
```

### Tool Usage:

| Tool | When to Use | When NOT to |
|------|-------------|-------------|
| Read | Batch reads | Single quick check |
| Edit | One precise edit | Large rewrites (use Write) |
| Bash | One command | Long scripts |
| Grep | Find patterns | Count lines (use wc) |

### Files:
- `.ai-context.json` - Project config (per-project)
- `graphify-out/AI_CONTEXT.md` - Auto-generated context
- `graphify-out/graph.json` - Knowledge graph

> **PENTING:** Setiap session baru, WAJIB check Mem0 terlebih dahulu!

---

## Core Principle

```
┌─────────────────────────────────────────────────────────────────┐
│  CORE (infrastructure) = AGNORSTIC                              │
│  • Scanner, Watcher, Worker - generik, tidak tahu template      │
│  • Tidak pernah import templates/ langsung                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ uses interfaces
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  DOMAIN (interfaces) = GENERIK                                   │
│  • Template interface - tidak tahu OJS/WordPress                │
│  • Repository interfaces                                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ implements
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  TEMPLATES = SPESIFIK                                           │
│  • templates/ojs/ - semua logic yang tahu OJS                  │
│  • templates/wordpress/ - (future)                             │
└─────────────────────────────────────────────────────────────────┘
```

**Dependency Rule**: Core → Domain → Templates (inward only)

---

## Platform Purpose

```
┌─────────────────────────────────────────────────────────────────┐
│                     FIM MONITOR PLATFORM                       │
├─────────────────────────────────────────────────────────────────┤
│  Core Engine (Generic - works with or without template)         │
│  ├── File Scanner (Hash, Permission, Metadata)                 │
│  ├── FIM Watcher (fsnotify)                                   │
│  ├── Background Worker (Job Queue)                             │
│  └── Database (SQLite - Platform Schema)                       │
├─────────────────────────────────────────────────────────────────┤
│  Templates (CMS-Specific - Optional enrichment layer)           │
│  ├── OJS Template       → submission_files, users, journals   │
│  ├── WordPress Template → wp_posts, wp_uploads (future)        │
│  └── Custom Template   → (user-defined rules)                 │
└─────────────────────────────────────────────────────────────────┘
```

**Important**: A project can work WITHOUT a template (pure FIM for generic VPS). Template is an **optional enrichment layer**.

---

## Backend Architecture

### Directory Structure

```
backend/
├── cmd/                        # Entry points
│   ├── manage/                 # CLI management tool
│   ├── server/                 # HTTP API server
│   └── worker/                 # Background worker
│
├── bin/                        # Compiled binaries
│
├── internal/                   # Private application code
│   ├── domain/                 # Interfaces (agnostic)
│   │   ├── models/            # Data structures
│   │   ├── repository/        # Repository interfaces
│   │   └── template/          # Template interface (generik)
│   │
│   ├── application/            # Use cases
│   │   ├── usecase/           # Business operations
│   │   └── dto/               # Data Transfer Objects
│   │
│   ├── infrastructure/         # Implementations (generik)
│   │   ├── database/sqlite/   # SQLite repos
│   │   ├── http/              # HTTP handlers
│   │   ├── auth/              # Authentication
│   │   ├── scanner/            # File scanning (generik)
│   │   ├── watcher/           # FIM watcher (generik)
│   │   ├── worker/            # Background worker (generik)
│   │   └── alert/             # Alert dispatcher
│   │
│   ├── templates/              # CMS-specific implementations
│   │   └── ojs/               # OJS template
│   │       ├── service.go      # Implements template.Template
│   │       ├── config.go      # DefaultConfig
│   │       ├── correlator.go  # CorrelateFile
│   │       ├── orphan.go      # DetectOrphans
│   │       ├── metrics.go     # GetMetrics
│   │       ├── integrity.go   # ValidateIntegrity
│   │       ├── detector.go    # Version detection
│   │       ├── handlers.go    # HTTP handlers
│   │       └── mysql/         # OJS MySQL queries
│   │           ├── connection.go
│   │           └── queries.go
│   │
│   └── wire/                   # Dependency injection
│
└── database/
    ├── migrations/             # Database migrations
    └── ojs_monitor.db         # SQLite database
```

---

## Template System

### Architecture

```
domain/template/ (interfaces - GENERIK)
├── template.go      # Template interface
└── registry.go     # Template registry

templates/ojs/ (implementasi - SPESIFIK)
├── service.go      # implements template.Template (entry point)
├── config.go      # DefaultConfig()
├── correlator.go  # CorrelateFile()
├── orphan.go      # DetectOrphans()
├── metrics.go     # GetMetrics()
├── integrity.go   # ValidateIntegrity()
└── mysql/         # OJS-specific database code
```

### Template Interface

```go
type Template interface {
    Name() string                                    // "ojs", "wordpress"
    Version() string                                 // "3.x", "6.x"
    Priority() int                                   // Detection priority
    
    DefaultConfig() *TemplateConfig                  // Default settings
    
    CreateDBConnection(ctx, config) DBConnection    // CMS-specific connection
    
    // Optional - return nil if not supported
    DetectOrphans(ctx, db, files) ([]*ProjectFile, error)
    GetMetrics(ctx, db) (*TemplateMetrics, error)
    ValidateIntegrity(ctx, db, project) ([]IntegrityWarning, error)
    CorrelateFile(ctx, db, filePath, eventType) (*CorrelationResult, error)
    
    RequiredDBConfig() []string                      // Required DB fields
    Compatible(ctx, db) (bool, error)             // Auto-detect
}
```

### Project Without Template

Projects can exist WITHOUT a template (pure FIM mode):

```sql
-- template_id is nullable
projects.template_id INTEGER REFERENCES templates(id)
```

The project works with generic FIM:
- File scanning (hash, permissions)
- Real-time watching (fsnotify)
- Alert dispatching

Template enrichment is **optional**:
- Correlation (who uploaded/modified)
- Orphan detection
- CMS-specific metrics

---

## Layer Responsibilities

### Domain Layer (`internal/domain/`)

**AGNOSTIC** - Pure interfaces, no external dependencies:
- `models/` - Data structures (Project, ProjectFile, FIMEvent)
- `repository/` - Repository interfaces
- `template/` - Template interface (interface only, no template knowledge)

### Application Layer (`internal/application/`)

**USE CASES** - Business logic orchestration:
- `usecase/scan/` - Scan operations
- `usecase/fim/` - FIM event operations
- `usecase/project/` - Project CRUD
- `usecase/job/` - Job management

### Infrastructure Layer (`internal/infrastructure/`)

**GENERIC** - Implements interfaces, no CMS knowledge:
- `database/sqlite/` - SQLite repositories
- `http/` - HTTP handlers
- `scanner/` - Generic file scanning
- `watcher/` - Real-time FIM using fsnotify
- `worker/` - Background job processor
- `alert/` - Alert dispatcher

### Templates Layer (`internal/templates/`)

**SPECIFIC** - CMS-specific implementations:
- `templates/ojs/` - All OJS-specific logic
- `templates/wordpress/` - (future)
- `templates/drupal/` - (future)

---

## Adding a New Template

1. Create directory: `internal/templates/<name>/`
2. Implement `template.Template` interface
3. Create `mysql/` package for CMS-specific queries
4. Register in `init.go`:

```go
// internal/templates/wordpress/init.go
func init() {
    template.Register(New())
}
```

---

## Database Schema

### Projects Table

```sql
CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    template TEXT,                    -- Nullable (generic projects allowed)
    template_id INTEGER REFERENCES templates(id),  -- Nullable
    template_version TEXT,
    -- ... other fields
);
```

### Templates Table

```sql
CREATE TABLE templates (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,       -- "ojs", "wordpress"
    version TEXT NOT NULL,            -- "3.x", "6.x"
    priority INTEGER DEFAULT 100,
    default_config TEXT,              -- JSON config
    enabled INTEGER DEFAULT 1
);
```

---

## CLI Workflow

### 1. Initial Setup

```bash
cd backend
make build
./manage migrate
./manage seed  # Creates admin/admin123
```

### 2. Start Services

```bash
# Daemon mode
./manage server:start

# Or separately
./fim-server    # Terminal 1
./worker        # Terminal 2
```

---

## Makefile Targets

```bash
make build              # Build all binaries
make test             # Run tests
make server:start     # Start services
make server:stop      # Stop services
```

---

## Working Directory

Always work from:
```
/home/arissupriy/stai/ojs-monitor/backend
```

---

## Build & Test

```bash
go build ./...      # Build all packages
go test ./...       # Run all tests
```

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

Allowed: `git diff`, `git status`, `git show`, `git log`
Disallowed: `git push`, `git merge`, `git rebase`, `git force push`

---

## Output Format

When done, always report:
1. Summary of changes
2. Files modified
3. Reason for changes
4. Validations run
5. Remaining risks

Don't say something succeeded without verification.

## AI Memory System (Graphify + Mem0 + Obsidian)

### MEMORY COLLECTIONS (Named Memories)

**WAJIB query memory ini di setiap session baru:**

| Collection | Query Pattern | Contents |
|------------|--------------|----------|
| `ojs-monitor:context` | "ojs monitor project overview architecture" | Project overview, tech stack, purpose |
| `ojs-monitor:decisions` | "ojs monitor design decision architecture rationale" | Design decisions & why |
| `ojs-monitor:progress` | "ojs monitor progress implementation milestone" | Current status, what's done |
| `ojs-monitor:preferences` | "ojs monitor user preference settings config" | User preferences |

### MEMORY WORKFLOW (MANDATORY)

```
SESSION START
    │
    ├─► 1. mcp__mem0-memory__search_memories(
    │       query="ojs monitor project overview architecture",
    │       user_id="ojs-monitor", top_k=5)
    │
    ├─► 2. mcp__mem0-memory__search_memories(
    │       query="ojs monitor design decision",
    │       user_id="ojs-monitor", top_k=3)
    │
    ├─► 3. mcp__mem0-memory__search_memories(
    │       query="ojs monitor progress implementation",
    │       user_id="ojs-monitor", top_k=3)
    │
    └─► 4. graphify query → Get code context
            ↓
        Answer user question
            ↓
        If major decision → SAVE to Mem0
            ↓
        If code changed → graphify update + obsidian export
```

### SAVE MEMORY COMMANDS

```bash
# Save to ojs-monitor:context
mcp__mem0-memory__add_memory(
    text="[ojs-monitor:context] OJS Monitor - FIM Platform...",
    user_id="ojs-monitor"
)

# Save to ojs-monitor:decisions
mcp__mem0-memory__add_memory(
    text="[ojs-monitor:decisions] Decision: fsnotify for FIM...",
    user_id="ojs-monitor"
)

# Save to ojs-monitor:progress
mcp__mem0-memory__add_memory(
    text="[ojs-monitor:progress] Phase 1: Scanner complete...",
    user_id="ojs-monitor"
)
```

### MEMORY COLLECTIONS (Named Memories)

**WAJIB query memory ini di setiap session:**

| Memory Name | Query Pattern | Contents |
|-------------|--------------|----------|
| `ojs-monitor:context` | "ojs monitor fim scanner watcher context" | Project overview, architecture, tech stack |
| `ojs-monitor:decisions` | "ojs monitor design decision architecture" | Design decisions & rationale |
| `ojs-monitor:progress` | "ojs monitor progress fase implementasi" | Implementation progress & milestones |
| `ojs-monitor:preferences` | "ojs monitor user preference settings" | User preferences for this project |

**Commands:**
```bash
# Search memories by pattern
mcp__mem0-memory__search_memories(query="ojs monitor context", user_id="ojs-monitor", top_k=5)

# List all project memories
mcp__mem0-memory__get_memories(user_id="ojs-monitor", top_k=50)

# Save new memory
mcp__mem0-memory__add_memory(
    user_id="ojs-monitor",
    text="Memory content here",
    metadata={"collection": "ojs-monitor:context", "type": "project-info"}
)
```

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes_tool` or `query_graph_tool` instead of Grep
- **Understanding impact**: `get_impact_radius_tool` instead of manually tracing imports
- **Code review**: `detect_changes_tool` + `get_review_context_tool` instead of reading entire files
- **Finding relationships**: `query_graph_tool` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview_tool` + `list_communities_tool`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### Key Tools

| Tool | Use when |
| ------ | ---------- |
| `detect_changes_tool` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context_tool` | Need source snippets for review — token-efficient |
| `get_impact_radius_tool` | Understanding blast radius of a change |
| `get_affected_flows_tool` | Finding which execution paths are impacted |
| `query_graph_tool` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes_tool` | Finding functions/classes by name or keyword |
| `get_architecture_overview_tool` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes_tool` for code review.
3. Use `get_affected_flows_tool` to understand impact.
4. Use `query_graph_tool` pattern="tests_for" to check coverage.
