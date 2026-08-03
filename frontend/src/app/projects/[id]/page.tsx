"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  ShieldAlert,
  ShieldCheck,
  Shield,
  Activity,
  Users,
  Upload,
  Settings,
  RefreshCw,
  FileWarning,
  Terminal,
  Database,
  Server,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Clock,
  Hash,
  Folder,
  Calendar,
  TrendingUp,
  X,
  BookOpen,
  FileText,
  Globe,
  UserCheck,
  Play,
  Bell,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import ProtectedLayout from "@/components/ProtectedLayout";

interface Project {
  id: number;
  name: string;
  description: string;
  template: string;
  status: string;
  baseline_total: number;
  baseline_processed: number;
}

interface Metrics {
  status: string;
  baseline_total: number;
  baseline_processed: number;
  active_admins: number;
  new_users: number;
  validated_users: number;
  unvalidated_disabled: number;
  uploads_by_new_users: number;
  bad_self_reg: number;
  new_files_count: number;
  modified_files_count: number;
  deleted_files_count: number;
  orphan_files_count: number;
}

interface OJSDetails {
  version: string;
  journals: number;
  users: number;
  submissions: number;
  articles: number;
  review_assignments: number;
  primary_locale: string;
  installed_locales: string;
  min_password_len: number;
}

interface FimFile {
  id: number;
  path: string;
  hash: string;
  status: string;
  created_at: string;
  file_type?: string;
}

interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

interface FIMStats {
  today: { added: number; modified: number; deleted: number; total: number };
  this_week: { added: number; modified: number; deleted: number; total: number };
  this_month: { added: number; modified: number; deleted: number; total: number };
  all_time: { added: number; modified: number; deleted: number; orphan: number; total: number };
}

interface Job {
  id: number;
  type: string;
  status: string;
  error: string;
  created_at: string;
  files_success: number;
  files_skipped: number;
  files_error: number;
}

type TabType = "overview" | "fim" | "orphan" | "db" | "jobs";

// Metric Card Component
function MetricCard({
  title,
  value,
  icon: Icon,
  trend,
  variant = "default",
}: {
  title: string;
  value: number;
  icon: React.ReactNode;
  trend?: string;
  variant?: "default" | "success" | "warning" | "danger";
}) {
  const colors = {
    default: "text-slate-400",
    success: "text-emerald-400",
    warning: "text-amber-400",
    danger: "text-red-400",
  };

  const bgColors = {
    default: "bg-slate-800/50",
    success: "bg-emerald-500/10",
    warning: "bg-amber-500/10",
    danger: "bg-red-500/10",
  };

  return (
    <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-slate-500 text-xs font-medium mb-1">{title}</p>
          <p className={`text-2xl font-bold ${colors[variant]}`}>{value}</p>
          {trend && <p className="text-xs text-slate-500 mt-1">{trend}</p>}
        </div>
        <div className={`p-2.5 rounded-lg ${bgColors[variant]}`}>
          <div className={colors[variant]}>{Icon}</div>
        </div>
      </div>
    </div>
  );
}

// Status Badge Component
function StatusBadge({ status }: { status: string }) {
  const variants: Record<string, { class: string; label: string }> = {
    unconfigured: { class: "bg-slate-600/20 text-slate-400", label: "Unconfigured" },
    pending_baseline: { class: "bg-blue-500/20 text-blue-400", label: "Pending" },
    queued: { class: "bg-blue-500/20 text-blue-400", label: "Queued" },
    counting: { class: "bg-amber-500/20 text-amber-400", label: "Counting" },
    scanning: { class: "bg-amber-500/20 text-amber-400", label: "Scanning" },
    reconciling: { class: "bg-amber-500/20 text-amber-400", label: "Reconciling" },
    active: { class: "bg-emerald-500/20 text-emerald-400", label: "Active" },
    error: { class: "bg-red-500/20 text-red-400", label: "Error" },
  };

  const { class: cls, label } = variants[status] || variants.unconfigured;
  return (
    <span className={`px-2.5 py-1 rounded-full text-xs font-semibold ${cls}`}>
      {label}
    </span>
  );
}

