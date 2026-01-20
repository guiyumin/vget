import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Progress } from "@/components/ui/progress";
import { FileDropInput } from "@/components/ui/file-drop-input";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PanelProps, generateOutputPath } from "../types";

const VIDEO_EXTENSIONS = ["mp4", "mkv", "webm", "mov", "avi"];

export function ExtractAudioPanel({
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
  const [audioFormat, setAudioFormat] = useState("mp3");

  const outputPath = inputFile ? generateOutputPath(outputDir, inputFile, audioFormat) : "";

  const handleExtractAudio = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_extract_audio", {
        inputPath: inputFile,
        outputPath,
        format: audioFormat,
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
        <Label>Audio Format</Label>
        <Select value={audioFormat} onValueChange={setAudioFormat}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="mp3">MP3</SelectItem>
            <SelectItem value="aac">AAC</SelectItem>
            <SelectItem value="flac">FLAC</SelectItem>
            <SelectItem value="wav">WAV</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {inputFile && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>{outputPath}</p>
        </div>
      )}
      {loading && <Progress value={progress} />}
      <div className="pt-2">
        <Button onClick={handleExtractAudio} disabled={!inputFile || !outputDir || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Extract
        </Button>
      </div>
    </div>
  );
}
