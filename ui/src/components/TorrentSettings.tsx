import { useState, useEffect } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchTorrentConfig,
  saveTorrentConfig,
  testTorrentConnection,
  type TorrentConfig,
} from "../utils/apis";

interface TorrentSettingsProps {
  isConnected: boolean;
}

export function TorrentSettings({ isConnected }: TorrentSettingsProps) {
  const { t, refresh } = useApp();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    message: string;
  } | null>(null);

  // Form state
  const [enabled, setEnabled] = useState(false);
  const [client, setClient] = useState("transmission");
  const [host, setHost] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [useHttps, setUseHttps] = useState(false);
  const [defaultSavePath, setDefaultSavePath] = useState("");

  // Load initial config
  useEffect(() => {
    const loadConfig = async () => {
      try {
        const res = await fetchTorrentConfig();
        if (res.code === 200) {
          setEnabled(res.data.enabled);
          setClient(res.data.client || "transmission");
          setHost(res.data.host || "");
          setUsername(res.data.username || "");
          setPassword(res.data.password || "");
          setUseHttps(res.data.use_https || false);
          setDefaultSavePath(res.data.default_save_path || "");
        }
      } catch {
        // Ignore errors
      } finally {
        setLoading(false);
      }
    };
    loadConfig();
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setTestResult(null);
    try {
      const config: TorrentConfig = {
        enabled,
        client,
        host,
        username,
        password,
        use_https: useHttps,
        default_save_path: defaultSavePath,
      };
      const res = await saveTorrentConfig(config);
      if (res.code === 200) {
        setTestResult({ success: true, message: "Settings saved" });
        refresh();
      } else {
        setTestResult({ success: false, message: res.message });
      }
    } catch {
      setTestResult({ success: false, message: "Failed to save" });
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await testTorrentConnection();
      if (res.code === 200) {
        setTestResult({
          success: true,
          message: `${t.torrent_test_success} (${res.data.client})`,
        });
      } else {
        setTestResult({ success: false, message: res.message });
      }
    } catch {
      setTestResult({ success: false, message: "Connection failed" });
    } finally {
      setTesting(false);
    }
  };

  const inputBaseClass =
    "flex-1 px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm font-mono focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50";

  if (loading) {
    return (
      <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
        <div className="text-sm text-zinc-500">Loading...</div>
      </div>
    );
  }

  return (
    <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-white">
          {t.torrent_settings}
        </h2>
        <div className="flex gap-2">
          {enabled && (
            <button
              className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-transparent border border-zinc-300 dark:border-zinc-700 text-zinc-500 hover:border-zinc-500 hover:text-zinc-900 dark:hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
              onClick={handleTest}
              disabled={!isConnected || testing || !host}
            >
              {testing ? t.torrent_testing : t.torrent_test}
            </button>
          )}
          <button
            className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-blue-500 border border-blue-500 text-white hover:bg-blue-600 hover:border-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleSave}
            disabled={!isConnected || saving}
          >
            {saving ? "..." : t.save}
          </button>
        </div>
      </div>

      {testResult && (
        <div
          className={`mb-4 px-3 py-2 rounded-md text-sm ${
            testResult.success
              ? "bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300"
              : "bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300"
          }`}
        >
          {testResult.message}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {/* Enable Toggle */}
        <div className="flex items-center gap-3">
          <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
            {t.torrent_enabled}
          </span>
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              disabled={!isConnected || saving}
              className="sr-only peer"
            />
            <div className="w-9 h-5 bg-zinc-300 dark:bg-zinc-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-zinc-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-500"></div>
          </label>
        </div>

        {enabled && (
          <>
            {/* Client Type */}
            <div className="flex items-center gap-3">
              <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.torrent_client}
              </span>
              <select
                value={client}
                onChange={(e) => setClient(e.target.value)}
                disabled={!isConnected || saving}
                className="flex-1 px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 disabled:opacity-50 cursor-pointer"
              >
                <option value="transmission">Transmission</option>
                <option value="qbittorrent">qBittorrent</option>
                <option value="synology">Synology Download Station</option>
              </select>
            </div>

            {/* Host */}
            <div className="flex items-center gap-3">
              <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.torrent_host}
              </span>
              <input
                type="text"
                className={inputBaseClass}
                placeholder="192.168.1.100:9091"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                disabled={!isConnected || saving}
              />
            </div>

            {/* Username */}
            <div className="flex items-center gap-3">
              <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.username}
              </span>
              <input
                type="text"
                className={inputBaseClass}
                placeholder="(optional)"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={!isConnected || saving}
              />
            </div>

            {/* Password */}
            <div className="flex items-center gap-3">
              <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.password}
              </span>
              <input
                type="password"
                className={inputBaseClass}
                placeholder="(optional)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={!isConnected || saving}
              />
            </div>

            {/* HTTPS */}
            <div className="flex items-center gap-3">
              <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
                HTTPS
              </span>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={useHttps}
                  onChange={(e) => setUseHttps(e.target.checked)}
                  disabled={!isConnected || saving}
                  className="sr-only peer"
                />
                <div className="w-9 h-5 bg-zinc-300 dark:bg-zinc-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-zinc-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-500"></div>
              </label>
            </div>

            {/* Default Save Path */}
            <div className="flex items-center gap-3">
              <span className="min-w-[120px] text-sm text-zinc-700 dark:text-zinc-200">
                Save Path
              </span>
              <input
                type="text"
                className={inputBaseClass}
                placeholder="(use client default)"
                value={defaultSavePath}
                onChange={(e) => setDefaultSavePath(e.target.value)}
                disabled={!isConnected || saving}
              />
            </div>
          </>
        )}

        <div className="text-xs text-zinc-400 dark:text-zinc-600 mt-2">
          {client === "transmission" && "Default port: 9091"}
          {client === "qbittorrent" && "Default port: 8080"}
          {client === "synology" && "Default port: 5000 (HTTP) / 5001 (HTTPS)"}
        </div>
      </div>
    </div>
  );
}
