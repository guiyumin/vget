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
import { DialogProps } from "../types";

export function ExtractFramesDialog({
  open,
  inputFile,
  outputFile,
  loading,
  progress,
  onSelectInput,
  onSelectOutputDir,
  onClose,
  setLoading,
  setProgress,
  setJobId,
}: DialogProps) {
  const [fps, setFps] = useState(1);

  const handleExtractFrames = async () => {
    if (!inputFile || !outputFile) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_extract_frames", {
        inputPath: inputFile,
        outputDir: outputFile,
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
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Extract Frames</DialogTitle>
          <DialogDescription>Extract images from video</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Input Video</Label>
            <div className="flex gap-2">
              <Input value={inputFile} readOnly placeholder="Select a video..." className="flex-1" />
              <Button variant="outline" onClick={onSelectInput}>
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
          <div className="space-y-2">
            <Label>Output Folder</Label>
            <div className="flex gap-2">
              <Input value={outputFile} readOnly placeholder="Select folder..." className="flex-1" />
              <Button variant="outline" onClick={onSelectOutputDir}>
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
          <Button onClick={handleExtractFrames} disabled={!inputFile || !outputFile || loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            Extract
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
