# Plan: AI Transcription & Summarization Feature

## Overview
Add AI-powered transcription and summarization to vget for converting downloaded podcasts/videos to text.

## User Requirements
- **Transcription**: Both cloud APIs (OpenAI, Qwen) AND local (Ollama, whisper.cpp)
- **Summarization**: Interface-based for OpenAI, Anthropic, Ollama, Qwen, and extensible
- **CLI**: Clean separation - `vget <url>` downloads only, `vget ai` does AI
- **Output**: Each step saves its result (transcript → .md/.srt, summary → .md)

---

## CLI Design: Clean Separation

### Principle
```bash
vget <url>                    # Download ONLY. No AI flags. Simple.
vget ai <url-or-file> ...     # AI operations. If URL, download first.
```

**Why?**
- No flag pollution on download command
- `vget ai` is the AI entry point - handles everything AI-related
- If you pass a URL to `vget ai`, it downloads first then processes

### `vget ai` Command
```bash
vget ai <url-or-file> --operation1 --operation2 ...
```

- **Input**: URL (downloads first) or local file path
- **Operations**: Flags like `--transcribe`, `--summarize`, `--translate`, etc.
- **Order matters**: Flags are processed left-to-right as a pipeline
- **Validation**: Each operation validates its input type, helpful errors on mismatch

### Examples
```bash
# From URL (downloads first, then processes)
vget ai https://xiaoyuzhoufm.com/episode/xxx --transcribe --summarize

# From local file
vget ai podcast.mp3 --transcribe --summarize
vget ai podcast.mp3 --transcribe                # transcribe only
vget ai transcript.md --summarize               # summarize existing text

# Future: translation pipeline
vget ai podcast.mp3 --transcribe --translate --summarize
```

### Operation Flags

| Flag | Input Types | Output | Description |
|------|-------------|--------|-------------|
| `--transcribe` | audio, video | text (.transcript.md, .srt for video) | Speech-to-text |
| `--summarize` | text | text (.summary.md) | LLM summarization |
| `--translate` | text | text | (future) Translation |
| `--chapters` | text + timestamps | chapter list | (future) Chapter generation |

### Validation Rules
```bash
# Valid
vget ai podcast.mp3 --transcribe                    # audio → transcribe ✓
vget ai podcast.mp3 --transcribe --summarize        # audio → transcribe → summarize ✓
vget ai notes.md --summarize                        # text → summarize ✓

# Invalid - helpful errors
vget ai podcast.mp3 --summarize
# Error: --summarize requires text input, but got audio file.
# Hint: Add --transcribe first: vget ai podcast.mp3 --transcribe --summarize

vget ai notes.md --transcribe
# Error: --transcribe requires audio/video input, but got text file.
```

### Output Files
Each operation saves its result:
```
podcast.mp3 --transcribe
  → podcast.transcript.md
  → podcast.srt (if video)

podcast.mp3 --transcribe --summarize
  → podcast.transcript.md
  → podcast.srt (if video)
  → podcast.summary.md
```

---

## Package Structure

```
internal/core/ai/
├── pipeline.go           # Pipeline executor, validates operation chain
├── operation.go          # Operation interface and registry
├── operations/
│   ├── transcribe.go     # --transcribe operation
│   ├── summarize.go      # --summarize operation
│   └── translate.go      # --translate operation (future)
├── provider/
│   ├── transcriber.go    # Transcriber interface
│   ├── summarizer.go     # Summarizer interface
│   └── translator.go     # Translator interface (future)
├── transcriber/
│   ├── registry.go       # Provider factory pattern
│   ├── openai.go         # OpenAI Whisper API
│   ├── ollama.go         # Ollama + whisper.cpp local
│   └── qwen.go           # Qwen audio model
├── summarizer/
│   ├── registry.go       # Provider factory pattern
│   ├── openai.go         # GPT-4o summarization
│   ├── anthropic.go      # Claude summarization
│   └── ollama.go         # Local LLM summarization
└── output/
    ├── srt.go            # SRT subtitle generator
    └── markdown.go       # Markdown generator
```

---

## Core Interfaces

