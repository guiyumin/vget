import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FileDropInput } from "@/components/ui/file-drop-input";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PdfPanelProps, PdfInfo, getBasename, generateOutputPath } from "../types";

const PDF_EXTENSIONS = ["pdf"];

export function DeletePagesPanel({ outputDir, loading, setLoading }: PdfPanelProps) {
  const [inputFile, setInputFile] = useState("");
  const [pdfInfo, setPdfInfo] = useState<PdfInfo | null>(null);
  const [pagesToDelete, setPagesToDelete] = useState("");

  const outputPath = inputFile
    ? generateOutputPath(outputDir, getBasename(inputFile), "pages_removed")
    : "";

  const handleFileSelected = async (file: string) => {
    setInputFile(file);
    setPagesToDelete("");
    try {
      const info = await invoke<PdfInfo>("pdf_get_info", { inputPath: file });
      setPdfInfo(info);
    } catch (e) {
      console.error("Failed to get PDF info:", e);
      setPdfInfo(null);
    }
  };

  const selectFile = async () => {
    const selected = await open({
      multiple: false,
      filters: [{ name: "PDF", extensions: ["pdf"] }],
    });
    if (selected) {
      await handleFileSelected(selected);
    }
  };

  const parsePages = (input: string): number[] => {
    const pages: Set<number> = new Set();
    const parts = input.split(",").map((s) => s.trim());

    for (const part of parts) {
      if (part.includes("-")) {
        const [start, end] = part.split("-").map((s) => parseInt(s.trim(), 10));
        if (!isNaN(start) && !isNaN(end)) {
          for (let i = Math.min(start, end); i <= Math.max(start, end); i++) {
            pages.add(i);
          }
        }
      } else {
        const num = parseInt(part, 10);
        if (!isNaN(num)) {
          pages.add(num);
        }
      }
    }

    return Array.from(pages).sort((a, b) => a - b);
  };

  const handleDelete = async () => {
    if (!inputFile || !outputDir || !pagesToDelete.trim()) return;

    const pages = parsePages(pagesToDelete);
    if (pages.length === 0) {
      toast.error("Please enter valid page numbers");
      return;
    }

    if (pdfInfo && pages.length >= pdfInfo.pages) {
      toast.error("Cannot delete all pages from PDF");
      return;
    }

    setLoading(true);
    try {
      await invoke("pdf_delete_pages", {
        inputPath: inputFile,
        outputPath,
        pages,
      });
      toast.success(`Deleted ${pages.length} page(s) successfully!`);
      setInputFile("");
      setPdfInfo(null);
      setPagesToDelete("");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  const parsedPages = parsePages(pagesToDelete);

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>Input PDF</Label>
        <FileDropInput
          value={inputFile}
          placeholder="Drop a PDF here or click to select"
          accept={PDF_EXTENSIONS}
          acceptHint=".pdf"
          onSelectClick={selectFile}
          onDrop={handleFileSelected}
          disabled={loading}
          invalidDropMessage="Please drop a PDF file"
        />
      </div>

      {pdfInfo && (
        <div className="p-3 bg-muted rounded-md space-y-1">
          <p className="text-sm">
            <span className="text-muted-foreground">Total pages:</span>{" "}
            <span className="font-medium">{pdfInfo.pages}</span>
          </p>
          {pdfInfo.title && (
            <p className="text-sm">
              <span className="text-muted-foreground">Title:</span> {pdfInfo.title}
            </p>
          )}
        </div>
      )}

      <div className="space-y-2">
        <Label>Pages to Delete</Label>
        <Input
          value={pagesToDelete}
          onChange={(e) => setPagesToDelete(e.target.value)}
          placeholder="e.g., 1, 3, 5-7"
          disabled={!inputFile}
        />
        <p className="text-xs text-muted-foreground">
          Enter page numbers separated by commas. Use ranges like 5-7 for consecutive pages.
        </p>
        {parsedPages.length > 0 && (
          <p className="text-xs text-muted-foreground">
            Will delete pages: {parsedPages.join(", ")}
          </p>
        )}
      </div>

      {inputFile && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>
            {outputPath}
          </p>
        </div>
      )}

      <div className="pt-2">
        <Button
          onClick={handleDelete}
          disabled={!inputFile || !outputDir || parsedPages.length === 0 || loading}
        >
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Delete Pages
        </Button>
      </div>
    </div>
  );
}
