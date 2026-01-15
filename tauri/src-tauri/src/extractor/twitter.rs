use super::types::*;
use regex::Regex;
use reqwest::header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE, COOKIE, USER_AGENT};
use serde::Deserialize;
use std::sync::LazyLock;
use url::Url;

const BEARER_TOKEN: &str = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs=1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA";
const GUEST_TOKEN_URL: &str = "https://api.x.com/1.1/guest/activate.json";
const GRAPHQL_URL: &str = "https://x.com/i/api/graphql/2ICDjqPd81tulZcYrtpTuQ/TweetResultByRestId";
const SYNDICATION_URL: &str = "https://cdn.syndication.twimg.com/tweet-result";

static URL_REGEX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?:twitter\.com|x\.com)/(?:[^/]+)/status/(\d+)").unwrap());

static RESOLUTION_REGEX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"/(\d+)x(\d+)/").unwrap());

pub struct TwitterExtractor {
    client: reqwest::Client,
    auth_token: Option<String>,
    csrf_token: Option<String>,
}

impl TwitterExtractor {
    pub fn new(auth_token: Option<String>) -> Self {
        Self {
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(30))
                .build()
                .unwrap(),
            auth_token,
            csrf_token: None,
        }
    }

    /// Check if URL is a Twitter/X status URL
    pub fn matches(url: &Url) -> bool {
        let host = url.host_str().unwrap_or("");
        if !["twitter.com", "x.com", "mobile.twitter.com", "mobile.x.com"].contains(&host) {
            return false;
        }
        URL_REGEX.is_match(url.as_str())
    }

    /// Extract media info from Twitter URL
    pub async fn extract(url_str: &str, auth_token: Option<String>) -> Result<MediaInfo, ExtractError> {
        let mut extractor = Self::new(auth_token);
        extractor.do_extract(url_str).await
    }

    async fn do_extract(&mut self, url_str: &str) -> Result<MediaInfo, ExtractError> {
        // Extract tweet ID
        let caps = URL_REGEX
            .captures(url_str)
            .ok_or_else(|| ExtractError::Parse("Could not extract tweet ID from URL".into()))?;
        let tweet_id = caps.get(1).unwrap().as_str();

        // If authenticated, use GraphQL with auth
        if self.auth_token.is_some() {
            return self.fetch_from_graphql_auth(tweet_id).await;
        }

        // Try syndication API first (simpler, works for public tweets)
        if let Ok(info) = self.fetch_from_syndication(tweet_id).await {
            return Ok(info);
        }

        // Fallback to GraphQL with guest token
        let guest_token = self.fetch_guest_token().await?;
        self.fetch_from_graphql(tweet_id, &guest_token).await
    }

    async fn fetch_guest_token(&self) -> Result<String, ExtractError> {
        let resp = self
            .client
            .post(GUEST_TOKEN_URL)
            .header(AUTHORIZATION, format!("Bearer {}", BEARER_TOKEN))
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(ExtractError::Parse(format!(
                "Guest token request failed: {}",
                resp.status()
            )));
        }

        #[derive(Deserialize)]
        struct GuestTokenResponse {
            guest_token: String,
        }

        let data: GuestTokenResponse = resp.json().await?;
        Ok(data.guest_token)
    }

    async fn fetch_from_syndication(&self, tweet_id: &str) -> Result<MediaInfo, ExtractError> {
        let url = format!("{}?id={}&token=x", SYNDICATION_URL, tweet_id);

        let resp = self
            .client
            .get(&url)
            .header(USER_AGENT, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
            .header("Accept", "application/json")
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(ExtractError::Parse(format!(
                "Syndication request failed: {}",
                resp.status()
            )));
        }

        let data: SyndicationResponse = resp.json().await?;
        self.parse_syndication_response(&data, tweet_id)
    }

    async fn fetch_from_graphql(
        &self,
        tweet_id: &str,
        guest_token: &str,
    ) -> Result<MediaInfo, ExtractError> {
        let (variables, features) = build_graphql_params(tweet_id);
        let url = format!(
            "{}?variables={}&features={}",
            GRAPHQL_URL,
            urlencoding::encode(&variables),
            urlencoding::encode(&features)
        );

        let resp = self
            .client
            .get(&url)
            .header(AUTHORIZATION, format!("Bearer {}", BEARER_TOKEN))
            .header("x-guest-token", guest_token)
            .header(CONTENT_TYPE, "application/json")
            .header(USER_AGENT, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(ExtractError::Parse(format!(
                "GraphQL request failed: {}",
                resp.status()
            )));
        }

        let body = resp.text().await?;
        self.parse_graphql_response(&body, tweet_id)
    }

    async fn fetch_csrf_token(&mut self) -> Result<(), ExtractError> {
        let auth_token = self.auth_token.as_ref().ok_or(ExtractError::AuthRequired)?;

        let resp = self
            .client
            .get("https://x.com")
            .header(USER_AGENT, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
            .header(COOKIE, format!("auth_token={}", auth_token))
            .send()
            .await?;

        // Extract ct0 cookie from response
        for cookie in resp.cookies() {
            if cookie.name() == "ct0" {
                self.csrf_token = Some(cookie.value().to_string());
                return Ok(());
            }
        }

        Err(ExtractError::Parse("Could not obtain CSRF token".into()))
    }

    async fn fetch_from_graphql_auth(&mut self, tweet_id: &str) -> Result<MediaInfo, ExtractError> {
        // Fetch CSRF token if needed
        if self.csrf_token.is_none() {
            self.fetch_csrf_token().await?;
        }

        let csrf_token = self.csrf_token.as_ref().unwrap();
        let auth_token = self.auth_token.as_ref().unwrap();

        let (variables, features) = build_graphql_params(tweet_id);
        let url = format!(
            "{}?variables={}&features={}",
            GRAPHQL_URL,
            urlencoding::encode(&variables),
            urlencoding::encode(&features)
        );

        let mut headers = HeaderMap::new();
        headers.insert(AUTHORIZATION, HeaderValue::from_str(&format!("Bearer {}", BEARER_TOKEN)).unwrap());
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        headers.insert(USER_AGENT, HeaderValue::from_static("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"));
        headers.insert("x-twitter-auth-type", HeaderValue::from_static("OAuth2Session"));
        headers.insert("x-twitter-client-language", HeaderValue::from_static("en"));
        headers.insert("x-twitter-active-user", HeaderValue::from_static("yes"));
        headers.insert("x-csrf-token", HeaderValue::from_str(csrf_token).unwrap());
        headers.insert(
            COOKIE,
            HeaderValue::from_str(&format!("auth_token={}; ct0={}", auth_token, csrf_token)).unwrap(),
        );

        let resp = self.client.get(&url).headers(headers).send().await?;

        if !resp.status().is_success() {
            return Err(ExtractError::Parse(format!(
                "GraphQL auth request failed: {}",
                resp.status()
            )));
        }

        let body = resp.text().await?;
        self.parse_graphql_response(&body, tweet_id)
    }

    fn parse_syndication_response(
        &self,
        data: &SyndicationResponse,
        tweet_id: &str,
    ) -> Result<MediaInfo, ExtractError> {
        let title = truncate_text(&data.text, 100);
        let uploader = data.user.as_ref().map(|u| u.screen_name.clone());

        let mut formats = Vec::new();

        // Process media_details
        if let Some(media_details) = &data.media_details {
            for media in media_details {
                match media.r#type.as_str() {
                    "video" | "animated_gif" => {
                        if let Some(video_info) = &media.video_info {
                            for variant in &video_info.variants {
                                if variant.content_type != "video/mp4" {
                                    continue;
                                }

                                let (width, height) = extract_resolution(&variant.url);
                                let quality = if height > 0 {
                                    Some(format!("{}p", height))
                                } else {
                                    estimate_quality(variant.bitrate)
                                };

                                formats.push(Format {
                                    id: format!("mp4_{}", variant.bitrate.unwrap_or(0)),
                                    url: variant.url.clone(),
                                    ext: "mp4".into(),
                                    quality,
                                    width: if width > 0 { Some(width) } else { None },
                                    height: if height > 0 { Some(height) } else { None },
                                    filesize: None,
                                    audio_url: None,
                                });
                            }
                        }
                    }
                    "photo" => {
                        let image_url = get_high_quality_image_url(&media.media_url_https);
                        let ext = get_image_extension(&media.media_url_https);
                        formats.push(Format {
                            id: "photo".into(),
                            url: image_url,
                            ext,
                            quality: Some("orig".into()),
                            width: media.original_info_width,
                            height: media.original_info_height,
                            filesize: None,
                            audio_url: None,
                        });
                    }
                    _ => {}
                }
            }
        }

        // Also check video field for single video tweets
        if formats.is_empty() {
            if let Some(video) = &data.video {
                for variant in &video.variants {
                    if variant.r#type != "video/mp4" {
                        continue;
                    }
                    if let Some(src) = &variant.src {
                        let (width, height) = extract_resolution(src);
                        formats.push(Format {
                            id: "mp4_direct".into(),
                            url: src.clone(),
                            ext: "mp4".into(),
                            quality: if height > 0 { Some(format!("{}p", height)) } else { None },
                            width: if width > 0 { Some(width) } else { None },
                            height: if height > 0 { Some(height) } else { None },
                            filesize: None,
                            audio_url: None,
                        });
                    }
                }
            }
        }

        if formats.is_empty() {
            return Err(ExtractError::NotAvailable);
        }

        // Sort by bitrate/quality (highest first)
        formats.sort_by(|a, b| {
            let height_a = a.height.unwrap_or(0);
            let height_b = b.height.unwrap_or(0);
            height_b.cmp(&height_a)
        });

        // Determine media type
        let media_type = if formats.iter().any(|f| f.ext == "mp4") {
            MediaType::Video
        } else {
            MediaType::Image
        };

        Ok(MediaInfo {
            id: tweet_id.to_string(),
            title,
            uploader,
            thumbnail: None,
            duration: None,
            media_type,
            formats,
        })
    }

    fn parse_graphql_response(
        &self,
        body: &str,
        tweet_id: &str,
    ) -> Result<MediaInfo, ExtractError> {
        let resp: GraphQLResponse =
            serde_json::from_str(body).map_err(|e| ExtractError::Parse(e.to_string()))?;

        let result = resp
            .data
            .tweet_result
            .result
            .ok_or(ExtractError::NotAvailable)?;

        // Handle different result types
        match result.typename.as_str() {
            "TweetTombstone" => return Err(ExtractError::NotAvailable),
            "TweetUnavailable" => {
                return match result.reason.as_deref() {
                    Some("NsfwLoggedOut") => Err(ExtractError::AuthRequired),
                    Some("Protected") => Err(ExtractError::AuthRequired),
                    _ => Err(ExtractError::NotAvailable),
                };
            }
            _ => {}
        }

        // Get legacy data (may be nested in tweet field)
        let legacy = result
            .legacy
            .as_ref()
            .or_else(|| result.tweet.as_ref().and_then(|t| t.legacy.as_ref()))
            .ok_or_else(|| ExtractError::Parse("Could not find tweet data".into()))?;

        let title = truncate_text(&legacy.full_text, 100);
        let uploader = result
            .core
            .as_ref()
            .and_then(|c| c.user_results.result.as_ref())
            .map(|u| u.legacy.screen_name.clone());

        let extended_entities = legacy
            .extended_entities
            .as_ref()
            .ok_or(ExtractError::NotAvailable)?;

        let mut formats = Vec::new();
        let mut duration: Option<u64> = None;

        for media in &extended_entities.media {
            match media.r#type.as_str() {
                "video" | "animated_gif" => {
                    if let Some(video_info) = &media.video_info {
                        if duration.is_none() && video_info.duration_millis > 0 {
                            duration = Some((video_info.duration_millis / 1000) as u64);
                        }

                        for variant in &video_info.variants {
                            if variant.content_type != "video/mp4" {
                                continue;
                            }

                            let (width, height) = extract_resolution(&variant.url);
                            let quality = if height > 0 {
                                Some(format!("{}p", height))
                            } else {
                                estimate_quality(variant.bitrate)
                            };

                            formats.push(Format {
                                id: format!("mp4_{}", variant.bitrate.unwrap_or(0)),
                                url: variant.url.clone(),
                                ext: "mp4".into(),
                                quality,
                                width: if width > 0 { Some(width) } else { None },
                                height: if height > 0 { Some(height) } else { None },
                                filesize: None,
                                audio_url: None,
                            });
                        }
                    }
                }
                "photo" => {
                    let image_url = get_high_quality_image_url(&media.media_url_https);
                    let ext = get_image_extension(&media.media_url_https);
                    formats.push(Format {
                        id: "photo".into(),
                        url: image_url,
                        ext,
                        quality: Some("orig".into()),
                        width: media.original_info.as_ref().map(|i| i.width),
                        height: media.original_info.as_ref().map(|i| i.height),
                        filesize: None,
                        audio_url: None,
                    });
                }
                _ => {}
            }
        }

        if formats.is_empty() {
            return Err(ExtractError::NotAvailable);
        }

        // Sort by quality (highest first)
        formats.sort_by(|a, b| {
            let height_a = a.height.unwrap_or(0);
            let height_b = b.height.unwrap_or(0);
            height_b.cmp(&height_a)
        });

        let media_type = if formats.iter().any(|f| f.ext == "mp4") {
            MediaType::Video
        } else {
            MediaType::Image
        };

        Ok(MediaInfo {
            id: tweet_id.to_string(),
            title,
            uploader,
            thumbnail: None,
            duration,
            media_type,
            formats,
        })
    }
}

