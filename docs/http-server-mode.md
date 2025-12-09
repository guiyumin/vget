# HTTP Server Mode (`vget serve`)

## Overview

Add an HTTP server mode to vget that accepts download requests via HTTP API and runs as a background daemon.

**Command:** `vget serve [-p port] [-o output_dir] [-d]`

**Examples:**
```bash
vget serve              # Foreground, port 8080
vget serve -d           # Background daemon, port 8080
vget serve -p 9000      # Custom port
vget serve -d -p 9000 -o ~/downloads
```

- Listens on port 8080 by default (override with `-p`)
- `-d` runs as background daemon
- Accepts URLs via HTTP POST, downloads to designated directory or returns file

## Architecture Assessment

**Refactoring Required: Minimal**

The current codebase is modular and well-suited for this feature:
- Extraction logic is decoupled from CLI
- Download functions can run without TUI
- Config system is extensible
- Existing patterns (WebDAV, Telegram, batch) show precedent

**Reusable Components (95% of existing code):**
- `internal/extractor/*` - All extractors work as-is
- `internal/downloader/*` - Download logic (skip TUI)
- `internal/config/*` - Config loading
- `internal/cli/root.go` - `runDownload()` logic

## Implementation Plan

**Command:**
```
vget serve [-p port] [-o output_dir] [-d]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `-p, --port` | 8080 | HTTP listen port |
| `-o, --output` | `./downloads` | Output directory |
| `-d, --daemon` | false | Run in background |

**Config additions to `~/.config/vget/config.yml`:**
```yaml
server:
  port: 8080
  output_dir: /path/to/downloads
  max_concurrent: 3
  api_key: "optional-secret-key"
```

Note: CLI flags override config values.

### Response Structure

All endpoints return a consistent JSON structure:
```go
type Response[T any] struct {
    Code    int    `json:"code"`
    Data    T      `json:"data"`
    Message string `json:"message"`
}
```

### Endpoints

1. `GET /health`
   ```json
   {
     "code": 200,
     "data": {
       "status": "ok",
       "version": "v0.7.1"
     },
     "message": "everything is good"
   }
   ```

2. `POST /download`
   ```json
   // Request
   {
     "url": "https://twitter.com/...",
     "filename": "optional.mp4",
     "return_file": false
   }

   // Response (return_file=false)
   {
     "code": 200,
     "data": {
       "id": "abc123",
       "status": "queued"
     },
     "message": "download started"
   }

   // Response (return_file=true)
   // Returns file directly with Content-Disposition header
   ```

3. `GET /status/:id`
   ```json
   {
     "code": 200,
     "data": {
       "id": "abc123",
       "status": "downloading",
       "progress": 45.5,
       "filename": "video.mp4"
     },
     "message": "downloading"
   }
   ```

   ```json
   // Error case
   {
     "code": 404,
     "data": null,
     "message": "job not found"
   }
   ```

4. `GET /jobs`
   ```json
   {
     "code": 200,
     "data": {
       "jobs": [
         {"id": "abc123", "url": "...", "status": "completed"},
         {"id": "def456", "url": "...", "status": "downloading", "progress": 67.2}
       ]
     },
     "message": "2 jobs found"
   }
   ```

5. `DELETE /jobs/:id`
   ```json
   {
     "code": 200,
     "data": {"id": "def456"},
     "message": "job cancelled"
   }
   ```

### Authentication

Optional API key authentication via header `X-API-Key`. If `api_key` is set in config, all requests must include it.

### Internal Architecture

```
HTTP Request
    ↓
Auth middleware (check X-API-Key if configured)
    ↓
Route to handler
    ↓
POST /download → Add job to queue → Return job ID
    ↓
Worker pool (max_concurrent workers)
    ↓
Worker picks job → extractor.Match(url) → ext.Extract(url) → download
    ↓
Update job status (queued → downloading → completed/failed)
    ↓
Auto-cleanup completed jobs after 1 hour
```

## File Changes Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/server/server.go` | New | HTTP server, handlers, response helpers |
| `internal/server/job.go` | New | Job struct, queue, worker pool |
| `internal/cli/serve.go` | New | Cobra command, daemon management |
| `internal/config/config.go` | Minor | Add `Server` config struct |

**Estimated total:** ~500-700 lines

## Daemon Mode (`-d` flag)

**Implementation Options:**

1. **Simple Background (Recommended for MVP):**
   - Fork process, redirect stdout/stderr to log file
   - Store PID in `~/.config/vget/serve.pid`
   - `vget serve stop` to kill daemon

2. **Systemd Integration (Future):**
   - Generate systemd unit file
   - `vget serve install` to install as service

**Daemon Management:**
```bash
vget serve -d              # Start daemon
vget serve stop            # Stop daemon
vget serve status          # Check if running
vget serve logs            # Tail log file
```

## Technical Details

### Download Without TUI

Current download functions use Bubbletea TUI. For server mode:

```go
// Option 1: Add flag to skip TUI
func (d *Downloader) Download(url, output, id string, skipTUI bool) error

// Option 2: New method (cleaner)
func (d *Downloader) DownloadBackground(url, output string,
    progressFn func(downloaded, total int64)) error
```

The underlying `downloadWithProgress()` in `progress.go` already handles the HTTP download - just need to bypass `tea.NewProgram()`.

### Concurrency Model

```go
type Server struct {
    jobs     map[string]*Job
    jobsMu   sync.RWMutex
    queue    chan *Job
    workers  int
    outputDir string
}

func (s *Server) Start() {
    // Start worker pool
    for i := 0; i < s.workers; i++ {
        go s.worker()
    }
    // Start HTTP server
    http.ListenAndServe(...)
}

func (s *Server) worker() {
    for job := range s.queue {
        s.processJob(job)
    }
}
```

## Usage Examples

**Start server:**
```bash
vget serve -p 9000 -o ~/Downloads/vget
vget serve -d  # Run in background
```

**Download via API:**
```bash
# Queue download (saves to server's output directory)
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"url": "https://twitter.com/user/status/123"}'

# Download and return file directly
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"url": "https://...", "return_file": true}' \
  -o video.mp4

# Check status
curl http://localhost:8080/status/abc123

# List all jobs
curl http://localhost:8080/jobs
```

## Dependencies

No new dependencies required. Uses standard library:
- `net/http` - HTTP server
- `encoding/json` - JSON handling
- `os/exec` - Daemon fork (for `-d` flag)

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Browser automation blocks server | Run Rod extractions in goroutine pool |
| Memory leak from job history | Auto-cleanup after 1 hour |
| No auth by default | Add warning on startup, recommend API key |
| Port conflict | Clear error message, suggest `-p` flag |

## Testing Plan

1. Unit tests for queue/job management
2. Integration tests for API endpoints
3. Manual testing with various extractors
4. Daemon start/stop lifecycle testing

## Future Enhancements

- WebUI for job monitoring
- WebSocket for real-time progress updates
- Download scheduling (cron-like)
- Webhook notifications on completion
- Multi-user support with separate queues
