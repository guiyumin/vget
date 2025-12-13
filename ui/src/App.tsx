import { useState, useEffect, useCallback } from "react";
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
  clearHistory,
} from "./utils/apis";
import { FaGear } from "react-icons/fa6";
import { CiLight, CiDark } from "react-icons/ci";

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
    if (darkMode) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
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

  const handleClearHistory = async () => {
    await clearHistory();
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
    <div className="max-w-3xl mx-auto p-8 bg-zinc-100 dark:bg-zinc-950 min-h-screen text-zinc-900 dark:text-white transition-colors">
      <header className="flex justify-between items-center mb-8 pb-6 border-b border-zinc-300 dark:border-zinc-700">
        <div className="flex items-center gap-3">
          <img
            src={logo}
            alt="vget"
            className={`w-10 h-10 object-contain transition-all ${isConnected ? "" : "grayscale opacity-50"}`}
          />
          <h1 className="text-2xl font-bold bg-gradient-to-br from-amber-400 to-orange-500 bg-clip-text text-transparent">
            VGet Server
          </h1>
        </div>
        <div className="flex items-center gap-4">
          <button
            className="bg-transparent border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1.5 cursor-pointer text-base leading-none transition-colors hover:border-zinc-500 hover:bg-white dark:hover:bg-zinc-900"
            onClick={() => setShowConfigEditor(!showConfigEditor)}
            title={t.settings}
          >
            <FaGear />
          </button>
          <button
            className="bg-transparent border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1.5 cursor-pointer text-base leading-none transition-colors hover:border-zinc-500 hover:bg-white dark:hover:bg-zinc-900"
            onClick={() => setDarkMode(!darkMode)}
            title={darkMode ? "Switch to light mode" : "Switch to dark mode"}
          >
            {darkMode ? <CiLight /> : <CiDark />}
          </button>
          <span className="text-zinc-400 dark:text-zinc-600 text-sm px-2 py-1 bg-white dark:bg-zinc-900 rounded">
            {health?.version || "..."}
          </span>
        </div>
      </header>

      {!configExists && (
        <div className="flex items-start gap-3 p-3 mb-4 bg-amber-100 dark:bg-amber-900 border border-amber-500 rounded-lg">
          <span className="text-xl leading-none">⚠️</span>
          <div className="flex-1">
            <p className="text-amber-800 dark:text-amber-100 text-sm">{serverT.no_config_warning}</p>
            <p className="text-amber-700 dark:text-amber-200 text-xs mt-1 opacity-80">{serverT.run_init_hint}</p>
          </div>
          <button
            className="px-4 py-2 border border-amber-500 rounded-md bg-amber-200/30 dark:bg-amber-500/20 text-amber-800 dark:text-amber-100 text-sm cursor-pointer whitespace-nowrap transition-colors hover:bg-amber-200/50 dark:hover:bg-amber-500/40"
            onClick={() => setShowConfigEditor(true)}
          >
            <FaGear className="inline mr-1" /> {t.settings}
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

      <div className="flex items-center gap-3 mb-4">
        <span className="text-zinc-700 dark:text-zinc-200 text-sm whitespace-nowrap">{t.download_to}</span>
        <input
          type="text"
          className={`flex-1 px-3 py-2 border rounded font-mono text-sm transition-colors
            ${editingDir
              ? "border-blue-500 bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white"
              : "border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 text-zinc-700 dark:text-zinc-200 cursor-default"
            } focus:outline-none placeholder:text-zinc-400 dark:placeholder:text-zinc-600`}
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
          <div className="flex gap-2">
            <button
              onClick={handleSaveDir}
              className="px-3 py-1.5 border border-green-500 text-green-500 rounded text-xs cursor-pointer whitespace-nowrap hover:bg-green-500 hover:text-white transition-colors"
            >
              {t.save}
            </button>
            <button
              onClick={handleCancelEdit}
              className="px-3 py-1.5 border border-zinc-300 dark:border-zinc-700 text-zinc-500 rounded text-xs cursor-pointer whitespace-nowrap hover:border-zinc-500 hover:text-zinc-900 dark:hover:text-white transition-colors"
            >
              {t.cancel}
            </button>
          </div>
        ) : (
          <button
            onClick={handleEditDir}
            className="px-3 py-1.5 border border-zinc-300 dark:border-zinc-700 text-zinc-500 rounded text-xs cursor-pointer whitespace-nowrap hover:border-blue-500 hover:text-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            disabled={!isConnected}
          >
            {t.edit}
          </button>
        )}
      </div>

      <form className="flex gap-3 mb-8" onSubmit={handleSubmit}>
        <input
          type="text"
          className="flex-1 px-4 py-3 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white text-base focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder={t.paste_url}
          disabled={!isConnected || submitting}
        />
        <button
          type="submit"
          className="px-6 py-3 border-none rounded-lg bg-blue-500 text-white text-base font-medium cursor-pointer hover:bg-blue-600 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 disabled:cursor-not-allowed transition-colors"
          disabled={!isConnected || !url.trim() || submitting}
        >
          {submitting ? t.adding : t.download}
        </button>
      </form>

      {/* Kuaidi100 - only shown for Chinese language */}
      {configLang === "zh" && <Kuaidi100 isConnected={isConnected} />}

      <section>
        <div className="flex items-center gap-3 mb-4">
          <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200">{t.jobs}</h2>
          <span className="text-zinc-700 dark:text-zinc-200 text-sm">
            {jobs.length} {t.total}
          </span>
          <div className="flex gap-2 ml-auto">
            <button
              className="px-2 py-1 border border-zinc-300 dark:border-zinc-700 rounded bg-transparent text-zinc-500 text-[0.7rem] cursor-pointer transition-colors hover:border-red-500 hover:text-red-500 disabled:opacity-50 disabled:cursor-not-allowed"
              onClick={handleClearHistory}
              disabled={
                !isConnected ||
                !jobs.some(
                  (j) =>
                    j.status === "completed" ||
                    j.status === "failed" ||
                    j.status === "cancelled"
                )
              }
              title={t.clear_all}
            >
              {t.clear_all}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">Loading...</div>
        ) : sortedJobs.length === 0 ? (
          <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">
            <p>{t.no_downloads}</p>
            <p className="text-sm mt-2">{t.paste_hint}</p>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {sortedJobs.map((job) => (
              <DownloadJobCard
                key={job.id}
                job={job}
                onCancel={() => handleCancel(job.id)}
                onClear={() => handleCancel(job.id)}
                t={t}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
