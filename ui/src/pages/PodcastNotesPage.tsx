import { useState, useEffect, useCallback } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchAIConfig,
  fetchAIModels,
  fetchAudioFiles,
  uploadAudioFile,
  type AIConfigData,
  type AIModelsData,
  type AudioFile,
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
import { FileSelector } from "../components/podcast-notes/FileSelector";
import { ProcessingStepper } from "../components/podcast-notes/ProcessingStepper";
import { useProcessing } from "../components/podcast-notes/useProcessing";

interface SelectedFile {
  path: string;
  filename: string;
  size: number;
  source: "downloaded" | "uploaded";
  has_transcript?: boolean;
  has_summary?: boolean;
}

export function PodcastNotesPage() {
  const { t, showToast } = useApp();
  const [aiConfig, setAIConfig] = useState<AIConfigData | null>(null);
  const [aiModels, setAIModels] = useState<AIModelsData | null>(null);
  const [downloadedFiles, setDownloadedFiles] = useState<AudioFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<SelectedFile | null>(null);

  // Account and model selection
  const [account, setAccount] = useState("");
  const [transcriptionModel, setTranscriptionModel] = useState("");
  const [summarizationModel, setSummarizationModel] = useState("");
  const [includeSummary, setIncludeSummary] = useState(true);

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
      const [aiRes, modelsRes, filesRes] = await Promise.all([
        fetchAIConfig(),
        fetchAIModels(),
        fetchAudioFiles(),
      ]);

      let models: AIModelsData | null = null;
      if (modelsRes.code === 200) {
        models = modelsRes.data;
        setAIModels(models);
      }

      if (aiRes.code === 200) {
        setAIConfig(aiRes.data);
        const defaultAcc = aiRes.data.default_account || "";
        if (defaultAcc) {
          setAccount(defaultAcc);
          const acc = aiRes.data.accounts.find((a) => a.label === defaultAcc);
          const provider = acc?.provider || "openai";
          if (models) {
            // Set default transcription model
            const transcriptionModels =
              models.transcription[
                provider as keyof typeof models.transcription
              ] || [];
            setTranscriptionModel(transcriptionModels[0] || "whisper-1");

            // Set default summarization model
            const summaryProvider = provider as "openai" | "anthropic" | "qwen";
            const summaryModels = models.summarization[summaryProvider];
            setSummarizationModel(
              summaryModels?.[0]?.id || models.summarization.default
            );
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

  const getAccount = (label: string) => {
    return aiConfig?.accounts.find((acc) => acc.label === label);
  };

  const getAccountEncrypted = (label: string) => {
    return getAccount(label)?.is_encrypted ?? true;
  };

  const getAccountProvider = (label: string) => {
    return getAccount(label)?.provider || "openai";
  };

  const getTranscriptionModels = (provider: string) => {
    return (
      aiModels?.transcription[
        provider as keyof typeof aiModels.transcription
      ] || []
    );
  };

  const getSummarizationModels = (provider: string) => {
    const p = provider as "openai" | "anthropic" | "qwen";
    return aiModels?.summarization[p] || [];
  };

  const handleAccountChange = (accountName: string) => {
    setAccount(accountName);
    const provider = getAccountProvider(accountName);

    // Update transcription model
    const transcriptionModels = getTranscriptionModels(provider);
    setTranscriptionModel(transcriptionModels[0] || "whisper-1");

    // Update summarization model
    const summaryModels = getSummarizationModels(provider);
    setSummarizationModel(
      summaryModels[0]?.id || aiModels?.summarization.default || ""
    );
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

    const isEncrypted = getAccountEncrypted(account);
    if (isEncrypted && !pinToUse) {
      setShowPINInput(true);
      setPIN("");
      return;
    }

    setShowPINInput(false);

    const result = await startProcessing(
      selectedFile.path,
      account,
      transcriptionModel,
      summarizationModel,
      includeSummary,
      pinToUse
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

  if (!hasAIAccount) {
    return (
      <div className="max-w-4xl mx-auto flex flex-col gap-4">
        <h1 className="text-xl font-medium text-zinc-900 dark:text-white">
          {t.ai_speech_to_text}
        </h1>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4 flex flex-col gap-3">
          <div className="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
            <FaGear />
            <span className="font-medium">AI Account Required</span>
          </div>
          <p className="text-yellow-700 dark:text-yellow-300 text-sm">
            Configure an AI account in Settings to use transcription and
            summarization features.
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
          {t.ai_settings || "Settings"}
        </h3>

        <div className="grid grid-cols-2 gap-4">
          {/* Left: Account & Models */}
          <div className="space-y-3">
            {/* Account Selection */}
            <div className="flex items-center gap-2">
              <label className="text-sm text-zinc-600 dark:text-zinc-400 w-28">
                {t.ai_account_name || "Account"}:
              </label>
              <select
                value={account}
                onChange={(e) => handleAccountChange(e.target.value)}
                className={`${selectClass} flex-1`}
                disabled={isProcessing}
              >
                {aiConfig?.accounts.map((acc) => (
                  <option key={acc.label} value={acc.label}>
                    {acc.label} - {acc.provider}
                  </option>
                ))}
              </select>
            </div>

            {/* Transcription Model */}
            <div className="flex items-center gap-2">
              <label className="text-sm text-zinc-600 dark:text-zinc-400 w-28">
                {t.ai_transcription_model || "Transcription"}:
              </label>
              <select
                value={transcriptionModel}
                onChange={(e) => setTranscriptionModel(e.target.value)}
                className={`${selectClass} flex-1`}
                disabled={isProcessing}
              >
                {getTranscriptionModels(getAccountProvider(account)).map(
                  (m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  )
                )}
              </select>
            </div>

            {/* Summarization Model */}
            <div className="flex items-center gap-2">
              <label className="text-sm text-zinc-600 dark:text-zinc-400 w-28">
                {t.ai_summary_model || "Summarization"}:
              </label>
              <select
                value={summarizationModel}
                onChange={(e) => setSummarizationModel(e.target.value)}
                className={`${selectClass} flex-1`}
                disabled={isProcessing || !includeSummary}
              >
                {getSummarizationModels(getAccountProvider(account)).map(
                  (m) => (
                    <option key={m.id} value={m.id}>
                      {m.name}
                    </option>
                  )
                )}
              </select>
            </div>
          </div>

          {/* Right: Options & Button */}
          <div className="flex flex-col justify-between">
            {/* Include Summary Toggle */}
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={includeSummary}
                onChange={(e) => setIncludeSummary(e.target.checked)}
                disabled={isProcessing}
                className="w-4 h-4 rounded border-zinc-300 dark:border-zinc-600 text-blue-600 focus:ring-blue-500"
              />
              <span className="text-sm text-zinc-600 dark:text-zinc-400">
                {t.ai_summarize || "Include Summary"}
              </span>
            </label>

            {/* Start/Cancel Button */}
            <div className="flex gap-2 mt-auto">
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
                  disabled={!selectedFile}
                  className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  <FaPlay />
                  {t.ai_run || "Start Processing"}
                </button>
              )}
            </div>
          </div>
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
            {selectedFile
              ? t.ai_no_outputs_yet
              : t.ai_select_file_for_outputs}
          </div>
        )}
      </div>
    </div>
  );
}
