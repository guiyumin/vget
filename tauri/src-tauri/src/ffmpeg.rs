use ffmpeg_sidecar::command::FfmpegCommand;
use ffmpeg_sidecar::event::{FfmpegEvent, LogLevel};
use serde::{Deserialize, Serialize};
use std::path::Path;
use std::process::Command;

/// Parse ffmpeg time string (HH:MM:SS.microseconds) to seconds
fn parse_time_to_secs(time_str: &str) -> Option<f32> {
    let parts: Vec<&str> = time_str.split(':').collect();
    if parts.len() == 3 {
        let hours: f32 = parts[0].parse().ok()?;
        let minutes: f32 = parts[1].parse().ok()?;
        let seconds: f32 = parts[2].parse().ok()?;
        Some(hours * 3600.0 + minutes * 60.0 + seconds)
    } else {
        // Try parsing as plain seconds
        time_str.parse().ok()
    }
}

/// Check if ffmpeg sidecar is available
pub fn ffmpeg_available() -> bool {
    FfmpegCommand::new().print_command().spawn().is_ok()
}

/// Merge separate video and audio files into a single output file.
/// Uses stream copy (-c copy) for fast merging without re-encoding.
pub async fn merge_video_audio(
    video_path: &str,
    audio_path: &str,
    output_path: &str,
    delete_originals: bool,
) -> Result<(), String> {
    // Validate input files exist
    if !Path::new(video_path).exists() {
        return Err(format!("Video file not found: {}", video_path));
    }
    if !Path::new(audio_path).exists() {
        return Err(format!("Audio file not found: {}", audio_path));
    }

    // Create output directory if needed
    if let Some(parent) = Path::new(output_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    // Run ffmpeg merge command
    // -i video_path -i audio_path -c copy -map 0:v -map 1:a output_path
    tokio::task::spawn_blocking({
        let video_path = video_path.to_string();
        let audio_path = audio_path.to_string();
        let output_path = output_path.to_string();

        move || {
            let mut cmd = FfmpegCommand::new();
            cmd.args(["-y"]) // Overwrite output
                .input(&video_path)
                .input(&audio_path)
                .args(["-map", "0:v"]) // Video from first input
                .args(["-map", "1:a"]) // Audio from second input
                .args(["-c", "copy"]) // Stream copy, no re-encoding
                .output(&output_path);

            let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;

            // Collect events and check for errors
            let mut error_msg: Option<String> = None;

            for event in child.iter().expect("Failed to iterate ffmpeg events") {
                match event {
                    FfmpegEvent::Log(LogLevel::Error, msg) => {
                        eprintln!("[ffmpeg error] {}", msg);
                        error_msg = Some(msg);
                    }
                    FfmpegEvent::Log(LogLevel::Warning, msg) => {
                        eprintln!("[ffmpeg warning] {}", msg);
                    }
                    FfmpegEvent::Progress(progress) => {
                        // Could emit progress events here if needed
                        let _ = progress;
                    }
                    FfmpegEvent::Done => {
                        break;
                    }
                    _ => {}
                }
            }

            // Check if output file was created
            if !Path::new(&output_path).exists() {
                return Err(error_msg.unwrap_or_else(|| "FFmpeg failed to create output file".to_string()));
            }

            Ok(())
        }
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))??;

    // Delete original files if requested
    if delete_originals {
        if let Err(e) = std::fs::remove_file(video_path) {
            eprintln!("[ffmpeg] Warning: could not remove video file: {}", e);
        }
        if let Err(e) = std::fs::remove_file(audio_path) {
            eprintln!("[ffmpeg] Warning: could not remove audio file: {}", e);
        }
    }

    Ok(())
}

/// Get the path to the bundled ffmpeg binary
pub fn get_ffmpeg_path() -> std::path::PathBuf {
    // ffmpeg-sidecar will automatically find the binary
    // For Tauri sidecar, it's in the app's resource directory
    ffmpeg_sidecar::paths::ffmpeg_path()
}

// ============ MEDIA INFO ============

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MediaInfoResult {
    pub filename: String,
    pub format_name: String,
    pub format_long_name: String,
    pub duration: Option<f64>,
    pub size: u64,
    pub bit_rate: Option<u64>,
    pub streams: Vec<StreamInfo>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamInfo {
    pub index: u32,
    pub codec_type: String,
    pub codec_name: String,
    pub codec_long_name: Option<String>,
    pub width: Option<u32>,
    pub height: Option<u32>,
    pub sample_rate: Option<String>,
    pub channels: Option<u32>,
    pub bit_rate: Option<String>,
    pub duration: Option<String>,
}

/// Get media file information using ffprobe
pub async fn get_media_info(input_path: &str) -> Result<MediaInfoResult, String> {
    if !Path::new(input_path).exists() {
        return Err(format!("File not found: {}", input_path));
    }

    let input = input_path.to_string();

    tokio::task::spawn_blocking(move || {
        let ffprobe_path = ffmpeg_sidecar::ffprobe::ffprobe_path();

        let output = Command::new(ffprobe_path)
            .args([
                "-v", "quiet",
                "-print_format", "json",
                "-show_format",
                "-show_streams",
                &input,
            ])
            .output()
            .map_err(|e| format!("Failed to run ffprobe: {}", e))?;

        if !output.status.success() {
            return Err("ffprobe failed to analyze file".to_string());
        }

        let json_str = String::from_utf8_lossy(&output.stdout);
        let probe: serde_json::Value = serde_json::from_str(&json_str)
            .map_err(|e| format!("Failed to parse ffprobe output: {}", e))?;

        let format = probe.get("format").ok_or("No format info found")?;
        let streams_arr = probe.get("streams").and_then(|s| s.as_array());

        let mut streams = Vec::new();
        if let Some(arr) = streams_arr {
            for s in arr {
                streams.push(StreamInfo {
                    index: s.get("index").and_then(|v| v.as_u64()).unwrap_or(0) as u32,
                    codec_type: s.get("codec_type").and_then(|v| v.as_str()).unwrap_or("unknown").to_string(),
                    codec_name: s.get("codec_name").and_then(|v| v.as_str()).unwrap_or("unknown").to_string(),
                    codec_long_name: s.get("codec_long_name").and_then(|v| v.as_str()).map(|s| s.to_string()),
                    width: s.get("width").and_then(|v| v.as_u64()).map(|v| v as u32),
                    height: s.get("height").and_then(|v| v.as_u64()).map(|v| v as u32),
                    sample_rate: s.get("sample_rate").and_then(|v| v.as_str()).map(|s| s.to_string()),
                    channels: s.get("channels").and_then(|v| v.as_u64()).map(|v| v as u32),
                    bit_rate: s.get("bit_rate").and_then(|v| v.as_str()).map(|s| s.to_string()),
                    duration: s.get("duration").and_then(|v| v.as_str()).map(|s| s.to_string()),
                });
            }
        }

        Ok(MediaInfoResult {
            filename: format.get("filename").and_then(|v| v.as_str()).unwrap_or("").to_string(),
            format_name: format.get("format_name").and_then(|v| v.as_str()).unwrap_or("").to_string(),
            format_long_name: format.get("format_long_name").and_then(|v| v.as_str()).unwrap_or("").to_string(),
            duration: format.get("duration").and_then(|v| v.as_str()).and_then(|s| s.parse().ok()),
            size: format.get("size").and_then(|v| v.as_str()).and_then(|s| s.parse().ok()).unwrap_or(0),
            bit_rate: format.get("bit_rate").and_then(|v| v.as_str()).and_then(|s| s.parse().ok()),
            streams,
        })
    })
    .await
    .map_err(|e| format!("Task join error: {}", e))?
}

// ============ VIDEO CONVERT ============

/// Convert video to a different format
pub fn convert_video_sync(
    input_path: &str,
    output_path: &str,
    progress_callback: impl Fn(f32) + Send + 'static,
) -> Result<(), String> {
    if !Path::new(input_path).exists() {
        return Err(format!("Input file not found: {}", input_path));
    }

    // Create output directory if needed
    if let Some(parent) = Path::new(output_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    let mut cmd = FfmpegCommand::new();
    cmd.args(["-y"])
        .input(input_path)
        .args(["-c:v", "libx264"])
        .args(["-c:a", "aac"])
        .args(["-preset", "medium"])
        .output(output_path);

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;
    let mut error_msg: Option<String> = None;

    for event in child.iter().expect("Failed to iterate ffmpeg events") {
        match event {
            FfmpegEvent::Progress(progress) => {
                // Progress time is a string like "00:00:05.123456"
                if let Some(secs) = parse_time_to_secs(&progress.time) {
                    progress_callback(secs);
                }
            }
            FfmpegEvent::Log(LogLevel::Error, msg) => {
                eprintln!("[ffmpeg error] {}", msg);
                error_msg = Some(msg);
            }
            FfmpegEvent::Done => break,
            _ => {}
        }
    }

    if !Path::new(output_path).exists() {
        return Err(error_msg.unwrap_or_else(|| "FFmpeg failed to create output file".to_string()));
    }

    Ok(())
}

// ============ VIDEO COMPRESS ============

/// Compress video to reduce file size
pub fn compress_video_sync(
    input_path: &str,
    output_path: &str,
    crf: u8, // 18-28 typically, higher = more compression
    progress_callback: impl Fn(f32) + Send + 'static,
) -> Result<(), String> {
    if !Path::new(input_path).exists() {
        return Err(format!("Input file not found: {}", input_path));
    }

    if let Some(parent) = Path::new(output_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    let crf_str = crf.to_string();
    let mut cmd = FfmpegCommand::new();
    cmd.args(["-y"])
        .input(input_path)
        .args(["-c:v", "libx264"])
        .args(["-crf", &crf_str])
        .args(["-preset", "medium"])
        .args(["-c:a", "aac"])
        .args(["-b:a", "128k"])
        .output(output_path);

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;
    let mut error_msg: Option<String> = None;

    for event in child.iter().expect("Failed to iterate ffmpeg events") {
        match event {
            FfmpegEvent::Progress(progress) => {
                if let Some(secs) = parse_time_to_secs(&progress.time) {
                    progress_callback(secs);
                }
            }
            FfmpegEvent::Log(LogLevel::Error, msg) => {
                eprintln!("[ffmpeg error] {}", msg);
                error_msg = Some(msg);
            }
            FfmpegEvent::Done => break,
            _ => {}
        }
    }

    if !Path::new(output_path).exists() {
        return Err(error_msg.unwrap_or_else(|| "FFmpeg failed to create output file".to_string()));
    }

    Ok(())
}

// ============ VIDEO TRIM ============

/// Trim video to specified time range
pub fn trim_video_sync(
    input_path: &str,
    output_path: &str,
    start_time: &str, // Format: "HH:MM:SS" or "SS"
    end_time: &str,
    progress_callback: impl Fn(f32) + Send + 'static,
) -> Result<(), String> {
    if !Path::new(input_path).exists() {
        return Err(format!("Input file not found: {}", input_path));
    }

    if let Some(parent) = Path::new(output_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    let mut cmd = FfmpegCommand::new();
    cmd.args(["-y"])
        .args(["-ss", start_time])
        .args(["-to", end_time])
        .input(input_path)
        .args(["-c", "copy"]) // Stream copy for fast trimming
        .output(output_path);

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;
    let mut error_msg: Option<String> = None;

    for event in child.iter().expect("Failed to iterate ffmpeg events") {
        match event {
            FfmpegEvent::Progress(progress) => {
                if let Some(secs) = parse_time_to_secs(&progress.time) {
                    progress_callback(secs);
                }
            }
            FfmpegEvent::Log(LogLevel::Error, msg) => {
                eprintln!("[ffmpeg error] {}", msg);
                error_msg = Some(msg);
            }
            FfmpegEvent::Done => break,
            _ => {}
        }
    }

    if !Path::new(output_path).exists() {
        return Err(error_msg.unwrap_or_else(|| "FFmpeg failed to create output file".to_string()));
    }

    Ok(())
}

// ============ EXTRACT AUDIO ============

/// Extract audio from video file
pub fn extract_audio_sync(
    input_path: &str,
    output_path: &str,
    format: &str, // mp3, aac, flac, wav
    progress_callback: impl Fn(f32) + Send + 'static,
) -> Result<(), String> {
    if !Path::new(input_path).exists() {
        return Err(format!("Input file not found: {}", input_path));
    }

    if let Some(parent) = Path::new(output_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    let mut cmd = FfmpegCommand::new();
    cmd.args(["-y"])
        .input(input_path)
        .args(["-vn"]); // No video

    // Set codec based on format
    match format {
        "mp3" => {
            cmd.args(["-c:a", "libmp3lame"]);
            cmd.args(["-b:a", "192k"]);
        }
        "aac" => {
            cmd.args(["-c:a", "aac"]);
            cmd.args(["-b:a", "192k"]);
        }
        "flac" => {
            cmd.args(["-c:a", "flac"]);
        }
        "wav" => {
            cmd.args(["-c:a", "pcm_s16le"]);
        }
        _ => {
            cmd.args(["-c:a", "copy"]); // Try to copy
        }
    }

    cmd.output(output_path);

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;
    let mut error_msg: Option<String> = None;

    for event in child.iter().expect("Failed to iterate ffmpeg events") {
        match event {
            FfmpegEvent::Progress(progress) => {
                if let Some(secs) = parse_time_to_secs(&progress.time) {
                    progress_callback(secs);
                }
            }
            FfmpegEvent::Log(LogLevel::Error, msg) => {
                eprintln!("[ffmpeg error] {}", msg);
                error_msg = Some(msg);
            }
            FfmpegEvent::Done => break,
            _ => {}
        }
    }

    if !Path::new(output_path).exists() {
        return Err(error_msg.unwrap_or_else(|| "FFmpeg failed to create output file".to_string()));
    }

    Ok(())
}

// ============ EXTRACT FRAMES ============

/// Extract frames from video as images
pub fn extract_frames_sync(
    input_path: &str,
    output_dir: &str,
    fps: f32, // frames per second to extract (e.g., 1.0 = 1 frame per second)
    progress_callback: impl Fn(f32) + Send + 'static,
) -> Result<Vec<String>, String> {
    if !Path::new(input_path).exists() {
        return Err(format!("Input file not found: {}", input_path));
    }

    std::fs::create_dir_all(output_dir)
        .map_err(|e| format!("Failed to create output directory: {}", e))?;

    let output_pattern = format!("{}/frame_%04d.jpg", output_dir);
    let fps_filter = format!("fps={}", fps);

    let mut cmd = FfmpegCommand::new();
    cmd.args(["-y"])
        .input(input_path)
        .args(["-vf", &fps_filter])
        .args(["-q:v", "2"]) // High quality JPEG
        .output(&output_pattern);

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;
    let mut error_msg: Option<String> = None;

    for event in child.iter().expect("Failed to iterate ffmpeg events") {
        match event {
            FfmpegEvent::Progress(progress) => {
                if let Some(secs) = parse_time_to_secs(&progress.time) {
                    progress_callback(secs);
                }
            }
            FfmpegEvent::Log(LogLevel::Error, msg) => {
                eprintln!("[ffmpeg error] {}", msg);
                error_msg = Some(msg);
            }
            FfmpegEvent::Done => break,
            _ => {}
        }
    }

    // Collect output files
    let mut frames = Vec::new();
    if let Ok(entries) = std::fs::read_dir(output_dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.extension().map(|e| e == "jpg").unwrap_or(false) {
                frames.push(path.to_string_lossy().to_string());
            }
        }
    }

    if frames.is_empty() && error_msg.is_some() {
        return Err(error_msg.unwrap());
    }

    frames.sort();
    Ok(frames)
}

// ============ AUDIO CONVERT ============

/// Convert audio to a different format
pub fn convert_audio_sync(
    input_path: &str,
    output_path: &str,
    format: &str, // mp3, aac, flac, wav
    bitrate: Option<&str>, // e.g., "192k"
    progress_callback: impl Fn(f32) + Send + 'static,
) -> Result<(), String> {
    if !Path::new(input_path).exists() {
        return Err(format!("Input file not found: {}", input_path));
    }

    if let Some(parent) = Path::new(output_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    let mut cmd = FfmpegCommand::new();
    cmd.args(["-y"])
        .input(input_path);

    // Set codec based on format
    match format {
        "mp3" => {
            cmd.args(["-c:a", "libmp3lame"]);
            if let Some(br) = bitrate {
                cmd.args(["-b:a", br]);
            } else {
                cmd.args(["-b:a", "192k"]);
            }
        }
        "aac" => {
            cmd.args(["-c:a", "aac"]);
            if let Some(br) = bitrate {
                cmd.args(["-b:a", br]);
            } else {
                cmd.args(["-b:a", "192k"]);
            }
        }
        "flac" => {
            cmd.args(["-c:a", "flac"]);
        }
        "wav" => {
            cmd.args(["-c:a", "pcm_s16le"]);
        }
        "ogg" => {
            cmd.args(["-c:a", "libvorbis"]);
            if let Some(br) = bitrate {
                cmd.args(["-b:a", br]);
            }
        }
        _ => {
            return Err(format!("Unsupported format: {}", format));
        }
    }

    cmd.output(output_path);

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn ffmpeg: {}", e))?;
    let mut error_msg: Option<String> = None;

    for event in child.iter().expect("Failed to iterate ffmpeg events") {
        match event {
            FfmpegEvent::Progress(progress) => {
                if let Some(secs) = parse_time_to_secs(&progress.time) {
                    progress_callback(secs);
                }
            }
            FfmpegEvent::Log(LogLevel::Error, msg) => {
                eprintln!("[ffmpeg error] {}", msg);
                error_msg = Some(msg);
            }
            FfmpegEvent::Done => break,
            _ => {}
        }
    }

    if !Path::new(output_path).exists() {
        return Err(error_msg.unwrap_or_else(|| "FFmpeg failed to create output file".to_string()));
    }

    Ok(())
}
