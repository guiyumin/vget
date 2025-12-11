import { useState, useEffect, useCallback } from "react";
import "./App.css";
import logo from "./assets/logo.png";

type JobStatus =
  | "queued"
  | "downloading"
  | "completed"
  | "failed"
  | "cancelled";

interface Job {
  id: string;
  url: string;
  status: JobStatus;
  progress: number;
  filename?: string;
  error?: string;
}

interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

interface HealthData {
  status: string;
  version: string;
}

interface WebDAVServer {
  url: string;
  username: string;
  password: string;
}

interface ConfigData {
  output_dir: string;
  language: string;
  format: string;
  quality: string;
  twitter_auth_token: string;
  server_port: number;
  server_max_concurrent: number;
  server_api_key: string;
  webdav_servers: Record<string, WebDAVServer>;
}

interface JobsData {
  jobs: Job[];
}

interface UITranslations {
  download_to: string;
  edit: string;
  save: string;
  cancel: string;
  paste_url: string;
  download: string;
  adding: string;
  jobs: string;
  total: string;
  no_downloads: string;
  paste_hint: string;
  queued: string;
  downloading: string;
  completed: string;
  failed: string;
  cancelled: string;
  settings: string;
  language: string;
  format: string;
  quality: string;
  twitter_auth: string;
  server_port: string;
  max_concurrent: string;
  api_key: string;
  webdav_servers: string;
  add: string;
  delete: string;
  name: string;
  url: string;
  username: string;
  password: string;
  no_webdav_servers: string;
}

interface ServerTranslations {
  no_config_warning: string;
  run_init_hint: string;
}

interface I18nData {
  language: string;
  ui: UITranslations;
  server: ServerTranslations;
  config_exists: boolean;
}

const defaultTranslations: UITranslations = {
  download_to: "Download to:",
  edit: "Edit",
  save: "Save",
  cancel: "Cancel",
  paste_url: "Paste URL to download...",
  download: "Download",
  adding: "Adding...",
  jobs: "Jobs",
  total: "total",
  no_downloads: "No downloads yet",
  paste_hint: "Paste a URL above to get started",
  queued: "queued",
  downloading: "downloading",
  completed: "completed",
  failed: "failed",
  cancelled: "cancelled",
  settings: "Settings",
  language: "Language",
  format: "Format",
  quality: "Quality",
  twitter_auth: "Twitter Auth",
  server_port: "Server Port",
  max_concurrent: "Max Concurrent",
  api_key: "API Key",
  webdav_servers: "WebDAV Servers",
  add: "Add",
  delete: "Delete",
  name: "Name",
  url: "URL",
  username: "Username",
  password: "Password",
  no_webdav_servers: "No WebDAV servers configured",
};

const defaultServerTranslations: ServerTranslations = {
  no_config_warning: "No config file found. Using default settings.",
  run_init_hint: "Run 'vget init' to configure vget interactively.",
};

async function fetchHealth(): Promise<ApiResponse<HealthData>> {
  const res = await fetch("/health");
  return res.json();
}

async function fetchJobs(): Promise<ApiResponse<JobsData>> {
  const res = await fetch("/jobs");
  return res.json();
}

async function fetchConfig(): Promise<ApiResponse<ConfigData>> {
  const res = await fetch("/config");
  return res.json();
}

async function fetchI18n(): Promise<ApiResponse<I18nData>> {
  const res = await fetch("/i18n");
  return res.json();
}

async function updateConfig(
  outputDir: string
): Promise<ApiResponse<ConfigData>> {
  const res = await fetch("/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ output_dir: outputDir }),
  });
  return res.json();
}

async function setConfigValue(
  key: string,
  value: string
): Promise<ApiResponse<{ key: string; value: string }>> {
  const res = await fetch("/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key, value }),
  });
  return res.json();
}

async function postDownload(
  url: string
): Promise<ApiResponse<{ id: string; status: string }>> {
  const res = await fetch("/download", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
  return res.json();
}

async function addWebDAVServer(
  name: string,
  url: string,
  username: string,
  password: string
): Promise<ApiResponse<{ name: string }>> {
  const res = await fetch("/config/webdav", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, url, username, password }),
  });
  return res.json();
}

