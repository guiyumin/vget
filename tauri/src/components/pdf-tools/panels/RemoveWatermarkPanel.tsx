import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { FileDropInput } from "@/components/ui/file-drop-input";
import { Loader2, FlaskConical, Info } from "lucide-react";
import { toast } from "sonner";
import { PdfPanelProps, WatermarkRemovalResult, getBasename, generateOutputPath } from "../types";

const PDF_EXTENSIONS = ["pdf"];

export function RemoveWatermarkPanel({ outputDir, loading, setLoading }: PdfPanelProps) {
  const [inputFile, setInputFile] = useState("");
  const [result, setResult] = useState<WatermarkRemovalResult | null>(null);

  const outputPath = inputFile
    ? generateOutputPath(outputDir, getBasename(inputFile), "no_watermark")
    : "";

  const handleFileSelected = (file: string) => {
    setInputFile(file);
    setResult(null);
  };

  const selectFile = async () => {
    const selected = await open({
      multiple: false,
      filters: [{ name: "PDF", extensions: ["pdf"] }],
    });
    if (selected) {
      handleFileSelected(selected);
    }
  };

  const handleRemove = async () => {
    if (!inputFile || !outputDir) return;
    setLoading(true);
    setResult(null);
    try {
      const res = await invoke<WatermarkRemovalResult>("pdf_remove_watermark", {
        inputPath: inputFile,
        outputPath,
      });
      setResult(res);
      if (res.success) {
        toast.success("Watermark removal completed!");
      } else {
        toast.info("No watermarks detected");
      }
    } catch (e) {
      toast.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Beta notice */}
      <Alert className="border-amber-500/50 bg-amber-500/10">
        <FlaskConical className="h-4 w-4 text-amber-500" />
        <AlertDescription className="text-sm">
          <span className="font-medium text-amber-600 dark:text-amber-400">Beta Feature</span>
          <p className="mt-1 text-muted-foreground">
            This tool can remove <span className="font-medium">some</span> watermarks, but not all.
            It works best with overlay-type watermarks (text or images added as separate layers).
            Watermarks that are "baked into" the page content may not be removable.
          </p>
          <p className="mt-2 text-muted-foreground">
            Give it a try - it might just work for your PDF!
          </p>
        </AlertDescription>
      </Alert>

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

      {inputFile && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">Output</Label>
          <p className="text-sm text-muted-foreground break-all" title={outputPath}>
            {outputPath}
          </p>
        </div>
      )}

      {result && (
        <Alert className={result.success ? "border-green-500/50 bg-green-500/10" : "border-blue-500/50 bg-blue-500/10"}>
          <Info className={`h-4 w-4 ${result.success ? "text-green-500" : "text-blue-500"}`} />
          <AlertDescription className="text-sm">
            {result.message}
          </AlertDescription>
        </Alert>
      )}

      <div className="pt-2">
        <Button
          onClick={handleRemove}
          disabled={!inputFile || !outputDir || loading}
        >
          {loading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
          Try to Remove Watermark
        </Button>
      </div>
    </div>
  );
}
