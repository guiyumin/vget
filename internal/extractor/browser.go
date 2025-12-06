package extractor

import (
	"fmt"
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

// BrowserExtractor uses browser automation to intercept media URLs
type BrowserExtractor struct {
	site    *config.Site
	visible bool
}

// NewBrowserExtractor creates a new browser extractor for the given site
func NewBrowserExtractor(site *config.Site, visible bool) *BrowserExtractor {
	return &BrowserExtractor{site: site, visible: visible}
}

func (e *BrowserExtractor) Name() string {
	return "browser"
}

func (e *BrowserExtractor) Match(u *url.URL) bool {
	return true // Called only when site matches
}

// capturedRequest holds information about an intercepted request
type capturedRequest struct {
	url     string
	headers map[string]string
}

func (e *BrowserExtractor) Extract(rawURL string) (Media, error) {
	if e.site == nil {
		return nil, fmt.Errorf("no site configuration provided")
	}

	// Parse the page URL to get origin
	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	pageOrigin := fmt.Sprintf("%s://%s", pageURL.Scheme, pageURL.Host)

	// Determine what extension to look for
	targetExt := "." + e.site.Type // e.g., ".m3u8", ".mp4"

	fmt.Printf("Looking for %s requests...\n", e.site.Type)

	// Launch browser
	l := e.createLauncher(!e.visible) // headless unless --visible flag
	defer l.Cleanup()

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	defer page.MustClose()

	// Try multiple strategies in order
	strategies := []struct {
		name string
		fn   func() string
	}{
		{"fetch_intercept", func() string { return e.strategyFetchIntercept(page, rawURL, targetExt) }},
		{"network_hijack", func() string { return e.strategyNetworkHijack(page, rawURL, pageOrigin, targetExt) }},
		{"video_player", func() string { return e.findM3U8FromVideoPlayer(page) }},
		{"performance_api", func() string { return e.findM3U8InPerformance(page, targetExt) }},
		{"page_source", func() string { html, _ := page.HTML(); return e.findM3U8InSource(html) }},
	}

	var mediaURL string
	for _, strategy := range strategies {
		fmt.Printf("Trying strategy: %s\n", strategy.name)
		mediaURL = strategy.fn()
		if mediaURL != "" {
			fmt.Printf("Found via %s: %s\n", strategy.name, mediaURL)
			break
		}
	}

	if mediaURL == "" {
		return nil, fmt.Errorf("no %s request captured", e.site.Type)
	}

	// Extract page title
	title := page.MustEval(`() => document.title`).String()
	title = strings.TrimSpace(title)
	if title == "" {
		pageURL, _ := url.Parse(rawURL)
		title = filepath.Base(pageURL.Path)
		if title == "" || title == "/" {
			title = pageURL.Host
		}
	}

	// Generate ID from URL
	parsedURL, _ := url.Parse(mediaURL)
	id := filepath.Base(parsedURL.Path)
	if idx := strings.LastIndex(id, "."); idx > 0 {
		id = id[:idx]
	}
	if id == "" || id == "/" {
		id = "video"
	}

	return &VideoMedia{
		ID:    id,
		Title: title,
		Formats: []VideoFormat{
			{
				URL:     mediaURL,
				Quality: "best",
				Ext:     e.site.Type,
				Headers: map[string]string{"Referer": rawURL, "Origin": pageOrigin},
			},
		},
	}, nil
}

// strategyFetchIntercept injects a fetch interceptor before page loads
func (e *BrowserExtractor) strategyFetchIntercept(page *rod.Page, rawURL, targetExt string) string {
	// Inject script to intercept fetch calls before any JS runs
	page.MustEvalOnNewDocument(`
		window.__capturedM3U8 = [];
		const originalFetch = window.fetch;
		window.fetch = function(...args) {
			const url = args[0]?.url || args[0] || '';
			if (url.toLowerCase().includes('.m3u8')) {
				window.__capturedM3U8.push(url);
			}
			return originalFetch.apply(this, args);
		};
		// Also intercept XMLHttpRequest
		const originalOpen = XMLHttpRequest.prototype.open;
		XMLHttpRequest.prototype.open = function(method, url, ...rest) {
			if (url.toLowerCase().includes('.m3u8')) {
				window.__capturedM3U8.push(url);
			}
			return originalOpen.call(this, method, url, ...rest);
		};
	`)

	_ = page.Navigate(rawURL)
	_ = page.WaitLoad()
	time.Sleep(3 * time.Second) // Wait for video player to init

	// Get captured URLs
	result, err := page.Eval(`() => window.__capturedM3U8 || []`)
	if err != nil {
		return ""
	}

	arr := result.Value.Arr()
	for _, v := range arr {
		url := v.String()
		if strings.Contains(strings.ToLower(url), targetExt) {
			return url
		}
	}

	return ""
}

// strategyNetworkHijack uses HijackRequests to intercept network requests
func (e *BrowserExtractor) strategyNetworkHijack(page *rod.Page, rawURL, _, targetExt string) string {
	var result string
	var mu sync.Mutex
	done := make(chan struct{})
	closed := false

	router := page.HijackRequests()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		reqURL := ctx.Request.URL().String()
		if strings.Contains(strings.ToLower(reqURL), targetExt) {
			mu.Lock()
			if result == "" {
				result = reqURL
				if !closed {
					closed = true
					close(done)
				}
			}
			mu.Unlock()
		}
		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})

	go router.Run()

	// Wait a moment for hijack to be ready
	time.Sleep(100 * time.Millisecond)

	_ = page.Navigate(rawURL)
	_ = page.WaitLoad()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
	}

	router.Stop()
	return result
}

