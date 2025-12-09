# HTTP Server Mode (`vget server`)

## Overview

HTTP server mode that accepts download requests via API, with an embedded WebUI for job monitoring.

**Commands:**

```bash
vget server start              # Foreground, port 8080
vget server start -d           # Background daemon, port 8080
vget server start -p 9000      # Custom port
vget server start -d -p 9000 -o ~/downloads
vget server stop               # Stop daemon
vget server restart            # Restart server
vget server status             # Check if running
vget server logs               # View recent logs
vget server logs -f            # Follow logs (tail -f)
```

- Listens on port 8080 by default (override with `-p`)
- `-d` runs as background daemon
- WebUI available at `http://localhost:8080/`
- API accepts URLs via HTTP POST
- Supports video, audio, and image downloads via extractors

## WebUI

The server includes an embedded React SPA for job monitoring:

- Real-time job status updates (polling)
- Download form to submit URLs
- Progress bars for active downloads
- Cancel button for queued/downloading jobs
- Configuration panel for output directory
- i18n support (zh, en, jp, kr, es, fr, de)
- Dark theme

Access at `http://localhost:8080/` when server is running.

## Configuration

**CLI Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `-p, --port` | 8080 | HTTP listen port |
| `-o, --output` | config default or `~/Downloads` | Output directory |
| `-d, --daemon` | false | Run in background |

**Config file (`~/.config/vget/config.yml`):**

```yaml
output_dir: ~/Downloads/vget

server:
  port: 8080
  max_concurrent: 10
  api_key: "optional-secret-key"
```

**Priority order for output directory:** CLI flag `-o` > `output_dir` in config > default (`~/Downloads/vget`)

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

#### `GET /config`

```json
{
  "code": 200,
  "data": {
    "output_dir": "/path/to/downloads"
  },
  "message": "config retrieved"
}
```

#### `PUT /config`

Update server configuration at runtime.

```json
// Request
{
  "output_dir": "/new/path/to/downloads"
}

// Response
{
  "code": 200,
  "data": {
    "output_dir": "/new/path/to/downloads"
  },
  "message": "config updated"
}
```

#### `GET /i18n`

Get UI translations for the configured language.

```json
{
  "code": 200,
  "data": {
    "language": "zh",
    "ui": { ... },
    "server": { ... },
    "config_exists": true
  },
  "message": "translations retrieved"
}
```

### Authentication

Optional API key authentication via header `X-API-Key`. If `api_key` is set in config, all API requests must include it. The WebUI and `/health` endpoint are accessible without authentication.

## Daemon Mode

```bash
vget server start -d       # Start daemon
vget server stop           # Stop daemon
vget server restart        # Restart daemon
vget server status         # Check if running
vget server logs -f        # Follow logs
```

- PID stored in `~/.config/vget/serve.pid`
- Logs written to `~/.config/vget/serve.log`

## Development

### Running in Dev Mode

For UI development with hot reload:

**Terminal 1 - Go server (API on :8080):**

```bash
go run ./cmd/vget server start
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
├── server.go                # HTTP server, handlers, download logic
├── job.go                   # Job queue, worker pool
├── embed.go                 # go:embed for UI
└── dist/                    # Built UI (embedded)
internal/cli/server.go       # Cobra commands, daemon management, service install
internal/config/config.go    # ServerConfig struct
```

### Internal Flow

```
HTTP Request
    ↓
Logging middleware
    ↓
Auth middleware (check X-API-Key if configured, skip for /health and UI)
    ↓
Route to handler
    ↓
POST /download → Add job to queue → Return job ID
    ↓
Worker pool (max_concurrent workers, default 10)
    ↓
Worker picks job → extractor.Match(url) → ext.Extract(url) → download with progress
    ↓
Update job status (queued → downloading → completed/failed/cancelled)
    ↓
Auto-cleanup completed/failed/cancelled jobs after 1 hour (runs every 10 minutes)
```

### Supported Media Types

The server uses the extractor system to handle different media:

- **Video** (Twitter, YouTube, etc.) - Selects best format (prefers with audio, then highest bitrate)
- **Audio** (podcasts, music)
- **Images** (downloads all images from multi-image posts)

For unsupported URLs, falls back to `sites.yml` config or generic browser extractor.

## Usage Examples

**Start server:**

