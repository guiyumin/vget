import { useState, useEffect, useCallback, useRef } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchAIConfig,
  fetchAIModels,
  fetchAudioFiles,
  uploadAudioFile,
  transcribeAudio,
  summarizeText,
  type AIConfigData,
  type AIModelsData,
  type AudioFile,
} from "../utils/apis";
import {
  FaMicrophone,
  FaFileLines,
  FaSpinner,
  FaLock,
  FaGear,
  FaUpload,
  FaCheck,
  FaVideo,
  FaMusic,
  FaFolderOpen,
  FaDownload,
} from "react-icons/fa6";
import { Link } from "@tanstack/react-router";
import clsx from "clsx";

const VIDEO_EXTENSIONS = [".mp4", ".webm", ".mkv", ".avi", ".mov", ".flv", ".wmv"];

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
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [aiConfig, setAIConfig] = useState<AIConfigData | null>(null);
  const [aiModels, setAIModels] = useState<AIModelsData | null>(null);
  const [downloadedFiles, setDownloadedFiles] = useState<AudioFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<SelectedFile | null>(null);
  const [processing, setProcessing] = useState<
    "transcribe" | "summarize" | null
  >(null);

  // Account and model selection for each step
  const [transcribeAccount, setTranscribeAccount] = useState("");
  const [transcribeModel, setTranscribeModel] = useState("");
  const [summarizeAccount, setSummarizeAccount] = useState("");
  const [summarizeModel, setSummarizeModel] = useState("");

  // PIN handling
  const [pin, setPIN] = useState("");
  const [showPINInput, setShowPINInput] = useState(false);
  const [pendingAction, setPendingAction] = useState<
    "transcribe" | "summarize" | null
  >(null);

  // Results
  const [transcript, setTranscript] = useState<string | null>(null);
  const [summary, setSummary] = useState<{
    summary: string;
    keyPoints: string[];
  } | null>(null);

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
          setTranscribeAccount(defaultAcc);
          setSummarizeAccount(defaultAcc);
          const account = aiRes.data.accounts.find(
            (acc) => acc.label === defaultAcc
          );
          const provider = account?.provider || "openai";
          if (models) {
            const transcriptionProvider = provider as keyof typeof models.transcription;
            setTranscribeModel(
              models.transcription[transcriptionProvider]?.[0] || "whisper-1"
            );
            const defaultModel = models.summarization.default || "gpt-5-nano";
            const summaryProvider = provider as "openai" | "anthropic" | "qwen";
            const summaryModels = models.summarization[summaryProvider];
            setSummarizeModel(summaryModels?.[0]?.id || defaultModel);
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
    const p = provider as keyof NonNullable<typeof aiModels>["transcription"];
    return aiModels?.transcription[p] || [];
  };

  const getSummarizationModels = (provider: string) => {
    const p = provider as "openai" | "anthropic" | "qwen";
    return aiModels?.summarization[p] || [];
  };

  const handleTranscribeAccountChange = (accountName: string) => {
    setTranscribeAccount(accountName);
    const provider = getAccountProvider(accountName) as keyof NonNullable<typeof aiModels>["transcription"];
    setTranscribeModel(aiModels?.transcription[provider]?.[0] || "whisper-1");
  };

  const handleSummarizeAccountChange = (accountName: string) => {
    setSummarizeAccount(accountName);
    const provider = getAccountProvider(accountName) as "openai" | "anthropic" | "qwen";
    const defaultModel = aiModels?.summarization.default || "gpt-5-nano";
    setSummarizeModel(aiModels?.summarization[provider]?.[0]?.id || defaultModel);
  };

  const handleSelectDownloadedFile = (file: AudioFile) => {
    setSelectedFile({
      path: file.full_path,
      filename: file.name,
      size: file.size,
      source: "downloaded",
      has_transcript: file.has_transcript,
      has_summary: file.has_summary,
    });
    setTranscript(null);
    setSummary(null);
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploading(true);
    setTranscript(null);
    setSummary(null);

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
      } else {
        showToast("error", res.message || "Upload failed");
      }
    } catch {
      showToast("error", "Upload failed");
    } finally {
      setUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  const handleTranscribe = useCallback(
    async (pinToUse?: string) => {
      if (!selectedFile) return;
      const isEncrypted = getAccountEncrypted(transcribeAccount);
      if (isEncrypted && !pinToUse) return;

      setProcessing("transcribe");
      setShowPINInput(false);

      try {
        const res = await transcribeAudio({
          file_path: selectedFile.path,
          account: transcribeAccount,
          model: transcribeModel,
          pin: pinToUse || undefined,
        });

        if (res.code === 200) {
          setTranscript(res.data.text);
          showToast("success", "Transcription completed");
          loadData();
        } else if (res.code === 401) {
          showToast("error", "Incorrect PIN");
        } else {
          showToast("error", res.message || "Transcription failed");
        }
      } catch {
        showToast("error", "Transcription failed");
      } finally {
        setProcessing(null);
        setPIN("");
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedFile, transcribeAccount, transcribeModel, showToast, loadData]
  );

  const handleSummarize = useCallback(
    async (pinToUse?: string) => {
      if (!selectedFile) return;
      const isEncrypted = getAccountEncrypted(summarizeAccount);
      if (isEncrypted && !pinToUse) return;

      setProcessing("summarize");
      setShowPINInput(false);

      try {
        // Always use file_path to ensure summary is written next to the original file
        // Use the transcript file if it exists, otherwise use the original file path
        const transcriptPath = selectedFile.path.replace(
          /\.[^.]+$/,
          ".transcript.md"
        );

        const res = await summarizeText({
          file_path: selectedFile.has_transcript || transcript ? transcriptPath : selectedFile.path,
          account: summarizeAccount,
          model: summarizeModel,
          pin: pinToUse || undefined,
        });

        if (res.code === 200) {
          setSummary({
            summary: res.data.summary,
            keyPoints: res.data.key_points,
          });
          showToast("success", "Summarization completed");
          loadData();
        } else if (res.code === 401) {
          showToast("error", "Incorrect PIN");
        } else {
          showToast("error", res.message || "Summarization failed");
        }
      } catch {
        showToast("error", "Summarization failed");
      } finally {
        setProcessing(null);
        setPIN("");
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedFile, transcript, summarizeAccount, summarizeModel, showToast, loadData]
  );

  const requestAction = (action: "transcribe" | "summarize") => {
    const accountName =
      action === "transcribe" ? transcribeAccount : summarizeAccount;
    const isEncrypted = getAccountEncrypted(accountName);

    if (isEncrypted) {
      setPendingAction(action);
      setShowPINInput(true);
      setPIN("");
    } else {
      if (action === "transcribe") {
        handleTranscribe();
      } else {
        handleSummarize();
      }
    }
  };

  const submitPIN = () => {
    if (pendingAction === "transcribe") {
      handleTranscribe(pin);
    } else if (pendingAction === "summarize") {
      handleSummarize(pin);
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const isVideoFile = (filename: string) => {
    const ext = filename.toLowerCase().slice(filename.lastIndexOf("."));
    return VIDEO_EXTENSIONS.includes(ext);
  };

  const getBaseName = (filename: string) => {
    return filename.replace(/\.[^.]+$/, "");
  };

  const canSummarize = selectedFile?.has_transcript || transcript;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <FaSpinner className="animate-spin text-2xl text-zinc-400" />
      </div>
    );
  }

  if (!hasAIAccount) {
    return (
      <div className="max-w-3xl mx-auto flex flex-col gap-4">
        <h1 className="text-xl font-medium text-zinc-900 dark:text-white">
          {t.ai_podcast_notes}
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
    "px-3 py-2 border border-zinc-300 dark:border-zinc-600 rounded-lg bg-white dark:bg-zinc-700 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500";

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-6">
      <h1 className="text-xl font-medium text-zinc-900 dark:text-white">
        {t.ai_podcast_notes}
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
                Cancel
              </button>
              <button
                onClick={submitPIN}
                disabled={pin.length !== 4}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Confirm
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Section 1: File Selection */}
      <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 overflow-hidden">
        <input
          ref={fileInputRef}
          type="file"
          accept=".mp3,.m4a,.wav,.aac,.ogg,.flac,.opus,.wma,.mp4,.webm,.mkv,.avi,.mov,.flv,.wmv"
          onChange={handleFileUpload}
          className="hidden"
        />
        <div className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-700 flex items-center justify-between">
          <h2 className="font-medium text-zinc-900 dark:text-white">
            1. Select Media File
          </h2>
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {uploading ? (
              <>
                <FaSpinner className="animate-spin" />
                Uploading...
              </>
            ) : (
              <>
                <FaUpload />
                Upload
              </>
            )}
          </button>
        </div>
        <div className="max-h-64 overflow-y-auto">
          {downloadedFiles.length === 0 ? (
            <div className="p-4 text-center text-zinc-500 dark:text-zinc-400 text-sm">
              No media files found
            </div>
          ) : (
            <div className="divide-y divide-zinc-200 dark:divide-zinc-700">
              {downloadedFiles.map((file) => (
                <button
                  key={file.path}
                  onClick={() => handleSelectDownloadedFile(file)}
                  className={clsx(
                    "w-full px-4 py-3 text-left hover:bg-zinc-50 dark:hover:bg-zinc-700/50 transition-colors",
                    selectedFile?.path === file.full_path &&
                      "bg-blue-50 dark:bg-blue-900/20"
                  )}
                >
                  <div className="flex items-start gap-3">
                    {isVideoFile(file.name) ? (
                      <FaVideo className="text-purple-500 mt-1 shrink-0" />
                    ) : (
                      <FaMusic className="text-blue-500 mt-1 shrink-0" />
                    )}
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-zinc-900 dark:text-white truncate">
                        {file.name}
                      </div>
                      <div className="text-xs text-zinc-500 dark:text-zinc-400 mt-0.5">
                        {formatFileSize(file.size)}
                      </div>
                      <div className="flex gap-2 mt-1">
                        {file.has_transcript && (
                          <span className="inline-flex items-center gap-1 text-xs text-green-600 dark:text-green-400">
                            <FaCheck className="text-[10px]" /> Transcript
                          </span>
                        )}
                        {file.has_summary && (
                          <span className="inline-flex items-center gap-1 text-xs text-green-600 dark:text-green-400">
                            <FaCheck className="text-[10px]" /> Summary
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Section 2: AI Configuration & Actions */}
      {selectedFile && (
        <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
          <h2 className="font-medium text-zinc-900 dark:text-white mb-4">
            2. {t.ai_select_model}
          </h2>

          {/* Selected file info */}
          <div className="flex items-center gap-3 p-3 bg-zinc-50 dark:bg-zinc-700/50 rounded-lg mb-4">
            {isVideoFile(selectedFile.filename) ? (
              <FaVideo className="text-purple-500" />
            ) : (
              <FaMusic className="text-blue-500" />
            )}
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium text-zinc-900 dark:text-white truncate">
                {selectedFile.filename}
              </div>
              <div className="text-xs text-zinc-500 dark:text-zinc-400">
                {formatFileSize(selectedFile.size)}
              </div>
            </div>
          </div>

          {/* Transcription row */}
          <div className="flex items-center gap-3 mb-3">
            <div className="w-24 text-sm text-zinc-600 dark:text-zinc-400">
              {t.ai_transcribe}:
            </div>
            <select
              value={transcribeAccount}
              onChange={(e) => handleTranscribeAccountChange(e.target.value)}
              className={selectClass}
              disabled={processing !== null}
            >
              {aiConfig?.accounts.map((acc) => (
                <option key={acc.label} value={acc.label}>
                  {acc.label} ({acc.provider})
                </option>
              ))}
            </select>
            <select
              value={transcribeModel}
              onChange={(e) => setTranscribeModel(e.target.value)}
              className={selectClass}
              disabled={processing !== null}
            >
              {getTranscriptionModels(getAccountProvider(transcribeAccount)).map(
                (model) => (
                  <option key={model} value={model}>
                    {model}
                  </option>
                )
              )}
            </select>
            <button
              onClick={() => requestAction("transcribe")}
              disabled={processing !== null}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {processing === "transcribe" ? (
                <FaSpinner className="animate-spin" />
              ) : (
                <FaMicrophone />
              )}
              {processing === "transcribe" ? t.ai_processing : t.ai_run}
            </button>
          </div>

          {/* Summarization row */}
          <div className="flex items-center gap-3">
            <div className="w-24 text-sm text-zinc-600 dark:text-zinc-400">
              {t.ai_summarize}:
            </div>
            <select
              value={summarizeAccount}
              onChange={(e) => handleSummarizeAccountChange(e.target.value)}
              className={selectClass}
              disabled={processing !== null || !canSummarize}
            >
              {aiConfig?.accounts.map((acc) => (
                <option key={acc.label} value={acc.label}>
                  {acc.label} ({acc.provider})
                </option>
              ))}
            </select>
            <select
              value={summarizeModel}
              onChange={(e) => setSummarizeModel(e.target.value)}
              className={selectClass}
              disabled={processing !== null || !canSummarize}
            >
              {getSummarizationModels(getAccountProvider(summarizeAccount)).map(
                (model) => (
                  <option key={model.id} value={model.id}>
                    {model.name}
                  </option>
                )
              )}
            </select>
            <button
              onClick={() => requestAction("summarize")}
              disabled={processing !== null || !canSummarize}
              className="flex items-center gap-2 px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {processing === "summarize" ? (
                <FaSpinner className="animate-spin" />
              ) : (
                <FaFileLines />
              )}
              {processing === "summarize" ? t.ai_processing : t.ai_run}
            </button>
          </div>

          {!canSummarize && (
            <p className="text-xs text-zinc-500 dark:text-zinc-400 mt-2 ml-24 pl-3">
              Transcribe first to enable summarization
            </p>
          )}
        </div>
      )}

      {/* Section 3: Outputs */}
      {selectedFile && (selectedFile.has_transcript || selectedFile.has_summary || transcript || summary) && (
        <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
          <h2 className="font-medium text-zinc-900 dark:text-white mb-4">
            3. Outputs
          </h2>

          {/* Output files list */}
          <div className="space-y-2">
            {/* Chunks directory */}
            <div className="flex items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
              <FaFolderOpen className="text-yellow-500 shrink-0" />
              <span>{getBaseName(selectedFile.filename)}.chunks/</span>
              <span className="text-xs text-zinc-400">(audio chunks)</span>
            </div>

            {/* Transcript file */}
            {(selectedFile.has_transcript || transcript) && (
              <a
                href={`/api/download?path=${encodeURIComponent(selectedFile.path.replace(/\.[^.]+$/, ".transcript.md"))}`}
                download
                className="flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:underline cursor-pointer"
              >
                <FaFileLines className="text-blue-500 shrink-0" />
                <span>{getBaseName(selectedFile.filename)}.transcript.md</span>
                <FaDownload className="text-xs" />
              </a>
            )}

            {/* Summary file */}
            {(selectedFile.has_summary || summary) && (
              <a
                href={`/api/download?path=${encodeURIComponent(selectedFile.path.replace(/\.[^.]+$/, ".summary.md"))}`}
                download
                className="flex items-center gap-2 text-sm text-purple-600 dark:text-purple-400 hover:underline cursor-pointer"
              >
                <FaFileLines className="text-purple-500 shrink-0" />
                <span>{getBaseName(selectedFile.filename)}.summary.md</span>
                <FaDownload className="text-xs" />
              </a>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
