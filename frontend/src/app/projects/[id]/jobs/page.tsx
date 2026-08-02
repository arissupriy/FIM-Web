"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  Terminal,
  CheckCircle,
  XCircle,
  Clock,
  RefreshCw,
  Play,
  Calendar,
  Shield,
  ChevronRight,
} from "lucide-react";
import Link from "next/link";
import ProtectedLayout from "@/components/ProtectedLayout";

interface Job {
  id: number;
  project_id: number;
  type: string;
  status: string;
  files_total: number;
  files_processed: number;
  error: string;
  scheduled_at: number;
  created_at: string;
  started_at: string;
  finished_at: string;
}

export default function JobsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCancelModal, setShowCancelModal] = useState(false);
  const [selectedJob, setSelectedJob] = useState<Job | null>(null);
  const [cancelling, setCancelling] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) {
      setError("Not authenticated");
      setLoading(false);
      return;
    }

    fetch(`http://localhost:8080/api/projects/${id}/jobs`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        if (data.success && data.data) {
          // Handle nested structure: {data: {data: [...], success: true}}
          setJobs(data.data.data || data.data || []);
        } else {
          setError(data.error || "Failed to load jobs");
        }
      })
      .catch(err => {
        console.error(err);
        setError("Failed to connect to server");
      });
    setLoading(false);
  }, [id]);

  const handleCancelJob = async () => {
    if (!selectedJob) return;
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    setCancelling(true);
    try {
      const res = await fetch(`http://localhost:8080/api/jobs/${selectedJob.id}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success) {
        setShowCancelModal(false);
        setSelectedJob(null);
        // Refresh jobs
        const jobRes = await fetch(`http://localhost:8080/api/projects/${id}/jobs`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        const jobData = await jobRes.json();
        if (jobData.success) setJobs(jobData.data.data || jobData.data || []);
      } else {
        alert(data.error || "Failed to cancel job");
      }
    } catch (err) {
      console.error(err);
      alert("Failed to cancel job");
    }
    setCancelling(false);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "completed": return <CheckCircle className="w-5 h-5 text-emerald-400" />;
      case "failed": return <XCircle className="w-5 h-5 text-red-400" />;
      case "running": return <RefreshCw className="w-5 h-5 text-blue-400 animate-spin" />;
      case "queued": return <Play className="w-5 h-5 text-amber-400" />;
      case "scheduled": return <Calendar className="w-5 h-5 text-purple-400" />;
      case "cancelled": return <XCircle className="w-5 h-5 text-slate-400" />;
      default: return <Clock className="w-5 h-5 text-slate-400" />;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "completed": return "bg-emerald-500/20 text-emerald-400";
      case "failed": return "bg-red-500/20 text-red-400";
      case "running": return "bg-blue-500/20 text-blue-400";
      case "queued": return "bg-amber-500/20 text-amber-400";
      case "scheduled": return "bg-purple-500/20 text-purple-400";
      case "cancelled": return "bg-slate-500/20 text-slate-400";
      default: return "bg-slate-700 text-slate-400";
    }
  };

  const getStatusLabel = (status: string) => {
    switch (status) {
      case "completed": return "Completed";
      case "failed": return "Failed";
      case "running": return "Running";
      case "queued": return "Queued";
      case "scheduled": return "Scheduled";
      case "cancelled": return "Cancelled";
      default: return status;
    }
  };

  const formatScheduledTime = (timestamp: number) => {
    if (!timestamp) return null;
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const diff = date.getTime() - now.getTime();

    // If scheduled time is in the past, show "Overdue"
    if (diff < 0) {
      return { text: "Overdue", isPast: true };
    }

    // If less than 24 hours, show relative time
    if (diff < 24 * 60 * 60 * 1000) {
      const hours = Math.floor(diff / (60 * 60 * 1000));
      const minutes = Math.floor((diff % (60 * 60 * 1000)) / (60 * 1000));
      if (hours > 0) {
        return { text: `in ${hours}h ${minutes}m`, isPast: false };
      }
      return { text: `in ${minutes}m`, isPast: false };
    }

    // Show full date
    return {
      text: date.toLocaleString("id-ID", {
        day: "2-digit",
        month: "short",
        hour: "2-digit",
        minute: "2-digit",
      }),
      isPast: false,
    };
  };

  const getTypeLabel = (type: string) => {
    switch (type) {
      case "initial_baseline": return "Initial Baseline";
      case "integrity_scan": return "Integrity Scan";
      case "rescan": return "Rescan";
      default: return type.replace("_", " ");
    }
  };

  const formatDate = (dateStr: string) => {
    if (!dateStr) return "-";
    const date = new Date(dateStr);
    return date.toLocaleString("id-ID", {
      day: "2-digit",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getDuration = (started: string, finished: string) => {
    if (!started || !finished) return "-";
    const start = new Date(started).getTime();
    const end = new Date(finished).getTime();
    const diff = Math.round((end - start) / 1000);
    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.round(diff / 60)}m`;
    return `${Math.round(diff / 3600)}h ${Math.round((diff % 3600) / 60)}m`;
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

  if (error) {
    return (
      <ProtectedLayout>
        <div className="p-6 max-w-4xl mx-auto">
          <div className="flex items-center gap-4 mb-6">
            <Link href={`/projects/${id}`} className="p-2 rounded-lg hover:bg-slate-800 transition">
              <ArrowLeft className="w-5 h-5 text-slate-400" />
            </Link>
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                <Terminal className="w-6 h-6 text-blue-500" />
                Scan Jobs
              </h1>
            </div>
          </div>
          <div className="glass-panel rounded-xl p-8 text-center border border-red-500/30">
            <XCircle className="w-12 h-12 text-red-400 mx-auto mb-4" />
            <p className="text-red-400">{error}</p>
          </div>
        </div>
      </ProtectedLayout>
    );
  }

  return (
    <ProtectedLayout>
      <div className="p-6 max-w-6xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-4">
            <Link href={`/projects/${id}`} className="p-2 rounded-lg hover:bg-slate-800 transition">
              <ArrowLeft className="w-5 h-5 text-slate-400" />
            </Link>
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                <Terminal className="w-6 h-6 text-blue-500" />
                Scan Jobs
              </h1>
              <p className="text-slate-500 text-sm">Scan history and progress</p>
            </div>
          </div>
          <Link
            href={`/projects/${id}/fim`}
            className="px-4 py-2 rounded-lg font-medium bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition flex items-center gap-2"
          >
            <Shield className="w-4 h-4" />
            Go to FIM
          </Link>
        </div>

        {/* Jobs List */}
        {jobs.length === 0 ? (
          <div className="glass-panel rounded-xl border border-slate-700/50 p-12 text-center">
            <Terminal className="w-12 h-12 text-slate-600 mx-auto mb-4" />
            <p className="text-slate-400">No scan history</p>
            <p className="text-slate-500 text-sm mt-1">Run a baseline scan to start monitoring</p>
          </div>
        ) : (
          <div className="glass-panel rounded-xl border border-slate-700/50 overflow-hidden">
            <table className="w-full">
              <thead className="bg-slate-800/50">
                <tr className="text-left text-xs text-slate-400">
                  <th className="p-4 font-medium">Status</th>
                  <th className="p-4 font-medium">Type</th>
                  <th className="p-4 font-medium">Progress</th>
                  <th className="p-4 font-medium">Created</th>
                  <th className="p-4 font-medium">Duration</th>
                  <th className="p-4 font-medium"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-700/50">
                {jobs.map((job) => {
                  const scheduled = formatScheduledTime(job.scheduled_at);
                  return (
                    <tr key={job.id} className="hover:bg-slate-800/30 transition">
                      <td className="p-4">
                        <div className="flex items-center gap-2">
                          {getStatusIcon(job.status)}
                          <span className={`px-2 py-1 rounded text-xs font-bold capitalize ${getStatusBadge(job.status)}`}>
                            {getStatusLabel(job.status)}
                          </span>
                        </div>
                        {job.status === "scheduled" && scheduled && (
                          <div className={`text-xs mt-1 ${scheduled.isPast ? "text-red-400" : "text-purple-400"}`}>
                            {scheduled.isPast ? "⚠️" : "📅"} {scheduled.text}
                          </div>
                        )}
                      </td>
                      <td className="p-4">
                        <div className="flex items-center gap-2 text-slate-300">
                          <Shield className="w-4 h-4" />
                          <span>{getTypeLabel(job.type)}</span>
                        </div>
                      </td>
                      <td className="p-4">
                        {job.status === "running" ? (
                          <div className="flex items-center gap-3">
                            <div className="flex-1 h-2 bg-slate-700 rounded-full overflow-hidden max-w-[150px]">
                              <div
                                className="h-full bg-blue-500 transition-all animate-pulse"
                                style={{ width: job.files_total > 0 ? `${(job.files_processed / job.files_total) * 100}%` : "0%" }}
                              />
                            </div>
                            <span className="text-slate-400 text-sm">
                              {job.files_processed}/{job.files_total}
                            </span>
                          </div>
                        ) : job.status === "scheduled" ? (
                          <div className="flex items-center gap-2 text-purple-400">
                            <Calendar className="w-4 h-4" />
                            <span className="text-sm">Pending</span>
                          </div>
                        ) : (
                          <span className="text-slate-400 text-sm">
                            {job.files_processed || 0} files
                          </span>
                        )}
                      </td>
                      <td className="p-4">
                        <div className="text-slate-400 text-sm">
                          {job.status === "scheduled" ? (
                            <span className="text-purple-400">
                              Scheduled: {scheduled?.text}
                            </span>
                          ) : (
                            formatDate(job.created_at)
                          )}
                        </div>
                      </td>
                      <td className="p-4">
                        <div className="flex items-center gap-2">
                          <span className="text-slate-400 text-sm">
                            {job.status === "completed" || job.status === "failed"
                              ? getDuration(job.started_at, job.finished_at)
                              : "-"}
                          </span>
                          {(job.status === "queued" || job.status === "scheduled") && (
                            <button
                              onClick={() => { setSelectedJob(job); setShowCancelModal(true); }}
                              className="px-2 py-1 bg-red-500/20 hover:bg-red-500/30 text-red-400 text-xs font-medium rounded transition-colors ml-2"
                            >
                              Cancel
                            </button>
                          )}
                        </div>
                      </td>
                      <td className="p-4">
                        <ChevronRight className="w-4 h-4 text-slate-500" />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Cancel Confirmation Modal */}
        {showCancelModal && selectedJob && (
          <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
            <div className="glass-panel rounded-xl border border-red-500/30 p-6 max-w-md mx-4">
              <div className="flex items-center gap-3 mb-4">
                <div className="p-2 rounded-full bg-red-500/20">
                  <XCircle className="w-6 h-6 text-red-400" />
                </div>
                <h3 className="text-lg font-semibold text-white">Cancel Job?</h3>
              </div>
              <p className="text-slate-400 mb-4">
                Are you sure you want to cancel this job? This action cannot be undone.
              </p>
              <div className="bg-red-500/10 rounded-lg p-3 mb-4">
                <div className="flex items-center gap-2 text-slate-300">
                  <Shield className="w-4 h-4" />
                  <span className="font-medium">{getTypeLabel(selectedJob.type)}</span>
                </div>
                <p className="text-slate-500 text-sm mt-1">
                  Status: <span className="capitalize text-amber-400">{getStatusLabel(selectedJob.status)}</span>
                </p>
                {selectedJob.status === "scheduled" && (
                  <p className="text-slate-500 text-sm">
                    Scheduled: {formatScheduledTime(selectedJob.scheduled_at)?.text}
                  </p>
                )}
              </div>
              <div className="flex gap-3">
                <button
                  onClick={() => { setShowCancelModal(false); setSelectedJob(null); }}
                  className="flex-1 px-4 py-2 rounded-lg font-medium bg-slate-700 text-slate-300 hover:bg-slate-600"
                >
                  Keep Job
                </button>
                <button
                  onClick={handleCancelJob}
                  disabled={cancelling}
                  className="flex-1 px-4 py-2 rounded-lg font-medium bg-red-500 text-white hover:bg-red-600 disabled:opacity-50"
                >
                  {cancelling ? "Cancelling..." : "Cancel Job"}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </ProtectedLayout>
  );
}
