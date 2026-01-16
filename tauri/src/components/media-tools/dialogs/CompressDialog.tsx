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
import { DialogProps, generateOutputPath } from "../types";

export function CompressDialog({
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
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Compress Video</DialogTitle>
          <DialogDescription>Reduce video file size</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Input File</Label>
            <div className="flex gap-2">
              <Input value={inputFile} readOnly placeholder="Select a video..." className="flex-1" />
              <Button variant="outline" onClick={onSelectInput}>
                <FolderOpen className="h-4 w-4" />
              </Button>
            </div>
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
              <p className="text-sm text-muted-foreground truncate">{outputPath}</p>
            </div>
          )}
          {loading && <Progress value={progress} />}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleCompress} disabled={!inputFile || !outputDir || loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            Compress
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
