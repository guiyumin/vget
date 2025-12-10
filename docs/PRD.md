# vget – Product Requirement Document (PRD)

**Version:** 1.2
**Author:** Yumin
**Language:** Golang
**UI:** Bubble Tea (TUI)
**Purpose:** A modern, multi-source video downloader with elegant CLI & TUI.

---

## 1. Product Vision & Core Positioning

### One-Line Vision

**vget:** A modern, minimalist, high-speed video downloader that works like wget, with a beautiful Bubble Tea TUI. Starting with X/Twitter, expanding to more platforms.

### Core Philosophy

vget's core value is not "protocol-level innovation", but rather:

- **Ultimate user experience** - Simple CLI, beautiful TUI
- **Single binary distribution** - No Python/Node dependencies
- **Clean architecture** - Extensible extractor system
- **Modern developer experience** - Golang + Bubble Tea + Worker Pool

### Why Not Just Use yt-dlp?

| Aspect | yt-dlp | vget |
|--------|--------|------|
| Installation | Python + pip | Single binary |
| UI | CLI only | CLI + Bubble Tea TUI |
| Complexity | 500+ flags | Minimal, opinionated |
| Focus | 1000+ sites | Quality over quantity |

vget aims to be the "modern wget for videos" - simple, fast, beautiful.

---

## 2. Product Goals

### 2.1 MVP Goals (v0.1 - Twitter Focus)

**Target:** Working Twitter/X video downloader

- [x] Project structure setup
- [x] Twitter/X extractor (native Go, no yt-dlp dependency) ✅
  - Bearer token + guest token authentication
  - Tweet API parsing
  - Video variant extraction (multiple qualities)
- [ ] Direct MP4 downloader with progress bar
- [ ] HLS (.m3u8) support (Twitter uses this for some videos)
- [x] Simple CLI: `vget <twitter-url>` ✅
- [x] Auto-select best quality ✅
- [ ] Basic retry on failure

### 2.2 v0.2 Goals

- [x] Multi-threaded segmented downloads (range requests) ✅ **Implemented**
- [x] Output filename customization (`-o`) ✅ **Implemented**
- Proxy support (`--proxy`)

### 2.3 v0.3 Goals

- Bubble Tea TUI (`vget --ui`)
- More platform extractors (based on demand)
- Optional yt-dlp bridge for unsupported sites

---

## 3. User Experience (UX) Goals

### CLI Minimalism

```bash
vget https://example.com/video
```

### TUI Mode (Bubble Tea)

```bash
vget --ui URL
```

### Display Features

- Per-thread speed
- Total speed
- ETA
- Progress bar
- Task queue
- Pause/Resume capability
- Download history

### Automatic Content Type Detection

```
URL → Extractor → (MP4 / HLS / DASH / Playlist)
```

**Fully automatic:** Users don't need to think about the underlying protocol.

---

## 4. Feature Specification

### 4.1 Downloader Engine (Core)

| Feature | Description | Status |
|---------|-------------|--------|
| Multi-Stream Download | HTTP Range requests with parallel streams (default 8) | ✅ Implemented |
| Concurrent Download | goroutine + worker pool pattern | ✅ Implemented |
| Chunk-based Transfer | 16MB chunks with 128KB buffers per stream | ✅ Implemented |
| Progress Display | Real-time speed, ETA, elapsed time, avg speed | ✅ Implemented |
| Auto Retry | Exponential backoff retry (5 retries per chunk) | ✅ Implemented |
| File Merge | Merge multiple segments into MP4 | Planned |
| Verification | Support md5/sha256 (optional) | Planned |
| Speed Limit | Throttle mode (optional) | Planned |
| Download Queue | Multiple simultaneous tasks | Planned |

#### Multi-Stream Download Architecture (Implemented)

```
┌─────────────────────────────────────────────────────────────┐
│                    MultiStreamConfig                         │
│  Streams: 8 (parallel connections)                          │
│  ChunkSize: 16MB (per chunk)                                │
│  BufferSize: 128KB (per stream read buffer)                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 HEAD Request (Check Support)                 │
│  - Get Content-Length                                       │
│  - Check Accept-Ranges: bytes                               │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
    [Range Supported]                [Range Not Supported]
              │                               │
              ▼                               ▼
┌─────────────────────────┐       ┌─────────────────────────┐
│   Calculate Chunks      │       │  Single-Stream Fallback │
│   File ÷ ChunkSize      │       │  (128KB buffer)         │
└─────────────────────────┘       └─────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Worker Pool (8 workers)                   │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐   │
│  │ W1  │ │ W2  │ │ W3  │ │ W4  │ │ W5  │ │ W6  │ │ W7  │...│
│  └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘   │
│     │       │       │       │       │       │       │       │
│     ▼       ▼       ▼       ▼       ▼       ▼       ▼       │
│  Range:  Range:  Range:  Range:  Range:  Range:  Range:     │
│  0-16M   16M-32M 32M-48M ...                                │
└─────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│              file.WriteAt(data, offset)                      │
│              (Thread-safe positional writes)                 │
└─────────────────────────────────────────────────────────────┘
```

