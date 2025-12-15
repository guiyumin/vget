import { useState, useEffect } from "react";
import { useApp } from "../context/AppContext";
import { Link } from "@tanstack/react-router";
import clsx from "clsx";
import {
  fetchWebDAVRemotes,
  fetchWebDAVList,
  submitWebDAVDownload,
  type WebDAVRemote,
  type WebDAVFile,
} from "../utils/apis";
import {
  FaFolder,
  FaFile,
  FaChevronRight,
  FaDownload,
  FaArrowUp,
} from "react-icons/fa6";

function formatSize(bytes: number): string {
  if (bytes === 0) return "-";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

export function WebDAVPage() {
  const { t, isConnected, refresh, showToast } = useApp();

  // State
  const [remotes, setRemotes] = useState<WebDAVRemote[]>([]);
  const [remotesLoaded, setRemotesLoaded] = useState(false);
  const [selectedRemote, setSelectedRemote] = useState<string>("");
  const [currentPath, setCurrentPath] = useState<string>("/");
  const [files, setFiles] = useState<WebDAVFile[]>([]);
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Load remotes on mount
  useEffect(() => {
    if (!isConnected) return;

    fetchWebDAVRemotes().then((res) => {
      if (res.code === 200 && res.data.remotes) {
        setRemotes(res.data.remotes);
        // Auto-select first remote if available
        if (res.data.remotes.length > 0) {
          setSelectedRemote(res.data.remotes[0].name);
        }
      }
      setRemotesLoaded(true);
    });
  }, [isConnected]);

  // Load directory contents when remote or path changes
  useEffect(() => {
    if (!selectedRemote) return;

    const loadDirectory = async () => {
      setLoading(true);
      setError(null);
      setSelectedFiles(new Set());

      try {
        const res = await fetchWebDAVList(selectedRemote, currentPath);
        if (res.code === 200) {
          setFiles(res.data.files || []);
        } else {
          setError(res.message);
          setFiles([]);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load directory");
        setFiles([]);
      } finally {
        setLoading(false);
      }
    };

    loadDirectory();
  }, [selectedRemote, currentPath]);

  // Navigate to a path
  const navigateTo = (path: string) => {
    setCurrentPath(path);
  };

  // Navigate up one level
  const navigateUp = () => {
    if (currentPath === "/") return;
    const parts = currentPath.split("/").filter(Boolean);
    parts.pop();
    setCurrentPath("/" + parts.join("/"));
  };

  // Toggle file selection
  const toggleSelect = (path: string) => {
    const newSelected = new Set(selectedFiles);
    if (newSelected.has(path)) {
      newSelected.delete(path);
    } else {
      newSelected.add(path);
    }
    setSelectedFiles(newSelected);
  };

  // Select/deselect all files
  const toggleSelectAll = () => {
    const selectableFiles = files.filter((f) => !f.isDir);
    if (selectedFiles.size === selectableFiles.length) {
      setSelectedFiles(new Set());
    } else {
      setSelectedFiles(new Set(selectableFiles.map((f) => f.path)));
    }
  };

  // Download selected files
  const handleDownload = async () => {
    if (selectedFiles.size === 0) return;

    setSubmitting(true);
    try {
      const res = await submitWebDAVDownload(
        selectedRemote,
        Array.from(selectedFiles)
      );
      if (res.code === 200) {
        const count = selectedFiles.size;
        setSelectedFiles(new Set());
        refresh(); // Refresh jobs list
        showToast(
          "success",
          count === 1
            ? t.download_queued || "Download queued"
            : `${count} ${t.downloads_queued || "downloads queued"}`
        );
      } else {
        setError(res.message);
        showToast("error", res.message);
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to start download";
      setError(msg);
      showToast("error", msg);
    } finally {
      setSubmitting(false);
    }
  };

  // Build breadcrumb parts
  const pathParts = currentPath.split("/").filter(Boolean);

  // Calculate selected size
  const selectedSize = files
    .filter((f) => selectedFiles.has(f.path))
    .reduce((sum, f) => sum + f.size, 0);

  const selectableFiles = files.filter((f) => !f.isDir);
  const allSelected =
    selectableFiles.length > 0 &&
    selectedFiles.size === selectableFiles.length;

  // Show loading while fetching remotes
  if (!remotesLoaded) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">{t.webdav_browser}</h1>
        <div className="bg-zinc-100 dark:bg-zinc-800 rounded-lg p-8 text-center">
          <p className="text-zinc-500 dark:text-zinc-400">{t.loading}</p>
        </div>
      </div>
    );
  }

  // No remotes configured
  if (remotes.length === 0) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">{t.webdav_browser}</h1>
        <div className="bg-zinc-100 dark:bg-zinc-800 rounded-lg p-8 text-center">
          <p className="text-zinc-500 dark:text-zinc-400 mb-4">
            {t.no_webdav_servers}
          </p>
          <Link
            to="/config"
            className="inline-block px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            {t.go_to_settings}
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-6">{t.webdav_browser}</h1>

      {/* Remote Selector */}
      <div className="mb-4">
        <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">
          {t.select_remote}
        </label>
        <select
          value={selectedRemote}
          onChange={(e) => {
            setSelectedRemote(e.target.value);
            setCurrentPath("/");
          }}
          className="w-full max-w-xs px-3 py-2 border border-zinc-300 dark:border-zinc-600 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
        >
          {remotes.map((remote) => (
            <option key={remote.name} value={remote.name}>
              {remote.name}
            </option>
          ))}
        </select>
      </div>

      {/* File Browser */}
      <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 overflow-hidden">
        {/* Breadcrumb */}
        <div className="px-4 py-3 bg-zinc-50 dark:bg-zinc-900 border-b border-zinc-200 dark:border-zinc-700 flex items-center gap-1 text-sm overflow-x-auto">
          <button
            onClick={() => navigateTo("/")}
            className="text-blue-600 dark:text-blue-400 hover:underline font-medium"
          >
            {selectedRemote}:
          </button>
          <FaChevronRight className="text-zinc-400 text-xs flex-shrink-0" />
          {pathParts.map((part, i) => (
            <span key={i} className="flex items-center gap-1">
              <button
                onClick={() =>
                  navigateTo("/" + pathParts.slice(0, i + 1).join("/"))
                }
                className="text-blue-600 dark:text-blue-400 hover:underline"
              >
                {part}
              </button>
              {i < pathParts.length - 1 && (
                <FaChevronRight className="text-zinc-400 text-xs flex-shrink-0" />
              )}
            </span>
          ))}
        </div>

        {/* Error */}
        {error && (
          <div className="px-4 py-3 bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300 border-b border-red-200 dark:border-red-800">
            {error}
          </div>
        )}

        {/* File List */}
        <div className="divide-y divide-zinc-200 dark:divide-zinc-700">
          {loading ? (
            <div className="px-4 py-8 text-center text-zinc-500 dark:text-zinc-400">
              {t.loading}
            </div>
          ) : files.length === 0 ? (
            <div className="px-4 py-8 text-center text-zinc-500 dark:text-zinc-400">
              {t.empty_directory}
            </div>
          ) : (
            <>
              {/* Header */}
              <div className="px-4 py-2 bg-zinc-50 dark:bg-zinc-900 flex items-center gap-4 text-xs font-medium text-zinc-500 dark:text-zinc-400 uppercase">
                <div className="w-6">
                  {selectableFiles.length > 0 && (
                    <input
                      type="checkbox"
                      checked={allSelected}
                      onChange={toggleSelectAll}
                      className="rounded"
                    />
                  )}
                </div>
                <div className="flex-1">{t.name}</div>
                <div className="w-24 text-right">Size</div>
              </div>

              {/* Parent directory */}
              {currentPath !== "/" && (
                <button
                  onClick={navigateUp}
                  className="w-full px-4 py-3 flex items-center gap-4 hover:bg-zinc-50 dark:hover:bg-zinc-700/50 text-left"
                >
                  <div className="w-6"></div>
                  <div className="flex items-center gap-2 flex-1 text-zinc-600 dark:text-zinc-400">
                    <FaArrowUp className="text-zinc-400" />
                    <span>..</span>
                  </div>
                  <div className="w-24 text-right text-zinc-400">-</div>
                </button>
              )}

              {/* Files */}
              {files.map((file) => (
                <div
                  key={file.path}
                  className={clsx(
                    "px-4 py-3 flex items-center gap-4",
                    file.isDir
                      ? "cursor-pointer hover:bg-zinc-50 dark:hover:bg-zinc-700/50"
                      : selectedFiles.has(file.path)
                        ? "bg-blue-50 dark:bg-blue-900/20"
                        : "hover:bg-zinc-50 dark:hover:bg-zinc-700/50"
                  )}
                  onClick={() => file.isDir && navigateTo(file.path)}
                >
                  <div className="w-6">
                    {!file.isDir && (
                      <input
                        type="checkbox"
                        checked={selectedFiles.has(file.path)}
                        onChange={(e) => {
                          e.stopPropagation();
                          toggleSelect(file.path);
                        }}
                        onClick={(e) => e.stopPropagation()}
                        className="rounded"
                      />
                    )}
                  </div>
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    {file.isDir ? (
                      <FaFolder className="text-amber-500 flex-shrink-0" />
                    ) : (
                      <FaFile className="text-zinc-400 flex-shrink-0" />
                    )}
                    <span className="truncate">{file.name}</span>
                  </div>
                  <div className="w-24 text-right text-zinc-500 dark:text-zinc-400 text-sm">
                    {formatSize(file.size)}
                  </div>
                </div>
              ))}
            </>
          )}
        </div>

        {/* Actions */}
        {selectedFiles.size > 0 && (
          <div className="px-4 py-3 bg-zinc-50 dark:bg-zinc-900 border-t border-zinc-200 dark:border-zinc-700 flex items-center justify-between">
            <span className="text-sm text-zinc-600 dark:text-zinc-400">
              {selectedFiles.size} {t.selected_files} ({formatSize(selectedSize)}
              )
            </span>
            <button
              onClick={handleDownload}
              disabled={submitting}
              className={clsx(
                "px-4 py-2 rounded-lg flex items-center gap-2 text-white",
                submitting
                  ? "bg-zinc-400 cursor-not-allowed"
                  : "bg-blue-600 hover:bg-blue-700"
              )}
            >
              <FaDownload />
              {submitting ? t.adding : t.download_selected}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
