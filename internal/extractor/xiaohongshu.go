package extractor

import (
	"encoding/json"
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
	"github.com/guiyumin/vget/internal/config"
)

// XiaohongshuExtractor handles Xiaohongshu video/image downloads using browser automation
type XiaohongshuExtractor struct{}

func (e *XiaohongshuExtractor) Name() string {
	return "xiaohongshu"
}

func (e *XiaohongshuExtractor) Match(u *url.URL) bool {
	return true
}

// xhsNoteDetail represents the note detail from __INITIAL_STATE__
type xhsNoteDetail struct {
	Note struct {
		NoteID    string `json:"noteId"`
		Title     string `json:"title"`
		Desc      string `json:"desc"`
		Type      string `json:"type"` // "normal" (image) or "video"
		User      struct {
			Nickname string `json:"nickname"`
			UserID   string `json:"userId"`
		} `json:"user"`
		ImageList []struct {
			URLDefault string `json:"urlDefault"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
		} `json:"imageList"`
		Video struct {
			Media struct {
				Stream struct {
					H264 []struct {
						MasterURL string `json:"masterUrl"`
					} `json:"h264"`
				} `json:"stream"`
			} `json:"media"`
		} `json:"video"`
	} `json:"note"`
}

func (e *XiaohongshuExtractor) Extract(rawURL string) (Media, error) {
	// Resolve short URL if needed
	finalURL := rawURL
	if strings.Contains(rawURL, "xhslink.com") {
		resolved, err := e.resolveShortURL(rawURL)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve short URL: %w", err)
		}
		finalURL = resolved
	}

	// Extract note ID from URL
	noteID := e.extractNoteID(finalURL)
	if noteID == "" {
		return nil, fmt.Errorf("could not extract note ID from URL: %s", finalURL)
	}

	// Launch browser and extract data
	return e.extractWithBrowser(finalURL, noteID)
}

func (e *XiaohongshuExtractor) extractNoteID(rawURL string) string {
	// Pattern: /explore/{noteId} or /discovery/item/{noteId}
	patterns := []string{
		`/explore/([a-zA-Z0-9]+)`,
		`/discovery/item/([a-zA-Z0-9]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(rawURL)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func (e *XiaohongshuExtractor) resolveShortURL(shortURL string) (string, error) {
	// Use browser to follow redirect
	l := e.createLauncher(true) // headless for redirect resolution
	defer l.Cleanup()

	u := l.MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	defer page.MustClose()

	page.MustNavigate(shortURL)
	page.MustWaitDOMStable()

	return page.MustInfo().URL, nil
}

func (e *XiaohongshuExtractor) extractWithBrowser(targetURL, noteID string) (Media, error) {
	// Launch browser (non-headless for now, to handle login if needed)
	l := e.createLauncher(false)
	defer l.Cleanup()

	fmt.Println("Launching browser...")

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}
	fmt.Printf("Browser launched, connecting to: %s\n", u)

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()
	fmt.Println("Connected to browser")

	// Load cookies if available
	e.loadCookies(browser)

	page := stealth.MustPage(browser)
	defer page.MustClose()
	fmt.Println("Created stealth page")

	fmt.Printf("Navigating to: %s\n", targetURL)

	// Navigate to the page
	page.MustNavigate(targetURL)
	fmt.Println("Waiting for page to stabilize...")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second) // Extra wait for JS rendering
	fmt.Println("Page loaded, checking for data...")

	// Wait for login if needed (up to 120 seconds)
	var result string
	maxWait := 120 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	for {
		// Try to extract __INITIAL_STATE__
		result = page.MustEval(`() => {
			if (window.__INITIAL_STATE__ &&
			    window.__INITIAL_STATE__.note &&
			    window.__INITIAL_STATE__.note.noteDetailMap) {
				const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
				return JSON.stringify(noteDetailMap);
			}
			return "";
		}`).String()

		if result != "" {
			fmt.Println("Data extracted successfully!")
			break
		}

		elapsed := time.Since(startTime)
		if elapsed >= maxWait {
			return nil, fmt.Errorf("timeout waiting for note data (login may be required)")
		}

		// Check if this is the first iteration - show login prompt
		if elapsed < checkInterval*2 {
			fmt.Println("\n┌────────────────────────────────────────────────────┐")
			fmt.Println("│  Login required! Please scan the QR code in the   │")
			fmt.Println("│  browser window to log in to Xiaohongshu.         │")
			fmt.Println("│                                                    │")
			fmt.Println("│  Waiting up to 2 minutes for login...             │")
			fmt.Println("└────────────────────────────────────────────────────┘")
		}

		remaining := maxWait - elapsed
		fmt.Printf("\rWaiting for login... %d seconds remaining", int(remaining.Seconds()))

		time.Sleep(checkInterval)

		// Refresh the page state after waiting
		page.MustWaitDOMStable()
	}
	fmt.Println() // newline after progress

	// Save cookies for future sessions
	e.saveCookies(browser)

	// Parse the response
	var noteDetailMap map[string]xhsNoteDetail
	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return nil, fmt.Errorf("failed to parse note data: %w", err)
	}

	// Debug: print available keys
	fmt.Printf("Looking for noteID: %s\n", noteID)
	fmt.Printf("Available keys in noteDetailMap:\n")
	for key := range noteDetailMap {
		fmt.Printf("  - %s\n", key)
	}

	noteDetail, exists := noteDetailMap[noteID]
	if !exists {
		// Try to find any key that contains the noteID
		for key, detail := range noteDetailMap {
			if strings.Contains(key, noteID) {
				fmt.Printf("Found matching key: %s\n", key)
				noteDetail = detail
				exists = true
				break
			}
		}
	}

	// If still not found, just use the first entry if there's only one
	if !exists && len(noteDetailMap) == 1 {
		for key, detail := range noteDetailMap {
			fmt.Printf("Using single available key: %s\n", key)
			noteDetail = detail
			exists = true
			_ = key
			break
		}
	}

	if !exists {
		return nil, fmt.Errorf("note %s not found in response (available: %d keys)", noteID, len(noteDetailMap))
	}

	note := noteDetail.Note
	title := note.Title
	if title == "" {
		title = note.Desc
	}
	if title == "" {
		title = note.NoteID
	}

	// Check if it's a video or image post
	if note.Type == "video" {
		return e.extractVideo(note.NoteID, title, note.User.Nickname, noteDetail)
	}

	// Image post
	return e.extractImages(note.NoteID, title, note.User.Nickname, noteDetail)
}

func (e *XiaohongshuExtractor) extractVideo(id, title, uploader string, detail xhsNoteDetail) (Media, error) {
	var videoURL string

	// Try to get video URL from the structure
	h264Streams := detail.Note.Video.Media.Stream.H264
	if len(h264Streams) > 0 && h264Streams[0].MasterURL != "" {
		videoURL = h264Streams[0].MasterURL
	}

	if videoURL == "" {
		return nil, fmt.Errorf("could not find video URL in note data")
	}

	// Ensure HTTPS
	if strings.HasPrefix(videoURL, "//") {
		videoURL = "https:" + videoURL
	}

	return &VideoMedia{
		ID:       id,
		Title:    title,
		Uploader: uploader,
		Formats: []VideoFormat{
			{
				URL:     videoURL,
				Quality: "best",
				Ext:     "mp4",
			},
		},
	}, nil
}

func (e *XiaohongshuExtractor) extractImages(id, title, uploader string, detail xhsNoteDetail) (Media, error) {
	if len(detail.Note.ImageList) == 0 {
		return nil, fmt.Errorf("no images found in note")
	}

	var images []Image
	for _, img := range detail.Note.ImageList {
		imgURL := img.URLDefault
		if strings.HasPrefix(imgURL, "//") {
			imgURL = "https:" + imgURL
		}

		ext := "jpg" // default
		if strings.Contains(imgURL, ".png") {
			ext = "png"
		} else if strings.Contains(imgURL, ".webp") {
			ext = "webp"
		}

		images = append(images, Image{
			URL:    imgURL,
			Ext:    ext,
			Width:  img.Width,
			Height: img.Height,
		})
	}

	return &ImageMedia{
		ID:       id,
		Title:    title,
		Uploader: uploader,
		Images:   images,
	}, nil
}

func (e *XiaohongshuExtractor) createLauncher(headless bool) *launcher.Launcher {
	// Use Rod's auto-downloaded Chromium with persistent user data
	// This keeps login state between runs
	userDataDir := e.getUserDataDir()

	l := launcher.New().
		Headless(headless).
		UserDataDir(userDataDir).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage")

	return l
}

// getUserDataDir returns the persistent browser data directory
// Located at ~/.config/vget/browser/
// This is shared across all extractors that need browser automation
func (e *XiaohongshuExtractor) getUserDataDir() string {
	configDir, err := config.ConfigDir()
	if err != nil {
		// Fallback to temp dir if config dir unavailable
		return filepath.Join(os.TempDir(), "vget-browser")
	}
	return filepath.Join(configDir, "browser")
}

func (e *XiaohongshuExtractor) loadCookies(browser *rod.Browser) {
	// Try to load cookies from ~/.config/vget/xhs_cookies.json
	configDir, err := config.ConfigDir()
	if err != nil {
		return
	}

	cookiePath := filepath.Join(configDir, "xhs_cookies.json")
	data, err := os.ReadFile(cookiePath)
	if err != nil {
		return // No cookies file, that's fine
	}

	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return
	}

	fmt.Println("Loaded saved cookies from previous session")
	browser.MustSetCookies(cookies...)
}

func (e *XiaohongshuExtractor) saveCookies(browser *rod.Browser) {
	// Save cookies to ~/.config/vget/xhs_cookies.json
	configDir, err := config.ConfigDir()
	if err != nil {
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return
	}

	cookies, err := browser.GetCookies()
	if err != nil {
		return
	}

	// Filter to only XHS-related cookies
	var xhsCookies []*proto.NetworkCookie
	for _, c := range cookies {
		if strings.Contains(c.Domain, "xiaohongshu") || strings.Contains(c.Domain, "xhscdn") {
			xhsCookies = append(xhsCookies, c)
		}
	}

	if len(xhsCookies) == 0 {
		return
	}

	data, err := json.MarshalIndent(xhsCookies, "", "  ")
	if err != nil {
		return
	}

	cookiePath := filepath.Join(configDir, "xhs_cookies.json")
	if err := os.WriteFile(cookiePath, data, 0600); err != nil {
		fmt.Printf("Warning: failed to save cookies: %v\n", err)
		return
	}
	fmt.Printf("Saved %d cookies for future sessions\n", len(xhsCookies))
}

func init() {
	Register(&XiaohongshuExtractor{},
		"xiaohongshu.com",
		"xhslink.com",
	)
}
