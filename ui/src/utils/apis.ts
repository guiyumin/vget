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
  const res = await fetch("/health");
  return res.json();
}

export async function fetchJobs(): Promise<ApiResponse<JobsData>> {
  const res = await fetch("/jobs");
  return res.json();
}

export async function fetchConfig(): Promise<ApiResponse<ConfigData>> {
  const res = await fetch("/config");
  return res.json();
}

export async function fetchI18n(): Promise<ApiResponse<I18nData>> {
  const res = await fetch("/i18n");
  return res.json();
}

export async function updateConfig(
  outputDir: string
): Promise<ApiResponse<ConfigData>> {
  const res = await fetch("/config", {
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
  const res = await fetch("/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key, value }),
  });
  return res.json();
}

export async function postDownload(
  url: string
): Promise<ApiResponse<{ id: string; status: string }>> {
  const res = await fetch("/download", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
  return res.json();
}

export async function addWebDAVServer(
  name: string,
  url: string,
  username: string,
  password: string
): Promise<ApiResponse<{ name: string }>> {
  const res = await fetch("/config/webdav", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, url, username, password }),
  });
  return res.json();
}

export async function deleteWebDAVServer(
  name: string
): Promise<ApiResponse<{ name: string }>> {
  const res = await fetch(`/config/webdav/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
  return res.json();
}

export async function deleteJob(
  id: string
): Promise<ApiResponse<{ id: string }>> {
  const res = await fetch(`/jobs/${id}`, { method: "DELETE" });
  return res.json();
}

export async function clearHistory(): Promise<
  ApiResponse<{ cleared: number }>
> {
  const res = await fetch("/jobs", { method: "DELETE" });
  return res.json();
}
