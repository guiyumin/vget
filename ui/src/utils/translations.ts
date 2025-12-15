export interface UITranslations {
  download_to: string;
  edit: string;
  save: string;
  cancel: string;
  paste_url: string;
  download: string;
  bulk_download: string;
  coming_soon: string;
  adding: string;
  jobs: string;
  total: string;
  no_downloads: string;
  paste_hint: string;
  queued: string;
  downloading: string;
  completed: string;
  failed: string;
  cancelled: string;
  settings: string;
  language: string;
  format: string;
  quality: string;
  twitter_auth: string;
  server_port: string;
  max_concurrent: string;
  api_key: string;
  webdav_servers: string;
  add: string;
  delete: string;
  name: string;
  url: string;
  username: string;
  password: string;
  no_webdav_servers: string;
  clear_history: string;
  clear_all: string;
  // WebDAV
  webdav_browser: string;
  select_remote: string;
  empty_directory: string;
  download_selected: string;
  selected_files: string;
  loading: string;
  go_to_settings: string;
  // Torrent
  torrent: string;
  torrent_hint: string;
  torrent_submit: string;
  torrent_submitting: string;
  torrent_success: string;
  torrent_not_configured: string;
  torrent_settings: string;
  torrent_client: string;
  torrent_host: string;
  torrent_test: string;
  torrent_testing: string;
  torrent_test_success: string;
  torrent_enabled: string;
}

export interface ServerTranslations {
  no_config_warning: string;
  run_init_hint: string;
}

export const defaultTranslations: UITranslations = {
  download_to: "Download to:",
  edit: "Edit",
  save: "Save",
  cancel: "Cancel",
  paste_url: "Paste URL to download...",
  download: "Download",
  bulk_download: "Bulk Download",
  coming_soon: "Coming Soon",
  adding: "Adding...",
  jobs: "Jobs",
  total: "total",
  no_downloads: "No downloads yet",
  paste_hint: "Paste a URL above to get started",
  queued: "queued",
  downloading: "downloading",
  completed: "completed",
  failed: "failed",
  cancelled: "cancelled",
  settings: "Settings",
  language: "Language",
  format: "Format",
  quality: "Quality",
  twitter_auth: "Twitter Auth",
  server_port: "Server Port",
  max_concurrent: "Max Concurrent",
  api_key: "API Key",
  webdav_servers: "WebDAV Servers",
  add: "Add",
  delete: "Delete",
  name: "Name",
  url: "URL",
  username: "Username",
  password: "Password",
  no_webdav_servers: "No WebDAV servers configured",
  clear_history: "Clear",
  clear_all: "Clear All",
  // WebDAV
  webdav_browser: "WebDAV",
  select_remote: "Select Remote",
  empty_directory: "Empty directory",
  download_selected: "Download Selected",
  selected_files: "selected",
  loading: "Loading...",
  go_to_settings: "Go to Settings",
  // Torrent
  torrent: "BT/Magnet",
  torrent_hint: "Paste magnet link or torrent URL...",
  torrent_submit: "Send",
  torrent_submitting: "Sending...",
  torrent_success: "Torrent added successfully",
  torrent_not_configured: "Torrent client not configured. Go to Settings to set up.",
  torrent_settings: "Torrent Client",
  torrent_client: "Client Type",
  torrent_host: "Host",
  torrent_test: "Test Connection",
  torrent_testing: "Testing...",
  torrent_test_success: "Connection successful",
  torrent_enabled: "Enable Torrent",
};

export const defaultServerTranslations: ServerTranslations = {
  no_config_warning: "No config file found. Using default settings.",
  run_init_hint: "Run 'vget init' to configure vget interactively.",
};
