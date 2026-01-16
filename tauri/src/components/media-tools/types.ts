export interface MediaInfo {
  filename: string;
  format_name: string;
  format_long_name: string;
  duration: number | null;
  size: number;
  bit_rate: number | null;
  streams: StreamInfo[];
}

export interface StreamInfo {
  index: number;
  codec_type: string;
  codec_name: string;
  codec_long_name: string | null;
  width: number | null;
  height: number | null;
  sample_rate: string | null;
  channels: number | null;
  bit_rate: string | null;
  duration: string | null;
}

export type ToolId =
  | "info"
  | "convert"
  | "compress"
  | "trim"
  | "extract-audio"
  | "extract-frames"
  | "audio-convert";

export interface DialogProps {
  open: boolean;
  inputFile: string;
  outputFile: string;
  loading: boolean;
  progress: number;
  mediaInfo: MediaInfo | null;
  onSelectInput: () => Promise<void>;
  onSelectOutput: (ext: string) => Promise<void>;
  onSelectOutputDir: () => Promise<void>;
  onClose: () => void;
  setLoading: (loading: boolean) => void;
  setProgress: (progress: number) => void;
  setJobId: (jobId: string | null) => void;
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

export function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  return `${m}:${s.toString().padStart(2, "0")}`;
}
