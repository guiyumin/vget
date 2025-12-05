# Telegram Support Research

Research notes on adding Telegram video/media download support to vget.

## Technical Feasibility

### Challenges

1. **Authentication Required** - Telegram videos (especially from private channels/groups) require user authentication via MTProto protocol or Bot API
2. **No Public URLs** - Unlike Twitter/YouTube, Telegram doesn't expose direct media URLs. Videos are accessed through Telegram's encrypted API
3. **API Credentials Required** - Must register at https://my.telegram.org to get `api_id` and `api_hash`. Using someone else's credentials risks account ban.

### Comparison with Other Platforms

| Platform | Difficulty | Why |
|----------|------------|-----|
| Twitter | Easy | Public API endpoints |
| YouTube | Medium | yt-dlp patterns work |
| Xiaoyuzhou | Easy | RSS/public API |
| **Telegram** | **Hard** | Auth + MTProto protocol |

## Reference Implementation: tdl

Analyzed the `tdl` project (github.com/iyear/tdl) - a mature Go CLI for Telegram downloads.

### Architecture

```
CLI Layer (cmd/)
    ↓
Application Layer (app/)
    ↓
Core Services (core/)
    ↓
Package Utilities (pkg/)
```

### Core Dependency

```go
github.com/gotd/td  // Pure Go MTProto 2.0 implementation
```

### Authentication Methods

| Method | How it works |
|--------|--------------|
| **Desktop Import** | Reads encrypted `tdata/` from Telegram Desktop |
| **Phone + Code** | Interactive SMS verification |
| **QR Code** | Scan with mobile app |

### API Credentials Handling

From `pkg/tclient/app.go`:

```go
var Apps = map[string]App{
    // application created by tdl author
    AppBuiltin: {AppID: 15055931, AppHash: "..."},

    // official Telegram Desktop credentials
    AppDesktop: {AppID: 2040, AppHash: "..."},
}
```

### Risk Levels

| Approach | Risk | Why |
|----------|------|-----|
| Your own API ID | Low | Telegram knows it's you |
| Desktop import + Desktop API ID | Low | Consistent app identity |
| Using shared/builtin ID | Medium | Shared across all users |
| Hardcoding random ID | High | Suspicious to Telegram |

### Key Components Worth Borrowing

1. **Iterator + Resume Pattern** - Fingerprinting dialogs, storing finished indices
2. **Data Center Pooling** - Lazy connection initialization, per-DC clients
3. **Middleware Architecture** - Retry, recovery, flood-wait as composable layers
4. **Message Link Parsing** - Handles various `t.me/...` URL formats

## Implementation Options for vget

### Option 1: Native Implementation

Import `gotd/td` directly into vget.

**Pros:**
- Full control, native integration
- Same patterns as other extractors

**Cons:**
- Auth flow complexity (phone, 2FA, sessions)
- ~500-800+ lines for minimal implementation
- Session management, DC pooling, flood-wait handling

### Option 2: Shell out to tdl

Detect `t.me` URLs and delegate to `tdl`.

**Pros:**
- Zero implementation effort
- Handles all edge cases

**Cons:**
- External dependency
- Less integrated UX

### Option 3: Desktop Session Import Only

Use Telegram Desktop's API ID with session import.

**Pros:**
- Lower friction (no phone verification if Desktop installed)
- Lower ban risk (consistent app identity)

**Cons:**
- Requires Telegram Desktop
- Still significant implementation

## Decision

**Not pursuing Telegram support for now.** Reasons:

1. High complexity vs. payoff
2. API credential requirement adds user friction
3. Ban risk if implemented incorrectly
4. `tdl` already exists and works well
5. Better ROI focusing on other platforms

## Alternative for Users

Users needing Telegram downloads should use:
- **Telegram Desktop** - Built-in "Save as..." and auto-download
- **tdl** - `go install github.com/iyear/tdl@latest`

## References

- tdl source: https://github.com/iyear/tdl
- gotd/td (MTProto library): https://github.com/gotd/td
- Telegram API registration: https://my.telegram.org
