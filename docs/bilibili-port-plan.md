# BBDown Go Port Plan

This document outlines the plan for porting [BBDown](https://github.com/nilaoda/BBDown) (a C# Bilibili downloader) to Go, integrating it into vget.

## Overview

**BBDown** is a comprehensive Bilibili downloader with ~6,500 lines of C# code. Key capabilities:

- Download videos, anime (Bangumi), courses (Cheese), playlists
- Multiple quality levels (144P to 8K) and codecs (AVC, HEVC, AV1)
- Audio formats: AAC, FLAC, Dolby Atmos, E-AC-3
- Authentication via QR code or cookie/token
- Subtitles, danmaku (bullet comments), cover images
- FFmpeg/MP4Box muxing integration

## Architecture Comparison

### BBDown (C#)

```
BBDown/                          # CLI Application
├── Program.cs                   # Entry point (897 lines)
├── CommandLineInvoker.cs        # CLI parsing
├── BBDownUtil.cs                # URL parsing utilities
├── BBDownDownloadUtil.cs        # HTTP download logic
├── BBDownMuxer.cs               # Audio/video muxing
├── BBDownLoginUtil.cs           # QR code login
└── Model/                       # Data models

BBDown.Core/                     # Core library
├── Parser.cs                    # API response parsing (467 lines)
├── AppHelper.cs                 # gRPC/Protobuf for APP API
├── Config.cs                    # Global configuration
├── FetcherFactory.cs            # Content type routing
├── IFetcher.cs                  # Fetcher interface
├── Entity/                      # Data models
├── Fetcher/                     # Content type handlers
│   ├── NormalInfoFetcher.cs     # Regular videos
│   ├── BangumiInfoFetcher.cs    # Anime
│   ├── CheeseInfoFetcher.cs     # Courses
│   └── ...                      # Others
└── Util/                        # HTTP, subtitles, danmaku
```

### vget Target (Go)

```
internal/extractor/
├── bilibili.go                  # Main extractor + interface
├── bilibili_api.go              # API client (WEB/TV/APP/INTL)
├── bilibili_parser.go           # Stream parsing
├── bilibili_auth.go             # Authentication (QR, cookie)
├── bilibili_fetcher.go          # Fetcher interface + factory
├── bilibili_fetcher_normal.go   # Regular videos
├── bilibili_fetcher_bangumi.go  # Anime
├── bilibili_fetcher_cheese.go   # Courses
├── bilibili_fetcher_space.go    # User uploads
├── bilibili_fetcher_list.go     # Playlists/collections
├── bilibili_subtitle.go         # Subtitle processing
├── bilibili_danmaku.go          # Bullet comments
└── bilibili_proto/              # Generated protobuf (for APP API)
```

## Bilibili API Structure

BBDown supports 4 different APIs for accessing content:

| API  | Endpoint             | Auth Method         | Use Case              |
| ---- | -------------------- | ------------------- | --------------------- |
| WEB  | api.bilibili.com     | Cookie (SESSDATA)   | Standard access       |
| TV   | api.snm0516.aisee.tv | App key + signature | Unrestricted streams  |
| APP  | grpc.biliapi.net     | gRPC + Protobuf     | FLAC, Dolby, 8K       |
| INTL | api.biliintl.com     | Similar to WEB      | International content |

### WBI Signature (WEB API)

Bilibili uses a dynamic signature scheme called WBI:

1. Extract `img_key` and `sub_key` from website HTML
2. Combine and reorder using a fixed mapping table
3. Generate MD5 signature of parameters + key
4. Keys rotate periodically (need to refresh)

### APP API (gRPC)

The APP API uses Protocol Buffers with custom headers:

- Device info (Dalvik, Android version)
- Access key authentication
- Protobuf request/response encoding

## Implementation Phases

### Phase 1: Foundation (Priority: High)

**Goal**: Basic video download for regular Bilibili videos

#### 1.1 Data Models

```go
// internal/extractor/bilibili.go

type BilibiliVideoInfo struct {
    AID       int64    // av number
    BVID      string   // BV number
    CID       int64    // cid (for video stream)
    Title     string
    Desc      string
    Pic       string   // cover URL
    Duration  int64
    Pages     []Page   // multi-part videos
}

type Page struct {
    CID       int64
    Page      int
    Title     string
    Duration  int64
}

type VideoStream struct {
    Quality   int      // 127=8K, 120=4K, 116=1080P60, etc.
    Codec     string   // avc, hevc, av1
    URL       string
    Bandwidth int64
    Width     int
    Height    int
}

type AudioStream struct {
    Quality   int      // 30280=320kbps, 30232=128kbps, 30216=64kbps
    Codec     string   // mp4a, flac, ec-3
    URL       string
    Bandwidth int64
}
```

#### 1.2 URL Pattern Matching

```go
// Match patterns:
// - https://www.bilibili.com/video/BV1xx411c7mD
// - https://www.bilibili.com/video/av170001
// - https://b23.tv/BV1xx411c7mD (short URL)
// - bilibili://video/170001

func (e *BilibiliExtractor) Match(url string) bool {
    patterns := []string{
        `bilibili\.com/video/(BV[\w]+|av\d+)`,
        `b23\.tv/(BV[\w]+|av\d+|\w+)`,
        `bilibili://video/\d+`,
    }
    // ...
}
```

#### 1.3 BV/AV ID Conversion

```go
// AV to BV and vice versa (algorithm from BBDown)
// BV is base58-like encoding of AV number

