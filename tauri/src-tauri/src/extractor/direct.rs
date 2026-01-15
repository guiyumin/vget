use super::types::*;
use std::collections::HashMap;
use url::Url;

const VIDEO_EXTENSIONS: &[&str] = &["mp4", "mkv", "webm", "avi", "mov", "flv", "m3u8", "ts"];
const AUDIO_EXTENSIONS: &[&str] = &["mp3", "m4a", "aac", "flac", "wav", "ogg", "opus"];
const IMAGE_EXTENSIONS: &[&str] = &["jpg", "jpeg", "png", "gif", "webp", "bmp", "svg"];

pub struct DirectExtractor;

impl DirectExtractor {
    /// Check if URL is a direct file link
    pub fn matches(url: &Url) -> bool {
        let path = url.path().to_lowercase();
        VIDEO_EXTENSIONS.iter().any(|ext| path.ends_with(&format!(".{}", ext)))
            || AUDIO_EXTENSIONS.iter().any(|ext| path.ends_with(&format!(".{}", ext)))
            || IMAGE_EXTENSIONS.iter().any(|ext| path.ends_with(&format!(".{}", ext)))
    }

    /// Extract media info from direct URL
    pub async fn extract(url: &str) -> Result<MediaInfo, ExtractError> {
        let parsed = Url::parse(url).map_err(|_| ExtractError::InvalidUrl(url.to_string()))?;
        let path = parsed.path();

        // Get filename from path
        let filename = path
            .rsplit('/')
            .next()
            .unwrap_or("video")
            .to_string();

        // Determine extension and media type
        let ext = path
            .rsplit('.')
            .next()
            .unwrap_or("mp4")
            .to_lowercase();

        let media_type = if VIDEO_EXTENSIONS.contains(&ext.as_str()) {
            MediaType::Video
        } else if AUDIO_EXTENSIONS.contains(&ext.as_str()) {
            MediaType::Audio
        } else {
            MediaType::Image
        };

        // Try HEAD request to get file size
        let client = reqwest::Client::new();
        let filesize = client
            .head(url)
            .send()
            .await
            .ok()
            .and_then(|resp| {
                resp.headers()
                    .get("content-length")
                    .and_then(|v| v.to_str().ok())
                    .and_then(|v| v.parse().ok())
            });

        Ok(MediaInfo {
            id: filename.clone(),
            title: filename,
            uploader: parsed.host_str().map(|s| s.to_string()),
            thumbnail: None,
            duration: None,
            media_type,
            formats: vec![Format {
                id: "direct".to_string(),
                url: url.to_string(),
                ext,
                quality: None,
                width: None,
                height: None,
                filesize,
                audio_url: None,
                headers: HashMap::new(),
            }],
        })
    }
}
