import { useState, useRef, useCallback } from "react";
import clsx from "clsx";
import { useApp } from "../context/AppContext";
import { FaUpload, FaFileAlt } from "react-icons/fa";
import { postBulkDownload } from "../utils/apis";

export function BulkDownloadPage() {
  const { t, isConnected, showToast, refresh } = useApp();
  const [urlText, setUrlText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Parse URLs from text, filtering empty lines and comments
  const parseUrls = useCallback((text: string): string[] => {
    return text
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith("#"));
  }, []);

  const urls = parseUrls(urlText);

  // Handle file selection
  const handleFileSelect = useCallback(
    async (file: File) => {
      if (!file.name.endsWith(".txt")) {
        showToast("error", "Please select a .txt file");
        return;
      }

      try {
        const text = await file.text();
        setUrlText(text);
      } catch {
        showToast("error", "Failed to read file");
      }
    },
    [showToast]
  );

  // Handle file input change
  const handleFileInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) {
        handleFileSelect(file);
      }
      // Reset input so the same file can be selected again
      e.target.value = "";
    },
    [handleFileSelect]
  );

  // Handle drag events
  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(false);
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDragOver(false);

      const file = e.dataTransfer.files?.[0];
      if (file) {
        handleFileSelect(file);
      }
    },
    [handleFileSelect]
  );

  // Handle submit all URLs
  const handleSubmitAll = useCallback(async () => {
    if (urls.length === 0 || submitting) return;

    setSubmitting(true);

    try {
      const res = await postBulkDownload(urls);
      if (res.code === 200) {
        const { queued, failed } = res.data;
        setUrlText("");
        refresh();
        if (queued > 0 && failed === 0) {
          showToast("success", `${queued} ${t.downloads_queued}`);
        } else if (queued > 0 && failed > 0) {
          showToast("warning", `${queued} queued, ${failed} invalid`);
        } else if (failed > 0) {
          showToast("error", `${failed} invalid URL(s)`);
        }
      } else {
        showToast("error", res.message || "Failed to queue downloads");
      }
    } catch {
      showToast("error", "Failed to submit downloads");
    } finally {
      setSubmitting(false);
    }
  }, [urls, submitting, showToast, t, refresh]);

  // Handle clear
  const handleClear = useCallback(() => {
    setUrlText("");
  }, []);

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-4">
      <h1 className="text-lg sm:text-xl font-semibold text-zinc-800 dark:text-zinc-100">
        {t.bulk_download}
      </h1>

      {/* File drop zone / select */}
      <div
        className={clsx(
          "border-2 border-dashed rounded-lg p-4 sm:p-6 transition-colors",
          "flex flex-col items-center gap-3",
          dragOver
            ? "border-blue-500 bg-blue-50 dark:bg-blue-950/30"
            : "border-zinc-300 dark:border-zinc-700 hover:border-zinc-400 dark:hover:border-zinc-600"
        )}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        <FaUpload
          className={clsx(
            "text-2xl sm:text-3xl",
            dragOver
              ? "text-blue-500"
              : "text-zinc-400 dark:text-zinc-600"
          )}
        />
        <div className="flex flex-col sm:flex-row items-center gap-2 sm:gap-3">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg text-sm font-medium hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            disabled={!isConnected}
          >
            <FaFileAlt />
            {t.bulk_select_file}
          </button>
          <span className="text-zinc-500 dark:text-zinc-400 text-xs sm:text-sm text-center">
            {t.bulk_drag_drop}
          </span>
        </div>
        <input
          ref={fileInputRef}
          type="file"
          accept=".txt"
          onChange={handleFileInputChange}
          className="hidden"
        />
      </div>

      {/* URL textarea */}
      <div className="flex flex-col gap-2">
        <textarea
          className={clsx(
            "w-full h-48 sm:h-64 px-3 sm:px-4 py-3 border rounded-lg font-mono text-xs sm:text-sm resize-y",
            "bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white",
            "border-zinc-300 dark:border-zinc-700",
            "focus:outline-none focus:border-blue-500",
            "placeholder:text-zinc-400 dark:placeholder:text-zinc-600",
            "disabled:opacity-50"
          )}
          value={urlText}
          onChange={(e) => setUrlText(e.target.value)}
          placeholder={t.bulk_paste_urls}
          disabled={!isConnected || submitting}
        />
        <p className="text-xs text-zinc-500 dark:text-zinc-400">
          {t.bulk_invalid_hint}
        </p>
      </div>

      {/* Actions */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="text-sm text-zinc-600 dark:text-zinc-400">
          {urls.length > 0 && (
            <span>
              {urls.length} {t.bulk_url_count}
            </span>
          )}
        </div>
        <div className="flex gap-3">
          <button
            type="button"
            onClick={handleClear}
            className="flex-1 sm:flex-none px-4 py-2 border border-zinc-300 dark:border-zinc-700 text-zinc-600 dark:text-zinc-400 rounded-lg text-sm hover:border-zinc-400 dark:hover:border-zinc-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={!urlText || submitting}
          >
            {t.bulk_clear}
          </button>
          <button
            type="button"
            onClick={handleSubmitAll}
            className="flex-1 sm:flex-none px-6 py-2 bg-blue-500 text-white rounded-lg text-sm font-medium hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={!isConnected || urls.length === 0 || submitting}
          >
            {submitting ? t.bulk_submitting : t.bulk_submit_all}
          </button>
        </div>
      </div>
    </div>
  );
}
