import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Download, Folder, Link, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  useDownloadsStore,
  setupDownloadListeners,
  startDownload,
} from "@/stores/downloads";
import { MediaInfo, Config } from "./types";
import { DownloadItem } from "./DownloadItem";

export function HomePage() {
  const [url, setUrl] = useState("");
  const [isExtracting, setIsExtracting] = useState(false);
  const [config, setConfig] = useState<Config | null>(null);
  const downloads = useDownloadsStore((state) => state.downloads);
  const clearCompleted = useDownloadsStore((state) => state.clearCompleted);

  useEffect(() => {
    setupDownloadListeners();

    invoke<Config>("get_config")
      .then(setConfig)
      .catch(console.error);
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || !config) return;

    setIsExtracting(true);
    try {
      const mediaInfo = await invoke<MediaInfo>("extract_media", { url });

      if (mediaInfo.formats.length === 0) {
        toast.error("No downloadable formats found");
        return;
      }

      const format = mediaInfo.formats[0];
      const ext = format.ext || "mp4";
      const sanitizedTitle = mediaInfo.title
        .replace(/[/\\?%*:|"<>]/g, "-")
        .substring(0, 100);
      const outputPath = `${config.output_dir}/${sanitizedTitle}.${ext}`;

      await startDownload(
        format.url,
        mediaInfo.title,
        outputPath,
        format.headers,
        format.audio_url || undefined
      );
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
      <header className="h-14 border-b border-border flex items-center px-6">
        <h1 className="text-xl font-semibold">Download</h1>
      </header>

      <div className="p-6">
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

        <div className="mt-4 max-w-2xl">
          <p className="text-sm text-muted-foreground">
            Supports Twitter/X, Bilibili, Xiaohongshu, YouTube, Apple Podcasts,
            and direct URLs
          </p>
        </div>

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