### Operation Interface (`internal/core/ai/operation.go`)
```go
type MediaType string

const (
    MediaTypeAudio MediaType = "audio"
    MediaTypeVideo MediaType = "video"
    MediaTypeText  MediaType = "text"
)

// Operation represents a processing step in the pipeline
type Operation interface {
    Name() string                          // "transcribe", "summarize", etc.
    AcceptedInputTypes() []MediaType       // What input types this operation accepts
    OutputType() MediaType                 // What this operation produces
    Execute(ctx context.Context, input *PipelineData) (*PipelineData, error)
}

// PipelineData flows between operations
type PipelineData struct {
    FilePath    string              // Original or intermediate file path
    MediaType   MediaType           // Current data type
    Text        string              // Text content (for text type)
    Segments    []TranscriptSegment // Timestamped segments (from transcription)
    Metadata    map[string]any      // Additional metadata
}
```

### Provider Interfaces (`internal/core/ai/provider/`)
```go
// transcriber.go
type TranscriptionResult struct {
    Text     string
    Segments []TranscriptSegment
    Language string
    Duration time.Duration
}

type Transcriber interface {
    Name() string
    Transcribe(ctx context.Context, filePath string, opts TranscriptionOptions) (*TranscriptionResult, error)
    SupportedFormats() []string
}

// summarizer.go
type SummarizationResult struct {
    Summary   string
    KeyPoints []string
}

type Summarizer interface {
    Name() string
    Summarize(ctx context.Context, text string, opts SummarizationOptions) (*SummarizationResult, error)
}
```

---

## Security: Encrypted API Keys

**API keys are NEVER stored in plain text.** They are encrypted using AES-256-GCM.

### Encryption Scheme

- **Encryption**: AES-256-GCM (authenticated encryption)
- **Key derivation**: PBKDF2 with 100,000 iterations
- **User secret**: 4-digit PIN (balance of security and convenience)
- **Storage format**: base64(salt + nonce + ciphertext)

### Why 4-digit PIN?

- Quick to type for frequent operations
- Protects against casual file access
- Not designed for high-security scenarios
- Users who need more security should use environment variables

### Usage

```bash
# Configure with TUI wizard (creates encrypted keys)
vget ai config

# Run with password prompt
vget ai podcast.mp3 --transcribe
# Enter PIN: ****

# Run with password flag (scripting)
vget ai podcast.mp3 --transcribe --password 1234

# Use specific account
vget ai podcast.mp3 --transcribe --account work --password 1234
```

---

## Multi-Account Support

Users can configure multiple AI accounts with aliases:

```bash
# List accounts
vget ai accounts
# personal (openai) - default
# work (openai)

# Use specific account
vget ai podcast.mp3 --transcribe --account work

# Set default account
vget ai accounts default work

# Delete account
vget ai accounts delete old_account
```

---

## Config Schema (`internal/core/config/config.go`)

```go
type AIConfig struct {
    // Multiple accounts with aliases
    Accounts       map[string]AIAccount `yaml:"accounts,omitempty"`
    DefaultAccount string               `yaml:"default_account,omitempty"`
}

type AIAccount struct {
    Provider      string          `yaml:"provider,omitempty"`
    Transcription AIServiceConfig `yaml:"transcription,omitempty"`
    Summarization AIServiceConfig `yaml:"summarization,omitempty"`
}

type AIServiceConfig struct {
    Model           string `yaml:"model,omitempty"`
    APIKeyEncrypted string `yaml:"api_key_encrypted,omitempty"` // AES-256-GCM encrypted
    BaseURL         string `yaml:"base_url,omitempty"`
}
```

**Config file example:**
```yaml
ai:
  default_account: personal
  accounts:
    personal:
      provider: openai
      transcription:
        model: whisper-1
        api_key_encrypted: "YWJjZGVm..."
      summarization:
        model: gpt-4o
        api_key_encrypted: "YWJjZGVm..."
    work:
      provider: openai
      transcription:
        model: whisper-1
        api_key_encrypted: "eHl6MTIz..."
```

---

## Docker Server UI (`ui/`)

The web UI needs corresponding AI features:

### UI Components
- AI settings panel (provider selection, API keys)
- Operation checkboxes on download form
- Progress indicators for each pipeline step
- Result viewer for transcripts/summaries

### API Endpoints
```
POST /api/ai/process
  body: { file: string, operations: ["transcribe", "summarize"] }

GET /api/ai/status/:jobId
  response: { status: "processing", step: "transcribe", progress: 45 }

GET /api/ai/result/:jobId
  response: { transcript: "...", summary: "...", files: [...] }
```

