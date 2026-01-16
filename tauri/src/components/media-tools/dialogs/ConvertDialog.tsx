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
import { DialogProps, generateOutputPath } from "../types";

export function ConvertDialog({
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
  const [outputFormat, setOutputFormat] = useState("mp4");

  const outputPath = inputFile ? generateOutputPath(outputDir, inputFile, outputFormat, "converted") : "";

  const handleConvert = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setProgress(0);
    try {
      const id = await invoke<string>("ffmpeg_convert_video", {
        inputPath: inputFile,
        outputPath,
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
          <DialogTitle>Convert Video</DialogTitle>
          <DialogDescription>
            Convert video to a different format
          </DialogDescription>
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
          <div className="space-y-2">
            <Label>Output Format</Label>
            <Select value={outputFormat} onValueChange={setOutputFormat}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="mp4">MP4</SelectItem>
                <SelectItem value="mkv">MKV</SelectItem>
                <SelectItem value="webm">WebM</SelectItem>
                <SelectItem value="mov">MOV</SelectItem>
              </SelectContent>
            </Select>
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
          <Button onClick={handleConvert} disabled={!inputFile || !outputDir || loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            Convert
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
