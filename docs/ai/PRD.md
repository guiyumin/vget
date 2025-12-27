# PRD: AI Transcription & Summarization MVP

## Overview

Add `vget ai` command for local audio transcription and summarization.

**MVP Scope:** Local MP3 → Chunk → Transcribe → Summarize

## User Story

```bash
# User has a downloaded podcast
vget ai podcast.mp3 --transcribe --summarize

# Output:
# → podcast.transcript.md (full transcript with timestamps)
# → podcast.summary.md (LLM-generated summary)
```

## MVP Constraints

| Constraint | Decision |
|------------|----------|
| Input | Local audio files only (no URL download) |
| Transcription | OpenAI Whisper API only (cloud, simple) |
| Summarization | OpenAI GPT-4o only (cloud, simple) |
| Chunking | Fixed 10-min chunks with 10s overlap |
| Output | Markdown files only (no SRT for MVP) |
| Config | TUI wizard (`vget ai config`) + `vget config set` |

## CLI Interface

```bash
# Configure via TUI wizard (recommended)
vget ai config       # Primary - creates account with encrypted API key
vget config ai       # Alias (same wizard)

# Run with password prompt
vget ai podcast.mp3 --transcribe
# Enter PIN: ****

# Run with password flag (for scripting)
vget ai podcast.mp3 --transcribe --password 1234

# Use specific account (if multiple configured)
vget ai podcast.mp3 --transcribe --account work --password 1234

# List configured accounts
vget ai accounts

# Delete an account
vget ai accounts delete my_openai
```

## Security: Encrypted API Keys

**API keys are NEVER stored in plain text.** They are encrypted using AES-256-GCM with a key derived from a 4-digit PIN.

### Why 4-digit PIN?

| Approach | Pros | Cons |
|----------|------|------|
| No encryption | Simple | Keys exposed in plain text |
| Full password | Most secure | Inconvenient for frequent use |
| **4-digit PIN** | **Balance of security + UX** | **Limited entropy (10k combinations)** |

The 4-digit PIN provides:
- Protection against casual file access
- Quick to type for frequent operations
- Not meant to protect against determined attackers with file access

### Encryption Flow

```
User enters: API key + 4-digit PIN
     ↓
Derive key: PBKDF2(PIN, salt, 100000 iterations) → AES key
     ↓
Encrypt: AES-256-GCM(API key, AES key) → ciphertext
     ↓
Store: base64(salt + nonce + ciphertext) in config.yml
```

### Runtime Flow

```
User runs: vget ai podcast.mp3 --transcribe --password 1234
     ↓
Load: encrypted API key from config
     ↓
Derive key: PBKDF2(PIN, salt, 100000) → AES key
     ↓
Decrypt: AES-256-GCM(ciphertext, AES key) → API key
     ↓
Use: API key for OpenAI request
     ↓
Clear: API key from memory after use
```

## Multi-Account Support

Users can configure multiple AI accounts with aliases:

```bash
# Add accounts via wizard
vget ai config
# → Creates account with alias (e.g., "personal", "work")

# List accounts
vget ai accounts
# personal (openai) - default
# work (openai)

# Use specific account
vget ai podcast.mp3 --transcribe --account work --password 1234

# Set default account
vget ai accounts default work
```

## TUI Wizard (`vget ai config`)

