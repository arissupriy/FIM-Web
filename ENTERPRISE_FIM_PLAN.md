# Enterprise FIM Implementation Plan
**Project:** OJS Monitor - File Integrity Monitoring
**Date:** 2026-08-02
**Current Maturity:** Basic FIM → Enterprise FIM

---

## Executive Summary

Based on gap analysis, here are the key findings:

| Feature | Current | Target | Effort |
|---------|---------|--------|--------|
| Kernel-level monitoring | ✅ fsnotify | ✅ fanotify/inotify | Done |
| System call tracing | ❌ None | ✅ auditd integration | Medium |
| User/process tracking | ⚠️ Basic | ✅ Full audit trail | High |
| Permission/ACL monitoring | ❌ None | ✅ Track mode, UID, GID | Medium |
| Baseline integrity | ⚠️ SHA-256 | ✅ Hash + permission baseline | Low |
| Compliance reports | ❌ None | ✅ PCI-DSS, HIPAA templates | High |
| SIEM integration | ❌ None | ✅ syslog, webhook, SNMP | Medium |
| Real-time alerting | ⚠️ Log only | ✅ Email, Slack, Webhook | Medium |

---

## Phase 1: Foundation (Quick Wins)

### 1.1 File Permission & Ownership Tracking

**Priority:** High | **Effort:** Medium

**Changes:**
```
backend/
├── db.go                    # Add permission columns
├── watcher.go               # Capture stat() data
├── worker.go                # Include in baseline
└── models.go                # Update struct
```

**Database Migration:**
```sql
ALTER TABLE project_files ADD COLUMN file_mode TEXT;
ALTER TABLE project_files ADD COLUMN file_uid INTEGER;
ALTER TABLE project_files ADD COLUMN file_gid INTEGER;
ALTER TABLE project_files ADD COLUMN file_perm_changes INTEGER DEFAULT 0;
```

**Implementation:**
1. Read `os.FileInfo.Sys()` to get `syscall.Stat_t` (mode, uid, gid)
2. Store permission snapshot in baseline
3. Detect permission changes (chmod, chown)
4. Classify permission changes as HIGH risk

**Files to modify:**
- `db.go`: Add migration for permission fields
- `watcher.go`: Capture stat in `getFileHash()` → rename to `getFileMetadata()`
- `models.go`: Add `FileMode`, `FileUID`, `FileGID` to `ProjectFile`
- `worker.go`: Include permissions in baseline scan
- `handlers.go`: Add permission change detection endpoint

---

### 1.2 Real-Time Alerting System

**Priority:** High | **Effort:** Medium

**Architecture:**
```
FIM Event → Alert Queue → Alert Processor → Channels (ALL enabled)
                                    ├── Email (SMTP/TLS)
                                    ├── Slack (Webhook)
                                    ├── Webhook (HTTP POST)
                                    └── Syslog (RFC 5424)
```

**Changes:**
```
backend/
├── alerts/
│   ├── dispatcher.go       # Alert routing
│   ├── channels/
│   │   ├── email.go        # SMTP integration
│   │   ├── webhook.go     # HTTP callbacks
│   │   ├── slack.go       # Slack webhooks
│   │   └── syslog.go      # RFC 5424
│   ├── queue.go            # In-memory queue
│   └── templates.go        # Alert templates
└── handlers.go             # Alert config API
```

**API Endpoints:**
```
POST   /api/projects/{id}/alerts/config     # Configure alert channels
GET    /api/projects/{id}/alerts/config     # Get alert config
POST   /api/projects/{id}/alerts/test       # Test alert delivery
GET    /api/alerts/history                  # Alert history
PUT    /api/alerts/{id}/acknowledge        # Ack alert
```

**Database:**
```sql
CREATE TABLE alert_configs (
    id, project_id, channel_type,
    config_json, enabled, created_at
);

CREATE TABLE alert_history (
    id, project_id, event_id, channel_type,
    status, error, sent_at
);
```

**Alert Levels:**
- CRITICAL: Security-relevant files modified (config, .php, .htaccess)
- HIGH: Unknown source changes, permission changes
- MEDIUM: Deleted files, orphan detection
- LOW: System-generated files (cache, logs)

---

