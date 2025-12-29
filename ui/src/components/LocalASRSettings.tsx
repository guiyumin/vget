import { useState, useEffect } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchLocalASRCapabilities,
  updateLocalASRConfig,
  type LocalASRCapabilities,
} from "../utils/apis";
import { FaServer, FaMicrochip, FaRobot } from "react-icons/fa6";

interface LocalASRSettingsProps {
  isConnected: boolean;
}

export function LocalASRSettings({ isConnected }: LocalASRSettingsProps) {
  const { t, showToast } = useApp();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [capabilities, setCapabilities] = useState<LocalASRCapabilities | null>(
    null
  );
  const [enabled, setEnabled] = useState(false);
  const [selectedModel, setSelectedModel] = useState("");

  const loadCapabilities = async () => {
    try {
      const res = await fetchLocalASRCapabilities();
      if (res.code === 200) {
        setCapabilities(res.data);
        setEnabled(res.data.enabled);
        setSelectedModel(res.data.current_model || res.data.default_model || "");
      }
    } catch {
      // Ignore errors
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadCapabilities();
  }, []);

  const handleToggle = async () => {
    setSaving(true);
    try {
      const res = await updateLocalASRConfig({ enabled: !enabled });
      if (res.code === 200) {
        setEnabled(res.data.enabled);
        showToast("success", res.data.enabled ? "Local ASR enabled" : "Local ASR disabled");
        loadCapabilities();
      } else {
        showToast("error", res.message || "Failed to update settings");
      }
    } catch {
      showToast("error", "Failed to update settings");
    } finally {
      setSaving(false);
    }
  };

  const handleModelChange = async (model: string) => {
    setSaving(true);
    try {
      const res = await updateLocalASRConfig({ model });
      if (res.code === 200) {
        setSelectedModel(res.data.model);
        showToast("success", "Model updated");
      } else {
        showToast("error", res.message || "Failed to update model");
      }
    } catch {
      showToast("error", "Failed to update model");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
        <div className="text-sm text-zinc-500">{t.loading}</div>
      </div>
    );
  }

  const gpuAvailable = capabilities?.gpu?.type !== "none";
  const serviceAvailable = capabilities?.available;

  return (
    <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-white flex items-center gap-2">
          <FaRobot className="text-blue-500" />
          {t.local_asr_title || "Local Transcription"}
        </h2>
        <button
          onClick={handleToggle}
          disabled={!isConnected || saving || !serviceAvailable}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            enabled && serviceAvailable
              ? "bg-blue-500"
              : "bg-zinc-300 dark:bg-zinc-600"
          } ${
            !isConnected || saving || !serviceAvailable
              ? "opacity-50 cursor-not-allowed"
              : "cursor-pointer"
          }`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              enabled && serviceAvailable ? "translate-x-6" : "translate-x-1"
            }`}
          />
        </button>
      </div>

      {/* Service Status */}
      <div className="mb-4 p-3 bg-zinc-50 dark:bg-zinc-800 rounded-lg">
        <div className="flex items-center gap-2 mb-2">
          <FaServer
            className={serviceAvailable ? "text-green-500" : "text-red-500"}
          />
          <span className="text-sm font-medium text-zinc-700 dark:text-zinc-200">
            {t.local_asr_service || "ASR Service"}
          </span>
          <span
            className={`text-xs px-2 py-0.5 rounded ${
              serviceAvailable
                ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
                : "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300"
            }`}
          >
            {serviceAvailable
              ? t.local_asr_available || "Available"
              : t.local_asr_unavailable || "Not Available"}
          </span>
        </div>
        {!serviceAvailable && (
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            {t.local_asr_start_service ||
              "Start the ASR service with: docker compose up -d asr"}
          </p>
        )}
      </div>

      {/* GPU Status */}
      {serviceAvailable && capabilities?.gpu && (
        <div className="mb-4 p-3 bg-zinc-50 dark:bg-zinc-800 rounded-lg">
          <div className="flex items-center gap-2 mb-1">
            <FaMicrochip
              className={gpuAvailable ? "text-green-500" : "text-zinc-400"}
            />
            <span className="text-sm font-medium text-zinc-700 dark:text-zinc-200">
              {t.local_asr_gpu || "GPU"}
            </span>
            <span
              className={`text-xs px-2 py-0.5 rounded ${
                gpuAvailable
                  ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
                  : "bg-zinc-200 text-zinc-600 dark:bg-zinc-700 dark:text-zinc-400"
              }`}
            >
              {capabilities.gpu.type === "nvidia"
                ? "NVIDIA CUDA"
                : capabilities.gpu.type === "metal"
                ? "Apple Metal"
                : t.local_asr_cpu_only || "CPU Only"}
            </span>
          </div>
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            {capabilities.gpu.name}
            {capabilities.gpu.memory_gb
              ? ` (${capabilities.gpu.memory_gb}GB)`
              : ""}
            {capabilities.gpu.cuda_version
              ? ` - CUDA ${capabilities.gpu.cuda_version}`
              : ""}
          </p>
        </div>
      )}

      {/* Model Selection */}
      {serviceAvailable && capabilities?.models && (
        <div className="mb-4">
          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-2">
            {t.local_asr_model || "Transcription Model"}
          </label>
          <select
            value={selectedModel}
            onChange={(e) => handleModelChange(e.target.value)}
            disabled={!isConnected || saving || !enabled}
            className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 disabled:opacity-50 cursor-pointer"
          >
            {capabilities.models.map((model) => (
              <option key={model.id} value={model.id}>
                {model.name}
                {model.recommended_gpu && !gpuAvailable ? " (GPU recommended)" : ""}
              </option>
            ))}
          </select>
          {selectedModel && capabilities.models && (
            <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
              {capabilities.models.find((m) => m.id === selectedModel)?.description}
            </p>
          )}
        </div>
      )}

      {/* Info */}
      <div className="text-xs text-zinc-400 dark:text-zinc-600">
        {t.local_asr_info ||
          "Local transcription runs on your machine without sending data to external APIs."}
      </div>
    </div>
  );
}
