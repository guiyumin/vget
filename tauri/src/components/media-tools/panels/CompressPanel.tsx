import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { Progress } from "@/components/ui/progress";
import { FileDropInput } from "@/components/ui/file-drop-input";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PanelProps, generateOutputPath } from "../types";

const VIDEO_EXTENSIONS = ["mp4", "mkv", "webm", "mov", "avi"];

export function CompressPanel({
  inputFile,
  outputDir,
  loading,
  progress,
  onSelectInput,
  onFileDrop,
  setLoading,
  setProgress,
  setJobId,
}: PanelProps) {
  const [quality, setQuality] = useState(23);

  const outputPath = inputFile ? generateOutputPath(outputDir, inputFile, "mp4", "compressed") : "";

  const handleCompress = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_compress_video", {
        inputPath: inputFile,
        outputPath,
        quality,
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
      <div className="space-y-2">
        <Label>Quality (CRF: {quality})</Label>
        <div className="flex items-center gap-4">
          <span className="text-xs text-muted-foreground">High</span>
          <Slider
            value={[quality]}
            onValueChange={([v]) => setQuality(v)}
            min={18}
            max={28}
            step={1}
            className="flex-1"
          />
          <span className="text-xs text-muted-foreground">Low</span>
        </div>
        <p className="text-xs text-muted-foreground">
          Lower values = higher quality, larger file size
        </p>
      </div>
      {inputFile && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>{outputPath}</p>
        </div>
      )}
      {loading && <Progress value={progress} />}
      <div className="pt-2">
        <Button onClick={handleCompress} disabled={!inputFile || !outputDir || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Compress
        </Button>
      </div>
    </div>
  );
}
