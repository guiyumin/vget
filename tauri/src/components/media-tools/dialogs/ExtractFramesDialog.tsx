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
import { Slider } from "@/components/ui/slider";
import { Progress } from "@/components/ui/progress";
import { FolderOpen, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { DialogProps, getBasename } from "../types";

export function ExtractFramesDialog({
  open,
  inputFile,
  outputDir,
  loading,
  progress,
  onSelectInput,
  onClose,
  setLoading,
  setProgress,
  setJobId,
}: DialogProps) {
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
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="overflow-hidden">
        <DialogHeader>
          <DialogTitle>Extract Frames</DialogTitle>
          <DialogDescription>Extract images from video</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 overflow-hidden">
          <div className="space-y-2">
            <Label>Input Video</Label>
            <div className="flex gap-2">
              <Input value={inputFile} readOnly placeholder="Select a video..." className="min-w-0 flex-1" />
              <Button variant="outline" onClick={onSelectInput} className="shrink-0">
                <FolderOpen className="h-4 w-4" />
              </Button>
            </div>
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
            <div className="space-y-2 overflow-hidden">
              <Label className="text-muted-foreground">Output Folder</Label>
              <p className="text-sm text-muted-foreground break-all" title={outputFolder}>{outputFolder}</p>
            </div>
          )}
          {loading && <Progress value={progress} />}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleExtractFrames} disabled={!inputFile || !outputDir || loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            Extract
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
