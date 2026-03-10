# Telegram Support

Implementation plan for Telegram media download support in vget.

## Overview

vget aims to be an all-in-one media downloader. Telegram support is part of this vision, even though `tdl` (6k+ stars) exists as a dedicated tool.

**Current**: Desktop session import using Telegram Desktop's API credentials.

**Future**: Full CLI Telegram client capabilities (phone login, QR login, etc.).

## Technical Background

### How Telegram Auth Works

```
api_id + api_hash  =  identifies THE APP (vget)
user session       =  identifies THE USER's account
```

- Sessions are tied to the `api_id` they were created with
- Desktop session import reuses existing login from Telegram Desktop
- No phone/SMS verification needed if user has Desktop installed

### API Credentials

Currently using Telegram Desktop's public credentials:

```go
const (
    TelegramDesktopAppID   = 2040
    TelegramDesktopAppHash = "b18441a1ff607e10a989891a5462e627"
)
```

These are safe to use:
- Already public (used by Telegram Desktop itself)
- Used by many third-party tools (tdl, etc.)
- Telegram cannot revoke without breaking Desktop app

Future: Register vget's own credentials for `--phone` login method.

### Login Methods & Ban Risk

| Method | API Credentials | Ban Risk | Why |
|--------|-----------------|----------|-----|
| `--import-desktop` | Desktop's (2040) | Low | Reusing session, same app identity |
| `--phone` (future) | vget's own | **Zero** | Fresh session with registered app |
| `--qr` (future) | vget's own | **Zero** | Fresh session with registered app |
| `--bot-token` (future) | N/A | **Zero** | Bot tokens are inherently safe |

## Dependencies

```go
github.com/gotd/td                    // Pure Go MTProto 2.0 implementation
github.com/gotd/td/session/tdesktop   // Desktop session import
```

## Implementation Status

### Phase 1: MVP (Implemented)

#### 1. Session Management Commands

```bash
vget telegram login                  # Shows available login methods
vget telegram login --import-desktop # Import from Telegram Desktop
vget telegram logout                 # Clear stored session
vget telegram status                 # Show login state
```

**Desktop import flow (`--import-desktop`):**
- Reads Desktop's `tdata/` directory
  - macOS: `~/Library/Application Support/Telegram Desktop/tdata/`
  - Linux: `~/.local/share/TelegramDesktop/tdata/`
  - Windows: `%APPDATA%/Telegram Desktop/tdata/`
  - **Custom path**: Set via `vget config set telegram.tdata_path /path/to/tdata`
- Imports session using Desktop's API credentials (2040)
- Session stored in `~/.config/vget/telegram/desktop-session.json`

#### Session Storage & Multi-Account

**Session file layout:**
```
~/.config/vget/telegram/
├── desktop-session.json        # Imported from Telegram Desktop (current)
└── cli-sessions/               # Future: phone/QR login sessions
    ├── account1.json
    └── account2.json
```

**Current behavior:**
- Desktop import stores session at `desktop-session.json`
- If Desktop has multiple accounts, vget imports the **first/primary** account
- Re-importing **overwrites** the previous session

**Multi-account workflow (current):**
1. Switch to desired account in Telegram Desktop
2. Run `vget telegram login --import-desktop`
3. vget now uses that account
4. To switch: repeat steps 1-2

**Future (full CLI client):**
```bash
# Phone login creates named session in cli-sessions/
vget telegram login --phone --name work
vget telegram login --phone --name personal

# Use specific account
vget --account work https://t.me/channel/123
```

For now, Telegram Desktop manages multi-account; vget imports whichever is active.

#### Future Login Methods

| Flag | Description | Status |
|------|-------------|--------|
| `--import-desktop` | Import from Telegram Desktop | Implemented |
| `--phone` | Phone + SMS/code verification | Planned |
| `--qr` | QR code login (scan with mobile) | Planned |
| `--bot-token` | Bot authentication | Planned |

**Phone login flow (`--phone`):**
1. User enters phone number
2. Telegram sends verification code:
   - **Primary**: In-app message to existing Telegram sessions (Desktop/mobile)
   - **Fallback**: SMS (if no active sessions or user requests it)
3. User enters code
4. (Optional) Enters 2FA password if enabled
5. Session created with vget's API credentials

