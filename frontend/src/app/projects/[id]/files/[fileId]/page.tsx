"use client";

import { useEffect, useState, use } from "react";
import {
  ArrowLeft,
  Loader2,
  FileText,
  Clock,
  Hash,
  Folder,
  Database,
  User,
  BookOpen,
  Link as LinkIcon,
  AlertTriangle,
  CheckCircle,
  XCircle,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import ProtectedLayout from "@/components/ProtectedLayout";

interface FileDetail {
  id: number;
  path: string;
  hash: string;
  status: string;
  file_type: string;
  file_size: number;
  file_mod_time: string;
  created_at: string;
  updated_at: string;
}

interface FileHistory {
  id: number;
  hash: string;
  status: string;
  date: string;
}

interface OJSRelation {
  type: string;
  file_id: number;
  original_name: string;
  submission_id: number;
  article_title: string;
  author_name: string;
  uploader_user_id: number;
  uploader_name: string;
  uploader_email: string;
  file_type: string;
  date_uploaded: string;
  stage_id: number;
  review_round: number;
  revision: number;
}

export default function FileDetailPage({
  params,
}: {
  params: Promise<{ id: string; fileId: string }>;
}) {
  const router = useRouter();
  const { id, fileId } = use(params);

  const [loading, setLoading] = useState(true);
  const [file, setFile] = useState<FileDetail | null>(null);
  const [history, setHistory] = useState<FileHistory[]>([]);
  const [relations, setRelations] = useState<OJSRelation[]>([]);
  const [loadingRelations, setLoadingRelations] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("ojs_token");
    if (!token) {
      router.push("/login");
      return;
    }

    const fetchFileDetail = async () => {
      try {
        const res = await fetch(`http://localhost:8080/api/projects/${id}/files/${fileId}`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        const data = await res.json();
        if (data.success && data.data) {
          setFile(data.data.file);
          setHistory(data.data.history || []);
        }
      } catch (err) {
        console.error(err);
      }
      setLoading(false);
    };

    fetchFileDetail();
  }, [id, fileId, router]);

  const fetchRelations = async () => {
    if (!file || file.file_type !== "uploads") return;

    setLoadingRelations(true);
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${id}/files/${fileId}/ojs-relations`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success && data.data) {
        setRelations(data.data.relations || []);
      }
    } catch (err) {
      console.error(err);
    }
    setLoadingRelations(false);
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "ADDED":
        return "bg-emerald-500/20 text-emerald-400 border-emerald-500/30";
      case "MODIFIED":
        return "bg-blue-500/20 text-blue-400 border-blue-500/30";
      case "DELETED":
        return "bg-red-500/20 text-red-400 border-red-500/30";
      case "ORPHAN":
        return "bg-amber-500/20 text-amber-400 border-amber-500/30";
      default:
        return "bg-slate-500/20 text-slate-400 border-slate-500/30";
    }
  };

  const getStageName = (stageId: number) => {
    const stages: Record<number, string> = {
      1: "Submission",
      2: "Review",
      3: "Copyediting",
      4: "Production",
      5: "Proofing",
    };
    return stages[stageId] || `Stage ${stageId}`;
  };

  if (loading) {
    return (
      <ProtectedLayout>
        <div className="flex items-center justify-center h-64">
          <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
        </div>
      </ProtectedLayout>
    );
  }

  if (!file) {
    return (
      <ProtectedLayout>
        <div className="flex items-center justify-center h-64 text-red-400">
          File not found
        </div>
      </ProtectedLayout>
    );
  }

  return (
    <ProtectedLayout>
      <div className="max-w-5xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center gap-4">
          <Link
            href={`/projects/${id}`}
            className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
          >
            <ArrowLeft className="w-5 h-5 text-slate-400" />
          </Link>
          <div>
            <h1 className="text-xl font-bold text-slate-100">File Detail</h1>
            <p className="text-sm text-slate-500">File ID: {file.id}</p>
          </div>
        </div>

        {/* Status Badge */}
        <div className={`p-4 rounded-xl border ${getStatusColor(file.status)}`}>
          <div className="flex items-center gap-3">
            {file.status === "ADDED" && <CheckCircle className="w-6 h-6" />}
            {file.status === "MODIFIED" && <FileText className="w-6 h-6" />}
            {file.status === "DELETED" && <XCircle className="w-6 h-6" />}
            {file.status === "ORPHAN" && <AlertTriangle className="w-6 h-6" />}
            <div>
              <p className="font-bold text-lg">{file.status}</p>
              <p className="text-sm opacity-80">
                {file.status === "ADDED" && "File was added to the system"}
                {file.status === "MODIFIED" && "File content has changed"}
                {file.status === "DELETED" && "File was deleted from disk"}
                {file.status === "ORPHAN" && "File exists but not in OJS database"}
              </p>
            </div>
          </div>
        </div>

        {/* File Info */}
        <div className="glass-panel rounded-xl p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-4 flex items-center gap-2">
            <FileText className="w-5 h-5 text-blue-400" />
            File Information
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <p className="text-xs text-slate-500 mb-1">File Path</p>
              <p className="text-slate-200 font-mono text-sm break-all bg-slate-800/50 p-3 rounded-lg">
                {file.path}
              </p>
            </div>
            <div className="space-y-4">
              <div>
                <p className="text-xs text-slate-500 mb-1">File Type</p>
                <p className="text-slate-200 font-medium">
                  <span className={`px-2 py-1 rounded text-xs font-medium ${
                    file.file_type === "uploads"
                      ? "bg-amber-500/20 text-amber-400"
                      : "bg-blue-500/20 text-blue-400"
                  }`}>
                    {file.file_type === "uploads" ? "📁 Upload File" : "⚙️ Project File"}
                  </span>
                </p>
              </div>
              <div>
                <p className="text-xs text-slate-500 mb-1">File Size</p>
                <p className="text-slate-200 font-medium">{formatFileSize(file.file_size)}</p>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4 pt-4 border-t border-slate-700/50">
            <div>
              <p className="text-xs text-slate-500 mb-1 flex items-center gap-1">
                <Clock className="w-3 h-3" /> Last Modified on Disk
              </p>
              <p className="text-slate-200 font-medium">
                {new Date(file.file_mod_time).toLocaleString()}
              </p>
            </div>
            <div>
              <p className="text-xs text-slate-500 mb-1 flex items-center gap-1">
                <Clock className="w-3 h-3" /> First Seen in System
              </p>
              <p className="text-slate-200 font-medium">
                {new Date(file.created_at).toLocaleString()}
              </p>
            </div>
            <div>
              <p className="text-xs text-slate-500 mb-1 flex items-center gap-1">
                <Clock className="w-3 h-3" /> Last Updated in System
              </p>
              <p className="text-slate-200 font-medium">
                {new Date(file.updated_at).toLocaleString()}
              </p>
            </div>
            <div>
              <p className="text-xs text-slate-500 mb-1 flex items-center gap-1">
                <Hash className="w-3 h-3" /> File Hash (SHA256)
              </p>
              <p className="text-slate-200 font-mono text-xs break-all">
                {file.hash || "N/A (empty hash)"}
              </p>
            </div>
          </div>
        </div>

        {/* OJS Relations (for upload files) */}
        {file.file_type === "uploads" && (
          <div className="glass-panel rounded-xl p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-slate-200 flex items-center gap-2">
                <Database className="w-5 h-5 text-purple-400" />
                OJS Database Relations
              </h2>
              {!loadingRelations && relations.length === 0 && (
                <button
                  onClick={fetchRelations}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
                >
                  Check OJS Relations
                </button>
              )}
            </div>

            {loadingRelations ? (
              <div className="flex items-center gap-3 text-slate-400">
                <Loader2 className="w-5 h-5 animate-spin" />
                <span>Checking OJS database...</span>
              </div>
            ) : relations.length > 0 ? (
              <div className="space-y-4">
                {relations.map((rel, idx) => (
                  <div
                    key={idx}
                    className="bg-slate-800/50 rounded-xl p-4 border border-slate-700/50"
                  >
                    <div className="flex items-start gap-3">
                      <div className="p-2 rounded-lg bg-purple-500/20">
                        <LinkIcon className="w-5 h-5 text-purple-400" />
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <span className="px-2 py-0.5 rounded text-xs font-medium bg-purple-500/20 text-purple-400">
                            Submission File
                          </span>
                          <span className="text-xs text-slate-500">
                            File ID: {rel.file_id}
                          </span>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          <div>
                            <p className="text-xs text-slate-500">Original Name</p>
                            <p className="text-slate-200 font-mono text-sm">{rel.original_name}</p>
                          </div>
                          <div>
                            <p className="text-xs text-slate-500">Submission ID</p>
                            <p className="text-slate-200 font-medium">#{rel.submission_id}</p>
                          </div>
                          <div className="md:col-span-2">
                            <p className="text-xs text-slate-500">Article Title</p>
                            <p className="text-slate-200 font-medium">{rel.article_title}</p>
                          </div>
                          <div>
                            <p className="text-xs text-slate-500 flex items-center gap-1">
                              <User className="w-3 h-3" /> Uploader
                            </p>
                            <p className="text-slate-200">
                              {rel.uploader_name} ({rel.uploader_email})
                            </p>
                          </div>
                          <div>
                            <p className="text-xs text-slate-500">Workflow Stage</p>
                            <p className="text-slate-200">{getStageName(rel.stage_id)}</p>
                          </div>
                          <div>
                            <p className="text-xs text-slate-500">Date Uploaded</p>
                            <p className="text-slate-200">
                              {rel.date_uploaded ? new Date(rel.date_uploaded).toLocaleString() : "N/A"}
                            </p>
                          </div>
                          <div>
                            <p className="text-xs text-slate-500">Revision</p>
                            <p className="text-slate-200">#{rel.revision}</p>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8">
                <AlertTriangle className="w-10 h-10 text-amber-400 mx-auto mb-3" />
                <p className="text-slate-400">No OJS database records found for this file</p>
                <p className="text-slate-500 text-sm mt-1">
                  This file may not be registered in the OJS submissions database
                </p>
              </div>
            )}
          </div>
        )}

        {/* Change History */}
        <div className="glass-panel rounded-xl p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-4 flex items-center gap-2">
            <Clock className="w-5 h-5 text-amber-400" />
            Change History
          </h2>
          {history.length > 0 ? (
            <div className="space-y-3">
              {history.map((h, idx) => (
                <div
                  key={h.id}
                  className="flex items-center gap-4 p-3 bg-slate-800/30 rounded-lg"
                >
                  <div className={`w-2 h-2 rounded-full ${idx === 0 ? "bg-emerald-400" : "bg-slate-600"}`} />
                  <span className={`px-2 py-0.5 rounded text-xs font-bold ${getStatusColor(h.status)}`}>
                    {h.status}
                  </span>
                  <span className="text-slate-400 text-sm flex-1">{h.date}</span>
                  {h.hash && (
                    <span className="text-slate-500 font-mono text-xs">
                      {h.hash.substring(0, 16)}...
                    </span>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-slate-500 text-center py-4">No change history available</p>
          )}
        </div>
      </div>
    </ProtectedLayout>
  );
}
