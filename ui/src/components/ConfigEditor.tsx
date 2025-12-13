import { useState } from "react";
import { ConfigRow } from "./ConfigRow";

interface WebDAVServer {
  url: string;
  username: string;
  password: string;
}

interface UITranslations {
  settings: string;
  save: string;
  cancel: string;
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

interface ConfigEditorProps {
  isConnected: boolean;
  t: UITranslations;
  // Initial values from config
  initialLang: string;
  initialFormat: string;
  initialQuality: string;
  initialMaxConcurrent: number;
  initialApiKey: string;
  initialKuaidi100Key: string;
  initialKuaidi100Customer: string;
  serverPort: number;
  webdavServers: Record<string, WebDAVServer>;
  // Callbacks
  onSave: (values: ConfigValues) => Promise<void>;
  onCancel: () => void;
  onAddWebDAV: (
    name: string,
    url: string,
    username: string,
    password: string
  ) => Promise<void>;
  onDeleteWebDAV: (name: string) => Promise<void>;
}

export interface ConfigValues {
  language: string;
  format: string;
  quality: string;
  twitterAuth: string;
  maxConcurrent: string;
  apiKey: string;
  kuaidi100Key: string;
  kuaidi100Customer: string;
}

export function ConfigEditor({
  isConnected,
  t,
  initialLang,
  initialFormat,
  initialQuality,
  initialMaxConcurrent,
  initialApiKey,
  initialKuaidi100Key,
  initialKuaidi100Customer,
  serverPort,
  webdavServers,
  onSave,
  onCancel,
  onAddWebDAV,
  onDeleteWebDAV,
}: ConfigEditorProps) {
  const [savingConfig, setSavingConfig] = useState(false);

  // Pending values (local state for editing)
  const [pendingLang, setPendingLang] = useState(initialLang || "en");
  const [pendingFormat, setPendingFormat] = useState(initialFormat || "mp4");
  const [pendingQuality, setPendingQuality] = useState(
    initialQuality || "best"
  );
  const [pendingTwitterAuth, setPendingTwitterAuth] = useState("");
  const [pendingMaxConcurrent, setPendingMaxConcurrent] = useState(
    String(initialMaxConcurrent || 10)
  );
  const [pendingApiKey, setPendingApiKey] = useState(initialApiKey || "");
  const [pendingKuaidi100Key, setPendingKuaidi100Key] = useState(
    initialKuaidi100Key || ""
  );
  const [pendingKuaidi100Customer, setPendingKuaidi100Customer] = useState(
    initialKuaidi100Customer || ""
  );

  // WebDAV add form
  const [newWebDAVName, setNewWebDAVName] = useState("");
  const [newWebDAVUrl, setNewWebDAVUrl] = useState("");
  const [newWebDAVUsername, setNewWebDAVUsername] = useState("");
  const [newWebDAVPassword, setNewWebDAVPassword] = useState("");
  const [addingWebDAV, setAddingWebDAV] = useState(false);

  const handleSave = async () => {
    setSavingConfig(true);
    try {
      await onSave({
        language: pendingLang,
        format: pendingFormat,
        quality: pendingQuality,
        twitterAuth: pendingTwitterAuth,
        maxConcurrent: pendingMaxConcurrent,
        apiKey: pendingApiKey,
        kuaidi100Key: pendingKuaidi100Key,
        kuaidi100Customer: pendingKuaidi100Customer,
      });
    } finally {
      setSavingConfig(false);
    }
  };

  const handleCancel = () => {
    // Reset to initial values
    setPendingLang(initialLang || "en");
    setPendingFormat(initialFormat || "mp4");
    setPendingQuality(initialQuality || "best");
    setPendingTwitterAuth("");
    setPendingMaxConcurrent(String(initialMaxConcurrent || 10));
    setPendingApiKey(initialApiKey || "");
    setPendingKuaidi100Key(initialKuaidi100Key || "");
    setPendingKuaidi100Customer(initialKuaidi100Customer || "");
    // Reset WebDAV form
    setNewWebDAVName("");
    setNewWebDAVUrl("");
    setNewWebDAVUsername("");
    setNewWebDAVPassword("");
    onCancel();
  };

  const handleAddWebDAV = async () => {
    if (!newWebDAVName.trim() || !newWebDAVUrl.trim()) return;
    setAddingWebDAV(true);
    try {
      await onAddWebDAV(
        newWebDAVName.trim(),
        newWebDAVUrl.trim(),
        newWebDAVUsername,
        newWebDAVPassword
      );
      setNewWebDAVName("");
      setNewWebDAVUrl("");
      setNewWebDAVUsername("");
      setNewWebDAVPassword("");
    } finally {
      setAddingWebDAV(false);
    }
  };

  const handleDeleteWebDAV = async (name: string) => {
    await onDeleteWebDAV(name);
  };

  const inputBaseClass =
    "flex-1 px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm font-mono focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50";

  return (
    <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4 mb-4">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-white">
          {t.settings}
        </h2>
        <div className="flex gap-2">
          <button
            className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-transparent border border-zinc-300 dark:border-zinc-700 text-zinc-500 hover:border-zinc-500 hover:text-zinc-900 dark:hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleCancel}
            disabled={savingConfig}
          >
            {t.cancel}
          </button>
          <button
            className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-blue-500 border border-blue-500 text-white hover:bg-blue-600 hover:border-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleSave}
            disabled={!isConnected || savingConfig}
          >
            {savingConfig ? "..." : t.save}
          </button>
        </div>
      </div>
      <div className="flex flex-col gap-3">
        <ConfigRow
          label={t.language}
          value={pendingLang}
          options={["en", "zh", "jp", "kr", "es", "fr", "de"]}
          disabled={!isConnected || savingConfig}
          onChange={setPendingLang}
        />
        <ConfigRow
          label={t.format}
          value={pendingFormat}
          options={["mp4", "webm", "best"]}
          disabled={!isConnected || savingConfig}
          onChange={setPendingFormat}
        />
        <ConfigRow
          label={t.quality}
          value={pendingQuality}
          options={["best", "1080p", "720p", "480p"]}
          disabled={!isConnected || savingConfig}
          onChange={setPendingQuality}
        />
        <div className="flex items-center gap-3">
          <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
            {t.twitter_auth}
          </span>
          <input
            type="password"
            className={inputBaseClass}
            placeholder="auth_token"
            value={pendingTwitterAuth}
            onChange={(e) => setPendingTwitterAuth(e.target.value)}
            disabled={!isConnected || savingConfig}
          />
        </div>
        <div className="flex items-center gap-3">
          <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
            {t.server_port}
          </span>
          <span className="flex-1 px-2 py-1.5 bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded text-zinc-500 text-sm font-mono">
            {serverPort || 8080}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
            {t.max_concurrent}
          </span>
          <input
            type="number"
            className={`${inputBaseClass} w-20 flex-none`}
            value={pendingMaxConcurrent}
            onChange={(e) => setPendingMaxConcurrent(e.target.value)}
            disabled={!isConnected || savingConfig}
            min="1"
            max="50"
          />
        </div>
        <div className="flex items-center gap-3">
          <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
            {t.api_key}
          </span>
          <input
            type="password"
            className={inputBaseClass}
            placeholder="(optional)"
            value={pendingApiKey}
            onChange={(e) => setPendingApiKey(e.target.value)}
            disabled={!isConnected || savingConfig}
          />
        </div>

