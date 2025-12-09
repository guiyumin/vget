# HTTP Server Mode (`vget serve`)

## Overview

HTTP server mode that accepts download requests via API, with an embedded WebUI for job monitoring.

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
- WebUI available at `http://localhost:8080/`
- API accepts URLs via HTTP POST

## WebUI

The server includes an embedded React SPA for job monitoring:

- Real-time job status updates (1s polling)
- Download form to submit URLs
- Progress bars for active downloads
- Cancel button for queued/downloading jobs
- Dark theme

Access at `http://localhost:8080/` when server is running.

## Configuration

**CLI Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `-p, --port` | 8080 | HTTP listen port |
| `-o, --output` | `./downloads` | Output directory |
| `-d, --daemon` | false | Run in background |

**Config file (`~/.config/vget/config.yml`):**
```yaml
server:
  port: 8080
  output_dir: /path/to/downloads
  max_concurrent: 3
  api_key: "optional-secret-key"
```

CLI flags override config values.

## API Reference

### Response Structure

All endpoints return a consistent JSON structure:
```json
{
  "code": 200,
  "data": { ... },
  "message": "description"
}
```

### Endpoints

#### `GET /health`
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

#### `POST /download`
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

#### `GET /status/:id`
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

#### `GET /jobs`
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

#### `DELETE /jobs/:id`
```json
{
  "code": 200,
  "data": {"id": "def456"},
  "message": "job cancelled"
}
```

### Authentication

Optional API key authentication via header `X-API-Key`. If `api_key` is set in config, all API requests must include it. The WebUI is accessible without authentication.

## Daemon Mode

```bash
vget serve -d              # Start daemon
vget serve stop            # Stop daemon
vget serve status          # Check if running
```

- PID stored in `~/.config/vget/serve.pid`
- Logs written to `~/.config/vget/serve.log`

## Development

### Running in Dev Mode

For UI development with hot reload:

**Terminal 1 - Go server (API on :8080):**
```bash
go run ./cmd/vget serve
```

**Terminal 2 - Vite dev server (UI on :5173):**
```bash
cd ui && npm run dev
```

Open `http://localhost:5173` - Vite proxies API calls to the Go server.

### Building

```bash
# Build UI and Go binary
make build

# Or manually:
cd ui && npm install && npm run build
cp -r ui/dist/* internal/server/dist/
go build -o build/vget ./cmd/vget
```

## Architecture

```
ui/                          # React SPA source
internal/server/
├── server.go                # HTTP server, handlers
├── job.go                   # Job queue, worker pool
├── embed.go                 # go:embed for UI
└── dist/                    # Built UI (embedded)
internal/cli/serve.go        # Cobra command
internal/config/config.go    # ServerConfig struct
```

### Internal Flow

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

## Usage Examples

**Start server:**
```bash
vget serve -p 9000 -o ~/Downloads/vget
vget serve -d  # Run in background
```

**Download via API:**
```bash
# Queue download
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

# Cancel job
curl -X DELETE http://localhost:8080/jobs/abc123
```

## Future Enhancements

- WebSocket for real-time progress updates
- Download scheduling (cron-like)
- Webhook notifications on completion
- Multi-user support with separate queues
