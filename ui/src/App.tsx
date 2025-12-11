import { useState, useEffect, useCallback } from "react";
import "./App.css";
import logo from "./assets/logo.png";
import { Kuaidi100 } from "./components/Kuaidi100";
import { ConfigEditor, type ConfigValues } from "./components/ConfigEditor";
import { DownloadJobCard } from "./components/DownloadJobCard";
import {
  type UITranslations,
  type ServerTranslations,
  defaultTranslations,
  defaultServerTranslations,
} from "./utils/translations";
import {
  type Job,
  type JobStatus,
  type HealthData,
  type WebDAVServer,
  fetchHealth,
  fetchJobs,
  fetchConfig,
  fetchI18n,
  updateConfig,
  setConfigValue,
  postDownload,
  addWebDAVServer,
  deleteWebDAVServer,
  deleteJob,
} from "./utils/apis";

export default function App() {
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
  const [serverT, setServerT] = useState<ServerTranslations>(
    defaultServerTranslations
  );
  const [configExists, setConfigExists] = useState(true);
  const [showConfigEditor, setShowConfigEditor] = useState(false);
  const [configLang, setConfigLang] = useState("");
  const [configFormat, setConfigFormat] = useState("");
  const [configQuality, setConfigQuality] = useState("");
  // Config values from server
  const [serverPort, setServerPort] = useState(8080);
  const [maxConcurrent, setMaxConcurrent] = useState(10);
  const [apiKey, setApiKey] = useState("");
  const [webdavServers, setWebdavServers] = useState<
    Record<string, WebDAVServer>
  >({});
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
      await setConfigValue(
        "express.kuaidi100.customer",
        values.kuaidi100Customer
      );
    }
    setShowConfigEditor(false);
    refresh();
  };

  const handleAddWebDAV = async (
    name: string,
    url: string,
    username: string,
    password: string
  ) => {
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
            onClick={() => setShowConfigEditor(!showConfigEditor)}
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
            onClick={() => setShowConfigEditor(true)}
          >
            ⚙️ {t.settings}
          </button>
        </div>
      )}

      {showConfigEditor && (
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
          onCancel={() => setShowConfigEditor(false)}
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
          <span className="count">
            {jobs.length} {t.total}
          </span>
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
              <DownloadJobCard
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
