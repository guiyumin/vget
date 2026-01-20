export interface PdfInfo {
  path: string;
  pages: number;
  title: string | null;
  author: string | null;
}

export interface WatermarkRemovalResult {
  success: boolean;
  items_removed: number;
  message: string;
}

export interface Config {
  output_dir: string;
}

export type PdfToolId = "merge" | "images-to-pdf" | "delete-pages" | "remove-watermark" | "md-to-pdf";

export interface PdfPanelProps {
  outputDir: string;
  loading: boolean;
  setLoading: (loading: boolean) => void;
}

export function getBasename(filePath: string): string {
  const name = filePath.split(/[/\\]/).pop() || "";
  const lastDot = name.lastIndexOf(".");
  return lastDot > 0 ? name.substring(0, lastDot) : name;
}

export function generateOutputPath(
  outputDir: string,
  baseName: string,
  suffix?: string
): string {
  const safeName = baseName.replace(/[/\\?%*:|"<>]/g, "-");
  const suffixStr = suffix ? `_${suffix}` : "";
  return `${outputDir}/${safeName}${suffixStr}.pdf`;
}
