package extractor

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/core/config"
)

// BV/AV conversion constants (from https://github.com/Colerar/abv)
const (
	xorCode  int64 = 23442827791579
	maskCode int64 = (1 << 51) - 1
	maxAID   int64 = maskCode + 1
	minAID   int64 = 1
	base     int64 = 58
	bvLen    int   = 9
)

var (
	alphabet    = []byte("FcwAPNKTMug3GV5Lj7EJnHpWsx4tb8haYeviqBz6rkCy12mUSDQX9RdoZf")
	revAlphabet = make(map[byte]int64)
)

func init() {
	for i, c := range alphabet {
		revAlphabet[c] = int64(i)
	}
}

// BVToAV converts a BV ID to AV number
func BVToAV(bvid string) (int64, error) {
	// Remove "BV1" prefix if present
	if strings.HasPrefix(strings.ToUpper(bvid), "BV1") {
		bvid = bvid[3:]
	} else if strings.HasPrefix(strings.ToUpper(bvid), "BV") {
		bvid = bvid[2:]
	}

	if len(bvid) != bvLen {
		return 0, fmt.Errorf("invalid BV ID length: expected %d, got %d", bvLen, len(bvid))
	}

	bv := []byte(bvid)
	// Swap positions
	bv[0], bv[6] = bv[6], bv[0]
	bv[1], bv[4] = bv[4], bv[1]

	var avid int64
	for _, b := range bv {
		val, ok := revAlphabet[b]
		if !ok {
			return 0, fmt.Errorf("invalid character in BV ID: %c", b)
		}
		avid = avid*base + val
	}

	return (avid & maskCode) ^ xorCode, nil
}

// AVToBV converts an AV number to BV ID
func AVToBV(avid int64) (string, error) {
	if avid < minAID {
		return "", fmt.Errorf("AV %d is smaller than %d", avid, minAID)
	}
	if avid >= maxAID {
		return "", fmt.Errorf("AV %d is bigger than %d", avid, maxAID)
	}

	bvid := make([]byte, bvLen)
	tmp := (maxAID | avid) ^ xorCode

	for i := bvLen - 1; tmp != 0; i-- {
		bvid[i] = alphabet[tmp%base]
		tmp /= base
	}

	// Swap positions
	bvid[0], bvid[6] = bvid[6], bvid[0]
	bvid[1], bvid[4] = bvid[4], bvid[1]

	return "BV1" + string(bvid), nil
}

// URL patterns for Bilibili
var (
	bilibiliVideoRegex   = regexp.MustCompile(`bilibili\.com/video/(BV[\w]+|av\d+)`)
	bilibiliShortRegex   = regexp.MustCompile(`b23\.tv/(BV[\w]+|av\d+|\w+)`)
	bilibiliBangumiRegex = regexp.MustCompile(`bilibili\.com/bangumi/play/(ep|ss)(\d+)`)
	bvRegex              = regexp.MustCompile(`(?i)^BV1[\w]{9}$`)
	avRegex              = regexp.MustCompile(`(?i)^av(\d+)$`)
)

// Quality definitions
var qualityMap = map[int]string{
	127: "8K",
	126: "Dolby Vision",
	125: "HDR",
	120: "4K",
	116: "1080P60",
	112: "1080P+",
	80:  "1080P",
	74:  "720P60",
	64:  "720P",
	32:  "480P",
	16:  "360P",
}

// BilibiliExtractor handles Bilibili video extraction
type BilibiliExtractor struct {
	client *http.Client
	cookie string
	wbi    string // WBI signing key
}

// Name returns the extractor name
func (b *BilibiliExtractor) Name() string {
	return "bilibili"
}

// Match checks if URL is a Bilibili video URL
func (b *BilibiliExtractor) Match(u *url.URL) bool {
	urlStr := u.String()
	return bilibiliVideoRegex.MatchString(urlStr) ||
		bilibiliShortRegex.MatchString(urlStr) ||
		bilibiliBangumiRegex.MatchString(urlStr)
}

