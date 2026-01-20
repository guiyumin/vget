import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { Progress } from "@/components/ui/progress";
import { FileDropInput } from "@/components/ui/file-drop-input";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PanelProps, getBasename } from "../types";

const VIDEO_EXTENSIONS = ["mp4", "mkv", "webm", "mov", "avi"];

export function ExtractFramesPanel({
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
  const [fps, setFps] = useState(1);

  const outputFolder = inputFile ? `${outputDir}/${getBasename(inputFile)}_frames` : "";

  const handleExtractFrames = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_extract_frames", {
        inputPath: inputFile,
        outputDir: outputFolder,
        fps,
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
        <Label>Input Video</Label>
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
        <Label>Frames per Second: {fps}</Label>
        <Slider
          value={[fps]}
          onValueChange={([v]) => setFps(v)}
          min={0.1}
          max={5}
          step={0.1}
        />
        <p className="text-xs text-muted-foreground">
          1 = one frame per second, 0.1 = one frame every 10 seconds
        </p>
      </div>
      {inputFile && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output Folder</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputFolder}>{outputFolder}</p>
        </div>
      )}
      {loading && <Progress value={progress} />}
      <div className="pt-2">
        <Button onClick={handleExtractFrames} disabled={!inputFile || !outputDir || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Extract
        </Button>
      </div>
    </div>
  );
}