**QR login flow (`--qr`):**
1. vget displays QR code in terminal
2. User scans with Telegram mobile app
3. Session created automatically
4. No phone number or code needed

**Bot token flow (`--bot-token`):**
1. User provides bot token from @BotFather
2. Authenticate as bot (limited permissions)
3. Useful for downloading from public channels only

#### 2. URL Parsing

Support these `t.me` formats:

| Format | Example | Type |
|--------|---------|------|
| Public channel | `https://t.me/channel/123` | Public |
| Private channel | `https://t.me/c/123456789/123` | Private |
| User/bot post | `https://t.me/username/123` | Public |
| Single from album | `https://t.me/channel/123?single` | Public |

#### 3. Single Message Download

```bash
vget https://t.me/somechannel/456
```

- Extract media (video/audio/document) from one message
- Download with progress bar (existing Bubbletea infrastructure)
- Save to current directory or `-o` path

#### 4. Media Type Detection

```go
MediaTypeVideo     // .mp4, .mov
MediaTypeAudio     // .mp3, .ogg voice messages
MediaTypeDocument  // .pdf, .zip, etc.
MediaTypePhoto     // .jpg (lower priority)
```

### Phase 2: Nice-to-Have

| Feature | Description |
|---------|-------------|
| Batch download | `vget https://t.me/channel/100-200` (range) |
| Resume | Continue interrupted downloads |
| Album support | Download all media from grouped messages |
| Channel dump | `vget https://t.me/channel --all` |

## File Structure

```
internal/core/extractor/
├── telegram.go              # Thin wrapper, registers extractor, re-exports
├── telegram/
│   ├── constants.go         # API credentials (DesktopAppID, DesktopAppHash)
│   ├── parser.go            # URL parsing
│   ├── session.go           # Session path/exists helpers
│   ├── media.go             # Media extraction helpers
│   ├── extractor.go         # Extractor implementation
│   ├── download.go          # Download functionality + DownloadWithOptions
│   └── takeout.go           # TakeoutSession for bulk downloads

internal/cli/
├── telegram.go              # login/logout/status commands
├── batch.go                 # Batch download with auto-takeout for Telegram
```

## vget vs tdl

| Aspect | tdl | vget |
|--------|-----|------|
| Scope | Telegram-only | Multi-platform |
| Features | Many advanced (batch, resume, takeout) | Simple + auto-takeout for batch |
| Philosophy | Power tool | All-in-one simplicity |

## Reference Implementation

The `tdl` project (github.com/iyear/tdl) was analyzed for patterns:

### Worth Borrowing

1. **URL Parsing** (`pkg/tmessage/parse.go`) - handles various t.me formats
2. **Media Extraction** (`core/tmedia/media.go`) - unified media type abstraction
3. **Middleware Pattern** - retry, recovery, flood-wait as composable layers

### Skip for MVP

- Iterator + Resume pattern (Phase 2)
- Data Center pooling (overkill for single downloads)

### Implemented from tdl

- **Takeout mode** - auto-enabled for batch downloads (2+ Telegram URLs)

## Protected/Restricted Content

### Understanding `noforwards` Flag

Channel owners can enable "Restrict saving content" which sets the `noforwards` flag on the channel or individual messages. This:
- Disables "Forward" button in official apps
- Disables "Save" button for media in official apps
- Shows "Saving content is restricted" message

### Why Downloads Still Work

**Key insight**: `noforwards` is a **client-side UI restriction**, not an API-level restriction.

The official Telegram apps *choose* to respect this flag by hiding UI buttons. But at the API level:
- If you have access to a message, you can read its content
- If you can read the content, you can download attached media
- The API does not block file downloads based on `noforwards`

This is how `tdl` and similar tools work - they use the Telegram Client API (MTProto) directly, bypassing the UI restrictions that official apps enforce.

### How tdl Detects Protected Content

From `tdl/core/forwarder/forwarder.go`:

```go
func protectedDialog(peer peers.Peer) bool {
    switch p := peer.(type) {
    case peers.Chat:
        return p.Raw().GetNoforwards()
    case peers.Channel:
        return p.Raw().GetNoforwards()
    }
    return false
}

func protectedMessage(msg *tg.Message) bool {
    return msg.GetNoforwards()
}
```