// ============ Helper functions ============

fn build_graphql_params(tweet_id: &str) -> (String, String) {
    let variables = serde_json::json!({
        "tweetId": tweet_id,
        "withCommunity": false,
        "includePromotedContent": false,
        "withVoice": false
    });

    let features = serde_json::json!({
        "creator_subscriptions_tweet_preview_api_enabled": true,
        "tweetypie_unmention_optimization_enabled": true,
        "responsive_web_edit_tweet_api_enabled": true,
        "graphql_is_translatable_rweb_tweet_is_translatable_enabled": true,
        "view_counts_everywhere_api_enabled": true,
        "longform_notetweets_consumption_enabled": true,
        "responsive_web_twitter_article_tweet_consumption_enabled": false,
        "tweet_awards_web_tipping_enabled": false,
        "freedom_of_speech_not_reach_fetch_enabled": true,
        "standardized_nudges_misinfo": true,
        "tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
        "longform_notetweets_rich_text_read_enabled": true,
        "longform_notetweets_inline_media_enabled": true,
        "responsive_web_graphql_exclude_directive_enabled": true,
        "verified_phone_label_enabled": false,
        "responsive_web_media_download_video_enabled": false,
        "responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
        "responsive_web_graphql_timeline_navigation_enabled": true,
        "responsive_web_enhance_cards_enabled": false
    });

    (variables.to_string(), features.to_string())
}

