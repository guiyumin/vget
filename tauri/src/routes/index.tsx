import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  Download,
  Folder,
  Link,
  X,
  CheckCircle2,
  AlertCircle,
  Loader2,
} from "lucide-react";
import { toast } from "sonner";
import { Progress } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import {
  useDownloadsStore,
  setupDownloadListeners,
  startDownload,
  cancelDownload,
  type Download as DownloadType,
} from "@/stores/downloads";

interface MediaInfo {
  id: string;
  title: string;
  uploader: string | null;
  thumbnail: string | null;
  duration: number | null;
  media_type: string;
  formats: {
    id: string;
    url: string;
    ext: string;
    quality: string | null;
    filesize: number | null;
    audio_url: string | null;
    headers?: Record<string, string>;
  }[];
}

interface Config {
  output_dir: string;
}

export const Route = createFileRoute("/")({
  component: Home,
});

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

function formatSpeed(bytesPerSecond: number): string {
  return formatBytes(bytesPerSecond) + "/s";
}

function DownloadItem({ download }: { download: DownloadType }) {
  const handleCancel = async () => {
    try {
      await cancelDownload(download.id);
    } catch (err) {
      toast.error("Failed to cancel download");
    }
  };

  return (
    <div className="border border-border rounded-lg p-4 space-y-2">
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <p className="font-medium truncate">{download.title}</p>
          <p className="text-sm text-muted-foreground truncate">
            {download.url}
          </p>
        </div>
        <div className="flex items-center gap-2 ml-4">
          {download.status === "downloading" && (
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={handleCancel}
            >
              <X className="h-4 w-4" />
            </Button>
          )}
          {download.status === "completed" && (
            <CheckCircle2 className="h-5 w-5 text-green-500" />
          )}
          {download.status === "failed" && (
            <AlertCircle className="h-5 w-5 text-destructive" />
          )}
          {download.status === "cancelled" && (
            <X className="h-5 w-5 text-muted-foreground" />
          )}
        </div>
      </div>

      {download.status === "downloading" && download.progress && (
        <div className="space-y-1">
          <Progress value={download.progress.percent} className="h-2" />
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>
              {formatBytes(download.progress.downloaded)}
              {download.progress.total
                ? ` / ${formatBytes(download.progress.total)}`
                : ""}
            </span>
            <span>{formatSpeed(download.progress.speed)}</span>
          </div>
        </div>
      )}

      {download.status === "pending" && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span>Starting download...</span>
        </div>
      )}

      {download.error && (
        <p className="text-sm text-destructive">{download.error}</p>
      )}
    </div>
  );
}

function Home() {
  const [url, setUrl] = useState("");
  const [isExtracting, setIsExtracting] = useState(false);
  const [config, setConfig] = useState<Config | null>(null);
  const downloads = useDownloadsStore((state) => state.downloads);
  const clearCompleted = useDownloadsStore((state) => state.clearCompleted);

  useEffect(() => {
    // Setup download event listeners
    setupDownloadListeners();

    // Load config
    invoke<Config>("get_config")
      .then(setConfig)
      .catch(console.error);
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || !config) return;

    setIsExtracting(true);
    try {
      // Extract media info
      const mediaInfo = await invoke<MediaInfo>("extract_media", { url });

      if (mediaInfo.formats.length === 0) {
        toast.error("No downloadable formats found");
        return;
      }

      // Get best format (first one for now)
      const format = mediaInfo.formats[0];
      const ext = format.ext || "mp4";
      const sanitizedTitle = mediaInfo.title
        .replace(/[/\\?%*:|"<>]/g, "-")
        .substring(0, 100);
      const outputPath = `${config.output_dir}/${sanitizedTitle}.${ext}`;

      // Start download with headers if present
      await startDownload(format.url, mediaInfo.title, outputPath, format.headers);
      setUrl("");
      toast.success("Download started");
    } catch (err) {
      console.error("Extraction failed:", err);
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setIsExtracting(false);
    }
  };

  const handleOpenFolder = async () => {
    if (config?.output_dir) {
      try {
        await invoke("open_output_folder", { path: config.output_dir });
      } catch (err) {
        toast.error("Failed to open folder");
        console.error(err);
      }
    }
  };

  const activeDownloads = downloads.filter(
    (d) => d.status === "downloading" || d.status === "pending"
  );
  const completedDownloads = downloads.filter(
    (d) =>
      d.status === "completed" ||
      d.status === "failed" ||
      d.status === "cancelled"
  );

  return (
    <div className="h-full">
      {/* Header */}
      <header className="h-14 border-b border-border flex items-center px-6">
        <h1 className="text-xl font-semibold">Download</h1>
      </header>

      {/* Main Content */}
      <div className="p-6">
        {/* URL Input */}
        <form onSubmit={handleSubmit} className="max-w-2xl">
          <div className="relative">
            <Link className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-muted-foreground" />
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="Paste video URL here..."
              className="w-full pl-12 pr-32 py-4 rounded-xl border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <button
              type="submit"
              disabled={isExtracting || !url.trim()}
              className="absolute right-2 top-1/2 -translate-y-1/2 px-4 py-2 rounded-lg bg-primary text-primary-foreground font-medium disabled:opacity-50 disabled:cursor-not-allowed hover:opacity-90 transition-opacity flex items-center gap-2"
            >
              {isExtracting && <Loader2 className="h-4 w-4 animate-spin" />}
              {isExtracting ? "Extracting..." : "Download"}
            </button>
          </div>
        </form>

        {/* Supported Sites */}
        <div className="mt-4 max-w-2xl">
          <p className="text-sm text-muted-foreground">
            Supports Twitter/X, Bilibili, Xiaohongshu, YouTube, Apple Podcasts,
            and direct URLs
          </p>
        </div>

        {/* Downloads Section */}
        <div className="mt-8 max-w-2xl">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-medium">Downloads</h2>
            <div className="flex items-center gap-2">
              {completedDownloads.length > 0 && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearCompleted}
                  className="text-muted-foreground"
                >
                  Clear completed
                </Button>
              )}
              <button
                onClick={handleOpenFolder}
                className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <Folder className="h-4 w-4" />
                Open folder
              </button>
            </div>
          </div>

          {downloads.length === 0 ? (
            <div className="border border-dashed border-border rounded-xl p-12 text-center">
              <Download className="h-12 w-12 text-muted-foreground/50 mx-auto mb-4" />
              <p className="text-muted-foreground">
                No downloads yet. Paste a URL above to get started.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {activeDownloads.map((download) => (
                <DownloadItem key={download.id} download={download} />
              ))}
              {completedDownloads.map((download) => (
                <DownloadItem key={download.id} download={download} />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