// Extract retrieves video information from a Bilibili URL
func (b *BilibiliExtractor) Extract(urlStr string) (Media, error) {
	// Initialize HTTP client
	if b.client == nil {
		b.client = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects automatically
			},
		}
	}

	// Load cookie from config
	cfg := config.LoadOrDefault()
	if cfg.Bilibili.Cookie != "" {
		b.cookie = cfg.Bilibili.Cookie
	}

	// Resolve short URLs and extract video ID
	aid, bvid, err := b.resolveVideoID(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve video ID: %w", err)
	}

	// Get WBI keys for signing
	if err := b.fetchWBIKeys(); err != nil {
		// Non-fatal: continue without WBI
		fmt.Printf("Warning: failed to get WBI keys: %v\n", err)
	}

	// Fetch video info
	videoInfo, err := b.fetchVideoInfo(aid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video info: %w", err)
	}

	// Get first page CID
	if len(videoInfo.Pages) == 0 {
		return nil, fmt.Errorf("no video pages found")
	}
	cid := videoInfo.Pages[0].CID

	// Fetch play URL to get stream info
	streams, err := b.fetchPlayURL(aid, cid, bvid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch play URL: %w", err)
	}

	// Build formats from streams
	formats := b.buildFormats(streams)
	if len(formats) == 0 {
		return nil, fmt.Errorf("no playable streams found")
	}

	return &VideoMedia{
		ID:        bvid,
		Title:     videoInfo.Title,
		Uploader:  videoInfo.Owner.Name,
		Duration:  videoInfo.Duration,
		Thumbnail: videoInfo.Pic,
		Formats:   formats,
	}, nil
}

// resolveVideoID extracts aid and bvid from URL
func (b *BilibiliExtractor) resolveVideoID(urlStr string) (aid int64, bvid string, err error) {
	// Handle short URLs
	if strings.Contains(urlStr, "b23.tv") {
		urlStr, err = b.resolveShortURL(urlStr)
		if err != nil {
			return 0, "", err
		}
	}

	// Extract video ID from URL
	if matches := bilibiliVideoRegex.FindStringSubmatch(urlStr); len(matches) > 1 {
		id := matches[1]
		if bvRegex.MatchString(id) {
			bvid = id
			aid, err = BVToAV(bvid)
			if err != nil {
				return 0, "", err
			}
		} else if avMatches := avRegex.FindStringSubmatch(id); len(avMatches) > 1 {
			aid, err = strconv.ParseInt(avMatches[1], 10, 64)
			if err != nil {
				return 0, "", err
			}
			bvid, err = AVToBV(aid)
			if err != nil {
				return 0, "", err
			}
		}
	} else {
		return 0, "", fmt.Errorf("could not extract video ID from URL: %s", urlStr)
	}

	return aid, bvid, nil
}

