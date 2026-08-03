# DECISIONS.md
**Project:** OJS Monitor - Enterprise FIM
**Date:** 2026-08-02

---

## ✅ Decisions Made

### 1. Alert Channels
**Decision:** ALL channels will be implemented

| Channel | Implementation | Priority |
|---------|----------------|----------|
| Email | SMTP with TLS | Required |
| Slack | Slack Webhook API | Required |
| Webhook | HTTP POST (configurable) | Required |
| Syslog | RFC 5424 | Included in SIEM |

### 2. SIEM Platform
**Decision:** Elasticsearch (ELK) Stack

| Criteria | ELK | Rationale |
|----------|-----|-----------|
| Cost | Free (self-hosted) | Budget-friendly |
| Scalability | Excellent | Can grow with needs |
| Visualization | Kibana | Excellent dashboards |
| Integration | Native Go client | Easy to implement |
| Alerting | Built-in Watcher | Query-based alerts |
| Learning curve | Medium | Well documented |

**Fallback:** Syslog (RFC 5424) for basic integration

### 3. Compliance Framework
**Decision:** Multi-tier approach

| Framework | Priority | Use Case |
|-----------|----------|----------|
| **SOC 2 CC7.2** | Primary | Security incident identification |
| **NIST SP 800-53** | Secondary | AU-14 (Audit Logging), SI-7 (Software Integrity) |
| **Executive Summary** | Tertiary | Management/board reporting |

**Not Applicable:**
- ❌ PCI-DSS (payment card handling - not applicable)
- ❌ HIPAA (healthcare PHI - not applicable)
- ❌ ISO 27001 (can be added later)

### 4. auditd Integration
**Decision:** Full integration enabled

- ✅ Server has root/auditd access
- ✅ Can create custom audit rules
- ✅ Can parse audit.log
- ✅ Full user/process attribution available

### 5. Report Recipients
**Decision:** Configurable per schedule

- Each scheduled report can have different recipients
- Recipients stored as JSON array in database
- Email validation on save
- Support for multiple recipients per report

---

## 📋 Scope Summary

### In Scope
- File permission/ownership tracking (mode, uid, gid)
- ACL monitoring (Linux getfacl)
- Extended attributes (xattr)
- SELinux context tracking
- Real-time alerting (all channels)
- auditd integration (full)
- SIEM integration (Elasticsearch primary)
- Compliance reports (SOC2 + NIST)
- Scheduled reports with email delivery
- Audit trail hash chain

### Out of Scope (for now)
- PCI-DSS compliance report
- HIPAA compliance report
- Windows NTFS ACL monitoring
- macOS extended attributes
- YARA rule integration
- Binary diff/patch detection

---

## 🔧 Implementation Order

1. **Phase 1**: Foundation (Permission + Alerting + Hash)
2. **Phase 2**: Advanced (auditd + ACL + SIEM)
3. **Phase 3**: Compliance (Reports + Integrity)
4. **Phase 4**: UX (Frontend enhancements)

---

*Document this file - Update when decisions change*