fn truncate_text(s: &str, max_len: usize) -> String {
    let s = s.replace('\n', " ");
    let chars: Vec<char> = s.chars().collect();
    if chars.len() <= max_len {
        s
    } else {
        format!("{}...", chars[..max_len - 3].iter().collect::<String>())
    }
}

fn extract_resolution(url: &str) -> (u32, u32) {
    if let Some(caps) = RESOLUTION_REGEX.captures(url) {
        let width = caps.get(1).and_then(|m| m.as_str().parse().ok()).unwrap_or(0);
        let height = caps.get(2).and_then(|m| m.as_str().parse().ok()).unwrap_or(0);
        (width, height)
    } else {
        (0, 0)
    }
}

fn estimate_quality(bitrate: Option<i64>) -> Option<String> {
    bitrate.map(|b| {
        if b >= 2_000_000 {
            "1080p".into()
        } else if b >= 1_000_000 {
            "720p".into()
        } else if b >= 500_000 {
            "480p".into()
        } else {
            "360p".into()
        }
    })
}

fn get_high_quality_image_url(image_url: &str) -> String {
    let base_url = image_url.split('?').next().unwrap_or(image_url);
    let format = if base_url.contains(".png") {
        "png"
    } else if base_url.contains(".webp") {
        "webp"
    } else {
        "jpg"
    };
    format!("{}?format={}&name=orig", base_url, format)
}

