# NEXT_PLANS.md

**Project:** OJS Monitor - Enterprise FIM Upgrade
**Purpose:** Generic FIM Platform dengan CMS-specific templates
**Status:** Pre-flight decisions resolved

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
        ↓
Phase 2B: Config + Watcher Integration (2 tasks)
        ↓
Phase 2C: auditd Integration (4 tasks)
Phase 2D: ACL/xattr/SELinux (3 tasks)
        ↓
Phase 2E: SIEM Export (6 tasks)
        ↓
Phase 3: Compliance + Hash Chain (7 tasks)
        ↓
Phase 4: User Experience (3 tasks)
```

---

## Phase 2A: Alerting Core

**Depends on:** Pre-flight decisions resolved ✓

| Task | Description |
|------|-------------|
| P2-01 | Alert database schema (rule-based single-condition) |
| P2-02 | Alert dispatcher (reuse job queue) |
| P2-02a | Rate limiting (60s dedup window) |
| P2-03 | Email channel (SMTP) |
| P2-04 | Slack channel (webhook) |
| P2-05 | Webhook channel (custom URL) |

**Output:** Alert infrastructure with reusable channel pattern.

---

## Phase 2B: Configuration + Watcher Integration

**Depends on:** Phase 2A dispatcher + channels working end-to-end

| Task | Description |
|------|-------------|
| P2-06 | Alert config API |
| P2-07 | Watcher integration (fsnotify → dispatcher) |

**Output:** Alerts fire on HIGH/CRITICAL events.

---

## Phase 2C: auditd Integration

**Depends on:** Phase 2A dispatcher + stat() diff verified

| Task | Description |
|------|-------------|
| P2-A1 | audit.log parser |
| P2-A2 | audit rules generator |
| P2-A3 | FIM + audit correlation |
| P2-A4 | Actor attribution (user/pid) |

**Output:** FIM events enriched with audit attribution.

---

## Phase 2D: ACL / xattr / SELinux

**Depends on:** Phase 2A dispatcher (reuses same alert pattern as P1-05/P1-06)

| Task | Description |
|------|-------------|
| P2-08 | ACL monitoring (getfacl) |
| P2-09 | xattr capture |
| P2-10 | SELinux context (getfattr) |

**Output:** ACL, xattr, SELinux context changes tracked as integrity signals.

---

## Phase 2E: SIEM Export

**Depends on:** Stable event schema (payload format frozen)

| Task | Description |
|------|-------------|
| P2-11 | SIEM base client interface |
| P2-12 | Syslog channel (RFC 5424) |
| P2-13 | Splunk HEC channel |
| P2-14 | Elasticsearch bulk API |
| P2-15 | SIEM buffer/queue |
| P2-16 | SIEM config API |

**Output:** All alert/audit/ACL events forwarded to Elasticsearch (primary) with Syslog fallback.

---

## Phase 3: Compliance + Hash Chain

**Depends on:** Phase 2A-2E event data accumulating in storage

| Task | Description |
|------|-------------|
| P3-01 | Report package structure |
| P3-02 | SOC2 + NIST generator |
| P3-03 | CSV export |
| P3-04 | JSON export |
| P3-05 | Scheduled reports table |
| P3-06 | SHA-256 hash chain |
| P3-07 | Compliance UI |

**Output:** Compliance reports with tamper-evident hash chain.

---

## Phase 4: User Experience

**Depends on:** Phase 2A-3 API surface validated

| Task | Description |
|------|-------------|
| P4-01 | Alerts config UI (Email/Slack/Webhook) |
| P4-02 | Real-time alert stream |
| P4-03 | Alerts tab in project page |

**Output:** Config UI, live stream, compliance reports accessible from UI.

---

## Progress Tracker

| Phase | Total | Done | Pending |
|-------|--------|------|--------|
| Pre-flight | 7 | 7 | 0 |
| Phase 2A (Alert Core) | 5 | 0 | 5 |
| Phase 2B (Config+Watcher) | 2 | 0 | 2 |
| Phase 2C (auditd) | 4 | 0 | 4 |
| Phase 2D (ACL/xattr/SELinux) | 3 | 0 | 3 |
| Phase 2E (SIEM) | 6 | 0 | 6 |
| Phase 3 (Compliance) | 7 | 0 | 7 |
| Phase 4 (UX) | 3 | 0 | 3 |
| **Total** | **37** | **7** | **30** |

---

## Quick Commands

```bash
# Build all binaries
make build

# Check progress
grep "Progress Tracker" NEXT_PLANS.md

# Mark task done
# Replace "- [ ]" with "- [x]"

# Next task
# P2-01: Alert schema
```

---

*Update this file as tasks complete.*
