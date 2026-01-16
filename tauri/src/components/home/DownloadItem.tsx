import { X, CheckCircle2, AlertCircle, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Progress } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import { cancelDownload, type Download } from "@/stores/downloads";
import { formatBytes, formatSpeed } from "./types";

interface DownloadItemProps {
  download: Download;
}

export function DownloadItem({ download }: DownloadItemProps) {
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
