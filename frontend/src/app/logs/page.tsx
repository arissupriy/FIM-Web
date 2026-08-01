"use client";

import { useEffect, useState } from "react";
import { History, Loader2, User, LogIn, LogOut, Plus, Edit, Trash, Search, Shield, Zap } from "lucide-react";
import ProtectedLayout from "@/components/ProtectedLayout";

interface AuditLog {
  id: number;
  admin_id: number;
  admin_name: string;
  action: string;
  target: string;
  timestamp: string;
}

const actionConfig: Record<string, { icon: React.ReactNode; color: string; bg: string; label: string }> = {
  LOGIN: { icon: <LogIn className="w-4 h-4" />, color: "text-blue-400", bg: "bg-blue-500/10", label: "Login" },
  LOGOUT: { icon: <LogOut className="w-4 h-4" />, color: "text-slate-400", bg: "bg-slate-500/10", label: "Logout" },
  ADD_PROJECT: { icon: <Plus className="w-4 h-4" />, color: "text-emerald-400", bg: "bg-emerald-500/10", label: "Create" },
  UPDATE_PROJECT: { icon: <Edit className="w-4 h-4" />, color: "text-amber-400", bg: "bg-amber-500/10", label: "Update" },
  DELETE_PROJECT: { icon: <Trash className="w-4 h-4" />, color: "text-red-400", bg: "bg-red-500/10", label: "Delete" },
  START_SCAN: { icon: <Search className="w-4 h-4" />, color: "text-purple-400", bg: "bg-purple-500/10", label: "Scan" },
  AUTH_SUCCESS: { icon: <Shield className="w-4 h-4" />, color: "text-emerald-400", bg: "bg-emerald-500/10", label: "Auth" },
  AUTH_FAILED: { icon: <Zap className="w-4 h-4" />, color: "text-red-400", bg: "bg-red-500/10", label: "Failed" },
};

export default function Logs() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    fetch("http://localhost:8080/api/logs", {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        if (data.success && data.data) {
          setLogs(data.data);
        }
        setLoading(false);
      })
      .catch(() => {
        setLoading(false);
      });
  }, []);

  const filteredLogs = filter
    ? logs.filter(
        (log) =>
          log.admin_name.toLowerCase().includes(filter.toLowerCase()) ||
          log.action.toLowerCase().includes(filter.toLowerCase()) ||
          log.target.toLowerCase().includes(filter.toLowerCase())
      )
    : logs;

  // Group logs by date
  const groupedLogs = filteredLogs.reduce((groups, log) => {
    const date = new Date(log.timestamp).toLocaleDateString("en-US", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    });
    if (!groups[date]) {
      groups[date] = [];
    }
    groups[date].push(log);
    return groups;
  }, {} as Record<string, AuditLog[]>);

  return (
    <ProtectedLayout>
      {/* Page Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-100 flex items-center gap-3">
            <History className="w-7 h-7 text-blue-400" />
            Activity Logs
          </h1>
          <p className="text-slate-400 text-sm mt-1">
            Complete audit trail of all administrator actions
          </p>
        </div>
        <div className="flex items-center gap-2 px-3 py-1.5 bg-slate-800/50 rounded-lg">
          <span className="text-sm text-slate-400">{logs.length}</span>
          <span className="text-sm text-slate-500">total events</span>
        </div>
      </div>

      {/* Filter */}
      <div className="mb-6">
        <div className="relative max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-500" />
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Search logs by user, action, or target..."
            className="w-full bg-slate-800/50 border border-slate-700 rounded-xl py-2.5 pl-10 pr-4 text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-all"
          />
        </div>
      </div>

      {/* Logs List */}
      <div className="space-y-8">
        {loading ? (
          <div className="flex justify-center items-center h-64">
            <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
          </div>
        ) : filteredLogs.length === 0 ? (
          <div className="glass-panel rounded-xl p-12 text-center border border-slate-700/50">
            <History className="w-12 h-12 text-slate-600 mx-auto mb-4" />
            <p className="text-slate-400">
              {filter ? "No logs match your search criteria" : "No activity logs recorded yet"}
            </p>
          </div>
        ) : (
          Object.entries(groupedLogs).map(([date, dateLogs]) => (
            <div key={date}>
              {/* Date Header */}
              <div className="flex items-center gap-4 mb-4">
                <span className="text-sm font-medium text-slate-400">{date}</span>
                <div className="flex-1 h-px bg-slate-800" />
                <span className="text-xs text-slate-500">{dateLogs.length} events</span>
              </div>

              {/* Logs for this date */}
              <div className="space-y-2">
                {dateLogs.map((log) => {
                  const config = actionConfig[log.action] || {
                    icon: <Zap className="w-4 h-4" />,
                    color: "text-slate-400",
                    bg: "bg-slate-500/10",
                    label: log.action,
                  };

                  return (
                    <div
                      key={log.id}
                      className="glass-panel rounded-xl p-4 border border-slate-700/50 hover:border-slate-600 transition-all flex items-center gap-4"
                    >
                      {/* Action Icon */}
                      <div className={`p-2 rounded-lg ${config.bg} ${config.color}`}>
                        {config.icon}
                      </div>

                      {/* Content */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-3">
                          <div className="flex items-center gap-2">
                            <div className="w-6 h-6 bg-slate-700 rounded-full flex items-center justify-center">
                              <span className="text-slate-300 text-xs font-medium">
                                {log.admin_name.charAt(0).toUpperCase()}
                              </span>
                            </div>
                            <span className="font-medium text-slate-200">{log.admin_name}</span>
                          </div>
                          <span className={`px-2 py-0.5 rounded text-xs font-medium ${config.bg} ${config.color}`}>
                            {config.label}
                          </span>
                        </div>
                        <p className="text-slate-400 text-sm mt-1 truncate">{log.target}</p>
                      </div>

                      {/* Time */}
                      <div className="text-right flex-shrink-0">
                        <p className="text-slate-500 text-sm">
                          {new Date(log.timestamp).toLocaleTimeString("en-US", {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </p>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          ))
        )}
      </div>
    </ProtectedLayout>
  );
}
