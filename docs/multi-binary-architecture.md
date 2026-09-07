# Multi-Binary Architecture

## Overview

vget is split into separate binaries with a shared core module:

| Binary | Purpose | Distribution |
|--------|---------|--------------|
| `vget` | CLI tool | GitHub Releases (all platforms) |
| `vget-server` | HTTP server + Web UI | GitHub Releases + Docker Image |

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

| Binary | Size (stripped, `-s -w`) | Contains |
|--------|------|----------|
| `vget` | ~28 MB (was ~19 MB before tailcat) | CLI commands, Bubbletea TUI, extractors, downloaders, library server, tailcat transport |
| `vget-server` | ~25 MB | HTTP server, embedded Web UI, extractors, downloaders |

Measured 2026-09 on darwin/arm64. The tailcat transport (`tailscale.com` data
plane: WireGuard, netstack, DERP) adds about 12 MB, or 9 MB when built with
tailcat's `ts_omit_*` tag list as the release build does (see `.goreleaser.yml`).
`tailscale.com` is imported only from `internal/core/tailcat`; both binaries link
it because the shared WebDAV client dials tailcat remotes through it.

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

| Platform | CLI | Server |
|----------|-----|--------|
| Linux amd64 | vget-linux-amd64 | vget-server-linux-amd64 |
| Linux arm64 | vget-linux-arm64 | vget-server-linux-arm64 |
| macOS amd64 | vget-darwin-amd64 | vget-server-darwin-amd64 |
| macOS arm64 | vget-darwin-arm64 | vget-server-darwin-arm64 |
| Windows | vget-windows-amd64.exe | vget-server-windows-amd64.exe |
| Docker | - | guiyumin/vget |