export default function ProjectDetail({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const router = useRouter();
  const { id } = use(params);

  const [project, setProject] = useState<Project | null>(null);
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [ojsDetails, setOjsDetails] = useState<OJSDetails | null>(null);
  const [files, setFiles] = useState<FimFile[]>([]);
  const [orphanFiles, setOrphanFiles] = useState<FimFile[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabType>("overview");
  const [scanLoading, setScanLoading] = useState(false);
  const [showScanModal, setShowScanModal] = useState(false);
  const [fimFilter, setFimFilter] = useState<string>("all");
  const [fimSearch, setFimSearch] = useState<string>("");
  const [fimTypeFilter, setFimTypeFilter] = useState<string>("all");
  const [fimPage, setFimPage] = useState<number>(1);
  const [fimPagination, setFimPagination] = useState<PaginationInfo>({ page: 1, limit: 50, total: 0, total_pages: 0 });
  const [fimStats, setFimStats] = useState<FIMStats | null>(null);
  const [orphanPage, setOrphanPage] = useState<number>(1);
  const [orphanPagination, setOrphanPagination] = useState<PaginationInfo>({ page: 1, limit: 50, total: 0, total_pages: 0 });

  const fetchAllData = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const [pRes, mRes, dRes, jRes] = await Promise.all([
        fetch(`http://localhost:8080/api/projects/${id}`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
        fetch(`http://localhost:8080/api/audit/${id}`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
        fetch(`http://localhost:8080/api/projects/${id}/details`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
        fetch(`http://localhost:8080/api/projects/${id}/jobs`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
      ]);

      const [pData, mData, dData, jData] = await Promise.all([
        pRes.json(),
        mRes.json(),
        dRes.json(),
        jRes.json(),
      ]);

      if (pData.success) setProject(pData.data);
      if (mData.success) setMetrics(mData.data);
      if (dData.success) setOjsDetails(dData.data);
      if (jData.success) setJobs(jData.data || []);
    } catch (err) {
      console.error(err);
    }
    setLoading(false);
  };

  const fetchFimFiles = async (page: number = 1, search: string = "", status: string = "all", typeFilter: string = "all") => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const params = new URLSearchParams({ page: String(page), limit: "50" });
      if (search) params.set("search", search);
      if (status && status !== "all") params.set("status", status);
      if (typeFilter && typeFilter !== "all") params.set("type", typeFilter);

      const res = await fetch(`http://localhost:8080/api/projects/${id}/files?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.data?.files) {
        setFiles(data.data.files);
        setFimPagination(data.data.pagination);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const fetchOrphanFiles = async (page: number = 1, search: string = "") => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const params = new URLSearchParams({ page: String(page), limit: "50" });
      if (search) params.set("search", search);

      const res = await fetch(`http://localhost:8080/api/projects/${id}/orphan-files?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.data?.files) {
        setOrphanFiles(data.data.files);
        setOrphanPagination(data.data.pagination);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const fetchFIMStats = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${id}/files/stats`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success && data.data) {
        setFimStats(data.data);
      }
    } catch (err) {
      console.error(err);
    }
  };

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      if (activeTab === "fim") {
        setFimPage(1);
        fetchFimFiles(1, fimSearch, fimFilter, fimTypeFilter);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [fimSearch]);

  useEffect(() => {
    if (activeTab === "fim") {
      setFimPage(1);
      fetchFimFiles(1, fimSearch, fimFilter, fimTypeFilter);
    }
  }, [fimFilter, fimTypeFilter]);

  useEffect(() => {
    if (activeTab === "fim") {
      fetchFimFiles(fimPage, fimSearch, fimFilter, fimTypeFilter);
    }
  }, [fimPage, fimTypeFilter]);

  useEffect(() => {
    if (activeTab === "orphan") {
      fetchOrphanFiles(orphanPage);
    }
  }, [orphanPage, activeTab]);

  // Fetch files when tab changes to fim
  useEffect(() => {
    if (activeTab === "fim") {
      fetchFimFiles(1, "", "all", "all");
      fetchFIMStats();
    }
  }, [activeTab]);

  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) {
      router.push("/login");
      return;
    }
    fetchAllData();
    const interval = setInterval(fetchAllData, 10000);
    return () => clearInterval(interval);
  }, [id, router]);

  const handleStartScan = async (mode: "now" | "later" = "now") => {
    const token = localStorage.getItem("ojs_token");
    setShowScanModal(false);
    setScanLoading(true);
    try {
      let url;
      if (mode === "later") {
        url = `http://localhost:8080/api/projects/${id}/integrity-scan?mode=later`;
      } else {
        // Force scan - cancels existing jobs and runs immediately
        url = `http://localhost:8080/api/projects/${id}/scan/force`;
      }
      const res = await fetch(url, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (res.ok) {
        alert(data.message || "Scan started!");
        fetchAllData();
      } else {
        alert("Scan failed: " + data.error);
      }
    } catch (err: any) {
      alert("Error: " + err.message);
    }
    setScanLoading(false);
  };

  const handleCancelJob = async (jobId: number) => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    if (!confirm("Are you sure you want to cancel this queued job?")) return;

    try {
      const res = await fetch(`http://localhost:8080/api/jobs/${jobId}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success) {
        fetchAllData();
      } else {
        alert("Failed to cancel: " + data.error);
      }
    } catch (err: any) {
      alert("Error: " + err.message);
    }
  };

  if (loading && !project) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
      </div>
    );
  }

  if (!project) {
    return (
      <div className="flex items-center justify-center h-64 text-red-400">
        Project not found
      </div>
    );
  }

  const isScanning = ["queued", "counting", "scanning", "reconciling"].includes(project.status);

  // Determine health status
  const getHealthStatus = () => {
    if (!metrics || project.status !== "active") return { status: "unknown", color: "text-slate-400" };
    // Critical: uploads by new users (suspicious activity)
    if (metrics.uploads_by_new_users > 0)
      return { status: "critical", color: "text-red-400" };
    // Attention: new users, file changes, or insecure self-registration settings
    if (metrics.new_users > 0 || metrics.new_files_count > 0 || metrics.modified_files_count > 0 || metrics.bad_self_reg > 0)
      return { status: "attention", color: "text-amber-400" };
    return { status: "healthy", color: "text-emerald-400" };
  };

  const health = getHealthStatus();

  return (
    <ProtectedLayout>
      {/* Page Header */}
      <div className="mb-6">
        {/* Breadcrumb */}
        <div className="flex items-center gap-2 mb-4">
          <Link
            href="/"
            className="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <span className="text-slate-600">/</span>
          <span className="text-slate-400 text-sm">Projects</span>
          <span className="text-slate-600">/</span>
          <span className="text-slate-200 text-sm font-medium">{project.name}</span>
        </div>

        {/* Header Content */}
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-2xl font-bold text-slate-100">{project.name}</h1>
              <StatusBadge status={project.status} />
            </div>
            <p className="text-slate-400 text-sm">
              {project.description || "No description"} • {project.template}
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Link
              href={`/projects/${id}/config`}
              className="flex items-center gap-2 px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-medium transition-colors border border-slate-700"
            >
              <Settings className="w-4 h-4" />
              Configure
            </Link>
            <button
              disabled={isScanning || scanLoading || project.status === "unconfigured"}
              onClick={() => setShowScanModal(true)}
              className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 px-4 py-2 rounded-xl text-white text-sm font-medium transition-colors"
            >
              {isScanning || scanLoading ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <RefreshCw className="w-4 h-4" />
              )}
              {isScanning ? "Scanning..." : "Start Scan"}
            </button>
          </div>
        </div>
      </div>

      {/* Progress Bar */}
      {isScanning && (
        <div className="glass-panel rounded-xl p-4 mb-6 border border-blue-500/30">
          <div className="flex justify-between text-sm mb-2">
            <span className="text-blue-400 font-medium capitalize">
              {project.status.replace("_", " ")}
            </span>
            <span className="text-slate-400">
              {project.baseline_processed.toLocaleString()} / {project.baseline_total.toLocaleString()} files
            </span>
          </div>
          <div className="w-full bg-slate-800 rounded-full h-2 overflow-hidden">
            <div
              className="h-full bg-blue-500 rounded-full transition-all"
              style={{
                width: `${(project.baseline_processed / Math.max(project.baseline_total, 1)) * 100}%`,
              }}
            />
          </div>
        </div>
      )}

      {/* Tab Navigation */}
      <div className="flex gap-1 border-b border-slate-700 mb-6">
        {[
          { id: "overview", label: "Overview", icon: <Shield className="w-4 h-4" />, href: `/projects/${id}` },
          { id: "fim", label: "FIM", icon: <FileWarning className="w-4 h-4" />, href: `/projects/${id}/fim` },
          { id: "orphan", label: "Orphans", icon: <AlertTriangle className="w-4 h-4" />, danger: orphanFiles.length > 0, href: `/projects/${id}?tab=orphan` },
          { id: "db", label: "Database", icon: <Database className="w-4 h-4" />, href: `/projects/${id}/database` },
          { id: "compliance", label: "Compliance", icon: <ShieldCheck className="w-4 h-4" />, href: `/projects/${id}/compliance` },
          { id: "alerts", label: "Alerts", icon: <Bell className="w-4 h-4" />, href: `/projects/${id}/alerts` },
          { id: "jobs", label: "Jobs", icon: <Terminal className="w-4 h-4" />, href: `/projects/${id}/jobs` },
        ].map((tab) => (
          <Link
            key={tab.id}
            href={tab.href}
            className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? "border-blue-500 text-blue-400"
                : "border-transparent text-slate-400 hover:text-slate-200"
            }`}
          >
            <span className={tab.danger ? "text-red-400" : ""}>{tab.icon}</span>
            {tab.label}
            {tab.danger && (
              <span className="px-1.5 py-0.5 bg-red-500/20 text-red-400 rounded text-xs font-bold">
                {orphanFiles.length}
              </span>
            )}
          </Link>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === "overview" && (
        <div className="space-y-6">
          {/* Health Status Banner */}
          {project.status === "active" && metrics && (
            <div
              className={`glass-panel rounded-xl p-5 border ${
                health.status === "critical"
                  ? "border-red-500/30 bg-red-500/5"
                  : health.status === "attention"
                  ? "border-amber-500/30 bg-amber-500/5"
                  : "border-emerald-500/30 bg-emerald-500/5"
              }`}
            >
              <div className="flex items-center gap-4">
                {health.status === "critical" && <ShieldAlert className="w-8 h-8 text-red-400" />}
                {health.status === "attention" && <AlertTriangle className="w-8 h-8 text-amber-400" />}
                {health.status === "healthy" && <ShieldCheck className="w-8 h-8 text-emerald-400" />}
                <div>
                  <h3 className={`text-lg font-semibold ${health.color}`}>
                    System {health.status.charAt(0).toUpperCase() + health.status.slice(1)}
                  </h3>
                  <p className="text-slate-400 text-sm">
                    {health.status === "critical" && "Critical issues detected. Immediate attention required."}
                    {health.status === "attention" && "Some changes detected. Review recommended."}
                    {health.status === "healthy" && "All systems operational. No issues detected."}
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Metrics Grid */}
          {metrics && project.status === "active" && (
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
              <MetricCard
                title="New Files"
                value={metrics.new_files_count}
                icon={<FileWarning className="w-5 h-5" />}
                variant={metrics.new_files_count > 0 ? "warning" : "default"}
              />
              <MetricCard
                title="Modified Files"
                value={metrics.modified_files_count}
                icon={<Activity className="w-5 h-5" />}
                variant={metrics.modified_files_count > 0 ? "warning" : "default"}
              />
              <MetricCard
                title="Orphan Files"
                value={metrics.orphan_files_count}
                icon={<AlertTriangle className="w-5 h-5" />}
                variant={metrics.orphan_files_count > 0 ? "danger" : "success"}
              />
              <MetricCard
                title="New Users (24h)"
                value={metrics.new_users}
                icon={<Users className="w-5 h-5" />}
                variant={metrics.new_users > 0 ? "warning" : "default"}
              />
              <MetricCard
                title="Active Admins"
                value={metrics.active_admins}
                icon={<Shield className="w-5 h-5" />}
                variant={metrics.active_admins !== 1 ? "danger" : "success"}
              />
              <MetricCard
                title="Vulnerable Reg"
                value={metrics.bad_self_reg}
                icon={<XCircle className="w-5 h-5" />}
                variant={metrics.bad_self_reg > 0 ? "danger" : "success"}
              />
              <MetricCard
                title="Uploads by New Users"
                value={metrics.uploads_by_new_users}
                icon={<Upload className="w-5 h-5" />}
                variant={metrics.uploads_by_new_users > 0 ? "danger" : "default"}
              />
              <MetricCard
                title="Baseline Files"
                value={metrics.baseline_total}
                icon={<Database className="w-5 h-5" />}
              />
            </div>
          )}

          {/* OJS Details */}
          {ojsDetails && (
            <div className="glass-panel rounded-xl p-5">
              <div className="flex items-center gap-2 mb-4">
                <Globe className="w-5 h-5 text-blue-400" />
                <h3 className="text-sm font-semibold text-slate-300">OJS Instance Details</h3>
              </div>
              <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
                <div className="flex items-center gap-3 p-3 bg-slate-800/50 rounded-xl">
                  <div className="p-2 rounded-lg bg-blue-500/20">
                    <Server className="w-5 h-5 text-blue-400" />
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-100">{ojsDetails.version}</p>
                    <p className="text-xs text-slate-500">Version</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 p-3 bg-slate-800/50 rounded-xl">
                  <div className="p-2 rounded-lg bg-emerald-500/20">
                    <BookOpen className="w-5 h-5 text-emerald-400" />
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-100">{ojsDetails.journals}</p>
                    <p className="text-xs text-slate-500">Journals</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 p-3 bg-slate-800/50 rounded-xl">
                  <div className="p-2 rounded-lg bg-purple-500/20">
                    <Users className="w-5 h-5 text-purple-400" />
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-100">{ojsDetails.users.toLocaleString()}</p>
                    <p className="text-xs text-slate-500">Total Users</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 p-3 bg-slate-800/50 rounded-xl">
                  <div className="p-2 rounded-lg bg-amber-500/20">
                    <FileText className="w-5 h-5 text-amber-400" />
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-100">{ojsDetails.articles.toLocaleString()}</p>
                    <p className="text-xs text-slate-500">Published Articles</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 p-3 bg-slate-800/50 rounded-xl">
                  <div className="p-2 rounded-lg bg-red-500/20">
                    <UserCheck className="w-5 h-5 text-red-400" />
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-100">{ojsDetails.review_assignments}</p>
                    <p className="text-xs text-slate-500">Pending Reviews</p>
                  </div>
                </div>
              </div>
              <div className="mt-4 pt-4 border-t border-slate-700/50 grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-slate-500">Language</p>
                  <p className="text-slate-200 font-medium">{ojsDetails.primary_locale.toUpperCase()}</p>
                </div>
                <div>
                  <p className="text-slate-500">Installed Languages</p>
                  <p className="text-slate-200 font-medium">
                    {(() => {
                      try {
                        const locales = JSON.parse(ojsDetails.installed_locales);
                        return locales.join(", ").toUpperCase();
                      } catch {
                        return ojsDetails.installed_locales;
                      }
                    })()}
                  </p>
                </div>
                <div>
                  <p className="text-slate-500">Min Password Length</p>
                  <p className="text-slate-200 font-medium">{ojsDetails.min_password_len} characters</p>
                </div>
                <div>
                  <p className="text-slate-500">Total Submissions</p>
                  <p className="text-slate-200 font-medium">{ojsDetails.submissions.toLocaleString()}</p>
                </div>
              </div>
            </div>
          )}

          {/* Project Info */}
          <div className="glass-panel rounded-xl p-5">
            <h3 className="text-sm font-semibold text-slate-300 mb-4">Monitoring Configuration</h3>
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
              <div>
                <p className="text-xs text-slate-500 mb-1">Template</p>
                <p className="text-slate-200 font-medium">{project.template}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500 mb-1">Baseline</p>
                <p className="text-slate-200 font-medium">
                  {project.baseline_total > 0 ? project.baseline_total.toLocaleString() + " files" : "Not set"}
                </p>
              </div>
              <div>
                <p className="text-xs text-slate-500 mb-1">Status</p>
                <p className="text-slate-200 font-medium capitalize">{project.status.replace("_", " ")}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500 mb-1">Last Scan</p>
                <p className="text-slate-200 font-medium">
                  {jobs.length > 0 ? new Date(jobs[0].created_at).toLocaleDateString() : "Never"}
                </p>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === "fim" && (
        <div className="space-y-6">
          {/* Summary Cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="glass-panel rounded-xl p-4 border border-emerald-500/20 bg-emerald-500/5">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-emerald-500/20">
                  <FileWarning className="w-5 h-5 text-emerald-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-emerald-400">{metrics?.new_files_count || 0}</p>
                  <p className="text-xs text-emerald-400/70">New Files</p>
                </div>
              </div>
            </div>
            <div className="glass-panel rounded-xl p-4 border border-blue-500/20 bg-blue-500/5">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-blue-500/20">
                  <Activity className="w-5 h-5 text-blue-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-blue-400">{metrics?.modified_files_count || 0}</p>
                  <p className="text-xs text-blue-400/70">Modified</p>
                </div>
              </div>
            </div>
            <div className="glass-panel rounded-xl p-4 border border-red-500/20 bg-red-500/5">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-red-500/20">
                  <XCircle className="w-5 h-5 text-red-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-red-400">{metrics?.deleted_files_count || 0}</p>
                  <p className="text-xs text-red-400/70">Deleted</p>
                </div>
              </div>
            </div>
            <div className="glass-panel rounded-xl p-4 border border-amber-500/20 bg-amber-500/5">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-amber-500/20">
                  <ShieldAlert className="w-5 h-5 text-amber-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-amber-400">{metrics?.orphan_files_count || 0}</p>
                  <p className="text-xs text-amber-400/70">Orphan Files</p>
                </div>
              </div>
            </div>
          </div>

          {/* FIM Statistics by Time Period */}
          {fimStats && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              {/* Today */}
              <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-semibold text-slate-300">Today</h3>
                  <Clock className="w-4 h-4 text-slate-500" />
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-emerald-400">Added</span>
                    <span className="text-sm font-bold text-emerald-400">{fimStats.today.added}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-blue-400">Modified</span>
                    <span className="text-sm font-bold text-blue-400">{fimStats.today.modified}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-red-400">Deleted</span>
                    <span className="text-sm font-bold text-red-400">{fimStats.today.deleted}</span>
                  </div>
                  <div className="pt-2 border-t border-slate-700/50 flex justify-between items-center">
                    <span className="text-xs text-slate-400">Total</span>
                    <span className="text-sm font-bold text-slate-200">{fimStats.today.total}</span>
                  </div>
                </div>
              </div>

              {/* This Week */}
              <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-semibold text-slate-300">This Week</h3>
                  <Calendar className="w-4 h-4 text-slate-500" />
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-emerald-400">Added</span>
                    <span className="text-sm font-bold text-emerald-400">{fimStats.this_week.added}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-blue-400">Modified</span>
                    <span className="text-sm font-bold text-blue-400">{fimStats.this_week.modified}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-red-400">Deleted</span>
                    <span className="text-sm font-bold text-red-400">{fimStats.this_week.deleted}</span>
                  </div>
                  <div className="pt-2 border-t border-slate-700/50 flex justify-between items-center">
                    <span className="text-xs text-slate-400">Total</span>
                    <span className="text-sm font-bold text-slate-200">{fimStats.this_week.total}</span>
                  </div>
                </div>
              </div>

              {/* This Month */}
              <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-semibold text-slate-300">This Month</h3>
                  <TrendingUp className="w-4 h-4 text-slate-500" />
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-emerald-400">Added</span>
                    <span className="text-sm font-bold text-emerald-400">{fimStats.this_month.added}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-blue-400">Modified</span>
                    <span className="text-sm font-bold text-blue-400">{fimStats.this_month.modified}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-red-400">Deleted</span>
                    <span className="text-sm font-bold text-red-400">{fimStats.this_month.deleted}</span>
                  </div>
                  <div className="pt-2 border-t border-slate-700/50 flex justify-between items-center">
                    <span className="text-xs text-slate-400">Total</span>
                    <span className="text-sm font-bold text-slate-200">{fimStats.this_month.total}</span>
                  </div>
                </div>
              </div>

              {/* All Time */}
              <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-semibold text-slate-300">All Time</h3>
                  <Hash className="w-4 h-4 text-slate-500" />
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-emerald-400">Added</span>
                    <span className="text-sm font-bold text-emerald-400">{fimStats.all_time.added}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-blue-400">Modified</span>
                    <span className="text-sm font-bold text-blue-400">{fimStats.all_time.modified}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-red-400">Deleted</span>
                    <span className="text-sm font-bold text-red-400">{fimStats.all_time.deleted}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-xs text-amber-400">Orphan</span>
                    <span className="text-sm font-bold text-amber-400">{fimStats.all_time.orphan}</span>
                  </div>
                  <div className="pt-2 border-t border-slate-700/50 flex justify-between items-center">
                    <span className="text-xs text-slate-400">Total</span>
                    <span className="text-sm font-bold text-slate-200">{fimStats.all_time.total}</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Type Filter (Project vs Uploads) */}
          <div className="flex gap-2 flex-wrap items-center">
            <span className="text-xs text-slate-500 font-medium">Location:</span>
            <button
              onClick={() => setFimTypeFilter("all")}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                fimTypeFilter === "all" ? "bg-purple-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              All Files
            </button>
            <button
              onClick={() => setFimTypeFilter("project")}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                fimTypeFilter === "project" ? "bg-blue-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              <span className="flex items-center gap-1"><Folder className="w-3 h-3" /> Project Files</span>
            </button>
            <button
              onClick={() => setFimTypeFilter("uploads")}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                fimTypeFilter === "uploads" ? "bg-amber-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              <span className="flex items-center gap-1"><Upload className="w-3 h-3" /> Upload Files</span>
            </button>
          </div>

          {/* Status Filter Buttons */}
          <div className="flex gap-2 flex-wrap">
            <button
              onClick={() => setFimFilter("all")}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                fimFilter === "all" ? "bg-blue-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              All ({fimPagination.total})
            </button>
            <button
              onClick={() => setFimFilter("ADDED")}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                fimFilter === "ADDED" ? "bg-emerald-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              New
            </button>
            <button
              onClick={() => setFimFilter("MODIFIED")}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                fimFilter === "MODIFIED" ? "bg-blue-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              Modified
            </button>
            <button
              onClick={() => setFimFilter("DELETED")}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                fimFilter === "DELETED" ? "bg-red-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              Deleted
            </button>
            <button
              onClick={() => setFimFilter("ORPHAN")}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                fimFilter === "ORPHAN" ? "bg-amber-600 text-white" : "bg-slate-800 text-slate-400 hover:bg-slate-700"
              }`}
            >
              Orphan
            </button>
          </div>

          {/* Search */}
          <div className="relative">
            <input
              type="text"
              placeholder="Search by path..."
              value={fimSearch}
              onChange={(e) => setFimSearch(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 pl-10 text-slate-100 focus:border-blue-500 outline-none"
            />
            <Folder className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
          </div>

          {/* File List */}
          <div className="glass-panel rounded-xl overflow-hidden">
            <div className="p-4 border-b border-slate-700/50 flex items-center justify-between">
              <h3 className="font-semibold text-slate-200">File Changes</h3>
              <span className="text-sm text-slate-500">
                Page {fimPagination.page} of {fimPagination.total_pages || 1} ({fimPagination.total} total)
              </span>
            </div>
            {files.length === 0 ? (
              <div className="p-12 text-center">
                <CheckCircle className="w-12 h-12 text-emerald-400 mx-auto mb-4" />
                <p className="text-slate-400">No file changes detected</p>
              </div>
            ) : (
              <div className="divide-y divide-slate-700/50">
                {files.map((f) => (
                  <Link key={f.id} href={`/projects/${id}/files/${f.id}`}>
                    <div className="p-4 hover:bg-slate-800/30 transition-colors cursor-pointer">
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1 flex-wrap">
                            <span
                              className={`px-2 py-0.5 rounded text-xs font-bold ${
                                f.status === "ADDED"
                                  ? "bg-emerald-500/20 text-emerald-400"
                                  : f.status === "DELETED"
                                  ? "bg-red-500/20 text-red-400"
                                  : f.status === "ORPHAN"
                                  ? "bg-amber-500/20 text-amber-400"
                                  : "bg-blue-500/20 text-blue-400"
                              }`}
                            >
                              {f.status}
                            </span>
                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                              f.file_type === "uploads" ? "bg-amber-500/20 text-amber-400" : "bg-blue-500/20 text-blue-400"
                            }`}>
                              {f.file_type === "uploads" ? "Upload" : "Project"}
                            </span>
                            <span className="text-xs text-slate-500">
                              {f.created_at ? new Date(f.created_at).toLocaleString() : "N/A"}
                            </span>
                          </div>
                          <p className="text-slate-200 text-sm font-mono break-all leading-relaxed">
                            {f.path}
                          </p>
                        </div>
                        <div className="text-right flex-shrink-0">
                          <p className="text-xs text-slate-500 font-mono">
                            {f.hash ? f.hash.substring(0, 12) + "..." : "N/A"}
                          </p>
                        </div>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            )}

            {/* Pagination */}
            {fimPagination.total_pages > 1 && (
              <div className="p-4 border-t border-slate-700/50 flex items-center justify-between">
                <button
                  onClick={() => setFimPage(Math.max(1, fimPage - 1))}
                  disabled={fimPage <= 1}
                  className="px-4 py-2 rounded-lg text-sm font-medium bg-slate-800 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                >
                  Previous
                </button>
                <span className="text-sm text-slate-400">
                  Page {fimPage} of {fimPagination.total_pages}
                </span>
                <button
                  onClick={() => setFimPage(Math.min(fimPagination.total_pages, fimPage + 1))}
                  disabled={fimPage >= fimPagination.total_pages}
                  className="px-4 py-2 rounded-lg text-sm font-medium bg-slate-800 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                >
                  Next
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {activeTab === "orphan" && (
        <div className="space-y-4">
          <div className="glass-panel rounded-xl overflow-hidden border border-red-500/20">
            <div className="p-4 border-b border-slate-700/50 bg-red-500/5 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <AlertTriangle className="w-5 h-5 text-red-400" />
                <h3 className="font-semibold text-red-400">Orphan Files</h3>
              </div>
              <span className="text-sm text-slate-400">
                Page {orphanPagination.page} of {orphanPagination.total_pages || 1} ({orphanPagination.total} total)
              </span>
            </div>
            {orphanFiles.length === 0 ? (
              <div className="p-12 text-center">
                <ShieldCheck className="w-12 h-12 text-emerald-400 mx-auto mb-4" />
                <p className="text-slate-300 font-medium">No orphan files detected</p>
                <p className="text-slate-500 text-sm mt-1">All files are properly registered in the database</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead className="bg-slate-800/50">
                    <tr className="text-left text-xs text-slate-400">
                      <th className="p-3 font-medium">Path</th>
                      <th className="p-3 font-medium">Hash</th>
                      <th className="p-3 font-medium">Detected</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-700/50">
                    {orphanFiles.map((f) => (
                      <tr key={f.id} className="hover:bg-red-500/5">
                        <td className="p-3 text-red-300 text-sm font-mono max-w-xl truncate" title={f.path}>
                          {f.path}
                        </td>
                        <td className="p-3 text-slate-500 text-xs font-mono w-32 truncate">
                          {f.hash || "-"}
                        </td>
                        <td className="p-3 text-slate-500 text-xs whitespace-nowrap">
                          {new Date(f.created_at).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {orphanPagination.total_pages > 1 && (
              <div className="p-4 border-t border-slate-700/50 flex items-center justify-between">
                <button
                  onClick={() => setOrphanPage(Math.max(1, orphanPage - 1))}
                  disabled={orphanPage <= 1}
                  className="px-4 py-2 rounded-lg text-sm font-medium bg-slate-800 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                >
                  Previous
                </button>
                <span className="text-sm text-slate-400">
                  Page {orphanPage} of {orphanPagination.total_pages}
                </span>
                <button
                  onClick={() => setOrphanPage(Math.min(orphanPagination.total_pages, orphanPage + 1))}
                  disabled={orphanPage >= orphanPagination.total_pages}
                  className="px-4 py-2 rounded-lg text-sm font-medium bg-slate-800 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                >
                  Next
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {activeTab === "db" && metrics && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <MetricCard
            title="Active Admins"
            value={metrics.active_admins}
            icon={<Shield className="w-5 h-5" />}
            variant={metrics.active_admins !== 1 ? "danger" : "success"}
          />
          <MetricCard
            title="Self-Reg Vulnerable"
            value={metrics.bad_self_reg}
            icon={<XCircle className="w-5 h-5" />}
            variant={metrics.bad_self_reg > 0 ? "danger" : "success"}
          />
          <MetricCard
            title="Total Users"
            value={metrics.validated_users + metrics.new_users + metrics.unvalidated_disabled}
            icon={<Users className="w-5 h-5" />}
          />
          <MetricCard
            title="Validated Users"
            value={metrics.validated_users}
            icon={<CheckCircle className="w-5 h-5" />}
            variant="success"
          />
          <MetricCard
            title="Unvalidated (Can Login)"
            value={metrics.unvalidated_disabled}
            icon={<AlertTriangle className="w-5 h-5" />}
            variant={metrics.unvalidated_disabled > 0 ? "warning" : "default"}
          />
          <MetricCard
            title="Uploads by New Users"
            value={metrics.uploads_by_new_users}
            icon={<Upload className="w-5 h-5" />}
            variant={metrics.uploads_by_new_users > 0 ? "danger" : "default"}
          />
        </div>
      )}

      {activeTab === "jobs" && (
        <div className="glass-panel rounded-xl overflow-hidden">
          <div className="p-4 border-b border-slate-700/50">
            <h3 className="font-semibold text-slate-200">Scan History</h3>
          </div>
          {jobs.length === 0 ? (
            <div className="p-12 text-center">
              <Terminal className="w-12 h-12 text-slate-600 mx-auto mb-4" />
              <p className="text-slate-400">No scan history</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-slate-800/50">
                  <tr className="text-left text-xs text-slate-400">
                    <th className="p-3 font-medium">ID</th>
                    <th className="p-3 font-medium">Type</th>
                    <th className="p-3 font-medium">Status</th>
                    <th className="p-3 font-medium">Files</th>
                    <th className="p-3 font-medium">Created</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-700/50">
                  {jobs.map((j) => (
                    <tr key={j.id} className="hover:bg-slate-800/30">
                      <td className="p-3 text-slate-500 font-mono">#{j.id}</td>
                      <td className="p-3 text-slate-300 font-medium capitalize">{j.type.replace("_", " ")}</td>
                      <td className="p-3">
                        <span
                          className={`px-2 py-1 rounded text-xs font-bold ${
                            j.status === "completed"
                              ? "bg-emerald-500/20 text-emerald-400"
                              : j.status === "failed"
                              ? "bg-red-500/20 text-red-400"
                              : j.status === "running"
                              ? "bg-blue-500/20 text-blue-400"
                              : j.status === "queued"
                              ? "bg-amber-500/20 text-amber-400"
                              : "bg-slate-700 text-slate-400"
                          }`}
                        >
                          {j.status}
                        </span>
                      </td>
                      <td className="p-3 text-slate-400 text-sm">
                        {j.files_success > 0 && <span className="text-emerald-400">{j.files_success} OK</span>}
                        {j.files_skipped > 0 && <span className="text-amber-400 ml-2">{j.files_skipped} skip</span>}
                        {j.files_error > 0 && <span className="text-red-400 ml-2">{j.files_error} err</span>}
                      </td>
                      <td className="p-3 text-slate-500 text-xs whitespace-nowrap">
                        {new Date(j.created_at).toLocaleString()}
                      </td>
                      <td className="p-3">
                        {j.status === "queued" && (
                          <button
                            onClick={() => handleCancelJob(j.id)}
                            className="px-2 py-1 bg-red-500/20 hover:bg-red-500/30 text-red-400 text-xs font-medium rounded transition-colors"
                          >
                            Cancel
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Scan Modal */}
      {showScanModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
          <div className="glass-panel rounded-xl border border-blue-500/30 p-6 max-w-md mx-4">
            <div className="flex items-center gap-3 mb-4">
              <div className="p-2 rounded-full bg-blue-500/20">
                <RefreshCw className="w-6 h-6 text-blue-400" />
              </div>
              <h3 className="text-lg font-semibold text-white">Integrity Scan</h3>
            </div>
            <p className="text-slate-400 mb-4">
              Choose how to run the integrity scan:
            </p>
            <div className="space-y-3 mb-6">
              <button
                onClick={() => handleStartScan("now")}
                disabled={scanLoading}
                className="w-full p-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 hover:bg-emerald-500/20 transition text-left disabled:opacity-50"
              >
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-full bg-emerald-500/20">
                    <Play className="w-5 h-5 text-emerald-400" />
                  </div>
                  <div>
                    <p className="text-white font-medium">Run Now (Force)</p>
                    <p className="text-slate-400 text-sm">Start scan immediately</p>
                  </div>
                </div>
              </button>
              <button
                onClick={() => handleStartScan("later")}
                disabled={scanLoading}
                className="w-full p-4 rounded-lg border border-purple-500/30 bg-purple-500/10 hover:bg-purple-500/20 transition text-left disabled:opacity-50"
              >
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-full bg-purple-500/20">
                    <Calendar className="w-5 h-5 text-purple-400" />
                  </div>
                  <div>
                    <p className="text-white font-medium">Schedule for Later</p>
                    <p className="text-slate-400 text-sm">Add to queue, run at scheduled time</p>
                  </div>
                </div>
              </button>
            </div>
            <button
              onClick={() => setShowScanModal(false)}
              className="w-full px-4 py-2 rounded-lg font-medium bg-slate-700 text-slate-300 hover:bg-slate-600"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </ProtectedLayout>
  );
}
