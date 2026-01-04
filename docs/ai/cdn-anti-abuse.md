# CDN Anti-Abuse System for vmirror Downloads

This document describes the authentication and anti-abuse system for downloading AI models from vmirror.org CDN.

## Overview

vmirror.org is a CDN mirror for AI models, primarily serving users in China who cannot access HuggingFace or GitHub directly. To prevent abuse and bandwidth theft, we implement a lightweight authentication system.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           FIRST-TIME DOWNLOAD                                │
│                                                                              │
│  ┌─────────┐         ┌──────────────────┐         ┌─────────────────────┐   │
│  │  vget   │         │  vget-io API     │         │  PostgreSQL         │   │
│  │  CLI    │         │  (Next.js)       │         │                     │   │
│  └────┬────┘         └────────┬─────────┘         └──────────┬──────────┘   │
│       │                       │                              │              │
│       │  1. User runs:        │                              │              │
│       │     vget ai download whisper-tiny --from=vmirror     │              │
│       │                       │                              │              │
│       │  2. Prompt for email  │                              │              │
│       │     ───────────────►  │                              │              │
│       │     user@example.com  │                              │              │
│       │                       │                              │              │
│       │  3. Generate device   │                              │              │
│       │     fingerprint       │                              │              │
│       │     (machine ID +     │                              │              │
│       │      hardware serial) │                              │              │
│       │                       │                              │              │
│       │  4. POST /api/v1/token                               │              │
│       │     {email, fingerprint}                             │              │
│       │     ─────────────────►│                              │              │
│       │                       │  5. Check device limit       │              │
│       │                       │     ─────────────────────────►              │
│       │                       │                              │              │
│       │                       │  6. Store device, gen token  │              │
│       │                       │     ─────────────────────────►              │
│       │                       │                              │              │
│       │  7. {url: signed_url, expires_in}                    │              │
│       │     ◄─────────────────│                              │              │
│       │                       │                              │              │
│       │  8. Cache email in    │                              │              │
│       │     ~/.config/vget/auth.json                         │              │
│       │                       │                              │              │
└───────┼───────────────────────┼──────────────────────────────┼──────────────┘
        │                       │                              │
        ▼                       │                              │
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DOWNLOAD                                        │
│                                                                              │
│  ┌─────────┐                                          ┌─────────────────┐   │
│  │  vget   │                                          │  Bunny.net CDN  │   │
│  │  CLI    │                                          │  (vmirror.org)  │   │
│  └────┬────┘                                          └────────┬────────┘   │
│       │                                                        │            │
│       │  9. GET signed URL:                                    │            │
│       │     https://cdn2.vmirror.org/models/whisper-tiny.bin   │            │
│       │       ?token=abc123...&expires=1704067200              │            │
│       │     ──────────────────────────────────────────────────►│            │
│       │                                                        │            │
│       │  10. Bunny CDN Token Auth validates:                   │            │
│       │      - token matches SHA256(key + path + expires)      │            │
│       │      - expires > current time                          │            │
│       │                                                        │            │
│       │  11. File stream                                       │            │
│       │     ◄──────────────────────────────────────────────────│            │
│       │                                                        │            │
│       │  12. Save to ~/.config/vget/models/                    │            │
│       │                                                        │            │
└───────┼────────────────────────────────────────────────────────┼────────────┘
        ▼                                                        │
   Download complete!