Multi-step wizard for creating/editing AI accounts:

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  ██╗   ██╗ ██████╗ ███████╗████████╗     █████╗ ██╗        │
│  ██║   ██║██╔════╝ ██╔════╝╚══██╔══╝    ██╔══██╗██║        │
│  ██║   ██║██║  ███╗█████╗     ██║       ███████║██║        │
│  ╚██╗ ██╔╝██║   ██║██╔══╝     ██║       ██╔══██║██║        │
│   ╚████╔╝ ╚██████╔╝███████╗   ██║       ██║  ██║██║        │
│    ╚═══╝   ╚═════╝ ╚══════╝   ╚═╝       ╚═╝  ╚═╝╚═╝        │
│                                                             │
│  Step 1/6                                                   │
│                                                             │
│  Account Name                                               │
│  Enter a name for this AI account (e.g., personal, work)    │
│                                                             │
│  > personal_                                                │
│                                                             │
│  ← Back • → Next • enter Confirm • esc Quit                 │
└─────────────────────────────────────────────────────────────┘
```

### Wizard Steps

| Step | Title | Type | Description |
|------|-------|------|-------------|
| 1 | Account Name | Input | Alias for this account (e.g., "personal") |
| 2 | Provider | Select | OpenAI, Skip |
| 3 | Transcription API Key | Input | API key for Whisper |
| 4 | Summarization API Key | Input | Same key or different |
| 5 | 4-Digit PIN | Input | PIN to encrypt API keys |
| 6 | Review & Save | Confirm | Yes/No |

### Wizard Flow

```
Step 1: Account Name
  └─> Enter alias (e.g., "personal")

Step 2: Provider
  ├─> OpenAI → Step 3
  └─> Skip → Exit (no account created)

Step 3: Transcription API Key
  └─> Enter API key (masked input)

Step 4: Summarization API Key
  ├─> "Use same key" → Step 5
  └─> Enter different key → Step 5

Step 5: 4-Digit PIN
  └─> Enter PIN (masked, e.g., ****) + confirm

Step 6: Review & Save
  ├─> Yes → Encrypt keys, save to config.yml
  └─> No → Cancel
```

### PIN Requirements

- Exactly 4 digits (0-9)
- Must confirm by entering twice
- Used to encrypt/decrypt ALL API keys in this account
- Required every time you run AI commands

## Output Files

```
podcast.mp3 --transcribe
  → podcast.transcript.md

podcast.mp3 --transcribe --summarize
  → podcast.transcript.md
  → podcast.summary.md
```

### Transcript Format (podcast.transcript.md)

```markdown
# Transcript: podcast.mp3

**Duration:** 1h 23m 45s
**Transcribed:** 2024-01-15 10:30:00
**Provider:** openai/whisper-1

---

[00:00:00] Welcome to today's episode. We're going to discuss...

[00:00:15] The main topic is about building reliable systems...

[00:05:30] Let me give you an example of how this works in practice...
```

### Summary Format (podcast.summary.md)

```markdown
# Summary: podcast.mp3

**Source:** podcast.transcript.md
**Summarized:** 2024-01-15 10:35:00
**Provider:** openai/gpt-4o

---

## Key Points

- Point 1: ...
- Point 2: ...
- Point 3: ...

## Summary

[2-3 paragraph summary of the content]
```

## Technical Architecture

### Package Structure

```
internal/core/ai/
├── ai.go                 # Main entry point, orchestrator
├── chunker.go            # Audio chunking with ffmpeg
├── transcriber/
│   ├── transcriber.go    # Interface
│   └── openai.go         # OpenAI Whisper implementation
├── summarizer/
│   ├── summarizer.go     # Interface
│   └── openai.go         # OpenAI GPT implementation
└── output/
    └── markdown.go       # Markdown file generation
```

### Config Schema

```go
// Add to internal/core/config/config.go
type AIConfig struct {
    // Multiple accounts with aliases
    Accounts       map[string]AIAccount `yaml:"accounts,omitempty"`
    DefaultAccount string               `yaml:"default_account,omitempty"`
}

type AIAccount struct {
    // Provider for this account (openai, anthropic, etc.)
    Provider string `yaml:"provider,omitempty"`

    // Transcription settings
    Transcription AIServiceConfig `yaml:"transcription,omitempty"`

    // Summarization settings
    Summarization AIServiceConfig `yaml:"summarization,omitempty"`
}

