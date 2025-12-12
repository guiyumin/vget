package extractor

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/guiyumin/vget/internal/core/config"
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

// NewGenericBrowserExtractor creates a browser extractor for unknown sites (defaults to m3u8)
func NewGenericBrowserExtractor(visible bool) *BrowserExtractor {
	return &BrowserExtractor{
		site:    &config.Site{Type: "m3u8"},
		visible: visible,
	}
}

func (e *BrowserExtractor) Name() string {
	return "browser"
}

func (e *BrowserExtractor) Match(u *url.URL) bool {
	return true // Called only when site matches
}

// extractionStrategy defines a method for finding media URLs
type extractionStrategy func(page *rod.Page, targetExt string) string

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

	// Normalize extension to lowercase once for consistent matching
	targetExt := strings.ToLower("." + e.site.Type) // e.g., ".m3u8", ".mp4"

	fmt.Printf("  Trying to detecting %s stream...\n", e.site.Type)

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

	// Try network interception first, then fallback strategies
	mediaURL := e.captureFromNetwork(page, rawURL, targetExt)

	// Fallback strategies if network capture didn't find anything
	if mediaURL == "" {
		strategies := []extractionStrategy{
			e.findInPerformanceAPI,
			e.findInVideoPlayer,
			e.findInPageSource,
		}

		for _, strategy := range strategies {
			if found := strategy(page, targetExt); found != "" {
				mediaURL = found
				break
			}
		}
	}

	if mediaURL == "" {
		return nil, fmt.Errorf("website not supported (no %s stream found)", e.site.Type)
	}

	fmt.Printf("Found: %s\n", mediaURL)

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

// captureFromNetwork intercepts network requests to find media URLs
func (e *BrowserExtractor) captureFromNetwork(page *rod.Page, rawURL, targetExt string) string {
	// Enable Network domain to capture requests
	_ = proto.NetworkEnable{}.Call(page)

	// Also enable Fetch domain to intercept at lower level
	_ = proto.FetchEnable{
		Patterns: []*proto.FetchRequestPattern{
			{URLPattern: "*"},
		},
	}.Call(page)

	// Use channel for thread-safe communication
	foundURL := make(chan string, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Separate context for the listener so we can stop it independently
	listenerCtx, stopListener := context.WithCancel(context.Background())
	listenerDone := make(chan struct{})

	// Listen for network requests at CDP level
	go func() {
		defer close(listenerDone)
		page.Context(listenerCtx).EachEvent(
			func(ev *proto.NetworkRequestWillBeSent) {
				reqURL := ev.Request.URL
				if strings.Contains(strings.ToLower(reqURL), targetExt) {
					select {
					case foundURL <- reqURL:
					default:
						// Already found one, ignore
					}
				}
			},
			func(ev *proto.FetchRequestPaused) {
				reqURL := ev.Request.URL
				// Continue the request regardless
				_ = proto.FetchContinueRequest{RequestID: ev.RequestID}.Call(page)
				if strings.Contains(strings.ToLower(reqURL), targetExt) {
					select {
					case foundURL <- reqURL:
					default:
						// Already found one, ignore
					}
				}
			},
		)()
	}()

	// Navigate with timeout to prevent hanging on slow/broken pages
	navCtx, navCancel := context.WithTimeout(ctx, 10*time.Second)
	_ = page.Context(navCtx).Navigate(rawURL)
	_ = page.Context(navCtx).WaitLoad()
	navCancel()

	// Wait for capture or timeout
	var result string
	select {
	case url := <-foundURL:
		result = url
	case <-ctx.Done():
		// Timeout: check one more time in case URL arrived just as we timed out
		select {
		case url := <-foundURL:
			result = url
		default:
			// No URL found
		}
	}

	// Stop the listener and wait for it to finish
	stopListener()
	<-listenerDone

	return result
}

// findInPerformanceAPI uses the browser's Performance API to find resource requests
func (e *BrowserExtractor) findInPerformanceAPI(page *rod.Page, targetExt string) string {
	// Pass targetExt to JavaScript for filtering (already lowercase)
	result, err := page.Eval(`(ext) => {
		return performance.getEntriesByType('resource')
			.map(r => r.name)
			.filter(url => url.toLowerCase().includes(ext));
	}`, targetExt)
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

// findInVideoPlayer queries the video player for its source URL
func (e *BrowserExtractor) findInVideoPlayer(page *rod.Page, targetExt string) string {
	// targetExt is already lowercase
	result, err := page.Eval(`(ext) => {
		// Check for video.js
		const vjsPlayer = document.querySelector('.video-js');
		if (vjsPlayer && vjsPlayer.player) {
			const src = vjsPlayer.player.currentSrc();
			if (src && src.toLowerCase().includes(ext)) return src;
		}

		// Check video element sources
		const video = document.querySelector('video');
		if (video) {
			if (video.src && video.src.toLowerCase().includes(ext)) return video.src;
			const sources = video.querySelectorAll('source');
			for (const source of sources) {
				if (source.src && source.src.toLowerCase().includes(ext)) return source.src;
			}
		}

		// Check for any global player variable
		if (window.player && window.player.src) {
			const src = typeof window.player.src === 'function' ? window.player.src() : window.player.src;
			if (src && src.toLowerCase().includes(ext)) return src;
		}
		return '';
	}`, targetExt)
	if err != nil {
		return ""
	}
	return result.Value.String()
}

// findInPageSource searches for media URLs in page HTML/JavaScript source
func (e *BrowserExtractor) findInPageSource(page *rod.Page, targetExt string) string {
	html, err := page.HTML()
	if err != nil {
		return ""
	}

	// Escape special regex characters in targetExt (already lowercase)
	escapedExt := regexp.QuoteMeta(targetExt)

	// Case-insensitive patterns
	patterns := []string{
		// Full URL with extension
		`(?i)https?://[^"'\s<>]+` + escapedExt + `[^"'\s<>]*`,
		// Quoted string containing extension
		`(?i)["']([^"']*` + escapedExt + `[^"']*)["']`,
		// src attribute with extension
		`(?i)src\s*[=:]\s*["']([^"']*` + escapedExt + `[^"']*)["']`,
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			var foundURL string
			if len(match) > 1 {
				foundURL = match[1]
			} else {
				foundURL = match[0]
			}

			if !strings.Contains(strings.ToLower(foundURL), targetExt) {
				continue
			}

			if strings.HasPrefix(foundURL, "data:") {
				continue
			}

			foundURL = strings.TrimSpace(foundURL)
			if foundURL != "" {
				return foundURL
			}
		}
	}

	return ""
}

func (e *BrowserExtractor) createLauncher(headless bool) *launcher.Launcher {
	userDataDir := e.getUserDataDir()

	// Check for ROD_BROWSER env var (set in Docker)
	browserPath := os.Getenv("ROD_BROWSER")

	l := launcher.New().
		Headless(headless).
		UserDataDir(userDataDir).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("disable-software-rasterizer").
		Set("disable-extensions").
		Set("disable-background-networking").
		Set("disable-sync").
		Set("disable-translate").
		Set("no-first-run").
		Set("safebrowsing-disable-auto-update").
		Set("window-size", "1920,1080").
		Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Explicitly set browser path if provided (required for Docker)
	if browserPath != "" {
		l = l.Bin(browserPath)
	}

	return l
}

func (e *BrowserExtractor) getUserDataDir() string {
	configDir, err := config.ConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "vget-browser")
	}
	return filepath.Join(configDir, "browser")
}
