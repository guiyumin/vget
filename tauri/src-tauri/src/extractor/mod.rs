mod direct;
mod twitter;
mod types;

pub use types::*;

use crate::config::get_config;
use url::Url;

/// Extract media information from a URL
pub async fn extract_media(url_str: &str) -> Result<MediaInfo, ExtractError> {
    let url = Url::parse(url_str).map_err(|_| ExtractError::InvalidUrl(url_str.to_string()))?;

    // Check for Twitter/X URLs
    if twitter::TwitterExtractor::matches(&url) {
        // Load auth token from config
        let auth_token = get_config()
            .ok()
            .and_then(|c| c.twitter.auth_token);
        return twitter::TwitterExtractor::extract(url_str, auth_token).await;
    }

    // Check for direct file URLs
    if direct::DirectExtractor::matches(&url) {
        return direct::DirectExtractor::extract(url_str).await;
    }

    // TODO: Add more extractors here
    // - Bilibili
    // - Xiaoyuzhou
    // - etc.

    Err(ExtractError::NoExtractor(url_str.to_string()))
}
