import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Loader2, Plus, X, GripVertical } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { useDropZone } from "@/hooks/useDropZone";
import { PdfPanelProps, generateOutputPath } from "../types";

const IMAGE_EXTENSIONS = ["png", "jpg", "jpeg", "gif", "bmp", "webp"];

export function ImagesToPdfPanel({ outputDir, loading, setLoading }: PdfPanelProps) {
  const [images, setImages] = useState<string[]>([]);

  const outputPath =
    images.length > 0 ? generateOutputPath(outputDir, "images", undefined) : "";

  const { ref: dropZoneRef, isDragging } = useDropZone<HTMLDivElement>({
    accept: IMAGE_EXTENSIONS,
    onDrop: (paths) => {
      setImages((prev) => [...prev, ...paths]);
    },
    onInvalidDrop: () => {
      toast.error("Please drop image files only (png, jpg, gif, bmp, webp)");
    },
    enabled: !loading,
  });

  const selectImages = async () => {
    const selected = await open({
      multiple: true,
      filters: [{ name: "Images", extensions: ["png", "jpg", "jpeg", "gif", "bmp", "webp"] }],
    });
    if (selected) {
      const newImages = Array.isArray(selected) ? selected : [selected];
      setImages((prev) => [...prev, ...newImages]);
    }
  };

  const removeImage = (index: number) => {
    setImages((prev) => prev.filter((_, i) => i !== index));
  };

  const moveImage = (from: number, to: number) => {
    if (to < 0 || to >= images.length) return;
    setImages((prev) => {
      const newImages = [...prev];
      const [removed] = newImages.splice(from, 1);
      newImages.splice(to, 0, removed);
      return newImages;
    });
  };

  const handleConvert = async () => {
    if (images.length === 0 || !outputDir) return;
    setLoading(true);
    try {
      await invoke("pdf_images_to_pdf", {
        imagePaths: images,
        outputPath,
      });
      toast.success("Images converted to PDF successfully!");
      setImages([]);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  const getFileName = (path: string) => path.split(/[/\\]/).pop() || path;

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>Images (drag to reorder)</Label>
        <div className="space-y-2">
          {images.map((image, index) => (
            <div
              key={`${image}-${index}`}
              className="flex items-center gap-2 p-2 bg-muted rounded-md"
            >
              <div className="flex flex-col gap-0.5">
                <button
                  onClick={() => moveImage(index, index - 1)}
                  disabled={index === 0}
                  className="p-0.5 hover:bg-background rounded disabled:opacity-30"
                >
                  <GripVertical className="h-3 w-3 rotate-180" />
                </button>
                <button
                  onClick={() => moveImage(index, index + 1)}
                  disabled={index === images.length - 1}
                  className="p-0.5 hover:bg-background rounded disabled:opacity-30"
                >
                  <GripVertical className="h-3 w-3" />
                </button>
              </div>
              <span className="text-sm text-muted-foreground w-6">{index + 1}.</span>
              <span className="flex-1 text-sm truncate" title={image}>
                {getFileName(image)}
              </span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => removeImage(index)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          ))}
        </div>
        <div
          ref={dropZoneRef}
          className={cn(
            "rounded-md transition-all",
            isDragging && "ring-2 ring-primary ring-offset-2 ring-offset-background"
          )}
        >
          <Button
            variant="outline"
            onClick={selectImages}
            disabled={loading}
            className={cn(
              "w-full",
              isDragging && "border-primary bg-primary/5"
            )}
          >
            <Plus className="h-4 w-4 mr-2" />
            {isDragging ? "Drop images here..." : "Add Images or Drop Here"}
          </Button>
        </div>
      </div>

      {images.length > 0 && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>
            {outputPath}
          </p>
        </div>
      )}

      <div className="pt-2">
        <Button
          onClick={handleConvert}
          disabled={images.length === 0 || !outputDir || loading}
        >
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Convert {images.length > 0 ? `(${images.length} images)` : ""}
        </Button>
      </div>
    </div>
  );
}
