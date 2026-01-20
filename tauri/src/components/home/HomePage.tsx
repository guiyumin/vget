import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { Download, Folder, Link, Loader2, Upload, FileText } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  useDownloadsStore,
  setupDownloadListeners,
  startDownload,
} from "@/stores/downloads";
import { MediaInfo, Config } from "./types";
import { DownloadItem } from "./DownloadItem";
import { cn } from "@/lib/utils";
import { useDropZone } from "@/hooks/useDropZone";

export function HomePage() {
  const [url, setUrl] = useState("");
  const [isExtracting, setIsExtracting] = useState(false);
  const [config, setConfig] = useState<Config | null>(null);
  const [bulkProgress, setBulkProgress] = useState<{ current: number; total: number } | null>(null);
  const downloads = useDownloadsStore((state) => state.downloads);
  const clearCompleted = useDownloadsStore((state) => state.clearCompleted);
  const { t } = useTranslation();

  // Handle bulk file import
  const handleFileImport = async (filePath: string) => {
    try {
      const content = await invoke<string>("read_text_file", { path: filePath });
      const urls = content
        .split("\n")
        .map((line) => line.trim())
        .filter((line) => line && (line.startsWith("http://") || line.startsWith("https://")));

      if (urls.length === 0) {
        toast.error(t("home.noUrlsInFile") || "No valid URLs found in file");
        return;
      }

      toast.success(t("home.foundUrls", { count: urls.length }) || `Found ${urls.length} URLs`);

      // Process URLs one by one
      setBulkProgress({ current: 0, total: urls.length });
      for (let i = 0; i < urls.length; i++) {
        setBulkProgress({ current: i + 1, total: urls.length });
        await processUrl(urls[i]);
      }
      setBulkProgress(null);
    } catch (err) {
      console.error("Failed to read file:", err);
      toast.error(t("home.failedToReadFile") || "Failed to read file");
    }
  };

  // Drop zone for bulk download (.txt files)
  const { ref: dropZoneRef, isDragging } = useDropZone<HTMLDivElement>({
    accept: ["txt"],
    onDrop: (paths) => {
      handleFileImport(paths[0]);
    },
    onInvalidDrop: (_paths, ext) => {
      if (ext === "md" || ext === "markdown") {
        toast.error(t("home.dropMdHint") || "For Markdown files, go to PDF Tools → Markdown to PDF");
      } else {
        toast.error(t("home.dropTxtFile") || "Please drop a .txt file containing URLs");
      }
    },
  });

  useEffect(() => {
    setupDownloadListeners();

    invoke<Config>("get_config")
      .then(setConfig)
      .catch(console.error);
  }, []);

  const handleSelectFile = async () => {
    const selected = await open({
      multiple: false,
      filters: [{ name: "Text", extensions: ["txt"] }],
    });
    if (selected && typeof selected === "string") {
      await handleFileImport(selected);
    }
  };

  const processUrl = async (inputUrl: string) => {
    if (!inputUrl.trim() || !config) return;

    try {
      const mediaInfo = await invoke<MediaInfo>("extract_media", { url: inputUrl });

      if (mediaInfo.formats.length === 0) {
        console.warn(`No formats found for: ${inputUrl}`);
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
    } catch (err) {
      console.error(`Failed to process URL ${inputUrl}:`, err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || !config) return;

    setIsExtracting(true);
    try {
      const mediaInfo = await invoke<MediaInfo>("extract_media", { url });

      if (mediaInfo.formats.length === 0) {
        toast.error(t("home.noFormats"));
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
      toast.success(t("home.downloadStarted"));
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
        toast.error(t("home.failedToOpenFolder"));
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
        <h1 className="text-xl font-semibold">{t("home.title")}</h1>
        {bulkProgress && (
          <span className="ml-4 text-sm text-muted-foreground">
            {t("home.processingBulk", { current: bulkProgress.current, total: bulkProgress.total }) ||
              `Processing ${bulkProgress.current}/${bulkProgress.total}`}
          </span>
        )}
      </header>

      <div className="p-6">
        {/* Single URL input */}
        <form onSubmit={handleSubmit} className="max-w-2xl">
          <div className="relative">
            <Link className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-muted-foreground" />
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={t("home.urlPlaceholder")}
              className="w-full pl-12 pr-32 py-4 rounded-xl border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <button
              type="submit"
              disabled={isExtracting || !url.trim()}
              className="absolute right-2 top-1/2 -translate-y-1/2 px-4 py-2 rounded-lg bg-primary text-primary-foreground font-medium disabled:opacity-50 disabled:cursor-not-allowed hover:opacity-90 transition-opacity flex items-center gap-2"
            >
              {isExtracting && <Loader2 className="h-4 w-4 animate-spin" />}
              {isExtracting ? t("home.extracting") : t("home.download")}
            </button>
          </div>
        </form>

        <div className="mt-3 max-w-2xl">
          <p className="text-sm text-muted-foreground">
            {t("home.supportsHint")}
          </p>
        </div>

        {/* Bulk download drop zone */}
        <div className="mt-6 max-w-2xl">
          <div
            ref={dropZoneRef}
            onClick={handleSelectFile}
            className={cn(
              "border-2 border-dashed rounded-xl p-6 text-center cursor-pointer transition-all",
              isDragging
                ? "border-primary bg-primary/5"
                : "border-muted-foreground/25 hover:border-muted-foreground/50 hover:bg-muted/30"
            )}
          >
            <div className="flex items-center justify-center gap-3">
              {isDragging ? (
                <Upload className="h-8 w-8 text-primary" />
              ) : (
                <FileText className="h-8 w-8 text-muted-foreground" />
              )}
              <div className="text-left">
                <p className={cn(
                  "font-medium",
                  isDragging ? "text-primary" : "text-foreground"
                )}>
                  {t("home.bulkDownloadTitle") || "Bulk Download"}
                </p>
                <p className="text-sm text-muted-foreground">
                  {t("home.bulkDownloadHint") || "Drop a .txt file with URLs or click to select"}
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Downloads list */}
        <div className="mt-8 max-w-2xl">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-medium">{t("home.downloads")}</h2>
            <div className="flex items-center gap-2">
              {completedDownloads.length > 0 && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearCompleted}
                  className="text-muted-foreground"
                >
                  {t("home.clearCompleted")}
                </Button>
              )}
              <button
                onClick={handleOpenFolder}
                className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <Folder className="h-4 w-4" />
                {t("home.openFolder")}
              </button>
            </div>
          </div>

          {downloads.length === 0 ? (
            <div className="border border-dashed border-border rounded-xl p-12 text-center">
              <Download className="h-12 w-12 text-muted-foreground/50 mx-auto mb-4" />
              <p className="text-muted-foreground">
                {t("home.noDownloadsYet")}
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
