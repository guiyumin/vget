export interface WebDAVServer {
  url: string;
  username: string;
  password: string;
}

export interface TwitterConfig {
  auth_token: string | null;
}

export interface ServerConfig {
  max_concurrent: number;
}

export interface BilibiliConfig {
  cookie: string | null;
}

export interface Kuaidi100Config {
  customer: string | null;
  key: string | null;
}

export interface ExpressConfig {
  kuaidi100: Kuaidi100Config | null;
}

export interface Config {
  language: string;
  output_dir: string;
  format: string;
  quality: string;
  theme: string;
  webdav_servers: Record<string, WebDAVServer>;
  twitter: TwitterConfig;
  server: ServerConfig;
  express: ExpressConfig;
  bilibili: BilibiliConfig;
}
