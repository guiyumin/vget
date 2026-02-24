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

// Auth APIs

export interface AuthStatusData {
  api_key_configured: boolean;
}

export interface GenerateTokenData {
  jwt: string;
}

export async function fetchAuthStatus(): Promise<ApiResponse<AuthStatusData>> {
  const res = await fetch("/api/auth/status");
  return res.json();
}

export async function generateApiToken(): Promise<ApiResponse<GenerateTokenData>> {
  const res = await fetch("/api/auth/token", { method: "POST" });
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

// History APIs

export interface HistoryRecord {
  id: string;
  url: string;
  filename: string;
  status: "completed" | "failed";
  size_bytes: number;
  started_at: number;   // Unix timestamp
  completed_at: number; // Unix timestamp
  duration_seconds: number;
  error?: string;
}

export interface HistoryStats {
  completed: number;
  failed: number;
  total_bytes: number;
}

export interface HistoryData {
  records: HistoryRecord[];
  total: number;
  limit: number;
  offset: number;
  stats: HistoryStats;
}

export async function fetchHistory(
  limit = 50,
  offset = 0
): Promise<ApiResponse<HistoryData>> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const res = await fetch(`/api/history?${params}`);
  return res.json();
}

export async function deleteHistoryRecord(
  id: string
): Promise<ApiResponse<{ id: string }>> {
  const res = await fetch(`/api/history/${id}`, { method: "DELETE" });
  return res.json();
}

export async function clearAllHistory(): Promise<
  ApiResponse<{ cleared: number }>
> {
  const res = await fetch("/api/history", { method: "DELETE" });
  return res.json();
}
