# NEXT_PLANS.md

**Project:** OJS Monitor - Enterprise FIM Upgrade
**Purpose:** Generic FIM Platform dengan CMS-specific templates
**Status:** Ready for Phase 2A

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|------------|
| Alert Channels | Email + Slack + Webhook | ALL |
| SIEM Platform | Elasticsearch (ELK) | Syslog fallback |
| Compliance | SOC2 primary + NIST mapping | Executive summary |
| FIM Library | **fsnotify** (native Go) | Cleaner than inotifywait |
| Queue | Reuse existing job queue | Single failure path |
| Dedup | Explicit P2-02a | file+risk_level window |
| Schema | Rule-based (single-condition start) | Grow without rewrite |

---

## Pre-flight Checklist

- [x] **fsnotify migration** - Native Go watcher (not inotifywait)
- [ ] **Decision: Alert schema** - Rule-based (single-condition)
- [ ] **Decision: Queue reuse** - Reuse job queue
- [ ] **Decision: Dedup window** - N seconds per file+risk_level
- [ ] **P1-05 fsnotify stat() diff** - Permission changes need before/after stat comparison
- [ ] **P1-06 flag permission changes** - HIGH risk detection

---

## Dependency Graph

```
┌─────────────────────────────────────────────────────────────┐
│                    Phase 2A: Alert Core                      │
│  P2-01 (schema) → P2-02 (dispatcher) → channels (03-05)      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                Phase 2B: Config + Integration                │
│  P2-06 (config API) → P2-07 (watcher integration)             │
└─────────────────────────────────────────────────────────────┘
                            │
          ┌───────────────┬───────────┬───────────┐
          ▼               ▼           ▼           ▼
       Phase 2C       Phase 2D    Phase 2E    Phase 2F
      auditd         ACL/xattr   SIEM       Compliance
      (4 tasks)     SELinux    (6 tasks)    (7 tasks)
```

---

## Phase 2A: Alerting Core (Build First - semua phase lain kirim event ke sini)

**Depends on:** Pre-flight decisions (alert schema shape, queue reuse, dedup window) resolved.

| Task | Description |
|------|-------------|
| P2-01 | Alert database schema (alert_configs + alert_history) |
| P2-02 | Alert dispatcher + queue |
| P2-02a | Dedup/rate-limiting per file+risk_level |
| P2-03 | Email channel (SMTP) |
| P2-04 | Slack channel (webhook) |
| P2-05 | Webhook channel (custom URL) |

**Output:** Alert infrastructure lengkap, reusable channel pattern.

---

## Phase 2B: Configuration + Watcher Integration

**Depends on:** Phase 2A dispatcher + channels working end-to-end.

| Task | Description |
|------|-------------|
| P2-06 | Alert config API |
| P2-07 | Watcher integration (fsnotify → dispatcher) |

**Output:** Alerts fire on HIGH/CRITICAL events.

---

## Phase 2C: auditd Integration

**Depends on:** Phase 2A/2B dispatcher live + P1-05/P1-06 stat()-diff verified (pre-flight checklist).

| Task | Description |
|------|-------------|
| P2-A1 | audit.log parser |
| P2-A2 | audit rules generator |
| P2-A3 | FIM + audit correlation |
| P2-A4 | Actor attribution (user/pid) |

**Output:** FIM events enriched with actor/process attribution from audit.log correlation.

---

## Phase 2D: ACL / xattr / SELinux

**Depends on:** Phase 2A dispatcher (reuses same alert pattern as permission tracking in P1-05/P1-06).

| Task | Description |
|------|-------------|
| P2-08 | ACL monitoring (getfacl) |
| P2-09 | xattr capture |
| P2-10 | SELinux context (getfattr) |

**Output:** ACL, xattr, and SELinux context changes tracked as additional integrity signals.

---

## Phase 2E: SIEM Export

**Depends on:** Stable event schema from Phase 2A/2C/2D (payload format shouldn't still be changing).

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

## Phase 3: Compliance & Reporting

**Depends on:** Phase 2 event data (alerts, audit correlation, ACL/SIEM logs) accumulating in storage.

| Task | Description |
|------|-------------|
| P3-01 | Report package structure |
| P3-02 | SOC2 + NIST + Executive generator |
| P3-03 | CSV export |
| P3-04 | JSON export |
| P3-05 | Scheduled reports table |
| P3-06 | SHA-256 hash chain |
| P3-07 | Compliance UI |

**Output:** SOC2/NIST-mapped compliance reports with CSV/JSON export and tamper-evident hash chain.

---

## Phase 4: User Experience

**Depends on:** Phase 2/3 API surface validated (see debug-page checkpoint after P2-07).

| Task | Description |
|------|-------------|
| P4-01 | Alerts config UI (Email/Slack/Webhook) |
| P4-02 | Real-time alert stream + Sonner toasts |
| P4-03 | Alerts tab in project page |

**Output:** Alert config, live alert stream, and compliance reports accessible from project UI.

---

## Progress Tracker

| Phase | Total | Done | Pending |
|-------|--------|-------|---------|
| Phase 1 (Foundation) | 6 | 6 | 0 |
| Phase 2A (Alert Core) | 5 | 0 | 5 |
| Phase 2B (Config+Watcher) | 2 | 0 | 2 |
| Phase 2C (auditd) | 4 | 0 | 4 |
| Phase 2D (ACL/xattr/SELinux) | 3 | 0 | 3 |
| Phase 2E (SIEM) | 6 | 0 | 6 |
| Phase 3 (Compliance) | 7 | 0 | 7 |
| Phase 4 (UX) | 3 | 0 | 3 |
| **Total** | **36** | **6** | **30** |

---

## Quick Commands

```bash
# Check current phase
grep "Current Task" NEXT_PLANS.md

# Mark task done
# Replace "- [ ]" with "- [x]"

# Start specific task
# Read the relevant files and implement
```

---

*Update this file as tasks are completed*