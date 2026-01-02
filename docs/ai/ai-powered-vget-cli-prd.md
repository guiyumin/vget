# PRD: AI-Powered vget CLI

## Overview

This document covers CLI-specific implementation for vget AI features.

**Key Design Decisions:**
- `CGO_ENABLED=0` - Pure Go binary, no C dependencies
- GPU-enabled whisper.cpp embedded in binary
- Models downloaded on first use from Hugging Face

See [ai-powered-vget-prd.md](./ai-powered-vget-prd.md) for shared concepts.

---

## Platform Support

| Platform | AI Features | GPU | Build |
|----------|-------------|-----|-------|
| **macOS ARM64** | ✅ | Metal | whisper.cpp with `-DWHISPER_METAL=ON` |
| **Windows AMD64** | ✅ | CUDA | whisper.cpp with `-DGGML_CUDA=ON` |
| macOS AMD64 | ❌ | - | Error message |
| Linux AMD64 | ❌ | - | Error message |
| Linux ARM64 | ❌ | - | Error message |

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
vget ai transcribe podcast.mp3                    # → podcast.transcript.md

# Specify language (required)
vget ai transcribe podcast.mp3 --language zh
vget ai transcribe podcast.mp3 -l en

# Choose model
vget ai transcribe podcast.mp3 -l zh --model whisper-small

# Output to specific file
vget ai transcribe podcast.mp3 -l zh -o my-transcript.md
```

### Convert Transcript

```bash
# Convert markdown transcript to subtitle formats
vget ai convert podcast.transcript.md --to srt    # → podcast.srt
vget ai convert podcast.transcript.md --to vtt    # → podcast.vtt
vget ai convert podcast.transcript.md --to txt    # → podcast.txt

# Specify output file
vget ai convert podcast.transcript.md --to srt -o subtitles.srt
```

### Text-to-Speech (TODO)

> Planned feature - not yet implemented

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
│   ├── whisper_embed_*.go    # Platform-specific embedded binaries
│   └── models.go             # Model registry and download
├── chunker/
│   └── chunker.go            # Audio chunking for large files
└── output/
    └── convert.go            # SRT/VTT/TXT conversion
```

---

## GitHub Actions Build

whisper.cpp binaries are built with GPU acceleration and embedded during release:

```yaml
# .github/workflows/release.yml
jobs:
  build-whisper-darwin-arm64:
    runs-on: macos-14
    steps:
      - name: Build whisper.cpp with Metal
        run: |
          git clone --branch v1.8.2 https://github.com/ggerganov/whisper.cpp
          cd whisper.cpp
          cmake -B build -DWHISPER_METAL=ON
          cmake --build build
          cp build/bin/whisper-cli ../internal/core/ai/transcriber/bin/

  build-whisper-windows-amd64:
    runs-on: windows-latest
    steps:
      - name: Install CUDA Toolkit
        uses: Jimver/cuda-toolkit@v0.2.19
        with:
          cuda: '12.6.3'
      - name: Build whisper.cpp with CUDA
        run: |
          git clone --branch v1.8.2 https://github.com/ggerganov/whisper.cpp
          cd whisper.cpp
          cmake -B build -DGGML_CUDA=ON
          cmake --build build
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

| Platform | Error Message |
|----------|---------------|
| macOS AMD64 | "AI features are not available on Intel Macs. Please use a Mac with Apple Silicon (M1/M2/M3/M4)" |
| Linux AMD64 | "AI features are not available on Linux" |
| Linux ARM64 | "AI features are not available on Linux" |
| Windows (no GPU) | "AI features require NVIDIA GPU with CUDA support. No NVIDIA GPU detected" |

---

## Model Downloads

### Available Models

| Model | Size | Description |
|-------|------|-------------|
| whisper-tiny | 78MB | Fastest, basic quality |
| whisper-base | 148MB | Good for quick drafts |
| whisper-small | 488MB | Balanced for most uses |
| whisper-medium | 1.5GB | Higher accuracy |
| whisper-large-v3 | 3.1GB | Highest accuracy, slowest |
| **whisper-large-v3-turbo** | 1.6GB | Best quality + fast **(default)** |

### Download Sources

| Source | URL | Use Case |
|--------|-----|----------|
| huggingface | huggingface.co/ggerganov/whisper.cpp | Default |
| vmirror | vmirror.org | Faster in China |

---

## Binary Size Estimates

| Component | Size |
|-----------|------|
| vget core | ~20MB |
| go-ffmpreg (WASM) | ~8MB |
| Pure Go decoders | ~1MB |
| whisper.cpp (embedded) | ~5-8MB |
| **Total binary** | **~35-40MB** |

---

## Testing

```bash
# Build for local testing
CGO_ENABLED=0 go build -o build/vget ./cmd/vget

# Test transcription (will download model on first run)
./build/vget ai transcribe testdata/sample.mp3 -l en

# Test model management
./build/vget ai models
./build/vget ai models -r
./build/vget ai download whisper-small

# Test convert
./build/vget ai convert sample.transcript.md --to srt
```

---

## References

- [whisper.cpp](https://github.com/ggerganov/whisper.cpp) - C++ Whisper implementation
- [go-ffmpreg](https://codeberg.org/gruf/go-ffmpreg) - Embedded ffmpeg WASM
- [go-mp3](https://github.com/hajimehoshi/go-mp3) - Pure Go MP3 decoder