```bash
vget server start -p 9000 -o ~/Downloads/vget
vget server start -d  # Run in background
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

# Get/update config
curl http://localhost:8080/config
curl -X PUT http://localhost:8080/config \
  -H "Content-Type: application/json" \
  -d '{"output_dir": "/new/path"}'
```

## Job Queue Details

- Job queue buffer size: 100 jobs
- Jobs have unique 16-character hex IDs
- Job statuses: `queued`, `downloading`, `completed`, `failed`, `cancelled`
- Progress tracking via callback during download
- Context-based cancellation support

---

## Future Enhancements

- WebSocket for real-time progress updates (currently uses polling)
- Webhook notifications on completion
- Multi-user support with separate queues
- Download scheduling (see below)

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

---

## Service Installation

One-command installation for NAS and Linux servers.

### Commands

```bash
sudo vget server install      # Install as systemd service (interactive)
sudo vget server install -y   # Install with defaults (non-interactive)
sudo vget server uninstall    # Remove service
vget server install --help    # Show options
```

### Interactive TUI Flow

When running `sudo vget server install`, a Bubbletea TUI guides the user:

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   vget service installer                                    │
│                                                             │
│   This will install vget as a system service:               │
│                                                             │
│   ✓ Copy binary to /usr/local/bin/vget                      │
│   ✓ Create systemd service at /etc/systemd/system/          │
│   ✓ Enable auto-start on boot                               │
│   ✓ Start the vget server                                   │
│                                                             │
│   Service configuration:                                    │
│   ┌─────────────────────────────────────────────────────┐   │
│   │  Port:        8080                                  │   │
│   │  Output dir:  /var/lib/vget/downloads               │   │
│   │  Run as user: vget                                  │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                             │
│   [ Configure ]    [ Install ]    [ Cancel ]                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Configuration Screen (if "Configure" selected)

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   Service Configuration                                     │
│                                                             │
│   Port:              8080                                   │
│   Output directory:  /var/lib/vget/downloads                │
│   Run as user:       vget  (will be created if needed)      │
│   API key:           (none)                                 │
│                                                             │
│   Use arrow keys to navigate, Enter to edit                 │
│                                                             │
│   [ Back ]    [ Save & Install ]                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### What `vget server install` Does

1. **Pre-flight checks**
   - Verify running as root (prompt for sudo if not)
   - Check if systemd is available
   - Check if service already exists (offer reinstall/update)

2. **Setup**
   - Create `vget` system user (if configured to run as non-root)
   - Create output directory with proper permissions
   - Copy binary to `/usr/local/bin/vget`

3. **Service installation**
   - Write service file to `/etc/systemd/system/vget.service`
   - Write config to `/etc/vget/config.yml`
   - Run `systemctl daemon-reload`
   - Run `systemctl enable vget`
   - Run `systemctl start vget`

4. **Success screen**
   ```
   ┌─────────────────────────────────────────────────────────────┐
   │                                                             │
   │   ✓ vget service installed successfully!                   │
   │                                                             │
   │   WebUI:    http://localhost:8080                           │
   │   Status:   sudo systemctl status vget                      │
   │   Logs:     sudo journalctl -u vget -f                      │
   │   Stop:     sudo systemctl stop vget                        │
   │   Remove:   sudo vget server uninstall                      │
   │                                                             │
   └─────────────────────────────────────────────────────────────┘
   ```

### Generated systemd Service File

```ini
# /etc/systemd/system/vget.service
[Unit]
Description=vget media downloader server
After=network.target

[Service]
Type=simple
User=vget
Group=vget
ExecStart=/usr/local/bin/vget server start --config /etc/vget/config.yml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### CLI Flags (non-interactive mode)

```bash
# Skip TUI, use defaults
sudo vget server install -y

# Custom configuration
sudo vget server install -p 9000 -o /data/downloads -u root

# Uninstall
sudo vget server uninstall
```

### Platform Support

Currently only Linux with systemd is supported.

On unsupported platforms, the command exits immediately with a helpful message:

```
$ vget server install

vget server install is only supported on Linux with systemd.

To run vget as a service on macOS, see:
https://github.com/guiyumin/vget/blob/main/docs/manual-service-setup.md

$ echo $?
0
```

No TUI is shown - just a clear message and clean exit.
