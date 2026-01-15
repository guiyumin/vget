use super::{DownloadProgress, DownloadStatus};
use futures::StreamExt;
use reqwest::Client;
use std::collections::HashMap;
use std::path::Path;
use std::time::Instant;
use tauri::{Emitter, Window};
use tokio::fs::File;
use tokio::io::AsyncWriteExt;
use tokio::sync::watch::Receiver;

pub struct SimpleDownloader {
    client: Client,
}

impl SimpleDownloader {
    pub fn new() -> Self {
        Self {
            client: Client::builder()
                .user_agent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
                .build()
                .unwrap_or_default(),
        }
    }

    pub async fn download(
        &self,
        job_id: &str,
        url: &str,
        output_path: &str,
        window: &Window,
        cancel_rx: Receiver<bool>,
        headers: Option<HashMap<String, String>>,
    ) -> Result<(), String> {
        // Ensure parent directory exists
        if let Some(parent) = Path::new(output_path).parent() {
            tokio::fs::create_dir_all(parent)
                .await
                .map_err(|e| format!("Failed to create directory: {}", e))?;
        }

        // Start download with optional headers
        let mut request = self.client.get(url);

        if let Some(hdrs) = headers {
            for (key, value) in hdrs {
                request = request.header(&key, &value);
            }
        }

        let response = request
            .send()
            .await
            .map_err(|e| format!("Failed to fetch: {}", e))?;

        if !response.status().is_success() {
            return Err(format!("HTTP error: {}", response.status()));
        }

        let total = response.content_length();
        let mut downloaded: u64 = 0;
        let mut last_emit = Instant::now();
        let mut last_downloaded: u64 = 0;

        // Create file
        let mut file = File::create(output_path)
            .await
            .map_err(|e| format!("Failed to create file: {}", e))?;

        // Stream download
        let mut stream = response.bytes_stream();

        while let Some(chunk_result) = stream.next().await {
            // Check for cancellation
            if *cancel_rx.borrow() {
                drop(file);
                let _ = tokio::fs::remove_file(output_path).await;
                return Err("Download cancelled".to_string());
            }

            let chunk = chunk_result.map_err(|e| format!("Stream error: {}", e))?;
            downloaded += chunk.len() as u64;

            file.write_all(&chunk)
                .await
                .map_err(|e| format!("Write error: {}", e))?;

            // Emit progress every 100ms
            if last_emit.elapsed().as_millis() >= 100 {
                let elapsed = last_emit.elapsed().as_secs_f64();
                let speed = if elapsed > 0.0 {
                    ((downloaded - last_downloaded) as f64 / elapsed) as u64
                } else {
                    0
                };

                let percent = total.map(|t| (downloaded as f64 / t as f64) * 100.0).unwrap_or(0.0);

                let progress = DownloadProgress {
                    job_id: job_id.to_string(),
                    downloaded,
                    total,
                    speed,
                    percent,
                };

                let _ = window.emit("download-progress", &progress);

                last_emit = Instant::now();
                last_downloaded = downloaded;
            }
        }

        file.flush()
            .await
            .map_err(|e| format!("Flush error: {}", e))?;

        // Emit completion
        let progress = DownloadProgress {
            job_id: job_id.to_string(),
            downloaded,
            total,
            speed: 0,
            percent: 100.0,
        };

        let _ = window.emit("download-progress", &progress);
        let _ = window.emit(
            "download-complete",
            serde_json::json!({
                "jobId": job_id,
                "status": DownloadStatus::Completed,
                "outputPath": output_path,
            }),
        );

        Ok(())
    }
}

impl Default for SimpleDownloader {
    fn default() -> Self {
        Self::new()
    }
}
