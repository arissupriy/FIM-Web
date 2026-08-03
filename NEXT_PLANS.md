# NEXT_PLANS.md
**Project:** OJS Monitor - Enterprise FIM Upgrade
**Created:** 2026-08-02
**Status:** Ready for Implementation

## Decisions Made

| Decision | Choice |
|----------|--------|
| Alert Channels | ALL (Email + Slack + Webhook) |
| SIEM Platform | Elasticsearch (ELK) - primary, Syslog - fallback |
| Compliance Framework | SOC2 (primary) + NIST (mapping) + Executive summary |
| auditd Access | Full integration available |
| Report Recipients | Configurable per schedule |

---

## 📋 Task List (Execute One by One)

### Phase 1: Foundation

- [x] **[P1-01] Tambah kolom permission ke database** - Add file_mode, file_uid, file_gid columns to project_files table
- [x] **[P1-02] Update models.go dengan permission fields** - Add FileMode, FileUID, FileGID to ProjectFile struct
- [x] **[P1-03] Update getFileHash → getFileMetadata** - Capture stat() data including permissions
- [x] **[P1-04] Update worker.go baseline scan** - Include permissions in baseline hash calculation
- [x] **[P1-05] Update watcher.go untuk permission tracking** - Detect chmod/chown changes
- [x] **[P1-06] Add permission change detection** - Flag permission changes as HIGH risk

### Phase 2: Alerting System

- [ ] **[P2-01] Alert system database schema** - alert_configs and alert_history tables
- [ ] **[P2-02] Alert dispatcher core** - In-memory queue and routing
- [ ] **[P2-03] Implement email alert channel** - SMTP integration
- [ ] **[P2-04] Implement Slack alert channel** - Slack webhook API
- [ ] **[P2-05] Implement webhook alert channel** - HTTP POST to configurable URL
- [ ] **[P2-06] Alert configuration API** - CRUD endpoints for alert channels
- [ ] **[P2-07] Integrate alerts in watcher** - Send alerts on HIGH/CRITICAL events

### Phase 2: Advanced Monitoring (auditd)

- [ ] **[P2-A1] Create auditd parser module** - Parse /var/log/audit/audit.log
- [ ] **[P2-A2] Create auditd rules generator** - Generate /etc/audit/rules.d/ojs-monitor.rules
- [ ] **[P2-A3] Implement audit correlation** - Link audit events with FIM events
- [ ] **[P2-A4] Update actor attribution** - Populate user/pid from audit logs

- [ ] **[P2-08] Create ACL monitoring module** - Linux getfacl integration
- [ ] **[P2-09] Create xattr monitoring module** - Extended attributes capture
- [ ] **[P2-10] Create SELinux context module** - getfattr for SELinux labels
- [ ] **[P2-11] Add ACL event table** - Track ACL changes
- [ ] **[P2-12] Detect ACL permission changes** - Alert on ACL modifications

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

> **Phase 1: Foundation - COMPLETED**
> **Next: Phase 2: Alerting System [P2-01]**

---

## Progress Tracker

| Phase | Total | Completed | Pending |
|-------|-------|-----------|---------|
| Phase 1 | 6 | 6 | 0 |
| Phase 2 | 16 | 0 | 16 |
| Phase 3 | 7 | 0 | 7 |
| Phase 4 | 3 | 0 | 3 |
| **Total** | **32** | **6** | **26** |

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