type AIServiceConfig struct {
    // Model to use (e.g., "whisper-1", "gpt-4o")
    Model string `yaml:"model,omitempty"`

    // Encrypted API key (base64 encoded: salt + nonce + ciphertext)
    // NEVER store plain text API keys
    APIKeyEncrypted string `yaml:"api_key_encrypted,omitempty"`

    // Optional custom base URL
    BaseURL string `yaml:"base_url,omitempty"`
}
```

### Config File Example

```yaml
# ~/.config/vget/config.yml
ai:
  default_account: personal
  accounts:
    personal:
      provider: openai
      transcription:
        model: whisper-1
        api_key_encrypted: "YWJjZGVm..."  # AES-256-GCM encrypted
      summarization:
        model: gpt-4o
        api_key_encrypted: "YWJjZGVm..."  # Can be same or different key
    work:
      provider: openai
      transcription:
        model: whisper-1
        api_key_encrypted: "eHl6MTIz..."
      summarization:
        model: gpt-4o-mini
        api_key_encrypted: "eHl6MTIz..."
```

### Interfaces

```go
// internal/core/ai/transcriber/transcriber.go
type TranscriptionResult struct {
    Text     string
    Segments []Segment
    Language string
    Duration time.Duration
}

type Segment struct {
    Start time.Duration
    End   time.Duration
    Text  string
}

type Transcriber interface {
    Transcribe(ctx context.Context, audioPath string) (*TranscriptionResult, error)
}

// internal/core/ai/summarizer/summarizer.go
type SummarizationResult struct {
    Summary   string
    KeyPoints []string
}

type Summarizer interface {
    Summarize(ctx context.Context, text string) (*SummarizationResult, error)
}
```

## Processing Pipeline

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Load MP3   │ ──▶ │   Chunk     │ ──▶ │ Transcribe  │ ──▶ │  Summarize  │
│             │     │  (ffmpeg)   │     │  (Whisper)  │     │  (GPT-4o)   │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
                           │                   │                   │
                           ▼                   ▼                   ▼
                    .chunks/ dir        .transcript.md       .summary.md
```

### Step 1: Chunking

- Check if file exceeds Whisper's 25MB limit
- If yes: split into 10-minute chunks with 10s overlap using ffmpeg
- Save chunks to `{filename}.chunks/` directory
- Generate `manifest.json` for resumability

```go
// internal/core/ai/chunker.go
type Chunker struct {
    MaxChunkDuration time.Duration // 10 minutes
    OverlapDuration  time.Duration // 10 seconds
}

func (c *Chunker) NeedsChunking(filePath string) (bool, error)
func (c *Chunker) Split(filePath string) ([]ChunkInfo, error)

type ChunkInfo struct {
    Index    int
    FilePath string
    Start    time.Duration
    End      time.Duration
}
```

### Step 2: Transcription

- Send each chunk to OpenAI Whisper API
- Collect segments with timestamps
- Merge chunks with fuzzy deduplication (handle overlaps)
- Write `{filename}.transcript.md`

### Step 3: Summarization

- Read transcript text
- Send to OpenAI GPT-4o with summarization prompt
- Write `{filename}.summary.md`

## Error Handling

| Error | Behavior |
|-------|----------|
| No API key configured | Show helpful error with config command |
| API rate limit | Retry with exponential backoff (3 attempts) |
| Chunk transcription fails | Mark chunk as failed, continue, warn at end |
| File not found | Clear error message |
| Unsupported format | List supported formats (mp3, m4a, wav, mp4) |

## Progress Display

```
Transcribing podcast.mp3...
  Checking file size... 45MB (needs chunking)
  Splitting into chunks... 5 chunks
  [1/5] Transcribing chunk 1... done (23s)
  [2/5] Transcribing chunk 2... done (21s)
  [3/5] Transcribing chunk 3... done (24s)
  [4/5] Transcribing chunk 4... done (22s)
  [5/5] Transcribing chunk 5... done (18s)
  Merging transcripts... done
  Writing podcast.transcript.md... done

Summarizing...
  Reading transcript... 12,456 words
  Generating summary... done
  Writing podcast.summary.md... done

Complete!
  Transcript: podcast.transcript.md
  Summary:    podcast.summary.md
  Cost:       ~$0.42 (transcription) + ~$0.08 (summary)
```