fn get_image_extension(image_url: &str) -> String {
    let base_url = image_url.split('?').next().unwrap_or(image_url);
    if base_url.ends_with(".png") {
        "png".into()
    } else if base_url.ends_with(".webp") {
        "webp".into()
    } else if base_url.ends_with(".gif") {
        "gif".into()
    } else {
        "jpg".into()
    }
}

// ============ Response structs ============

#[derive(Debug, Deserialize)]
struct SyndicationResponse {
    #[serde(default)]
    text: String,
    user: Option<SyndicationUser>,
    #[serde(rename = "mediaDetails")]
    media_details: Option<Vec<SyndicationMedia>>,
    video: Option<SyndicationVideo>,
}

#[derive(Debug, Deserialize)]
struct SyndicationUser {
    screen_name: String,
}

#[derive(Debug, Deserialize)]
struct SyndicationMedia {
    r#type: String,
    #[serde(default)]
    media_url_https: String,
    #[serde(rename = "original_info_width")]
    original_info_width: Option<u32>,
    #[serde(rename = "original_info_height")]
    original_info_height: Option<u32>,
    video_info: Option<SyndicationVideoInfo>,
}

#[derive(Debug, Deserialize)]
struct SyndicationVideoInfo {
    variants: Vec<SyndicationVariant>,
}

