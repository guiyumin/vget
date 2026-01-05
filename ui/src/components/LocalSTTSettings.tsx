import { useState, useEffect } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchLocalASRCapabilities,
  updateLocalASRConfig,
  type LocalASRCapabilities,
} from "../utils/apis";
import { FaMicrochip, FaRobot, FaCircleCheck, FaCircleXmark } from "react-icons/fa6";

interface LocalSTTSettingsProps {
  isConnected: boolean;
}

export function LocalSTTSettings({ isConnected }: LocalSTTSettingsProps) {
  const { t, showToast } = useApp();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [capabilities, setCapabilities] = useState<LocalASRCapabilities | null>(
    null
  );
  const [selectedModel, setSelectedModel] = useState("");

  const loadCapabilities = async () => {
    try {
      const res = await fetchLocalASRCapabilities();
      if (res.code === 200) {
        setCapabilities(res.data);
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

  const handleModelChange = async (model: string) => {
    setSaving(true);
    try {
      const res = await updateLocalASRConfig({ model });
      if (res.code === 200) {
        setSelectedModel(res.data.model);
        showToast("success", t.local_stt_model_updated);
      } else {
        showToast("error", res.message || t.local_stt_update_failed);
      }
    } catch {
      showToast("error", t.local_stt_update_failed);
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

  const gpuAvailable = capabilities?.gpu?.type === "nvidia";
  const isAvailable = capabilities?.available && gpuAvailable;

  return (
    <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-white flex items-center gap-2">
          <FaRobot className="text-blue-500" />
          {t.local_stt_title}
        </h2>
        {/* Status indicator - read-only, system determined */}
        <span
          className={`flex items-center gap-1 text-xs px-2 py-1 rounded ${
            isAvailable
              ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
              : "bg-zinc-200 text-zinc-600 dark:bg-zinc-700 dark:text-zinc-400"
          }`}
        >
          {isAvailable ? (
            <>
              <FaCircleCheck className="text-green-500" />
              {t.local_stt_available}
            </>
          ) : (
            <>
              <FaCircleXmark className="text-zinc-400" />
              {t.local_stt_unavailable}
            </>
          )}
        </span>
      </div>

      {/* GPU Status */}
      <div className="mb-4 p-3 bg-zinc-50 dark:bg-zinc-800 rounded-lg">
        <div className="flex items-center gap-2 mb-1">
          <FaMicrochip
            className={gpuAvailable ? "text-green-500" : "text-zinc-400"}
          />
          <span className="text-sm font-medium text-zinc-700 dark:text-zinc-200">
            GPU
          </span>
          <span
            className={`text-xs px-2 py-0.5 rounded ${
              gpuAvailable
                ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
                : "bg-zinc-200 text-zinc-600 dark:bg-zinc-700 dark:text-zinc-400"
            }`}
          >
            {gpuAvailable ? "NVIDIA CUDA" : t.local_stt_no_gpu}
          </span>
        </div>
        {gpuAvailable && capabilities?.gpu && (
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            {capabilities.gpu.name}
            {capabilities.gpu.memory_gb
              ? ` (${capabilities.gpu.memory_gb}GB)`
              : ""}
            {capabilities.gpu.cuda_version
              ? ` - CUDA ${capabilities.gpu.cuda_version}`
              : ""}
          </p>
        )}
        {!gpuAvailable && (
          <p className="text-xs text-zinc-500 dark:text-zinc-400 mt-1">
            {t.local_stt_gpu_required}
          </p>
        )}
      </div>

      {/* Model Selection - only show when available */}
      {isAvailable && capabilities?.models && (
        <div className="mb-4">
          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-2">
            {t.local_stt_model}
          </label>
          <select
            value={selectedModel}
            onChange={(e) => handleModelChange(e.target.value)}
            disabled={!isConnected || saving}
            className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 disabled:opacity-50 cursor-pointer"
          >
            {capabilities.models.map((model) => (
              <option key={model.name} value={model.name}>
                {model.name} ({model.size}){model.downloaded ? " ✓" : ""}
              </option>
            ))}
          </select>
          {selectedModel && capabilities.models && (
            <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
              {capabilities.models.find((m) => m.name === selectedModel)?.description}
            </p>
          )}
        </div>
      )}

      {/* Info */}
      <div className="text-xs text-zinc-400 dark:text-zinc-500">
        {t.local_stt_info}
      </div>
    </div>
  );
}
