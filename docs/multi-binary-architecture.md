# Multi-Binary Architecture

## Overview

vget is split into separate binaries with a shared core module:

| Binary | Purpose | Distribution |
|--------|---------|--------------|
| `vget` | CLI tool | GitHub Releases (all platforms) |
| `vget-server` | HTTP server + Web UI | GitHub Releases + Docker Image |
| `vget-desktop` | Desktop GUI (Fyne) | GitHub Releases (future) |

## Current Structure

```
cmd/
  vget/main.go              # CLI entry point
  vget-server/main.go       # Server entry point

internal/
  core/                     # Shared by all binaries
    config/                 # Config file management
    downloader/             # Download logic, progress callbacks
    extractor/              # URL matching, media extraction
    i18n/                   # Translations
    tracker/                # Package tracking (kuaidi100)
    version/                # Version info
    webdav/                 # WebDAV client

  cli/                      # CLI-specific (Cobra + Bubbletea TUI)
  server/                   # Server-specific (HTTP + job queue + embedded UI)
  updater/                  # Self-update (CLI only)
```

## Build Commands

```bash
# CLI only
go build -o build/vget ./cmd/vget

# Server (works on all platforms)
go build -o build/vget-server ./cmd/vget-server

# Both
go build ./cmd/...
```

## Binary Comparison

| Binary | Size | Contains |
|--------|------|----------|
| `vget` | ~28 MB | CLI commands, Bubbletea TUI, extractors, downloaders |
| `vget-server` | ~25 MB | HTTP server, embedded Web UI, extractors, downloaders |

The server binary is smaller because it doesn't include CLI components (Cobra commands, Bubbletea TUI).

## Docker

The Docker image uses `vget-server` directly:

```dockerfile
# Build
RUN go build -ldflags="-s -w" -o /vget-server ./cmd/vget-server

# Run
ENTRYPOINT ["entrypoint.sh"]  # Runs vget-server
```

## vget-server CLI

```bash
# Start server with defaults (port 8080)
vget-server

# Custom port
vget-server -port 9000

# Custom output directory
vget-server -output /path/to/downloads

# Show version
vget-server -version
```

Configuration is read from `~/.config/vget/config.yml` (same as CLI).

## Release Artifacts

| Platform | CLI | Server | Desktop |
|----------|-----|--------|---------|
| Linux amd64 | vget-linux-amd64 | vget-server-linux-amd64 | (future) |
| Linux arm64 | vget-linux-arm64 | vget-server-linux-arm64 | (future) |
| macOS amd64 | vget-darwin-amd64 | vget-server-darwin-amd64 | (future) |
| macOS arm64 | vget-darwin-arm64 | vget-server-darwin-arm64 | (future) |
| Windows | vget-windows-amd64.exe | vget-server-windows-amd64.exe | (future) |
| Docker | - | guiyumin/vget | - |

## Future: Desktop App

When implementing the desktop app:

1. Create `cmd/vget-desktop/main.go` using Fyne
2. Create `internal/desktop/` for Fyne UI components
3. Desktop will import from `internal/core/` (shared)
4. Desktop can use `internal/updater/` for self-update

```
cmd/
  vget-desktop/main.go      # Fyne entry point

internal/
  desktop/                  # Desktop-specific (Fyne UI)
```
