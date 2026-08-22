# Docker Browser Launch Hang Bug

## Problem

Browser-based extractors (m3u8 detection, etc.) would hang indefinitely in Docker while working fine on macOS CLI.

**Symptoms:**
- CLI on Mac: Works
- CLI in Docker: Hangs at "Trying to detecting m3u8 stream..."
- Server in Docker: Same hang

## Root Cause

The `go-rod` library wasn't using the `ROD_BROWSER` environment variable to locate the system Chromium. Instead, it was attempting to download its own browser binary, which would hang in the containerized environment.

Even though `ROD_BROWSER=/usr/bin/chromium` was set in the Dockerfile, rod's `launcher.New()` doesn't automatically use this env var - it needs explicit `Bin()` call.

## Solution

In `internal/extractor/browser.go`, explicitly set the browser binary path:

```go
func (e *BrowserExtractor) createLauncher(headless bool) *launcher.Launcher {
    // Check for ROD_BROWSER env var (set in Docker)
    browserPath := os.Getenv("ROD_BROWSER")

    l := launcher.New().
        Headless(headless).
        // ... other options

    // Explicitly set browser path if provided (required for Docker)
    if browserPath != "" {
        l = l.Bin(browserPath)
    }

    return l
}
```

## Additional Changes

1. **Switched from Alpine to Debian** - Alpine's musl libc and Chromium package caused compatibility issues. Debian's glibc-based Chromium is more stable.

2. **Added Chrome flags** for better headless stability:
   - `disable-software-rasterizer`
   - `disable-extensions`
   - `disable-background-networking`
   - `window-size=1920,1080`
   - Custom user-agent to avoid bot detection

## Files Changed

- `internal/extractor/browser.go` - Added explicit `Bin()` call and Chrome flags
- `Dockerfile` - Switched to Debian bookworm-slim
- `docker/entrypoint.sh` - Changed from `su-exec` to `gosu` (Debian equivalent)
