import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { revealItemInDir } from "@tauri-apps/plugin-opener";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FileText, Loader2, Upload, FolderOpen, CheckCircle2 } from "lucide-react";
import { toast } from "sonner";
import { PdfPanelProps, getBasename } from "../types";
import { cn } from "@/lib/utils";
import { useDropZone } from "@/hooks/useDropZone";

export function Md2PdfPanel({ outputDir, loading, setLoading }: PdfPanelProps) {
  const { t } = useTranslation();
  const [inputFile, setInputFile] = useState("");
  const [pageSize, setPageSize] = useState("A4");
  const [generatedPdf, setGeneratedPdf] = useState("");

  const outputPath = inputFile
    ? `${outputDir}/${getBasename(inputFile)}.pdf`
    : "";

  // Drop zone for markdown files
  const { ref: dropZoneRef, isDragging } = useDropZone<HTMLDivElement>({
    accept: ["md", "markdown", "txt"],
    onDrop: (paths) => {
      setInputFile(paths[0]);
    },
    onInvalidDrop: () => {
      toast.error(t("pdfTools.tools.md2pdf.invalidFile") || "Please drop a Markdown file (.md, .markdown, .txt)");
    },
    enabled: !inputFile, // Only enable when no file is selected
  });

  const selectFile = async () => {
    const selected = await open({
      multiple: false,
      filters: [{ name: "Markdown", extensions: ["md", "markdown", "txt"] }],
    });
    if (selected && typeof selected === "string") {
      setInputFile(selected);
    }
  };

  const handleConvert = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    try {
      await invoke("md_to_pdf", {
        inputPath: inputFile,
        outputPath,
        theme: "light",
        pageSize,
      });
      toast.success(t("pdfTools.tools.md2pdf.success"));
      setGeneratedPdf(outputPath);
      setInputFile("");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  const handleRevealPdf = async () => {
    if (!generatedPdf) return;
    try {
      await revealItemInDir(generatedPdf);
    } catch (e) {
      toast.error(String(e));
    }
  };

  const handleConvertAnother = () => {
    setGeneratedPdf("");
  };

  const getFileName = (path: string) => path.split(/[/\\]/).pop() || path;

  // Show success state if PDF was just generated
  if (generatedPdf) {
    return (
      <div className="space-y-4">
        <div className="flex flex-col items-center justify-center py-6 space-y-4">
          <div className="flex items-center justify-center w-12 h-12 rounded-full bg-green-100 dark:bg-green-900/30">
            <CheckCircle2 className="h-6 w-6 text-green-600 dark:text-green-400" />
          </div>
          <div className="text-center space-y-1">
            <p className="font-medium">{t("pdfTools.tools.md2pdf.success")}</p>
            <p className="text-sm text-muted-foreground">
              {t("pdfTools.tools.md2pdf.clickToReveal") || "Click below to reveal the file"}
            </p>
          </div>
        </div>

        <div
          onClick={handleRevealPdf}
          className="flex items-center gap-3 p-3 bg-muted rounded-md cursor-pointer hover:bg-muted/80 transition-colors group"
        >
          <FileText className="h-5 w-5 shrink-0 text-red-500" />
          <span className="flex-1 text-sm truncate" title={generatedPdf}>
            {getFileName(generatedPdf)}
          </span>
          <FolderOpen className="h-4 w-4 text-muted-foreground group-hover:text-foreground transition-colors" />
        </div>

        <div className="pt-2">
          <Button variant="outline" onClick={handleConvertAnother} className="w-full">
            {t("pdfTools.tools.md2pdf.convertAnother") || "Convert Another"}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>{t("pdfTools.tools.md2pdf.inputFile")}</Label>
        {inputFile ? (
          <div className="flex items-center gap-2 p-3 bg-muted rounded-md">
            <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="flex-1 text-sm truncate" title={inputFile}>
              {getFileName(inputFile)}
            </span>
            <Button variant="ghost" size="sm" onClick={selectFile}>
              {t("pdfTools.tools.md2pdf.change")}
            </Button>
          </div>
        ) : (
          <div
            ref={dropZoneRef}
            onClick={selectFile}
            className={cn(
              "border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors",
              isDragging
                ? "border-primary bg-primary/5"
                : "border-muted-foreground/25 hover:border-muted-foreground/50 hover:bg-muted/50"
            )}
          >
            <Upload className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {t("pdfTools.tools.md2pdf.dropHint") || "Drop a Markdown file here or click to select"}
            </p>
            <p className="text-xs text-muted-foreground/70 mt-1">
              .md, .markdown, .txt
            </p>
          </div>
        )}
      </div>

      <div className="space-y-2">
        <Label>{t("pdfTools.tools.md2pdf.pageSize")}</Label>
        <Select value={pageSize} onValueChange={setPageSize}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="A4">A4</SelectItem>
            <SelectItem value="Letter">Letter</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {inputFile && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">
            {t("pdfTools.tools.md2pdf.output")}
          </Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>
            {outputPath}
          </p>
        </div>
      )}

      <div className="pt-2">
        <Button onClick={handleConvert} disabled={!inputFile || !outputDir || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          {t("pdfTools.tools.md2pdf.convert")}
        </Button>
      </div>
    </div>
  );
}
