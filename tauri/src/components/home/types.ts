export interface MediaInfo {
  id: string;
  title: string;
  uploader: string | null;
  thumbnail: string | null;
  duration: number | null;
  media_type: string;
  formats: {
    id: string;
    url: string;
    ext: string;
    quality: string | null;
    filesize: number | null;
    audio_url: string | null;
    headers?: Record<string, string>;
  }[];
}

export interface Config {
  output_dir: string;
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

export function formatSpeed(bytesPerSecond: number): string {
  return formatBytes(bytesPerSecond) + "/s";
}