### Operation Differences

| Operation | Protected Content | How It Works |
|-----------|-------------------|--------------|
| **Download** | ✅ Works | Direct file access via API - `noforwards` doesn't apply |
| **Forward** | ⚠️ Blocked by API | Must use "clone" mode (re-upload as new message) |

### Takeout Mode for Bulk Downloads

For downloading many files, use Telegram's official "Data Export" feature via API:

```go
// From tdl/core/middlewares/takeout/takeout.go
req := &tg.AccountInitTakeoutSessionRequest{
    MessageChannels:   true,
    Files:             true,
    FileMaxSize:       4000 * 1024 * 1024,  // 4GB limit
}
```

Takeout sessions have **lower flood wait limits**, making bulk downloads faster and less likely to trigger rate limiting.

### vget Implementation Notes

For vget's Telegram support:
1. Desktop session import works for protected content - same API access as tdl
2. No special handling needed - just download the file if user has message access
3. Takeout mode is **auto-enabled** for batch downloads (2+ Telegram URLs)

## Takeout Mode (Implemented)

### What is Takeout?

Takeout is Telegram's official "Data Export" API feature (`AccountInitTakeoutSession`). It's designed for users to export their own data with **relaxed rate limits**.

| Without Takeout | With Takeout |
|-----------------|--------------|
| Normal flood wait limits | Lower flood wait limits |
| More likely to get rate-limited on bulk downloads | Designed for bulk export |
| Faster to hit `FLOOD_WAIT` errors | Can download more before limits |

### vget Implementation

Takeout is automatically enabled when batch downloading multiple Telegram URLs. No user flags needed.

**Usage:**

```bash
# Single file - no takeout (not needed)
vget https://t.me/channel/123

# Batch mode with 2+ Telegram URLs - takeout auto-enabled
vget -f urls.txt
```

**Implementation files:**

| File | Purpose |
|------|---------|
| `telegram/takeout.go` | `TakeoutSession` struct with `Start()`, `Finish()`, `Middleware()` |
| `telegram/download.go` | `DownloadWithOptions()` accepts `Takeout` bool |
| `cli/root.go` | `runTelegramBatchDownload()` uses takeout internally |
| `cli/batch.go` | Detects 2+ Telegram URLs → calls batch function |

**Core takeout logic:**

```go
// internal/extractor/telegram/takeout.go

type TakeoutSession struct {
    api       *tg.Client
    takeoutID int64
}

func (t *TakeoutSession) Start(ctx context.Context) error {
    req := &tg.AccountInitTakeoutSessionRequest{
        Files:       true,
        FileMaxSize: 4 * 1024 * 1024 * 1024, // 4GB
    }
    session, err := t.api.AccountInitTakeoutSession(ctx, req)
    if err != nil {
        return err
    }
    t.takeoutID = session.ID
    return nil
}

func (t *TakeoutSession) Finish(ctx context.Context) error {
    if t.takeoutID == 0 {
        return nil
    }
    req := &tg.AccountFinishTakeoutSessionRequest{Success: true}
    _, err := t.api.AccountFinishTakeoutSession(ctx, req)
    return err
}
```

**Batch download flow:**

```
urls.txt contains:
  https://t.me/channel/1
  https://t.me/channel/2
  https://twitter.com/user/status/123

vget -f urls.txt
  ↓
batch.go separates URLs:
  - telegramURLs: [t.me/1, t.me/2]  (2 URLs → use takeout)
  - otherURLs: [twitter.com/...]
  ↓
runTelegramBatchDownload(telegramURLs)
  → Each download uses takeout session
  ↓
runDownload() for other URLs
```

### Reference: How tdl Does It

tdl uses a similar pattern with middleware wrapping:

```go
// Wrap all API calls with takeout session ID
func (t takeout) Handle(next tg.Invoker) telegram.InvokeFunc {
    return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
        return next.Invoke(ctx, &tg.InvokeWithTakeoutRequest{
            TakeoutID: t.id,
            Query:     nopDecoder{input},
        }, output)
    }
}
```

## References

- tdl source: https://github.com/iyear/tdl
- gotd/td (MTProto library): https://github.com/gotd/td
- Telegram Desktop session format: https://github.com/nickoala/tdesktop-session
