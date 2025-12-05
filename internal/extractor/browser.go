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
	site *config.Site
}

// NewBrowserExtractor creates a new browser extractor for the given site
func NewBrowserExtractor(site *config.Site) *BrowserExtractor {
	return &BrowserExtractor{site: site}
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
	l := e.createLauncher(true) // headless
	defer l.Cleanup()

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	defer page.MustClose()

	// Set up request interception using Network events (more reliable than Hijack)
	var captured *capturedRequest
	var mu sync.Mutex
	done := make(chan struct{})

	// Enable network domain to listen for requests
	err = proto.NetworkEnable{}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("failed to enable network events: %w", err)
	}

	// Listen for network requests
	go page.EachEvent(func(ev *proto.NetworkRequestWillBeSent) {
		reqURL := ev.Request.URL
		lowerURL := strings.ToLower(reqURL)

		// Check if this is the type we're looking for
		if strings.Contains(lowerURL, targetExt) {
			mu.Lock()
			if captured == nil {
				headers := map[string]string{
					"Referer": rawURL,
					"Origin":  pageOrigin,
				}

				captured = &capturedRequest{
					url:     reqURL,
					headers: headers,
				}
				fmt.Printf("Captured %s URL: %s\n", e.site.Type, reqURL)
				close(done)
			}
			mu.Unlock()
		}
	})()

	// Navigate to page
	fmt.Printf("Loading page: %s\n", rawURL)
	err = page.Navigate(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for either capture or timeout
	select {
	case <-done:
		// Got it!
	case <-time.After(10 * time.Second):
		// Not captured via event listener, try alternative methods
		mu.Lock()
		alreadyCaptured := captured != nil
		mu.Unlock()

		if !alreadyCaptured {
			// Check Performance API for requests we might have missed
			if m3u8URL := e.findM3U8InPerformance(page, targetExt); m3u8URL != "" {
				mu.Lock()
				captured = &capturedRequest{
					url:     m3u8URL,
					headers: map[string]string{},
				}
				mu.Unlock()
			}
		}

		mu.Lock()
		alreadyCaptured = captured != nil
		mu.Unlock()

		if !alreadyCaptured {
			// Fallback: search page source
			html, err := page.HTML()
			if err == nil {
				if m3u8URL := e.findM3U8InSource(html); m3u8URL != "" {
					mu.Lock()
					captured = &capturedRequest{
						url:     m3u8URL,
						headers: map[string]string{},
					}
					mu.Unlock()
				}
			}
		}
	}

	mu.Lock()
	result := captured
	mu.Unlock()

	if result == nil {
		return nil, fmt.Errorf("no %s request captured", e.site.Type)
	}

	// Extract page title
	title := page.MustEval(`() => document.title`).String()
	title = strings.TrimSpace(title)
	if title == "" {
		// Fallback to URL path
		pageURL, _ := url.Parse(rawURL)
		title = filepath.Base(pageURL.Path)
		if title == "" || title == "/" {
			title = pageURL.Host
		}
	}

	// Generate ID from URL
	parsedURL, _ := url.Parse(result.url)
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
				URL:     result.url,
				Quality: "best",
				Ext:     e.site.Type,
				Headers: result.headers,
			},
		},
	}, nil
}

// findM3U8InPerformance uses the browser's Performance API to find resource requests
func (e *BrowserExtractor) findM3U8InPerformance(page *rod.Page, targetExt string) string {
	// Query the Performance API for all resource entries
	result, err := page.Eval(`() => {
		return performance.getEntriesByType('resource')
			.map(r => r.name)
			.filter(url => url.toLowerCase().includes('.m3u8') || url.toLowerCase().includes('.ts'));
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