### 1.3 Integrity Hash Improvements

**Priority:** Medium | **Effort:** Low

**Changes:**
1. **Large file support**: Chunked SHA-256 for files >10MB
2. **Hash algorithm options**: SHA-256 (default), SHA-512, BLAKE3
3. **Hash chaining**: Link events with previous hash for tamper detection

**Implementation:**
```go
// Chunked hashing for large files
func getFileHashChunked(filePath string, chunkSize int64) (string, error) {
    f, _ := os.Open(filePath)
    defer f.Close()

    hash := sha256.New()
    buf := make([]byte, chunkSize)

    for {
        n, err := f.Read(buf)
        if n > 0 {
            hash.Write(buf[:n])
        }
        if err == io.EOF {
            break
        }
    }
    return hex.EncodeToString(hash.Sum(nil)), nil
}
```

**Database:**
```sql
ALTER TABLE project_files ADD COLUMN hash_sha512 TEXT;
ALTER TABLE fim_events ADD COLUMN prev_event_hash TEXT;
ALTER TABLE fim_events ADD COLUMN event_hash TEXT;
```

---

## Phase 2: Advanced Monitoring

### 2.1 System Call Tracing (auditd Integration)

**Priority:** High | **Effort:** High

**Why:** inotify cannot track which user/process made changes. auditd provides:
- User ID (uid, euid, suid)
- Process ID and parent process
- Executable path
- Command arguments
- System call type (open, write, unlink, chmod, chown)

**Architecture:**
```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  auditd     │────▶│ auditd rules │────▶│ audit.log   │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                                │
                                                ▼
                                       ┌────────────────┐
                                       │ FIM Watcher    │
                                       │ (correlation)  │
                                       └────────────────┘
```

**Implementation:**
```bash
# /etc/audit/rules.d/ojs-monitor.rules
-w /var/www/ojs -p wa -k ojs_files
-w /var/www/ojs/plugins -p wa -k ojs_plugins
-w /var/www/ojs/config.php -p wa -k ojs_config
```

**Changes:**
```
backend/
├── audit/
│   ├── parser.go           # Parse audit.log
│   ├── correlator.go       # Match audit events with FIM events
│   └── rules.go            # Generate auditd rules
└── watcher.go              # Integrate audit correlation
```

**Correlation Logic:**
1. When FIM event fires (file modified), query audit.log for:
   - Same file path
   - Within ±5 second window
   - Extract user, pid, command
2. Update FIM event with actor info

---

### 2.2 ACL & Extended Attributes Monitoring

**Priority:** Medium | **Effort:** Medium

**Changes:**
```
backend/
├── acls/
│   ├── linux_acl.go        # getfacl/setfacl parsing
│   ├── xattr.go            # Extended attributes (attr, getfattr)
│   └── selinux.go          # SELinux context (getfattr)
└── models.go              # ACL data structures
```

**Database:**
```sql
CREATE TABLE file_acls (
    id, project_id, file_id, file_path,
    acl_type, acl_text, selinux_ctx,
    captured_at, UNIQUE(file_id)
);

CREATE TABLE acl_events (
    id, project_id, file_id, event_type,
    old_acl, new_acl, actor_info,
    detected_at
);
```

**Supported Platforms:**
- Linux: getfacl, xattr, SELinux (getfattr)
- BSD: ACLs via NFSv4
- Note: Windows NTFS ACLs requires Go-WinIO or separate implementation

---

### 2.3 SIEM Integration

**Priority:** Medium | **Effort:** Medium

**Supported Integrations:**

| Type | Priority | Protocol | Implementation |
|------|----------|----------|----------------|
| Elasticsearch | **Primary** | Bulk API (HTTPS) | Native Go client |
| Syslog | **Fallback** | RFC 5424/UDP | Direct socket |
| Webhook | **Optional** | HTTPS POST | HTTP client |
| Splunk HEC | **Optional** | HTTPS + token | HTTP client |

**Changes:**
```
backend/
├── siem/
│   ├── client.go           # Base client interface
│   ├── elastic.go          # Elasticsearch Bulk API (PRIMARY)
│   ├── syslog.go           # RFC 5424 sender (FALLBACK)
│   ├── webhook.go          # HTTP webhook (OPTIONAL)
│   ├── splunk.go           # Splunk HEC (OPTIONAL)
│   └── buffer.go          # Batch buffering
└── handlers.go             # SIEM config API
```

