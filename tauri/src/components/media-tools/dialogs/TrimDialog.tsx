import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { FolderOpen, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { DialogProps, formatDuration, generateOutputPath } from "../types";

export function TrimDialog({
  open,
  inputFile,
  outputDir,
  loading,
  progress,
  mediaInfo,
  onSelectInput,
  onClose,
  setLoading,
  setProgress,
  setJobId,
}: DialogProps) {
  const [startTime, setStartTime] = useState("00:00:00");
  const [endTime, setEndTime] = useState("00:00:10");

  const outputPath = inputFile ? generateOutputPath(outputDir, inputFile, "mp4", "trimmed") : "";

  useEffect(() => {
    if (mediaInfo?.duration) {
      setEndTime(formatDuration(mediaInfo.duration));
    }
  }, [mediaInfo]);

  const handleTrim = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_trim_video", {
        inputPath: inputFile,
        outputPath,
        startTime,
        endTime,
      });
      setJobId(id);
    } catch (e) {
      setLoading(false);
      toast.error(String(e));
    }
  };

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="overflow-hidden">
        <DialogHeader>
          <DialogTitle>Trim Video</DialogTitle>
          <DialogDescription>Cut a clip from your video</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 overflow-hidden">
          <div className="space-y-2">
            <Label>Input File</Label>
            <div className="flex gap-2">
              <Input value={inputFile} readOnly placeholder="Select a video..." className="min-w-0 flex-1" />
              <Button variant="outline" onClick={onSelectInput} className="shrink-0">
                <FolderOpen className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>Start Time</Label>
              <Input
                value={startTime}
                onChange={(e) => setStartTime(e.target.value)}
                placeholder="00:00:00"
              />
            </div>
            <div className="space-y-2">
              <Label>End Time</Label>
              <Input
                value={endTime}
                onChange={(e) => setEndTime(e.target.value)}
                placeholder="00:00:10"
              />
            </div>
          </div>
          {inputFile && (
            <div className="space-y-2 overflow-hidden">
              <Label className="text-muted-foreground">Output</Label>
              <p className="text-sm text-muted-foreground break-all" title={outputPath}>{outputPath}</p>
            </div>
          )}
          {loading && <Progress value={progress} />}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleTrim} disabled={!inputFile || !outputDir || loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            Trim
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
