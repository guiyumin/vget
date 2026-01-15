use super::types::*;
use regex::Regex;
use reqwest::header::{HeaderMap, HeaderValue, COOKIE, REFERER, USER_AGENT};
use serde::Deserialize;
use std::collections::{BTreeMap, HashMap};
use std::sync::LazyLock;
use url::Url;

use crate::config::get_config;

// BV/AV conversion constants (from https://github.com/Colerar/abv)
const XOR_CODE: i64 = 23442827791579;
const MASK_CODE: i64 = (1 << 51) - 1;
const MAX_AID: i64 = MASK_CODE + 1;
const BASE: i64 = 58;
const BV_LEN: usize = 9;

const ALPHABET: &[u8] = b"FcwAPNKTMug3GV5Lj7EJnHpWsx4tb8haYeviqBz6rkCy12mUSDQX9RdoZf";

// Mixin key encoding table for WBI signing
const MIXIN_KEY_ENC_TAB: [usize; 32] = [
    46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29,
    28, 14, 39, 12, 38, 41, 13,
];

// Quality definitions
fn quality_map(id: i32) -> Option<&'static str> {
    match id {
        127 => Some("8K"),
        126 => Some("Dolby Vision"),
        125 => Some("HDR"),
        120 => Some("4K"),
        116 => Some("1080P60"),
        112 => Some("1080P+"),
        80 => Some("1080P"),
        74 => Some("720P60"),
        64 => Some("720P"),
        32 => Some("480P"),
        16 => Some("360P"),
        _ => None,
    }
}

fn codec_name(codec_id: i32) -> &'static str {
    match codec_id {
        7 => "AVC",
        12 => "HEVC",
        13 => "AV1",
        _ => "Unknown",
    }
}

static VIDEO_REGEX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"bilibili\.com/video/(BV[\w]+|av\d+)").unwrap());

static SHORT_REGEX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"b23\.tv/(BV[\w]+|av\d+|\w+)").unwrap());

static BV_REGEX: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"(?i)^BV1[\w]{9}$").unwrap());

static AV_REGEX: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"(?i)^av(\d+)$").unwrap());

// Build reverse alphabet lookup
fn rev_alphabet() -> [i64; 256] {
    let mut rev = [0i64; 256];
    for (i, &c) in ALPHABET.iter().enumerate() {
        rev[c as usize] = i as i64;
    }
    rev
}

/// Convert BV ID to AV number
pub fn bv_to_av(bvid: &str) -> Result<i64, String> {
    let rev = rev_alphabet();

    // Remove "BV1" or "BV" prefix if present
    let bvid = if bvid.to_uppercase().starts_with("BV1") {
        &bvid[3..]
    } else if bvid.to_uppercase().starts_with("BV") {
        &bvid[2..]
    } else {
        bvid
    };

    if bvid.len() != BV_LEN {
        return Err(format!(
            "invalid BV ID length: expected {}, got {}",
            BV_LEN,
            bvid.len()
        ));
    }

    let mut bv: Vec<u8> = bvid.bytes().collect();

    // Swap positions
    bv.swap(0, 6);
    bv.swap(1, 4);

    let mut avid: i64 = 0;
    for b in bv.iter() {
        avid = avid * BASE + rev[*b as usize];
    }

    Ok((avid & MASK_CODE) ^ XOR_CODE)
}

/// Convert AV number to BV ID
pub fn av_to_bv(avid: i64) -> Result<String, String> {
    if avid < 1 {
        return Err(format!("AV {} is smaller than 1", avid));
    }
    if avid >= MAX_AID {
        return Err(format!("AV {} is bigger than {}", avid, MAX_AID));
    }

    let mut bvid = vec![0u8; BV_LEN];
    let mut tmp = (MAX_AID | avid) ^ XOR_CODE;

    for i in (0..BV_LEN).rev() {
        if tmp == 0 {
            break;
        }
        bvid[i] = ALPHABET[(tmp % BASE) as usize];
        tmp /= BASE;
    }

    // Swap positions
    bvid.swap(0, 6);
    bvid.swap(1, 4);

    Ok(format!("BV1{}", String::from_utf8(bvid).unwrap()))
}