// resolveShortURL follows redirects to get the full URL
func (b *BilibiliExtractor) resolveShortURL(shortURL string) (string, error) {
	req, err := http.NewRequest("HEAD", shortURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", b.userAgent())

	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return shortURL, nil
}

// fetchWBIKeys obtains WBI signing keys from nav API
func (b *BilibiliExtractor) fetchWBIKeys() error {
	api := "https://api.bilibili.com/x/web-interface/nav"

	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return err
	}
	b.setHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result struct {
		Data struct {
			WbiImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	// Extract keys from URLs
	imgKey := extractKeyFromURL(result.Data.WbiImg.ImgURL)
	subKey := extractKeyFromURL(result.Data.WbiImg.SubURL)

	// Generate mixin key
	b.wbi = getMixinKey(imgKey + subKey)

	return nil
}

// extractKeyFromURL extracts the key part from a wbi URL
func extractKeyFromURL(urlStr string) string {
	// URL like: https://i0.hdslb.com/bfs/wbi/xxx.png
	// Extract xxx (without extension)
	parts := strings.Split(urlStr, "/")
	if len(parts) == 0 {
		return ""
	}
	filename := parts[len(parts)-1]
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		return filename[:idx]
	}
	return filename
}

// getMixinKey generates the mixin key for WBI signing
func getMixinKey(orig string) string {
	mixinKeyEncTab := []int{
		46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
		27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	}

	var result strings.Builder
	for _, idx := range mixinKeyEncTab {
		if idx < len(orig) {
			result.WriteByte(orig[idx])
		}
	}
	return result.String()
}

// wbiSign signs the query parameters with WBI
func (b *BilibiliExtractor) wbiSign(params url.Values) string {
	if b.wbi == "" {
		return params.Encode()
	}

	// Add timestamp
	params.Set("wts", strconv.FormatInt(time.Now().Unix(), 10))

	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var query strings.Builder
	for i, k := range keys {
		if i > 0 {
			query.WriteByte('&')
		}
		query.WriteString(k)
		query.WriteByte('=')
		// Filter special characters
		v := filterWBIValue(params.Get(k))
		query.WriteString(url.QueryEscape(v))
	}

	// Calculate signature
	queryStr := query.String()
	hash := md5.Sum([]byte(queryStr + b.wbi))
	signature := hex.EncodeToString(hash[:])

	return queryStr + "&w_rid=" + signature
}

// filterWBIValue removes special characters from WBI values
func filterWBIValue(s string) string {
	// Remove !'()*
	var result strings.Builder
	for _, c := range s {
		if c != '!' && c != '\'' && c != '(' && c != ')' && c != '*' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// BilibiliVideoInfo represents video metadata
type BilibiliVideoInfo struct {
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Pic      string `json:"pic"`
	Duration int    `json:"duration"`
	Owner    struct {
		Mid  int64  `json:"mid"`
		Name string `json:"name"`
	} `json:"owner"`
	Pages []struct {
		CID      int64  `json:"cid"`
		Page     int    `json:"page"`
		Part     string `json:"part"`
		Duration int    `json:"duration"`
	} `json:"pages"`
}

// fetchVideoInfo retrieves video metadata
func (b *BilibiliExtractor) fetchVideoInfo(aid int64) (*BilibiliVideoInfo, error) {
	api := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?aid=%d", aid)

	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return nil, err
	}
	b.setHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    BilibiliVideoInfo `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s (code: %d)", result.Message, result.Code)
	}

	return &result.Data, nil
}

// BilibiliStreamInfo represents stream data
type BilibiliStreamInfo struct {
	Videos []struct {
		ID        int    `json:"id"`
		BaseURL   string `json:"baseUrl"`
		BackupURL []string `json:"backupUrl"`
		Bandwidth int64  `json:"bandwidth"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Codecs    string `json:"codecs"`
		CodecID   int    `json:"codecid"`
	} `json:"video"`
	Audios []struct {
		ID        int    `json:"id"`
		BaseURL   string `json:"baseUrl"`
		BackupURL []string `json:"backupUrl"`
		Bandwidth int64  `json:"bandwidth"`
		Codecs    string `json:"codecs"`
	} `json:"audio"`
}

// fetchPlayURL retrieves stream URLs
func (b *BilibiliExtractor) fetchPlayURL(aid, cid int64, bvid string) (*BilibiliStreamInfo, error) {
	params := url.Values{}
	params.Set("avid", strconv.FormatInt(aid, 10))
	params.Set("cid", strconv.FormatInt(cid, 10))
	params.Set("fnval", "4048") // DASH + HDR + Dolby + 8K + AV1
	params.Set("fnver", "0")
	params.Set("fourk", "1")
	params.Set("qn", "127") // Request highest quality

	// Sign with WBI if available
	query := b.wbiSign(params)

	api := "https://api.bilibili.com/x/player/wbi/playurl?" + query

	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return nil, err
	}
	b.setHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Dash *BilibiliStreamInfo `json:"dash"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s (code: %d)", result.Message, result.Code)
	}

	if result.Data.Dash == nil {
		return nil, fmt.Errorf("no DASH streams available")
	}

	return result.Data.Dash, nil
}

// buildFormats converts stream info to VideoFormat slice
func (b *BilibiliExtractor) buildFormats(streams *BilibiliStreamInfo) []VideoFormat {
	var formats []VideoFormat

	// Find best audio stream
	var bestAudioURL string
	var bestAudioBandwidth int64
	for _, audio := range streams.Audios {
		if audio.Bandwidth > bestAudioBandwidth {
			bestAudioBandwidth = audio.Bandwidth
			bestAudioURL = audio.BaseURL
		}
	}

	// Build video formats
	for _, video := range streams.Videos {
		quality := qualityMap[video.ID]
		if quality == "" {
			quality = fmt.Sprintf("%dp", video.Height)
		}

		codec := getCodecName(video.CodecID)

		format := VideoFormat{
			URL:      video.BaseURL,
			Quality:  fmt.Sprintf("%s [%s]", quality, codec),
			Ext:      "mp4",
			Width:    video.Width,
			Height:   video.Height,
			Bitrate:  int(video.Bandwidth / 1000), // Convert to kbps
			AudioURL: bestAudioURL,
			Headers: map[string]string{
				"Referer":    "https://www.bilibili.com/",
				"User-Agent": b.userAgent(),
			},
		}

		formats = append(formats, format)
	}

	// Sort by height (highest first), then by codec priority
	sort.Slice(formats, func(i, j int) bool {
		if formats[i].Height != formats[j].Height {
			return formats[i].Height > formats[j].Height
		}
		return formats[i].Bitrate > formats[j].Bitrate
	})

	return formats
}

// getCodecName converts codec ID to name
func getCodecName(codecID int) string {
	switch codecID {
	case 7:
		return "AVC"
	case 12:
		return "HEVC"
	case 13:
		return "AV1"
	default:
		return "Unknown"
	}
}

// setHeaders sets common request headers
func (b *BilibiliExtractor) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", b.userAgent())
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Accept", "application/json")

	if b.cookie != "" {
		req.Header.Set("Cookie", b.cookie)
	}
}

// userAgent returns a random-ish user agent
func (b *BilibiliExtractor) userAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
}

func init() {
	Register(&BilibiliExtractor{},
		"bilibili.com",
		"www.bilibili.com",
		"b23.tv",
	)
}
