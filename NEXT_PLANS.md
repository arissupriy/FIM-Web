# NEXT_PLANS.md
**Project:** OJS Monitor - Enterprise FIM Upgrade
**Created:** 2026-08-02
**Last Revised:** 2026-08-05 (reordered Phase 2 by dependency, merged duplicate section headers, added pre-flight decisions)
**Status:** Ready for Implementation

## Decisions Made

| Decision | Choice |
|----------|--------|
| Alert Channels | ALL (Email + Slack + Webhook) |
| SIEM Platform | Elasticsearch (ELK) - primary, Syslog - fallback |
| Compliance Framework | SOC2 (primary) + NIST (mapping) + Executive summary |
| auditd Access | Full integration available |
| Report Recipients | Configurable per schedule |

## Decisions Needed Before Phase 2 Starts

These weren't resolved in the original plan and will force rework later if skipped:

| Question | Why it matters | Suggested default |
|----------|-----------------|--------------------|
| `alert_configs` schema: flat (channel+threshold per row) or rule-based (condition tree: risk_level AND path pattern AND time_window)? | Changing this after P2-01/P2-06 are built means migrating existing config data and rewriting the CRUD API. | Rule-based, but start with a single-condition subset — leaves room to grow without a rewrite. |
| Alert dispatcher queue: new in-memory queue (P2-02 as written) or reuse the existing SQLite-backed background worker job queue from Phase 1? | Two separate queueing/retry mechanisms in the same codebase means two failure-handling paths to maintain and debug. | Reuse the existing job queue; add an `alert` job type. |
| Alert dedup / rate-limiting | A legit deploy touching many files (or a false-positive burst) will otherwise flood Slack/email with one message per file event. | Add a dedup window (e.g. same file+risk_level within N seconds → collapse into one alert) as an explicit sub-task under P2-07, not an afterthought. |

## Verify Before Starting Phase 2

- [ ] **Confirm P1-05 chmod/chown detection uses before/after `stat()` diff**, not just the raw fsnotify `Chmod` event. fsnotify's `Chmod` event only signals that *some* metadata changed — it doesn't distinguish mode vs uid vs gid vs timestamp. P1-06 (flagging permission changes as HIGH risk) and later P2-A3 (audit correlation) both depend on knowing exactly what changed, so this needs to be correct before building on top of it.

---

## 📋 Task List (Execute One by One)

### Phase 1: Foundation

- [ ] **[P1-01] Tambah kolom permission ke database** - Add file_mode, file_uid, file_gid columns to project_files table
- [ ] **[P1-02] Update models.go dengan permission fields** - Add FileMode, FileUID, FileGID to ProjectFile struct
- [ ] **[P1-03] Update getFileHash → getFileMetadata** - Capture stat() data including permissions
- [ ] **[P1-04] Update worker.go baseline scan** - Include permissions in baseline hash calculation
- [ ] **[P1-05] Update watcher.go untuk permission tracking** - Detect chmod/chown changes (verify before/after stat diff — see checklist above)
- [ ] **[P1-06] Add permission change detection** - Flag permission changes as HIGH risk

### Phase 2A: Alerting Core (build first — everything else in Phase 2 sends through this)

- [ ] **[P2-01] Alert system database schema** - alert_configs and alert_history tables (resolve schema-shape decision above first)
- [ ] **[P2-02] Alert dispatcher core** - Queue and routing (reuse existing job queue — see decision above)
- [ ] **[P2-02a] Alert dedup / rate-limiting** - Collapse repeated alerts within a time window per file+risk_level
- [ ] **[P2-03] Implement email alert channel** - SMTP integration
- [ ] **[P2-04] Implement Slack alert channel** - Slack webhook API
- [ ] **[P2-05] Implement webhook alert channel** - HTTP POST to configurable URL
- [ ] **[P2-06] Alert configuration API** - CRUD endpoints for alert channels
- [ ] **[P2-07] Integrate alerts in watcher** - Send alerts on HIGH/CRITICAL events

### Phase 2B: auditd Integration (depends on 2A dispatcher + stable Phase 1 event format)

- [ ] **[P2-A1] Create auditd parser module** - Parse /var/log/audit/audit.log
- [ ] **[P2-A2] Create auditd rules generator** - Generate /etc/audit/rules.d/ojs-monitor.rules
- [ ] **[P2-A3] Implement audit correlation** - Link audit events with FIM events
- [ ] **[P2-A4] Update actor attribution** - Populate user/pid from audit logs

### Phase 2C: ACL / xattr / SELinux (reuses the P1-05/P1-06 permission-tracking pattern)

- [ ] **[P2-08] Create ACL monitoring module** - Linux getfacl integration
- [ ] **[P2-09] Create xattr monitoring module** - Extended attributes capture
- [ ] **[P2-10] Create SELinux context module** - getfattr for SELinux labels
- [ ] **[P2-11] Add ACL event table** - Track ACL changes
- [ ] **[P2-12] Detect ACL permission changes** - Alert on ACL modifications

### Phase 2D: SIEM Export (last — needs a stable event schema from 2A/2B/2C)

- [ ] **[P2-13] Create SIEM base client interface** - Common client interface
- [ ] **[P2-14] Implement syslog channel** - RFC 5424 over UDP/TCP
- [ ] **[P2-15] Implement Splunk HEC channel** - Splunk HTTP Event Collector
- [ ] **[P2-16] Implement Elasticsearch channel** - Bulk API ingestion
- [ ] **[P2-17] Add SIEM buffer/queue** - Batch events for performance
- [ ] **[P2-18] Add SIEM configuration API** - Configure destinations

### Phase 3: Compliance & Reporting

- [ ] **[P3-01] Report directory structure** - reports/ package with generator
- [ ] **[P3-02] Report generator** - SOC2, NIST, Executive reports
- [ ] **[P3-03] CSV export** - Export report data to CSV
- [ ] **[P3-04] JSON export** - Export report data to JSON
- [ ] **[P3-05] Scheduled reports table** - Database schema and CRUD
- [ ] **[P3-06] Hash chain** - SHA-256 hash chain for audit integrity
- [ ] **[P3-07] Compliance UI** - Compliance page with report preview

### Phase 4: User Experience

- [ ] **[P4-01] Create alerts configuration UI** - Alert channel setup page with Email/Slack/Webhook forms
- [ ] **[P4-02] Create real-time alert stream** - Alert history table with Sonner toasts
- [ ] **[P4-03] Add Alerts tab to navigation** - Tab integration in project page

---

## 🎯 Current Task

> **Phase 1: Foundation - IN PROGRESS**
> **Next: [P1-01]**

---

## Progress Tracker

| Phase | Total | Completed | Pending |
|-------|-------|-----------|---------|
| Phase 1 | 6 | 0 | 6 |
| Phase 2A (Alerting Core) | 8 | 0 | 8 |
| Phase 2B (auditd) | 4 | 0 | 4 |
| Phase 2C (ACL/xattr/SELinux) | 5 | 0 | 5 |
| Phase 2D (SIEM) | 6 | 0 | 6 |
| Phase 3 | 7 | 0 | 7 |
| Phase 4 | 3 | 0 | 3 |
| **Total** | **39** | **0** | **39** |

*(Total task count rose from 32 to 39 because P2-02a was split out explicitly — it was implicit inside P2-02 before.)*

---

## Quick Commands

```bash
# Check current phase
cat NEXT_PLANS.md | grep "Current Task"

# Mark task as done
# Replace "- [ ]" with "- [x]" for completed tasks

# Start specific task
# Read the relevant files and implement
```

---

*Update this file as tasks are completed*