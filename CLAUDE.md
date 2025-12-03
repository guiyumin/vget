# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Build
go build ./cmd/vget

# Build to specific directory
go build -o build/vget ./cmd/vget

# Run directly
go run ./cmd/vget

# Build with version info (for releases)
go build -ldflags "-X github.com/guiyumin/vget/internal/version.Version=1.0.0" ./cmd/vget
```

## Architecture

vget is a media downloader CLI built with Go. It uses Cobra for command parsing and Bubbletea for interactive TUI elements (spinners, progress bars).

### Core Flow

1. **CLI Layer** (`internal/cli/`) - Cobra commands parse flags and dispatch to handlers
2. **Extractor Layer** (`internal/extractor/`) - URL matching and media metadata extraction
3. **Downloader Layer** (`internal/downloader/`) - HTTP download with Bubbletea progress TUI

### Media Types

The `MediaType` enum in `internal/extractor/extractor.go` defines supported media types:

- `MediaTypeVideo` - Video files (Twitter, YouTube, etc.)
- `MediaTypeAudio` - Audio files (podcasts)
- `MediaTypePDF` - PDF documents
- `MediaTypeEPUB` - EPUB ebooks
- `MediaTypeMOBI` - MOBI ebooks
- `MediaTypeAZW` - AZW ebooks
- `MediaTypeUnknown` - Fallback (treated as video)

Each type has specific terminal output formatting in `internal/cli/extract.go`.

### Extractor Pattern

To add support for a new site, implement the `Extractor` interface in `internal/extractor/`:

```go
type Extractor interface {
    Name() string
    Match(url string) bool
    Extract(url string) (*VideoInfo, error)
}
```

Set the appropriate `MediaType` in the returned `VideoInfo`:

```go
return &VideoInfo{
    ID:        "...",
    Title:     "...",
    MediaType: MediaTypeAudio, // or MediaTypeVideo, etc.
    Formats:   []Format{...},
}, nil
```

Extractors are auto-registered via `init()` functions. See `xiaoyuzhou.go` or `twitter.go` for examples.

### Commands

- `vget <url>` - Download media from URL
- `vget init` - Interactive config wizard
- `vget update` - Self-update to latest version
- `vget search --podcast <query>` - Search Xiaoyuzhou podcasts
- `vget ls <remote>:<path>` - List WebDAV remote directory
- `vget config show` - Show current configuration
- `vget config webdav ...` - Manage WebDAV servers

### i18n

Translations are embedded YAML files in `internal/i18n/locales/`. Supported: en, zh, jp, kr, es, fr, de.

Access translations via `i18n.T(langCode)` which returns a `*Translations` struct with typed fields.

### Config

User config lives in `~/.config/vget/config.yml`. The `vget init` command runs an interactive Bubbletea wizard to create it.

### Xiaohongshu (XHS) Extractor

The XHS extractor (`internal/extractor/xiaohongshu.go`) uses browser automation:

- **Browser**: Rod's auto-downloaded Chromium (NOT system Chrome)
- **Binary location**: `~/.cache/rod/browser/`
- **User data**: `~/.config/vget/xhs-browser/` (persistent between runs)
- **Stealth**: Uses `go-rod/stealth` for anti-bot detection

**Important**: Never use system Chrome profiles with browser automation - it can corrupt session data.

### Self-Update

`internal/updater/` uses go-selfupdate to fetch releases from GitHub (`guiyumin/vget`). Version is set in `internal/version/version.go`.

# My Rules

- BUILD OUTPUT DIRECTORY IS ./build
- YOU ONLY BUILD IT. I WILL TEST IT BY MYSELF.
-
