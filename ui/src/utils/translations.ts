export interface UITranslations {
  download_to: string;
  edit: string;
  save: string;
  cancel: string;
  paste_url: string;
  download: string;
  bulk_download: string;
  coming_soon: string;
  bulk_paste_urls: string;
  bulk_select_file: string;
  bulk_drag_drop: string;
  bulk_url_count: string;
  bulk_submit_all: string;
  bulk_submitting: string;
  bulk_clear: string;
  bulk_invalid_hint: string;
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
  // Toast
  download_queued: string;
  downloads_queued: string;
  // Podcast
  podcast: string;
  podcast_search: string;
  podcast_search_hint: string;
  podcast_searching: string;
  podcast_channels: string;
  podcast_episodes: string;
  podcast_no_results: string;
  podcast_episodes_count: string;
  podcast_back: string;
  podcast_download_started: string;
  // AI
  ai: string;
  ai_speech_to_text: string;
  ai_settings: string;
  ai_no_accounts: string;
  ai_encryption_note: string;
  ai_account_name: string;
  ai_provider: string;
  ai_api_key: string;
  ai_same_key_for_summary: string;
  ai_summary_api_key: string;
  ai_pin: string;
  ai_pin_hint: string;
  ai_advanced_options: string;
  ai_transcription_model: string;
  ai_transcription_url: string;
  ai_summary_model: string;
  ai_summary_url: string;
  ai_transcribe: string;
  ai_summarize: string;
  ai_processing: string;
  ai_processing_steps: string;
  ai_run: string;
  ai_select_model: string;
  ai_select_file_hint: string;
  ai_outputs: string;
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
  bulk_paste_urls: "Paste URLs here (one per line)...",
  bulk_select_file: "Select File",
  bulk_drag_drop: "or drag and drop a .txt file here",
  bulk_url_count: "URLs",
  bulk_submit_all: "Download All",
  bulk_submitting: "Submitting...",
  bulk_clear: "Clear",
  bulk_invalid_hint: "Empty lines and lines starting with # are ignored",
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
  // Toast
  download_queued: "Download started. Check progress on Download page.",
  downloads_queued: "downloads started. Check progress on Download page.",
  // Podcast
  podcast: "Podcast",
  podcast_search: "Search",
  podcast_search_hint: "Search podcasts or episodes...",
  podcast_searching: "Searching...",
  podcast_channels: "Podcasts",
  podcast_episodes: "Episodes",
  podcast_no_results: "No results found",
  podcast_episodes_count: "episodes",
  podcast_back: "Back",
  podcast_download_started: "Download started",
  // AI
  ai: "AI",
  ai_speech_to_text: "Speech to Text",
  ai_settings: "AI Settings",
  ai_no_accounts: "No AI accounts configured. Add one to use transcription and summarization.",
  ai_encryption_note: "API keys are encrypted with your PIN using AES-256-GCM. Leave PIN empty to store keys in plain text.",
  ai_account_name: "Account Name",
  ai_provider: "Provider",
  ai_api_key: "API Key",
  ai_same_key_for_summary: "Same Key for Summary",
  ai_summary_api_key: "Summary API Key",
  ai_pin: "4-Digit PIN",
  ai_pin_hint: "Optional. Used to encrypt your API keys",
  ai_advanced_options: "Advanced Options",
  ai_transcription_model: "Transcription Model",
  ai_transcription_url: "Transcription URL",
  ai_summary_model: "Summary Model",
  ai_summary_url: "Summary URL",
  ai_transcribe: "Transcribe",
  ai_summarize: "Summarize",
  ai_processing: "Processing...",
  ai_processing_steps: "Processing Steps",
  ai_run: "Run",
  ai_select_model: "Select AI and Model",
  ai_select_file_hint: "Select a file to start",
  ai_outputs: "Outputs",
};

export const defaultServerTranslations: ServerTranslations = {
  no_config_warning: "No config file found. Using default settings.",
  run_init_hint: "Run 'vget init' to configure vget interactively.",
};
