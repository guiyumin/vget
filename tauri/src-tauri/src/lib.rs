mod auth;
mod config;
mod downloader;
mod extractor;

use auth::{
    bilibili_check_status, bilibili_logout, bilibili_qr_generate, bilibili_qr_poll,
    bilibili_save_cookie, xhs_check_status, xhs_logout, xhs_open_login_window,
};
use config::{get_config as load_config, save_config as store_config, Config};
use downloader::{DownloadJob, DownloadManager, DownloadStatus, SimpleDownloader};
use extractor::{extract_media as do_extract, MediaInfo};
use std::sync::Arc;
use tauri::{Emitter, State};

// ============ CONFIG COMMANDS ============

#[tauri::command]
async fn get_config() -> Result<Config, String> {
    tauri::async_runtime::spawn_blocking(|| {
        load_config().map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn save_config(config: Config) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        store_config(&config).map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

// ============ EXTRACTOR COMMANDS ============

#[tauri::command]
async fn extract_media(url: String) -> Result<MediaInfo, String> {
    do_extract(&url).await.map_err(|e| e.to_string())
}

// ============ FOLDER COMMANDS ============

#[tauri::command]
async fn open_output_folder(path: String) -> Result<(), String> {
    use std::path::Path;
    use std::process::Command;

    let path = Path::new(&path);

    // Create directory if it doesn't exist
    if !path.exists() {
        std::fs::create_dir_all(path).map_err(|e| format!("Failed to create directory: {}", e))?;
    }

    // Open the folder using platform-specific command
    #[cfg(target_os = "macos")]
    {
        Command::new("open")
            .arg(path)
            .spawn()
            .map_err(|e| format!("Failed to open folder: {}", e))?;
    }

    #[cfg(target_os = "windows")]
    {
        Command::new("explorer")
            .arg(path)
            .spawn()
            .map_err(|e| format!("Failed to open folder: {}", e))?;
    }

    #[cfg(target_os = "linux")]
    {
        Command::new("xdg-open")
            .arg(path)
            .spawn()
            .map_err(|e| format!("Failed to open folder: {}", e))?;
    }

    Ok(())
}

// ============ DOWNLOAD COMMANDS ============

#[tauri::command]
async fn start_download(
    url: String,
    output_path: String,
    _format_id: Option<String>,
    headers: Option<std::collections::HashMap<String, String>>,
    window: tauri::Window,
    download_manager: State<'_, Arc<DownloadManager>>,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();

    // Create job and get cancellation receiver
    let job = DownloadJob {
        id: job_id.clone(),
        url: url.clone(),
        output_path: output_path.clone(),
        status: DownloadStatus::Pending,
        progress: None,
        error: None,
    };

    let cancel_rx = download_manager.add_job(job).await;

    // Update status to downloading
    download_manager
        .update_job(&job_id, DownloadStatus::Downloading, None, None)
        .await;

    // Clone for async task
    let dm = download_manager.inner().clone();
    let jid = job_id.clone();

    // Spawn download task
    tauri::async_runtime::spawn(async move {
        let downloader = SimpleDownloader::new();

        match downloader
            .download(&jid, &url, &output_path, &window, cancel_rx, headers)
            .await
        {
            Ok(()) => {
                dm.update_job(&jid, DownloadStatus::Completed, None, None)
                    .await;
            }
            Err(e) => {
                if e.contains("cancelled") {
                    dm.update_job(&jid, DownloadStatus::Cancelled, None, Some(e.clone()))
                        .await;
                } else {
                    dm.update_job(&jid, DownloadStatus::Failed, None, Some(e.clone()))
                        .await;
                }
                let _ = window.emit(
                    "download-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

#[tauri::command]
async fn cancel_download(
    job_id: String,
    download_manager: State<'_, Arc<DownloadManager>>,
) -> Result<(), String> {
    download_manager.cancel_job(&job_id).await
}

#[tauri::command]
async fn get_download_status(
    job_id: String,
    download_manager: State<'_, Arc<DownloadManager>>,
) -> Result<Option<DownloadJob>, String> {
    Ok(download_manager.get_job(&job_id).await)
}

// ============ TAURI SETUP ============

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .manage(Arc::new(DownloadManager::new()))
        .invoke_handler(tauri::generate_handler![
            // Config
            get_config,
            save_config,
            // Extractor
            extract_media,
            // Folder
            open_output_folder,
            // Download
            start_download,
            cancel_download,
            get_download_status,
            // Auth - Bilibili
            bilibili_check_status,
            bilibili_qr_generate,
            bilibili_qr_poll,
            bilibili_save_cookie,
            bilibili_logout,
            // Auth - Xiaohongshu
            xhs_check_status,
            xhs_logout,
            xhs_open_login_window,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