pub struct BilibiliExtractor {
    client: reqwest::Client,
    cookie: Option<String>,
    wbi_key: Option<String>,
}

impl BilibiliExtractor {
    pub fn new() -> Self {
        let cookie = get_config()
            .ok()
            .and_then(|c| c.bilibili.cookie);

        Self {
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(30))
                .redirect(reqwest::redirect::Policy::none())
                .build()
                .unwrap(),
            cookie,
            wbi_key: None,
        }
    }

    /// Check if URL is a Bilibili video URL
    pub fn matches(url: &Url) -> bool {
        let url_str = url.as_str();
        VIDEO_REGEX.is_match(url_str) || SHORT_REGEX.is_match(url_str)
    }

    /// Extract media info from Bilibili URL
    pub async fn extract(url_str: &str) -> Result<MediaInfo, ExtractError> {
        let mut extractor = Self::new();
        extractor.do_extract(url_str).await
    }

    async fn do_extract(&mut self, url_str: &str) -> Result<MediaInfo, ExtractError> {
        // Resolve video ID
        let (aid, bvid) = self.resolve_video_id(url_str).await?;

        // Fetch WBI keys (non-fatal if fails)
        if let Err(e) = self.fetch_wbi_keys().await {
            eprintln!("Warning: failed to get WBI keys: {}", e);
        }

        // Fetch video info
        let video_info = self.fetch_video_info(aid).await?;

        // Get first page CID
        let cid = video_info
            .pages
            .first()
            .map(|p| p.cid)
            .ok_or_else(|| ExtractError::Parse("no video pages found".into()))?;

        // Fetch play URL to get stream info
        let streams = self.fetch_play_url(aid, cid).await?;

        // Build formats from streams
        let formats = self.build_formats(&streams);

        if formats.is_empty() {
            return Err(ExtractError::NotAvailable);
        }

        Ok(MediaInfo {
            id: bvid,
            title: video_info.title,
            uploader: Some(video_info.owner.name),
            thumbnail: Some(video_info.pic),
            duration: Some(video_info.duration as u64),
            media_type: MediaType::Video,
            formats,
        })
    }

    async fn resolve_video_id(&self, url_str: &str) -> Result<(i64, String), ExtractError> {
        // Handle short URLs
        let resolved_url = if url_str.contains("b23.tv") {
            self.resolve_short_url(url_str).await?
        } else {
            url_str.to_string()
        };

        // Extract video ID from URL
        if let Some(caps) = VIDEO_REGEX.captures(&resolved_url) {
            let id = caps.get(1).unwrap().as_str();

            if BV_REGEX.is_match(id) {
                let aid = bv_to_av(id).map_err(|e| ExtractError::Parse(e))?;
                return Ok((aid, id.to_string()));
            } else if let Some(av_caps) = AV_REGEX.captures(id) {
                let aid: i64 = av_caps
                    .get(1)
                    .unwrap()
                    .as_str()
                    .parse()
                    .map_err(|_| ExtractError::Parse("invalid AV number".into()))?;
                let bvid = av_to_bv(aid).map_err(|e| ExtractError::Parse(e))?;
                return Ok((aid, bvid));
            }
        }

        Err(ExtractError::Parse(format!(
            "could not extract video ID from URL: {}",
            url_str
        )))
    }

    async fn resolve_short_url(&self, short_url: &str) -> Result<String, ExtractError> {
        let resp = self
            .client
            .head(short_url)
            .header(USER_AGENT, user_agent())
            .send()
            .await?;

        if resp.status().is_redirection() {
            if let Some(location) = resp.headers().get("location") {
                return Ok(location.to_str().unwrap_or(short_url).to_string());
            }
        }

        Ok(short_url.to_string())
    }

    async fn fetch_wbi_keys(&mut self) -> Result<(), ExtractError> {
        let api = "https://api.bilibili.com/x/web-interface/nav";

        let resp = self
            .client
            .get(api)
            .headers(self.build_headers())
            .send()
            .await?;

        let data: NavResponse = resp.json().await?;

        // Extract keys from URLs
        let img_key = extract_key_from_url(&data.data.wbi_img.img_url);
        let sub_key = extract_key_from_url(&data.data.wbi_img.sub_url);

        // Generate mixin key
        self.wbi_key = Some(get_mixin_key(&format!("{}{}", img_key, sub_key)));

        Ok(())
    }

    async fn fetch_video_info(&self, aid: i64) -> Result<VideoInfo, ExtractError> {
        let api = format!(
            "https://api.bilibili.com/x/web-interface/view?aid={}",
            aid
        );

        let resp = self
            .client
            .get(&api)
            .headers(self.build_headers())
            .send()
            .await?;

        let data: VideoInfoResponse = resp.json().await?;

        if data.code != 0 {
            return Err(ExtractError::Parse(format!(
                "API error: {} (code: {})",
                data.message, data.code
            )));
        }

        Ok(data.data)
    }

    async fn fetch_play_url(&self, aid: i64, cid: i64) -> Result<DashInfo, ExtractError> {
        let mut params = BTreeMap::new();
        params.insert("avid", aid.to_string());
        params.insert("cid", cid.to_string());
        params.insert("fnval", "4048".to_string()); // DASH + HDR + Dolby + 8K + AV1
        params.insert("fnver", "0".to_string());
        params.insert("fourk", "1".to_string());
        params.insert("qn", "127".to_string()); // Request highest quality

        // Sign with WBI if available
        let query = self.wbi_sign(&mut params);

        let api = format!(
            "https://api.bilibili.com/x/player/wbi/playurl?{}",
            query
        );

        let resp = self
            .client
            .get(&api)
            .headers(self.build_headers())
            .send()
            .await?;

        let data: PlayUrlResponse = resp.json().await?;

        if data.code != 0 {
            return Err(ExtractError::Parse(format!(
                "API error: {} (code: {})",
                data.message, data.code
            )));
        }

        data.data
            .dash
            .ok_or_else(|| ExtractError::Parse("no DASH streams available".into()))
    }

    fn wbi_sign(&self, params: &mut BTreeMap<&str, String>) -> String {
        let wbi_key = match &self.wbi_key {
            Some(k) => k,
            None => return build_query(params),
        };

        // Add timestamp
        let wts = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        params.insert("wts", wts.to_string());

        // Build query string (sorted by key)
        let mut query_parts: Vec<String> = Vec::new();
        for (k, v) in params.iter() {
            let filtered_v = filter_wbi_value(v);
            query_parts.push(format!(
                "{}={}",
                k,
                urlencoding::encode(&filtered_v)
            ));
        }
        let query_str = query_parts.join("&");

        // Calculate signature
        let digest = md5::compute(format!("{}{}", query_str, wbi_key));
        let signature = format!("{:x}", digest);

        format!("{}&w_rid={}", query_str, signature)
    }

    fn build_formats(&self, streams: &DashInfo) -> Vec<Format> {
        let mut formats = Vec::new();

        // Build headers for Bilibili CDN
        let mut headers = HashMap::new();
        headers.insert("Referer".to_string(), "https://www.bilibili.com/".to_string());
        headers.insert("User-Agent".to_string(), user_agent().to_string());

        // Find best audio stream
        let best_audio_url = streams
            .audio
            .as_ref()
            .and_then(|audios| {
                audios
                    .iter()
                    .max_by_key(|a| a.bandwidth)
                    .map(|a| a.base_url.clone())
            });

        // Build video formats
        for video in &streams.video {
            let quality = quality_map(video.id)
                .map(|q| q.to_string())
                .unwrap_or_else(|| format!("{}p", video.height));

            let codec = codec_name(video.codecid);

            formats.push(Format {
                id: format!("{}_{}", video.id, video.codecid),
                url: video.base_url.clone(),
                ext: "mp4".to_string(),
                quality: Some(format!("{} [{}]", quality, codec)),
                width: Some(video.width as u32),
                height: Some(video.height as u32),
                filesize: None,
                audio_url: best_audio_url.clone(),
                headers: headers.clone(),
            });
        }

        // Sort by height (highest first), then by bitrate
        formats.sort_by(|a, b| {
            let height_a = a.height.unwrap_or(0);
            let height_b = b.height.unwrap_or(0);
            if height_a != height_b {
                height_b.cmp(&height_a)
            } else {
                // Compare by id (higher quality id = better)
                b.id.cmp(&a.id)
            }
        });

        formats
    }

    fn build_headers(&self) -> HeaderMap {
        let mut headers = HeaderMap::new();
        headers.insert(USER_AGENT, HeaderValue::from_static(user_agent()));
        headers.insert(
            REFERER,
            HeaderValue::from_static("https://www.bilibili.com/"),
        );
        headers.insert(
            "Accept",
            HeaderValue::from_static("application/json"),
        );

        if let Some(cookie) = &self.cookie {
            if let Ok(value) = HeaderValue::from_str(cookie) {
                headers.insert(COOKIE, value);
            }
        }

        headers
    }
}

