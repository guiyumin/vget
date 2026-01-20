mod auth;
mod config;
mod downloader;
mod extractor;
mod ffmpeg;
mod md2pdf;
mod pdf;

use auth::{
    bilibili_check_status, bilibili_logout, bilibili_qr_generate, bilibili_qr_poll,
    bilibili_save_cookie, xhs_check_status, xhs_logout, xhs_open_login_window,
};
use config::{get_config as load_config, save_config as store_config, Config};
use downloader::{DownloadJob, DownloadManager, DownloadStatus, SimpleDownloader};
use extractor::{extract_media as do_extract, MediaInfo};
use ffmpeg::MediaInfoResult;
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
    audio_url: Option<String>,
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

        let result = if let Some(audio) = audio_url {
            // DASH stream: download video + audio separately, then merge
            downloader
                .download_and_merge(
                    &jid,
                    &url,
                    &audio,
                    &output_path,
                    &window,
                    cancel_rx,
                    headers,
                )
                .await
        } else {
            // Simple download
            downloader
                .download(&jid, &url, &output_path, &window, cancel_rx, headers)
                .await
        };

        match result {
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

// ============ FFMPEG MEDIA TOOLS ============

#[tauri::command]
async fn ffmpeg_get_media_info(input_path: String) -> Result<MediaInfoResult, String> {
    ffmpeg::get_media_info(&input_path).await
}

#[tauri::command]
async fn ffmpeg_convert_video(
    input_path: String,
    output_path: String,
    window: tauri::Window,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();
    let jid = job_id.clone();

    tauri::async_runtime::spawn(async move {
        let result = tokio::task::spawn_blocking({
            let input = input_path.clone();
            let output = output_path.clone();
            let jid = jid.clone();
            let win = window.clone();

            move || {
                ffmpeg::convert_video_sync(&input, &output, move |progress| {
                    let _ = win.emit(
                        "ffmpeg-progress",
                        serde_json::json!({
                            "jobId": jid,
                            "progress": progress,
                        }),
                    );
                })
            }
        })
        .await;

        match result {
            Ok(Ok(())) => {
                let _ = window.emit(
                    "ffmpeg-complete",
                    serde_json::json!({
                        "jobId": jid,
                        "outputPath": output_path,
                    }),
                );
            }
            Ok(Err(e)) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
            Err(e) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e.to_string(),
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

#[tauri::command]
async fn ffmpeg_compress_video(
    input_path: String,
    output_path: String,
    quality: u8, // CRF value: 18 (high quality) to 28 (low quality/small size)
    window: tauri::Window,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();
    let jid = job_id.clone();

    tauri::async_runtime::spawn(async move {
        let result = tokio::task::spawn_blocking({
            let input = input_path.clone();
            let output = output_path.clone();
            let jid = jid.clone();
            let win = window.clone();

            move || {
                ffmpeg::compress_video_sync(&input, &output, quality, move |progress| {
                    let _ = win.emit(
                        "ffmpeg-progress",
                        serde_json::json!({
                            "jobId": jid,
                            "progress": progress,
                        }),
                    );
                })
            }
        })
        .await;

        match result {
            Ok(Ok(())) => {
                let _ = window.emit(
                    "ffmpeg-complete",
                    serde_json::json!({
                        "jobId": jid,
                        "outputPath": output_path,
                    }),
                );
            }
            Ok(Err(e)) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
            Err(e) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e.to_string(),
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

#[tauri::command]
async fn ffmpeg_trim_video(
    input_path: String,
    output_path: String,
    start_time: String,
    end_time: String,
    window: tauri::Window,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();
    let jid = job_id.clone();

    tauri::async_runtime::spawn(async move {
        let result = tokio::task::spawn_blocking({
            let input = input_path.clone();
            let output = output_path.clone();
            let start = start_time.clone();
            let end = end_time.clone();
            let jid = jid.clone();
            let win = window.clone();

            move || {
                ffmpeg::trim_video_sync(&input, &output, &start, &end, move |progress| {
                    let _ = win.emit(
                        "ffmpeg-progress",
                        serde_json::json!({
                            "jobId": jid,
                            "progress": progress,
                        }),
                    );
                })
            }
        })
        .await;

        match result {
            Ok(Ok(())) => {
                let _ = window.emit(
                    "ffmpeg-complete",
                    serde_json::json!({
                        "jobId": jid,
                        "outputPath": output_path,
                    }),
                );
            }
            Ok(Err(e)) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
            Err(e) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e.to_string(),
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

#[tauri::command]
async fn ffmpeg_extract_audio(
    input_path: String,
    output_path: String,
    format: String, // mp3, aac, flac, wav
    window: tauri::Window,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();
    let jid = job_id.clone();

    tauri::async_runtime::spawn(async move {
        let result = tokio::task::spawn_blocking({
            let input = input_path.clone();
            let output = output_path.clone();
            let fmt = format.clone();
            let jid = jid.clone();
            let win = window.clone();

            move || {
                ffmpeg::extract_audio_sync(&input, &output, &fmt, move |progress| {
                    let _ = win.emit(
                        "ffmpeg-progress",
                        serde_json::json!({
                            "jobId": jid,
                            "progress": progress,
                        }),
                    );
                })
            }
        })
        .await;

        match result {
            Ok(Ok(())) => {
                let _ = window.emit(
                    "ffmpeg-complete",
                    serde_json::json!({
                        "jobId": jid,
                        "outputPath": output_path,
                    }),
                );
            }
            Ok(Err(e)) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
            Err(e) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e.to_string(),
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

#[tauri::command]
async fn ffmpeg_extract_frames(
    input_path: String,
    output_dir: String,
    fps: f32,
    window: tauri::Window,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();
    let jid = job_id.clone();

    tauri::async_runtime::spawn(async move {
        let result = tokio::task::spawn_blocking({
            let input = input_path.clone();
            let output = output_dir.clone();
            let jid = jid.clone();
            let win = window.clone();

            move || {
                ffmpeg::extract_frames_sync(&input, &output, fps, move |progress| {
                    let _ = win.emit(
                        "ffmpeg-progress",
                        serde_json::json!({
                            "jobId": jid,
                            "progress": progress,
                        }),
                    );
                })
            }
        })
        .await;

        match result {
            Ok(Ok(frames)) => {
                let _ = window.emit(
                    "ffmpeg-complete",
                    serde_json::json!({
                        "jobId": jid,
                        "outputPath": output_dir,
                        "frames": frames,
                    }),
                );
            }
            Ok(Err(e)) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
            Err(e) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e.to_string(),
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

#[tauri::command]
async fn ffmpeg_convert_audio(
    input_path: String,
    output_path: String,
    format: String,
    bitrate: Option<String>,
    window: tauri::Window,
) -> Result<String, String> {
    let job_id = uuid::Uuid::new_v4().to_string();
    let jid = job_id.clone();

    tauri::async_runtime::spawn(async move {
        let result = tokio::task::spawn_blocking({
            let input = input_path.clone();
            let output = output_path.clone();
            let fmt = format.clone();
            let br = bitrate.clone();
            let jid = jid.clone();
            let win = window.clone();

            move || {
                ffmpeg::convert_audio_sync(&input, &output, &fmt, br.as_deref(), move |progress| {
                    let _ = win.emit(
                        "ffmpeg-progress",
                        serde_json::json!({
                            "jobId": jid,
                            "progress": progress,
                        }),
                    );
                })
            }
        })
        .await;

        match result {
            Ok(Ok(())) => {
                let _ = window.emit(
                    "ffmpeg-complete",
                    serde_json::json!({
                        "jobId": jid,
                        "outputPath": output_path,
                    }),
                );
            }
            Ok(Err(e)) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e,
                    }),
                );
            }
            Err(e) => {
                let _ = window.emit(
                    "ffmpeg-error",
                    serde_json::json!({
                        "jobId": jid,
                        "error": e.to_string(),
                    }),
                );
            }
        }
    });

    Ok(job_id)
}

// ============ PDF TOOLS ============

#[tauri::command]
async fn pdf_get_info(input_path: String) -> Result<pdf::PdfInfo, String> {
    tauri::async_runtime::spawn_blocking(move || pdf::get_pdf_info(&input_path))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn pdf_merge(input_paths: Vec<String>, output_path: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || pdf::merge_pdfs(&input_paths, &output_path))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn pdf_images_to_pdf(image_paths: Vec<String>, output_path: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || pdf::images_to_pdf(&image_paths, &output_path))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn pdf_delete_pages(
    input_path: String,
    output_path: String,
    pages: Vec<u32>,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        pdf::delete_pages(&input_path, &output_path, &pages)
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn pdf_remove_watermark(
    input_path: String,
    output_path: String,
) -> Result<pdf::WatermarkRemovalResult, String> {
    tauri::async_runtime::spawn_blocking(move || pdf::remove_watermark(&input_path, &output_path))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn pdf_print(input_path: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || pdf::print_pdf(&input_path))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn pdf_open_external(input_path: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || pdf::open_pdf_external(&input_path))
        .await
        .map_err(|e| e.to_string())?
}

// ============ FILE UTILITIES ============

#[tauri::command]
async fn read_text_file(path: String) -> Result<String, String> {
    tauri::async_runtime::spawn_blocking(move || {
        std::fs::read_to_string(&path).map_err(|e| format!("Failed to read file: {}", e))
    })
    .await
    .map_err(|e| e.to_string())?
}

// ============ MARKDOWN TO PDF ============

#[tauri::command]
async fn md_to_pdf(
    input_path: String,
    output_path: String,
    theme: String,
    page_size: String,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        md2pdf::convert_md_to_pdf(&input_path, &output_path, &theme, &page_size)
    })
    .await
    .map_err(|e| e.to_string())?
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
            // FFmpeg Media Tools
            ffmpeg_get_media_info,
            ffmpeg_convert_video,
            ffmpeg_compress_video,
            ffmpeg_trim_video,
            ffmpeg_extract_audio,
            ffmpeg_extract_frames,
            ffmpeg_convert_audio,
            // PDF Tools
            pdf_get_info,
            pdf_merge,
            pdf_images_to_pdf,
            pdf_delete_pages,
            pdf_remove_watermark,
            pdf_print,
            pdf_open_external,
            // File utilities
            read_text_file,
            // Markdown to PDF
            md_to_pdf,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
