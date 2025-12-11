import { useState } from "react";
import { ConfigRow } from "./ConfigRow";
import "./ConfigEditor.css";

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
  onAddWebDAV: (name: string, url: string, username: string, password: string) => Promise<void>;
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
  const [pendingQuality, setPendingQuality] = useState(initialQuality || "best");
  const [pendingTwitterAuth, setPendingTwitterAuth] = useState("");
  const [pendingMaxConcurrent, setPendingMaxConcurrent] = useState(String(initialMaxConcurrent || 10));
  const [pendingApiKey, setPendingApiKey] = useState(initialApiKey || "");
  const [pendingKuaidi100Key, setPendingKuaidi100Key] = useState(initialKuaidi100Key || "");
  const [pendingKuaidi100Customer, setPendingKuaidi100Customer] = useState(initialKuaidi100Customer || "");

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

  return (
    <div className="settings-panel">
      <div className="settings-header">
        <h2>{t.settings}</h2>
        <div className="settings-actions">
          <button
            className="settings-cancel-btn"
            onClick={handleCancel}
            disabled={savingConfig}
          >
            {t.cancel}
          </button>
          <button
            className="settings-save-btn"
            onClick={handleSave}
            disabled={!isConnected || savingConfig}
          >
            {savingConfig ? "..." : t.save}
          </button>
        </div>
      </div>
      <div className="settings-grid">
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

        {/* Kuaidi100 Section */}
        <div className="setting-section-label">Kuaidi100 (快递查询)</div>
        <div className="setting-row">
          <span className="setting-label">API Key</span>
          <input
            type="password"
            className="setting-input-text"
            placeholder="(optional)"
            value={pendingKuaidi100Key}
            onChange={(e) => setPendingKuaidi100Key(e.target.value)}
            disabled={!isConnected || savingConfig}
          />
        </div>
        <div className="setting-row">
          <span className="setting-label">Customer ID</span>
          <input
            type="text"
            className="setting-input-text"
            placeholder="(optional)"
            value={pendingKuaidi100Customer}
            onChange={(e) => setPendingKuaidi100Customer(e.target.value)}
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
  );
}