```

## Components

### 1. vget CLI (`internal/core/auth/`)

**Device Fingerprint Generation:**

```go
// internal/core/auth/fingerprint.go
func GetDeviceFingerprint() string {
    parts := []string{getMachineID()}
    if serial := getHardwareSerial(); serial != "" {
        parts = append(parts, serial)
    }
    // Add hostname and username
    // Hash all together with SHA-256
    h := sha256.Sum256([]byte(strings.Join(parts, "|")))
    return hex.EncodeToString(h[:16]) // 32 hex chars
}
```

**Platform-specific sources:**

| OS | Machine ID | Hardware Serial |
|----|------------|-----------------|
| macOS | `ioreg` IOPlatformUUID | `system_profiler` serial |
| Linux | `/etc/machine-id` | `/sys/class/dmi/id/product_serial` |
| Windows | Registry MachineGuid | `wmic bios` serial |

**Device registration caching:**

```json
// ~/.config/vget/auth.json
{
  "email": "user@example.com",
  "fingerprint": "a3f8b2c1d4e5f678..."
}
```

### 2. Backend API (`vget-io/app/api/v1/token/`)

**Endpoint:** `POST /api/v1/token`

**Request:**
```json
{
  "email": "user@example.com",
  "fingerprint": "a3f8b2c1d4e5f678901234567890abcd",
  "file": "whisper-tiny.bin"
}
```

**Response:**
```json
{
  "url": "https://cdn2.vmirror.org/models/whisper-tiny.bin?token=abc123...&expires=1704067200",
  "expires_in": 43200
}
```

**Rate limits:**
| Limit | Value |
|-------|-------|
| Devices per email | 2 |
| Downloads per 6 hours | 6 (per email or fingerprint) |
| Signed URL expiry | 12 hours |

**Rate limit error response:**
```json
{
  "error": "rate_limit_exceeded",
  "error_code": "RATE_LIMIT"
}
```
HTTP status: 429

**Database schema:**
```sql
-- Device registrations (rate limiting)
CREATE TABLE vget_devices (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(email, fingerprint)
);

