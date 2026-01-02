# PRD: AI-Powered vget CLI

## Overview

This document covers CLI-specific implementation for vget AI features.

**Key Design Decisions:**
- `CGO_ENABLED=0` - Pure Go binary, no C dependencies
- GPU-enabled binaries embedded: whisper.cpp + sherpa-onnx
- Two ASR engines: whisper.cpp (99 languages) and sherpa-onnx (Parakeet, 25 EU languages)
- Models downloaded on first use from Hugging Face / GitHub

See [ai-powered-vget-prd.md](./ai-powered-vget-prd.md) for shared concepts.

---

## Platform Support

| Platform | AI Features | whisper.cpp | sherpa-onnx |
|----------|-------------|-------------|-------------|
| **macOS ARM64** | ✅ | Metal | CPU (ANE via onnxruntime) |
| **Windows AMD64** | ✅ | CUDA | CUDA |
| macOS AMD64 | ❌ | - | - |
| Linux AMD64 | ❌ | - | - |
| Linux ARM64 | ❌ | - | - |

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

# Choose model (whisper-* or parakeet-*)
vget ai transcribe podcast.mp3 -l zh --model whisper-small
vget ai transcribe podcast.mp3 -l de --model parakeet-v3   # 25 EU languages

# Output to specific file
vget ai transcribe podcast.mp3 -l zh -o my-transcript.md
```

**Model Selection:**
- `whisper-*` models: 99 languages, uses whisper.cpp
- `parakeet-*` models: 25 European languages, uses sherpa-onnx (faster for EU)

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
│  Embedded sherpa-onnx (Parakeet models)                         │
│  ├── macOS ARM64: CPU/ANE (~7MB)                                │
│  └── Windows AMD64: CUDA (~17MB)                                │
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
│   ├── sherpa.go             # sherpa-onnx CGO implementation
│   ├── sherpa_runner.go      # sherpa-onnx CLI runner (non-CGO)
│   ├── sherpa_embed_*.go     # Platform-specific sherpa binaries
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
        uses: Jimver/cuda-toolkit@v0.2.19
      - name: Build whisper.cpp with CUDA
        run: |
          cmake -B build -DGGML_CUDA=ON
          cmake --build build
          cp build/bin/Release/whisper-cli.exe ../internal/core/ai/transcriber/bin/whisper-windows-amd64.exe

  # sherpa-onnx builds (for Parakeet models)
  build-sherpa-darwin-arm64:
    runs-on: macos-14
    steps:
      - name: Build sherpa-onnx
        run: |
          git clone --branch v1.12.20 https://github.com/k2-fsa/sherpa-onnx
          cd sherpa-onnx
          cmake -B build -DSHERPA_ONNX_ENABLE_TTS=OFF -DSHERPA_ONNX_ENABLE_BINARY=ON
          cmake --build build
          cp build/bin/sherpa-onnx-offline ../internal/core/ai/transcriber/bin/sherpa-darwin-arm64

  build-sherpa-windows-amd64:
    runs-on: windows-latest
    steps:
      - name: Install CUDA Toolkit
        uses: Jimver/cuda-toolkit@v0.2.19
      - name: Build sherpa-onnx with CUDA
        run: |
          cmake -B build -DSHERPA_ONNX_ENABLE_GPU=ON -DSHERPA_ONNX_ENABLE_TTS=OFF
          cmake --build build
          cp build/bin/Release/sherpa-onnx-offline.exe ../internal/core/ai/transcriber/bin/sherpa-windows-amd64.exe
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

#### Parakeet Models (sherpa-onnx, 25 EU languages)

| Model | Size | Languages | Description |
|-------|------|-----------|-------------|
| **parakeet-v3** | 630MB | 25 EU | Fast, multilingual EU |
| parakeet-v2 | 630MB | 1 (en) | English only |

Supported languages: bg, hr, cs, da, nl, en, et, fi, fr, de, el, hu, it, lv, lt, mt, pl, pt, ro, sk, sl, es, sv, ru, uk

#### Whisper Models (whisper.cpp, 99 languages)

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
| huggingface | huggingface.co/ggerganov/whisper.cpp | Whisper models (default) |
| github | github.com/k2-fsa/sherpa-onnx/releases | Parakeet models |
| vmirror | vmirror.org | Faster in China |

---

## Binary Size Estimates

| Component | macOS ARM64 | Windows x64 |
|-----------|-------------|-------------|
| vget core | ~20MB | ~20MB |
| go-ffmpreg (WASM) | ~8MB | ~8MB |
| Pure Go decoders | ~1MB | ~1MB |
| whisper.cpp (embedded) | ~5MB | ~8MB |
| sherpa-onnx (embedded) | ~7MB | ~17MB |
| **Total binary** | **~41MB** | **~54MB** |

---

## Testing

```bash
# Build for local testing
CGO_ENABLED=0 go build -o build/vget ./cmd/vget

# Test transcription with Whisper
./build/vget ai transcribe testdata/sample.mp3 -l en

# Test transcription with Parakeet (EU languages)
./build/vget ai transcribe testdata/sample.mp3 -l de --model parakeet-v3

# Test model management
./build/vget ai models
./build/vget ai models -r
./build/vget ai download whisper-small
./build/vget ai download parakeet-v3

# Test convert
./build/vget ai convert sample.transcript.md --to srt
```

---

## References

- [whisper.cpp](https://github.com/ggerganov/whisper.cpp) - C++ Whisper implementation
- [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) - ONNX-based speech recognition (Parakeet)
- [go-ffmpreg](https://codeberg.org/gruf/go-ffmpreg) - Embedded ffmpeg WASM
- [go-mp3](https://github.com/hajimehoshi/go-mp3) - Pure Go MP3 decoder