const table = "fZodR9XQDSUm21yCkr6zBqiveYah8bt4xsWpHnJE7jL5VG3guMTKNPAwcF"
var s = []int{11, 10, 3, 8, 4, 6}
const xor = 177451812
const add = 8728348608

func BV2AV(bv string) int64 { ... }
func AV2BV(av int64) string { ... }
```

#### 1.4 WEB API Client

```go
// internal/extractor/bilibili_api.go

type BilibiliClient struct {
    httpClient *http.Client
    cookie     string
    wbi        *WBIKeys  // signature keys
}

func (c *BilibiliClient) GetVideoInfo(bvid string) (*BilibiliVideoInfo, error) {
    // GET https://api.bilibili.com/x/web-interface/view?bvid=xxx
}

func (c *BilibiliClient) GetPlayURL(bvid string, cid int64, qn int) (*PlayURLResponse, error) {
    // GET https://api.bilibili.com/x/player/wbi/playurl?bvid=xxx&cid=xxx&qn=xxx
    // Requires WBI signature
}
```

#### 1.5 Stream Extraction

```go
// Parse playurl API response to extract video/audio streams
func (c *BilibiliClient) ExtractStreams(resp *PlayURLResponse) ([]VideoStream, []AudioStream, error) {
    // Handle both DASH (video+audio separate) and legacy FLV formats
}
```

### Phase 2: Authentication (Priority: High)

**Goal**: Support both QR code login and manual cookie input, in both CLI and UI.

#### 2.1 Authentication Architecture

```
internal/
├── extractor/
│   └── bilibili_auth.go       # Core auth logic (API calls, token storage)
├── cli/
│   └── bilibili_login.go      # CLI: ASCII QR + cookie prompt
└── ui/                        # (existing React UI)
    └── components/
        └── BilibiliLogin.tsx  # UI: Image QR + cookie input field
```

#### 2.2 Core Auth Module

```go
// internal/extractor/bilibili_auth.go

type BilibiliAuth struct {
    configPath string  // ~/.config/vget/bilibili.json
}

type BilibiliCredentials struct {
    SESSDATA  string    `json:"sessdata"`
    BiliJCT   string    `json:"bili_jct"`
    DedeUserID string   `json:"dede_user_id"`
    ExpiresAt time.Time `json:"expires_at"`
}

// QR Code Login Flow
type QRLoginSession struct {
    URL       string  // QR code content URL
    QRCodeKey string  // Key for polling status
}

func (a *BilibiliAuth) GenerateQRCode() (*QRLoginSession, error) {
    // GET https://passport.bilibili.com/x/passport-login/web/qrcode/generate
    // Returns: { data: { url: "...", qrcode_key: "..." } }
}

type QRStatus int
const (
    QRWaiting   QRStatus = 86101  // Not scanned yet
    QRScanned   QRStatus = 86090  // Scanned, waiting confirm
    QRExpired   QRStatus = 86038  // QR code expired
    QRConfirmed QRStatus = 0      // Success
)

func (a *BilibiliAuth) PollQRStatus(qrcodeKey string) (QRStatus, *BilibiliCredentials, error) {
    // GET https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=xxx
    // Returns status code and credentials on success
}

// Manual Cookie
func (a *BilibiliAuth) SetCookie(cookie string) (*BilibiliCredentials, error) {
    // Parse cookie string: "SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx"
    // Validate by calling user info API
    // Save to config file
}

// Token Storage
func (a *BilibiliAuth) SaveCredentials(creds *BilibiliCredentials) error {
    // Save to ~/.config/vget/bilibili.json
}

