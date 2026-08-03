"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  Bell,
  Mail,
  MessageSquare,
  WebhookIcon,
  Plus,
  Trash2,
  Send,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Clock,
  Eye,
  Settings,
  RefreshCw,
  ShieldAlert,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import Link from "next/link";
import ProtectedLayout from "@/components/ProtectedLayout";
import { Toaster, toast } from "sonner";

// Types
interface AlertConfig {
  id: number;
  project_id: number;
  channel_type: string;
  config: Record<string, any>;
  enabled: boolean;
  min_risk_level: string;
  created_at: number;
  updated_at: number;
}

interface AlertHistory {
  id: number;
  project_id: number;
  event_id: number;
  channel_type: string;
  status: string;
  error_message: string;
  retry_count: number;
  sent_at: number;
  created_at: number;
  event_type: string;
  file_path: string;
}

// Risk level options
const RISK_LEVELS = [
  { value: "LOW", label: "Low", color: "text-green-400" },
  { value: "MEDIUM", label: "Medium", color: "text-yellow-400" },
  { value: "HIGH", label: "High", color: "text-orange-400" },
  { value: "CRITICAL", label: "Critical", color: "text-red-400" },
];

// Channel icons
const ChannelIcon = ({ channel, className = "w-4 h-4" }: { channel: string; className?: string }) => {
  switch (channel) {
    case "email":
      return <Mail className={className} />;
    case "slack":
      return <MessageSquare className={className} />;
    case "webhook":
      return <WebhookIcon className={className} />;
    default:
      return <Bell className={className} />;
  }
};

// Status badge component
const StatusBadge = ({ status }: { status: string }) => {
  const styles: Record<string, { bg: string; text: string; icon: React.ReactNode }> = {
    sent: {
      bg: "bg-green-500/20",
      text: "text-green-400",
      icon: <CheckCircle className="w-3 h-3" />,
    },
    failed: {
      bg: "bg-red-500/20",
      text: "text-red-400",
      icon: <XCircle className="w-3 h-3" />,
    },
    pending: {
      bg: "bg-yellow-500/20",
      text: "text-yellow-400",
      icon: <Clock className="w-3 h-3" />,
    },
    acknowledged: {
      bg: "bg-blue-500/20",
      text: "text-blue-400",
      icon: <Eye className="w-3 h-3" />,
    },
  };

  const style = styles[status] || styles.pending;

  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${style.bg} ${style.text}`}>
      {style.icon}
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
};

// Risk badge component
const RiskBadge = ({ level }: { level: string }) => {
  const styles: Record<string, { bg: string; text: string }> = {
    LOW: { bg: "bg-green-500/20", text: "text-green-400" },
    MEDIUM: { bg: "bg-yellow-500/20", text: "text-yellow-400" },
    HIGH: { bg: "bg-orange-500/20", text: "text-orange-400" },
    CRITICAL: { bg: "bg-red-500/20", text: "text-red-400" },
  };

  const style = styles[level] || styles.MEDIUM;

  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-bold ${style.bg} ${style.text}`}>
      {level}
    </span>
  );
};

