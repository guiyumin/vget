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

const PDF_EXTENSIONS = ["pdf"];

export function MergePdfPanel({ outputDir, loading, setLoading }: PdfPanelProps) {
  const [files, setFiles] = useState<string[]>([]);

  const outputPath =
    files.length > 0 ? generateOutputPath(outputDir, "merged", undefined) : "";

  const { ref: dropZoneRef, isDragging } = useDropZone<HTMLDivElement>({
    accept: PDF_EXTENSIONS,
    onDrop: (paths) => {
      setFiles((prev) => [...prev, ...paths]);
    },
    onInvalidDrop: () => {
      toast.error("Please drop PDF files only");
    },
    enabled: !loading,
  });

  const selectFiles = async () => {
    const selected = await open({
      multiple: true,
      filters: [{ name: "PDF", extensions: ["pdf"] }],
    });
    if (selected) {
      const newFiles = Array.isArray(selected) ? selected : [selected];
      setFiles((prev) => [...prev, ...newFiles]);
    }
  };

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const moveFile = (from: number, to: number) => {
    if (to < 0 || to >= files.length) return;
    setFiles((prev) => {
      const newFiles = [...prev];
      const [removed] = newFiles.splice(from, 1);
      newFiles.splice(to, 0, removed);
      return newFiles;
    });
  };

  const handleMerge = async () => {
    if (files.length < 2 || !outputDir) return;
    setLoading(true);
    try {
      await invoke("pdf_merge", {
        inputPaths: files,
        outputPath,
      });
      toast.success("PDFs merged successfully!");
      setFiles([]);
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
        <Label>PDF Files (drag to reorder)</Label>
        <div className="space-y-2">
          {files.map((file, index) => (
            <div
              key={`${file}-${index}`}
              className="flex items-center gap-2 p-2 bg-muted rounded-md"
            >
              <div className="flex flex-col gap-0.5">
                <button
                  onClick={() => moveFile(index, index - 1)}
                  disabled={index === 0}
                  className="p-0.5 hover:bg-background rounded disabled:opacity-30"
                >
                  <GripVertical className="h-3 w-3 rotate-180" />
                </button>
                <button
                  onClick={() => moveFile(index, index + 1)}
                  disabled={index === files.length - 1}
                  className="p-0.5 hover:bg-background rounded disabled:opacity-30"
                >
                  <GripVertical className="h-3 w-3" />
                </button>
              </div>
              <span className="text-sm text-muted-foreground w-6">{index + 1}.</span>
              <span className="flex-1 text-sm truncate" title={file}>
                {getFileName(file)}
              </span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => removeFile(index)}
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
            onClick={selectFiles}
            disabled={loading}
            className={cn(
              "w-full",
              isDragging && "border-primary bg-primary/5"
            )}
          >
            <Plus className="h-4 w-4 mr-2" />
            {isDragging ? "Drop PDF files here..." : "Add PDF Files or Drop Here"}
          </Button>
        </div>
      </div>

      {files.length > 0 && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>
            {outputPath}
          </p>
        </div>
      )}

      <div className="pt-2">
        <Button
          onClick={handleMerge}
          disabled={files.length < 2 || !outputDir || loading}
        >
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Merge {files.length > 0 ? `(${files.length} files)` : ""}
        </Button>
      </div>
    </div>
  );
}