**API Endpoints:**
```
POST /api/siem/config              # Configure SIEM destination
GET  /api/siem/config             # Get SIEM config
POST /api/siem/test               # Test connection
GET  /api/siem/status             # Connection health
```

**Event Format (CEF-like):**
```
CEF:Version|Device Vendor|Device Product|Device Version|Signature ID|Name|Severity|Extension
```

---

## Phase 3: Compliance & Reporting

### 3.1 Compliance Report Templates

**Priority:** High | **Effort:** High

**Supported Frameworks:**
- **SOC 2 CC7.2** (Primary): Service Organization Control - Security incident identification
- **NIST SP 800-53** (Secondary): AU-14 (Audit Logging), SI-7 (Software Integrity)
- **Executive Summary**: High-level overview for management

**Report Types:**
```
backend/
├── reports/
│   ├── templates/           # Report templates
│   │   ├── soc2.html       # SOC 2 CC7.2 compliance
│   │   ├── nist.html       # NIST SP 800-53 mapping
│   │   └── executive.html  # Management summary
│   ├── generator.go         # Report generation
│   ├── exporter.go          # PDF/CSV export
│   └── signer.go            # Digital signing
└── handlers.go              # Report API
```

**Report Sections:**
1. Executive Summary
2. Scope & Methodology
3. Baseline Status
4. Change Summary (period)
5. High-Risk Events
6. Unknown Source Analysis
7. Compliance Controls Mapping
8. Recommendations

**API Endpoints:**
```
POST   /api/reports/generate          # Generate report
GET    /api/reports/{id}              # Get report
GET    /api/reports/{id}/download     # Download PDF/CSV
POST   /api/reports/schedule          # Schedule report
GET    /api/reports/scheduled         # List scheduled reports
```

**Export Formats:**
- PDF (using go-wkhtmltopdf or goldmark)
- CSV (for data analysis)
- JSON (for SIEM ingestion)

---

### 3.2 Scheduled Reports

**Priority:** Medium | **Effort:** Medium

**Database:**
```sql
CREATE TABLE scheduled_reports (
    id, project_id, name, framework,
    schedule_cron, last_run, next_run,
    recipients (JSON), enabled, format
);
```

**Schedule Options:**
- Daily (midnight)
- Weekly (Monday 6am)
- Monthly (1st 6am)
- Quarterly (for compliance)

---

### 3.3 Audit Trail Integrity

**Priority:** High | **Effort:** Medium

**Hash Chain Implementation:**
```go
type FIMEvent struct {
    // ... existing fields ...
    PrevEventHash string `json:"prev_event_hash"` // Hash of previous event
    EventHash     string `json:"event_hash"`       // SHA-256 of this event
    Signature     string `json:"signature"`        // Optional GPG signature
}

func (e *FIMEvent) ComputeHash() string {
    data := fmt.Sprintf("%d|%s|%s|%s|%s|%d",
        e.ID, e.EventType, e.FilePath, e.FileHash,
        e.ActorName, e.Timestamp)
    return sha256.Sum256([]byte(data))
}
```

**Verification Endpoint:**
```
POST /api/audit/verify               # Verify chain integrity
GET  /api/audit/integrity-report     # Integrity status
```

---

## Phase 4: User Experience

### 4.1 Enhanced Frontend Dashboard

**Priority:** Medium | **Effort:** Medium

**New Components:**
```
frontend/src/app/
├── projects/[id]/
│   ├── alerts/                   # Alert configuration UI
│   │   └── page.tsx
│   ├── compliance/               # Compliance reports
│   │   ├── page.tsx
│   │   └── [reportId]/page.tsx
│   └── audit/                    # Audit trail viewer
│       └── page.tsx
└── settings/
    └── siem/page.tsx             # SIEM configuration
```

**Key Features:**
1. Alert channel configuration (email, Slack, webhook)
2. Real-time alert stream
3. Compliance report generator
4. Export to CSV/PDF
5. SIEM destination configuration

---

## Implementation Priority Matrix

