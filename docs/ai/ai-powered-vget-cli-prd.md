# PRD: AI-Powered vget CLI

## Overview

This document covers CLI-specific implementation for vget AI features.

**Key Design Decisions:**

- `CGO_ENABLED=0` - Pure Go binary, no C dependencies
- GPU-enabled whisper.cpp binary embedded
- Models downloaded on first use from Hugging Face / GitHub

See [ai-powered-vget-prd.md](./ai-powered-vget-prd.md) for shared concepts.

---

## Platform Support

| Platform          | AI Features | whisper.cpp |
| ----------------- | ----------- | ----------- |
| **macOS ARM64**   | ✅          | Metal       |
| **Windows AMD64** | ✅          | CUDA        |
| macOS AMD64       | ❌          | -           |
| Linux AMD64       | ❌          | -           |
| Linux ARM64       | ❌          | -           |

---

## CLI Commands

### Model Management

```bash
# List downloaded models (local)
vget ai models

# List models available to download (remote)
vget ai models --remote
vget ai models -r

# Download a model (default: from Hugging Face)
vget ai models download whisper-large-v3-turbo

# Download from vmirror.org (faster in China)
vget ai models download whisper-large-v3-turbo --from=vmirror

# Shortcut alias for download
vget ai download whisper-large-v3-turbo

# Remove a model
vget ai models rm whisper-large-v3-turbo
```

### Speech-to-Text

```bash
# Basic transcription (outputs markdown with timestamps)
vget ai transcribe podcast.mp3 -l zh              # → podcast.transcript.md

# Output format is detected from -o extension
vget ai transcribe podcast.mp3 -l zh -o output.md   # → Markdown
vget ai transcribe podcast.mp3 -l zh -o output.srt  # → SRT subtitles
vget ai transcribe podcast.mp3 -l zh -o output.vtt  # → VTT subtitles
vget ai transcribe podcast.mp3 -l zh -o output.txt  # → Plain text

# Choose model
vget ai transcribe podcast.mp3 -l zh --model whisper-small
vget ai transcribe podcast.mp3 -l en --model whisper-large-v3-turbo
```

**Output Formats:**

- `.md` - Markdown with timestamps (default)
- `.srt` - SubRip subtitle format
- `.vtt` - WebVTT subtitle format
- `.txt` - Plain text (no timestamps)

### Text-to-Speech (TODO)

> CLI: Cloud TTS APIs only (OpenAI, Azure) - keeps binary small
> Docker: Local TTS via IndexTTS with voice cloning - see [Docker PRD](./ai-powered-vget-docker-prd.md)

```bash
# Cloud TTS (CLI)
vget ai tts "Hello world" -o output.wav --voice alloy  # OpenAI
vget ai tts "你好世界" -o output.wav --provider azure   # Azure
```

### OCR (TODO)

> Planned feature - not yet implemented

---

## Architecture

### Binary Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                      vget CLI Binary                            │
│                     (CGO_ENABLED=0)                             │
├─────────────────────────────────────────────────────────────────┤
│  Embedded whisper.cpp (GPU-enabled)                             │
│  ├── macOS ARM64: Metal (~5MB)                                  │
│  └── Windows AMD64: CUDA (~8MB)                                 │
├─────────────────────────────────────────────────────────────────┤
│  Audio Decoders (Pure Go)                                       │
│  ├── MP3  → go-mp3 (hajimehoshi/go-mp3)                        │
│  ├── WAV  → go-audio/wav                                        │
│  ├── FLAC → mewkiz/flac                                         │
│  └── M4A/AAC/OGG → go-ffmpreg (embedded WASM, ~8MB)            │
├─────────────────────────────────────────────────────────────────┤
│  Model Manager                                                  │
│  ├── Download models on first use                               │
│  ├── Show progress with TUI                                     │
│  └── Store in ~/.config/vget/models/                           │
└─────────────────────────────────────────────────────────────────┘
```

### Package Structure

```
internal/core/ai/
├── ai.go                     # Main AI orchestrator
├── transcriber/
│   ├── transcriber.go        # Transcriber interface
│   ├── whisper.go            # whisper.cpp CGO implementation
│   ├── whisper_runner.go     # whisper.cpp CLI runner (non-CGO)
│   ├── whisper_embed_*.go    # Platform-specific whisper binaries
│   └── models.go             # Model registry and download
├── chunker/
│   └── chunker.go            # Audio chunking for large files
└── output/
    └── convert.go            # SRT/VTT/TXT conversion
