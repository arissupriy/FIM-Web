"use client";

import { useEffect, useState } from "react";
import {
  Shield,
  ShieldAlert,
  ShieldCheck,
  Activity,
  Settings,
  Plus,
  FileWarning,
  Loader2,
  AlertTriangle,
  Database,
  Clock,
  Server,
  TrendingUp,
  Eye,
  CheckCircle,
  XCircle,
  RefreshCw,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import ProtectedLayout from "@/components/ProtectedLayout";

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

interface Project {
  id: number;
  name: string;
  description: string;
  template: string;
  status: string;
  baseline_total: number;
  baseline_processed: number;
  metrics?: Metrics;
}

// Stats Card Component
function StatsCard({
  title,
  value,
  icon: Icon,
  trend,
  trendUp,
  variant = "default",
}: {
  title: string;
  value: number | string;
  icon: React.ReactNode;
  trend?: string;
  trendUp?: boolean;
  variant?: "default" | "success" | "warning" | "danger";
}) {
  const variants = {
    default: "bg-slate-800/50 border-slate-700/50",
    success: "bg-emerald-500/5 border-emerald-500/20",
    warning: "bg-amber-500/5 border-amber-500/20",
    danger: "bg-red-500/5 border-red-500/20",
  };

  const iconVariants = {
    default: "text-blue-400",
    success: "text-emerald-400",
    warning: "text-amber-400",
    danger: "text-red-400",
  };

  return (
    <div
      className={`glass-panel rounded-xl p-5 border ${variants[variant]} relative overflow-hidden`}
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-slate-400 text-sm font-medium mb-1">{title}</p>
          <p className={`text-2xl font-bold ${variant === "danger" ? "text-red-400" : "text-slate-100"}`}>
            {value}
          </p>
          {trend && (
            <p className={`text-xs mt-1 flex items-center gap-1 ${trendUp ? "text-emerald-400" : "text-red-400"}`}>
              <TrendingUp className={`w-3 h-3 ${trendUp ? "" : "rotate-180"}`} />
              {trend}
            </p>
          )}
        </div>
        <div className={`p-3 rounded-lg bg-slate-800/50`}>
          <div className={iconVariants[variant]}>{Icon}</div>
        </div>
      </div>
    </div>
  );
}

// Project Status Badge
function StatusBadge({ status, metrics }: { status: string; metrics?: Metrics }) {
  const getStatus = () => {
    if (status === "unconfigured")
      return { label: "Unconfigured", className: "bg-slate-600/20 text-slate-400 border-slate-600/30" };
    if (status === "pending_baseline")
      return { label: "Pending", className: "bg-blue-500/20 text-blue-400 border-blue-500/30" };
    if (["queued", "counting", "scanning", "reconciling"].includes(status))
      return { label: "Scanning...", className: "bg-amber-500/20 text-amber-400 border-amber-500/30" };
    if (status === "error")
      return { label: "Error", className: "bg-red-500/20 text-red-400 border-red-500/30" };

    if (metrics) {
      if (metrics.orphan_files_count > 0 || metrics.active_admins !== 1 || metrics.bad_self_reg > 0 || metrics.uploads_by_new_users > 0)
        return { label: "Critical", className: "bg-red-500/20 text-red-400 border-red-500/30" };
      if (metrics.new_users > 0 || metrics.new_files_count > 0 || metrics.modified_files_count > 0)
        return { label: "Attention", className: "bg-amber-500/20 text-amber-400 border-amber-500/30" };
      return { label: "Healthy", className: "bg-emerald-500/20 text-emerald-400 border-emerald-500/30" };
    }
    return { label: "Unknown", className: "bg-slate-600/20 text-slate-400 border-slate-600/30" };
  };

  const { label, className } = getStatus();
  return (
    <span className={`px-2.5 py-1 rounded-full text-xs font-semibold border ${className}`}>
      {label}
    </span>
  );
}

// Mini Chart Component
function MiniChart({ data }: { data: number[] }) {
  const max = Math.max(...data, 1);
  return (
    <div className="flex items-end gap-0.5 h-8">
      {data.map((val, i) => (
        <div
          key={i}
          className="w-2 bg-blue-500/30 rounded-t"
          style={{ height: `${(val / max) * 100}%` }}
        />
      ))}
    </div>
  );
}

export default function Dashboard() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [newTemplate, setNewTemplate] = useState("OJS 3.x");
  const [creating, setCreating] = useState(false);

  const router = useRouter();

  const fetchProjects = () => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    fetch("http://localhost:8080/api/projects", {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        if (data && data.success && data.data) {
          setProjects(data.data);
        }
        setLoading(false);
      })
      .catch(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) {
      router.push("/login");
      return;
    }
    fetchProjects();
    const interval = setInterval(fetchProjects, 10000);
    return () => clearInterval(interval);
  }, [router]);

  const handleCreateProject = async (e: React.FormEvent) => {
    e.preventDefault();
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    setCreating(true);
    try {
      const res = await fetch("http://localhost:8080/api/projects", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name: newName, description: newDesc, template: newTemplate }),
      });
      const data = await res.json();
      if (data.success) {
        setShowModal(false);
        setNewName("");
        setNewDesc("");
        setNewTemplate("OJS 3.x");
        router.push("/projects/" + data.data.id + "/config");
      } else {
        alert("Gagal: " + data.error);
      }
    } catch {
      alert("Error: Gagal membuat proyek");
    }
    setCreating(false);
  };

  // Calculate aggregate stats
  const totalProjects = projects.length;
  const healthyProjects = projects.filter(p => {
    if (!p.metrics || p.status !== "active") return false;
    const m = p.metrics;
    return m.uploads_by_new_users === 0 && m.new_users === 0 && m.new_files_count === 0 && m.modified_files_count === 0;
  }).length;
  const criticalProjects = projects.filter(p => {
    if (!p.metrics || p.status !== "active") return false;
    const m = p.metrics;
    return m.uploads_by_new_users > 0; // Only uploads by new users is critical
  }).length;
  const totalOrphans = projects.reduce((sum, p) => sum + (p.metrics?.orphan_files_count || 0), 0);
  const totalAlerts = projects.reduce((sum, p) => {
    if (!p.metrics || p.status !== "active") return sum;
    const m = p.metrics;
    return sum + (m.new_files_count || 0) + (m.modified_files_count || 0) + (m.new_users || 0) + (m.bad_self_reg || 0);
  }, 0);

  return (
    <ProtectedLayout>
      {/* Page Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-100">Security Dashboard</h1>
          <p className="text-slate-400 text-sm mt-1">
            Real-time monitoring for OJS deployments
          </p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-4 py-2.5 rounded-xl font-semibold text-sm transition-all shadow-lg shadow-blue-500/20 hover:shadow-blue-500/30"
        >
          <Plus className="w-4 h-4" />
          Add Project
        </button>
      </div>

      {/* Stats Overview */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatsCard
          title="Total Projects"
          value={totalProjects}
          icon={<Server className="w-5 h-5" />}
          variant="default"
        />
        <StatsCard
          title="Healthy"
          value={healthyProjects}
          icon={<ShieldCheck className="w-5 h-5" />}
          variant="success"
        />
        <StatsCard
          title="Critical"
          value={criticalProjects}
          icon={<ShieldAlert className="w-5 h-5" />}
          variant="danger"
        />
        <StatsCard
          title="Total Alerts"
          value={totalAlerts}
          icon={<AlertTriangle className="w-5 h-5" />}
          variant="warning"
        />
      </div>

      {/* Main Content */}
      {loading ? (
        <div className="flex justify-center items-center h-64">
          <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
        </div>
      ) : projects.length === 0 ? (
        <div className="glass-panel rounded-2xl p-12 text-center border border-slate-700/50">
          <div className="w-16 h-16 bg-slate-800 rounded-2xl flex items-center justify-center mx-auto mb-6">
            <Shield className="w-8 h-8 text-slate-500" />
          </div>
          <h2 className="text-xl font-semibold text-slate-200 mb-2">
            No Projects Configured
          </h2>
          <p className="text-slate-500 mb-6 max-w-md mx-auto">
            Add your first OJS deployment to start monitoring file integrity and database activity.
          </p>
          <button
            onClick={() => setShowModal(true)}
            className="inline-flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white px-6 py-3 rounded-xl font-semibold transition-all"
          >
            <Plus className="w-4 h-4" />
            Add Your First Project
          </button>
        </div>
      ) : (
        <div className="space-y-6">
          {/* Section Header */}
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-200">Projects</h2>
            <div className="flex items-center gap-2 text-sm text-slate-400">
              <span className="flex items-center gap-1">
                <span className="w-2 h-2 rounded-full bg-emerald-500"></span>
                {healthyProjects} Healthy
              </span>
              <span className="flex items-center gap-1">
                <span className="w-2 h-2 rounded-full bg-red-500"></span>
                {criticalProjects} Critical
              </span>
            </div>
          </div>

          {/* Project Grid */}
          <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
            {projects.map((project) => (
              <Link
                href={`/projects/${project.id}`}
                key={project.id}
                className="group glass-panel rounded-xl p-5 border border-slate-700/50 hover:border-slate-600 transition-all hover:shadow-xl hover:shadow-slate-900/50"
              >
                {/* Project Header */}
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <h3 className="font-semibold text-slate-100 group-hover:text-blue-400 transition-colors">
                        {project.name}
                      </h3>
                    </div>
                    <p className="text-slate-500 text-sm line-clamp-1">
                      {project.description || "No description"}
                    </p>
                  </div>
                  <StatusBadge status={project.status} metrics={project.metrics} />
                </div>

                {/* Template Badge */}
                <div className="flex items-center gap-2 mb-4">
                  <span className="px-2 py-1 bg-slate-800/50 rounded-lg text-xs text-slate-400 font-medium">
                    {project.template}
                  </span>
                  {project.status !== "unconfigured" && (
                    <span className="px-2 py-1 bg-slate-800/50 rounded-lg text-xs text-slate-400">
                      {project.baseline_total > 0
                        ? `${project.baseline_total.toLocaleString()} files`
                        : "No baseline"}
                    </span>
                  )}
                </div>

                {/* Metrics Row */}
                {project.status === "active" && project.metrics && (
                  <div className="grid grid-cols-4 gap-2 p-3 bg-slate-800/30 rounded-xl mb-4">
                    <div className="text-center">
                      <div className={`text-lg font-bold ${project.metrics.new_files_count > 0 ? "text-amber-400" : "text-slate-400"}`}>
                        {project.metrics.new_files_count}
                      </div>
                      <div className="text-xs text-slate-500">New</div>
                    </div>
                    <div className="text-center">
                      <div className={`text-lg font-bold ${project.metrics.modified_files_count > 0 ? "text-amber-400" : "text-slate-400"}`}>
                        {project.metrics.modified_files_count}
                      </div>
                      <div className="text-xs text-slate-500">Modified</div>
                    </div>
                    <div className="text-center">
                      <div className={`text-lg font-bold ${project.metrics.orphan_files_count > 0 ? "text-red-400" : "text-slate-400"}`}>
                        {project.metrics.orphan_files_count}
                      </div>
                      <div className="text-xs text-slate-500">Orphans</div>
                    </div>
                    <div className="text-center">
                      <div className={`text-lg font-bold ${project.metrics.new_users > 0 ? "text-amber-400" : "text-slate-400"}`}>
                        {project.metrics.new_users}
                      </div>
                      <div className="text-xs text-slate-500">New Users</div>
                    </div>
                  </div>
                )}

                {/* Progress Bar for Scanning */}
                {["queued", "counting", "scanning", "reconciling"].includes(project.status) && (
                  <div className="mb-4">
                    <div className="flex justify-between text-xs text-slate-400 mb-2">
                      <span className="capitalize">{project.status.replace("_", " ")}</span>
                      <span>
                        {Math.round((project.baseline_processed / Math.max(project.baseline_total, 1)) * 100)}%
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

                {/* Footer */}
                <div className="flex items-center justify-between pt-3 border-t border-slate-700/50">
                  <div className="flex items-center gap-2">
                    <Settings className="w-4 h-4 text-slate-500" />
                    <span className="text-xs text-slate-500">Configure</span>
                  </div>
                  <span className="text-xs text-blue-400 group-hover:text-blue-300 font-medium flex items-center gap-1">
                    View Details
                    <Eye className="w-3 h-3" />
                  </span>
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Create Project Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-700 rounded-2xl w-full max-w-md shadow-2xl">
            {/* Modal Header */}
            <div className="p-6 border-b border-slate-700">
              <h2 className="text-lg font-semibold text-slate-100">Add New Project</h2>
              <p className="text-sm text-slate-400 mt-1">Configure a new OJS deployment for monitoring</p>
            </div>

            {/* Modal Body */}
            <form onSubmit={handleCreateProject} className="p-6 space-y-5">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Project Name
                </label>
                <input
                  required
                  autoFocus
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-all"
                  placeholder="e.g., Journal of Education"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Description
                </label>
                <textarea
                  required
                  rows={3}
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-all resize-none"
                  placeholder="Brief description of this OJS deployment..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Template
                </label>
                <select
                  value={newTemplate}
                  onChange={(e) => setNewTemplate(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all"
                >
                  <option value="OJS 3.x">OJS 3.x</option>
                  <option value="OJS 2.x">OJS 2.x</option>
                  <option value="Custom">Custom</option>
                </select>
              </div>

              {/* Modal Footer */}
              <div className="flex gap-3 pt-4">
                <button
                  type="button"
                  onClick={() => {
                    setShowModal(false);
                    setNewName("");
                    setNewDesc("");
                    setNewTemplate("OJS 3.x");
                  }}
                  className="flex-1 bg-slate-800 hover:bg-slate-700 text-slate-300 py-3 rounded-xl font-semibold transition-all border border-slate-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creating || !newName.trim()}
                  className="flex-1 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white py-3 rounded-xl font-semibold transition-all flex items-center justify-center gap-2"
                >
                  {creating ? (
                    <Loader2 className="w-5 h-5 animate-spin" />
                  ) : (
                    <>
                      <Plus className="w-4 h-4" />
                      Create Project
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </ProtectedLayout>
  );
}
