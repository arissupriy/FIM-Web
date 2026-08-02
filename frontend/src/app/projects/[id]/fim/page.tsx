"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  Shield,
  Activity,
  File,
  Play,
  Square,
  RefreshCw,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Clock,
  Filter,
  Search,
  Eye,
  FileWarning,
  FileCheck,
  FileText,
  Zap,
  Pause,
  Database,
  Settings,
  GitBranch,
  Plus,
  Trash2,
  Edit,
  Hash,
  Calendar,
  ShieldAlert,
  ShieldCheck,
  Layers,
  EyeOff,
  ChevronRight,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import ProtectedLayout from "@/components/ProtectedLayout";

interface Project {
  id: number;
  name: string;
  status: string;
  baseline_at: number;
  baseline_total: number;
  watcher_status: string;
  integrity_scan_enabled: number;
  last_integrity_scan: number;
}

interface ProjectFile {
  id: number;
  file_path: string;
  hash: string;
  file_size: number;
  mod_time: number;
  status: string;
  file_type: string;
  created_at: string;
  updated_at: string;
}

interface FIMEvent {
  id: number;
  project_id: number;
  file_id: number | null;
  event_type: string;
  file_path: string;
  file_hash: string;
  actor_type: string;
  actor_name: string;
  actor_details: string;
  risk_level: string;
  classification: string;
  source: string;
  details: string;
  alert_sent: boolean;
  timestamp: string;
}

interface FIMStats {
  all_time: { events: number; high_risk: number; critical_risk: number; unknown_source: number; alerts_sent: number };
  this_month: { events: number; high_risk: number; critical_risk: number; unknown_source: number; alerts_sent: number };
  this_week: { events: number; high_risk: number; critical_risk: number; unknown_source: number; alerts_sent: number };
  today: { events: number; high_risk: number; critical_risk: number; unknown_source: number; alerts_sent: number };
}

interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

interface FileStats {
  baseline: number;
  current: number;
  added: number;
  modified: number;
  deleted: number;
  unknown: number;
}

type TabType = "baseline" | "events" | "orphans" | "settings";

