use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default = "default_language")]
    pub language: String,
    #[serde(default = "default_output_dir")]
    pub output_dir: String,
    #[serde(default = "default_quality")]
    pub quality: String,
    #[serde(default)]
    pub twitter_auth_token: Option<String>,
    #[serde(default)]
    pub bilibili_cookie: Option<String>,
}

fn default_language() -> String {
    "en".to_string()
}

fn default_output_dir() -> String {
    dirs::download_dir()
        .map(|p| p.join("vget").to_string_lossy().to_string())
        .unwrap_or_else(|| "~/Downloads/vget".to_string())
}

fn default_quality() -> String {
    "best".to_string()
}

impl Default for Config {
    fn default() -> Self {
        Self {
            language: default_language(),
            output_dir: default_output_dir(),
            quality: default_quality(),
            twitter_auth_token: None,
            bilibili_cookie: None,
        }
    }
}

fn config_dir() -> PathBuf {
    // Share config with CLI: ~/.config/vget/
    // Don't use dirs::config_dir() as it returns ~/Library/Application Support/ on macOS
    dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join(".config")
        .join("vget")
}

fn config_path() -> PathBuf {
    config_dir().join("config.yml")
}

pub fn get_config() -> Result<Config, Box<dyn std::error::Error>> {
    let path = config_path();
    if path.exists() {
        let contents = fs::read_to_string(&path)?;
        let config: Config = serde_yaml::from_str(&contents)?;
        Ok(config)
    } else {
        Ok(Config::default())
    }
}

pub fn save_config(config: &Config) -> Result<(), Box<dyn std::error::Error>> {
    let path = config_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let contents = serde_yaml::to_string(config)?;
    fs::write(path, contents)?;
    Ok(())
}
