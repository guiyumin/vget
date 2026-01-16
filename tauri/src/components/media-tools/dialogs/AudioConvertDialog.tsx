import { useState } from "react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Progress } from "@/components/ui/progress";
import { FolderOpen, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { DialogProps } from "../types";

export function AudioConvertDialog({
  open,
  inputFile,
  outputFile,
  loading,
  progress,
  onSelectInput,
  onSelectOutput,
  onClose,
  setLoading,
  setProgress,
  setJobId,
}: DialogProps) {
  const [audioFormat, setAudioFormat] = useState("mp3");

  const handleConvertAudio = async () => {
    if (!inputFile || !outputFile) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_convert_audio", {
        inputPath: inputFile,
        outputPath: outputFile,
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
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Convert Audio</DialogTitle>
          <DialogDescription>Convert audio to a different format</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Input Audio</Label>
            <div className="flex gap-2">
              <Input value={inputFile} readOnly placeholder="Select audio file..." className="flex-1" />
              <Button variant="outline" onClick={onSelectInput}>
                <FolderOpen className="h-4 w-4" />
              </Button>
            </div>
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
          <div className="space-y-2">
            <Label>Output File</Label>
            <div className="flex gap-2">
              <Input value={outputFile} readOnly placeholder="Select output..." className="flex-1" />
              <Button variant="outline" onClick={() => onSelectOutput(audioFormat)}>
                <FolderOpen className="h-4 w-4" />
              </Button>
            </div>
          </div>
          {loading && <Progress value={progress} />}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleConvertAudio} disabled={!inputFile || !outputFile || loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            Convert
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