## Implementation Steps

### Phase 1: Foundation

1. `internal/core/config/config.go` - Add AIConfig structs
2. `internal/cli/config.go` - Add `ai.*` key handling

### Phase 2: TUI Wizard

3. `internal/core/config/ai_wizard.go` - AI config TUI wizard (Bubbletea)
4. `internal/cli/ai.go` - `vget ai` command with `config` subcommand
5. `internal/cli/config.go` - Add `vget config ai` alias

### Phase 3: Core AI Pipeline

6. `internal/core/ai/ai.go` - Main orchestrator
7. `internal/core/ai/chunker.go` - ffmpeg-based audio chunking

### Phase 4: Transcription

8. `internal/core/ai/transcriber/transcriber.go` - Interface
9. `internal/core/ai/transcriber/openai.go` - Whisper API client
10. `internal/core/ai/output/markdown.go` - Transcript formatter

### Phase 5: Summarization

11. `internal/core/ai/summarizer/summarizer.go` - Interface
12. `internal/core/ai/summarizer/openai.go` - GPT-4o client

## Dependencies

```go
// go.mod additions
github.com/sashabaranov/go-openai  // OpenAI API client
```

## ffmpeg Requirement

MVP requires ffmpeg for chunking large files:

```bash
# macOS
brew install ffmpeg

# Ubuntu/Debian
apt install ffmpeg

# If ffmpeg not found + file needs chunking
# → Error: "Large files require ffmpeg. Install: brew install ffmpeg"
```

## i18n Translations

Add AI-related translations to all 7 locales:

```yaml
# internal/core/i18n/locales/en.yml
ai:
  wizard:
    title: "AI Configuration"
    transcription_provider: "Transcription Provider"
    transcription_provider_desc: "Choose an AI provider for speech-to-text"
    transcription_api_key: "Transcription API Key"
    transcription_api_key_desc: "Enter your OpenAI API key"
    summarization_provider: "Summarization Provider"
    summarization_provider_desc: "Choose an AI provider for text summarization"
    summarization_api_key: "Summarization API Key"
    summarization_api_key_desc: "Enter your API key"
    reuse_api_key: "Use same API key as transcription?"
    skip: "Skip"
    openai_whisper: "OpenAI Whisper"
    openai_gpt: "OpenAI GPT"
  progress:
    checking_file: "Checking file size..."
    splitting_chunks: "Splitting into chunks..."
    transcribing_chunk: "Transcribing chunk %d/%d..."
    merging_transcripts: "Merging transcripts..."
    summarizing: "Generating summary..."
    writing_file: "Writing %s..."
    complete: "Complete!"
  errors:
    no_api_key: "No API key configured. Run: vget ai config"
    file_not_found: "File not found: %s"
    ffmpeg_required: "Large files require ffmpeg. Install: brew install ffmpeg"
```

## Future Extensions (Not MVP)

- [ ] Local transcription (Ollama + whisper.cpp)
- [ ] Additional providers (Anthropic, Qwen)
- [ ] URL input (download then process)
- [ ] SRT output format
- [ ] Speaker diarization
- [ ] Resume interrupted processing
- [ ] Chinese cloud providers

## Success Criteria

1. `vget ai config` wizard configures AI providers correctly
2. `vget ai podcast.mp3 --transcribe` produces accurate transcript
3. `vget ai podcast.mp3 --transcribe --summarize` produces transcript + summary
4. Large files (>25MB) are automatically chunked
5. Progress is visible during processing
6. Errors are clear and actionable