```

---

## GitHub Actions Build

AI binaries are built with GPU acceleration and embedded during release:

```yaml
# .github/workflows/release.yml
jobs:
  # whisper.cpp builds
  build-whisper-darwin-arm64:
    runs-on: macos-14
    steps:
      - name: Build whisper.cpp with Metal
        run: |
          git clone --branch v1.8.2 https://github.com/ggerganov/whisper.cpp
          cd whisper.cpp
          cmake -B build -DWHISPER_METAL=ON
          cmake --build build
          cp build/bin/whisper-cli ../internal/core/ai/transcriber/bin/whisper-darwin-arm64

  build-whisper-windows-amd64:
    runs-on: windows-latest
    steps:
      - name: Install CUDA Toolkit
        uses: Jimver/cuda-toolkit@v0.2.30
        with:
          cuda: "12.6.0"
      - name: Build whisper.cpp with CUDA
        run: |
          cmake -B build -DGGML_CUDA=ON
          cmake --build build
          cp build/bin/Release/whisper-cli.exe ../internal/core/ai/transcriber/bin/whisper-windows-amd64.exe
```

---

## Platform Detection

### Windows NVIDIA GPU Detection

```go
// hasNvidiaGPU checks if NVIDIA GPU is available
func hasNvidiaGPU() bool {
    cmd := exec.Command("nvidia-smi")
    err := cmd.Run()
    return err == nil
}

func extractWhisperBinary() (string, error) {
    if !hasNvidiaGPU() {
        return "", fmt.Errorf("AI features require NVIDIA GPU with CUDA support. No NVIDIA GPU detected")
    }
    // Extract embedded binary...
}
```

### Unsupported Platform Messages

| Platform         | Error Message                                                                                    |
| ---------------- | ------------------------------------------------------------------------------------------------ |
| macOS AMD64      | "AI features are not available on Intel Macs. Please use a Mac with Apple Silicon (M1/M2/M3/M4)" |
| Linux AMD64      | "AI features are not available on Linux"                                                         |
| Linux ARM64      | "AI features are not available on Linux"                                                         |
| Windows (no GPU) | "AI features require NVIDIA GPU with CUDA support. No NVIDIA GPU detected"                       |

---

## Model Downloads

### Whisper Models (whisper.cpp, 99 languages)

| Model                      | Size  | Description                       |
| -------------------------- | ----- | --------------------------------- |
| whisper-tiny               | 78MB  | Fastest, basic quality            |
| whisper-base               | 148MB | Good for quick drafts             |
| whisper-small              | 488MB | Balanced for most uses            |
| whisper-medium             | 1.5GB | Higher accuracy                   |
| whisper-large-v3           | 3.1GB | Highest accuracy, slowest         |
| **whisper-large-v3-turbo** | 1.6GB | Best quality + fast **(default)** |

### Download Sources

| Source      | URL                                    | Use Case                 |
| ----------- | -------------------------------------- | ------------------------ |
| huggingface | huggingface.co/ggerganov/whisper.cpp   | Whisper models (default) |
| vmirror     | vmirror.org                            | Faster in China          |

---

## Binary Size Estimates

| Component              | macOS ARM64 | Windows x64 |
| ---------------------- | ----------- | ----------- |
| vget core              | ~20MB       | ~20MB       |
| go-ffmpreg (WASM)      | ~8MB        | ~8MB        |
| Pure Go decoders       | ~1MB        | ~1MB        |
| whisper.cpp (embedded) | ~3MB        | ~8MB        |
| **Total binary**       | **~32MB**   | **~37MB**   |

---

## Testing

```bash
# Build for local testing
CGO_ENABLED=0 go build -o build/vget ./cmd/vget

# Test transcription with Whisper (default markdown output)
./build/vget ai transcribe testdata/sample.mp3 -l en

# Test transcription with different output formats
./build/vget ai transcribe testdata/sample.mp3 -l en -o output.srt
./build/vget ai transcribe testdata/sample.mp3 -l en -o output.vtt

# Test model management
./build/vget ai models
./build/vget ai models -r
./build/vget ai download whisper-small
```

---

## References

- [whisper.cpp](https://github.com/ggerganov/whisper.cpp) - C++ Whisper implementation
- [go-ffmpreg](https://codeberg.org/gruf/go-ffmpreg) - Embedded ffmpeg WASM
- [go-mp3](https://github.com/hajimehoshi/go-mp3) - Pure Go MP3 decoder
