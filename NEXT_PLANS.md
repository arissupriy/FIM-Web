# NEXT_PLANS.md

**Project:** OJS Monitor - Enterprise FIM Upgrade
**Purpose:** Generic FIM Platform dengan CMS-specific templates
**Status:** Pre-flight decisions resolved

> ⚠️ **IMPORTANT: Integration Required**
> Semua fase HARUS terhubung ke FIM Core. Jangan buat fase sebagai standalone module.
> Setiap fase yang dibuat HARUS di-integrasi, bukan hanya diimplementasi.

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|---------|------------|
| Alert Channels | Email + Slack + Webhook | ALL |
| SIEM Platform | Elasticsearch (ELK) | Syslog fallback |
| Compliance | SOC2 primary + NIST mapping | Executive summary |
| FIM Library | **fsnotify** (native Go) | Clean, no external deps |
| Queue | Reuse existing job queue | Single failure path |
| Dedup | 60 seconds per file+risk_level | Bursty deploys won't flood alerts |
| Schema | Rule-based (single-condition start | Grow without rewrite |
| Permission Tracking | stat() before/after diff | Exact change detection |

---

## ⚠️ INTEGRATION ARCHITECTURE (WAJIB)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           FIM CORE                                       │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │   Scanner   │───▶│  FIMEvent   │◀───│   Watcher   │                 │
│  └─────────────┘    └──────┬──────┘    └─────────────┘                 │
│                            │                                              │
│         ┌──────────────────┼──────────────────┐                          │
│         │                  │                  │                          │
│         ▼                  ▼                  ▼                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │   Alert     │    │   Auditd    │    │    ACL      │                 │
│  │  (2A/2B)   │    │   (2C)     │    │   (2D)     │                 │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                 │
│         │                  │                  │                          │
│         └──────────────────┼──────────────────┘                          │
│                            │                                              │
│                            ▼                                              │
│                   ┌─────────────────┐                                    │
│                   │   Dispatcher     │                                    │
│                   │   (Alert Core)   │                                    │
│                   └────────┬─────────┘                                    │
│                            │                                              │
│         ┌─────────────────┼─────────────────┐                            │
│         │                 │                 │                            │
│         ▼                 ▼                 ▼                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                   │
│  │   Email     │    │   Slack     │    │   Webhook   │                   │
│  └─────────────┘    └─────────────┘    └─────────────┘                   │
│                                                                         │
│                            │                                              │
│                            ▼                                              │
│                   ┌─────────────────┐                                    │
│                   │   SIEM Export    │                                    │
│                   │     (2E)        │                                    │
│                   └────────┬─────────┘                                    │
│                            │                                              │
│         ┌─────────────────┼─────────────────┐                            │
│         ▼                 ▼                 ▼                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                   │
│  │  Syslog     │    │ Elasticsearch│    │  Splunk HEC │                   │
│  └─────────────┘    └─────────────┘    └─────────────┘                   │
│                                                                         │
│                            │                                              │
│                            ▼                                              │
│                   ┌─────────────────┐                                    │
│                   │   Compliance    │                                    │
│                   │     (3)        │                                    │
│                   └─────────────────┘                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Pre-flight Checklist

- [x] **fsnotify migration** - Native Go fsnotify (not inotifywait)
- [x] **Decision: Alert schema** - Rule-based (single-condition)
- [x] **Decision: Queue reuse** - Reuse job queue
- [x] **Decision: Dedup window** - 60 seconds
- [x] **Decision: Permission diff** - stat() before/after comparison
- [x] **P1-05 stat() diff** - Permission changes detected
- [x] **P1-06 flag permission changes** - HIGH risk detection

---

## Dependency Graph

```
Phase 2A: Alert Core (5 tasks)
        │
Phase 2B: Config + Watcher Integration (2 tasks)
        │
        ├──────────────────────┐
        │                      │
Phase 2C: auditd Integration  Phase 2D: ACL/xattr/SELinux
       (4 tasks)                 (3 tasks)
        │                      │
        └──────────┬───────────┘
                   │
Phase 2E: SIEM Export (6 tasks)
        │
Phase 3: Compliance + Hash Chain (7 tasks)
        │
Phase 4: User Experience (3 tasks)
```

---

## Phase 2A: Alerting Core ✅ CONNECTED

**Status:** ✅ TERINTEGRASI dengan FIM Core
**Depends on:** Pre-flight decisions resolved ✓

| Task | Description | Status | Integration |
|------|-------------|--------|-------------|
| ~~P2-01~~ | ~~Alert database schema~~ | ✅ Done | ✅ `alert_configs`, `alert_history` tables |
| ~~P2-02~~ | ~~Alert dispatcher (reuse job queue)~~ | ✅ Done | ✅ `globalAlertDispatcher` in watcher.go |
| ~~P2-02a~~ | ~~Rate limiting (60s dedup window)~~ | ✅ Done | ✅ In dispatcher.go |
| ~~P2-03~~ | ~~Email channel (SMTP)~~ | ✅ Done | ✅ `EmailChannel` implements `Channel` |
| ~~P2-04~~ | ~~Slack channel (webhook)~~ | ✅ Done | ✅ `SlackChannel` implements `Channel` |
| ~~P2-05~~ | ~~Webhook channel (custom URL)~~ | ✅ Done | ✅ `WebhookChannel` implements `Channel` |

**Output:** Alert infrastructure with reusable channel pattern.

---

## Phase 2B: Configuration + Watcher Integration ✅ CONNECTED

**Status:** ✅ TERINTEGRASI dengan FIM Core
**Depends on:** Phase 2A dispatcher + channels working end-to-end

| Task | Description | Status | Integration |
|------|-------------|--------|-------------|
| ~~P2-06~~ | ~~Alert config API~~ | ✅ Done | ✅ `/api/v1/alerts/*` routes |
| ~~P2-07~~ | ~~Watcher integration (fsnotify → dispatcher)~~ | ✅ Done | ✅ `watcher.go` calls `SetAlertDispatcher` |

**Output:** Alerts fire on HIGH/CRITICAL events.

---

## Phase 2C: auditd Integration ⚠️ NEEDS INTEGRATION

**Status:** ⚠️ CODE ADA, BELUM TERINTEGRASI
**Depends on:** Phase 2A dispatcher + stat() diff verified

| Task | Description | Status | Integration Required |
|------|-------------|--------|---------------------|
| ~~P2-A1~~ | ~~audit.log parser~~ | ✅ Done | ❌ Wire to FIMEvent |
| ~~P2-A2~~ | ~~audit rules generator~~ | ✅ Done | ❌ Wire to FIMEvent |
| ~~P2-A3~~ | ~~FIM + audit correlation~~ | ✅ Done | ❌ Wire to Dispatcher |
| ~~P2-A4~~ | ~~Actor attribution (user/pid)~~ | ✅ Done | ❌ Wire to FIMEvent |

**Files:**
- `internal/infrastructure/audit/parser.go`
- `internal/infrastructure/audit/rules.go`
- `internal/infrastructure/audit/correlation.go`
- `internal/infrastructure/audit/attribution.go`

**Integration Required:**
```
audit/parser.go → FIMEvent → Dispatcher → Alert
                ↓
           enrich actor info (user/pid from audit.log)
```

---

## Phase 2D: ACL / xattr / SELinux ⚠️ NEEDS INTEGRATION

**Status:** ⚠️ CODE ADA, BELUM TERINTEGRASI
**Depends on:** Phase 2A dispatcher (reuses same alert pattern as P1-05/P1-06)

| Task | Description | Status | Integration Required |
|------|-------------|--------|---------------------|
| ~~P2-08~~ | ~~ACL monitoring (getfacl)~~ | ✅ Done | ❌ Wire to FIMEvent |
| ~~P2-09~~ | ~~xattr capture~~ | ✅ Done | ❌ Wire to FIMEvent |
| ~~P2-10~~ | ~~SELinux context (getfattr)~~ | ✅ Done | ❌ Wire to Dispatcher |

**Files:**
- `internal/infrastructure/acl/acl.go`
- `internal/infrastructure/acl/xattr.go`
- `internal/infrastructure/acl/selinux.go`

**Integration Required:**
```
acl/acl.go → FIMEvent → Dispatcher → Alert
xattr.go ──→ FIMEvent ──→ Dispatcher ──→ Alert
selinux.go ────────────────────────────→ Alert
```

---

## Phase 2E: SIEM Export ⚠️ NEEDS INTEGRATION

**Status:** ⚠️ CODE ADA, BELUM TERINTEGRASI
**Depends on:** Stable event schema (payload format frozen)

| Task | Description | Status | Integration Required |
|------|-------------|--------|---------------------|
| ~~P2-11~~ | ~~SIEM base client interface~~ | ✅ Done | ❌ Wire to FIMEvent |
| ~~P2-12~~ | ~~Syslog channel (RFC 5424)~~ | ✅ Done | ❌ Wire to Dispatcher |
| ~~P2-13~~ | ~~Splunk HEC channel~~ | ✅ Done | ❌ Wire to Dispatcher |
| ~~P2-14~~ | ~~Elasticsearch bulk API~~ | ✅ Done | ❌ Wire to Dispatcher |
| ~~P2-15~~ | ~~SIEM buffer/queue~~ | ✅ Done | ❌ Wire to FIMEvent |
| ~~P2-16~~ | ~~SIEM config API~~ | ✅ Done | ❌ Add `/api/v1/siem/*` routes |

**Files:**
- `internal/infrastructure/siem/client.go`
- `internal/infrastructure/siem/syslog.go`
- `internal/infrastructure/siem/elasticsearch.go`
- `internal/infrastructure/siem/buffer.go`
- `internal/infrastructure/siem/registry.go`

**Integration Required:**
```
FIMEvent ──▶ siem/buffer.go ──▶ SIEM Client ──▶ Elasticsearch/Syslog/Splunk
                                            ↑
                                      Dispatcher ◀── Alert Events
```

**Routes Required:**
```
/api/v1/siem/config        - SIEM configuration
/api/v1/siem/test          - Test connection
/api/v1/siem/status        - SIEM status
```

---

## Phase 3: Compliance + Hash Chain ⚠️ NEEDS INTEGRATION

**Status:** ⚠️ CODE ADA (frontend), BELUM TERINTEGRASI (backend)
**Depends on:** Phase 2A-2E event data accumulating in storage

| Task | Description | Status | Integration Required |
|------|-------------|--------|---------------------|
| ~~P3-01~~ | ~~Report package structure~~ | ✅ Done | ❌ Wire to FIMEvent table |
| ~~P3-02~~ | ~~SOC2 + NIST generator~~ | ✅ Done | ❌ Use FIMEvent data |
| ~~P3-03~~ | ~~CSV export~~ | ✅ Done | ❌ Wire to API |
| ~~P3-04~~ | ~~JSON export~~ | ✅ Done | ❌ Wire to API |
| P3-05 | Scheduled reports table | | ❌ Add `scheduled_reports` table |
| ~~P3-06~~ | ~~SHA-256 hash chain~~ | ✅ Done | ❌ Wire to scan results |
| P3-07 | Compliance UI | | ❌ Connect frontend to backend |

**Files:**
- `internal/infrastructure/compliance/report.go`

**Integration Required:**
```
FIMEvent table ──▶ compliance/report.go ──▶ Report Data
                                            │
                                            ▼
                                     /api/v1/compliance/*
```

**Routes Required:**
```
/api/v1/projects/{id}/compliance/report  - Generate report
/api/v1/projects/{id}/compliance/export   - Export (CSV/JSON)
/api/v1/projects/{id}/compliance/verify  - Verify hash chain
```

**Frontend:** `frontend/src/app/projects/[id]/compliance/page.tsx` exists but NOT connected to backend.

---

## Phase 4: User Experience

**Depends on:** Phase 2A-3 API surface validated

| Task | Description | Status | Integration |
|------|-------------|--------|-------------|
| P4-01 | Alerts config UI (Email/Slack/Webhook) | | Needs backend routes ✅ |
| P4-02 | Real-time alert stream | | WebSocket or SSE |
| P4-03 | Alerts tab in project page | | ✅ Partially done |

**Output:** Config UI, live stream, compliance reports accessible from UI.

---

## ⚠️ Integration Tasks (HARUS DIBUAT)

### INT-01: Wire auditd to FIM Pipeline ✅ PLANNED
```
File: internal/infrastructure/audit/handler.go (NEW)
├── AuditHandler struct (eventRepo, projectRepo)
├── IngestEvents() POST handler
└── ConvertAuditToFIMEvent()

Integration Flow:
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   auditd    │────▶│  parser.go   │────▶│  Converter   │────▶│  eventRepo  │
│  (syscall/  │     │ ParseEvent() │     │ audit→FIM    │     │  .Save()     │
│   execve)    │     │              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘

Files to create/modify:
1. NEW: internal/infrastructure/audit/handler.go
   - AuditHandler struct
   - IngestEvents(w, r) POST /fim/audit/ingest
   - ConvertAuditToFIMEvent(ae *audit.Event) *models.FIMEvent

2. UPDATE: internal/infrastructure/http/router.go
   - Add: r.Post("/fim/audit/ingest", cfg.AuditHandler.IngestEvents)

3. UPDATE: internal/wire/wire.go
   - Wire: NewAuditHandler(eventRepo, projectRepo)

Conversion Logic:
- audit.Type → FIMEvent.EventType (SYSCALL → MODIFIED/CREATED/DELETED)
- audit.Path → FIMEvent.FilePath
- audit.ProcessID/ProcessName → FIMEvent.ActorID/ActorName
- audit.UserID/LoginUID → FIMEvent.ActorDetails
- audit.Syscall → risk classification (execve=HIGH, open=LOW, etc)
```

### INT-02: Wire ACL to FIM Pipeline
```
File: internal/infrastructure/acl/acl.go → FIMEvent
- GetACL/CompareACL → FIMEvent.ActorDetails
- Detect permission changes → trigger alert
- Send to eventRepo on changes
```

### INT-03: Wire xattr to FIM Pipeline
```
File: internal/infrastructure/acl/xattr.go → FIMEvent
- GetXAttr → FIMEvent.ActorDetails (SELinux labels)
- xattr changes → HIGH risk events
- Send to eventRepo on changes
```

### INT-04 Status: ✅ COMPLETED - NO ROUTES NEEDED
```
SIEM is an EVENT SINK, not source.
Events flow: audit/ACL/FIM → eventRepo → SIEM buffer → Elasticsearch/Syslog/Splunk

Files created/updated:
1. ✅ internal/infrastructure/siem/dispatcher.go (NEW)
   - FIMDispatcher struct
   - NewFIMDispatcher(client, workers) factory
   - DispatchFIMEvent() - sends FIMEvent to SIEM
   - ConvertFIMEventToSIEM() - conversion logic
   - Global dispatcher pattern (optional)

2. ✅ internal/infrastructure/audit/handler.go (UPDATE)
   - Added: dispatchSIEM(event) calls

3. ✅ internal/infrastructure/acl/handler.go (UPDATE)
   - Added: dispatchSIEM(event) calls

Architecture:
┌─────────────┐
│ FIMEvent   │ ← Events from audit/ACL handlers
└──────┬──────┘
       │ eventRepo.Create()
       ▼
┌─────────────┐
│ FIMEvent   │ ← SQLite table
└──────┬──────┘
       │ FIMDispatcher.Dispatch()
       ▼
┌─────────────┐     ┌─────────────┐
│ Buffer     │────▶│ Worker Pool │
│ (30s/batch)│     │ (retry queue)│
└──────┬──────┘     └──────┬──────┘
       │                    │
       ▼                    ▼
┌─────────────────────────────────────────┐
│ SIEM Clients (Elasticsearch/Syslog/Splunk) │
└─────────────────────────────────────────┘
```

### INT-05: Wire Compliance to FIM Data ⚠️ NEEDS ROUTES
```
Compliance reads from FIMEvent table and generates reports.

Integration Flow:
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  FIMEvent   │────▶│ compliance/  │────▶│  Report      │
│  (SQLite)   │     │  report.go   │     │  Data        │
└──────────────┘     └──────────────┘     └──────────────┘
                                              │
                            ┌─────────────────┼─────────────────┐
                            ▼                 ▼                 ▼
                     ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
                     │   CSV       │    │   JSON      │    │ Hash Chain  │
                     │   Export    │    │   Export    │    │ Verification│
                     └─────────────┘    └─────────────┘    └─────────────┘

Files to create:
1. internal/infrastructure/compliance/handler.go (NEW)
   - GET /projects/{id}/compliance/report
   - GET /projects/{id}/compliance/export
   - GET /projects/{id}/compliance/verify
   - POST /projects/{id}/compliance/scheduled

2. UPDATE: internal/infrastructure/http/router.go
   - Add: r.Route("/projects/{id}/compliance", ...)
```

### INT-06: Add Compliance Routes ⚠️ PART OF INT-05
```
This task is now PART of INT-05.
Compliance routes to add:
- GET  /projects/{id}/compliance/report
- GET  /projects/{id}/compliance/export
- GET  /projects/{id}/compliance/verify
- POST /projects/{id}/compliance/scheduled
```

---

## Progress Tracker

| Phase | Total | Done | Pending | Integrated |
|-------|-------|------|---------|------------|
| Pre-flight | 7 | 7 | 0 | 7 ✅ |
| Phase 2A (Alert Core) | 5 | 5 | 0 | 5 ✅ |
| Phase 2B (Config+Watcher) | 2 | 2 | 0 | 2 ✅ |
| Phase 2C (auditd) | 4 | 4 | 0 | 1 ✅ |
| Phase 2D (ACL/xattr/SELinux) | 3 | 3 | 0 | 1 ✅ |
| Phase 2E (SIEM) | 6 | 6 | 0 | 1 ✅ |
| Phase 3 (Compliance) | 7 | 5 | 2 | 0 ❌ |
| Phase 4 (UX) | 3 | 0 | 3 | 0 ❌ |
| **Integration Tasks** | **3** | **3** | **0** | **3 ✅** |
| **Total** | **40** | **34** | **6** | **17** |

## Integration Architecture (Updated)

```
FIM CORE:
  Scanner ──▶ FIMEvent ◀── Watcher (fsnotify)
  Alert (2A/2B) | auditd (2C) ✅ | ACL (2D) ✅
              │                    │                 │
              └────────────────────┴─────────────────┘
                               ▼
                          eventRepo
                               │
    ┌─────────────────────────┼─────────────────────────┐
    │                         │                         │
    ▼                         ▼                         ▼
Alert Dispatcher         SIEM (INT-04)          Compliance (INT-05)
Email/Slack             NO ROUTES               NEEDS ROUTES
```

**Route Analysis:**
- EVENT SOURCE (external daemon) → YES routes: POST /fim/audit/ingest, /fim/acl/ingest
- EVENT SINK (background processing) → NO routes: SIEM background worker reads eventRepo
- USER-FACING (reports) → YES routes: GET /projects/{id}/compliance/*

**Decision Framework:**
```
APAKAH ROUTE DIBUTUHKAN?
├── EVENT SOURCE (external daemon feeding data)?
│   └── YA → Route untuk receive events (auditd, ACL monitor)
├── EVENT SINK (reading dari DB, processing)?
│   └── TIDAK → Background worker, bukan HTTP (SIEM export)
└── USER-FACING (report, config, query)?
    └── YA → Routes untuk request/response (Compliance reports)
```

---

## Completed Tasks

- ✅ P1-05/P1-06: Permission tracking with stat() diff
- ✅ fsnotify migration
- ✅ P2-01: Alert database schema (alert_configs, alert_history)
- ✅ P2-02: Alert dispatcher wired to watcher
- ✅ P2-02a: Rate limiting (60s dedup window)
- ✅ P2-03: Email SMTP channel (TLS support)
- ✅ P2-04: Slack webhook channel
- ✅ P2-05: Webhook custom channel
- ✅ P2-06: Alert config API (with tests)
- ✅ P2-07: Watcher integration (fsnotify → dispatcher)
- ✅ P2-A1: audit.log parser (streaming + JSON format support)
- ✅ P2-A2: audit rules generator (FIM + compliance + OJS rulesets)
- ✅ P2-A3: FIM + audit correlation (PID/path/time matching, actor extraction)
- ✅ P2-A4: Actor attribution (user/pid, session tracking, process chain)
- ✅ P2-08: ACL monitoring (getfacl, extended ACL detection)
- ✅ P2-09: xattr capture (security.xattr, SELinux labels)
- ✅ P2-10: SELinux context (getfattr, file capabilities)
- ✅ P3-01: Report package structure (compliance/report.go)
- ✅ P3-02: SOC2 + NIST generator (CC6.1, CC7.2, DE.CM-1, PR.AC)
- ✅ P3-03: CSV export (ToCSV method)
- ✅ P3-04: JSON export (ToJSON method)
- ✅ P3-06: SHA-256 hash chain (tamper-evident)

---

## Quick Commands

```bash
# Build all binaries
cd backend && make build

# Check migration status
./bin/manage db:status

# Run migrations
./bin/manage migrate

# Rollback last migration
./bin/manage db:down

# Reset database
./bin/manage db:reset

# Next task: INT-01 - Wire auditd to FIM Pipeline
```

---

## Integration Checklist

Setiap kali membuat fase baru:

```
□ Implementasi fase
□ Tambah routes di router.go
□ Wire ke Dispatcher (jika alert)
□ Wire ke SIEM (jika event)
□ Wire ke Compliance (jika report)
□ Tambah integration test
□ Update graph diagram di atas
```

### INT-01 Status: ✅ COMPLETED
- [x] Analisa gap (parser.go → eventRepo)
- [x] Buat: audit/handler.go
- [x] Tambah route: POST /fim/audit/ingest
- [x] Wire: NewAuditHandler to wire.go
- [ ] Test: convert audit.Event → FIMEvent

Files created/modified:
1. ✅ internal/infrastructure/audit/handler.go (NEW)
   - AuditHandler struct with eventRepo + projectRepo
   - IngestEvents() - POST handler for raw audit lines
   - ConvertAuditToFIMEvent() - conversion logic
   - Risk classification (EXECVE=HIGH, LOGIN=CRITICAL, etc)
   - Actor detection (OJS_USER, PROCESS, SYSTEM_USER)
   - Source classification (TRUSTED, MODIFIED, UNKNOWN_SOURCE)

2. ✅ internal/infrastructure/http/router.go (UPDATE)
   - Added: AuditHandler field in RouterConfig
   - Added: r.Post("/fim/audit/ingest", cfg.AuditHandler.IngestEvents)
   - Added: r.Get("/fim/audit/status", cfg.AuditHandler.GetStatus)

3. ✅ cmd/server/main.go (UPDATE)
   - Added: audit import
   - Added: auditHandler := audit.NewAuditHandler(fimEventRepo, projectRepo)
   - Added: AuditHandler: auditHandler in RouterConfig

### INT-02 Status: ✅ COMPLETED
- [x] ACL handler for FIM integration
- [x] Wire ACL handler to router
- [x] Test build

Files created/modified:
1. ✅ internal/infrastructure/acl/handler.go (NEW)
   - ACLHandler struct with eventRepo + projectRepo
   - IngestChanges() - POST /fim/acl/ingest
   - ScanACLs() - POST /fim/acl/scan
   - ConvertACLChangeToFIMEvent()
   - Risk classification (execute=HIGH, write=MEDIUM)

2. ✅ internal/infrastructure/http/router.go (UPDATE)
   - Added: ACLHandler field in RouterConfig
   - Added: r.Post("/fim/acl/ingest", cfg.ACLHandler.IngestChanges)
   - Added: r.Post("/fim/acl/scan", cfg.ACLHandler.ScanACLs)
   - Added: r.Get("/fim/acl/status", cfg.ACLHandler.GetStatus)

3. ✅ cmd/server/main.go (UPDATE)
   - Added: acl import
   - Added: aclHandler := infraacl.NewACLHandler(fimEventRepo, projectRepo)
   - Added: ACLHandler: aclHandler in RouterConfig

### INT-03 Status: ⏳ Pending
- [ ] xattr monitoring → FIMEvent
- [ ] SELinux labels → HIGH risk

---

## Wire.go Integration Map

File: `internal/wire/wire.go` - WAJIB diupdate setiap ada fase baru:

```go
type Repositories struct {
    Project      repository.ProjectRepository
    Job          repository.JobRepository
    File         repository.FileRepository
    FIMEvent     repository.FIMEventRepository
    Auth         repository.AuthRepository
    AlertConfig  repository.AlertConfigRepository
    AlertHistory repository.AlertHistoryRepository
    // TODO: Add audit repository (INT-01)
    // TODO: Add SIEM config repository (INT-04)
    // TODO: Add compliance repository (INT-05)
}
```

---

*Updated: 2026-08-06*
*Status: Phase 2A/2B/2C/2D connected, Phase 2E/3 code done but NOT integrated*
*Next: INT-05 (Compliance routes) - GET /projects/{id}/compliance/*
