# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Build
go build ./cmd/vget

# Run directly
go run ./cmd/vget

# Build with version info (for releases)
go build -ldflags "-X github.com/guiyumin/vget/internal/version.Version=1.0.0" ./cmd/vget
```

## Architecture

vget is a video downloader CLI built with Go. It uses Cobra for command parsing and Bubbletea for interactive TUI elements (spinners, progress bars).

### Core Flow

1. **CLI Layer** (`internal/cli/`) - Cobra commands parse flags and dispatch to handlers
2. **Extractor Layer** (`internal/extractor/`) - URL matching and video metadata extraction
3. **Downloader Layer** (`internal/downloader/`) - HTTP download with Bubbletea progress TUI

### Extractor Pattern

To add support for a new site, implement the `Extractor` interface in `internal/extractor/`:

```go
type Extractor interface {
    Name() string
    Match(url string) bool
    Extract(url string) (*VideoInfo, error)
}
```

Then register it in `internal/extractor/registry.go`:

```go
func init() {
    Register(&TwitterExtractor{})
    Register(&YourNewExtractor{})  // Add here
}
```

### i18n

Translations are embedded YAML files in `internal/i18n/locales/`. Supported: en, zh, jp, kr, es, fr, de.

Access translations via `i18n.T(langCode)` which returns a `*Translations` struct with typed fields.

### Config

User config lives in `.vget.yml` (or `.vget.yaml`) in the current directory. The `vget init` command runs an interactive Bubbletea wizard to create it.

### Self-Update

`internal/updater/` uses go-selfupdate to fetch releases from GitHub (`guiyumin/vget`). Version is set in `internal/version/version.go`.
