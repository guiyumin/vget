import { create } from "zustand";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";

export type DownloadStatus =
  | "pending"
  | "downloading"
  | "completed"
  | "failed"
  | "cancelled";

export interface DownloadProgress {
  job_id: string;
  downloaded: number;
  total: number | null;
  speed: number;
  percent: number;
}

export interface Download {
  id: string;
  url: string;
  title: string;
  outputPath: string;
  status: DownloadStatus;
  progress: DownloadProgress | null;
  error: string | null;
}

interface DownloadsState {
  downloads: Download[];
  addDownload: (download: Download) => void;
  updateDownload: (id: string, updates: Partial<Download>) => void;
  removeDownload: (id: string) => void;
  clearCompleted: () => void;
}

export const useDownloadsStore = create<DownloadsState>((set) => ({
  downloads: [],

  addDownload: (download) =>
    set((state) => ({
      downloads: [download, ...state.downloads],
    })),

  updateDownload: (id, updates) =>
    set((state) => ({
      downloads: state.downloads.map((d) =>
        d.id === id ? { ...d, ...updates } : d
      ),
    })),

  removeDownload: (id) =>
    set((state) => ({
      downloads: state.downloads.filter((d) => d.id !== id),
    })),

  clearCompleted: () =>
    set((state) => ({
      downloads: state.downloads.filter(
        (d) => d.status !== "completed" && d.status !== "failed"
      ),
    })),
}));

// Listen to download events from Rust
let listenersInitialized = false;

export async function setupDownloadListeners() {
  // Prevent duplicate listeners
  if (listenersInitialized) return;
  listenersInitialized = true;

  await listen<DownloadProgress>("download-progress", (event) => {
    const progress = event.payload;
    useDownloadsStore.getState().updateDownload(progress.job_id, {
      progress,
      status: "downloading",
    });
  });

  await listen<{ jobId: string; outputPath: string }>(
    "download-complete",
    (event) => {
      useDownloadsStore.getState().updateDownload(event.payload.jobId, {
        status: "completed",
        progress: null,
      });
    }
  );

  await listen<{ jobId: string; error: string }>("download-error", (event) => {
    const { jobId, error } = event.payload;
    useDownloadsStore.getState().updateDownload(jobId, {
      status: error.includes("cancelled") ? "cancelled" : "failed",
      error,
      progress: null,
    });
  });
}

export async function startDownload(
  url: string,
  title: string,
  outputPath: string,
  headers?: Record<string, string>
): Promise<string> {
  const jobId = await invoke<string>("start_download", {
    url,
    outputPath,
    formatId: null,
    headers: headers || null,
  });

  useDownloadsStore.getState().addDownload({
    id: jobId,
    url,
    title,
    outputPath,
    status: "pending",
    progress: null,
    error: null,
  });

  return jobId;
}

export async function cancelDownload(jobId: string): Promise<void> {
  await invoke("cancel_download", { jobId });
}
