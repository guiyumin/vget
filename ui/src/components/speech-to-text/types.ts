// Step keys matching backend
export type StepKey =
  | "extract_audio"
  | "compress_audio"
  | "chunk_audio"
  | "transcribe"
  | "merge"
  | "summarize";

// Step status matching backend
export type StepStatus =
  | "pending"
  | "in_progress"
  | "completed"
  | "skipped"
  | "failed";

// Job status matching backend
export type AIJobStatus =
  | "queued"
  | "processing"
  | "completed"
  | "failed"
  | "cancelled";

// Step definition
export interface ProcessingStep {
  key: StepKey;
  name: string;
  status: StepStatus;
  progress: number;
  detail?: string;
  started_at?: string;
  finished_at?: string;
}

// Job result from backend
export interface AIJobResult {
  transcript_path?: string;
  summary_path?: string;
  raw_text?: string;
  summary?: string;
}

// Full AI job from backend
export interface AIJob {
  id: string;
  file_path: string;
  file_name: string;
  status: AIJobStatus;
  current_step: StepKey;
  steps: ProcessingStep[];
  overall_progress: number;
  result?: AIJobResult;
  error?: string;
  created_at: string;
  updated_at: string;
}

// Processing configuration
export interface ProcessingConfig {
  account: string;
  model: string;
  includeSummary: boolean;
}

// Selected file
export interface SelectedFile {
  path: string;
  filename: string;
  size: number;
  source: "downloaded" | "uploaded";
  has_transcript?: boolean;
  has_summary?: boolean;
}

// Audio file from API
export interface AudioFile {
  name: string;
  path: string;
  full_path: string;
  size: number;
  mod_time: string;
  has_transcript: boolean;
  has_summary: boolean;
}

// Processing state for reducer
export interface ProcessingState {
  status: "idle" | "processing" | "completed" | "error";
  jobId: string | null;
  steps: ProcessingStep[];
  currentStepIndex: number;
  result: AIJobResult | null;
  error: string | null;
}

// Default steps template
export const DEFAULT_STEPS: ProcessingStep[] = [
  { key: "extract_audio", name: "Extract Audio", status: "pending", progress: 0 },
  { key: "compress_audio", name: "Compress Audio", status: "pending", progress: 0 },
  { key: "chunk_audio", name: "Chunk Audio", status: "pending", progress: 0 },
  { key: "transcribe", name: "Transcribe", status: "pending", progress: 0 },
  { key: "merge", name: "Merge Chunks", status: "pending", progress: 0 },
  { key: "summarize", name: "Generate Summary", status: "pending", progress: 0 },
];
