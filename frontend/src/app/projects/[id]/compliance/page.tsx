"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  FileText,
  Download,
  RefreshCw,
  Plus,
  Trash2,
  Shield,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Clock,
  Calendar,
  FileJson,
  FileSpreadsheet,
  Lock,
  Unlock,
  TrendingUp,
} from "lucide-react";
import Link from "next/link";
import ProtectedLayout from "@/components/ProtectedLayout";
import { Toaster, toast } from "sonner";

interface ScheduledReport {
  id: number;
  project_id: number;
  name: string;
  framework: string;
  format: string;
  schedule_cron: string;
  recipients: string[];
  enabled: boolean;
  last_run: number;
  next_run: number;
  created_at: number;
}

interface IntegrityStatus {
  valid: boolean;
  errors: string[];
  verified_at: number;
}

interface ReportData {
  project_name: string;
  generated_at: string;
  period: { start: string; end: string };
  summary: {
    total_events: number;
    critical_events: number;
    high_events: number;
    medium_events: number;
    low_events: number;
    created_events: number;
    modified_events: number;
    deleted_events: number;
    permission_changes: number;
    unknown_source_events: number;
  };
  baseline: { total_files: number };
  high_risk_files: Array<{ path: string; event_count: number; last_event: string; risk_level: string }>;
  compliance_controls: Array<{
    id: string;
    name: string;
    status: string;
    framework: string;
    evidence: string;
  }>;
}

const FRAMEWORKS = [
  { value: "soc2", label: "SOC 2", description: "Service Organization Control 2" },
  { value: "nist", label: "NIST", description: "NIST SP 800-53" },
  { value: "executive", label: "Executive", description: "Management Summary" },
];

const FORMATS = [
  { value: "json", label: "JSON", icon: FileJson },
  { value: "csv", label: "CSV", icon: FileSpreadsheet },
];

