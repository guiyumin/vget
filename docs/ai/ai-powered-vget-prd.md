# PRD: AI-Powered vget

## Overview

vget integrates local-first AI capabilities for media processing. The design prioritizes:

- **GPU acceleration** - Fast transcription using Metal (macOS) or CUDA (Windows)
- **Zero runtime dependencies** - Single binary with embedded whisper.cpp
- **Offline capable** - Once models downloaded, works without internet

## Platform Support

AI features require GPU acceleration for practical performance. CPU-only transcription is too slow for good user experience.

| Platform | AI Features | GPU Acceleration |
|----------|-------------|------------------|
| **macOS ARM64** | ✅ Yes | Metal |
| **Windows AMD64** | ✅ Yes | CUDA (NVIDIA GPU required) |
| macOS AMD64 | ❌ No | - |
| Linux AMD64 | ❌ No | - |
| Linux ARM64 | ❌ No | - |

**Why no Linux/Intel Mac support?**
- CPU-only transcription takes 10-30x longer than audio duration
- Poor user experience leads to complaints
- NAS/VPS users (majority of Linux users) don't have GPUs
- For Linux with NVIDIA GPU, use the Docker image

## AI Features

| Feature | Runtime | Use Case | Status |
|---------|---------|----------|--------|
| Speech-to-Text (STT) | whisper.cpp | Transcription, subtitles | **Active** |
| Text-to-Speech (TTS) | Piper | Audiobook generation, accessibility | TODO |
| OCR | Tesseract | Image text extraction, scanned PDFs | TODO |
| PDF Processing | pdfcpu, poppler | Text extraction, manipulation | TODO |

**ASR Engine:**
- **whisper.cpp** - 99 languages, uses Whisper models (ggml format)

---

## Architecture

### Binary Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                      vget CLI Binary                            │
│                     (CGO_ENABLED=0)                             │
├─────────────────────────────────────────────────────────────────┤
│  Embedded whisper.cpp binary (GPU-enabled)                      │
│  ├── macOS ARM64: Metal acceleration (~3MB)                     │
│  └── Windows AMD64: CUDA acceleration (~8MB)                    │
├─────────────────────────────────────────────────────────────────┤
│  Audio Decoders (Pure Go)                                       │
│  ├── MP3  → go-mp3                                              │
│  ├── WAV  → go-audio/wav                                        │
│  ├── FLAC → mewkiz/flac                                         │
│  └── Others → go-ffmpreg (embedded WASM)                        │
├─────────────────────────────────────────────────────────────────┤
│  Model Manager                                                  │
│  ├── Download models on first use                               │
│  └── Store in ~/.config/vget/models/                            │
└─────────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
~/.config/vget/
├── config.yml
├── bin/                              # Extracted runtime binaries
│   └── whisper-cli                   # Extracted from embedded binary
└── models/                           # AI models (downloaded on first use)
    ├── whisper-large-v3-turbo.bin    # Whisper model (~1.6GB)
    ├── whisper-small.bin             # Whisper model (~488MB)
    └── ...
```

---

## Feature Details

### Speech-to-Text (STT)

**Purpose:** Convert audio/video to text transcripts and subtitles.

#### Runtime: whisper.cpp

GPU-enabled whisper.cpp binary is embedded in vget:
- **macOS ARM64**: Built with Metal support
- **Windows AMD64**: Built with CUDA support

#### Model Options

| Model | Size | Languages | Use Case |
|-------|------|-----------|----------|
| whisper-tiny | ~78MB | 99 | Quick test |
| whisper-base | ~148MB | 99 | Fast drafts |
| whisper-small | ~488MB | 99 | Balanced |
| whisper-medium | ~1.5GB | 99 | Higher accuracy |
| whisper-large-v3 | ~3.1GB | 99 | Highest accuracy |
| **whisper-large-v3-turbo** | ~1.6GB | 99 | **Best (recommended)** |

#### Output Formats

Output format is detected from `-o` file extension:

| Extension | Format | Description |
|-----------|--------|-------------|
| `.md` | Markdown | Timestamped text (default) |
| `.srt` | SubRip | Standard subtitle format |
| `.vtt` | WebVTT | Web subtitle format |
| `.txt` | Plain text | No timestamps |

```bash
vget ai transcribe audio.mp3 -l zh                # → audio.transcript.md
vget ai transcribe audio.mp3 -l zh -o out.srt    # → out.srt
vget ai transcribe audio.mp3 -l zh -o out.vtt    # → out.vtt
```

### Text-to-Speech (TTS) - TODO

**Purpose:** Generate natural speech from text.

**Planned implementation:**
- **CLI**: Cloud TTS APIs only (OpenAI, Azure) - keeps binary small
- **Docker**: [IndexTTS](https://github.com/index-tts/index-tts) for local TTS with voice cloning

**IndexTTS features:**
- Zero-shot voice cloning from single audio sample
- Chinese + English with cross-lingual support
- Emotional expressiveness control
- Apache 2.0 license

### OCR - TODO

**Purpose:** Extract text from images and scanned documents.

### PDF Processing - TODO

**Purpose:** Extract, manipulate, and convert PDF documents.

---

## Core Interfaces

### Transcriber Interface

```go
type Transcriber interface {
    Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (*TranscribeResult, error)
}

type TranscribeOptions struct {
    Language string   // ISO 639-1 code or "auto"
    Model    string   // Model name
}

type TranscribeResult struct {
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
```

### Model Interface

```go
// Model represents an AI model file
type Model interface {
    Name() string
    Size() int64
    DownloadURL() string

    // EnsureDownloaded downloads the model if not present
    EnsureDownloaded(ctx context.Context) error

    // Path returns the local file path
    Path() string
}
```

---

## Error Handling

| Scenario | Error Message |
|----------|---------------|
| macOS AMD64 | "AI features are not available on Intel Macs. Please use a Mac with Apple Silicon (M1/M2/M3/M4)" |
| Linux | "AI features are not available on Linux" |
| Windows without NVIDIA GPU | "AI features require NVIDIA GPU with CUDA support. No NVIDIA GPU detected" |
| Model not found | Auto-download, show progress |
| Download failed | Retry with backoff, then error |

---

## Implementation Status

### Phase 1: Core Infrastructure ✅
- [x] Embedded whisper.cpp binary (Metal/CUDA)
- [x] Model manager with download
- [x] Platform detection and error messages

### Phase 2: Speech-to-Text ✅
- [x] whisper.cpp integration
- [x] `vget ai transcribe` command (auto-detect format from -o extension)
- [x] `vget ai models` command

### Phase 3: Text-to-Speech - TODO
- [ ] Piper integration
- [ ] `vget ai speak` command

### Phase 4: OCR - TODO
- [ ] Tesseract integration
- [ ] `vget ai ocr` command

---

## Success Criteria

1. `vget ai transcribe` works on first run (auto-downloads model)
2. Fast transcription with GPU acceleration
3. Clear error messages for unsupported platforms
4. Offline capable after initial model download