export default function AlertsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const projectId = parseInt(id);

  // State
  const [configs, setConfigs] = useState<AlertConfig[]>([]);
  const [history, setHistory] = useState<AlertHistory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"email" | "slack" | "webhook">("email");
  const [showForm, setShowForm] = useState(false);

  // Form state for each channel
  const [emailForm, setEmailForm] = useState({
    smtp_host: "",
    smtp_port: "587",
    smtp_user: "",
    smtp_password: "",
    from_address: "",
    to_addresses: "",
    use_tls: true,
  });

  const [slackForm, setSlackForm] = useState({
    webhook_url: "",
    channel: "",
    username: "",
    icon_emoji: "",
  });

  const [webhookForm, setWebhookForm] = useState({
    url: "",
    method: "POST",
    headers: "",
  });

  // Risk level state
  const [riskLevel, setRiskLevel] = useState("HIGH");
  const [enabled, setEnabled] = useState(true);

  // Fetch data
  const fetchConfigs = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/alerts/config`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (Array.isArray(data)) {
        setConfigs(data);

        // Populate forms with existing config
        const existingConfig = data.find((c: AlertConfig) => c.channel_type === activeTab);
        if (existingConfig) {
          setRiskLevel(existingConfig.min_risk_level || "HIGH");
          setEnabled(existingConfig.enabled);

          if (activeTab === "email" && existingConfig.config) {
            setEmailForm({
              smtp_host: existingConfig.config.smtp_host || "",
              smtp_port: String(existingConfig.config.smtp_port || "587"),
              smtp_user: existingConfig.config.smtp_user || "",
              smtp_password: existingConfig.config.smtp_password || "",
              from_address: existingConfig.config.from_address || "",
              to_addresses: Array.isArray(existingConfig.config.to_addresses)
                ? existingConfig.config.to_addresses.join(", ")
                : "",
              use_tls: existingConfig.config.use_tls !== false,
            });
          } else if (activeTab === "slack" && existingConfig.config) {
            setSlackForm({
              webhook_url: existingConfig.config.webhook_url || "",
              channel: existingConfig.config.channel || "",
              username: existingConfig.config.username || "",
              icon_emoji: existingConfig.config.icon_emoji || "",
            });
          } else if (activeTab === "webhook" && existingConfig.config) {
            const headers = existingConfig.config.headers || {};
            setWebhookForm({
              url: existingConfig.config.url || "",
              method: existingConfig.config.method || "POST",
              headers: Object.entries(headers)
                .map(([k, v]) => `${k}: ${v}`)
                .join("\n"),
            });
          }
        }
      }
    } catch (err) {
      console.error(err);
    }
  };

  const fetchHistory = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/alerts/history?limit=50`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (Array.isArray(data)) {
        setHistory(data);
      }
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    fetchConfigs();
    fetchHistory();
    setLoading(false);

    // Refresh every 30 seconds
    const interval = setInterval(() => {
      fetchHistory();
    }, 30000);

    return () => clearInterval(interval);
  }, [projectId]);

  // Save configuration
  const handleSave = async () => {
    setSaving(true);

    let config: Record<string, any> = {};

    if (activeTab === "email") {
      config = {
        smtp_host: emailForm.smtp_host,
        smtp_port: parseInt(emailForm.smtp_port) || 587,
        smtp_user: emailForm.smtp_user,
        smtp_password: emailForm.smtp_password,
        from_address: emailForm.from_address,
        to_addresses: emailForm.to_addresses.split(",").map((s) => s.trim()).filter(Boolean),
        use_tls: emailForm.use_tls,
      };
    } else if (activeTab === "slack") {
      config = {
        webhook_url: slackForm.webhook_url,
        channel: slackForm.channel,
        username: slackForm.username,
        icon_emoji: slackForm.icon_emoji,
      };
    } else if (activeTab === "webhook") {
      const headers: Record<string, string> = {};
      webhookForm.headers.split("\n").forEach((line) => {
        const [key, ...valueParts] = line.split(":");
        if (key && valueParts.length) {
          headers[key.trim()] = valueParts.join(":").trim();
        }
      });
      config = {
        url: webhookForm.url,
        method: webhookForm.method,
        headers,
      };
    }

    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/alerts/config`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          channel_type: activeTab,
          config,
          enabled,
          min_risk_level: riskLevel,
        }),
      });

      const data = await res.json();

      if (data.success) {
        toast.success("Configuration saved successfully!");
        fetchConfigs();
        setShowForm(false);
      } else {
        toast.error(data.error || "Failed to save configuration");
      }
    } catch (err) {
      toast.error("Failed to save configuration");
    }

    setSaving(false);
  };

  // Test configuration
  const handleTest = async () => {
    setTesting(activeTab);

    let config: Record<string, any> = {};

    if (activeTab === "email") {
      config = {
        smtp_host: emailForm.smtp_host,
        smtp_port: parseInt(emailForm.smtp_port) || 587,
        smtp_user: emailForm.smtp_user,
        smtp_password: emailForm.smtp_password,
        from_address: emailForm.from_address,
        to_addresses: emailForm.to_addresses.split(",").map((s) => s.trim()).filter(Boolean),
        use_tls: emailForm.use_tls,
      };
    } else if (activeTab === "slack") {
      config = {
        webhook_url: slackForm.webhook_url,
        channel: slackForm.channel,
        username: slackForm.username,
        icon_emoji: slackForm.icon_emoji,
      };
    } else if (activeTab === "webhook") {
      const headers: Record<string, string> = {};
      webhookForm.headers.split("\n").forEach((line) => {
        const [key, ...valueParts] = line.split(":");
        if (key && valueParts.length) {
          headers[key.trim()] = valueParts.join(":").trim();
        }
      });
      config = {
        url: webhookForm.url,
        method: webhookForm.method,
        headers,
      };
    }

    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/alerts/test`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          channel_type: activeTab,
          config,
        }),
      });

      const data = await res.json();

      if (data.success) {
        toast.success(`Test alert sent via ${activeTab}!`);
      } else {
        toast.error(data.message || `Failed to send test alert via ${activeTab}`);
      }
    } catch (err) {
      toast.error(`Failed to send test alert via ${activeTab}`);
    }

    setTesting(null);
  };

  // Delete configuration
  const handleDelete = async () => {
    if (!confirm(`Are you sure you want to delete the ${activeTab} configuration?`)) {
      return;
    }

    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(
        `http://localhost:8080/api/projects/${projectId}/alerts/config?channel_type=${activeTab}`,
        {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
        }
      );

      const data = await res.json();

      if (data.success) {
        toast.success("Configuration deleted");
        fetchConfigs();

        // Reset forms
        setEmailForm({
          smtp_host: "",
          smtp_port: "587",
          smtp_user: "",
          smtp_password: "",
          from_address: "",
          to_addresses: "",
          use_tls: true,
        });
        setSlackForm({
          webhook_url: "",
          channel: "",
          username: "",
          icon_emoji: "",
        });
        setWebhookForm({
          url: "",
          method: "POST",
          headers: "",
        });
      } else {
        toast.error("Failed to delete configuration");
      }
    } catch (err) {
      toast.error("Failed to delete configuration");
    }
  };

  // Check if channel is configured
  const isConfigured = (channel: string) => {
    return configs.some((c) => c.channel_type === channel && c.enabled);
  };

  // Get current form values
  const currentForm = activeTab === "email" ? emailForm : activeTab === "slack" ? slackForm : webhookForm;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <ProtectedLayout>
      <Toaster position="top-right" theme="dark" />

      {/* Breadcrumb */}
      <div className="flex items-center gap-2 mb-4">
        <Link
          href={`/projects/${id}`}
          className="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
        </Link>
        <span className="text-slate-600">/</span>
        <Link href="/" className="text-slate-400 text-sm hover:text-slate-200">
          Projects
        </Link>
        <span className="text-slate-600">/</span>
        <span className="text-slate-200 text-sm font-medium">Alerts</span>
      </div>

      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-blue-500/20">
            <Bell className="w-6 h-6 text-blue-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-slate-100">Alert Configuration</h1>
            <p className="text-slate-400 text-sm">Configure notification channels for FIM alerts</p>
          </div>
        </div>
      </div>

      {/* Channel Tabs */}
      <div className="flex gap-2 mb-6">
        {[
          { id: "email" as const, label: "Email", icon: Mail },
          { id: "slack" as const, label: "Slack", icon: MessageSquare },
          { id: "webhook" as const, label: "Webhook", icon: WebhookIcon },
        ].map((channel) => (
          <button
            key={channel.id}
            onClick={() => {
              setActiveTab(channel.id);
              setShowForm(false);
            }}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg font-medium transition-colors ${
              activeTab === channel.id
                ? "bg-blue-500/20 text-blue-400 border border-blue-500/50"
                : "bg-slate-800 text-slate-400 border border-slate-700 hover:bg-slate-700"
            }`}
          >
            <channel.icon className="w-4 h-4" />
            {channel.label}
            {isConfigured(channel.id) && (
              <CheckCircle className="w-4 h-4 text-green-400" />
            )}
          </button>
        ))}
      </div>

      {/* Configuration Form */}
      <div className="glass-panel rounded-xl p-6 mb-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Settings className="w-5 h-5 text-slate-400" />
            <h2 className="text-lg font-semibold text-slate-200 capitalize">{activeTab} Settings</h2>
          </div>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-2 text-sm text-slate-400">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="w-4 h-4 rounded border-slate-600 bg-slate-800 text-blue-500 focus:ring-blue-500"
              />
              Enable
            </label>
          </div>
        </div>

        {/* Email Form */}
        {activeTab === "email" && (
          <div className="grid gap-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  SMTP Host <span className="text-red-400">*</span>
                </label>
                <input
                  type="text"
                  value={emailForm.smtp_host}
                  onChange={(e) => setEmailForm({ ...emailForm, smtp_host: e.target.value })}
                  placeholder="smtp.example.com"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Port <span className="text-red-400">*</span>
                </label>
                <input
                  type="number"
                  value={emailForm.smtp_port}
                  onChange={(e) => setEmailForm({ ...emailForm, smtp_port: e.target.value })}
                  placeholder="587"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Username</label>
                <input
                  type="text"
                  value={emailForm.smtp_user}
                  onChange={(e) => setEmailForm({ ...emailForm, smtp_user: e.target.value })}
                  placeholder="alerts@example.com"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Password</label>
                <input
                  type="password"
                  value={emailForm.smtp_password}
                  onChange={(e) => setEmailForm({ ...emailForm, smtp_password: e.target.value })}
                  placeholder="••••••••"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  From Address <span className="text-red-400">*</span>
                </label>
                <input
                  type="email"
                  value={emailForm.from_address}
                  onChange={(e) => setEmailForm({ ...emailForm, from_address: e.target.value })}
                  placeholder="noreply@example.com"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div className="flex items-center gap-4 pt-6">
                <label className="flex items-center gap-2 text-sm text-slate-300">
                  <input
                    type="checkbox"
                    checked={emailForm.use_tls}
                    onChange={(e) => setEmailForm({ ...emailForm, use_tls: e.target.checked })}
                    className="w-4 h-4 rounded border-slate-600 bg-slate-800 text-blue-500 focus:ring-blue-500"
                  />
                  Use TLS
                </label>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">
                Recipients <span className="text-red-400">*</span>
              </label>
              <input
                type="text"
                value={emailForm.to_addresses}
                onChange={(e) => setEmailForm({ ...emailForm, to_addresses: e.target.value })}
                placeholder="admin@example.com, security@example.com (comma separated)"
                className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              />
            </div>
          </div>
        )}

        {/* Slack Form */}
        {activeTab === "slack" && (
          <div className="grid gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">
                Webhook URL <span className="text-red-400">*</span>
              </label>
              <input
                type="text"
                value={slackForm.webhook_url}
                onChange={(e) => setSlackForm({ ...slackForm, webhook_url: e.target.value })}
                placeholder="https://hooks.slack.com/services/..."
                className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Channel Override</label>
                <input
                  type="text"
                  value={slackForm.channel}
                  onChange={(e) => setSlackForm({ ...slackForm, channel: e.target.value })}
                  placeholder="#alerts (optional)"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Bot Username</label>
                <input
                  type="text"
                  value={slackForm.username}
                  onChange={(e) => setSlackForm({ ...slackForm, username: e.target.value })}
                  placeholder="OJS Monitor (optional)"
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Icon Emoji</label>
              <input
                type="text"
                value={slackForm.icon_emoji}
                onChange={(e) => setSlackForm({ ...slackForm, icon_emoji: e.target.value })}
                placeholder=":warning: (optional)"
                className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              />
            </div>
          </div>
        )}

        {/* Webhook Form */}
        {activeTab === "webhook" && (
          <div className="grid gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">
                URL <span className="text-red-400">*</span>
              </label>
              <input
                type="text"
                value={webhookForm.url}
                onChange={(e) => setWebhookForm({ ...webhookForm, url: e.target.value })}
                placeholder="https://your-endpoint.com/webhook"
                className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Method</label>
                <select
                  value={webhookForm.method}
                  onChange={(e) => setWebhookForm({ ...webhookForm, method: e.target.value })}
                  className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                >
                  <option value="POST">POST</option>
                  <option value="PUT">PUT</option>
                  <option value="PATCH">PATCH</option>
                </select>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Headers (one per line)</label>
              <textarea
                value={webhookForm.headers}
                onChange={(e) => setWebhookForm({ ...webhookForm, headers: e.target.value })}
                placeholder="Authorization: Bearer token&#10;X-Custom-Header: value"
                rows={3}
                className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 font-mono text-sm"
              />
            </div>
          </div>
        )}

        {/* Risk Level */}
        <div className="mt-6 pt-6 border-t border-slate-700">
          <label className="block text-sm font-medium text-slate-300 mb-2">Alert when risk level is:</label>
          <div className="flex gap-2">
            {RISK_LEVELS.map((level) => (
              <button
                key={level.value}
                onClick={() => setRiskLevel(level.value)}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                  riskLevel === level.value
                    ? `${level.color} bg-slate-700 border border-current`
                    : "text-slate-400 bg-slate-800 border border-slate-700 hover:bg-slate-700"
                }`}
              >
                {level.label}
              </button>
            ))}
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center justify-between mt-6 pt-6 border-t border-slate-700">
          <div className="flex gap-2">
            {isConfigured(activeTab) && (
              <button
                onClick={handleDelete}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-red-500/20 text-red-400 hover:bg-red-500/30 border border-red-500/50 text-sm font-medium transition-colors"
              >
                <Trash2 className="w-4 h-4" />
                Delete
              </button>
            )}
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleTest}
              disabled={testing === activeTab}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-slate-700 text-slate-300 hover:bg-slate-600 border border-slate-600 text-sm font-medium transition-colors disabled:opacity-50"
            >
              {testing === activeTab ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Send className="w-4 h-4" />
              )}
              Test
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium transition-colors disabled:opacity-50"
            >
              {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <CheckCircle className="w-4 h-4" />}
              Save
            </button>
          </div>
        </div>
      </div>

      {/* Alert History */}
      <div className="glass-panel rounded-xl p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Clock className="w-5 h-5 text-slate-400" />
            <h2 className="text-lg font-semibold text-slate-200">Recent Alerts</h2>
          </div>
          <button
            onClick={fetchHistory}
            className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 text-slate-400 hover:bg-slate-700 border border-slate-700 text-sm transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>

        {history.length === 0 ? (
          <div className="text-center py-12 text-slate-500">
            <Bell className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>No alerts sent yet</p>
            <p className="text-sm">Configure a channel above to start receiving alerts</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-slate-700">
                  <th className="text-left py-3 px-2 text-sm font-medium text-slate-400">Channel</th>
                  <th className="text-left py-3 px-2 text-sm font-medium text-slate-400">Event</th>
                  <th className="text-left py-3 px-2 text-sm font-medium text-slate-400">File</th>
                  <th className="text-left py-3 px-2 text-sm font-medium text-slate-400">Status</th>
                  <th className="text-left py-3 px-2 text-sm font-medium text-slate-400">Time</th>
                </tr>
              </thead>
              <tbody>
                {history.map((alert) => (
                  <tr key={alert.id} className="border-b border-slate-800 hover:bg-slate-800/50">
                    <td className="py-3 px-2">
                      <div className="flex items-center gap-2">
                        <ChannelIcon channel={alert.channel_type} />
                        <span className="text-sm text-slate-300 capitalize">{alert.channel_type}</span>
                      </div>
                    </td>
                    <td className="py-3 px-2">
                      <span className="text-sm text-slate-300">{alert.event_type || "N/A"}</span>
                    </td>
                    <td className="py-3 px-2">
                      <span className="text-sm text-slate-400 max-w-xs truncate block">
                        {alert.file_path || "N/A"}
                      </span>
                    </td>
                    <td className="py-3 px-2">
                      <StatusBadge status={alert.status} />
                    </td>
                    <td className="py-3 px-2">
                      <span className="text-sm text-slate-500">
                        {alert.created_at
                          ? new Date(alert.created_at * 1000).toLocaleString()
                          : "N/A"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </ProtectedLayout>
  );
}