**Performance Comparison:**

| Metric | Before (Single Stream) | After (Multi-Stream) |
|--------|----------------------|---------------------|
| Streams | 1 | 8 (configurable) |
| Buffer | 32KB | 128KB per stream |
| Typical Speed | ~10-20 MB/s | ~50-80 MB/s |
| WebDAV Support | Basic | Full with Range requests |

### 4.2 Extractor Layer (URL Parsing)

#### Extractor Interface

```go
type Extractor interface {
    // Match returns true if this extractor can handle the URL
    Match(url string) bool
    // Extract returns video info (title, formats, etc.)
    Extract(url string) (*VideoInfo, error)
}

type VideoInfo struct {
    ID       string
    Title    string
    Formats  []Format  // Multiple qualities available
    Duration int
}

type Format struct {
    URL      string
    Quality  string    // "1080p", "720p", etc.
    Ext      string    // "mp4", "m3u8"
    Width    int
    Height   int
    Bitrate  int
}
```

#### Supported Extractors

| Extractor | Status | Notes |
|-----------|--------|-------|
| Twitter/X | MVP | Native Go implementation |
| Direct MP4 | MVP | Content-Type detection |
| HLS | MVP | m3u8 parsing |
| DASH | v0.2 | mpd XML parsing |
| yt-dlp bridge | v0.3 | Optional fallback |

#### Twitter/X Extractor Details

```
URL: https://x.com/user/status/123456789
           ↓
    Extract tweet ID
           ↓
    Get guest token (POST /1.1/guest/activate.json)
           ↓
    Fetch tweet (GET /1.1/statuses/show/{id}.json)
           ↓
    Parse extended_entities.media[].video_info.variants
           ↓
    Return VideoInfo with all quality options
```

### 4.3 CLI Specification

```bash
# Basic download
vget <url>

# Specify quality
vget -q 1080p <url>

# Segment thread count
vget -t 32 <url>

# Output filename
vget -o out.mp4 <url>

# Proxy
vget --proxy socks5://127.0.0.1:1080 <url>

# Cookie
vget --cookies cookies.txt <url>

# Custom headers
vget -H "Referer: https://xxx" <url>

# Parse only, don't download
vget --info <url>
```

### 4.4 TUI (Bubble Tea) Design

#### Components

- Header (speed, ETA)
- Global progress bar
- Per-thread speed bars
- Error messages
- Undo/Pause/Resume controls
- Log window
- Task queue

#### Keyboard Shortcuts

| Key | Function |
|-----|----------|
| `space` | Pause/Resume |
| `p` | Pause |
| `r` | Retry |
| `q` | Quit |
| `↑↓` | Switch tasks |

#### TUI Aesthetic

- lipgloss + Nord theme
- Clean and minimalist
- Style similar to glow, gh-dash, gum

---

## 5. Architecture Design

```
/cmd/vget
    main.go              # Entry point, CLI parsing
/internal
    /cli
        root.go          # Main command & WebDAV download handler
        config.go        # Config management commands
        extract.go       # Extraction with spinner
        ls.go            # Directory listing command
        search.go        # Search command
        completion.go    # Shell completion
    /extractor
        extractor.go     # Extractor interface & media types
        twitter.go       # Twitter/X extractor
        xiaoyuzhou.go    # Xiaoyuzhou podcast extractor
        instagram.go     # Instagram extractor
        tiktok.go        # TikTok extractor
        xiaohongshu.go   # Xiaohongshu extractor
        registry.go      # Extractor registration & matching
    /downloader
        downloader.go    # Download interface
        progress.go      # Progress tracking & Bubble Tea TUI
        multistream.go   # Multi-stream parallel downloader ✅ NEW
        utils.go         # Helper functions
    /webdav
        client.go        # WebDAV client with Range request support ✅ NEW
    /config
        config.go        # User configuration & WebDAV servers
                         # IMPORTANT: Config is read fresh per-command (no restart needed)
    /i18n
        i18n.go          # Internationalization
        /locales/*.yml   # Translation files (en, zh, jp, kr, es, fr, de)
    /updater
        updater.go       # Self-update functionality
    /version
        version.go       # Version info
```

---

## 6. Technical Implementation Details

### 6.1 Extractor Logic

**Pseudocode:**

```
if url endsWith .mp4 → MP4Extractor
if content-type == application/vnd.apple.mpegurl → HLSExtractor
if content-type == application/dash+xml → DASHExtractor
if URL contains "playlist" → PlaylistExtractor
```

#### HLS Flow

1. Download m3u8
2. Find master playlist
3. Select highest bitrate
4. Parse TS segments
5. Build task list in order