#[derive(Debug, Deserialize)]
struct SyndicationVariant {
    #[serde(default)]
    bitrate: Option<i64>,
    content_type: String,
    url: String,
}

#[derive(Debug, Deserialize)]
struct SyndicationVideo {
    variants: Vec<SyndicationVideoVariant>,
}

#[derive(Debug, Deserialize)]
struct SyndicationVideoVariant {
    r#type: String,
    src: Option<String>,
}

// GraphQL response structs
#[derive(Debug, Deserialize)]
struct GraphQLResponse {
    data: GraphQLData,
}

#[derive(Debug, Deserialize)]
struct GraphQLData {
    #[serde(rename = "tweetResult")]
    tweet_result: GraphQLTweetResult,
}

#[derive(Debug, Deserialize)]
struct GraphQLTweetResult {
    result: Option<GraphQLResult>,
}

#[derive(Debug, Deserialize)]
struct GraphQLResult {
    #[serde(rename = "__typename")]
    typename: String,
    legacy: Option<GraphQLLegacy>,
    core: Option<GraphQLCore>,
    tweet: Option<Box<GraphQLResult>>,
    reason: Option<String>,
}

#[derive(Debug, Deserialize)]
struct GraphQLCore {
    user_results: GraphQLUserResults,
}

#[derive(Debug, Deserialize)]
struct GraphQLUserResults {
    result: Option<GraphQLUser>,
}

#[derive(Debug, Deserialize)]
struct GraphQLUser {
    legacy: GraphQLUserLegacy,
}

#[derive(Debug, Deserialize)]
struct GraphQLUserLegacy {
    screen_name: String,
}

#[derive(Debug, Deserialize)]
struct GraphQLLegacy {
    full_text: String,
    extended_entities: Option<GraphQLExtendedEntities>,
}

#[derive(Debug, Deserialize)]
struct GraphQLExtendedEntities {
    media: Vec<GraphQLMedia>,
}

#[derive(Debug, Deserialize)]
struct GraphQLMedia {
    r#type: String,
    #[serde(default)]
    media_url_https: String,
    original_info: Option<GraphQLOriginalInfo>,
    video_info: Option<GraphQLVideoInfo>,
}

#[derive(Debug, Deserialize)]
struct GraphQLOriginalInfo {
    width: u32,
    height: u32,
}

#[derive(Debug, Deserialize)]
struct GraphQLVideoInfo {
    #[serde(default)]
    duration_millis: i64,
    variants: Vec<GraphQLVariant>,
}

#[derive(Debug, Deserialize)]
struct GraphQLVariant {
    #[serde(default)]
    bitrate: Option<i64>,
    content_type: String,
    url: String,
}
