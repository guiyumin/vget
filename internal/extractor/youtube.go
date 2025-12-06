package extractor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/guiyumin/vget/internal/config"
)

// YouTubeExtractor handles YouTube video downloads using browser automation + Innertube API
type YouTubeExtractor struct{}

func (e *YouTubeExtractor) Name() string {
	return "youtube"
}

func (e *YouTubeExtractor) Match(u *url.URL) bool {
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be")
}

// YouTubeSession holds the extracted tokens needed for Innertube API
type YouTubeSession struct {
	POToken     string                 `json:"poToken"`
	VisitorData string                 `json:"visitorData"`
	Cookies     []*proto.NetworkCookie `json:"cookies,omitempty"`
}

// InnertubeResponse represents the /player API response
type InnertubeResponse struct {
	StreamingData struct {
		Formats []struct {
			ITag            int    `json:"itag"`
			URL             string `json:"url"`
			MimeType        string `json:"mimeType"`
			Bitrate         int    `json:"bitrate"`
			Width           int    `json:"width"`
			Height          int    `json:"height"`
			QualityLabel    string `json:"qualityLabel"`
			SignatureCipher string `json:"signatureCipher"`
		} `json:"formats"`
		AdaptiveFormats []struct {
			ITag            int    `json:"itag"`
			URL             string `json:"url"`
			MimeType        string `json:"mimeType"`
			Bitrate         int    `json:"bitrate"`
			Width           int    `json:"width"`
			Height          int    `json:"height"`
			QualityLabel    string `json:"qualityLabel"`
			SignatureCipher string `json:"signatureCipher"`
			ContentLength   string `json:"contentLength"`
		} `json:"adaptiveFormats"`
		HLSManifestURL string `json:"hlsManifestUrl"`
	} `json:"streamingData"`
	VideoDetails struct {
		VideoID          string `json:"videoId"`
		Title            string `json:"title"`
		LengthSeconds    string `json:"lengthSeconds"`
		Author           string `json:"author"`
		ShortDescription string `json:"shortDescription"`
		Thumbnail        struct {
			Thumbnails []struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
	} `json:"videoDetails"`
	PlayabilityStatus struct {
		Status          string `json:"status"`
		Reason          string `json:"reason"`
		PlayableInEmbed bool   `json:"playableInEmbed"`
	} `json:"playabilityStatus"`
}

func (e *YouTubeExtractor) Extract(rawURL string) (Media, error) {
	// Extract video ID from URL
	videoID := e.extractVideoID(rawURL)
	if videoID == "" {
		return nil, fmt.Errorf("could not extract video ID from URL: %s", rawURL)
	}

	fmt.Printf("Extracting YouTube video: %s\n", videoID)

	// Step 1: Get session tokens via browser
	session, err := e.extractSessionTokens(videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract session tokens: %w", err)
	}

	if session.POToken != "" {
		fmt.Printf("Got session - POToken: %d chars, VisitorData: %s...\n",
			len(session.POToken), truncate(session.VisitorData, 20))
	} else {
		fmt.Println("Warning: No POToken captured, trying without it...")
	}

	// Step 2: Call Innertube API with tokens
	response, err := e.callInnertubeAPI(videoID, session)
	if err != nil {
		return nil, fmt.Errorf("failed to call Innertube API: %w", err)
	}

	// Step 3: Parse response into Media
	return e.parseResponse(response)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (e *YouTubeExtractor) extractVideoID(rawURL string) string {
	// Handle various YouTube URL formats
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/|youtube\.com/shorts/)([a-zA-Z0-9_-]{11})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(rawURL)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// Try to find v= parameter
	u, err := url.Parse(rawURL)
	if err == nil {
		if v := u.Query().Get("v"); v != "" {
			return v
		}
	}

	return ""
}

func (e *YouTubeExtractor) extractSessionTokens(videoID string) (*YouTubeSession, error) {
	// Launch browser
	l := e.createLauncher(true) // headless
	defer l.Cleanup()

	fmt.Println("Launching browser for token extraction...")

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	defer page.MustClose()

	// Variables to capture tokens
	var session YouTubeSession
	var capturedPOToken bool
	var capturedVisitorData bool
	var mu sync.Mutex

	// Set up request interception - catch multiple request types
	router := page.HijackRequests()

	// Intercept /player requests for POToken
	router.MustAdd("*youtubei/v1/player*", func(ctx *rod.Hijack) {
		mu.Lock()
		defer mu.Unlock()

		body := ctx.Request.Body()
		if body != "" {
			var reqBody map[string]interface{}
			if err := json.Unmarshal([]byte(body), &reqBody); err == nil {
				// Extract poToken from serviceIntegrityDimensions
				if sid, ok := reqBody["serviceIntegrityDimensions"].(map[string]interface{}); ok {
					if pot, ok := sid["poToken"].(string); ok && pot != "" {
						session.POToken = pot
						capturedPOToken = true
						fmt.Printf("Captured POToken: %d chars\n", len(pot))
					}
				}

				// Extract visitorData from context.client
				if ctxMap, ok := reqBody["context"].(map[string]interface{}); ok {
					if client, ok := ctxMap["client"].(map[string]interface{}); ok {
						if vd, ok := client["visitorData"].(string); ok && vd != "" {
							session.VisitorData = vd
							capturedVisitorData = true
							fmt.Printf("Captured VisitorData: %s...\n", truncate(vd, 20))
						}
					}
				}
			}
		}

		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})

	// Also intercept /next requests which often contain tokens
	router.MustAdd("*youtubei/v1/next*", func(ctx *rod.Hijack) {
		mu.Lock()
		defer mu.Unlock()

		body := ctx.Request.Body()
		if body != "" {
			var reqBody map[string]interface{}
			if err := json.Unmarshal([]byte(body), &reqBody); err == nil {
				if sid, ok := reqBody["serviceIntegrityDimensions"].(map[string]interface{}); ok {
					if pot, ok := sid["poToken"].(string); ok && pot != "" && !capturedPOToken {
						session.POToken = pot
						capturedPOToken = true
						fmt.Printf("Captured POToken from /next: %d chars\n", len(pot))
					}
				}
			}
		}

		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})

	go router.Run()

	// Start with watch page (more reliable for POToken than embed)
	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	fmt.Printf("Navigating to: %s\n", watchURL)

	err = page.Navigate(watchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for page load
	page.MustWaitDOMStable()

	// Wait for token capture with timeout
	maxWait := 25 * time.Second
	start := time.Now()
	triedPlay := false
	triedEmbed := false

	for {
		mu.Lock()
		hasPOToken := capturedPOToken
		hasVisitorData := capturedVisitorData
		mu.Unlock()

		// Success: got both tokens
		if hasPOToken && hasVisitorData {
			fmt.Println("Token capture complete!")
			break
		}

		elapsed := time.Since(start)

		// After 8 seconds, try to trigger playback
		if elapsed > 8*time.Second && !triedPlay {
			triedPlay = true
			fmt.Println("Trying to trigger playback...")

			_ = page.MustEval(`() => {
				// Dismiss any dialogs
				const dismissBtns = document.querySelectorAll('button[aria-label*="Dismiss"], .ytp-ad-skip-button, paper-button[aria-label*="No thanks"]');
				dismissBtns.forEach(btn => btn.click());

				// Click play button
				const playBtn = document.querySelector('button.ytp-large-play-button, button.ytp-play-button');
				if (playBtn) playBtn.click();

				// Try to play video directly
				const video = document.querySelector('video');
				if (video) {
					video.muted = true;
					video.play().catch(() => {});
				}
			}`)

			time.Sleep(3 * time.Second)
		}

		// After 15 seconds, try embed page if we still don't have POToken
		if elapsed > 15*time.Second && !triedEmbed && !hasPOToken {
			triedEmbed = true
			fmt.Println("Trying embed page...")

			embedURL := fmt.Sprintf("https://www.youtube.com/embed/%s?autoplay=1", videoID)
			page.MustNavigate(embedURL)
			page.MustWaitDOMStable()

			time.Sleep(2 * time.Second)

			// Try to play in embed
			_ = page.MustEval(`() => {
				const playBtn = document.querySelector('button.ytp-large-play-button');
				if (playBtn) playBtn.click();
				const video = document.querySelector('video');
				if (video) {
					video.muted = true;
					video.play().catch(() => {});
				}
			}`)
		}

		// Timeout - try to get visitorData from page config as fallback
		if elapsed > maxWait {
			if !hasVisitorData {
				visitorData := page.MustEval(`() => {
					try {
						return ytcfg.get('VISITOR_DATA') ||
						       window.ytInitialPlayerResponse?.responseContext?.visitorData ||
						       '';
					} catch(e) {
						return '';
					}
				}`).String()

				if visitorData != "" {
					session.VisitorData = visitorData
					capturedVisitorData = true
					fmt.Printf("Got VisitorData from page config: %s...\n", truncate(visitorData, 20))
				}
			}
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Get cookies
	cookies, err := browser.GetCookies()
	if err == nil {
		session.Cookies = cookies
	}

	// Save session for reuse
	e.saveSession(&session)

	if session.VisitorData == "" {
		return nil, fmt.Errorf("failed to capture visitorData")
	}

	mu.Lock()
	if capturedPOToken {
		fmt.Printf("Session ready: POToken (%d chars), VisitorData (%d chars)\n",
			len(session.POToken), len(session.VisitorData))
	} else {
		fmt.Println("Warning: No POToken captured, proceeding with VisitorData only...")
	}
	mu.Unlock()

	return &session, nil
}

func (e *YouTubeExtractor) callInnertubeAPI(videoID string, session *YouTubeSession) (*InnertubeResponse, error) {
	// Use iOS client - no signature cipher needed!
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "IOS",
				"clientVersion": "20.11.6",
				"deviceMake":    "Apple",
				"deviceModel":   "iPhone16,2",
				"userAgent":     "com.google.ios.youtube/20.11.6 (iPhone16,2; U; CPU iOS 18_1_0 like Mac OS X;)",
				"osName":        "iOS",
				"osVersion":     "18.1.0.22B83",
				"hl":            "en",
				"gl":            "US",
				"visitorData":   session.VisitorData,
			},
		},
		"videoId": videoID,
		"playbackContext": map[string]interface{}{
			"contentPlaybackContext": map[string]interface{}{
				"signatureTimestamp": 20073,
			},
		},
		"contentCheckOk": true,
		"racyCheckOk":    true,
	}

	// Add poToken if available
	if session.POToken != "" {
		payload["serviceIntegrityDimensions"] = map[string]interface{}{
			"poToken": session.POToken,
		}
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call Innertube API
	apiURL := "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "com.google.ios.youtube/20.11.6 (iPhone16,2; U; CPU iOS 18_1_0 like Mac OS X;)")
	req.Header.Set("X-Youtube-Client-Name", "5") // iOS client ID
	req.Header.Set("X-Youtube-Client-Version", "20.11.6")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Debug: save response
	e.saveDebugResponse(body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response InnertubeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check playability
	if response.PlayabilityStatus.Status != "OK" {
		return nil, fmt.Errorf("video not playable: %s - %s",
			response.PlayabilityStatus.Status,
			response.PlayabilityStatus.Reason)
	}

	return &response, nil
}

func (e *YouTubeExtractor) parseResponse(resp *InnertubeResponse) (Media, error) {
	var formats []VideoFormat

	// Find best audio tracks (one for mp4, one for webm)
	var bestMP4Audio, bestWebMAudio string
	var bestMP4Bitrate, bestWebMBitrate int

	for _, f := range resp.StreamingData.AdaptiveFormats {
		if f.URL == "" || !strings.Contains(f.MimeType, "audio/") {
			continue
		}

		if strings.Contains(f.MimeType, "mp4a") && f.Bitrate > bestMP4Bitrate {
			bestMP4Audio = f.URL
			bestMP4Bitrate = f.Bitrate
		}
		if strings.Contains(f.MimeType, "opus") && f.Bitrate > bestWebMBitrate {
			bestWebMAudio = f.URL
			bestWebMBitrate = f.Bitrate
		}
	}

	// First try HLS manifest (has both video + audio)
	if resp.StreamingData.HLSManifestURL != "" {
		formats = append(formats, VideoFormat{
			URL:     resp.StreamingData.HLSManifestURL,
			Quality: "auto (HLS)",
			Ext:     "m3u8",
		})
	}

	// Add combined formats (video + audio together)
	for _, f := range resp.StreamingData.Formats {
		if f.URL == "" {
			continue
		}

		ext := "mp4"
		if strings.Contains(f.MimeType, "webm") {
			ext = "webm"
		}

		formats = append(formats, VideoFormat{
			URL:     f.URL,
			Quality: f.QualityLabel,
			Ext:     ext,
			Width:   f.Width,
			Height:  f.Height,
			Bitrate: f.Bitrate,
		})
	}

	// Add adaptive formats with paired audio
	for _, f := range resp.StreamingData.AdaptiveFormats {
		if f.URL == "" {
			continue
		}

		// Only include video formats
		if !strings.Contains(f.MimeType, "video/") {
			continue
		}

		ext := "mp4"
		audioURL := bestMP4Audio
		if strings.Contains(f.MimeType, "webm") {
			ext = "webm"
			audioURL = bestWebMAudio
		}

		quality := f.QualityLabel
		if quality == "" {
			quality = fmt.Sprintf("%dp", f.Height)
		}

		// Mark if needs merging
		qualityLabel := quality
		if audioURL != "" {
			qualityLabel = quality + " (needs merge)"
		} else {
			qualityLabel = quality + " (no audio)"
		}

		formats = append(formats, VideoFormat{
			URL:      f.URL,
			AudioURL: audioURL,
			Quality:  qualityLabel,
			Ext:      ext,
			Width:    f.Width,
			Height:   f.Height,
			Bitrate:  f.Bitrate,
		})
	}

	if len(formats) == 0 {
		return nil, fmt.Errorf("no downloadable formats found (all may require cipher decryption)")
	}

	// Get thumbnail
	var thumbnail string
	if len(resp.VideoDetails.Thumbnail.Thumbnails) > 0 {
		thumbnail = resp.VideoDetails.Thumbnail.Thumbnails[len(resp.VideoDetails.Thumbnail.Thumbnails)-1].URL
	}

	return &VideoMedia{
		ID:        resp.VideoDetails.VideoID,
		Title:     resp.VideoDetails.Title,
		Uploader:  resp.VideoDetails.Author,
		Thumbnail: thumbnail,
		Formats:   formats,
	}, nil
}

func (e *YouTubeExtractor) createLauncher(headless bool) *launcher.Launcher {
	userDataDir := e.getUserDataDir()

	l := launcher.New().
		Headless(headless).
		UserDataDir(userDataDir).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage")

	// Support HTTP_PROXY / HTTPS_PROXY environment variables
	if proxy := os.Getenv("HTTPS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		fmt.Printf("Using proxy: %s\n", proxy)
	} else if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		fmt.Printf("Using proxy: %s\n", proxy)
	}

	return l
}

func (e *YouTubeExtractor) getUserDataDir() string {
	configDir, err := config.ConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "vget-browser")
	}
	return filepath.Join(configDir, "browser")
}

func (e *YouTubeExtractor) saveSession(session *YouTubeSession) {
	configDir, err := config.ConfigDir()
	if err != nil {
		return
	}

	// Don't save cookies to reduce file size
	sessionToSave := YouTubeSession{
		POToken:     session.POToken,
		VisitorData: session.VisitorData,
	}

	data, err := json.MarshalIndent(sessionToSave, "", "  ")
	if err != nil {
		return
	}

	sessionPath := filepath.Join(configDir, "youtube_session.json")
	_ = os.WriteFile(sessionPath, data, 0600)
}

func (e *YouTubeExtractor) saveDebugResponse(body []byte) {
	configDir, err := config.ConfigDir()
	if err != nil {
		return
	}

	debugPath := filepath.Join(configDir, "youtube_debug_response.json")
	_ = os.WriteFile(debugPath, body, 0644)
	fmt.Printf("Debug: saved API response to %s\n", debugPath)
}

func init() {
	Register(&YouTubeExtractor{},
		"youtube.com",
		"www.youtube.com",
		"youtu.be",
		"m.youtube.com",
		"music.youtube.com",
	)
}