### UI Flow
1. User downloads media (or selects existing file)
2. Checkboxes: ☑ Transcribe ☑ Summarize
3. Click "Process" → shows progress for each step
4. Results displayed with download links for generated files

---

## Implementation Steps

### Step 1: Core Infrastructure
1. Create `internal/core/ai/operation.go` - Operation interface
2. Create `internal/core/ai/pipeline.go` - Pipeline executor with validation
3. Create `internal/core/ai/provider/transcriber.go` - Transcriber interface
4. Create `internal/core/ai/provider/summarizer.go` - Summarizer interface
5. Add `AIConfig` to `internal/core/config/config.go`
6. Add config CLI keys to `internal/cli/config.go`

### Step 2: Output Formatters
1. Create `internal/core/ai/output/srt.go`
2. Create `internal/core/ai/output/markdown.go`

### Step 3: Operations
1. Create `internal/core/ai/operations/transcribe.go`
2. Create `internal/core/ai/operations/summarize.go`

### Step 4: OpenAI Provider (MVP)
1. Implement `transcriber/openai.go` (Whisper API)
2. Implement `summarizer/openai.go` (GPT-4o)

### Step 5: CLI Integration
1. Create `internal/cli/ai.go` - `vget ai` command
2. Create `internal/cli/ai_init.go` - `vget ai init` TUI wizard
3. Add i18n translations (7 languages)

### Step 6: Additional Providers
1. `summarizer/anthropic.go` (Claude)
2. `transcriber/ollama.go` + `summarizer/ollama.go`
3. `transcriber/qwen.go`

### Step 7: Docker Server UI
1. Add AI settings component
2. Add AI processing UI on download page
3. Add API endpoints for AI processing
4. Add result viewer

---

## Files to Create