func (a *BilibiliAuth) LoadCredentials() (*BilibiliCredentials, error) {
    // Load from ~/.config/vget/bilibili.json
    // Return nil if not found or expired
}

func (a *BilibiliAuth) GetCookieString() string {
    // Return formatted cookie for HTTP requests
    // "SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx"
}
```

#### 2.3 CLI Login Interface

```go
// internal/cli/bilibili_login.go

// Command: vget login bilibili
func BilibiliLoginCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "bilibili",
        Short: "Login to Bilibili",
    }

    cmd.AddCommand(
        bilibiliQRLoginCmd(),     // vget login bilibili qr
        bilibiliCookieLoginCmd(), // vget login bilibili cookie
    )

    return cmd
}

// QR Login in Terminal
func bilibiliQRLogin() error {
    auth := extractor.NewBilibiliAuth()

    // 1. Generate QR code
    session, _ := auth.GenerateQRCode()

    // 2. Display ASCII QR in terminal
    qr, _ := qrcode.New(session.URL, qrcode.Medium)
    fmt.Println(qr.ToSmallString(false))
    fmt.Println("Scan with Bilibili app, or open:", session.URL)

    // 3. Poll for confirmation
    for {
        status, creds, _ := auth.PollQRStatus(session.QRCodeKey)
        switch status {
        case extractor.QRWaiting:
            // Show spinner
        case extractor.QRScanned:
            fmt.Println("Scanned! Please confirm in app...")
        case extractor.QRExpired:
            return errors.New("QR code expired")
        case extractor.QRConfirmed:
            auth.SaveCredentials(creds)
            fmt.Println("Login successful!")
            return nil
        }
        time.Sleep(time.Second)
    }
}

// Cookie Login in Terminal
func bilibiliCookieLogin() error {
    fmt.Println("Enter your Bilibili cookie (SESSDATA=xxx; bili_jct=xxx):")
    reader := bufio.NewReader(os.Stdin)
    cookie, _ := reader.ReadString('\n')

    auth := extractor.NewBilibiliAuth()
    creds, err := auth.SetCookie(strings.TrimSpace(cookie))
    if err != nil {
        return fmt.Errorf("invalid cookie: %w", err)
    }

    fmt.Printf("Login successful! User ID: %s\n", creds.DedeUserID)
    return nil
}
```

#### 2.4 UI Login Interface

```typescript
// ui/components/BilibiliLogin.tsx

interface QRLoginState {
  qrUrl: string;
  qrCodeKey: string;
  status: 'waiting' | 'scanned' | 'expired' | 'success';
}

export function BilibiliLogin() {
  const [mode, setMode] = useState<'qr' | 'cookie'>('qr');
  const [qrState, setQrState] = useState<QRLoginState | null>(null);
  const [cookie, setCookie] = useState('');

  // QR Code Login
  async function startQRLogin() {
    const session = await api.bilibili.generateQR();
    setQrState({ qrUrl: session.url, qrCodeKey: session.qrcode_key, status: 'waiting' });
    pollQRStatus(session.qrcode_key);
  }

  async function pollQRStatus(key: string) {
    const interval = setInterval(async () => {
      const result = await api.bilibili.pollQR(key);
      if (result.status === 'confirmed') {
        clearInterval(interval);
        setQrState(s => ({ ...s!, status: 'success' }));
      } else if (result.status === 'expired') {
        clearInterval(interval);
        setQrState(s => ({ ...s!, status: 'expired' }));
      } else if (result.status === 'scanned') {
        setQrState(s => ({ ...s!, status: 'scanned' }));
      }
    }, 1000);
  }

  // Cookie Login
  async function submitCookie() {
    await api.bilibili.setCookie(cookie);
  }

  return (
    <div>
      <Tabs value={mode} onChange={setMode}>
        <Tab value="qr">QR Code</Tab>
        <Tab value="cookie">Cookie</Tab>
      </Tabs>

      {mode === 'qr' && (
        <div>
          {qrState ? (
            <>
              <QRCodeImage value={qrState.qrUrl} />
              <StatusText status={qrState.status} />
            </>
          ) : (
            <Button onClick={startQRLogin}>Generate QR Code</Button>
          )}
        </div>
      )}

      {mode === 'cookie' && (
        <div>
          <p>Get cookie from browser DevTools → Application → Cookies</p>
          <TextArea
            placeholder="SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx"
            value={cookie}
            onChange={setCookie}
          />
          <Button onClick={submitCookie}>Save Cookie</Button>
        </div>
      )}
    </div>
  );
}
```

#### 2.5 API Endpoints for UI

```go
// internal/server/bilibili_routes.go