        {/* Kuaidi100 Section */}
        <div className="text-sm font-semibold text-zinc-900 dark:text-white mt-4 mb-2 pt-3 border-t border-zinc-300 dark:border-zinc-700">
          Kuaidi100 (快递查询)
        </div>
        <div className="flex items-center gap-3">
          <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
            API Key
          </span>
          <input
            type="password"
            className={inputBaseClass}
            placeholder="(optional)"
            value={pendingKuaidi100Key}
            onChange={(e) => setPendingKuaidi100Key(e.target.value)}
            disabled={!isConnected || savingConfig}
          />
        </div>
        <div className="flex items-center gap-3">
          <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
            Customer ID
          </span>
          <input
            type="text"
            className={inputBaseClass}
            placeholder="(optional)"
            value={pendingKuaidi100Customer}
            onChange={(e) => setPendingKuaidi100Customer(e.target.value)}
            disabled={!isConnected || savingConfig}
          />
        </div>
      </div>

      {/* WebDAV Servers Section */}
      <div className="mt-4 pt-4 border-t border-zinc-300 dark:border-zinc-700">
        <div className="text-sm font-semibold text-zinc-900 dark:text-white mb-3">
          {t.webdav_servers}
        </div>
        {Object.keys(webdavServers).length === 0 ? (
          <div className="text-zinc-500 dark:text-zinc-600 text-sm py-2">
            {t.no_webdav_servers}
          </div>
        ) : (
          <div className="flex flex-col gap-2 mb-3">
            {Object.entries(webdavServers).map(([name, server]) => (
              <div
                key={name}
                className="flex items-center justify-between px-3 py-2 bg-zinc-100 dark:bg-zinc-950 border border-zinc-300 dark:border-zinc-700 rounded"
              >
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium text-zinc-900 dark:text-white">
                    {name}
                  </span>
                  <span className="text-xs text-zinc-500 font-mono">
                    {server.url}
                  </span>
                </div>
                <button
                  className="px-2 py-1 border border-red-500 rounded bg-transparent text-red-500 text-xs cursor-pointer hover:bg-red-500 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  onClick={() => handleDeleteWebDAV(name)}
                  disabled={!isConnected}
                >
                  {t.delete}
                </button>
              </div>
            ))}
          </div>
        )}
        <div className="flex flex-wrap gap-2 mt-2">
          <input
            type="text"
            className="px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm w-[100px] focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
            placeholder={t.name}
            value={newWebDAVName}
            onChange={(e) => setNewWebDAVName(e.target.value)}
            disabled={!isConnected || addingWebDAV}
          />
          <input
            type="text"
            className="flex-1 min-w-[150px] px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
            placeholder={t.url}
            value={newWebDAVUrl}
            onChange={(e) => setNewWebDAVUrl(e.target.value)}
            disabled={!isConnected || addingWebDAV}
          />
          <input
            type="text"
            className="px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm w-[100px] focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
            placeholder={t.username}
            value={newWebDAVUsername}
            onChange={(e) => setNewWebDAVUsername(e.target.value)}
            disabled={!isConnected || addingWebDAV}
          />
          <input
            type="password"
            className="px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm w-[100px] focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
            placeholder={t.password}
            value={newWebDAVPassword}
            onChange={(e) => setNewWebDAVPassword(e.target.value)}
            disabled={!isConnected || addingWebDAV}
          />
          <button
            className="px-3 py-1.5 border border-blue-500 rounded bg-blue-500 text-white text-sm cursor-pointer hover:bg-blue-600 hover:border-blue-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            onClick={handleAddWebDAV}
            disabled={
              !isConnected ||
              addingWebDAV ||
              !newWebDAVName.trim() ||
              !newWebDAVUrl.trim()
            }
          >
            {addingWebDAV ? "..." : t.add}
          </button>
        </div>
      </div>
    </div>
  );
}
