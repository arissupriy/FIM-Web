"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  Database,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Users,
  Upload,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Activity,
  BookOpen,
  Globe,
  UserCheck,
  RefreshCw,
} from "lucide-react";
import Link from "next/link";
import ProtectedLayout from "@/components/ProtectedLayout";

interface DashboardMetrics {
  status: string;
  baseline_total: number;
  baseline_processed: number;
  exec_files_count: number;
  new_files_count: number;
  modified_files_count: number;
  deleted_files_count: number;
  orphan_files_count: number;
  new_users: number;
  validated_users: number;
  unvalidated_disabled: number;
  uploads_by_new_users: number;
  active_admins: number;
  bad_self_reg: number;
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

export default function DatabasePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null);
  const [ojsDetails, setOjsDetails] = useState<OJSDetails | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    const fetchData = async () => {
      try {
        const [mRes, dRes] = await Promise.all([
          fetch(`http://localhost:8080/api/audit/${id}`, {
            headers: { Authorization: `Bearer ${token}` },
          }),
          fetch(`http://localhost:8080/api/projects/${id}/details`, {
            headers: { Authorization: `Bearer ${token}` },
          }),
        ]);

        const mData = await mRes.json();
        const dData = await dRes.json();

        if (mData.success) setMetrics(mData.data);
        if (dData.success) setOjsDetails(dData.data);
      } catch (err) {
        console.error(err);
      }
      setLoading(false);
    };

    fetchData();
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [id]);

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
      <div className="p-6 max-w-6xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-4">
            <Link href={`/projects/${id}`} className="p-2 rounded-lg hover:bg-slate-800 transition">
              <ArrowLeft className="w-5 h-5 text-slate-400" />
            </Link>
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                <Database className="w-6 h-6 text-blue-500" />
                Database Audit
              </h1>
              <p className="text-slate-500 text-sm">OJS database metrics and security audit</p>
            </div>
          </div>
          <Link
            href={`/projects/${id}/fim`}
            className="px-4 py-2 rounded-lg font-medium bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition flex items-center gap-2"
          >
            <Activity className="w-4 h-4" />
            Go to FIM
          </Link>
        </div>

        {/* OJS Info */}
        {ojsDetails && (
          <div className="glass-panel rounded-xl border border-slate-700/50 p-6 mb-6">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <BookOpen className="w-5 h-5 text-blue-400" />
              OJS Instance Information
            </h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <p className="text-xs text-slate-500">Version</p>
                <p className="text-white font-medium">{ojsDetails.version}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Journals</p>
                <p className="text-white font-medium">{ojsDetails.journals}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Users</p>
                <p className="text-white font-medium">{ojsDetails.users}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Submissions</p>
                <p className="text-white font-medium">{ojsDetails.submissions}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Articles</p>
                <p className="text-white font-medium">{ojsDetails.articles}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Review Assignments</p>
                <p className="text-white font-medium">{ojsDetails.review_assignments}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Primary Locale</p>
                <p className="text-white font-medium">{ojsDetails.primary_locale}</p>
              </div>
              <div>
                <p className="text-xs text-slate-500">Min Password Length</p>
                <p className="text-white font-medium">{ojsDetails.min_password_len} chars</p>
              </div>
            </div>
          </div>
        )}

        {/* Security Metrics */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {/* Active Admins */}
          <div className={`glass-panel rounded-xl p-5 border ${
            metrics?.active_admins === 1 ? "border-emerald-500/30 bg-emerald-500/5" : "border-red-500/30 bg-red-500/5"
          }`}>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500 mb-1">Active Admins</p>
                <p className={`text-2xl font-bold ${metrics?.active_admins === 1 ? "text-emerald-400" : "text-red-400"}`}>
                  {metrics?.active_admins || 0}
                </p>
              </div>
              {metrics?.active_admins === 1 ? (
                <ShieldCheck className="w-8 h-8 text-emerald-400" />
              ) : (
                <ShieldAlert className="w-8 h-8 text-red-400" />
              )}
            </div>
            {metrics?.active_admins !== 1 && (
              <p className="text-xs text-red-400 mt-2">Warning: Expected exactly 1 admin</p>
            )}
          </div>

          {/* Self-Registration */}
          <div className={`glass-panel rounded-xl p-5 border ${
            (metrics?.bad_self_reg || 0) === 0 ? "border-emerald-500/30 bg-emerald-500/5" : "border-red-500/30 bg-red-500/5"
          }`}>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500 mb-1">Self-Reg Vulnerable</p>
                <p className={`text-2xl font-bold ${(metrics?.bad_self_reg || 0) === 0 ? "text-emerald-400" : "text-red-400"}`}>
                  {metrics?.bad_self_reg || 0}
                </p>
              </div>
              {(metrics?.bad_self_reg || 0) === 0 ? (
                <CheckCircle className="w-8 h-8 text-emerald-400" />
              ) : (
                <XCircle className="w-8 h-8 text-red-400" />
              )}
            </div>
            {(metrics?.bad_self_reg || 0) > 0 && (
              <p className="text-xs text-red-400 mt-2">Danger: Insecure self-registration enabled</p>
            )}
          </div>

          {/* Uploads by New Users */}
          <div className={`glass-panel rounded-xl p-5 border ${
            (metrics?.uploads_by_new_users || 0) === 0 ? "border-emerald-500/30 bg-emerald-500/5" : "border-amber-500/30 bg-amber-500/5"
          }`}>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500 mb-1">Uploads by New Users</p>
                <p className={`text-2xl font-bold ${(metrics?.uploads_by_new_users || 0) === 0 ? "text-emerald-400" : "text-amber-400"}`}>
                  {metrics?.uploads_by_new_users || 0}
                </p>
              </div>
              <Upload className="w-8 h-8 text-amber-400" />
            </div>
          </div>

          {/* Total Users */}
          <div className="glass-panel rounded-xl p-5 border border-slate-700/50">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500 mb-1">Total Users</p>
                <p className="text-2xl font-bold text-white">
                  {(metrics?.validated_users || 0) + (metrics?.new_users || 0) + (metrics?.unvalidated_disabled || 0)}
                </p>
              </div>
              <Users className="w-8 h-8 text-blue-400" />
            </div>
          </div>

          {/* Validated Users */}
          <div className="glass-panel rounded-xl p-5 border border-emerald-500/30">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500 mb-1">Validated Users</p>
                <p className="text-2xl font-bold text-emerald-400">{metrics?.validated_users || 0}</p>
              </div>
              <CheckCircle className="w-8 h-8 text-emerald-400" />
            </div>
          </div>

          {/* Unvalidated Users */}
          <div className={`glass-panel rounded-xl p-5 border ${
            (metrics?.unvalidated_disabled || 0) === 0 ? "border-emerald-500/30" : "border-amber-500/30"
          }`}>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500 mb-1">Unvalidated (Can Login)</p>
                <p className={`text-2xl font-bold ${(metrics?.unvalidated_disabled || 0) === 0 ? "text-emerald-400" : "text-amber-400"}`}>
                  {metrics?.unvalidated_disabled || 0}
                </p>
              </div>
              <AlertTriangle className="w-8 h-8 text-amber-400" />
            </div>
          </div>
        </div>

        {/* New Users Alert */}
        {(metrics?.new_users || 0) > 0 && (
          <div className="glass-panel rounded-xl border border-amber-500/30 bg-amber-500/5 p-4 mt-6">
            <div className="flex items-center gap-3">
              <AlertTriangle className="w-5 h-5 text-amber-400" />
              <div>
                <p className="text-amber-400 font-medium">{metrics?.new_users} new users registered (24h)</p>
                <p className="text-slate-400 text-sm">Review if these registrations are expected</p>
              </div>
            </div>
          </div>
        )}
      </div>
    </ProtectedLayout>
  );
}