func RegisterBilibiliRoutes(r *mux.Router) {
    r.HandleFunc("/api/bilibili/qr/generate", handleGenerateQR).Methods("POST")
    r.HandleFunc("/api/bilibili/qr/poll", handlePollQR).Methods("GET")
    r.HandleFunc("/api/bilibili/cookie", handleSetCookie).Methods("POST")
    r.HandleFunc("/api/bilibili/status", handleAuthStatus).Methods("GET")
}

func handleGenerateQR(w http.ResponseWriter, r *http.Request) {
    auth := extractor.NewBilibiliAuth()
    session, _ := auth.GenerateQRCode()
    json.NewEncoder(w).Encode(session)
}

func handlePollQR(w http.ResponseWriter, r *http.Request) {
    key := r.URL.Query().Get("qrcode_key")
    auth := extractor.NewBilibiliAuth()
    status, creds, _ := auth.PollQRStatus(key)

    if status == extractor.QRConfirmed {
        auth.SaveCredentials(creds)
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": statusToString(status),
    })
}

func handleSetCookie(w http.ResponseWriter, r *http.Request) {
    var req struct{ Cookie string }
    json.NewDecoder(r.Body).Decode(&req)

    auth := extractor.NewBilibiliAuth()
    creds, err := auth.SetCookie(req.Cookie)
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "user_id": creds.DedeUserID,
    })
}
```

#### 2.6 CLI Commands Summary

```bash
# QR Code login (shows ASCII QR in terminal)
vget login bilibili qr

# Cookie login (interactive prompt)
vget login bilibili cookie

# Cookie login (non-interactive, for scripts)
vget login bilibili cookie --value "SESSDATA=xxx; bili_jct=xxx"

# Check login status
vget login bilibili status

# Logout (remove saved credentials)
vget login bilibili logout
```

#### 2.7 Credential Storage

```json
// ~/.config/vget/bilibili.json
{
  "sessdata": "abc123...",
  "bili_jct": "def456...",
  "dede_user_id": "12345678",
  "expires_at": "2024-06-01T00:00:00Z"
}
```

#### 2.8 Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/skip2/go-qrcode` | Generate QR code for terminal (ASCII) |
| `github.com/mdp/qrterminal/v3` | Alternative: better terminal QR rendering |

### Phase 3: Extended Content Types (Priority: Medium)

#### 3.1 Fetcher Interface

```go
// internal/extractor/bilibili_fetcher.go

type BilibiliFetcher interface {
    Name() string
    Match(id string) bool
    Fetch(id string, client *BilibiliClient) ([]*BilibiliVideoInfo, error)
}

// Factory function
func CreateFetcher(id string) BilibiliFetcher {
    switch {
    case strings.HasPrefix(id, "ep"):
        return &BangumiFetcher{}
    case strings.HasPrefix(id, "ss"):
        return &BangumiFetcher{}
    case strings.HasPrefix(id, "cheese:"):
        return &CheeseFetcher{}
    case strings.HasPrefix(id, "mid"):
        return &SpaceFetcher{}
    default:
        return &NormalFetcher{}
    }
}
```

#### 3.2 Anime (Bangumi) Fetcher

```go
// internal/extractor/bilibili_fetcher_bangumi.go

// Handles:
// - https://www.bilibili.com/bangumi/play/ep123
// - https://www.bilibili.com/bangumi/play/ss456
// - https://www.bilibili.com/bangumi/media/md789

type BangumiFetcher struct{}

func (f *BangumiFetcher) Fetch(id string, client *BilibiliClient) ([]*BilibiliVideoInfo, error) {
    // GET https://api.bilibili.com/pgc/view/web/season?ep_id=xxx
    // or season_id for ss/md IDs
}
```

#### 3.3 Course (Cheese) Fetcher

```go
// internal/extractor/bilibili_fetcher_cheese.go

// Handles paid courses:
// - https://www.bilibili.com/cheese/play/ep123

type CheeseFetcher struct{}
```

#### 3.4 User Space Fetcher

```go
// internal/extractor/bilibili_fetcher_space.go

// Download all videos from a user:
// - mid123456 (user ID)

type SpaceFetcher struct{}
```

### Phase 4: APP API & Advanced Quality (Priority: Medium)

#### 4.1 Protobuf Setup

```bash
# Generate Go code from proto files
protoc --go_out=. --go-grpc_out=. bilibili_proto/*.proto
```

Proto files to port from BBDown:

- `bilibili.app.playurl.v1.proto`
- `bilibili.pgc.gateway.player.v2.proto`
- Various header and metadata protos

