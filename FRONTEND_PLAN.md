# FRONTEND IMPLEMENTATION PLAN
**Project:** OJS Monitor - Alerting System UI
**Date:** 2026-08-02
**Tech Stack:** Next.js 16, Tailwind CSS v4, Lucide React, Sonner (toasts)

---

## Research Summary

### Frontend Architecture
| Aspect | Implementation |
|--------|----------------|
| Framework | Next.js 16 (App Router) |
| Styling | Tailwind CSS v4 + custom `glass-panel` |
| State | React useState + Context API (no Zustand) |
| Data Fetching | Native `fetch` with manual polling |
| Icons | Lucide React |
| Theme | Dark slate (`#0f172a`) |

### UI/UX Decisions
| Decision | Choice |
|----------|--------|
| Toast Library | **Sonner** (5KB, modern, accessible) |
| Alert History | Table view with filters |
| Channel Config | Tab-based (Email, Slack, Webhook) |
| Validation | Test before save pattern |

---

## Implementation Structure

```
frontend/src/
├── app/projects/[id]/
│   ├── alerts/
│   │   └── page.tsx          # Alert configuration & history
│   └── alerts/page.tsx        # Redirect or tab container
├── components/alerts/
│   ├── AlertConfiguration.tsx    # Main configuration component
│   ├── AlertHistory.tsx          # Alert history table
│   ├── AlertTable.tsx            # Table with filters
│   ├── AlertFilters.tsx          # Filter controls
│   ├── ChannelSelector.tsx        # Tab: Email/Slack/Webhook
│   ├── EmailSettings.tsx         # SMTP form
│   ├── SlackSettings.tsx          # Webhook form
│   ├── WebhookSettings.tsx        # Generic webhook form
│   ├── TestButton.tsx            # Test connection button
│   └── StatusBadge.tsx           # Alert status indicator
└── hooks/
    └── useAlerts.ts              # Alert data fetching hook
```

---

## Components Detail

### 1. AlertConfiguration Page (`/projects/[id]/alerts`)

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│  ← Back to Project                    Alert Configuration   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Email    │ │   Slack     │ │  Webhook    │  + Add    │
│  │    ✓       │ │    ●        │ │             │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
│                                                             │
│  ─────────────────────────────────────────────────────    │
│                                                             │
│  [Channel Settings Form]                                   │
│                                                             │
│  ┌─ Risk Level ────────────────────────────────────────┐   │
│  │ Alert when risk is: [HIGH ▼]                         │   │
│  └────────────────────────────────────────────────────┘   │
│                                                             │
│  [Test Connection]  [Save Configuration]                    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  Recent Alerts                                             │
│  ─────────────────────────────────────────────────────    │
│  [Table with recent alerts...]                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

### 2. EmailSettings Component

**Fields:**
```typescript
interface EmailConfig {
  smtp_host: string;      // Required
  smtp_port: number;      // Default: 587
  smtp_user?: string;     // Optional
  smtp_password?: string; // Optional, masked
  from_address: string;   // Required
  to_addresses: string[]; // Required, at least 1
  use_tls: boolean;       // Default: true
}
```

**Validation:**
- SMTP Host: Required, valid hostname
- Port: 1-65535
- From Address: Valid email format
- To Addresses: At least 1 valid email
- Test button: Validates before save

---

### 3. SlackSettings Component

**Fields:**
```typescript
interface SlackConfig {
  webhook_url: string;  // Required, valid Slack webhook URL
  channel?: string;    // Optional, override channel
  username?: string;    // Optional, bot name
  icon_emoji?: string; // Optional, :warning:
}
```

**Validation:**
- Webhook URL: Must be valid Slack webhook format
- Test button: Sends test message

---

### 4. WebhookSettings Component

**Fields:**
```typescript
interface WebhookConfig {
  url: string;           // Required, valid URL
  method: 'POST' | 'PUT' | 'PATCH';
  headers?: Record<string, string>;
  body_template?: string;
}
```

**Template Variables:**
- `{{.EventType}}`
- `{{.FilePath}}`
- `{{.FileHash}}`
- `{{.FileMode}}`
- `{{.RiskLevel}}`
- `{{.ProjectName}}`
- `{{.Timestamp}}`
- `{{.ActorName}}`

---

### 5. AlertHistory Component

**Features:**
- Paginated table (20 per page)
- Filterable by: status, channel, date range
- Sortable columns
- Status badges: Sent (green), Failed (red), Pending (yellow)
- Click to view details

**Columns:**
| Column | Type |
|--------|------|
| Severity | Badge (CRITICAL/HIGH/MEDIUM/LOW) |
| Event | Event type + file path |
| Channel | Email/Slack/Webhook icon |
| Status | Sent/Failed/Acknowledged |
| Time | Relative time |
| Actions | Acknowledge button |

