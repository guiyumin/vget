# Torrent Dispatch Feature

## Overview

Allow vget to dispatch magnet links and .torrent files to remote torrent clients running on NAS devices or servers. vget does NOT download torrents itself - it only manages/dispatches jobs to existing torrent clients.

## Motivation

Chinese users requested BitTorrent support. Many use private trackers (PT sites) and have NAS devices (Synology, QNAP, etc.) running 24/7 with torrent clients. They want to quickly send magnets to their NAS without opening the web UI.

## Scope

### In Scope
- Send magnet links to remote torrent client
- Send .torrent file URLs to remote torrent client
- List active torrents (optional, for status checking)
- Support multiple torrent clients: Transmission, qBittorrent, Synology Download Station

### Out of Scope
- Actually downloading torrents (use existing clients)
- Torrent search/discovery
- Auto-detection of NAS devices (unreliable)
- Torrent client running inside vget Docker container

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  vget Docker (Web UI)                                   │
│                                                         │
│    Browser  →  POST /api/torrent  →  Torrent Library    │
│                                                         │
└─────────────────────┬───────────────────────────────────┘
                      │ HTTP/RPC
                      ▼
┌─────────────────────────────────────────────────────────┐
│  Remote Torrent Client (on NAS or server)               │
│  - Transmission (RPC on port 9091)                      │
│  - qBittorrent (Web API on port 8080)                   │
│  - Synology Download Station (API on port 5000/5001)    │
└─────────────────────────────────────────────────────────┘
```

## Implementation Status

### Completed
- [x] `internal/torrent/client.go` - Interface definition
- [x] `internal/torrent/transmission.go` - Transmission RPC client
- [x] `internal/torrent/qbittorrent.go` - qBittorrent Web API client
- [x] `internal/torrent/synology.go` - Synology Download Station client
- [x] `internal/core/config/config.go` - TorrentConfig struct added
- [x] `internal/server/server.go` - API endpoints added
- [x] `ui/src/routes/torrent.tsx` - Route file
- [x] `ui/src/pages/TorrentPage.tsx` - Page component
- [x] `ui/src/components/Torrent.tsx` - BT/Magnet submit component
- [x] `ui/src/components/TorrentSettings.tsx` - Settings component
- [x] `ui/src/components/Sidebar.tsx` - Menu item added
- [x] `ui/src/utils/translations.ts` - Translation strings added
- [x] `ui/src/utils/apis.ts` - API functions added
- [x] `ui/src/context/AppContext.tsx` - torrentEnabled state added

### TODO (Future)

- [x] Add backend i18n translations (internal/i18n/locales/*.yml)
- [ ] CLI support if users request it (vget bittorrent / bt / magnet / cili)

## Configuration

### Config File (~/.config/vget/config.yml)
```yaml
torrent:
  enabled: true
  client: transmission
  host: "192.168.1.100:9091"
  username: "admin"
  password: "secret"
```

### NOT in `vget init`
Torrent config is optional and should NOT be part of the initial setup wizard. Users configure it through the Web UI settings page.

## Supported Clients

| Client | Default Port | Protocol | Notes |
|--------|-------------|----------|-------|
| Transmission | 9091 | JSON-RPC | Most common on Linux NAS |
| qBittorrent | 8080 | REST API | Popular alternative |
| Synology DS | 5000/5001 | REST API | Built into Synology NAS |

## Testing

### Local Testing (without NAS)
```bash
# Run Transmission in Docker
docker run -d --name transmission \
  -p 9091:9091 \
  -e USER=admin \
  -e PASS=admin \
  linuxserver/transmission

# Run qBittorrent in Docker
docker run -d --name qbittorrent \
  -p 8080:8080 \
  linuxserver/qbittorrent
```

### Test Magnets
Use legal test torrents:
- Ubuntu ISO: `magnet:?xt=urn:btih:...` (search for current release)
- Blender Open Movies

## Security Considerations

- Torrent client credentials stored in config file (same as other credentials)
- HTTPS support for remote connections
- No auto-discovery to avoid network scanning concerns

## Future Enhancements (Not Planned)
- Deluge support
- Aria2 support (already has remote RPC)
- QNAP Download Station
- Torrent notifications via Telegram