| Feature | Impact | Effort | Priority |
|---------|--------|--------|----------|
| Permission/ACL tracking | High | Medium | P1 |
| Real-time alerting | High | Medium | P1 |
| Hash improvements | Medium | Low | P1 |
| auditd integration | High | High | P2 |
| SIEM integration | High | Medium | P2 |
| Compliance reports | High | High | P2 |
| Audit trail integrity | Medium | Medium | P3 |
| Scheduled reports | Medium | Medium | P3 |
| Enhanced frontend | Medium | Medium | P3 |

---

## Technical Notes

### Platform Support

| Feature | Linux | macOS | Windows |
|---------|-------|-------|---------|
| inotify/fsnotify | ✅ | ⚠️ Limited | ❌ |
| auditd | ✅ | ❌ | ❌ |
| ACL (getfacl) | ✅ | ⚠️ Limited | ❌ |
| SELinux | ✅ | N/A | N/A |
| Extended attrs | ✅ | ✅ | ❌ |

### Performance Considerations

1. **auditd correlation**: Use efficient grep/awk on audit.log
2. **ACL capture**: Cache ACLs, update on change only
3. **SIEM buffering**: Batch events (configurable, default 100 events)
4. **Report generation**: Background job, async download

### Security Considerations

1. **SIEM credentials**: Encrypt in database, use env vars
2. **Alert channels**: Support TLS for all
3. **Report signing**: Optional GPG signature
4. **Audit log protection**: Write-ahead log, immutable after creation

---

## Migration Strategy

### Phase 1 - No Breaking Changes
- Add nullable columns
- Backfill existing data where possible
- Feature flags for new functionality

### Phase 2 - Graceful Degradation
- auditd integration: Fallback to current if not available
- ACL monitoring: Skip if platform doesn't support
- SIEM: Queue events if destination unreachable

### Phase 3 - Cleanup
- Remove feature flags
- Make new columns non-nullable
- Update API contracts

---

## Decisions (2026-08-02)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Alert Channels | **ALL** (Email, Slack, Webhook) | Maximum flexibility, different use cases |
| SIEM Platform | **Elasticsearch (ELK)** | Open source, free tier, scalable, good visualization, easy integration |
| Compliance Framework | **Multi-framework** (SOC2 primary + NIST mapping) | SOC2 best for service providers; NIST for flexibility |
| auditd Access | **Yes** | Full auditd integration available |
| Report Recipients | **Configurable** | Dynamic per-scheduled report |

### Why Elasticsearch (ELK)?

| Criteria | ELK | Splunk | Graylog | Syslog |
|----------|-----|--------|---------|--------|
| Cost | Free (self-hosted) | Expensive | Free (self-hosted) | Free (basic) |
| Scalability | Excellent | Excellent | Good | Limited |
| Visualization | Kibana (excellent) | Built-in (excellent) | Built-in (good) | None |
| Integration | Easy | Easy | Easy | Manual |
| Learning curve | Medium | Low | Low | Low |
| Alerting | Built-in | Built-in | Built-in | External |

**Winner: ELK** - Best balance of features, cost, and flexibility for a journal publishing platform.

### Why SOC2 + NIST?

| Framework | Use Case | Applicability |
|-----------|----------|---------------|
| PCI-DSS | Payment card handling | ❌ Not applicable |
| HIPAA | Healthcare data (PHI) | ❌ Not applicable |
| SOC2 | Service organizations | ✅ Best fit |
| NIST | Any organization | ✅ Flexible framework |

**Winner: SOC2 primary** - Industry standard for demonstrating security controls; **NIST mapping** for customizable controls.

---

## Next Steps

1. ✅ ~~Review this plan~~ - Decisions made
2. ✅ ~~Prioritize phases~~ - Phase 1 (Foundation) first
3. ✅ ~~Technical decisions~~ - All answered
4. **Start implementation** - Begin with Phase 1, Task P1-01

---

## Implementation Ready

**Alert Channels:** Email + Slack + Webhook (all three)
**SIEM Integration:** Elasticsearch (ELK) + Syslog fallback
**Compliance Reports:** SOC2 primary + NIST mapping + Executive summary
**auditd:** Full integration enabled

---

*Generated by Claude Code*
