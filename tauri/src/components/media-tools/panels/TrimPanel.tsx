import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { FileDropInput } from "@/components/ui/file-drop-input";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PanelProps, formatDuration, generateOutputPath } from "../types";

const VIDEO_EXTENSIONS = ["mp4", "mkv", "webm", "mov", "avi"];

export function TrimPanel({
  inputFile,
  outputDir,
  loading,
  progress,
  mediaInfo,
  onSelectInput,
  onFileDrop,
  setLoading,
  setProgress,
  setJobId,
}: PanelProps) {
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
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>Input File</Label>
        <FileDropInput
          value={inputFile}
          placeholder="Drop a video here or click to select"
          accept={VIDEO_EXTENSIONS}
          acceptHint=".mp4, .mkv, .webm, .mov, .avi"
          onSelectClick={onSelectInput}
          onDrop={onFileDrop}
          disabled={loading}
          invalidDropMessage="Please drop a video file (mp4, mkv, webm, mov, avi)"
        />
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
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>{outputPath}</p>
        </div>
      )}
      {loading && <Progress value={progress} />}
      <div className="pt-2">
        <Button onClick={handleTrim} disabled={!inputFile || !outputDir || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Trim
        </Button>
      </div>
    </div>
  );
}