#### 4.2 APP API Client

```go
// internal/extractor/bilibili_api_app.go

type BilibiliAppClient struct {
    grpcClient *grpc.ClientConn
    accessKey  string
}

func (c *BilibiliAppClient) GetPlayView(aid, cid int64) (*PlayViewReply, error) {
    // gRPC call to grpc.biliapi.net
    // Required for FLAC, Dolby Atmos, 8K HDR
}
```

### Phase 5: Post-Processing (Priority: Low)

#### 5.1 Subtitle Processing

```go
// internal/extractor/bilibili_subtitle.go

// Download and convert subtitles
func (c *BilibiliClient) GetSubtitles(bvid string, cid int64) ([]Subtitle, error) {
    // Parse from video info API
    // Convert BCC (Bilibili) format to SRT
}
```

#### 5.2 Danmaku (Bullet Comments)

```go
// internal/extractor/bilibili_danmaku.go

// Download danmaku in various formats
func (c *BilibiliClient) GetDanmaku(cid int64, format string) ([]byte, error) {
    // Formats: xml, protobuf, ass
    // GET https://api.bilibili.com/x/v1/dm/list.so?oid=xxx
}
```

## Quality Priority Mapping

```go
var QualityMap = map[int]string{
    127: "8K",
    126: "Dolby Vision",
    125: "HDR",
    120: "4K",
    116: "1080P60",
    112: "1080P+",
    80:  "1080P",
    74:  "720P60",
    64:  "720P",
    32:  "480P",
    16:  "360P",
    6:   "144P (for mobile)",
}

var AudioQualityMap = map[int]string{
    30280: "320kbps",
    30232: "128kbps",
    30216: "64kbps",
    30250: "Dolby Atmos",
    30251: "Hi-Res",
}
```

## Integration with vget

### CLI Commands

```bash
# Basic download
vget https://www.bilibili.com/video/BV1xx411c7mD

# With quality selection
vget -q 1080p https://www.bilibili.com/video/BV1xx411c7mD

# With authentication
vget --bilibili-cookie "SESSDATA=xxx" https://www.bilibili.com/video/BV1xx411c7mD

# Download specific pages (multi-part video)
vget -p 1-5 https://www.bilibili.com/video/BV1xx411c7mD

# Download anime series
vget https://www.bilibili.com/bangumi/play/ss12345
```

### Config Integration

```yaml
# ~/.config/vget/config.yml
bilibili:
  cookie: "SESSDATA=xxx; bili_jct=xxx"
  quality: "1080p"
  audio_quality: "320kbps"
  download_subtitle: true
  download_danmaku: false
  prefer_hevc: true
```

## Dependencies

| Package                      | Purpose                      |
| ---------------------------- | ---------------------------- |
| `google.golang.org/protobuf` | Protobuf for APP API         |
| `google.golang.org/grpc`     | gRPC client for APP API      |
| `github.com/skip2/go-qrcode` | QR code generation for login |

## Estimated Scope

| Phase                    | Go Lines (Est.) | Priority |
| ------------------------ | --------------- | -------- |
| Phase 1: Foundation      | ~1,500          | High     |
| Phase 2: Authentication  | ~400            | High     |
| Phase 3: Extended Types  | ~800            | Medium   |
| Phase 4: APP API         | ~1,200          | Medium   |
| Phase 5: Post-Processing | ~600            | Low      |
| **Total**                | **~4,500**      | -        |

## Implementation Order

1. **bilibili.go** - Extractor interface, URL matching, ID conversion
2. **bilibili_api.go** - WEB API client, WBI signature
3. **bilibili_parser.go** - Stream extraction from API responses
4. **bilibili_fetcher_normal.go** - Regular video support
5. **bilibili_auth.go** - Cookie parsing, token management
6. **bilibili_fetcher_bangumi.go** - Anime support
7. **bilibili_fetcher_cheese.go** - Course support
8. **bilibili_proto/** - Protobuf definitions (copy from BBDown)
9. **bilibili_api_app.go** - APP API for advanced quality
10. **bilibili_subtitle.go** - Subtitle download/conversion
11. **bilibili_danmaku.go** - Danmaku support

## References

- [BBDown Source](https://github.com/nilaoda/BBDown)
- [Bilibili API Documentation](https://github.com/SocialSisterYi/bilibili-API-collect) (unofficial)
- [BV/AV Conversion Algorithm](https://www.zhihu.com/question/381784377)
- [WBI Signature Mechanism](https://github.com/SocialSisterYi/bilibili-API-collect/blob/master/docs/misc/sign/wbi.md)
