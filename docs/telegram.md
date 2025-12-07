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
internal/extractor/
├── telegram.go              # Thin wrapper, registers extractor, re-exports
├── telegram/
│   ├── telegram.go          # Package constants (API credentials)
│   ├── parser.go            # URL parsing
│   ├── session.go           # Session path/exists helpers
│   ├── media.go             # Media extraction helpers
│   ├── extractor.go         # Extractor implementation
│   └── download.go          # Download functionality

internal/cli/
├── telegram.go              # login/logout/status commands
```

## vget vs tdl

| Aspect | tdl | vget |
|--------|-----|------|
| Scope | Telegram-only | Multi-platform |
| Features | Many advanced (batch, resume, takeout) | Simple: paste URL, get media |
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
- Takeout mode (for bulk exports)

## References

- tdl source: https://github.com/iyear/tdl
- gotd/td (MTProto library): https://github.com/gotd/td
- Telegram Desktop session format: https://github.com/nickoala/tdesktop-session
