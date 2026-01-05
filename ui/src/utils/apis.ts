import type { UITranslations, ServerTranslations } from "./translations";

export type JobStatus =
  | "queued"
  | "downloading"
  | "completed"
  | "failed"
  | "cancelled";

export interface Job {
  id: string;
  url: string;
  status: JobStatus;
  progress: number;
  downloaded: number;
  total: number;
  filename?: string;
  error?: string;
}

export interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

export interface HealthData {
  status: string;
  version: string;
}

export interface WebDAVServer {
  url: string;
  username: string;
  password: string;
}

export interface ConfigData {
  output_dir: string;
  language: string;
  format: string;
  quality: string;
  twitter_auth_token: string;
  server_port: number;
  server_max_concurrent: number;
  server_api_key: string;
  webdav_servers: Record<string, WebDAVServer>;
  express?: Record<string, Record<string, string>>;
  torrent_enabled?: boolean;
  bilibili_cookie?: string;
}

export interface TorrentConfig {
  enabled: boolean;
  client: string;
  host: string;
  username: string;
  password: string;
  use_https: boolean;
  default_save_path: string;
}

export interface TorrentAddResult {
  id: string;
  hash: string;
  name: string;
  duplicate: boolean;
}

export interface JobsData {
  jobs: Job[];
}

export interface I18nData {
  language: string;
  ui: UITranslations;
  server: ServerTranslations;
  config_exists: boolean;
}

export async function fetchHealth(): Promise<ApiResponse<HealthData>> {
  const res = await fetch("/api/health");
  return res.json();
}

export async function fetchJobs(): Promise<ApiResponse<JobsData>> {
  const res = await fetch("/api/jobs");
  return res.json();
}

export async function fetchConfig(): Promise<ApiResponse<ConfigData>> {
  const res = await fetch("/api/config");
  return res.json();
}

export async function fetchI18n(): Promise<ApiResponse<I18nData>> {
  const res = await fetch("/api/i18n");
  return res.json();
}

export async function updateConfig(
  outputDir: string
): Promise<ApiResponse<ConfigData>> {
  const res = await fetch("/api/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ output_dir: outputDir }),
  });
  return res.json();
}

export async function setConfigValue(
  key: string,
  value: string
): Promise<ApiResponse<{ key: string; value: string }>> {
  const res = await fetch("/api/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key, value }),
  });
  return res.json();
}

export async function postDownload(
  url: string,
  filename?: string
): Promise<ApiResponse<{ id: string; status: string }>> {
  const res = await fetch("/api/download", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, filename }),
  });
  return res.json();
}

export interface BulkDownloadJob {
  id: string;
  url: string;
  status: string;
  error?: string;
}

export interface BulkDownloadResult {
  jobs: BulkDownloadJob[];
  queued: number;
  failed: number;
}

export async function postBulkDownload(
  urls: string[]
): Promise<ApiResponse<BulkDownloadResult>> {
  const res = await fetch("/api/bulk-download", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ urls }),
  });
  return res.json();
}

export async function addWebDAVServer(
  name: string,
  url: string,
  username: string,
  password: string
): Promise<ApiResponse<{ name: string }>> {
  const res = await fetch("/api/config/webdav", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, url, username, password }),
  });
  return res.json();
}