| File | Purpose |
|------|---------|
| `internal/core/ai/operation.go` | Operation interface, registry |
| `internal/core/ai/pipeline.go` | Pipeline executor, validation |
| `internal/core/ai/provider/transcriber.go` | Transcriber interface |
| `internal/core/ai/provider/summarizer.go` | Summarizer interface |
| `internal/core/ai/operations/transcribe.go` | Transcribe operation |
| `internal/core/ai/operations/summarize.go` | Summarize operation |
| `internal/core/ai/transcriber/registry.go` | Provider factory |
| `internal/core/ai/transcriber/openai.go` | OpenAI Whisper |
| `internal/core/ai/transcriber/ollama.go` | Ollama/whisper.cpp |
| `internal/core/ai/transcriber/qwen.go` | Qwen audio (via Aliyun API) |
| `internal/core/ai/transcriber/volcengine.go` | 火山引擎 Volcengine |
| `internal/core/ai/transcriber/baidu.go` | 百度智能云 |
| `internal/core/ai/summarizer/registry.go` | Provider factory |
| `internal/core/ai/summarizer/openai.go` | GPT summarization |
| `internal/core/ai/summarizer/anthropic.go` | Claude summarization |
| `internal/core/ai/summarizer/ollama.go` | Local LLM |
| `internal/core/ai/summarizer/cli.go` | CLI tools (claude, gemini, codex, etc.) |
| `internal/core/ai/summarizer/router.go` | AI Routers (OpenRouter, MuleRouter) |
| `internal/core/ai/summarizer/aliyun.go` | 阿里云 Qwen |
| `internal/core/ai/summarizer/volcengine.go` | 火山引擎 Doubao |
| `internal/core/ai/summarizer/baidu.go` | 百度 ERNIE |
| `internal/core/ai/output/srt.go` | SRT formatter |
| `internal/core/ai/output/markdown.go` | Markdown formatter |
| `internal/cli/ai.go` | `vget ai` command |
| `internal/cli/ai_init.go` | `vget ai init` / `vget init ai` TUI wizard (aliases) |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/core/config/config.go` | Add `AIConfig` structs |
| `internal/cli/config.go` | Add `ai.*` key handling |
| `internal/cli/init.go` | Add `ai` subcommand to support `vget init ai` |
| `internal/core/i18n/i18n.go` | Add `AITranslations` struct |
| `internal/core/i18n/locales/*.yml` | Add `ai:` section (7 languages) |

---

## Chunking Strategy for Large Files

### Problem
- A 4-hour podcast = ~230MB MP3
- Most APIs have file size and duration limits
- Need to chunk large files, then merge results

### Transcription Pricing Comparison

| Provider | Model | Price | 4-hour podcast cost | Notes |
|----------|-------|-------|---------------------|-------|
| **Qwen** | qwen3-asr-flash | **$0.000035/sec** | **~$0.50** | Best for Chinese, WER 3.97% |
| OpenAI | whisper-1 | $0.006/min | ~$1.44 | 99+ languages, 25MB limit |
| Ollama | whisper (local) | **FREE** | $0 | Requires local setup |

*Qwen3-ASR-Flash is ~10x cheaper than OpenAI Whisper for the same content.*

### Video Transcription: Extract Audio First!

**IMPORTANT:** Video-to-text APIs are much more expensive than audio-to-text.

Always extract audio from video before transcription:
```bash
# ffmpeg extracts audio (fast, no re-encoding)
ffmpeg -i video.mp4 -vn -acodec copy audio.m4a

# Or convert to mp3 (smaller file)
ffmpeg -i video.mp4 -vn -ar 16000 -ac 1 -ab 64k audio.mp3
```

**Cost comparison for 1-hour video:**
| Approach | Cost |
|----------|------|
| Video → multimodal API (GPT-4o, Gemini) | $$$$ (tokens for frames) |
| Video → extract audio → transcription API | ~$0.13 (Qwen) or ~$0.36 (OpenAI) |

The pipeline should automatically extract audio from video files before sending to transcription.

### Model Limits (TO BE RESEARCHED)

| Provider | Model | Max Audio Size | Max Audio Duration | Max Text Tokens |
|----------|-------|----------------|--------------------|-----------------|
| OpenAI | whisper-1 | 25MB | ? | - |
| OpenAI | gpt-4o | - | - | 128k |
| OpenAI | gpt-4o-mini | - | - | 128k |
| Anthropic | claude-sonnet | - | - | 200k |
| Anthropic | claude-haiku | - | - | 200k |
| Qwen | qwen3-asr-flash | ? | 3min* | - |
| Ollama | whisper (local) | unlimited? | memory-bound | - |
| Ollama | llama3 (local) | - | - | 8k-128k (varies) |

### Transcription Language Support

vget supports 7 languages: **en, zh, jp, kr, es, fr, de**

| Provider | en | zh | jp | kr | es | fr | de | Notes |
|----------|:--:|:--:|:--:|:--:|:--:|:--:|:--:|-------|
| OpenAI Whisper | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | 99+ languages |
| Qwen3-ASR-Flash | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | 11 languages, best for zh (API-only) |
| Ollama/whisper | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | Same as OpenAI |
| Deepgram | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | 30+ languages |

All major transcription providers support vget's 7 languages.

### AI Routers (unified access to multiple providers)

Most routers use OpenAI-compatible APIs, so we support them generically:

| Router | Models | Markup | Self-host | Best for |
|--------|--------|--------|-----------|----------|
| **OpenRouter** | 300+ | 5% | No | Most popular, easy |
| **LiteLLM** | 100+ | FREE | **Yes** | Self-hosters |
| **Groq** | Limited | Low | No | Speed (LPU hardware) |
| **Gatewayz** | 10,000+ | ? | No | Most models |
| **MuleRouter** | Qwen | ? | No | Qwen + video |

**Config (generic - works with any OpenAI-compatible router):**
```yaml
ai:
  summarization:
    provider: router          # Generic router provider
    base_url: https://openrouter.ai/api/v1  # or any router
    api_key: sk-xxx
    model: anthropic/claude-sonnet-4
```

**Preset shortcuts:**
```yaml
ai:
  summarization:
    provider: openrouter      # Shortcut: auto-sets base_url
    api_key: sk-xxx
    model: google/gemini-2.5-flash  # FREE on OpenRouter!
```

**Implementation:**
```go
// internal/core/ai/summarizer/router.go
// One implementation handles ALL OpenAI-compatible routers
type RouterSummarizer struct {
    apiKey  string
    model   string
    baseURL string  // Any OpenAI-compatible endpoint
}

// Preset base URLs
var routerPresets = map[string]string{
    "openrouter": "https://openrouter.ai/api/v1",
    "groq":       "https://api.groq.com/openai/v1",
    "gatewayz":   "https://api.gatewayz.ai/v1",
    "mulerouter": "https://api.mulerouter.ai/v1",
    // Users can also specify custom base_url for LiteLLM, etc.
}
```

**TUI shows:**
```
# │    ── AI Routers (OpenAI-compatible) ──            │
# │      openrouter          OpenRouter (300+ models)   │
# │      groq                Groq (fast inference)      │
# │      gatewayz            Gatewayz (10k+ models)     │
# │      mulerouter          MuleRouter (Qwen focus)    │
# │      custom              Custom base_url...         │
```

### Chinese Cloud Providers (中国云服务商)

For users in China who need domestic providers:

| Provider | Transcription (ASR) | LLM | Pricing | Notes |
|----------|---------------------|-----|---------|-------|
| **阿里云 (Aliyun)** | 智能语音交互 | 通义千问 Qwen | ¥0.058/分钟 | 新用户3个月免费试用 |
| **火山引擎 (Volcengine)** | 豆包语音 | 豆包 Doubao | 阶梯计费 | 字节跳动旗下 |
| **百度智能云** | 百度语音 | 文心一言 ERNIE | 60秒内免费 | 免费额度多 |
| **腾讯云** | 腾讯语音 | 混元 Hunyuan | 按量计费 | ASR延迟低 |
| **科大讯飞** | 讯飞语音 | 讯飞星火 Spark | 在线免费 | 中文识别最强 |

**Config:**
```yaml
ai:
  transcription:
    provider: aliyun          # 阿里云
    api_key: xxx
    model: paraformer-v2

  summarization:
    provider: volcengine      # 火山引擎
    api_key: xxx
    model: doubao-pro
```

**TUI shows:**
```
# │    ── 中国云服务 (China Cloud) ──                  │
# │      aliyun              阿里云 (Qwen)              │
# │      volcengine          火山引擎 (豆包)            │
# │      baidu               百度智能云 (文心)          │
# │      tencent             腾讯云 (混元)              │
# │      iflytek             科大讯飞 (星火)            │
```

**Implementation files:**
- `internal/core/ai/transcriber/aliyun.go`
- `internal/core/ai/transcriber/volcengine.go`
- `internal/core/ai/summarizer/aliyun.go`
- `internal/core/ai/summarizer/volcengine.go`
- etc.

### CLI Tools as Providers (use existing installations)

Users may have AI CLI tools already installed and authenticated. We can use them as providers:

| CLI Tool | Command | Text | Image | Audio | Video | Notes |
|----------|---------|------|-------|-------|-------|-------|
| Claude Code | `claude` | ✓ | ✓ | ✗ | ✗ | Uses existing Anthropic auth |
| Codex | `codex` | ✓ | ? | ✗ | ✗ | Uses existing OpenAI auth |
| Gemini CLI | `gemini` | ✓ | ✓ | ? | ? | **FREE** (2.5 Flash included) |
| Mistral | `mistral` | ✓ | ? | ✗ | ✗ | Uses existing Mistral auth |
| OpenCode | `opencode` | ✓ | ? | ✗ | ✗ | Multi-provider |

**Benefits:**
- No API key configuration needed
- Uses existing CLI authentication
- May be included in user's existing subscription

**Usage:**
```yaml
ai:
  summarization:
    provider: cli/claude      # Use claude CLI
    # No API key needed - uses existing CLI auth
```

**Implementation:**
```go
// internal/core/ai/summarizer/cli.go
type CLISummarizer struct {
    command string  // "claude", "gemini", etc.
}

func (s *CLISummarizer) Summarize(ctx context.Context, text string, opts SummarizationOptions) (*SummarizationResult, error) {
    // Pipe text to CLI, capture output
    cmd := exec.CommandContext(ctx, s.command, "--prompt", "Summarize this text...")
    cmd.Stdin = strings.NewReader(text)
    output, err := cmd.Output()
    // ...
}
```

**CLI Invocation Reference:**

| CLI | Command | Output Format |
|-----|---------|---------------|
| Claude Code | `claude -p "prompt" --model haiku --output-format json` | `{"result": "..."}` |
| Codex | `codex exec "prompt" --json` | JSONL stream, extract `agent_message.text` |
| Gemini | `gemini "prompt" --output-format json` | `{"response": "..."}` |
| Ollama | `ollama run llama3.2:3b "prompt"` | Plain text |

### Prompt-Based JSON Extraction (Recommended)

When you need structured data from AI (e.g., parsed events, extracted metadata), use a **prompt-based approach** rather than CLI-specific flags like `--json-schema`:

**Approach:**
1. **Prompt defines the schema** - Tell AI exactly what JSON structure to return
2. **Strip markdown fences** - AI often wraps JSON in \`\`\`json...\`\`\`, so strip it
3. **Parse and validate** - Unmarshal JSON into Go struct

**Why this approach?**

| Approach | Pros | Cons |
|----------|------|------|
| **Prompt + strip fences** | Portable across all AI CLIs, won't break if CLI changes, we control the schema | Must handle markdown wrapping |
| **CLI `--json-schema`** | Cleaner output | CLI-specific (Claude only), may change, ties us to CLI internals |

The prompt-based approach is more stable because:
- We control the prompt and parsing logic
- Works identically across Claude, Codex, Gemini, Ollama
- If CLI flags change tomorrow, our code still works
- Simple function handles all edge cases

**Implementation:**
```go
// Prompt tells AI what JSON to return
prompt := `Extract metadata. Respond with ONLY JSON (no markdown):
{"title":"...", "duration":"...", "language":"..."}`

response, _ := callAI(prompt)

// Strip markdown fences (AI's habit of formatting code)
response = stripMarkdownCodeFences(response)

// Parse into struct
var metadata Metadata
json.Unmarshal([]byte(response), &metadata)

// stripMarkdownCodeFences removes ```json ... ``` wrappers
func stripMarkdownCodeFences(s string) string {
    s = strings.TrimSpace(s)
    if strings.HasPrefix(s, "```") {
        if idx := strings.Index(s, "\n"); idx != -1 {
            s = s[idx+1:]
        }
    }
    if strings.HasSuffix(s, "```") {
        s = s[:len(s)-3]
    }
    return strings.TrimSpace(s)
}
```

**TODO: Research and fill in exact limits for each provider/model**

### Chunking Rules

1. **Check file size vs model limit**
   - If file < model's limit → no chunking needed, direct upload
   - If file > model's limit → must chunk

2. **Chunk size depends on chosen model**
   - Use 80% of model's limit as chunk target (safety margin)
   - Example: OpenAI 25MB limit → chunk at 20MB

3. **ffmpeg requirement**
   - Only needed when chunking is required
   - CLI without ffmpeg + large file → error with helpful message
   - Docker always has ffmpeg

### Chunking Strategy: Overlap (Default)

Like DNA shotgun sequencing - overlapping reads ensure complete coverage and accurate assembly.

```
Chunk 1:  [====================]
Chunk 2:              [=====|overlap|===================]
Chunk 3:                              [=====|overlap|===================]
```

**ffmpeg commands:**
```bash
# 10-minute chunks with 10-second overlap
# Chunk 1: 0:00 - 10:00
ffmpeg -i input.mp3 -ss 0 -t 600 -c copy chunk_01.mp3

# Chunk 2: 9:50 - 20:00 (10s overlap)
ffmpeg -i input.mp3 -ss 590 -t 610 -c copy chunk_02.mp3

# Chunk 3: 19:50 - 30:00 (10s overlap)
ffmpeg -i input.mp3 -ss 1190 -t 610 -c copy chunk_03.mp3
```

**Why overlap works:**
- Simple, predictable chunk sizes
- No silence detection complexity
- Overlap ensures no sentence is cut off
- Same technique used in DNA sequencing (shotgun sequencing)

**Transcript deduplication when merging:**
```
Chunk 1 ends:   "...and that's why the market crashed. So"
Chunk 2 starts: "crashed. So the next day, investors..."

→ Match overlapping text
→ Merged: "...and that's why the market crashed. So the next day, investors..."
```

**Algorithm for deduplication:**
1. Take last N words from chunk 1 (N = ~20-50 words, based on overlap duration)
2. Find matching sequence at start of chunk 2
3. Merge at the match point, avoiding duplicates

### Alternative: Silence Detection

For cases where cleaner breaks are preferred (optional, not default):

```bash
# Detect silence periods
ffmpeg -i input.mp3 -af silencedetect=n=-30dB:d=0.5 -f null - 2>&1 | grep silence_end
```

Split at detected silence points for natural breaks. More complex but no deduplication needed.

### Chunker Interface

```go
type ChunkStrategy string

const (
    ChunkStrategyOverlap  ChunkStrategy = "overlap"  // Default: fixed size with overlap
    ChunkStrategySilence  ChunkStrategy = "silence"  // Alternative: split at silence
)

type ChunkerConfig struct {
    Strategy       ChunkStrategy
    MaxChunkSize   int64         // bytes, from model config
    MaxChunkDur    time.Duration // duration, from model config
    OverlapDur     time.Duration // overlap duration, default 10s
}

type Chunker interface {
    NeedsChunking(filePath string, modelLimit int64) bool
    Split(filePath string, cfg ChunkerConfig) ([]Chunk, error)
    Merge(chunks []TranscriptionResult) (*TranscriptionResult, error)
}
```

### Chunk Storage

Save chunks for resumability and re-transcription:

```
podcast.mp3
podcast.chunks/
├── manifest.json      # Chunk metadata
├── chunk_001.mp3      # 0:00 - 10:00
├── chunk_001.txt      # Transcript (after transcription)
├── chunk_002.mp3      # 9:55 - 20:00
├── chunk_002.txt
├── chunk_003.mp3
├── chunk_003.txt
└── ...
```

**manifest.json:**
```json
{
  "source": "podcast.mp3",
  "source_hash": "sha256:abc123...",
  "created_at": "2024-01-15T10:30:00Z",
  "strategy": "overlap",
  "overlap_seconds": 10,
  "chunk_duration_seconds": 600,
  "chunks": [
    {"index": 1, "file": "chunk_001.mp3", "start": 0, "end": 600, "status": "transcribed"},
    {"index": 2, "file": "chunk_002.mp3", "start": 590, "end": 1200, "status": "transcribed"},
    {"index": 3, "file": "chunk_003.mp3", "start": 1190, "end": 1800, "status": "pending"}
  ]
}
```

**Benefits:**
- Retry single chunk without re-chunking entire file
- Re-transcribe with different model (reuse chunks)
- Resume after interruption
- Debug problematic chunks manually
- Source hash ensures chunks match original file

### Resumability

```go
type ChunkManifest struct {
    Source       string    `json:"source"`
    SourceHash   string    `json:"source_hash"`
    CreatedAt    time.Time `json:"created_at"`
    Strategy     string    `json:"strategy"`
    OverlapSecs  int       `json:"overlap_seconds"`
    ChunkDurSecs int       `json:"chunk_duration_seconds"`
    Chunks       []ChunkInfo `json:"chunks"`
}

type ChunkInfo struct {
    Index    int           `json:"index"`
    File     string        `json:"file"`
    Start    float64       `json:"start"`
    End      float64       `json:"end"`
    Status   string        `json:"status"`  // pending, transcribed, failed
    Transcript string      `json:"transcript,omitempty"`  // or path to .txt
}

func (m *ChunkManifest) Save(dir string) error
func LoadManifest(dir string) (*ChunkManifest, error)
func (m *ChunkManifest) PendingChunks() []ChunkInfo
func (m *ChunkManifest) FailedChunks() []ChunkInfo
```

### Cost Estimation

Before starting, show user:
```
File: podcast.mp3 (230MB, 4h 12m)
Model: openai/whisper-1 (25MB limit)
Chunks: 10 chunks required
Estimated cost: ~$0.36 (4.2 hours × $0.006/min)

Proceed? [y/N]
```

---

## Example Usage

```bash
# Configure via TUI wizard (one-time)
# Both commands work (aliases):
vget ai init
vget init ai

# Multi-step wizard (same as vget init pattern):
# - Reads existing config as defaults
# - Skip any operation you don't need
# - Navigate back/forward to change previous steps
#
# Step 1: Transcription Provider
# ┌─────────────────────────────────────────────────────┐
# │  AI Configuration (1/4)              [←] [→] [Esc]  │
# ├─────────────────────────────────────────────────────┤
# │                                                     │
# │  Select Transcription Provider:                     │
# │                                                     │
# │    ── Cloud APIs (pay per use) ──                   │
# │      openai/whisper      Whisper API ($0.006/1M tokens) │
# │    > qwen/qwen3-asr-flash  Qwen3-ASR ($0.000035!) ← CHEAPEST  │
# │      deepgram            Deepgram API               │
# │                                                     │
# │    ── Local (free, requires setup) ──               │
# │      ollama/whisper      Ollama + Whisper model     │
# │      whisper.cpp         Local whisper.cpp          │
# │                                                     │
# │    ── Skip ──                                       │
# │      (none)              Skip transcription config  │
# │                                                     │
# └─────────────────────────────────────────────────────┘
#
# Step 2: Transcription Config (if cloud provider)
# ┌─────────────────────────────────────────────────────┐
# │  AI Configuration (2/4)              [←] [→] [Esc]  │
# ├─────────────────────────────────────────────────────┤
# │                                                     │
# │  OpenAI Whisper Configuration:                      │
# │                                                     │
# │  API Key: sk-xxx____________________________        │
# │                                                     │
# │  Model:                                             │
# │  > whisper-1                                        │
# │                                                     │
# └─────────────────────────────────────────────────────┘
#
# Step 3: Summarization Provider
# ┌─────────────────────────────────────────────────────┐
# │  AI Configuration (3/4)              [←] [→] [Esc]  │
# ├─────────────────────────────────────────────────────┤
# │                                                     │
# │  Select Summarization Provider:                     │
# │                                                     │
# │    ── Cloud APIs (pay per use) ──                   │
# │      openai/gpt-4o       GPT-4o (128k context)      │
# │    > anthropic/claude    Claude Sonnet (200k)       │  ← current config
# │      qwen/turbo          Qwen Turbo                 │
# │                                                     │
# │    ── Local (free, requires setup) ──               │
# │      ollama/llama3       Llama 3 (8B/70B)           │
# │      ollama/qwen         Qwen 2.5 local             │
# │                                                     │
# │    ── AI Routers (OpenAI-compatible) ──            │
# │      openrouter          OpenRouter (300+ models)   │
# │      groq                Groq (fast inference)      │
# │      custom              Custom base_url...         │
# │                                                     │
# │    ── CLI Tools (use existing installation) ──     │
# │      cli/gemini          Gemini CLI (FREE!)         │
# │      cli/claude          Claude Code CLI            │
# │      cli/codex           Codex CLI                  │
# │                                                     │
# │    ── Skip ──                                       │
# │      (none)              Skip summarization config  │
# │                                                     │
# └─────────────────────────────────────────────────────┘
#
# Step 4: Review & Save
# ┌─────────────────────────────────────────────────────┐
# │  AI Configuration (4/4)              [←] [Save]     │
# ├─────────────────────────────────────────────────────┤
# │                                                     │
# │  Review your AI configuration:                      │
# │                                                     │
# │  Transcription:                                     │
# │    Provider: openai/whisper                         │
# │    Model:    whisper-1                              │
# │    API Key:  sk-xxx...xxx (configured)              │
# │                                                     │
# │  Summarization:                                     │
# │    Provider: anthropic/claude                       │
# │    Model:    claude-sonnet-4-20250514               │
# │    API Key:  sk-ant...xxx (configured)              │
# │                                                     │
# │  [Save]  [Back]  [Cancel]                           │
# └─────────────────────────────────────────────────────┘
#
# If user skipped an operation:
# ┌─────────────────────────────────────────────────────┐
# │  Review your AI configuration:                      │
# │                                                     │
# │  Transcription:                                     │
# │    Provider: openai/whisper                         │
# │    Model:    whisper-1                              │
# │    API Key:  sk-xxx...xxx (configured)              │
# │                                                     │
# │  Summarization:                                     │
# │    (not configured)                                 │
# │                                                     │
# └─────────────────────────────────────────────────────┘

# Or use CLI for scripting/Docker:
vget config set ai.transcription.provider openai
vget config set ai.transcription.api_key sk-xxx

# From URL (downloads first, then processes)
vget ai https://xiaoyuzhoufm.com/episode/xxx --transcribe --summarize
# → Downloads podcast.mp3
# → podcast.transcript.md
# → podcast.summary.md

# From local file
vget ai podcast.mp3 --transcribe --summarize

# Transcribe only (no summary)
vget ai podcast.mp3 --transcribe

# Summarize existing transcript
vget ai podcast.transcript.md --summarize

# Video with subtitles
vget ai lecture.mp4 --transcribe --summarize
# → lecture.srt + lecture.transcript.md + lecture.summary.md
```
