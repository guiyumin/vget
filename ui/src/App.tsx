import { useState, useEffect, useCallback } from "react";
import "./App.css";
import logo from "./assets/logo.png";
import { Kuaidi100 } from "./components/Kuaidi100";
import { ConfigEditor, type ConfigValues } from "./components/ConfigEditor";

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
  express?: Record<string, Record<string, string>>;
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
  // Config values from server
  const [serverPort, setServerPort] = useState(8080);
  const [maxConcurrent, setMaxConcurrent] = useState(10);
  const [apiKey, setApiKey] = useState("");
  const [webdavServers, setWebdavServers] = useState<Record<string, WebDAVServer>>({});
  // Kuaidi100 config
  const [kuaidi100Key, setKuaidi100Key] = useState("");
  const [kuaidi100Customer, setKuaidi100Customer] = useState("");

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
        // Kuaidi100 config
        const kuaidi100Cfg = configRes.data.express?.kuaidi100;
        setKuaidi100Key(kuaidi100Cfg?.key || "");
        setKuaidi100Customer(kuaidi100Cfg?.customer || "");
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

  const handleSaveConfig = async (values: ConfigValues) => {
    // Save all values
    await setConfigValue("language", values.language || "en");
    await setConfigValue("format", values.format || "mp4");
    await setConfigValue("quality", values.quality || "best");
    await setConfigValue("server_max_concurrent", values.maxConcurrent || "10");
    await setConfigValue("server_api_key", values.apiKey);
    if (values.twitterAuth) {
      await setConfigValue("twitter.auth_token", values.twitterAuth);
    }
    if (values.kuaidi100Key) {
      await setConfigValue("express.kuaidi100.key", values.kuaidi100Key);
    }
    if (values.kuaidi100Customer) {
      await setConfigValue("express.kuaidi100.customer", values.kuaidi100Customer);
    }
    setShowSettings(false);
    refresh();
  };

  const handleAddWebDAV = async (name: string, url: string, username: string, password: string) => {
    const res = await addWebDAVServer(name, url, username, password);
    if (res.code === 200) {
      refresh();
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
        <ConfigEditor
          isConnected={isConnected}
          t={t}
          initialLang={configLang}
          initialFormat={configFormat}
          initialQuality={configQuality}
          initialMaxConcurrent={maxConcurrent}
          initialApiKey={apiKey}
          initialKuaidi100Key={kuaidi100Key}
          initialKuaidi100Customer={kuaidi100Customer}
          serverPort={serverPort}
          webdavServers={webdavServers}
          onSave={handleSaveConfig}
          onCancel={() => setShowSettings(false)}
          onAddWebDAV={handleAddWebDAV}
          onDeleteWebDAV={handleDeleteWebDAV}
        />
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

      {/* Kuaidi100 - only shown for Chinese language */}
      {configLang === "zh" && <Kuaidi100 isConnected={isConnected} />}

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

export default App;
