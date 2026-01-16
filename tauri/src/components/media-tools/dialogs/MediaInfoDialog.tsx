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
import { FolderOpen } from "lucide-react";
import { DialogProps, formatBytes, formatDuration } from "../types";

export function MediaInfoDialog({
  open,
  inputFile,
  mediaInfo,
  onSelectInput,
  onClose,
}: DialogProps) {
  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Media Info</DialogTitle>
          <DialogDescription>
            View detailed information about a media file
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="flex gap-2">
            <Input
              value={inputFile}
              readOnly
              placeholder="Select a file..."
              className="flex-1"
            />
            <Button variant="outline" onClick={onSelectInput}>
              <FolderOpen className="h-4 w-4" />
            </Button>
          </div>
          {mediaInfo && (
            <div className="space-y-3 text-sm">
              <div className="p-3 bg-muted rounded-lg space-y-2">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Format</span>
                  <span>{mediaInfo.format_long_name || mediaInfo.format_name}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Size</span>
                  <span>{formatBytes(mediaInfo.size)}</span>
                </div>
                {mediaInfo.duration && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Duration</span>
                    <span>{formatDuration(mediaInfo.duration)}</span>
                  </div>
                )}
                {mediaInfo.bit_rate && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Bitrate</span>
                    <span>{Math.round(mediaInfo.bit_rate / 1000)} kbps</span>
                  </div>
                )}
              </div>
              {mediaInfo.streams.map((stream) => (
                <div key={stream.index} className="p-3 bg-muted rounded-lg space-y-2">
                  <div className="font-medium capitalize">{stream.codec_type} Stream</div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Codec</span>
                    <span>{stream.codec_long_name || stream.codec_name}</span>
                  </div>
                  {stream.width && stream.height && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Resolution</span>
                      <span>{stream.width}x{stream.height}</span>
                    </div>
                  )}
                  {stream.sample_rate && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Sample Rate</span>
                      <span>{stream.sample_rate} Hz</span>
                    </div>
                  )}
                  {stream.channels && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Channels</span>
                      <span>{stream.channels}</span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