export default function FIMPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [activeTab, setActiveTab] = useState<TabType>("baseline");
  const [loading, setLoading] = useState(true);
  const [watcherRunning, setWatcherRunning] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [showResetModal, setShowResetModal] = useState(false);
  const [showScanModal, setShowScanModal] = useState(false);

  // Baseline Files state
  const [files, setFiles] = useState<ProjectFile[]>([]);
  const [filePagination, setFilePagination] = useState<PaginationInfo>({ page: 1, limit: 50, total: 0, total_pages: 0 });
  const [fileSearch, setFileSearch] = useState("");
  const [fileStatusFilter, setFileStatusFilter] = useState("all");
  const [fileTypeFilter, setFileTypeFilter] = useState("all");

  // FIM Events state
  const [events, setEvents] = useState<FIMEvent[]>([]);
  const [eventPagination, setEventPagination] = useState<PaginationInfo>({ page: 1, limit: 50, total: 0, total_pages: 0 });
  const [eventSearch, setEventSearch] = useState("");
  const [eventTypeFilter, setEventTypeFilter] = useState("all");
  const [eventRiskFilter, setEventRiskFilter] = useState("all");

  // Stats
  const [stats, setStats] = useState<FIMStats | null>(null);
  const [fileStats, setFileStats] = useState<FileStats | null>(null);

  // Fetch project info
  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;
    fetch(`http://localhost:8080/api/projects/${id}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        if (data.success) {
          setProject(data.data);
          setWatcherRunning(data.data.watcher_status === "running");
        }
      });
  }, [id]);

  // Fetch stats
  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;
    fetch(`http://localhost:8080/api/projects/${id}/events/stats`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        if (data.success) setStats(data.data);
      });

    // Fetch file stats
    fetch(`http://localhost:8080/api/projects/${id}/files/stats`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        if (data.success) setFileStats(data.data);
      });
    setLoading(false);
  }, [id]);

  // Fetch baseline files
  useEffect(() => {
    if (activeTab !== "baseline") return;
    const token = localStorage.getItem("ojs_token");
    if (!token) return;
    const params = new URLSearchParams({ page: "1", limit: "50" });
    if (fileSearch) params.set("search", fileSearch);
    if (fileStatusFilter !== "all") params.set("status", fileStatusFilter);
    if (fileTypeFilter !== "all") params.set("type", fileTypeFilter);
    fetch(`http://localhost:8080/api/projects/${id}/files?${params}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        if (data.data?.files) {
          setFiles(data.data.files);
          setFilePagination(data.data.pagination);
        }
      });
  }, [id, activeTab, fileSearch, fileStatusFilter, fileTypeFilter]);

  // Fetch FIM events
  useEffect(() => {
    if (activeTab !== "events") return;
    const token = localStorage.getItem("ojs_token");
    if (!token) return;
    const params = new URLSearchParams({ page: "1", limit: "50" });
    if (eventSearch) params.set("search", eventSearch);
    if (eventTypeFilter !== "all") params.set("type", eventTypeFilter);
    if (eventRiskFilter !== "all") params.set("risk_level", eventRiskFilter);
    fetch(`http://localhost:8080/api/projects/${id}/events?${params}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        if (data.data?.events) {
          setEvents(data.data.events);
          setEventPagination(data.data.pagination);
        }
      });
  }, [id, activeTab, eventSearch, eventTypeFilter, eventRiskFilter]);

  const toggleWatcher = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token || !project) return;
    setActionLoading(true);
    try {
      const endpoint = watcherRunning
        ? `http://localhost:8080/api/projects/${id}/watcher/stop`
        : `http://localhost:8080/api/projects/${id}/watcher/start`;
      const res = await fetch(endpoint, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        setWatcherRunning(!watcherRunning);
        // Refresh project
        const pRes = await fetch(`http://localhost:8080/api/projects/${id}`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        const pData = await pRes.json();
        if (pData.success) setProject(pData.data);
      }
    } catch (err) { console.error(err); }
    setActionLoading(false);
  };

  const runIntegrityScan = async (mode: "now" | "later" = "now") => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;
    setShowScanModal(false);
    setActionLoading(true);
    try {
      const url = mode === "later"
        ? `http://localhost:8080/api/projects/${id}/integrity-scan?mode=later`
        : `http://localhost:8080/api/projects/${id}/integrity-scan`;
      const res = await fetch(url, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (res.ok) {
        alert(data.message || "Integrity scan queued!");
      } else {
        alert(data.error || "Failed to start scan");
      }
    } catch (err) { console.error(err); }
    setActionLoading(false);
  };

  const handleResetBaseline = async () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;
    setActionLoading(true);
    try {
      const res = await fetch(`http://localhost:8080/api/projects/${id}/baseline/reset`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        setShowResetModal(false);
        alert("Baseline reset successfully!");
        // Refresh project
        const pRes = await fetch(`http://localhost:8080/api/projects/${id}`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        const pData = await pRes.json();
        if (pData.success) setProject(pData.data);
        // Refresh files and stats
        if (activeTab === "baseline") {
          setFilePagination({ page: 1, limit: 50, total: 0, total_pages: 0 });
        }
      }
    } catch (err) { console.error(err); }
    setActionLoading(false);
  };

  const formatDate = (timestamp: number | string | undefined | null) => {
    if (!timestamp) return "Never";
    // Handle both Unix timestamp (number) and ISO date string
    const date = typeof timestamp === 'string' ? new Date(timestamp) : new Date(timestamp * 1000);
    if (isNaN(date.getTime())) return "Unknown";
    return date.toLocaleDateString("id-ID", {
      day: "2-digit",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString("id-ID", {
      day: "2-digit",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "BASELINE": return <span className="px-2 py-0.5 text-xs rounded-full bg-slate-500/20 text-slate-400">BASELINE</span>;
      case "ADDED": return <span className="px-2 py-0.5 text-xs rounded-full bg-emerald-500/20 text-emerald-400">ADDED</span>;
      case "MODIFIED": return <span className="px-2 py-0.5 text-xs rounded-full bg-amber-500/20 text-amber-400">MODIFIED</span>;
      case "DELETED": return <span className="px-2 py-0.5 text-xs rounded-full bg-red-500/20 text-red-400">DELETED</span>;
      default: return <span className="px-2 py-0.5 text-xs rounded-full bg-slate-500/20 text-slate-400">{status}</span>;
    }
  };

  const getEventIcon = (type: string) => {
    switch (type) {
      case "CREATED": return <div className="w-2 h-2 rounded-full bg-emerald-400"></div>;
      case "MODIFIED": return <div className="w-2 h-2 rounded-full bg-amber-400"></div>;
      case "DELETED": return <div className="w-2 h-2 rounded-full bg-red-400"></div>;
      default: return <div className="w-2 h-2 rounded-full bg-slate-400"></div>;
    }
  };

  const getRiskBadge = (level: string) => {
    switch (level) {
      case "LOW": return <span className="px-2 py-0.5 text-xs rounded-full bg-emerald-500/20 text-emerald-400">LOW</span>;
      case "MEDIUM": return <span className="px-2 py-0.5 text-xs rounded-full bg-amber-500/20 text-amber-400">MEDIUM</span>;
      case "HIGH": return <span className="px-2 py-0.5 text-xs rounded-full bg-orange-500/20 text-orange-400">HIGH</span>;
      case "CRITICAL": return <span className="px-2 py-0.5 text-xs rounded-full bg-red-500/20 text-red-400">CRITICAL</span>;
      default: return <span className="px-2 py-0.5 text-xs rounded-full bg-slate-500/20 text-slate-400">{level}</span>;
    }
  };

  const formatPath = (path: string | undefined | null) => {
    if (!path) return "Unknown";
    const parts = path.split("/");
    return parts.slice(-2).join("/");
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
  };

  if (loading) {
    return (
      <ProtectedLayout>
        <div className="flex items-center justify-center h-64">
          <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
        </div>
      </ProtectedLayout>
    );
  }

  return (
    <ProtectedLayout>
      <div className="p-6 max-w-7xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-4">
            <Link href={`/projects/${id}`} className="p-2 rounded-lg hover:bg-slate-800 transition">
              <ArrowLeft className="w-5 h-5 text-slate-400" />
            </Link>
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                <Shield className="w-6 h-6 text-blue-500" />
                File Integrity Monitor
              </h1>
              <p className="text-slate-500 text-sm">{project?.name}</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={toggleWatcher}
              disabled={actionLoading || !project?.baseline_at}
              className={`flex items-center gap-2 px-4 py-2 rounded-lg font-medium transition ${
                watcherRunning
                  ? "bg-red-500/20 text-red-400 hover:bg-red-500/30"
                  : "bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30"
              } disabled:opacity-50 disabled:cursor-not-allowed`}
            >
              {actionLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : watcherRunning ? <Square className="w-4 h-4" /> : <Play className="w-4 h-4" />}
              {watcherRunning ? "Stop" : "Start"} Watcher
            </button>
            <button
              onClick={() => setShowScanModal(true)}
              disabled={actionLoading || !project?.baseline_at}
              className="flex items-center gap-2 px-4 py-2 rounded-lg font-medium bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition disabled:opacity-50"
            >
              {actionLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
              Integrity Scan
            </button>
          </div>
        </div>

        {/* Stats Grid */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          {/* Baseline Info */}
          <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-lg bg-blue-500/10">
                <GitBranch className="w-5 h-5 text-blue-400" />
              </div>
              <div>
                <p className="text-xs text-slate-500">Baseline</p>
                <p className="text-xl font-bold text-white">{project?.baseline_total?.toLocaleString() || 0}</p>
                <p className="text-xs text-slate-500">{project?.baseline_at ? formatDate(project.baseline_at) : "Not set"}</p>
              </div>
            </div>
          </div>

          {/* Current State */}
          <div className="glass-panel rounded-xl p-4 border border-emerald-500/20">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-lg bg-emerald-500/10">
                <File className="w-5 h-5 text-emerald-400" />
              </div>
              <div>
                <p className="text-xs text-slate-500">Tracked Files</p>
                <p className="text-xl font-bold text-emerald-400">{fileStats?.current?.toLocaleString() || 0}</p>
                <p className="text-xs text-emerald-400/70">
                  {fileStats && fileStats.added > 0 && `+${fileStats.added} added`}
                </p>
              </div>
            </div>
          </div>

          {/* Modified */}
          <div className="glass-panel rounded-xl p-4 border border-amber-500/20">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-lg bg-amber-500/10">
                <Edit className="w-5 h-5 text-amber-400" />
              </div>
              <div>
                <p className="text-xs text-slate-500">Modified</p>
                <p className="text-xl font-bold text-amber-400">{fileStats?.modified || 0}</p>
                <p className="text-xs text-slate-500">files changed</p>
              </div>
            </div>
          </div>

          {/* Unknown Source */}
          <div className="glass-panel rounded-xl p-4 border border-purple-500/20">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-lg bg-purple-500/10">
                <ShieldAlert className="w-5 h-5 text-purple-400" />
              </div>
              <div>
                <p className="text-xs text-slate-500">Unknown Source</p>
                <p className="text-xl font-bold text-purple-400">{stats?.all_time?.unknown_source || 0}</p>
                <p className="text-xs text-slate-500">needs review</p>
              </div>
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 border-b border-slate-700 mb-6">
          {[
            { id: "baseline", label: "Baseline Files", icon: <Database className="w-4 h-4" /> },
            { id: "events", label: "FIM Events", icon: <Activity className="w-4 h-4" /> },
            { id: "orphans", label: "Orphans", icon: <FileWarning className="w-4 h-4" />, badge: fileStats?.unknown || 0 },
            { id: "settings", label: "Settings", icon: <Settings className="w-4 h-4" /> },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as TabType)}
              className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.id
                  ? "border-blue-500 text-blue-400"
                  : "border-transparent text-slate-400 hover:text-slate-200"
              }`}
            >
              {tab.icon}
              {tab.label}
              {tab.badge !== undefined && tab.badge > 0 && (
                <span className="px-1.5 py-0.5 bg-red-500/20 text-red-400 rounded text-xs font-bold">
                  {tab.badge}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        {activeTab === "baseline" && (
          <div className="space-y-4">
            {/* Filters */}
            <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
              <div className="flex flex-wrap gap-4">
                <div className="flex-1 min-w-[200px]">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                    <input
                      type="text"
                      placeholder="Search files..."
                      value={fileSearch}
                      onChange={(e) => setFileSearch(e.target.value)}
                      className="w-full pl-10 pr-4 py-2 bg-slate-800/50 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-blue-500"
                    />
                  </div>
                </div>
                <select
                  value={fileStatusFilter}
                  onChange={(e) => setFileStatusFilter(e.target.value)}
                  className="px-3 py-2 bg-slate-800/50 border border-slate-700 rounded-lg text-white"
                >
                  <option value="all">All Status</option>
                  <option value="BASELINE">Baseline</option>
                  <option value="ADDED">Added</option>
                  <option value="MODIFIED">Modified</option>
                  <option value="DELETED">Deleted</option>
                </select>
                <select
                  value={fileTypeFilter}
                  onChange={(e) => setFileTypeFilter(e.target.value)}
                  className="px-3 py-2 bg-slate-800/50 border border-slate-700 rounded-lg text-white"
                >
                  <option value="all">All Types</option>
                  <option value="project">Project Files</option>
                  <option value="uploads">Upload Files</option>
                </select>
              </div>
            </div>

            {/* Files Table */}
            <div className="glass-panel rounded-xl border border-slate-700/50 overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead className="bg-slate-800/50">
                    <tr>
                      <th className="px-4 py-3 text-left text-xs font-medium text-slate-400">Status</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-slate-400">File Path</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-slate-400">Type</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-slate-400">Hash</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-slate-400">Modified</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-700/50">
                    {files.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="px-4 py-12 text-center text-slate-500">
                          <Database className="w-12 h-12 mx-auto mb-3 opacity-50" />
                          <p>No files found</p>
                          <p className="text-sm mt-1">Run baseline scan first</p>
                        </td>
                      </tr>
                    ) : (
                      files.map((file) => (
                        <tr
                          key={file.id}
                          className="hover:bg-slate-800/30 transition cursor-pointer"
                          onClick={() => router.push(`/projects/${id}/files/${file.id}`)}
                        >
                          <td className="px-4 py-3">{getStatusBadge(file.status)}</td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <span className="text-slate-300 font-mono text-sm truncate max-w-md">{formatPath(file.file_path)}</span>
                              <ChevronRight className="w-4 h-4 text-blue-400" />
                            </div>
                            <p className="text-slate-500 text-xs">{file.file_path}</p>
                          </td>
                          <td className="px-4 py-3">
                            <span className={`px-2 py-0.5 text-xs rounded-full ${
                              file.file_type === "uploads" ? "bg-blue-500/20 text-blue-400" : "bg-slate-500/20 text-slate-400"
                            }`}>
                              {file.file_type === "uploads" ? "Upload" : "Project"}
                            </span>
                          </td>
                          <td className="px-4 py-3">
                            <span className="text-slate-400 font-mono text-xs">{file.hash?.substring(0, 12) || "N/A"}...</span>
                          </td>
                          <td className="px-4 py-3">
                            <span className="text-slate-400 text-sm">{formatDate(file.mod_time)}</span>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
              {filePagination.total_pages > 1 && (
                <div className="px-4 py-3 bg-slate-800/50 border-t border-slate-700/50 flex justify-between">
                  <span className="text-sm text-slate-400">
                    Page {filePagination.page} of {filePagination.total_pages} ({filePagination.total} files)
                  </span>
                  <div className="flex gap-2">
                    <button
                      onClick={() => setFilePagination(p => ({ ...p, page: p.page - 1 }))}
                      disabled={filePagination.page <= 1}
                      className="px-3 py-1 rounded bg-slate-700 text-slate-300 disabled:opacity-50"
                    >
                      Prev
                    </button>
                    <button
                      onClick={() => setFilePagination(p => ({ ...p, page: p.page + 1 }))}
                      disabled={filePagination.page >= filePagination.total_pages}
                      className="px-3 py-1 rounded bg-slate-700 text-slate-300 disabled:opacity-50"
                    >
                      Next
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === "events" && (
          <div className="space-y-4">
            {/* Filters */}
            <div className="glass-panel rounded-xl p-4 border border-slate-700/50">
              <div className="flex flex-wrap gap-4">
                <div className="flex-1 min-w-[200px]">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                    <input
                      type="text"
                      placeholder="Search events..."
                      value={eventSearch}
                      onChange={(e) => setEventSearch(e.target.value)}
                      className="w-full pl-10 pr-4 py-2 bg-slate-800/50 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-blue-500"
                    />
                  </div>
                </div>
                <select
                  value={eventTypeFilter}
                  onChange={(e) => setEventTypeFilter(e.target.value)}
                  className="px-3 py-2 bg-slate-800/50 border border-slate-700 rounded-lg text-white"
                >
                  <option value="all">All Types</option>
                  <option value="CREATED">Created</option>
                  <option value="MODIFIED">Modified</option>
                  <option value="DELETED">Deleted</option>
                </select>
                <select
                  value={eventRiskFilter}
                  onChange={(e) => setEventRiskFilter(e.target.value)}
                  className="px-3 py-2 bg-slate-800/50 border border-slate-700 rounded-lg text-white"
                >
                  <option value="all">All Risk</option>
                  <option value="LOW">Low</option>
                  <option value="MEDIUM">Medium</option>
                  <option value="HIGH">High</option>
                  <option value="CRITICAL">Critical</option>
                </select>
              </div>
            </div>

            {/* Events Timeline */}
            <div className="glass-panel rounded-xl border border-slate-700/50 overflow-hidden">
              <div className="divide-y divide-slate-700/50">
                {events.length === 0 ? (
                  <div className="px-4 py-12 text-center text-slate-500">
                    <Activity className="w-12 h-12 mx-auto mb-3 opacity-50" />
                    <p>No events found</p>
                    <p className="text-sm mt-1">Start the watcher to monitor file changes</p>
                  </div>
                ) : (
                  events.map((event) => (
                    <div
                      key={event.id}
                      className={`px-4 py-3 hover:bg-slate-800/30 transition cursor-pointer ${
                        event.file_id ? "cursor-pointer" : "cursor-default"
                      }`}
                      onClick={() => event.file_id && router.push(`/projects/${id}/files/${event.file_id}`)}
                    >
                      <div className="flex items-start gap-4">
                        <div className="flex items-center gap-2 pt-1">
                          {getEventIcon(event.event_type)}
                          <span className={`text-xs font-medium ${
                            event.event_type === "CREATED" ? "text-emerald-400" :
                            event.event_type === "MODIFIED" ? "text-amber-400" : "text-red-400"
                          }`}>
                            {event.event_type}
                          </span>
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <p className={`font-mono text-sm truncate ${event.file_id ? "text-blue-400" : "text-slate-300"}`}>
                              {formatPath(event.file_path)}
                            </p>
                            {event.file_id && <ChevronRight className="w-4 h-4 text-blue-400 flex-shrink-0" />}
                          </div>
                          <p className="text-slate-500 text-xs truncate">{event.file_path}</p>
                        </div>
                        <div className="flex items-center gap-3 flex-shrink-0">
                          {getRiskBadge(event.risk_level)}
                          <span className={`px-2 py-0.5 text-xs rounded-full ${
                            event.source === "WATCHER" ? "bg-emerald-500/20 text-emerald-400" :
                            event.source === "INTEGRITY_SCAN" ? "bg-blue-500/20 text-blue-400" :
                            "bg-slate-500/20 text-slate-400"
                          }`}>
                            {event.source}
                          </span>
                          <span className="text-slate-500 text-xs whitespace-nowrap">
                            {formatTime(event.timestamp)}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
              {eventPagination.total_pages > 1 && (
                <div className="px-4 py-3 bg-slate-800/50 border-t border-slate-700/50 flex justify-between">
                  <span className="text-sm text-slate-400">
                    Page {eventPagination.page} of {eventPagination.total_pages} ({eventPagination.total} events)
                  </span>
                  <div className="flex gap-2">
                    <button
                      onClick={() => setEventPagination(p => ({ ...p, page: p.page - 1 }))}
                      disabled={eventPagination.page <= 1}
                      className="px-3 py-1 rounded bg-slate-700 text-slate-300 disabled:opacity-50"
                    >
                      Prev
                    </button>
                    <button
                      onClick={() => setEventPagination(p => ({ ...p, page: p.page + 1 }))}
                      disabled={eventPagination.page >= eventPagination.total_pages}
                      className="px-3 py-1 rounded bg-slate-700 text-slate-300 disabled:opacity-50"
                    >
                      Next
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === "orphans" && (
          <div className="glass-panel rounded-xl border border-slate-700/50 p-12 text-center">
            <FileWarning className="w-12 h-12 mx-auto mb-3 text-amber-400 opacity-50" />
            <p className="text-slate-400">Orphan files feature coming soon</p>
            <p className="text-sm text-slate-500 mt-1">Files not tracked in OJS database</p>
          </div>
        )}

        {activeTab === "settings" && (
          <div className="glass-panel rounded-xl border border-slate-700/50 p-6">
            <h3 className="text-lg font-semibold text-white mb-4">FIM Settings</h3>
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 bg-slate-800/50 rounded-lg">
                <div>
                  <p className="text-white font-medium">Real-time Watcher</p>
                  <p className="text-slate-500 text-sm">Monitor file changes using fsnotify</p>
                </div>
                <button
                  onClick={toggleWatcher}
                  disabled={!project?.baseline_at}
                  className={`px-4 py-2 rounded-lg font-medium ${
                    watcherRunning
                      ? "bg-red-500/20 text-red-400"
                      : "bg-emerald-500/20 text-emerald-400"
                  } disabled:opacity-50`}
                >
                  {watcherRunning ? "Running" : "Stopped"}
                </button>
              </div>
              <div className="flex items-center justify-between p-4 bg-slate-800/50 rounded-lg">
                <div>
                  <p className="text-white font-medium">Integrity Scan</p>
                  <p className="text-slate-500 text-sm">Compare current files against baseline</p>
                </div>
                <button
                  onClick={() => setShowScanModal(true)}
                  disabled={!project?.baseline_at}
                  className="px-4 py-2 rounded-lg font-medium bg-blue-500/20 text-blue-400 disabled:opacity-50"
                >
                  Run Scan
                </button>
              </div>
              <div className="p-4 bg-slate-800/50 rounded-lg">
                <p className="text-white font-medium mb-2">Last Baseline</p>
                <p className="text-slate-400">{project?.baseline_at ? formatDate(project.baseline_at) : "Not set"}</p>
                <p className="text-slate-500 text-sm mt-1">{project?.baseline_total?.toLocaleString() || 0} files in baseline</p>
              </div>
              <div className="flex items-center justify-between p-4 bg-red-500/5 border border-red-500/20 rounded-lg">
                <div>
                  <p className="text-white font-medium">Reset Baseline</p>
                  <p className="text-slate-500 text-sm">Clear all baseline data and events</p>
                </div>
                <button
                  onClick={() => setShowResetModal(true)}
                  disabled={!project?.baseline_at || watcherRunning}
                  className="px-4 py-2 rounded-lg font-medium bg-red-500/20 text-red-400 hover:bg-red-500/30 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Reset
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Reset Baseline Confirmation Modal */}
        {showResetModal && (
          <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
            <div className="glass-panel rounded-xl border border-red-500/30 p-6 max-w-md mx-4">
              <div className="flex items-center gap-3 mb-4">
                <div className="p-2 rounded-full bg-red-500/20">
                  <AlertTriangle className="w-6 h-6 text-red-400" />
                </div>
                <h3 className="text-lg font-semibold text-white">Reset Baseline?</h3>
              </div>
              <p className="text-slate-400 mb-4">
                This will permanently delete all baseline data and FIM events for this project.
                This action cannot be undone.
              </p>
              <div className="bg-red-500/10 rounded-lg p-3 mb-4">
                <p className="text-red-400 text-sm font-medium">Warning:</p>
                <ul className="text-slate-400 text-sm mt-1 space-y-1">
                  <li>• All {project?.baseline_total?.toLocaleString() || 0} baseline files will be deleted</li>
                  <li>• All FIM events will be cleared</li>
                  <li>• File tracking history will be lost</li>
                </ul>
              </div>
              <div className="flex gap-3">
                <button
                  onClick={() => setShowResetModal(false)}
                  className="flex-1 px-4 py-2 rounded-lg font-medium bg-slate-700 text-slate-300 hover:bg-slate-600"
                >
                  Cancel
                </button>
                <button
                  onClick={handleResetBaseline}
                  disabled={actionLoading}
                  className="flex-1 px-4 py-2 rounded-lg font-medium bg-red-500 text-white hover:bg-red-600 disabled:opacity-50"
                >
                  {actionLoading ? "Resetting..." : "Reset Baseline"}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Scan Modal */}
        <ScanModal
          show={showScanModal}
          onClose={() => setShowScanModal(false)}
          onRunNow={() => runIntegrityScan("now")}
          onSchedule={() => runIntegrityScan("later")}
          loading={actionLoading}
        />
      </div>
    </ProtectedLayout>
  );
}

// Scan Modal Component (defined outside main component)
function ScanModal({
  show,
  onClose,
  onRunNow,
  onSchedule,
  loading,
}: {
  show: boolean;
  onClose: () => void;
  onRunNow: () => void;
  onSchedule: () => void;
  loading: boolean;
}) {
  if (!show) return null;

  return (
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
            onClick={onRunNow}
            disabled={loading}
            className="w-full p-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 hover:bg-emerald-500/20 transition text-left disabled:opacity-50"
          >
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-full bg-emerald-500/20">
                <Play className="w-5 h-5 text-emerald-400" />
              </div>
              <div>
                <p className="text-white font-medium">Run Now (Force)</p>
                <p className="text-slate-400 text-sm">Start scan immediately, bypassing queue</p>
              </div>
            </div>
          </button>
          <button
            onClick={onSchedule}
            disabled={loading}
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
          onClick={onClose}
          className="w-full px-4 py-2 rounded-lg font-medium bg-slate-700 text-slate-300 hover:bg-slate-600"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