// strategyNetworkEvents uses Network domain events to capture requests
func (e *BrowserExtractor) strategyNetworkEvents(page *rod.Page, browser *rod.Browser, rawURL, pageOrigin, targetExt string) string {
	var result string
	var mu sync.Mutex
	done := make(chan struct{})

	_ = proto.NetworkEnable{}.Call(page)

	wait := browser.EachEvent(func(ev *proto.NetworkRequestWillBeSent) {
		reqURL := ev.Request.URL
		if strings.Contains(strings.ToLower(reqURL), targetExt) {
			mu.Lock()
			if result == "" {
				result = reqURL
				close(done)
			}
			mu.Unlock()
		}
	})

	go wait()

	_ = page.Reload()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}

	return result
}

// findM3U8InPerformance uses the browser's Performance API to find resource requests
func (e *BrowserExtractor) findM3U8InPerformance(page *rod.Page, targetExt string) string {
	// Query the Performance API for all resource entries
	result, err := page.Eval(`() => {
		return performance.getEntriesByType('resource')
			.map(r => r.name)
			.filter(url => url.toLowerCase().includes('.m3u8') || url.toLowerCase().includes('.ts') || url.toLowerCase().includes('hls'));
	}`)
	if err != nil {
		return ""
	}

	// Parse the result
	arr := result.Value.Arr()
	for _, v := range arr {
		url := v.String()
		if strings.Contains(strings.ToLower(url), targetExt) {
			return url
		}
	}

	return ""
}

// findM3U8InSource searches for m3u8 URLs in page HTML/JavaScript source
func (e *BrowserExtractor) findM3U8InSource(html string) string {
	// Common patterns for m3u8 URLs in page source
	patterns := []string{
		// Direct m3u8 URL patterns
		`https?://[^"'\s<>]+\.m3u8[^"'\s<>]*`,
		// URL in quotes
		`["']([^"']*\.m3u8[^"']*)["']`,
		// source attribute
		`src\s*[=:]\s*["']([^"']*\.m3u8[^"']*)["']`,
		// file/url parameter
		`(?:file|url|source|src)\s*[=:]\s*["']([^"']+)["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			var url string
			if len(match) > 1 {
				url = match[1]
			} else {
				url = match[0]
			}

			// Must contain m3u8
			if !strings.Contains(strings.ToLower(url), ".m3u8") {
				continue
			}

			// Skip data URLs and invalid URLs
			if strings.HasPrefix(url, "data:") {
				continue
			}

			// Clean up the URL
			url = strings.TrimSpace(url)
			if url != "" {
				return url
			}
		}
	}

	return ""
}

// extractM3U8FromJSON extracts m3u8 URL from JSON response body
func extractM3U8FromJSON(body string) string {
	// Look for m3u8 URLs in the JSON
	re := regexp.MustCompile(`https?://[^"'\s]+\.m3u8[^"'\s]*`)
	matches := re.FindAllString(body, -1)
	for _, match := range matches {
		// Clean up any trailing quotes or escaped chars
		match = strings.TrimRight(match, `"'\`)
		if strings.Contains(match, ".m3u8") {
			return match
		}
	}
	return ""
}

// findM3U8FromVideoPlayer queries the video player for its source URL
func (e *BrowserExtractor) findM3U8FromVideoPlayer(page *rod.Page) string {
	// Try to get the source from various video player APIs
	result, err := page.Eval(`() => {
		// Check for HLS.js
		if (window.Hls && window.hls) {
			return window.hls.url || '';
		}
		// Check for video.js
		const vjsPlayer = document.querySelector('.video-js');
		if (vjsPlayer && vjsPlayer.player) {
			const src = vjsPlayer.player.currentSrc();
			if (src && src.includes('.m3u8')) return src;
		}
		// Check video element sources
		const video = document.querySelector('video');
		if (video) {
			if (video.src && video.src.includes('.m3u8')) return video.src;
			const source = video.querySelector('source[src*=".m3u8"]');
			if (source) return source.src;
		}
		// Check for any global player variable
		if (window.player && window.player.src) {
			const src = typeof window.player.src === 'function' ? window.player.src() : window.player.src;
			if (src && src.includes('.m3u8')) return src;
		}
		return '';
	}`)
	if err != nil {
		return ""
	}
	return result.Value.String()
}

func (e *BrowserExtractor) createLauncher(headless bool) *launcher.Launcher {
	userDataDir := e.getUserDataDir()

	l := launcher.New().
		Headless(headless).
		UserDataDir(userDataDir).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage")

	return l
}

func (e *BrowserExtractor) getUserDataDir() string {
	configDir, err := config.ConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "vget-browser")
	}
	return filepath.Join(configDir, "browser")
}
