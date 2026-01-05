import { useState, useEffect, useCallback } from "react";
import clsx from "clsx";
import { useApp } from "../context/AppContext";
import {
  fetchAIConfig,
  fetchAIModels,
  fetchAudioFiles,
  uploadAudioFile,
  fetchLocalASRCapabilities,
  type AIConfigData,
  type AIModelsData,
  type AudioFile,
  type LocalASRCapabilities,
} from "../utils/apis";
import {
  FaSpinner,
  FaLock,
  FaGear,
  FaPlay,
  FaStop,
  FaFileLines,
  FaFolderOpen,
  FaDownload,
} from "react-icons/fa6";
import { Link } from "@tanstack/react-router";
import { FileSelector } from "../components/speech-to-text/FileSelector";
import { ProcessingStepper } from "../components/speech-to-text/ProcessingStepper";
import { useProcessing } from "../components/speech-to-text/useProcessing";

interface SelectedFile {
  path: string;
  filename: string;
  size: number;
  source: "downloaded" | "uploaded";
  has_transcript?: boolean;
  has_summary?: boolean;
}

export function SpeechToTextPage() {
  const { t, showToast } = useApp();
  const [aiConfig, setAIConfig] = useState<AIConfigData | null>(null);
  const [aiModels, setAIModels] = useState<AIModelsData | null>(null);
  const [downloadedFiles, setDownloadedFiles] = useState<AudioFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<SelectedFile | null>(null);

  // Model selection - unified format: "local:model" or "cloud:account:model"
  const [transcriptionSelection, setTranscriptionSelection] = useState("");
  const [summarizationSelection, setSummarizationSelection] = useState("");
  const [includeSummary, setIncludeSummary] = useState(true);
  const [audioLanguage, setAudioLanguage] = useState("zh"); // Language of the audio
  const [summaryLanguage, setSummaryLanguage] = useState("zh"); // Language for the summary output
  const [outputFormat, setOutputFormat] = useState("md"); // Output format: md, srt, vtt

  // Local ASR capabilities
  const [localASRCapabilities, setLocalASRCapabilities] =
    useState<LocalASRCapabilities | null>(null);

  // PIN handling
  const [pin, setPIN] = useState("");
  const [showPINInput, setShowPINInput] = useState(false);

  // Processing hook
  const {
    state: processingState,
    startProcessing,
    cancel,
    reset,
    isProcessing,
    isComplete,
    isError,
  } = useProcessing();

  const loadData = useCallback(async () => {
    try {
      const [aiRes, modelsRes, filesRes, localASRRes] = await Promise.all([
        fetchAIConfig(),
        fetchAIModels(),
        fetchAudioFiles(),
        fetchLocalASRCapabilities(),
      ]);

      let models: AIModelsData | null = null;
      if (modelsRes.code === 200) {
        models = modelsRes.data;
        setAIModels(models);
      }

      let localCaps: LocalASRCapabilities | null = null;
      if (localASRRes.code === 200 && localASRRes.data) {
        localCaps = localASRRes.data;
        setLocalASRCapabilities(localCaps);
      }

      if (aiRes.code === 200) {
        setAIConfig(aiRes.data);

        // Set default transcription selection
        // Prefer local if available and enabled
        if (
          localCaps?.available &&
          localCaps?.enabled &&
          localCaps.current_model
        ) {
          setTranscriptionSelection(localCaps.current_model); // e.g., "whisper-medium"
        } else {
          // Fall back to cloud
          const defaultAcc =
            aiRes.data.default_account || aiRes.data.accounts[0]?.label || "";
          if (defaultAcc && models) {
            const acc = aiRes.data.accounts.find((a) => a.label === defaultAcc);
            const provider = acc?.provider || "openai";
            const transcriptionModels =
              models.transcription[
                provider as keyof typeof models.transcription
              ] || [];
            if (transcriptionModels.length > 0) {
              setTranscriptionSelection(
                `${defaultAcc}:${transcriptionModels[0]}`
              ); // e.g., "my-openai:whisper-1"
            }
          }
        }

        // Set default summarization selection
        const defaultAcc =
          aiRes.data.default_account || aiRes.data.accounts[0]?.label || "";
        if (defaultAcc && models) {
          const acc = aiRes.data.accounts.find((a) => a.label === defaultAcc);
          const provider = acc?.provider || "openai";
          const summaryModels =
            models.summarization[provider as "openai" | "anthropic" | "qwen"] ||
            [];
          if (summaryModels.length > 0) {
            setSummarizationSelection(`${defaultAcc}:${summaryModels[0].id}`);
          }
        }
      }

      if (filesRes.code === 200) {
        setDownloadedFiles(filesRes.data.files || []);
      }
    } catch (e) {
      console.error("Failed to load data:", e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Reload files when processing completes
  useEffect(() => {
    if (isComplete) {
      loadData();
      showToast("success", "Processing completed!");
    }
  }, [isComplete, loadData, showToast]);

  // Show error toast
  useEffect(() => {
    if (isError && processingState.error) {
      showToast("error", processingState.error);
    }
  }, [isError, processingState.error, showToast]);

  const hasAIAccount = aiConfig && aiConfig.accounts.length > 0;
  const hasLocalASR =
    localASRCapabilities?.available && localASRCapabilities?.enabled;
  const canProcess = hasAIAccount || hasLocalASR;

  // Parse transcription selection: "model" (local) or "account:model" (cloud)
  const parseTranscriptionSelection = (sel: string) => {
    if (sel.includes(":")) {
      // Cloud model: "account:model"
      const [account, model] = sel.split(":");
      return { type: "cloud" as const, account, model };
    }
    // Local model: just "model"
    return { type: "local" as const, model: sel, account: "" };
  };

  // Parse summarization selection: "account:model"
  const parseSummarizationSelection = (sel: string) => {
    const parts = sel.split(":");
    return { account: parts[0] || "", model: parts[1] || "" };
  };

  const getAccountEncrypted = (label: string) => {
    const acc = aiConfig?.accounts.find((a) => a.label === label);
    return acc?.is_encrypted ?? true;
  };

  // Build transcription options: local models + cloud models
  const getTranscriptionOptions = () => {
    const options: { value: string; label: string }[] = [];

    // Local models
    if (localASRCapabilities?.models) {
      for (const m of localASRCapabilities.models) {
        options.push({
          value: m.name, // e.g., "whisper-medium"
          label: `Local: ${m.name}${m.downloaded ? " (available)" : ""}`,
        });
      }
    }

    // Cloud models
    if (aiConfig?.accounts && aiModels?.transcription) {
      for (const acc of aiConfig.accounts) {
        const models =
          aiModels.transcription[
            acc.provider as keyof typeof aiModels.transcription
          ] || [];
        for (const model of models) {
          options.push({
            value: `${acc.label}:${model}`, // e.g., "my-openai:whisper-1"
            label: `Cloud: ${model} (${acc.label})`,
          });
        }
      }
    }

    return options;
  };

  // Build summarization options: models from all accounts
  const getSummarizationOptions = () => {
    const options: { value: string; label: string }[] = [];

    if (aiConfig?.accounts && aiModels?.summarization) {
      for (const acc of aiConfig.accounts) {
        const models =
          aiModels.summarization[
            acc.provider as "openai" | "anthropic" | "qwen"
          ] || [];
        for (const model of models) {
          options.push({
            value: `${acc.label}:${model.id}`,
            label: `${acc.label} - ${model.name}`,
          });
        }
      }
    }

    return options;
  };

  const handleSelectFile = (file: AudioFile) => {
    setSelectedFile({
      path: file.full_path,
      filename: file.name,
      size: file.size,
      source: "downloaded",
      has_transcript: file.has_transcript,
      has_summary: file.has_summary,
    });
    reset();
  };

  const handleUpload = async (file: File) => {
    setUploading(true);
    try {
      const res = await uploadAudioFile(file);
      if (res.code === 200) {
        setSelectedFile({
          path: res.data.path,
          filename: res.data.filename,
          size: res.data.size,
          source: "uploaded",
        });
        showToast("success", `Uploaded: ${res.data.filename}`);
        reset();
      } else {
        showToast("error", res.message || "Upload failed");
      }
    } catch {
      showToast("error", "Upload failed");
    } finally {
      setUploading(false);
    }
  };

  const handleStartProcessing = async (pinToUse?: string) => {
    if (!selectedFile) return;

    const transcription = parseTranscriptionSelection(transcriptionSelection);
    const summarization = parseSummarizationSelection(summarizationSelection);
    const isLocalModel = transcription.type === "local";

    // Determine which accounts need PIN
    const accountsNeedingPin = new Set<string>();
    if (!isLocalModel && transcription.account) {
      if (getAccountEncrypted(transcription.account)) {
        accountsNeedingPin.add(transcription.account);
      }
    }
    if (includeSummary && summarization.account) {
      if (getAccountEncrypted(summarization.account)) {
        accountsNeedingPin.add(summarization.account);
      }
    }

    const needsPin = accountsNeedingPin.size > 0;
    if (needsPin && !pinToUse) {
      setShowPINInput(true);
      setPIN("");
      return;
    }

    setShowPINInput(false);

    // For local models, use summarization account (if summarizing)
    // For cloud models, use transcription account
    const account = isLocalModel
      ? includeSummary
        ? summarization.account
        : ""
      : transcription.account;

    const result = await startProcessing(
      selectedFile.path,
      account,
      transcription.model,
      summarization.model,
      includeSummary,
      pinToUse,
      audioLanguage,
      outputFormat,
      summaryLanguage
    );

    if (!result.success) {
      if (result.error?.includes("PIN") || result.error?.includes("decrypt")) {
        showToast("error", "Incorrect PIN");
      }
    }

    setPIN("");
  };

  const submitPIN = () => {
    handleStartProcessing(pin);
  };

  const getBaseName = (filename: string) => {
    return filename.replace(/\.[^.]+$/, "");
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <FaSpinner className="animate-spin text-2xl text-zinc-400" />
      </div>
    );
  }

  if (!canProcess) {
    return (
      <div className="max-w-4xl mx-auto flex flex-col gap-4">
        <h1 className="text-xl font-medium text-zinc-900 dark:text-white">
          {t.ai_speech_to_text}
        </h1>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4 flex flex-col gap-3">
          <div className="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
            <FaGear />
            <span className="font-medium">Setup Required</span>
          </div>
          <p className="text-yellow-700 dark:text-yellow-300 text-sm">
            Enable Local ASR or configure an AI account in Settings to use
            transcription features.
          </p>
          <Link
            to="/ai/settings"
            className="inline-flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:underline"
          >
            <FaGear /> {t.ai_settings}
          </Link>
        </div>
      </div>
    );
  }

  const selectClass =
    "px-2.5 py-1.5 border border-zinc-300 dark:border-zinc-600 rounded-md bg-white dark:bg-zinc-700 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500";

  return (
    <div className="max-w-4xl mx-auto flex flex-col gap-4 pb-8">
      <h1 className="text-xl font-medium text-zinc-900 dark:text-white shrink-0">
        {t.ai_speech_to_text}
      </h1>

      {/* PIN Modal */}
      {showPINInput && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-zinc-800 rounded-lg p-6 w-80 shadow-xl">
            <div className="flex items-center gap-2 mb-4">
              <FaLock className="text-zinc-500" />
              <h3 className="font-medium text-zinc-900 dark:text-white">
                Enter PIN
              </h3>
            </div>
            <input
              type="password"
              maxLength={4}
              value={pin}
              onChange={(e) => setPIN(e.target.value.replace(/\D/g, ""))}
              placeholder="4-digit PIN"
              className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-600 rounded-lg bg-white dark:bg-zinc-700 text-zinc-900 dark:text-white mb-4 text-center text-2xl tracking-widest"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter" && pin.length === 4) {
                  submitPIN();
                }
              }}
            />
            <div className="flex gap-2">
              <button
                onClick={() => setShowPINInput(false)}
                className="flex-1 px-4 py-2 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-zinc-700"
              >
                {t.cancel}
              </button>
              <button
                onClick={submitPIN}
                disabled={pin.length !== 4}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {t.save}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Section 1: File List */}
      <div className="flex-1 min-h-50 h-fit">
        <FileSelector
          files={downloadedFiles}
          selectedPath={selectedFile?.path || null}
          onSelect={handleSelectFile}
          onUpload={handleUpload}
          uploading={uploading}
          disabled={isProcessing}
        />
      </div>

      {/* Section 2: Model Configuration */}
      <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4 shrink-0">
        <h3 className="text-sm font-medium text-zinc-900 dark:text-white mb-3">
          {t.ai_settings}
        </h3>

        <div className="space-y-3">
          {/* Transcription Model - unified dropdown */}
          <div className="flex items-center justify-between gap-4">
            <label className="text-sm text-zinc-600 dark:text-zinc-400 w-20">
              {t.ai_transcription_model}:
            </label>
            <select
              value={transcriptionSelection}
              onChange={(e) => setTranscriptionSelection(e.target.value)}
              className={clsx(selectClass, "flex-1")}
              disabled={isProcessing}
            >
              {getTranscriptionOptions().map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          {/* Audio Language */}
          <div className="flex items-center justify-between gap-4">
            <label className="text-sm text-zinc-600 dark:text-zinc-400 w-20">
              {t.ai_audio_language}:
            </label>
            <select
              value={audioLanguage}
              onChange={(e) => setAudioLanguage(e.target.value)}
              className={clsx(selectClass, "flex-1")}
              disabled={isProcessing}
            >
              <option value="zh">Chinese (中文)</option>
              <option value="en">English</option>
              <option value="ja">Japanese (日本語)</option>
              <option value="ko">Korean (한국어)</option>
              <option value="es">Spanish (Español)</option>
              <option value="fr">French (Français)</option>
              <option value="de">German (Deutsch)</option>
              <option value="ru">Russian (Русский)</option>
              <option value="pt">Portuguese (Português)</option>
              <option value="ar">Arabic (العربية)</option>
            </select>
          </div>

          {/* Summarization Model - unified dropdown */}
          <div className="flex items-center justify-between gap-4">
            <label className="text-sm text-zinc-600 dark:text-zinc-400 w-20">
              {t.ai_summary_model}:
            </label>
            <select
              value={summarizationSelection}
              onChange={(e) => setSummarizationSelection(e.target.value)}
              className={clsx(
                selectClass,
                "flex-1",
                !includeSummary && "opacity-50"
              )}
              disabled={isProcessing || !includeSummary}
            >
              {getSummarizationOptions().map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            <input
              type="checkbox"
              checked={includeSummary}
              onChange={(e) => setIncludeSummary(e.target.checked)}
              disabled={isProcessing}
              className="w-4 h-4 rounded border-zinc-300 dark:border-zinc-600 text-blue-600 focus:ring-blue-500 cursor-pointer"
            />
          </div>

          {/* Summary Language */}
          <div className="flex items-center justify-between gap-4">
            <label className="text-sm text-zinc-600 dark:text-zinc-400 w-20">
              {t.ai_summary_language}:
            </label>
            <select
              value={summaryLanguage}
              onChange={(e) => setSummaryLanguage(e.target.value)}
              className={clsx(
                selectClass,
                "flex-1",
                !includeSummary && "opacity-50"
              )}
              disabled={isProcessing || !includeSummary}
            >
              <option value="zh">Chinese (中文)</option>
              <option value="en">English</option>
              <option value="ja">Japanese (日本語)</option>
              <option value="ko">Korean (한국어)</option>
              <option value="es">Spanish (Español)</option>
              <option value="fr">French (Français)</option>
              <option value="de">German (Deutsch)</option>
            </select>
          </div>

          {/* Output Format */}
          <div className="flex items-center justify-between gap-4">
            <label className="text-sm text-zinc-600 dark:text-zinc-400 w-20">
              {t.ai_output_format || "Output Format"}:
            </label>
            <select
              value={outputFormat}
              onChange={(e) => setOutputFormat(e.target.value)}
              className={clsx(selectClass, "flex-1")}
              disabled={isProcessing}
            >
              <option value="md">Markdown (.md)</option>
              <option value="srt">SRT Subtitles (.srt)</option>
              <option value="vtt">WebVTT (.vtt)</option>
            </select>
          </div>

        </div>

        {/* Start/Cancel Button */}
        <div className="flex gap-4 mt-4">
          {isProcessing ? (
            <button
              onClick={cancel}
              className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
            >
              <FaStop />
              {t.cancel}
            </button>
          ) : (
            <button
              onClick={() => handleStartProcessing()}
              disabled={!selectedFile || !transcriptionSelection}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              <FaPlay />
              {t.ai_run}
            </button>
          )}
        </div>
      </div>

      {/* Section 3: Processing Steps */}
      <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4 shrink-0">
        <h3 className="text-sm font-medium text-zinc-900 dark:text-white mb-3">
          {isProcessing ? t.ai_processing : t.ai_processing_steps}
        </h3>
        <ProcessingStepper
          steps={processingState.steps}
          currentStepIndex={processingState.currentStepIndex}
          overallProgress={processingState.overallProgress}
          isProcessing={isProcessing}
          emptyText={t.ai_select_file_hint}
          translations={t}
        />
      </div>

      {/* Section 4: Outputs - always rendered to maintain consistent layout */}
      <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4 shrink-0 min-h-30">
        <h2 className="font-medium text-zinc-900 dark:text-white mb-3">
          {t.ai_outputs}
        </h2>

        {selectedFile &&
        (selectedFile.has_transcript ||
          selectedFile.has_summary ||
          isComplete) ? (
          <div className="space-y-2">
            {/* Chunks directory */}
            <div className="flex items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
              <FaFolderOpen className="text-yellow-500 shrink-0" />
              <span>{getBaseName(selectedFile.filename)}.chunks/</span>
              <span className="text-xs text-zinc-400">(audio chunks)</span>
            </div>

            {/* Transcript file */}
            {(selectedFile.has_transcript ||
              processingState.result?.transcript_path) && (
              <a
                href={`/api/download?path=${encodeURIComponent(
                  selectedFile.path.replace(/\.[^.]+$/, ".transcript.md")
                )}`}
                download
                className="flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:underline cursor-pointer"
              >
                <FaFileLines className="text-blue-500 shrink-0" />
                <span>{getBaseName(selectedFile.filename)}.transcript.md</span>
                <FaDownload className="text-xs" />
              </a>
            )}

            {/* Summary file */}
            {(selectedFile.has_summary ||
              processingState.result?.summary_path) && (
              <a
                href={`/api/download?path=${encodeURIComponent(
                  selectedFile.path.replace(/\.[^.]+$/, ".summary.md")
                )}`}
                download
                className="flex items-center gap-2 text-sm text-purple-600 dark:text-purple-400 hover:underline cursor-pointer"
              >
                <FaFileLines className="text-purple-500 shrink-0" />
                <span>{getBaseName(selectedFile.filename)}.summary.md</span>
                <FaDownload className="text-xs" />
              </a>
            )}
          </div>
        ) : (
          <div className="text-sm text-zinc-400 dark:text-zinc-500">
            {selectedFile ? t.ai_no_outputs_yet : t.ai_select_file_for_outputs}
          </div>
        )}
      </div>
    </div>
  );
}