---

## API Integration

### Endpoints Used

```typescript
// Get alert configurations
GET /api/projects/{id}/alerts/config

// Save/update alert configuration
POST /api/projects/{id}/alerts/config
{
  "channel_type": "email",
  "config": { /* channel-specific config */ },
  "enabled": true,
  "min_risk_level": "HIGH"
}

// Delete alert configuration
DELETE /api/projects/{id}/alerts/config?channel_type=email

// Test alert configuration
POST /api/projects/{id}/alerts/test
{
  "channel_type": "email",
  "config": { /* test config */ }
}

// Get alert history
GET /api/projects/{id}/alerts/history?limit=20&offset=0

// Acknowledge alert
PUT /api/alerts/{id}/acknowledge
```

---

## State Management

**Pattern:** Following existing codebase patterns (useState + useEffect)

```typescript
// useAlerts.ts
"use client";

import { useState, useEffect, useCallback } from "react";
import { getAuthHeaders } from "@/contexts/AuthContext";

export function useAlerts(projectId: string) {
  const [configs, setConfigs] = useState<AlertConfig[]>([]);
  const [history, setHistory] = useState<AlertHistory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const fetchConfigs = useCallback(async () => {
    // Fetch configurations
  }, [projectId]);

  const fetchHistory = useCallback(async () => {
    // Fetch alert history
  }, [projectId]);

  const saveConfig = async (config: AlertConfig) => {
    // Save configuration
  };

  const testConfig = async (channelType: string, config: object) => {
    // Test configuration
  };

  const acknowledge = async (alertId: number) => {
    // Acknowledge alert
  };

  return {
    configs,
    history,
    loading,
    saving,
    fetchConfigs,
    fetchHistory,
    saveConfig,
    testConfig,
    acknowledge,
  };
}
```

---

## Component States

### AlertConfiguration States

```typescript
type ConfigState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "editing"; channel: string; config: object }
  | { status: "saving" }
  | { status: "testing"; channel: string }
  | { status: "success"; message: string }
  | { status: "error"; message: string };
```

### Test Button States

```typescript
type TestState =
  | { status: "idle" }
  | { status: "testing" }
  | { status: "success"; message: string }
  | { status: "error"; message: string };
```

---

## Accessibility

### Implementation Checklist
- [ ] `role="alert"` for critical toasts
- [ ] `aria-live="polite"` for non-critical
- [ ] Keyboard navigation (Tab, Enter, Escape)
- [ ] Focus management on modals
- [ ] Color contrast 4.5:1
- [ ] Screen reader text for status icons

### Example: StatusBadge
```tsx
<span
  className={statusStyles[status]}
  role="status"
  aria-label={`Alert status: ${status}`}
>
  {status === "sent" && <CheckCircle className="w-4 h-4" />}
  {status === "failed" && <XCircle className="w-4 h-4" />}
  <span className="ml-1">{label}</span>
</span>
```

---

## Responsive Design

### Breakpoints
| Breakpoint | Layout |
|------------|--------|
| Mobile (<640px) | Full-width cards, stacked form fields |
| Tablet (640-1024px) | 2-column form, compact table |
| Desktop (>1024px) | Side-by-side layout, full table |

### Mobile Considerations
- Channel tabs → Accordion
- Form fields → Full width
- Table → Card view
- Touch targets: 44px minimum

---

## Color Scheme (Dark Theme)

```css
/* Existing dark theme */
--background: #0f172a;
--foreground: #f8fafc;
--glass-panel: rgba(30, 41, 59, 0.4);

/* Alert-specific */
--severity-critical: #dc2626;  /* red-600 */
--severity-high: #f97316;      /* orange-500 */
--severity-medium: #eab308;    /* yellow-500 */
--severity-low: #22c55e;       /* green-500 */

--status-sent: #22c55e;        /* green-500 */
--status-failed: #ef4444;       /* red-500 */
--status-pending: #eab308;       /* yellow-500 */
--status-acknowledged: #3b82f6; /* blue-500 */
```

---

## Implementation Order

1. **Setup** - Install Sonner, create directory structure
2. **Types** - Define TypeScript interfaces
3. **API Hook** - Create useAlerts hook
4. **Components** - Build reusable components
5. **Page** - Assemble AlertConfiguration page
6. **Navigation** - Add link to project sidebar

---

## Testing Checklist

- [ ] Test email configuration save
- [ ] Test Slack webhook delivery
- [ ] Test webhook with custom headers
- [ ] Verify alert history displays correctly
- [ ] Test acknowledge functionality
- [ ] Test risk level filtering
- [ ] Verify responsive layout
- [ ] Test keyboard navigation
- [ ] Verify dark theme consistency

---

*Generated by Claude Code - Based on UI/UX research*
