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
      { "id": "abc123", "url": "...", "status": "completed" },
      {
        "id": "def456",
        "url": "...",
        "status": "downloading",
        "progress": 67.2
      }
    ]
  },
  "message": "2 jobs found"
}
```

#### `DELETE /jobs/:id`

```json
{
  "code": 200,
  "data": { "id": "def456" },
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
- Webhook notifications on completion
- Multi-user support with separate queues

---

## Download Scheduling (Planned)

Schedule downloads to run at specific times or on recurring intervals.

### Features

**One-time scheduled downloads:**

- Schedule a download to start at a specific datetime
- Use case: Queue large downloads for off-peak hours

**Recurring downloads (cron-style):**

- Standard cron expressions for repeat scheduling
- Use case: Automatically fetch new podcast episodes, YouTube channel updates

**Time-window restrictions:**

- Limit downloads to specific time windows
- Use case: Bandwidth management, only download during night hours

### API Endpoints

#### `POST /api/v1/schedules`

Create a new schedule.

```json
// Request
{
  "url": "https://example.com/video",
  "schedule": "0 2 * * *",          // cron expression (2 AM daily)
  "name": "Daily backup video",      // optional, human-readable name
  "enabled": true,
  "options": {
    "format": "best",
    "output_dir": "/downloads/scheduled"
  }
}

// Response
{
  "code": 200,
  "data": {
    "id": "sch_abc123",
    "url": "https://example.com/video",
    "schedule": "0 2 * * *",
    "next_run": "2025-01-15T02:00:00Z",
    "enabled": true
  },
  "message": "schedule created"
}
```

**Cron expression format:** `minute hour day month weekday`
| Expression | Description |
|------------|-------------|
| `0 2 * * *` | Every day at 2:00 AM |
| `0 */6 * * *` | Every 6 hours |
| `0 8 * * 1` | Every Monday at 8:00 AM |
| `30 22 * * 5` | Every Friday at 10:30 PM |

**One-time schedule:** Use `run_at` instead of `schedule`:

```json
{
  "url": "https://example.com/large-file",
  "run_at": "2025-01-15T03:00:00Z"
}
```

#### `GET /api/v1/schedules`

List all schedules.

```json
{
  "code": 200,
  "data": {
    "schedules": [
      {
        "id": "sch_abc123",
        "name": "Daily podcast",
        "url": "https://...",
        "schedule": "0 6 * * *",
        "next_run": "2025-01-15T06:00:00Z",
        "last_run": "2025-01-14T06:00:00Z",
        "last_status": "completed",
        "enabled": true
      }
    ]
  },
  "message": "1 schedule found"
}
```

#### `GET /api/v1/schedules/:id`

Get schedule details and history.

```json
{
  "code": 200,
  "data": {
    "id": "sch_abc123",
    "name": "Daily podcast",
    "url": "https://...",
    "schedule": "0 6 * * *",
    "enabled": true,
    "next_run": "2025-01-15T06:00:00Z",
    "history": [
      {
        "run_at": "2025-01-14T06:00:00Z",
        "status": "completed",
        "job_id": "job_xyz"
      },
      {
        "run_at": "2025-01-13T06:00:00Z",
        "status": "completed",
        "job_id": "job_abc"
      }
    ]
  },
  "message": "schedule found"
}
```

#### `PUT /api/v1/schedules/:id`

Update a schedule.

```json
// Request
{
  "schedule": "0 3 * * *",
  "enabled": false
}

// Response
{
  "code": 200,
  "data": {"id": "sch_abc123"},
  "message": "schedule updated"
}
```

#### `DELETE /api/v1/schedules/:id`

Delete a schedule.

```json
{
  "code": 200,
  "data": { "id": "sch_abc123" },
  "message": "schedule deleted"
}
```

#### `POST /api/v1/schedules/:id/run`

Trigger a scheduled download immediately (outside of schedule).

```json
{
  "code": 200,
  "data": {
    "id": "sch_abc123",
    "job_id": "job_xyz789"
  },
  "message": "schedule triggered"
}
```

### Configuration

```yaml
# ~/.config/vget/config.yml
server:
  scheduling:
    enabled: true
    max_schedules: 50 # max number of schedules
    history_retention: 30 # days to keep run history
    time_window: # optional global restriction
      start: "01:00" # downloads only between 1 AM
      end: "06:00" # and 6 AM
```

### Persistence

- Schedules stored in `~/.config/vget/schedules.json`
- Survives server restarts
- Run history kept for configured retention period

### WebUI Integration

- New "Schedules" tab in the dashboard
- Create/edit/delete schedules via UI
- View upcoming runs and execution history
- Toggle schedules on/off

### Implementation Notes

- Uses `robfig/cron/v3` library for cron parsing and scheduling
- Schedules are evaluated on server startup and when modified
- Scheduled jobs enter the same job queue as manual downloads
- If server is stopped during scheduled time, missed runs are skipped (no catch-up)