export async function deleteWebDAVServer(
  name: string
): Promise<ApiResponse<{ name: string }>> {
  const res = await fetch(`/api/config/webdav/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
  return res.json();
}

export async function deleteJob(
  id: string
): Promise<ApiResponse<{ id: string }>> {
  const res = await fetch(`/api/jobs/${id}`, { method: "DELETE" });
  return res.json();
}

export async function clearHistory(): Promise<
  ApiResponse<{ cleared: number }>
> {
  const res = await fetch("/api/jobs", { method: "DELETE" });
  return res.json();
}

// Torrent APIs

export async function fetchTorrentConfig(): Promise<
  ApiResponse<TorrentConfig>
> {
  const res = await fetch("/api/config/torrent");
  return res.json();
}

export async function saveTorrentConfig(
  config: TorrentConfig
): Promise<ApiResponse<{ enabled: boolean }>> {
  const res = await fetch("/api/config/torrent", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(config),
  });
  return res.json();
}

export async function testTorrentConnection(): Promise<
  ApiResponse<{ client: string }>
> {
  const res = await fetch("/api/config/torrent/test", {
    method: "POST",
  });
  return res.json();
}

export async function addTorrent(
  url: string,
  savePath?: string
): Promise<ApiResponse<TorrentAddResult>> {
  const res = await fetch("/api/torrent", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, save_path: savePath }),
  });
  return res.json();
}

// WebDAV Browsing APIs

export interface WebDAVRemote {
  name: string;
  url: string;
  hasAuth: boolean;
}

export interface WebDAVFile {
  name: string;
  path: string;
  size: number;
  isDir: boolean;
}

export interface WebDAVListData {
  remote: string;
  path: string;
  files: WebDAVFile[];
}

export async function fetchWebDAVRemotes(): Promise<
  ApiResponse<{ remotes: WebDAVRemote[] }>
> {
  const res = await fetch("/api/webdav/remotes");
  return res.json();
}

export async function fetchWebDAVList(
  remote: string,
  path: string
): Promise<ApiResponse<WebDAVListData>> {
  const params = new URLSearchParams({ remote, path });
  const res = await fetch(`/api/webdav/list?${params}`);
  return res.json();
}

export async function submitWebDAVDownload(
  remote: string,
  files: string[]
): Promise<ApiResponse<{ jobIds: string[]; count: number }>> {
  const res = await fetch("/api/webdav/download", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ remote, files }),
  });
  return res.json();
}

// Podcast APIs

export interface PodcastChannel {
  id: string;
  title: string;
  author: string;
  description: string;
  episode_count: number;
  feed_url?: string;
  source: "xiaoyuzhou" | "itunes";
}

export interface PodcastEpisode {
  id: string;
  title: string;
  podcast_name: string;
  duration: number;
  pub_date?: string;
  download_url: string;
  source: "xiaoyuzhou" | "itunes";
}

export interface PodcastSearchResult {
  source: "xiaoyuzhou" | "itunes";
  podcasts: PodcastChannel[];
  episodes: PodcastEpisode[];
}

export async function searchPodcasts(
  query: string,
  lang?: string
): Promise<ApiResponse<{ results: PodcastSearchResult[] }>> {
  const res = await fetch("/api/podcast/search", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, lang }),
  });
  return res.json();
}

export async function fetchPodcastEpisodes(
  podcastId: string,
  source: "xiaoyuzhou" | "itunes"
): Promise<ApiResponse<{ podcast_title: string; episodes: PodcastEpisode[] }>> {
  const res = await fetch("/api/podcast/episodes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ podcast_id: podcastId, source }),
  });
  return res.json();
}

// AI APIs

export interface AIAccount {
  label: string;
  provider: string;
  is_encrypted: boolean;
  is_default: boolean;
}

export interface AIConfigData {
  accounts: AIAccount[];
  default_account: string;
}

export interface AIModel {
  id: string;
  name: string;
  description: string;
  tier: string;
}

export interface AIModelsData {
  summarization: {
    openai: AIModel[];
    anthropic: AIModel[];
    qwen: AIModel[];
    default: string;
  };
  transcription: {
    openai: string[];
    anthropic: string[];
    qwen: string[];
  };
}

export interface AudioFile {
  name: string;
  path: string;
  full_path: string;
  size: number;
  mod_time: string;
  has_transcript: boolean;
  has_summary: boolean;
}

export interface TranscribeResult {
  text: string;
  output_path: string;
  duration: number;
  language: string;
}

export interface SummarizeResult {
  summary: string;
  key_points: string[];
  output_path: string;
}

export async function fetchAIConfig(): Promise<ApiResponse<AIConfigData>> {
  const res = await fetch("/api/ai/config");
  return res.json();
}

export async function fetchAIModels(): Promise<ApiResponse<AIModelsData>> {
  const res = await fetch("/api/ai/models");
  return res.json();
}

export async function addAIAccount(params: {
  label: string;
  provider: string;
  api_key: string;
  pin?: string;
}): Promise<ApiResponse<{ label: string }>> {
  const res = await fetch("/api/ai/config/account", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  return res.json();
}

export async function deleteAIAccount(
  label: string
): Promise<ApiResponse<{ label: string }>> {
  const res = await fetch(`/api/ai/config/account/${encodeURIComponent(label)}`, {
    method: "DELETE",
  });
  return res.json();
}

export async function setDefaultAIAccount(
  label: string
): Promise<ApiResponse<{ default_account: string }>> {
  const res = await fetch("/api/ai/config/default", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ label }),
  });
  return res.json();
}

export async function fetchAudioFiles(): Promise<
  ApiResponse<{ files: AudioFile[]; output_dir: string }>
> {
  const res = await fetch("/api/ai/files");
  return res.json();
}

export async function uploadAudioFile(
  file: File
): Promise<ApiResponse<{ path: string; filename: string; size: number }>> {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch("/api/ai/upload", {
    method: "POST",
    body: formData,
  });
  return res.json();
}

export async function transcribeAudio(params: {
  file_path: string;
  account?: string;
  model?: string;
  pin?: string;
}): Promise<ApiResponse<TranscribeResult>> {
  const res = await fetch("/api/ai/transcribe", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  return res.json();
}

export async function summarizeText(params: {
  file_path?: string;
  text?: string;
  account?: string;
  model?: string;
  pin?: string;
}): Promise<ApiResponse<SummarizeResult>> {
  const res = await fetch("/api/ai/summarize", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  return res.json();
}

// AI Processing Job Types
export type AIJobStatus =
  | "queued"
  | "processing"
  | "completed"
  | "failed"
  | "cancelled";

export type StepStatus =
  | "pending"
  | "in_progress"
  | "completed"
  | "skipped"
  | "failed";

export interface ProcessingStep {
  key: string;
  name: string;
  status: StepStatus;
  progress: number;
  detail?: string;
  started_at?: string;
  finished_at?: string;
}

export interface AIJobResult {
  transcript_path?: string;
  summary_path?: string;
  raw_text?: string;
  summary?: string;
}

export interface AIJob {
  id: string;
  file_path: string;
  file_name: string;
  status: AIJobStatus;
  current_step: string;
  steps: ProcessingStep[];
  overall_progress: number;
  result?: AIJobResult;
  error?: string;
  created_at: string;
  updated_at: string;
}

// AI Processing Job API Functions
export async function startAIProcessing(params: {
  file_path: string;
  account?: string;
  transcription_model: string; // Model name (e.g., "whisper-medium", "whisper-1")
  summarization_model?: string;
  pin?: string;
  include_summary: boolean;
  audio_language?: string; // Language of the audio (e.g., "zh", "en")
  output_format?: string; // Output format: "md", "srt", "vtt"
  summary_language?: string; // Language for the summary output (e.g., "zh", "en")
}): Promise<ApiResponse<{ job_id: string; status: AIJobStatus }>> {
  const res = await fetch("/api/ai/process", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  return res.json();
}

export async function getAIJob(id: string): Promise<ApiResponse<AIJob>> {
  const res = await fetch(`/api/ai/jobs/${id}`);
  return res.json();
}

export async function getAIJobs(): Promise<ApiResponse<{ jobs: AIJob[] }>> {
  const res = await fetch("/api/ai/jobs");
  return res.json();
}

export async function cancelAIJob(
  id: string
): Promise<ApiResponse<{ id: string }>> {
  const res = await fetch(`/api/ai/jobs/${id}`, {
    method: "DELETE",
  });
  return res.json();
}

export async function clearAIJobs(): Promise<
  ApiResponse<{ cleared: number }>
> {
  const res = await fetch("/api/ai/jobs", {
    method: "DELETE",
  });
  return res.json();
}

// Local ASR APIs

export interface LocalASRModel {
  name: string;
  engine: string;
  size: string;
  description: string;
  languages: number;
  downloaded: boolean;
}

export interface LocalASRCapabilities {
  available: boolean;
  service_url: string;
  enabled: boolean;
  current_model?: string;
  gpu?: {
    type: "nvidia" | "metal" | "none";
    name: string;
    memory_gb?: number;
    cuda_version?: string;
  };
  models?: LocalASRModel[];
  default_model?: string;
  error?: string;
  message?: string;
}

export async function fetchLocalASRCapabilities(): Promise<
  ApiResponse<LocalASRCapabilities>
> {
  const res = await fetch("/api/ai/local-asr/capabilities");
  return res.json();
}

export async function updateLocalASRConfig(params: {
  enabled?: boolean;
  service_url?: string;
  model?: string;
}): Promise<
  ApiResponse<{ enabled: boolean; service_url: string; model: string }>
> {
  const res = await fetch("/api/ai/local-asr/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  return res.json();
}
