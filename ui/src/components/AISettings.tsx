import { useState, useEffect } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchAIConfig,
  addAIAccount,
  deleteAIAccount,
  setDefaultAIAccount,
  type AIConfigData,
} from "../utils/apis";
import { FaTrash, FaStar, FaPlus } from "react-icons/fa6";

interface AISettingsProps {
  isConnected: boolean;
}

export function AISettings({ isConnected }: AISettingsProps) {
  const { t, showToast } = useApp();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [aiConfig, setAIConfig] = useState<AIConfigData | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);

  // Form state - simplified: just name, provider, key, pin
  const [name, setName] = useState("");
  const [provider, setProvider] = useState("openai");
  const [apiKey, setApiKey] = useState("");
  const [pin, setPIN] = useState("");

  const loadConfig = async () => {
    try {
      const res = await fetchAIConfig();
      if (res.code === 200) {
        setAIConfig(res.data);
      }
    } catch {
      // Ignore errors
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConfig();
  }, []);

  const resetForm = () => {
    setName("");
    setProvider("openai");
    setApiKey("");
    setPIN("");
  };

  const handleAdd = async () => {
    if (!name || !apiKey) {
      showToast("error", "Name and API key are required");
      return;
    }
    // PIN is optional, but if provided must be exactly 4 digits
    if (pin && pin.length !== 4) {
      showToast("error", "PIN must be exactly 4 digits if provided");
      return;
    }

    setSaving(true);
    try {
      const res = await addAIAccount({
        name,
        provider,
        api_key: apiKey,
        pin: pin || undefined,
      });

      if (res.code === 200) {
        showToast("success", "AI account added");
        resetForm();
        setShowAddForm(false);
        loadConfig();
      } else {
        showToast("error", res.message || "Failed to add account");
      }
    } catch {
      showToast("error", "Failed to add account");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (accountName: string) => {
    if (!confirm(`Delete AI account "${accountName}"?`)) return;

    try {
      const res = await deleteAIAccount(accountName);
      if (res.code === 200) {
        showToast("success", "AI account deleted");
        loadConfig();
      } else {
        showToast("error", res.message || "Failed to delete account");
      }
    } catch {
      showToast("error", "Failed to delete account");
    }
  };

  const handleSetDefault = async (accountName: string) => {
    try {
      const res = await setDefaultAIAccount(accountName);
      if (res.code === 200) {
        showToast("success", "Default account updated");
        loadConfig();
      } else {
        showToast("error", res.message || "Failed to set default");
      }
    } catch {
      showToast("error", "Failed to set default");
    }
  };

  const inputBaseClass =
    "flex-1 px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm font-mono focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50";

  if (loading) {
    return (
      <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
        <div className="text-sm text-zinc-500">{t.loading}</div>
      </div>
    );
  }

  const accounts = aiConfig?.accounts ? Object.entries(aiConfig.accounts) : [];

  return (
    <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-white">
          {t.ai_settings}
        </h2>
        {!showAddForm && (
          <button
            className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-blue-500 border border-blue-500 text-white hover:bg-blue-600 hover:border-blue-600 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
            onClick={() => setShowAddForm(true)}
            disabled={!isConnected}
          >
            <FaPlus className="text-[10px]" /> {t.add}
          </button>
        )}
      </div>

      {/* Existing Accounts */}
      {accounts.length > 0 && (
        <div className="mb-4 flex flex-col gap-2">
          {accounts.map(([accountName, account]) => (
            <div
              key={accountName}
              className="flex items-center justify-between p-3 bg-zinc-50 dark:bg-zinc-800 rounded-lg"
            >
              <div className="flex items-center gap-3">
                {aiConfig?.default_account === accountName && (
                  <FaStar className="text-yellow-500 text-sm" title="Default" />
                )}
                <div>
                  <div className="text-sm font-medium text-zinc-900 dark:text-white">
                    {accountName}
                  </div>
                  <div className="text-xs text-zinc-500 dark:text-zinc-400">
                    {account.provider}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {aiConfig?.default_account !== accountName && (
                  <button
                    onClick={() => handleSetDefault(accountName)}
                    className="p-1.5 text-zinc-400 hover:text-yellow-500 transition-colors"
                    title="Set as default"
                  >
                    <FaStar />
                  </button>
                )}
                <button
                  onClick={() => handleDelete(accountName)}
                  className="p-1.5 text-zinc-400 hover:text-red-500 transition-colors"
                  title="Delete"
                >
                  <FaTrash />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {accounts.length === 0 && !showAddForm && (
        <div className="text-sm text-zinc-500 dark:text-zinc-400 mb-4">
          {t.ai_no_accounts}
        </div>
      )}

      {/* Add Form */}
      {showAddForm && (
        <div className="border-t border-zinc-200 dark:border-zinc-700 pt-4">
          <div className="flex flex-col gap-3">
            {/* Account Name */}
            <div className="flex items-center gap-3">
              <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.ai_account_name}
              </span>
              <input
                type="text"
                className={inputBaseClass}
                placeholder="e.g., personal, work"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={!isConnected || saving}
              />
            </div>

            {/* Provider */}
            <div className="flex items-center gap-3">
              <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.ai_provider}
              </span>
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                disabled={!isConnected || saving}
                className="flex-1 px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 disabled:opacity-50 cursor-pointer"
              >
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="qwen">Qwen (Alibaba)</option>
              </select>
            </div>

            {/* API Key */}
            <div className="flex items-center gap-3">
              <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.ai_api_key}
              </span>
              <input
                type="password"
                className={inputBaseClass}
                placeholder="sk-..."
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                disabled={!isConnected || saving}
              />
            </div>

            {/* PIN */}
            <div className="flex items-center gap-3">
              <span className="min-w-[100px] text-sm text-zinc-700 dark:text-zinc-200">
                {t.ai_pin}
              </span>
              <input
                type="password"
                maxLength={4}
                className={`${inputBaseClass} max-w-[100px] text-center tracking-widest`}
                placeholder="****"
                value={pin}
                onChange={(e) => setPIN(e.target.value.replace(/\D/g, ""))}
                disabled={!isConnected || saving}
              />
              <span className="text-xs text-zinc-500 dark:text-zinc-400">
                {t.ai_pin_hint}
              </span>
            </div>

            {/* Buttons */}
            <div className="flex gap-2 mt-2">
              <button
                className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-transparent border border-zinc-300 dark:border-zinc-700 text-zinc-500 hover:border-zinc-500 hover:text-zinc-900 dark:hover:text-white"
                onClick={() => {
                  resetForm();
                  setShowAddForm(false);
                }}
                disabled={saving}
              >
                {t.cancel}
              </button>
              <button
                className="px-3 py-1.5 rounded text-xs cursor-pointer transition-colors bg-blue-500 border border-blue-500 text-white hover:bg-blue-600 hover:border-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
                onClick={handleAdd}
                disabled={!isConnected || saving || !name || !apiKey || (pin.length > 0 && pin.length !== 4)}
              >
                {saving ? "..." : t.save}
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="text-xs text-zinc-400 dark:text-zinc-600 mt-4">
        {t.ai_encryption_note}
      </div>
    </div>
  );
}
