import { useState, useEffect, useCallback } from "react";
import { useApp } from "../context/AppContext";
import {
  fetchAudioFiles,
  fetchAIConfig,
  transcribeAudio,
  summarizeText,
  type AudioFile,
  type AIConfigData,
} from "../utils/apis";
import {
  FaMicrophone,
  FaFileLines,
  FaSpinner,
  FaCheck,
  FaLock,
  FaGear,
} from "react-icons/fa6";
import { Link } from "@tanstack/react-router";
import clsx from "clsx";

// Common models for each provider
const TRANSCRIPTION_MODELS: Record<string, string[]> = {
  openai: ["whisper-1"],
  anthropic: ["whisper-1"], // Uses OpenAI whisper
  qwen: ["paraformer-v2", "whisper-large-v3"],
};

const SUMMARIZATION_MODELS: Record<string, string[]> = {
  openai: ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"],
  anthropic: [
    "claude-3-5-sonnet-20241022",
    "claude-3-5-haiku-20241022",
    "claude-3-opus-20240229",
  ],
  qwen: ["qwen-plus", "qwen-turbo", "qwen-max"],
};

export function PodcastNotesPage() {
  const { t, showToast } = useApp();
  const [files, setFiles] = useState<AudioFile[]>([]);
  const [aiConfig, setAIConfig] = useState<AIConfigData | null>(null);
  const [selectedFile, setSelectedFile] = useState<AudioFile | null>(null);
  const [loading, setLoading] = useState(true);
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
      const [filesRes, aiRes] = await Promise.all([
        fetchAudioFiles(),
        fetchAIConfig(),
      ]);
      if (filesRes.code === 200) {
        setFiles(filesRes.data.files || []);
      }
      if (aiRes.code === 200) {
        setAIConfig(aiRes.data);
        // Set default account for both steps
        const defaultAcc = aiRes.data.default_account || "";
        if (defaultAcc) {
          setTranscribeAccount(defaultAcc);
          setSummarizeAccount(defaultAcc);
          // Set default models based on provider
          const provider =
            aiRes.data.accounts[defaultAcc]?.provider || "openai";
          setTranscribeModel(
            TRANSCRIPTION_MODELS[provider]?.[0] || "whisper-1"
          );
          setSummarizeModel(SUMMARIZATION_MODELS[provider]?.[0] || "gpt-4o");
        }
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

  const hasAIAccount = aiConfig && Object.keys(aiConfig.accounts).length > 0;
  const accountList = aiConfig ? Object.entries(aiConfig.accounts) : [];

  // Get current account's encryption status
  const getAccountEncrypted = (accountName: string) => {
    return aiConfig?.accounts[accountName]?.is_encrypted ?? true;
  };

  // Get provider for account
  const getAccountProvider = (accountName: string) => {
    return aiConfig?.accounts[accountName]?.provider || "openai";
  };

  // Update model when account changes
  const handleTranscribeAccountChange = (accountName: string) => {
    setTranscribeAccount(accountName);
    const provider = getAccountProvider(accountName);
    setTranscribeModel(TRANSCRIPTION_MODELS[provider]?.[0] || "whisper-1");
  };

  const handleSummarizeAccountChange = (accountName: string) => {
    setSummarizeAccount(accountName);
    const provider = getAccountProvider(accountName);
    setSummarizeModel(SUMMARIZATION_MODELS[provider]?.[0] || "gpt-4o");
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
          file_path: selectedFile.full_path,
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
        const transcriptPath = selectedFile.full_path.replace(
          /\.[^.]+$/,
          ".transcript.md"
        );

        const res = await summarizeText({
          file_path: selectedFile.has_transcript ? transcriptPath : undefined,
          text:
            !selectedFile.has_transcript && transcript ? transcript : undefined,
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
    [
      selectedFile,
      summarizeAccount,
      summarizeModel,
      transcript,
      showToast,
      loadData,
    ]
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
    "px-2 py-1.5 border border-zinc-300 dark:border-zinc-600 rounded bg-zinc-100 dark:bg-zinc-700 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500";

  return (
    <div className="max-w-4xl mx-auto flex flex-col gap-6">
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

      <div className="grid md:grid-cols-2 gap-6">
        {/* File List */}
        <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 overflow-hidden">
          <div className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-700">
            <h2 className="font-medium text-zinc-900 dark:text-white">
              Audio Files
            </h2>
          </div>
          <div className="max-h-96 overflow-y-auto">
            {files.length === 0 ? (
              <div className="p-4 text-center text-zinc-500 dark:text-zinc-400 text-sm">
                No audio files found. Download podcasts first.
              </div>
            ) : (
              <div className="divide-y divide-zinc-200 dark:divide-zinc-700">
                {files.map((file) => (
                  <button
                    key={file.path}
                    onClick={() => {
                      setSelectedFile(file);
                      setTranscript(null);
                      setSummary(null);
                    }}
                    className={clsx(
                      "w-full px-4 py-3 text-left hover:bg-zinc-50 dark:hover:bg-zinc-700/50 transition-colors",
                      selectedFile?.path === file.path &&
                        "bg-blue-50 dark:bg-blue-900/20"
                    )}
                  >
                    <div className="flex items-start gap-3">
                      <FaMicrophone className="text-zinc-400 mt-1 shrink-0" />
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

        {/* Actions & Results */}
        <div className="flex flex-col gap-4">
          {selectedFile ? (
            <>
              {/* Selected File Info */}
              <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
                <div className="text-sm font-medium text-zinc-900 dark:text-white mb-1">
                  {selectedFile.name}
                </div>
                <div className="text-xs text-zinc-500 dark:text-zinc-400">
                  {formatFileSize(selectedFile.size)}
                </div>
              </div>

              {/* Transcription Section */}
              <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
                <div className="text-sm font-medium text-zinc-900 dark:text-white mb-3">
                  Transcription
                </div>
                <div className="flex flex-wrap gap-2 mb-3">
                  <select
                    value={transcribeAccount}
                    onChange={(e) =>
                      handleTranscribeAccountChange(e.target.value)
                    }
                    className={selectClass}
                    disabled={processing !== null}
                  >
                    {accountList.map(([name]) => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                  </select>
                  <select
                    value={transcribeModel}
                    onChange={(e) => setTranscribeModel(e.target.value)}
                    className={selectClass}
                    disabled={processing !== null}
                  >
                    {TRANSCRIPTION_MODELS[
                      getAccountProvider(transcribeAccount)
                    ]?.map((model) => (
                      <option key={model} value={model}>
                        {model}
                      </option>
                    ))}
                  </select>
                </div>
                <button
                  onClick={() => requestAction("transcribe")}
                  disabled={processing !== null}
                  className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {processing === "transcribe" ? (
                    <>
                      <FaSpinner className="animate-spin" />
                      Transcribing...
                    </>
                  ) : (
                    <>
                      <FaMicrophone />
                      Transcribe
                    </>
                  )}
                </button>
              </div>

              {/* Summarization Section */}
              <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
                <div className="text-sm font-medium text-zinc-900 dark:text-white mb-3">
                  Summarization
                </div>
                <div className="flex flex-wrap gap-2 mb-3">
                  <select
                    value={summarizeAccount}
                    onChange={(e) =>
                      handleSummarizeAccountChange(e.target.value)
                    }
                    className={selectClass}
                    disabled={
                      processing !== null ||
                      (!selectedFile.has_transcript && !transcript)
                    }
                  >
                    {accountList.map(([name]) => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                  </select>
                  <select
                    value={summarizeModel}
                    onChange={(e) => setSummarizeModel(e.target.value)}
                    className={selectClass}
                    disabled={
                      processing !== null ||
                      (!selectedFile.has_transcript && !transcript)
                    }
                  >
                    {SUMMARIZATION_MODELS[
                      getAccountProvider(summarizeAccount)
                    ]?.map((model) => (
                      <option key={model} value={model}>
                        {model}
                      </option>
                    ))}
                  </select>
                </div>
                <button
                  onClick={() => requestAction("summarize")}
                  disabled={
                    processing !== null ||
                    (!selectedFile.has_transcript && !transcript)
                  }
                  className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {processing === "summarize" ? (
                    <>
                      <FaSpinner className="animate-spin" />
                      Summarizing...
                    </>
                  ) : (
                    <>
                      <FaFileLines />
                      Summarize
                    </>
                  )}
                </button>
                {!selectedFile.has_transcript && !transcript && (
                  <p className="text-xs text-zinc-500 dark:text-zinc-400 text-center mt-2">
                    Transcribe the audio first to enable summarization
                  </p>
                )}
              </div>

              {/* Results */}
              {transcript && (
                <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
                  <h3 className="font-medium text-zinc-900 dark:text-white mb-2">
                    Transcript
                  </h3>
                  <div className="text-sm text-zinc-700 dark:text-zinc-300 max-h-48 overflow-y-auto whitespace-pre-wrap">
                    {transcript.slice(0, 2000)}
                    {transcript.length > 2000 && "..."}
                  </div>
                </div>
              )}

              {summary && (
                <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
                  <h3 className="font-medium text-zinc-900 dark:text-white mb-2">
                    Summary
                  </h3>
                  <div className="text-sm text-zinc-700 dark:text-zinc-300 mb-4">
                    {summary.summary}
                  </div>
                  {summary.keyPoints.length > 0 && (
                    <>
                      <h4 className="font-medium text-zinc-900 dark:text-white mb-2">
                        Key Points
                      </h4>
                      <ul className="text-sm text-zinc-700 dark:text-zinc-300 list-disc list-inside space-y-1">
                        {summary.keyPoints.map((point, i) => (
                          <li key={i}>{point}</li>
                        ))}
                      </ul>
                    </>
                  )}
                </div>
              )}
            </>
          ) : (
            <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-8 text-center text-zinc-500 dark:text-zinc-400">
              Select an audio file to transcribe and summarize
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
