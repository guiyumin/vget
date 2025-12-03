# Xiaohongshu MCP Analysis

Analysis of [xpzouying/xiaohongshu-mcp](https://github.com/xpzouying/xiaohongshu-mcp) for implementing a Xiaohongshu extractor in vget.

## Browser Automation Stack

The project uses **Rod** for browser automation via Chrome DevTools Protocol (CDP):

| Dependency | Purpose | Reputation |
|------------|---------|------------|
| `github.com/go-rod/rod` | Core browser automation library | ✅ 6k+ stars, actively maintained |
| `github.com/go-rod/stealth` | Anti-detection measures | ✅ Same maintainer as Rod |
| `github.com/xpzouying/headless_browser` | Thin wrapper (NOT recommended) | ⚠️ Personal lib, avoid |

### About `xpzouying/headless_browser`

This is a thin wrapper (~100 lines) that:
1. Wraps Rod with stealth mode enabled by default
2. Adds cookie loading from JSON
3. Provides simplified `NewPage()` API

**We should NOT use this library.** Instead, use Rod + stealth directly.

## How It Works

### 1. Browser Launch

Rod can launch Chrome in multiple ways:

```go
// Option 1: Auto-download Chromium
launcher.New().MustLaunch()

// Option 2: Use system Chrome (macOS)
launcher.New().
    Bin("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome").
    Headless(false).
    MustLaunch()

// Option 3: Connect to existing Chrome with remote debugging
// First: chrome --remote-debugging-port=9222
rod.New().ControlURL("ws://127.0.0.1:9222").MustConnect()
```

### 2. Data Extraction Strategy

Xiaohongshu embeds post data in `window.__INITIAL_STATE__` (server-side rendered). The MCP extracts it via JavaScript evaluation:

**Source:** `xiaohongshu/feed_detail.go:38-46`

```go
result := page.MustEval(`() => {
    if (window.__INITIAL_STATE__ &&
        window.__INITIAL_STATE__.note &&
        window.__INITIAL_STATE__.note.noteDetailMap) {
        const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
        return JSON.stringify(noteDetailMap);
    }
    return "";
}`).String()
```

### 3. URL Format

Post detail URL pattern:
```
https://www.xiaohongshu.com/explore/{feedID}?xsec_token={token}&xsec_source=pc_feed
```

**Source:** `xiaohongshu/feed_detail.go:72-74`

## Data Structures

### FeedDetail (Post Content)

**Source:** `xiaohongshu/types.go:94-106`

```go
type FeedDetail struct {
    NoteID       string            `json:"noteId"`
    XsecToken    string            `json:"xsecToken"`
    Title        string            `json:"title"`
    Desc         string            `json:"desc"`
    Type         string            `json:"type"`      // "normal" (image) or "video"
    Time         int64             `json:"time"`
    IPLocation   string            `json:"ipLocation"`
    User         User              `json:"user"`
    InteractInfo InteractInfo      `json:"interactInfo"`
    ImageList    []DetailImageInfo `json:"imageList"`
}
```

### Image Info

**Source:** `xiaohongshu/types.go:108-115`

```go
type DetailImageInfo struct {
    Width      int    `json:"width"`
    Height     int    `json:"height"`
    URLDefault string `json:"urlDefault"`  // <-- Direct image URL
    URLPre     string `json:"urlPre"`
    LivePhoto  bool   `json:"livePhoto,omitempty"`
}
```

### Video Info

**Source:** `xiaohongshu/types.go:76-84`

```go
type Video struct {
    Capa VideoCapability `json:"capa"`
}

type VideoCapability struct {
    Duration int `json:"duration"` // seconds
}
```

Note: Video URL extraction requires deeper analysis - the MCP primarily handles image posts.

## Cookie/Session Management

**Source:** `xiaohongshu/cookies/`

- Cookies stored in local file
- Loaded on browser init
- Saved after successful login (QR code flow)

```go
// Loading cookies on browser start
cookieLoader := cookies.NewLoadCookie(cookiePath)
if data, err := cookieLoader.LoadCookies(); err == nil {
    opts = append(opts, headless_browser.WithCookies(string(data)))
}
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `browser/browser.go` | Browser initialization wrapper |
| `configs/browser.go` | Headless mode & binary path config |
| `xiaohongshu/feed_detail.go` | Post detail extraction |
| `xiaohongshu/types.go` | Data structure definitions |
| `xiaohongshu/login.go` | QR code login flow |
| `service.go` | High-level service orchestration |

## vget Implementation (Completed)

The Xiaohongshu extractor has been implemented in vget. See `internal/extractor/xiaohongshu.go`.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    XiaohongshuExtractor                     │
├─────────────────────────────────────────────────────────────┤
│  Browser: Rod's auto-downloaded Chromium                    │
│  Data Dir: ~/.config/vget/xhs-browser/ (persistent)         │
│  Stealth: go-rod/stealth for anti-detection                 │
└─────────────────────────────────────────────────────────────┘
```

### Key Features

1. **Isolated Browser** - Uses Rod's Chromium, NOT system Chrome
   - Browser binary: `~/.cache/rod/browser/`
   - User data: `~/.config/vget/xhs-browser/`
   - No interference with user's Chrome profiles

2. **Login Wait** - If content requires login:
   - Shows prompt in terminal
   - Waits up to 2 minutes for QR code scan
   - Saves cookies for future sessions

3. **Persistent Sessions** - Browser data persists between runs
   - Login state preserved
   - No need to re-login each time

4. **URL Support:**
   ```
   https://www.xiaohongshu.com/explore/{noteId}
   https://www.xiaohongshu.com/discovery/item/{noteId}
   https://xhslink.com/{shortCode} (auto-resolved)
   ```

5. **Media Extraction:**
   - Images: `noteDetailMap[noteId].note.imageList[].urlDefault`
   - Videos: `noteDetailMap[noteId].note.video.media.stream.h264[0].masterUrl`

### Usage

```bash
# Download XHS video/images
vget https://www.xiaohongshu.com/explore/abc123

# Short URL also works
vget https://xhslink.com/xyz
```

### Important: Browser Isolation

**WARNING:** Never use system Chrome profiles with browser automation tools. This can corrupt session data.

The current implementation is safe:
- Uses Rod's separate Chromium binary
- Stores data in `~/.config/vget/xhs-browser/`
- Completely isolated from system Chrome

## Rod vs chromedp Comparison

Both are Go libraries for Chrome DevTools Protocol automation. Here's a detailed comparison:

### Stats (as of Dec 2024)

| Library | Stars | Age | Maintainer |
|---------|-------|-----|------------|
| chromedp | 12.5k | Older, more mature | Community |
| go-rod | 6.3k | Newer | Single author (ysmood) |

### API Style Comparison

**Same task: Navigate and extract text**

**chromedp:**
```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()

var title string
if err := chromedp.Run(ctx,
    chromedp.Navigate("https://example.com"),
    chromedp.Title(&title),
); err != nil {
    log.Fatal(err)
}
fmt.Println(title)
```

**Rod:**
```go
title := rod.New().MustConnect().
    MustPage("https://example.com").
    MustElement("title").
    MustText()
fmt.Println(title)
```

**Same task: Set cookies**

**chromedp:** (more verbose, requires lower-level API)
```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.DisableGPU,
)
allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
defer cancel()
ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()
// ... more setup needed for cookies
```

**Rod:**
```go
page := rod.New().MustConnect().MustPage()
page.MustSetCookies(&proto.NetworkCookieParam{
    Name:   "cookie1",
    Value:  "value1",
    Domain: "example.com",
})
```

### Feature Comparison

| Feature | chromedp | Rod |
|---------|----------|-----|
| API style | Verbose, context-based | Fluent, chainable |
| Error handling | Returns errors | `Must*` methods panic, or use non-Must |
| Stealth mode | ❌ No built-in | ✅ `go-rod/stealth` |
| Auto-wait | Manual | ✅ Auto-wait elements |
| Browser download | Manual | ✅ Auto-download |
| iFrame handling | Complex | ✅ Simple |
| Shadow DOM | Complex | ✅ Built-in |
| Test coverage | Good | 100% enforced |

### Stealth Mode (Critical for XHS)

**chromedp:** No built-in stealth. Need to manually:
- Override `navigator.webdriver`
- Modify user-agent
- Handle other fingerprinting

**Rod:** Built-in stealth plugin:
```go
import "github.com/go-rod/stealth"
page := stealth.MustPage(browser)  // Handles anti-bot automatically
```

### When to Choose Each

**Choose chromedp if:**
- You need maximum stability (older, battle-tested)
- You prefer explicit error handling everywhere
- You're already using it in existing code
- You don't need stealth/anti-bot features

**Choose Rod if:**
- You want cleaner, more readable code
- You need stealth mode for scraping
- You want auto-wait and auto-download features
- You're building something new

### Decision for vget: Use Rod + Stealth

For Xiaohongshu scraping, Rod is the better choice because:

1. **Stealth is critical** - XHS has anti-bot detection, Rod has built-in stealth
2. **Cleaner API** - Less boilerplate for our use case
3. **Auto-wait** - `MustWaitDOMStable()` handles JS rendering
4. **Good enough stability** - 6.3k stars, actively maintained

If we encounter issues with Rod, we can migrate to chromedp later - both use the same underlying CDP protocol.