-- Download logs (anti-spam)
CREATE TABLE vget_downloads (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    file TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**Signed URL generation (Bunny CDN Token Auth):**
```typescript
function generateSignedUrl(file: string): string {
  const urlPath = `/models/${file}`;
  const expires = Math.floor(Date.now() / 1000) + 43200; // 12 hours

  const hashableBase = BUNNY_CDN_TOKEN_AUTH_KEY + urlPath + expires;
  const token = createHash("sha256")
    .update(hashableBase)
    .digest("base64")
    .replace(/\n/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");

  return `${CDN_BASE_URL}${urlPath}?token=${token}&expires=${expires}`;
}
```

### 3. Bunny.net CDN Token Authentication

Configure in Bunny.net dashboard -> Pull Zone -> Security:

1. Enable **Token Authentication**
2. Set **Authentication Type** to **URL Token Authentication**
3. Set **Token Security Key** (same as `BUNNY_CDN_TOKEN_AUTH_KEY` in your backend)

Bunny CDN will automatically validate:
- `token` parameter matches `SHA256(key + path + expires)`
- `expires` timestamp is in the future

No Edge Rules needed - Token Auth is built into Bunny CDN.

## Security Analysis

### What this stops:

| Threat | Mitigated? | How |
|--------|------------|-----|
| Hotlinking from websites | Yes | User-Agent + token required |
| Casual curl/wget abuse | Yes | Need to register email first |
| Mass device spoofing | Partially | 2 device limit per email |
| Bandwidth monitoring | Yes | Each download tied to email |
| Download flooding | Yes | 6 downloads per 6 hours limit |

### What this doesn't stop:

| Threat | Why | Acceptable? |
|--------|-----|-------------|
| Determined attackers | Can extract token from config | Yes - models are free elsewhere |
| Fingerprint spoofing | Requires OS reinstall or manual effort | Yes - high friction |
| Email spam registration | No email verification | Consider adding if abused |

### Trade-offs:

```
┌─────────────────────────────────────────────────────────────┐
│                     SECURITY <-------> UX                    │
│                                                              │
│  More secure                              Better UX          │
│  <-------------------------------------------------------->  │
│                                                              │
│  Email + verification     Email only      No auth            │
│  + CAPTCHA               (current)        (User-Agent only)  │
│                              ^                               │
│                              |                               │
│                         We are here                          │
└─────────────────────────────────────────────────────────────┘
```

## Development Setup

### 1. Start the backend

```bash
cd /path/to/vget-io

# Run database migration
psql $PG_DATABASE_URL < lib/migrations/001_vget_devices.sql

# Start dev server
npm run dev  # http://localhost:3000
```

### 2. Test vget with local endpoint

```bash
cd /path/to/vget

# Build
CGO_ENABLED=0 go build -o build/vget ./cmd/vget

# Run with local auth endpoint
VGET_AUTH_ENDPOINT=http://localhost:3000/api/v1/token \
  ./build/vget ai download whisper-tiny --from=vmirror
```

### 3. Expected output

```
$ VGET_AUTH_ENDPOINT=http://localhost:3000/api/v1/token ./build/vget ai download whisper-tiny --from=vmirror

  Enter your email to download models: user@example.com

  📦 Downloading whisper-tiny (78MB)
  Source: vmirror.org
    URL: https://cdn2.vmirror.org/models/whisper-tiny.bin

        ████████████████████████████░░  93% 72.5 MB / 78.0 MB

  ✓ Download complete!
  Location: /Users/you/.config/vget/models/whisper-tiny.bin
```

### 4. Verify device was registered

```bash
cat ~/.config/vget/auth.json
```

```json
{
  "email": "user@example.com",
  "fingerprint": "a3f8b2c1d4e5f678901234567890abcd"
}
```

### 5. Subsequent downloads (no prompt)

```bash
./build/vget ai download whisper-small --from=vmirror
# Uses cached token, no email prompt
```

## Production Deployment

### Environment variables

**vget-io (Next.js):**
```env
PG_DATABASE_URL=postgres://user:pass@host:5432/db
BUNNY_CDN_TOKEN_AUTH_KEY=your-bunny-token-auth-key
```

**Bunny.net Pull Zone:**
- Enable Token Authentication with the same key as `BUNNY_CDN_TOKEN_AUTH_KEY`

### Monitoring

Track abuse via database queries:

```sql
-- Devices per email (find users at max limit)
SELECT email, COUNT(*) as device_count
FROM vget_devices
GROUP BY email
HAVING COUNT(*) >= 2
ORDER BY device_count DESC;

-- Recent registrations
SELECT email, fingerprint, created_at
FROM vget_devices
ORDER BY created_at DESC
LIMIT 50;

-- Suspicious: same fingerprint, different emails (potential spoofing)
SELECT fingerprint, array_agg(email) as emails, COUNT(*) as count
FROM vget_devices
GROUP BY fingerprint
HAVING COUNT(DISTINCT email) > 1;

-- Download frequency by email (last 24h)
SELECT email, COUNT(*) as downloads, array_agg(DISTINCT file) as files
FROM vget_downloads
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY email
ORDER BY downloads DESC
LIMIT 20;

-- Download frequency by IP (last 24h) - detect shared abuse
SELECT ip_address, COUNT(*) as downloads, array_agg(DISTINCT email) as emails
FROM vget_downloads
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY ip_address
HAVING COUNT(DISTINCT email) > 1
ORDER BY downloads DESC;

-- Most downloaded files
SELECT file, COUNT(*) as downloads
FROM vget_downloads
WHERE created_at > NOW() - INTERVAL '7 days'
GROUP BY file
ORDER BY downloads DESC;

-- Users hitting rate limits (6+ downloads in 6 hours)
SELECT email, fingerprint, COUNT(*) as downloads
FROM vget_downloads
WHERE created_at > NOW() - INTERVAL '6 hours'
GROUP BY email, fingerprint
HAVING COUNT(*) >= 6
ORDER BY downloads DESC;
```

## Files Reference

**vget (Go CLI):**
```
internal/core/auth/
├── email.go               # Email validation (RFC regex)
├── fingerprint.go         # GetDeviceFingerprint()
├── fingerprint_darwin.go  # macOS: IOPlatformUUID + hardware serial
├── fingerprint_linux.go   # Linux: /etc/machine-id + DMI serial
├── fingerprint_windows.go # Windows: MachineGuid + BIOS serial
└── token.go               # Signed URL request, error handling
```

**vget-io (Next.js backend):**
```
app/api/v1/token/
└── route.ts               # POST: get signed URL, GET: check devices

lib/migrations/
├── 001_vget_devices.sql   # Device registration table
└── 002_vget_downloads.sql # Download logs table
```

## FAQ

**Q: What if a user reinstalls their OS?**
A: They get a new fingerprint and register as a new device. With 2 devices per email, this should be fine for most users.

**Q: What if someone extracts the token from the config?**
A: The token is per-device and tied to the fingerprint. Using it from a different machine would require spoofing the fingerprint too.

**Q: Can someone spoof the fingerprint?**
A: Yes, but it requires significant effort (registry edits on Windows, file modifications on Linux). This is acceptable friction.

**Q: Why not use Cloudflare Turnstile?**
A: Turnstile requires a browser, which doesn't work in CLI environments. Also, Cloudflare is blocked/slow in China.

**Q: What's the fallback if vmirror is down?**
A: Users can download from official sources (HuggingFace/GitHub) by omitting `--from=vmirror`.

**Q: Why email instead of username?**
A: Email provides natural accountability and is already familiar to users. No need to invent a new identity system.
