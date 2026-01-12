# TODO

## Tomorrow's Tasks

3. [x] kuaidi100 - Bring Your Own Key (API is expensive)

## Features

- [x] `vget init` command
  - Language preference
  - Default output directory
  - Default format/quality
- [x] Self update
- [x] m3u8 streaming support
- [x] Bulk download from txt file
  - Read URLs from txt file
  - Sequential or parallel processing
- [x] Format/quality selection (`-q` flag)
- [x] Audio extraction (podcasts)
- [ ] Resume interrupted downloads
- [ ] Retry on failure
- [x] Progress bar with speed/ETA
- [ ] Quiet/verbose modes
- [ ] Dry run mode
- [ ] More extractors (YouTube, TikTok, etc.)
- [ ] Playlist support
- [x] Concurrent downloads
- [ ] Rate limiting
- [x] Cookie/auth support
- [ ] Metadata embedding
- [x] `vget server` - HTTP server mode
  - REST API for remote downloads
  - Run as background daemon (`vget server start -d`)
  - Web UI for submitting URLs
  - systemd service installation (`vget server install`)
- [x] WebDAV client integration
  - Connect to PikPak, other WebDAV-compatible cloud storage
  - Download files from cloud (`vget <remote>:<path>`)
  - Browse and select files with TUI (`vget ls <remote>:<path>`)

## Extractors

- [x] Twitter/X
- [x] Xiaoyuzhou (小宇宙) podcasts
  - [x] Episode download
  - [x] Search (`vget search --podcast <query>`)
  - [x] Podcast listing (all episodes)
- [x] YouTube (Docker only, uses yt-dlp/youtube-dl)
- [ ] TikTok
- [x] Apple Podcasts
- [x] Xiaohongshu (小红书/RED)
  - Requires browser automation (Rod) + cookie auth
  - Reference: [xpzouying/xiaohongshu-mcp](https://github.com/xpzouying/xiaohongshu-mcp) (7.2k stars, stable 1+ year)
  - Extraction approach:
    - Navigate to `https://www.xiaohongshu.com/explore/{feedID}?xsec_token=...`
    - Extract `window.__INITIAL_STATE__.note.noteDetailMap` via JS
    - Parse JSON for images (`urlDefault`) and video URLs
  - Feasibility: Moderate effort, more achievable than Instagram
  - Note: yt-dlp also has extractor but frequently breaks due to bot detection

## Tracking (Versatile Get)

- [ ] FedEx tracking
  - [ ] Scraping (default, no setup)
  - [ ] API mode (user provides own keys in config.yml)
- [ ] UPS tracking
  - [ ] Scraping (default, no setup)
  - [ ] API mode (user provides own keys in config.yml)
- [ ] USPS tracking
  - [ ] Scraping (default, no setup)
  - [ ] API mode (user provides own keys in config.yml)
  - [ ] kuaidi100 - Bring Your Own Key (API is expensive)

## DevOps

- [x] GoReleaser + GitHub Actions for tagged releases
- [x] Dockerfile for NAS deployment
  - Multi-stage build for minimal image
  - Support for Synology/QNAP/TrueNAS
  - compose.yml with NAS path examples