#### DASH Flow

1. Download mpd XML
2. Extract mediaBaseURL + segmentTemplate
3. Select a Representation
4. Generate task list for all segments

### 6.2 Downloader Engine (Implemented)

**Multi-Stream Configuration:**

```go
type MultiStreamConfig struct {
    Streams    int   // Number of parallel streams (default 8)
    ChunkSize  int64 // Size of each chunk (default 16MB)
    BufferSize int   // Buffer size per stream (default 128KB)
}
```

**Worker Pool Pattern:**

```go
// Create chunk channel and feed all chunks
chunkChan := make(chan chunk, len(chunks))
for _, c := range chunks {
    chunkChan <- c
}
close(chunkChan)

// Start N worker goroutines
for i := 0; i < config.Streams; i++ {
    go func() {
        for c := range chunkChan {
            downloadChunk(ctx, client, url, file, c, state)
        }
    }()
}
```

**Chunk Download with Range Requests:**

```go
req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.start, chunk.end))
// ...
file.WriteAt(data, offset)  // Thread-safe positional write
```

**Progress Tracking:**

```go
type downloadState struct {
    current     int64       // Atomic counter across all streams
    total       int64
    speed       float64     // Real-time speed
    startTime   time.Time
    endTime     time.Time
    finalSpeed  float64     // Average speed at completion
}
```

### 6.3 WebDAV Support (Implemented)

**Features:**

- Remote path syntax: `vget pikpak:/path/to/file.mp4`
- Full URL syntax: `vget webdav://user:pass@host/path`
- Multi-stream parallel downloads with HTTP Range requests
- Automatic fallback to single-stream if Range not supported
- Directory listing: `vget ls pikpak:/movies`

**WebDAV Client Architecture:**

```go
type Client struct {
    client   *webdav.Client  // go-webdav for PROPFIND/etc
    baseURL  string
    username string
    password string
}

// Methods
func (c *Client) Stat(ctx, path) (*FileInfo, error)
func (c *Client) List(ctx, path) ([]FileInfo, error)
func (c *Client) Open(ctx, path) (io.ReadCloser, int64, error)
func (c *Client) GetFileURL(path) string      // For Range requests
func (c *Client) GetAuthHeader() string       // Basic Auth header
func (c *Client) SupportsRangeRequests(ctx, path) (bool, error)
```

**Download Flow:**

```
pikpak:/movies/video.mp4
        │
        ▼
┌─────────────────────┐
│  Load config.yml    │
│  Get server creds   │
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  client.Stat()      │
│  Get file size      │
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  HEAD request       │
│  Check Range support│
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  Multi-stream DL    │
│  8 parallel streams │
└─────────────────────┘
```

### 6.4 Merge (mp4 / ts / m4s)

**HLS:**

```bash
cat part*.ts | ffmpeg -i - -c copy out.mp4
```

**DASH:**

- mp4box or pure Go mux (can be supported after v1)

---

## 7. Future Roadmap

### TODO

- **Optimize download speed for WebDAV/PikPak** - Current multi-stream implementation is significantly slower than rclone. Target: 30MB/s for PikPak. Investigate:
  - Connection reuse / keep-alive
  - Chunk size tuning
  - Number of parallel streams
  - Buffer sizes
  - TCP tuning

### v1 (MVP)

- MP4 / HLS / DASH download
- CLI
- TUI
- Multi-threaded segmentation
- Resume support
- Auto quality detection

### v1.5

- Multi-task queue
- History records
- Graceful pause/resume
- Auto proxy detection

### v2

- Plugin system (extractor plugins)
- `.vget/plugins/*.wasm` for custom site loaders

### v3

- Distributed downloading
- Integration with S3 / OSS / R2
- Become a true "media download platform"

---

## 8. Success Metrics

| Metric | Target |
|--------|--------|
| GitHub Stars | 1,000 (first month) / 5,000 (6 months) |
| CLI Installs | 5K+ |
| TUI Open Rate | > 40% |
| Issue Feedback | > 20 (community engagement) |
| Pull Requests | At least 5 external contributors |

---

## 9. Top Selling Points (Highlight in README)

- **Modern video downloader**
- **Fast, concurrent, resumable**
- **HLS & DASH built-in**
- **Beautiful Bubble Tea TUI**
- **Cross-platform single binary**
- **Plugin ecosystem (future)**

---

## 10. README Sample

```
vget
----
A modern, blazing-fast video downloader for the command line.
Supports MP4, HLS (m3u8), DASH (mpd), multi-thread downloads,
resume, cookies, proxies, and a beautiful Bubble Tea-powered TUI.

Usage:
  vget <url>            # auto detect and download
  vget --ui <url>       # open interactive TUI
  vget -t 32 <url>      # 32-thread segmented download
  vget -q 1080p <url>   # choose quality (HLS/DASH)
  vget --cookies c.txt  # cookie support
```