async function deleteWebDAVServer(
  name: string
): Promise<ApiResponse<{ name: string }>> {
  const res = await fetch(`/config/webdav/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
  return res.json();
}

async function deleteJob(id: string): Promise<ApiResponse<{ id: string }>> {
  const res = await fetch(`/jobs/${id}`, { method: "DELETE" });
  return res.json();
}

function App() {
  const [health, setHealth] = useState<HealthData | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [outputDir, setOutputDir] = useState("");
  const [editingDir, setEditingDir] = useState(false);
  const [newOutputDir, setNewOutputDir] = useState("");
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem("vget-theme");
    return saved ? saved === "dark" : true;
  });
  const [t, setT] = useState<UITranslations>(defaultTranslations);
  const [serverT, setServerT] = useState<ServerTranslations>(defaultServerTranslations);
  const [configExists, setConfigExists] = useState(true);
  const [showSettings, setShowSettings] = useState(false);
  const [configLang, setConfigLang] = useState("");
  const [configFormat, setConfigFormat] = useState("");
  const [configQuality, setConfigQuality] = useState("");
  const [savingConfig, setSavingConfig] = useState(false);
  // Config values from server
  const [serverPort, setServerPort] = useState(8080);
  const [maxConcurrent, setMaxConcurrent] = useState(10);
  const [apiKey, setApiKey] = useState("");
  const [webdavServers, setWebdavServers] = useState<Record<string, WebDAVServer>>({});
  // Local state for unsaved changes
  const [pendingLang, setPendingLang] = useState("");
  const [pendingFormat, setPendingFormat] = useState("");
  const [pendingQuality, setPendingQuality] = useState("");
  const [pendingTwitterAuth, setPendingTwitterAuth] = useState("");
  const [pendingMaxConcurrent, setPendingMaxConcurrent] = useState("10");
  const [pendingApiKey, setPendingApiKey] = useState("");
  // WebDAV add form
  const [newWebDAVName, setNewWebDAVName] = useState("");
  const [newWebDAVUrl, setNewWebDAVUrl] = useState("");
  const [newWebDAVUsername, setNewWebDAVUsername] = useState("");
  const [newWebDAVPassword, setNewWebDAVPassword] = useState("");
  const [addingWebDAV, setAddingWebDAV] = useState(false);

  useEffect(() => {
    document.documentElement.setAttribute(
      "data-theme",
      darkMode ? "dark" : "light"
    );
    localStorage.setItem("vget-theme", darkMode ? "dark" : "light");
  }, [darkMode]);

  const refresh = useCallback(async () => {
    try {
      const [healthRes, jobsRes, configRes, i18nRes] = await Promise.all([
        fetchHealth(),
        fetchJobs(),
        fetchConfig(),
        fetchI18n(),
      ]);
      if (healthRes.code === 200) setHealth(healthRes.data);
      if (jobsRes.code === 200) setJobs(jobsRes.data.jobs || []);
      if (configRes.code === 200) {
        setOutputDir(configRes.data.output_dir);
        setConfigLang(configRes.data.language || "");
        setConfigFormat(configRes.data.format || "");
        setConfigQuality(configRes.data.quality || "");
        setServerPort(configRes.data.server_port || 8080);
        setMaxConcurrent(configRes.data.server_max_concurrent || 10);
        setApiKey(configRes.data.server_api_key || "");
        setWebdavServers(configRes.data.webdav_servers || {});
        // Initialize pending values if not already set (first load)
        if (!pendingLang) setPendingLang(configRes.data.language || "en");
        if (!pendingFormat) setPendingFormat(configRes.data.format || "mp4");
        if (!pendingQuality) setPendingQuality(configRes.data.quality || "best");
        if (!pendingMaxConcurrent) setPendingMaxConcurrent(String(configRes.data.server_max_concurrent || 10));
        if (!pendingApiKey && configRes.data.server_api_key) setPendingApiKey(configRes.data.server_api_key);
      }
      if (i18nRes.code === 200) {
        setT(i18nRes.data.ui);
        setServerT(i18nRes.data.server);
        setConfigExists(i18nRes.data.config_exists);
      }
    } catch {
      setHealth(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 1000);
    return () => clearInterval(interval);
  }, [refresh]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || submitting) return;

    setSubmitting(true);
    try {
      const res = await postDownload(url.trim());
      if (res.code === 200) {
        setUrl("");
        refresh();
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancel = async (id: string) => {
    await deleteJob(id);
    refresh();
  };

  const handleEditDir = () => {
    setNewOutputDir(outputDir);
    setEditingDir(true);
  };

  const handleSaveDir = async () => {
    if (!newOutputDir.trim()) return;
    const res = await updateConfig(newOutputDir.trim());
    if (res.code === 200) {
      setOutputDir(res.data.output_dir);
      setEditingDir(false);
    }
  };

  const handleCancelEdit = () => {
    setEditingDir(false);
    setNewOutputDir("");
  };

  const handlePendingChange = (key: string, value: string) => {
    if (key === "language") setPendingLang(value);
    else if (key === "format") setPendingFormat(value);
    else if (key === "quality") setPendingQuality(value);
  };

  const handleSaveSettings = async () => {
    setSavingConfig(true);
    try {
      // Always save all values (creates config file if it doesn't exist)
      await setConfigValue("language", pendingLang || "en");
      await setConfigValue("format", pendingFormat || "mp4");
      await setConfigValue("quality", pendingQuality || "best");
      await setConfigValue("server_max_concurrent", pendingMaxConcurrent || "10");
      await setConfigValue("server_api_key", pendingApiKey);
      // Only save twitter auth if provided
      if (pendingTwitterAuth) {
        await setConfigValue("twitter.auth_token", pendingTwitterAuth);
      }
      setShowSettings(false);
      refresh();
    } finally {
      setSavingConfig(false);
    }
  };

  const handleCancelSettings = () => {
    // Reset pending values to current saved values
    setPendingLang(configLang || "en");
    setPendingFormat(configFormat || "mp4");
    setPendingQuality(configQuality || "best");
    setPendingTwitterAuth("");
    setPendingMaxConcurrent(String(maxConcurrent || 10));
    setPendingApiKey(apiKey || "");
    // Reset WebDAV form
    setNewWebDAVName("");
    setNewWebDAVUrl("");
    setNewWebDAVUsername("");
    setNewWebDAVPassword("");
    setShowSettings(false);
  };

  const handleAddWebDAV = async () => {
    if (!newWebDAVName.trim() || !newWebDAVUrl.trim()) return;
    setAddingWebDAV(true);
    try {
      const res = await addWebDAVServer(
        newWebDAVName.trim(),
        newWebDAVUrl.trim(),
        newWebDAVUsername,
        newWebDAVPassword
      );
      if (res.code === 200) {
        setNewWebDAVName("");
        setNewWebDAVUrl("");
        setNewWebDAVUsername("");
        setNewWebDAVPassword("");
        refresh();
      }
    } finally {
      setAddingWebDAV(false);
    }
  };

  const handleDeleteWebDAV = async (name: string) => {
    const res = await deleteWebDAVServer(name);
    if (res.code === 200) {
      refresh();
    }
  };

  const isConnected = health?.status === "ok";

  const sortedJobs = [...jobs].sort((a, b) => {
    const order: Record<JobStatus, number> = {
      downloading: 0,
      queued: 1,
      completed: 2,
      failed: 3,
      cancelled: 4,
    };
    return (order[a.status] ?? 5) - (order[b.status] ?? 5);
  });

  return (
    <div className="container">
      <header className="header">
        <div className="header-left">
          <img
            src={logo}
            alt="vget"
            className={`logo ${isConnected ? "" : "disconnected"}`}
          />
          <h1>VGet Server</h1>
        </div>
        <div className="header-right">
          <button
            className="settings-toggle"
            onClick={() => setShowSettings(!showSettings)}
            title={t.settings}
          >
            ⚙️
          </button>
          <button
            className="theme-toggle"
            onClick={() => setDarkMode(!darkMode)}
            title={darkMode ? "Switch to light mode" : "Switch to dark mode"}
          >
            {darkMode ? "☀️" : "🌙"}
          </button>
          <span className="version">{health?.version || "..."}</span>
        </div>
      </header>

      {!configExists && (
        <div className="warning-banner">
          <span className="warning-icon">⚠️</span>
          <div className="warning-text">
            <p>{serverT.no_config_warning}</p>
            <p className="warning-hint">{serverT.run_init_hint}</p>
          </div>
          <button
            className="warning-settings-btn"
            onClick={() => setShowSettings(true)}
          >
            ⚙️ {t.settings}
          </button>
        </div>
      )}

      {showSettings && (
        <div className="settings-panel">
          <div className="settings-header">
            <h2>{t.settings}</h2>
            <div className="settings-actions">
              <button
                className="settings-cancel-btn"
                onClick={handleCancelSettings}
                disabled={savingConfig}
              >
                {t.cancel}
              </button>
              <button
                className="settings-save-btn"
                onClick={handleSaveSettings}
                disabled={!isConnected || savingConfig}
              >
                {savingConfig ? "..." : t.save}
              </button>
            </div>
          </div>
          <div className="settings-grid">
            <SettingRow
              label={t.language}
              value={pendingLang}
              options={["en", "zh", "jp", "kr", "es", "fr", "de"]}
              disabled={!isConnected || savingConfig}
              onChange={(v) => handlePendingChange("language", v)}
            />
            <SettingRow
              label={t.format}
              value={pendingFormat}
              options={["mp4", "webm", "best"]}
              disabled={!isConnected || savingConfig}
              onChange={(v) => handlePendingChange("format", v)}
            />
            <SettingRow
              label={t.quality}
              value={pendingQuality}
              options={["best", "1080p", "720p", "480p"]}
              disabled={!isConnected || savingConfig}
              onChange={(v) => handlePendingChange("quality", v)}
            />
            <div className="setting-row">
              <span className="setting-label">{t.twitter_auth}</span>
              <input
                type="password"
                className="setting-input-text"
                placeholder="auth_token"
                value={pendingTwitterAuth}
                onChange={(e) => setPendingTwitterAuth(e.target.value)}
                disabled={!isConnected || savingConfig}
              />
            </div>
            <div className="setting-row">
              <span className="setting-label">{t.server_port}</span>
              <span className="setting-value-readonly">{serverPort || 8080}</span>
            </div>
            <div className="setting-row">
              <span className="setting-label">{t.max_concurrent}</span>
              <input
                type="number"
                className="setting-input-text setting-input-number"
                value={pendingMaxConcurrent}
                onChange={(e) => setPendingMaxConcurrent(e.target.value)}
                disabled={!isConnected || savingConfig}
                min="1"
                max="50"
              />
            </div>
            <div className="setting-row">
              <span className="setting-label">{t.api_key}</span>
              <input
                type="password"
                className="setting-input-text"
                placeholder="(optional)"
                value={pendingApiKey}
                onChange={(e) => setPendingApiKey(e.target.value)}
                disabled={!isConnected || savingConfig}
              />
            </div>
          </div>

          {/* WebDAV Servers Section */}
          <div className="webdav-section">
            <div className="webdav-header">{t.webdav_servers}</div>
            {Object.keys(webdavServers).length === 0 ? (
              <div className="webdav-empty">{t.no_webdav_servers}</div>
            ) : (
              <div className="webdav-list">
                {Object.entries(webdavServers).map(([name, server]) => (
                  <div key={name} className="webdav-item">
                    <div className="webdav-item-info">
                      <span className="webdav-name">{name}</span>
                      <span className="webdav-url">{server.url}</span>
                    </div>
                    <button
                      className="webdav-delete-btn"
                      onClick={() => handleDeleteWebDAV(name)}
                      disabled={!isConnected}
                    >
                      {t.delete}
                    </button>
                  </div>
                ))}
              </div>
            )}
            <div className="webdav-add-form">
              <input
                type="text"
                className="webdav-input"
                placeholder={t.name}
                value={newWebDAVName}
                onChange={(e) => setNewWebDAVName(e.target.value)}
                disabled={!isConnected || addingWebDAV}
              />
              <input
                type="text"
                className="webdav-input webdav-input-url"
                placeholder={t.url}
                value={newWebDAVUrl}
                onChange={(e) => setNewWebDAVUrl(e.target.value)}
                disabled={!isConnected || addingWebDAV}
              />
              <input
                type="text"
                className="webdav-input"
                placeholder={t.username}
                value={newWebDAVUsername}
                onChange={(e) => setNewWebDAVUsername(e.target.value)}
                disabled={!isConnected || addingWebDAV}
              />
              <input
                type="password"
                className="webdav-input"
                placeholder={t.password}
                value={newWebDAVPassword}
                onChange={(e) => setNewWebDAVPassword(e.target.value)}
                disabled={!isConnected || addingWebDAV}
              />
              <button
                className="webdav-add-btn"
                onClick={handleAddWebDAV}
                disabled={!isConnected || addingWebDAV || !newWebDAVName.trim() || !newWebDAVUrl.trim()}
              >
                {addingWebDAV ? "..." : t.add}
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="output-dir">
        <span className="output-dir-label">{t.download_to}</span>
        <input
          type="text"
          className="output-dir-input"
          value={editingDir ? newOutputDir : outputDir}
          onChange={(e) => setNewOutputDir(e.target.value)}
          onKeyDown={(e) => {
            if (editingDir && e.key === "Enter") handleSaveDir();
            if (editingDir && e.key === "Escape") handleCancelEdit();
          }}
          readOnly={!editingDir}
          placeholder="..."
        />
        {editingDir ? (
          <div className="output-dir-actions">
            <button onClick={handleSaveDir} className="save-btn">
              {t.save}
            </button>
            <button onClick={handleCancelEdit} className="cancel-edit-btn">
              {t.cancel}
            </button>
          </div>
        ) : (
          <button
            onClick={handleEditDir}
            className="edit-btn"
            disabled={!isConnected}
          >
            {t.edit}
          </button>
        )}
      </div>

      <form className="download-form" onSubmit={handleSubmit}>
        <input
          type="text"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder={t.paste_url}
          disabled={!isConnected || submitting}
        />
        <button
          type="submit"
          disabled={!isConnected || !url.trim() || submitting}
        >
          {submitting ? t.adding : t.download}
        </button>
      </form>

      <section className="jobs-section">
        <div className="jobs-header">
          <h2>{t.jobs}</h2>
          <span className="count">{jobs.length} {t.total}</span>
        </div>

        {loading ? (
          <div className="empty-state">Loading...</div>
        ) : sortedJobs.length === 0 ? (
          <div className="empty-state">
            <p>{t.no_downloads}</p>
            <p className="hint">{t.paste_hint}</p>
          </div>
        ) : (
          <div className="jobs-list">
            {sortedJobs.map((job) => (
              <JobCard
                key={job.id}
                job={job}
                onCancel={() => handleCancel(job.id)}
                t={t}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function JobCard({
  job,
  onCancel,
  t,
}: {
  job: Job;
  onCancel: () => void;
  t: UITranslations;
}) {
  const canCancel = job.status === "queued" || job.status === "downloading";

  const statusText: Record<JobStatus, string> = {
    queued: t.queued,
    downloading: t.downloading,
    completed: t.completed,
    failed: t.failed,
    cancelled: t.cancelled,
  };

  return (
    <div className="job-card">
      <div className="job-header">
        <code className="job-id">{job.id}</code>
        <div className="job-actions">
          <span className={`status-badge ${job.status}`}>
            {statusText[job.status]}
          </span>
          {canCancel && (
            <button className="cancel-btn" onClick={onCancel}>
              {t.cancel}
            </button>
          )}
        </div>
      </div>
      <p className="job-url">{job.url}</p>
      {job.filename && <p className="job-filename">{job.filename}</p>}
      {job.status === "downloading" && (
        <div className="progress-container">
          <div className="progress-bar">
            <div
              className="progress-fill"
              style={{ width: `${job.progress}%` }}
            />
          </div>
          <span className="progress-text">{job.progress.toFixed(1)}%</span>
        </div>
      )}
      {job.status === "failed" && job.error && (
        <div className="error-message">{job.error}</div>
      )}
    </div>
  );
}

function SettingRow({
  label,
  value,
  options,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  options: string[];
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className="setting-row">
      <span className="setting-label">{label}</span>
      <select
        className="setting-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
      >
        {options.map((opt) => (
          <option key={opt} value={opt}>
            {opt}
          </option>
        ))}
      </select>
    </div>
  );
}

export default App;