// Helper functions

fn user_agent() -> &'static str {
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
}

fn extract_key_from_url(url: &str) -> String {
    // URL like: https://i0.hdslb.com/bfs/wbi/xxx.png
    // Extract xxx (without extension)
    url.rsplit('/')
        .next()
        .and_then(|filename| filename.rsplit_once('.').map(|(name, _)| name.to_string()))
        .unwrap_or_default()
}

fn get_mixin_key(orig: &str) -> String {
    let bytes: Vec<u8> = orig.bytes().collect();
    MIXIN_KEY_ENC_TAB
        .iter()
        .filter_map(|&idx| bytes.get(idx).copied())
        .map(|b| b as char)
        .collect()
}

fn filter_wbi_value(s: &str) -> String {
    // Remove !'()*
    s.chars()
        .filter(|&c| c != '!' && c != '\'' && c != '(' && c != ')' && c != '*')
        .collect()
}

fn build_query(params: &BTreeMap<&str, String>) -> String {
    params
        .iter()
        .map(|(k, v)| format!("{}={}", k, urlencoding::encode(v)))
        .collect::<Vec<_>>()
        .join("&")
}

// Response structs

#[derive(Debug, Deserialize)]
struct NavResponse {
    data: NavData,
}

#[derive(Debug, Deserialize)]
struct NavData {
    wbi_img: WbiImg,
}

