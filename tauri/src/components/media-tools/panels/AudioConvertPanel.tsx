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

const AUDIO_EXTENSIONS = ["mp3", "aac", "flac", "wav", "ogg", "m4a"];

export function AudioConvertPanel({
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

  const outputPath = inputFile ? generateOutputPath(outputDir, inputFile, audioFormat, "converted") : "";

  const handleConvertAudio = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_convert_audio", {
        inputPath: inputFile,
        outputPath,
        format: audioFormat,
        bitrate: null,
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
        <Label>Input Audio</Label>
        <FileDropInput
          value={inputFile}
          placeholder="Drop an audio file here or click to select"
          accept={AUDIO_EXTENSIONS}
          acceptHint=".mp3, .aac, .flac, .wav, .ogg, .m4a"
          onSelectClick={onSelectInput}
          onDrop={onFileDrop}
          disabled={loading}
          invalidDropMessage="Please drop an audio file (mp3, aac, flac, wav, ogg, m4a)"
        />
      </div>
      <div className="space-y-2">
        <Label>Output Format</Label>
        <Select value={audioFormat} onValueChange={setAudioFormat}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="mp3">MP3</SelectItem>
            <SelectItem value="aac">AAC</SelectItem>
            <SelectItem value="flac">FLAC (Lossless)</SelectItem>
            <SelectItem value="wav">WAV (Uncompressed)</SelectItem>
            <SelectItem value="ogg">OGG Vorbis</SelectItem>
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
        <Button onClick={handleConvertAudio} disabled={!inputFile || !outputDir || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Convert
        </Button>
      </div>
    </div>
  );
}
