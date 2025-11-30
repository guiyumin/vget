# vget – Product Requirement Document (PRD)

**Version:** 1.0
**Author:** Yumin
**Language:** Golang
**UI:** Bubble Tea (TUI)
**Purpose:** A modern, multi-source video downloader with elegant CLI & TUI.

---

## 1. Product Vision & Core Positioning

### One-Line Vision

**vget:** A modern, multi-source, minimalist, high-speed video downloader that works like wget, functions like yt-dlp, and features a beautiful Bubble Tea TUI.

### Core Philosophy

vget's core value is not "protocol-level innovation", but rather:

- **Ultimate user experience**
- **Out-of-the-box multi-source capabilities**
- **Clean architecture**
- **Plugin extension ecosystem**
- **Modern CLI & TUI that developers love**
- **Experience-first approach with Golang + Bubble Tea + Worker Pool**

---

## 2. Product Goals

### 2.1 MVP Goals (Achievable within 30 days)

- Support MP4 direct link downloads
- Support HLS (.m3u8)
- Support DASH (.mpd)
- Support resume/checkpoint recovery
- Multi-threaded segmented downloads (range requests)
- Simple CLI: `vget URL`
- Optional: `vget --ui URL` to launch Bubble Tea TUI
- Automatic best quality selection
- Automatic retry and recovery
- Cookie/header support
- Modern progress experience (TUI + CLI progress bars)

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

| Feature | Description |
|---------|-------------|
| Segmented Download | Range Requests, default 16 segments, configurable |
| Concurrent Download | goroutine + worker pool |
| Auto Retry | Exponential backoff retry |
| Resume Support | `.vget-meta.json` tracking |
| File Merge | Merge multiple segments into MP4 |
| Verification | Support md5/sha256 (optional) |
| Speed Limit | Throttle mode (optional) |
| Download Queue | Multiple simultaneous tasks |
| Parallel vs Serial | User selectable |

### 4.2 Extractor Layer (URL Parsing)

vget's soul is the abstracted extractor system:

| Protocol | Support |
|----------|---------|
| MP4 | Direct GET |
| HLS | m3u8 parsing, quality selection |
| DASH | mpd XML parsing |
| Playlist | Auto merge to queue |
| Cookie / UA | User-provided cookies.txt |
| Header Override | CLI parameter specification |

Future extensibility: Plugin system similar to yt-dlp extractors (without built-in sensitive site support).

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
    main.go
/internal
    /cli
        parser.go
    /extractor
        mp4.go
        hls.go
        dash.go
        util.go
    /downloader
        manager.go
        worker.go
        merge.go
        retry.go
    /tui
        app.go
        model.go
        view.go
    /utils
        file.go
        http.go
        log.go
/plugins
    example_plugin.go
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

### 6.2 Downloader Engine

**Worker Pool:**

```
workerCount = userThreads or default (16)
for each segment:
    assign to worker
worker → download(segment)
```

**Segmentation Strategy:**

- `Range: bytes=start-end`
- Download to `.tmp/part-N`
- Merge after all complete

### 6.3 Merge (mp4 / ts / m4s)

**HLS:**

```bash
cat part*.ts | ffmpeg -i - -c copy out.mp4
```

**DASH:**

- mp4box or pure Go mux (can be supported after v1)

---

## 7. Future Roadmap

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