#[derive(Debug, Deserialize)]
struct WbiImg {
    img_url: String,
    sub_url: String,
}

#[derive(Debug, Deserialize)]
struct VideoInfoResponse {
    code: i32,
    message: String,
    data: VideoInfo,
}

#[derive(Debug, Deserialize)]
struct VideoInfo {
    title: String,
    #[serde(default)]
    pic: String,
    duration: i64,
    owner: Owner,
    pages: Vec<Page>,
}

#[derive(Debug, Deserialize)]
struct Owner {
    name: String,
}

#[derive(Debug, Deserialize)]
struct Page {
    cid: i64,
}

#[derive(Debug, Deserialize)]
struct PlayUrlResponse {
    code: i32,
    message: String,
    data: PlayUrlData,
}

#[derive(Debug, Deserialize)]
struct PlayUrlData {
    dash: Option<DashInfo>,
}

#[derive(Debug, Deserialize)]
struct DashInfo {
    video: Vec<VideoStream>,
    audio: Option<Vec<AudioStream>>,
}

#[derive(Debug, Deserialize)]
struct VideoStream {
    id: i32,
    #[serde(rename = "baseUrl")]
    base_url: String,
    bandwidth: i64,
    width: i32,
    height: i32,
    codecid: i32,
}

#[derive(Debug, Deserialize)]
struct AudioStream {
    #[serde(rename = "baseUrl")]
    base_url: String,
    bandwidth: i64,
}
