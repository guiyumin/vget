import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { ToastContainer, type ToastData, type ToastType } from "../components/Toast";
import {
  type UITranslations,
  type ServerTranslations,
  defaultTranslations,
  defaultServerTranslations,
} from "../utils/translations";
import {
  type Job,
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
} from "../utils/apis";
import { type ConfigValues } from "../components/ConfigEditor";

interface AppContextType {
  // Connection state
  health: HealthData | null;
  isConnected: boolean;
  loading: boolean;

  // Jobs
  jobs: Job[];

  // Config
  outputDir: string;
  configLang: string;
  configFormat: string;
  configQuality: string;
  serverPort: number;
  maxConcurrent: number;
  apiKey: string;
  webdavServers: Record<string, WebDAVServer>;
  kuaidi100Key: string;
  kuaidi100Customer: string;
  configExists: boolean;
  torrentEnabled: boolean;

  // Translations
  t: UITranslations;
  serverT: ServerTranslations;

  // Theme
  darkMode: boolean;
  setDarkMode: (dark: boolean) => void;

  // Actions
  refresh: () => Promise<void>;
  submitDownload: (url: string) => Promise<boolean>;
  cancelJob: (id: string) => Promise<void>;
  clearAllHistory: () => Promise<void>;
  updateOutputDir: (dir: string) => Promise<boolean>;
  saveConfig: (values: ConfigValues) => Promise<void>;
  addWebDAV: (
    name: string,
    url: string,
    username: string,
    password: string
  ) => Promise<void>;
  deleteWebDAV: (name: string) => Promise<void>;
  showToast: (type: ToastType, message: string) => void;
}

const AppContext = createContext<AppContextType | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const [health, setHealth] = useState<HealthData | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [outputDir, setOutputDir] = useState("");
  const [darkMode, setDarkModeState] = useState(() => {
    const saved = localStorage.getItem("vget-theme");
    return saved ? saved === "dark" : true;
  });
  const [t, setT] = useState<UITranslations>(defaultTranslations);
  const [serverT, setServerT] = useState<ServerTranslations>(
    defaultServerTranslations
  );
  const [configExists, setConfigExists] = useState(true);
  const [configLang, setConfigLang] = useState("");
  const [configFormat, setConfigFormat] = useState("");
  const [configQuality, setConfigQuality] = useState("");
  const [serverPort, setServerPort] = useState(8080);
  const [maxConcurrent, setMaxConcurrent] = useState(10);
  const [apiKey, setApiKey] = useState("");
  const [webdavServers, setWebdavServers] = useState<
    Record<string, WebDAVServer>
  >({});
  const [kuaidi100Key, setKuaidi100Key] = useState("");
  const [kuaidi100Customer, setKuaidi100Customer] = useState("");
  const [torrentEnabled, setTorrentEnabled] = useState(false);
  const [toasts, setToasts] = useState<ToastData[]>([]);

  const showToast = useCallback((type: ToastType, message: string) => {
    const id = Math.random().toString(36).substring(2, 9);
    setToasts((prev) => [...prev, { id, type, message }]);
  }, []);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const setDarkMode = useCallback((dark: boolean) => {
    setDarkModeState(dark);
  }, []);

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
        const kuaidi100Cfg = configRes.data.express?.kuaidi100;
        setKuaidi100Key(kuaidi100Cfg?.key || "");
        setKuaidi100Customer(kuaidi100Cfg?.customer || "");
        setTorrentEnabled(configRes.data.torrent_enabled || false);
      }
      if (i18nRes.code === 200) {
        // Merge with defaults to ensure new keys are available
        setT({ ...defaultTranslations, ...i18nRes.data.ui });
        setServerT({ ...defaultServerTranslations, ...i18nRes.data.server });
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

  const submitDownload = useCallback(
    async (url: string) => {
      const res = await postDownload(url.trim());
      if (res.code === 200) {
        refresh();
        return true;
      }
      return false;
    },
    [refresh]
  );

  const cancelJob = useCallback(
    async (id: string) => {
      await deleteJob(id);
      refresh();
    },
    [refresh]
  );

  const clearAllHistory = useCallback(async () => {
    await clearHistory();
    refresh();
  }, [refresh]);

  const updateOutputDir = useCallback(
    async (dir: string) => {
      const res = await updateConfig(dir.trim());
      if (res.code === 200) {
        setOutputDir(res.data.output_dir);
        return true;
      }
      return false;
    },
    []
  );

  const saveConfig = useCallback(
    async (values: ConfigValues) => {
      await setConfigValue("language", values.language || "en");
      await setConfigValue("format", values.format || "mp4");
      await setConfigValue("quality", values.quality || "best");
      await setConfigValue(
        "server_max_concurrent",
        values.maxConcurrent || "10"
      );
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
      refresh();
    },
    [refresh]
  );

  const addWebDAV = useCallback(
    async (name: string, url: string, username: string, password: string) => {
      const res = await addWebDAVServer(name, url, username, password);
      if (res.code === 200) {
        refresh();
      }
    },
    [refresh]
  );

  const deleteWebDAV = useCallback(
    async (name: string) => {
      const res = await deleteWebDAVServer(name);
      if (res.code === 200) {
        refresh();
      }
    },
    [refresh]
  );

  const isConnected = health?.status === "ok";

  return (
    <AppContext.Provider
      value={{
        health,
        isConnected,
        loading,
        jobs,
        outputDir,
        configLang,
        configFormat,
        configQuality,
        serverPort,
        maxConcurrent,
        apiKey,
        webdavServers,
        kuaidi100Key,
        kuaidi100Customer,
        configExists,
        torrentEnabled,
        t,
        serverT,
        darkMode,
        setDarkMode,
        refresh,
        submitDownload,
        cancelJob,
        clearAllHistory,
        updateOutputDir,
        saveConfig,
        addWebDAV,
        deleteWebDAV,
        showToast,
      }}
    >
      {children}
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </AppContext.Provider>
  );
}

export function useApp() {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error("useApp must be used within an AppProvider");
  }
  return context;
}