export default function CompliancePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const projectId = parseInt(id);

  // State
  const [reports, setReports] = useState<ScheduledReport[]>([]);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [reportData, setReportData] = useState<ReportData | null>(null);
  const [integrity, setIntegrity] = useState<IntegrityStatus | null>(null);
  const [activeTab, setActiveTab] = useState<"overview" | "scheduled" | "integrity">("overview");

  // Form state
  const [showForm, setShowForm] = useState(false);
  const [newReport, setNewReport] = useState({
    name: "",
    framework: "soc2",
    format: "json",
    schedule_cron: "0 6 * * 1",
    recipients: "",
    enabled: true,
  });

  // Fetch data
  const fetchReports = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/reports`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (Array.isArray(data)) {
        setReports(data);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const verifyIntegrity = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/reports/verify`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success !== false) {
        setIntegrity(data);
      }
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    fetchReports();
    setLoading(false);
  }, [projectId]);

  // Generate report
  const handleGenerate = async (framework: string, format: string, periodDays: number = 30) => {
    setGenerating(true);

    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/reports/generate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ framework, format, period_days: periodDays }),
      });

      if (format === "json") {
        const data = await res.json();
        if (res.ok) {
          setReportData(data);
          toast.success("Report generated successfully!");
        } else {
          toast.error(data.error || "Failed to generate report");
        }
      } else if (format === "csv") {
        // Download CSV file
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `fim-report-${framework}-${new Date().toISOString().split("T")[0]}.csv`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        toast.success("CSV report downloaded!");
      }
    } catch (err) {
      toast.error("Failed to generate report");
    }

    setGenerating(false);
  };

  // Create scheduled report
  const handleCreateReport = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    const recipients = newReport.recipients.split(",").map((s) => s.trim()).filter(Boolean);

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${projectId}/reports`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          ...newReport,
          recipients,
        }),
      });

      const data = await res.json();
      if (data.success) {
        toast.success("Scheduled report created!");
        fetchReports();
        setShowForm(false);
        setNewReport({
          name: "",
          framework: "soc2",
          format: "json",
          schedule_cron: "0 6 * * 1",
          recipients: "",
          enabled: true,
        });
      } else {
        toast.error(data.error || "Failed to create report");
      }
    } catch (err) {
      toast.error("Failed to create report");
    }
  };

  // Delete report
  const handleDeleteReport = async (reportId: number) => {
    if (!confirm("Are you sure you want to delete this scheduled report?")) return;

    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/reports/${reportId}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success) {
        toast.success("Report deleted");
        fetchReports();
      }
    } catch (err) {
      toast.error("Failed to delete report");
    }
  };

  // Get control status color
  const getControlStatusColor = (status: string) => {
    switch (status) {
      case "Pass":
        return "text-green-400";
      case "Fail":
        return "text-red-400";
      default:
        return "text-yellow-400";
    }
  };

  // Get control status icon
  const getControlStatusIcon = (status: string) => {
    switch (status) {
      case "Pass":
        return <CheckCircle className="w-4 h-4" />;
      case "Fail":
        return <XCircle className="w-4 h-4" />;
      default:
        return <AlertTriangle className="w-4 h-4" />;
    }
  };

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
        <span className="text-slate-200 text-sm font-medium">Compliance</span>
      </div>

      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-blue-500/20">
            <Shield className="w-6 h-6 text-blue-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-slate-100">Compliance Reports</h1>
            <p className="text-slate-400 text-sm">SOC 2, NIST, and Executive summaries</p>
          </div>
        </div>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-2 mb-6">
        {[
          { id: "overview" as const, label: "Overview", icon: FileText },
          { id: "scheduled" as const, label: "Scheduled Reports", icon: Calendar },
          { id: "integrity" as const, label: "Audit Integrity", icon: Lock },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg font-medium transition-colors ${
              activeTab === tab.id
                ? "bg-blue-500/20 text-blue-400 border border-blue-500/50"
                : "bg-slate-800 text-slate-400 border border-slate-700 hover:bg-slate-700"
            }`}
          >
            <tab.icon className="w-4 h-4" />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Overview Tab */}
      {activeTab === "overview" && (
        <div className="space-y-6">
          {/* Quick Generate */}
          <div className="glass-panel rounded-xl p-6">
            <h2 className="text-lg font-semibold text-slate-200 mb-4">Generate Report</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {FRAMEWORKS.map((fw) => (
                <div key={fw.value} className="space-y-3">
                  <div className="p-4 bg-slate-800 rounded-lg">
                    <h3 className="font-medium text-slate-200">{fw.label}</h3>
                    <p className="text-sm text-slate-400">{fw.description}</p>
                  </div>
                  <div className="flex gap-2">
                    {FORMATS.map((fmt) => (
                      <button
                        key={fmt.value}
                        onClick={() => handleGenerate(fw.value, fmt.value)}
                        disabled={generating}
                        className="flex-1 flex items-center justify-center gap-2 px-3 py-2 bg-slate-700 hover:bg-slate-600 rounded-lg text-sm text-slate-300 transition-colors disabled:opacity-50"
                      >
                        {generating ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <>
                            <fmt.icon className="w-4 h-4" />
                            {fmt.label}
                          </>
                        )}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Report Preview */}
          {reportData && (
            <div className="glass-panel rounded-xl p-6">
              <h2 className="text-lg font-semibold text-slate-200 mb-4">Report Preview</h2>

              {/* Summary */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                <div className="p-4 bg-slate-800 rounded-lg">
                  <p className="text-sm text-slate-400">Total Events</p>
                  <p className="text-2xl font-bold text-slate-100">{reportData.summary.total_events}</p>
                </div>
                <div className="p-4 bg-red-500/10 border border-red-500/20 rounded-lg">
                  <p className="text-sm text-slate-400">Critical</p>
                  <p className="text-2xl font-bold text-red-400">{reportData.summary.critical_events}</p>
                </div>
                <div className="p-4 bg-orange-500/10 border border-orange-500/20 rounded-lg">
                  <p className="text-sm text-slate-400">High</p>
                  <p className="text-2xl font-bold text-orange-400">{reportData.summary.high_events}</p>
                </div>
                <div className="p-4 bg-yellow-500/10 border border-yellow-500/20 rounded-lg">
                  <p className="text-sm text-slate-400">Medium</p>
                  <p className="text-2xl font-bold text-yellow-400">{reportData.summary.medium_events}</p>
                </div>
              </div>

              {/* Compliance Controls */}
              <h3 className="font-medium text-slate-300 mb-3">Compliance Controls</h3>
              <div className="space-y-2">
                {reportData.compliance_controls.map((control) => (
                  <div key={control.id} className="flex items-center justify-between p-3 bg-slate-800 rounded-lg">
                    <div className="flex items-center gap-3">
                      <span className={getControlStatusColor(control.status)}>
                        {getControlStatusIcon(control.status)}
                      </span>
                      <div>
                        <p className="text-sm font-medium text-slate-200">
                          {control.id}: {control.name}
                        </p>
                        <p className="text-xs text-slate-500">{control.framework}</p>
                      </div>
                    </div>
                    <span className={`text-sm font-medium ${getControlStatusColor(control.status)}`}>
                      {control.status}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Scheduled Reports Tab */}
      {activeTab === "scheduled" && (
        <div className="space-y-6">
          {/* Create New */}
          <div className="glass-panel rounded-xl p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-slate-200">Scheduled Reports</h2>
              <button
                onClick={() => setShowForm(!showForm)}
                className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded-lg text-white text-sm font-medium transition-colors"
              >
                <Plus className="w-4 h-4" />
                New Schedule
              </button>
            </div>

            {showForm && (
              <div className="p-4 bg-slate-800 rounded-lg space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-1">Report Name</label>
                    <input
                      type="text"
                      value={newReport.name}
                      onChange={(e) => setNewReport({ ...newReport, name: e.target.value })}
                      placeholder="Weekly SOC2 Report"
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-1">Framework</label>
                    <select
                      value={newReport.framework}
                      onChange={(e) => setNewReport({ ...newReport, framework: e.target.value })}
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200"
                    >
                      {FRAMEWORKS.map((fw) => (
                        <option key={fw.value} value={fw.value}>{fw.label}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-1">Schedule (cron)</label>
                    <input
                      type="text"
                      value={newReport.schedule_cron}
                      onChange={(e) => setNewReport({ ...newReport, schedule_cron: e.target.value })}
                      placeholder="0 6 * * 1"
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500"
                    />
                    <p className="text-xs text-slate-500 mt-1">Format: minute hour day month weekday</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-1">Recipients (comma-separated)</label>
                    <input
                      type="text"
                      value={newReport.recipients}
                      onChange={(e) => setNewReport({ ...newReport, recipients: e.target.value })}
                      placeholder="admin@example.com, security@example.com"
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500"
                    />
                  </div>
                </div>
                <div className="flex justify-end gap-2">
                  <button
                    onClick={() => setShowForm(false)}
                    className="px-4 py-2 bg-slate-700 hover:bg-slate-600 rounded-lg text-slate-300 text-sm font-medium"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleCreateReport}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded-lg text-white text-sm font-medium"
                  >
                    Create Schedule
                  </button>
                </div>
              </div>
            )}

            {/* Report List */}
            {reports.length === 0 ? (
              <div className="text-center py-12 text-slate-500">
                <Calendar className="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>No scheduled reports</p>
                <p className="text-sm">Create a schedule above to automate report generation</p>
              </div>
            ) : (
              <div className="space-y-2 mt-4">
                {reports.map((report) => (
                  <div key={report.id} className="flex items-center justify-between p-4 bg-slate-800 rounded-lg">
                    <div className="flex items-center gap-4">
                      <div className="p-2 bg-blue-500/20 rounded-lg">
                        <FileText className="w-5 h-5 text-blue-400" />
                      </div>
                      <div>
                        <p className="font-medium text-slate-200">{report.name}</p>
                        <p className="text-sm text-slate-400">
                          {FRAMEWORKS.find((f) => f.value === report.framework)?.label} • {report.schedule_cron}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {report.enabled ? (
                        <span className="px-2 py-1 bg-green-500/20 text-green-400 text-xs rounded">Enabled</span>
                      ) : (
                        <span className="px-2 py-1 bg-slate-600 text-slate-400 text-xs rounded">Disabled</span>
                      )}
                      <button
                        onClick={() => handleDeleteReport(report.id)}
                        className="p-2 text-slate-400 hover:text-red-400 transition-colors"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Integrity Tab */}
      {activeTab === "integrity" && (
        <div className="space-y-6">
          <div className="glass-panel rounded-xl p-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h2 className="text-lg font-semibold text-slate-200">Audit Trail Integrity</h2>
                <p className="text-sm text-slate-400">Verify the integrity of the event chain using hash chaining</p>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={verifyIntegrity}
                  className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 rounded-lg text-slate-300 text-sm font-medium"
                >
                  <RefreshCw className="w-4 h-4" />
                  Verify Chain
                </button>
              </div>
            </div>

            {integrity && (
              <div className={`p-4 rounded-lg ${
                integrity.valid
                  ? "bg-green-500/10 border border-green-500/20"
                  : "bg-red-500/10 border border-red-500/20"
              }`}>
                <div className="flex items-center gap-2 mb-3">
                  {integrity.valid ? (
                    <>
                      <CheckCircle className="w-5 h-5 text-green-400" />
                      <span className="font-medium text-green-400">Chain Integrity Verified</span>
                    </>
                  ) : (
                    <>
                      <XCircle className="w-5 h-5 text-red-400" />
                      <span className="font-medium text-red-400">Integrity Issues Detected</span>
                    </>
                  )}
                </div>

                {integrity.errors.length > 0 && (
                  <div className="mt-3 p-3 bg-slate-900 rounded-lg">
                    <p className="text-sm text-slate-400 mb-2">Errors found:</p>
                    <ul className="space-y-1">
                      {integrity.errors.slice(0, 5).map((error, i) => (
                        <li key={i} className="text-sm text-red-400 font-mono">{error}</li>
                      ))}
                      {integrity.errors.length > 5 && (
                        <li className="text-sm text-slate-500">
                          ... and {integrity.errors.length - 5} more errors
                        </li>
                      )}
                    </ul>
                  </div>
                )}

                <p className="text-xs text-slate-500 mt-3">
                  Last verified: {new Date(integrity.verified_at * 1000).toLocaleString()}
                </p>
              </div>
            )}

            {!integrity && (
              <div className="text-center py-8 text-slate-500">
                <Lock className="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>Click "Verify Chain" to check audit trail integrity</p>
              </div>
            )}
          </div>
        </div>
      )}
    </ProtectedLayout>
  );
}
