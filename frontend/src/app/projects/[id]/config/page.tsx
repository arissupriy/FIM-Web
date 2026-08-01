"use client";

import { useState, useEffect, use } from "react";
import {
  Save,
  ArrowLeft,
  Loader2,
  TestTube,
  Database,
  Folder,
  Shield,
  AlertCircle,
  CheckCircle,
  XCircle,
  Server,
  Key,
  FileText,
  Eye,
  EyeOff,
  Clock,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import ProtectedLayout from "@/components/ProtectedLayout";

export default function ConfigPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const router = useRouter();
  const { id } = use(params);

  const [formData, setFormData] = useState({
    id: 0,
    name: "",
    description: "",
    template: "OJS 3.x",
    app_paths: [] as string[],
    files_paths: [] as string[],
    blacklist_exts: [] as string[],
    whitelist_paths: [] as string[],
    db_host: "",
    db_user: "",
    db_pass: "",
    db_name: "",
    rescan_interval: 10,
  });
  const [loading, setLoading] = useState(false);
  const [fetching, setFetching] = useState(true);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; text: string } | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [activeSection, setActiveSection] = useState<"general" | "database" | "paths" | "rules">("general");

  useEffect(() => {
    if (!id) return;

    const token = localStorage.getItem("ojs_token");
    if (!token) {
      router.push("/login");
      return;
    }

    fetch(`http://localhost:8080/api/projects/${id}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        if (data && data.success && data.data) {
          const project = data.data;
          setFormData({
            id: project.id,
            name: project.name || "",
            description: project.description || "",
            template: project.template || "OJS 3.x",
            app_paths: project.app_paths || [],
            files_paths: project.files_paths || [],
            blacklist_exts: project.blacklist_exts || [],
            whitelist_paths: project.whitelist_paths || [],
            db_host: project.db_host || "localhost:3306",
            db_user: project.db_user || "",
            db_pass: project.db_pass || "",
            db_name: project.db_name || "",
            rescan_interval: project.rescan_interval || 10,
          });
        } else {
          setMessage({ type: "error", text: "Project not found." });
        }
        setFetching(false);
      })
      .catch(() => {
        setMessage({ type: "error", text: "Failed to load project data." });
        setFetching(false);
      });
  }, [id, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const token = localStorage.getItem("ojs_token");
    if (!token) return;

    setLoading(true);
    setMessage(null);

    try {
      const res = await fetch(`http://localhost:8080/api/projects/${formData.id}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(formData),
      });

      const data = await res.json();
      if (data.success) {
        setMessage({ type: "success", text: "Settings saved successfully!" });
        setTimeout(() => router.push("/"), 1500);
      } else {
        setMessage({ type: "error", text: "Failed: " + data.error });
      }
    } catch {
      setMessage({ type: "error", text: "Error: Failed to save." });
    }
    setLoading(false);
  };

  const handleTestConnection = async () => {
    const token = localStorage.getItem("ojs_token");
    setTestLoading(true);
    setTestResult(null);

    try {
      const res = await fetch(
        `http://localhost:8080/api/projects/${formData.id}/test-connection`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(formData),
        }
      );
      const data = await res.json();
      if (data.success) {
        setTestResult({ success: true, text: "Connection successful! All paths and database are accessible." });
      } else {
        setTestResult({ success: false, text: "Connection failed: " + data.error });
      }
    } catch {
      setTestResult({ success: false, text: "Connection error. Please try again." });
    }
    setTestLoading(false);
  };

  if (fetching) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
      </div>
    );
  }

  const sections = [
    { id: "general", label: "General", icon: <FileText className="w-4 h-4" /> },
    { id: "database", label: "Database", icon: <Database className="w-4 h-4" /> },
    { id: "paths", label: "Paths", icon: <Folder className="w-4 h-4" /> },
    { id: "rules", label: "Scanner Rules", icon: <Shield className="w-4 h-4" /> },
  ];

  return (
    <ProtectedLayout>
      {/* Page Header */}
      <div className="mb-6">
        <div className="flex items-center gap-2 mb-4">
          <Link
            href={`/projects/${id}`}
            className="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <span className="text-slate-600">/</span>
          <Link href="/" className="text-slate-400 text-sm hover:text-slate-200">Projects</Link>
          <span className="text-slate-600">/</span>
          <span className="text-slate-400 text-sm">{formData.name}</span>
          <span className="text-slate-600">/</span>
          <span className="text-slate-200 text-sm font-medium">Configure</span>
        </div>

        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-100">Project Configuration</h1>
            <p className="text-slate-400 text-sm mt-1">Configure {formData.name} settings and connections</p>
          </div>
        </div>
      </div>

      {/* Section Tabs */}
      <div className="flex gap-1 mb-6 p-1 bg-slate-800/50 rounded-xl w-fit">
        {sections.map((section) => (
          <button
            key={section.id}
            onClick={() => setActiveSection(section.id as typeof activeSection)}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              activeSection === section.id
                ? "bg-slate-700 text-slate-100"
                : "text-slate-400 hover:text-slate-200"
            }`}
          >
            {section.icon}
            {section.label}
          </button>
        ))}
      </div>

      <form onSubmit={handleSubmit} className="flex gap-6">
        {/* Main Form */}
        <div className="flex-1 space-y-6">
          {/* General Section */}
          {activeSection === "general" && (
            <div className="glass-panel rounded-xl p-6">
              <h2 className="text-lg font-semibold text-slate-200 mb-6">General Information</h2>
              <div className="space-y-5">
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Project Name <span className="text-red-400">*</span>
                  </label>
                  <input
                    required
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-all"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Description
                  </label>
                  <textarea
                    rows={3}
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all resize-none"
                    placeholder="Brief description of this OJS deployment..."
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    OJS Template
                  </label>
                  <select
                    value={formData.template}
                    onChange={(e) => setFormData({ ...formData, template: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all"
                  >
                    <option value="OJS 3.x">OJS 3.x</option>
                    <option value="OJS 2.x">OJS 2.x</option>
                    <option value="Custom">Custom</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    <Clock className="w-4 h-4 inline mr-1" />
                    Auto Rescan Interval
                  </label>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      min="0"
                      max="1440"
                      value={formData.rescan_interval}
                      onChange={(e) => setFormData({ ...formData, rescan_interval: parseInt(e.target.value) || 0 })}
                      className="w-32 bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all"
                    />
                    <span className="text-slate-400 text-sm">minutes (0 = disabled)</span>
                  </div>
                  <p className="text-xs text-slate-500 mt-2">
                    Set to 0 to disable automatic rescans. Recommended: 10-60 minutes for active monitoring.
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Database Section */}
          {activeSection === "database" && (
            <div className="glass-panel rounded-xl p-6">
              <h2 className="text-lg font-semibold text-slate-200 mb-6">MySQL Database Connection</h2>
              <div className="grid grid-cols-2 gap-5">
                <div className="col-span-2">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    <Database className="w-4 h-4 inline mr-1" />
                    Host <span className="text-red-400">*</span>
                  </label>
                  <input
                    required
                    type="text"
                    value={formData.db_host}
                    onChange={(e) => setFormData({ ...formData, db_host: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all"
                    placeholder="localhost:3306"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    <Server className="w-4 h-4 inline mr-1" />
                    Database Name <span className="text-red-400">*</span>
                  </label>
                  <input
                    required
                    type="text"
                    value={formData.db_name}
                    onChange={(e) => setFormData({ ...formData, db_name: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all"
                    placeholder="ojs_database"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    <Key className="w-4 h-4 inline mr-1" />
                    Username <span className="text-red-400">*</span>
                  </label>
                  <input
                    required
                    type="text"
                    value={formData.db_user}
                    onChange={(e) => setFormData({ ...formData, db_user: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-slate-100 focus:border-blue-500 outline-none transition-all"
                    placeholder="root"
                  />
                </div>
                <div className="col-span-2">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    <Key className="w-4 h-4 inline mr-1" />
                    Password
                  </label>
                  <div className="relative">
                    <input
                      type={showPassword ? "text" : "password"}
                      value={formData.db_pass}
                      onChange={(e) => setFormData({ ...formData, db_pass: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 pr-12 text-slate-100 focus:border-blue-500 outline-none transition-all"
                      placeholder="Database password"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
                    >
                      {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Paths Section */}
          {activeSection === "paths" && (
            <div className="space-y-5">
              <div className="glass-panel rounded-xl p-6">
                <h2 className="text-lg font-semibold text-slate-200 mb-6">Application Paths</h2>
                <p className="text-sm text-slate-400 mb-4">
                  Directory paths where OJS application files are located. These directories will be scanned for file integrity.
                </p>
                <ArrayInput
                  label="App Paths (Source Code)"
                  values={formData.app_paths}
                  onChange={(v) => setFormData({ ...formData, app_paths: v })}
                  placeholder="/var/www/html"
                />
              </div>
              <div className="glass-panel rounded-xl p-6">
                <h2 className="text-lg font-semibold text-slate-200 mb-6">Upload Files Paths</h2>
                <p className="text-sm text-slate-400 mb-4">
                  Directory paths where uploaded files are stored. These paths are monitored for unauthorized files.
                </p>
                <ArrayInput
                  label="Files Paths (Uploads)"
                  values={formData.files_paths}
                  onChange={(v) => setFormData({ ...formData, files_paths: v })}
                  placeholder="/var/www/ojs-files"
                />
              </div>
            </div>
          )}

          {/* Scanner Rules Section */}
          {activeSection === "rules" && (
            <div className="space-y-5">
              <div className="glass-panel rounded-xl p-6">
                <h2 className="text-lg font-semibold text-slate-200 mb-2">Blacklist Extensions</h2>
                <p className="text-sm text-slate-400 mb-4">
                  File extensions that will be flagged as suspicious. Typically includes executable files.
                </p>
                <ArrayInput
                  label="Blocked Extensions"
                  values={formData.blacklist_exts}
                  onChange={(v) => setFormData({ ...formData, blacklist_exts: v })}
                  placeholder="php, phtml, sh, pl, py"
                />
              </div>
              <div className="glass-panel rounded-xl p-6">
                <h2 className="text-lg font-semibold text-slate-200 mb-2">Whitelist Paths</h2>
                <p className="text-sm text-slate-400 mb-4">
                  Directories that should be excluded from scanning even if they contain suspicious files.
                </p>
                <ArrayInput
                  label="Excluded Paths"
                  values={formData.whitelist_paths}
                  onChange={(v) => setFormData({ ...formData, whitelist_paths: v })}
                  placeholder="/var/www/cache, /var/www/tmp"
                />
              </div>
            </div>
          )}

          {/* Messages */}
          {message && (
            <div
              className={`p-4 rounded-xl flex items-center gap-3 ${
                message.type === "success"
                  ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"
                  : "bg-red-500/10 text-red-400 border border-red-500/20"
              }`}
            >
              {message.type === "success" ? <CheckCircle className="w-5 h-5" /> : <XCircle className="w-5 h-5" />}
              {message.text}
            </div>
          )}
        </div>

        {/* Sidebar */}
        <div className="w-80 space-y-5">
          {/* Actions Card */}
          <div className="glass-panel rounded-xl p-5 border border-slate-700/50 sticky top-24">
            <h3 className="font-semibold text-slate-200 mb-4">Actions</h3>
            <div className="space-y-3">
              <button
                type="submit"
                disabled={loading}
                className="w-full flex items-center justify-center gap-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white py-3 rounded-xl font-semibold transition-all"
              >
                {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Save className="w-5 h-5" />}
                Save Changes
              </button>
              <button
                type="button"
                onClick={handleTestConnection}
                disabled={testLoading}
                className="w-full flex items-center justify-center gap-2 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 py-3 rounded-xl font-semibold transition-all border border-slate-700"
              >
                {testLoading ? (
                  <Loader2 className="w-5 h-5 animate-spin" />
                ) : (
                  <TestTube className="w-5 h-5 text-emerald-400" />
                )}
                Test Connection
              </button>
            </div>

            {testResult && (
              <div
                className={`mt-4 p-3 rounded-lg text-sm ${
                  testResult.success
                    ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"
                    : "bg-red-500/10 text-red-400 border border-red-500/20"
                }`}
              >
                <div className="flex items-start gap-2">
                  {testResult.success ? (
                    <CheckCircle className="w-4 h-4 mt-0.5" />
                  ) : (
                    <XCircle className="w-4 h-4 mt-0.5" />
                  )}
                  <span>{testResult.text}</span>
                </div>
              </div>
            )}
          </div>

          {/* Help Card */}
          <div className="glass-panel rounded-xl p-5 border border-slate-700/50">
            <h3 className="font-semibold text-slate-200 mb-3">Configuration Help</h3>
            <div className="space-y-3 text-sm text-slate-400">
              <div className="flex items-start gap-2">
                <AlertCircle className="w-4 h-4 mt-0.5 text-amber-400" />
                <p>Database credentials must have read access to OJS tables.</p>
              </div>
              <div className="flex items-start gap-2">
                <AlertCircle className="w-4 h-4 mt-0.5 text-amber-400" />
                <p>App paths should contain the OJS source code directory.</p>
              </div>
              <div className="flex items-start gap-2">
                <AlertCircle className="w-4 h-4 mt-0.5 text-amber-400" />
                <p>Run a baseline scan after saving to establish file integrity baseline.</p>
              </div>
            </div>
          </div>
        </div>
      </form>
    </ProtectedLayout>
  );
}

function ArrayInput({
  label,
  values,
  onChange,
  placeholder,
}: {
  label: string;
  values: string[];
  onChange: (v: string[]) => void;
  placeholder: string;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-slate-300 mb-3">{label}</label>
      <div className="space-y-2">
        {values.map((v, i) => (
          <div key={i} className="flex gap-2">
            <input
              type="text"
              value={v}
              onChange={(e) => {
                const newV = [...values];
                newV[i] = e.target.value;
                onChange(newV);
              }}
              placeholder={placeholder}
              className="flex-1 bg-slate-800 border border-slate-700 rounded-lg p-2.5 text-slate-200 focus:border-blue-500 outline-none text-sm"
            />
            <button
              type="button"
              onClick={() => onChange(values.filter((_, idx) => idx !== i))}
              className="px-3 bg-red-500/10 border border-red-500/20 text-red-400 hover:bg-red-500/20 rounded-lg font-bold transition-all"
            >
              ×
            </button>
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={() => onChange([...values, ""])}
        className="text-sm bg-slate-800 border border-slate-700 hover:bg-slate-700 text-blue-400 px-3 py-1.5 rounded-lg mt-3 transition-all"
      >
        + Add Path
      </button>
    </div>
  );
}
