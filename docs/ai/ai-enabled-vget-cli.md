# vget AI: Local Models for Speech & Language

## Overview

This document outlines the plan for local AI capabilities in vget, enabling offline speech-to-text (and future text-to-speech) without Docker or external services.

**Current focus:** Whisper transcription (speech-to-text)
**Future:** TTS (text-to-speech), translation, summarization

## Design Decision: Single Binary

**vget ships as a single binary.** When AI features are used without a downloaded model, the CLI exits gracefully with a helpful message guiding the user to download the required model.

This approach provides:
- Simple distribution (one binary per platform)
- Small download size for users who don't need AI features
- Clear guidance when AI features are needed

**Docker Alternative:** The Docker image includes the full whisper model (large-v3-turbo). If you cannot download models due to network restrictions, firewall, or other reasons, use Docker:

```bash
docker run -v ~/Downloads:/downloads ghcr.io/guiyumin/vget-server
```

## Current State

- **vget CLI**: Download-only binary (~27 MB, CGO_ENABLED=0)
- **vget-server**: Web UI with AI transcription (Docker-based, includes whisper.cpp)
- **Local ASR**: whisper.cpp integration exists but requires manual model download

## Model Options

### Whisper.cpp Models

| Model | Disk Size | RAM | Speed (M4 Max) | Quality | Notes |
|-------|-----------|-----|----------------|---------|-------|
| tiny | 78 MB | ~400 MB | ~60x real-time | Basic | Fastest, English-focused |
| base | 148 MB | ~500 MB | ~50x real-time | Fair | Good for quick drafts |
| small | 488 MB | ~1 GB | ~30x real-time | Good | Balanced for most uses |
| medium | 1.5 GB | ~3 GB | ~15x real-time | Better | Higher accuracy |
| large-v3 | 3.1 GB | ~6 GB | ~8x real-time | Best | Highest accuracy, slowest |
| **large-v3-turbo** | **1.6 GB** | **~3 GB** | **~20x real-time** | **Best** | **✅ Default** |
| distil-large-v3 | 756 MB | ~2 GB | ~40x real-time | Very Good | 5x faster than large-v3 |

**Quantized Variants** (smaller file size, slight quality trade-off):
| Model | Full Size | Q5_0 | Q8_0 |
|-------|-----------|------|------|
| medium | 1.5 GB | 539 MB | 823 MB |
| large-v3 | 3.1 GB | 1.08 GB | - |
| large-v3-turbo | 1.6 GB | 574 MB | 874 MB |

**Why large-v3-turbo is the default:**
- Based on large-v3 (best quality)
- 8x faster than large-v3
- Same size as medium but better quality and faster
- Best balance of speed, quality, and resource usage

### Future: NVIDIA Parakeet (CUDA Required)

| Model | Params | Speed | Quality | Notes |
|-------|--------|-------|---------|-------|
| parakeet-tdt-0.6b-v2 | 600M | RTFx 3386 | #1 on HF leaderboard | English only, requires NVIDIA GPU |
| parakeet-tdt-0.6b-v3 | 600M | RTFx 3000+ | Excellent | 25 European languages |
| parakeet-1.1b | 1.1B | RTFx 2000+ | Best | Fastest on Open ASR benchmark |

> **Note:** Parakeet models require NVIDIA GPU with CUDA. They use NeMo framework (not whisper.cpp).
> Parakeet can transcribe 60 minutes of audio in ~1 second on A100 GPU.
> Future consideration for Linux servers with NVIDIA hardware.

## CLI Interface

```bash
# Transcribe audio file
vget transcribe audio.mp3

# If model not found, exits with helpful message:
# "Whisper model not found. Download it with:
#   vget transcribe --download-model turbo
#
# Available models:
#   tiny     (78 MB)  - Fastest, basic quality
#   base    (148 MB)  - Quick drafts
#   small   (488 MB)  - Good for most uses
#   medium  (1.5 GB)  - Higher accuracy
#   large   (3.1 GB)  - Best accuracy, slowest
#   turbo   (1.6 GB)  - Best quality + fast (recommended)
#   distil  (756 MB)  - Fast, very good quality"

# Download a model
vget transcribe --download-model turbo

# Transcribe with specific model
vget transcribe audio.mp3 --model medium

# Transcribe with language hint
vget transcribe audio.mp3 --language zh

# List available/downloaded models
vget transcribe --list-models
```

## Model Storage

Models are stored in the user's config directory:

```
~/.config/vget/
├── config.yml
└── models/
    └── ggml-large-v3-turbo.bin  (1.5 GB)
```

## Model Download URLs

Models are downloaded from Hugging Face:

```bash
# Base URL
BASE=https://huggingface.co/ggerganov/whisper.cpp/resolve/main

# Full precision models
$BASE/ggml-tiny.bin              # 78 MB
$BASE/ggml-base.bin              # 148 MB
$BASE/ggml-small.bin             # 488 MB
$BASE/ggml-medium.bin            # 1.5 GB
$BASE/ggml-large-v3.bin          # 3.1 GB
$BASE/ggml-large-v3-turbo.bin    # 1.6 GB (default)

# Quantized models (smaller, slightly lower quality)
$BASE/ggml-medium-q5_0.bin       # 539 MB
$BASE/ggml-large-v3-q5_0.bin     # 1.08 GB
$BASE/ggml-large-v3-turbo-q5_0.bin  # 574 MB

# Distil model (different repo)
https://huggingface.co/distil-whisper/distil-large-v3-ggml/resolve/main/ggml-distil-large-v3.bin
```

## Build Configuration

The vget binary is built with `CGO_ENABLED=0` by default (no whisper.cpp linked). This keeps the binary small and portable.

For local development with Metal acceleration on macOS:

```bash
# Build whisper.cpp v1.8.2 with Metal
git clone --depth 1 --branch v1.8.2 https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF
cmake --build build -j

# Build vget with whisper.cpp linked
C_INCLUDE_PATH="whisper.cpp/include:whisper.cpp/ggml/include" \
LIBRARY_PATH="whisper.cpp/build/src:whisper.cpp/build/ggml/src:whisper.cpp/build/ggml/src/ggml-metal:whisper.cpp/build/ggml/src/ggml-blas" \
CGO_ENABLED=1 go build -o vget ./cmd/vget
```

## GitHub Actions (No Changes Needed)

The existing release workflow remains unchanged:

```yaml
# .github/workflows/release.yml
jobs:
  build-cli:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
          - os: macos-latest
            goos: darwin
            goarch: amd64
          - os: macos-latest
            goos: darwin
            goarch: arm64
          - os: windows-latest
            goos: windows
            goarch: amd64
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: CGO_ENABLED=0 go build -o vget-${{ matrix.goos }}-${{ matrix.goarch }} ./cmd/vget
```

## Implementation Plan

### Phase 1: CLI Transcribe Command
1. Add `vget transcribe` command
2. Check for model in `~/.config/vget/models/`
3. If model missing: exit 0 with download instructions
4. If model exists: run transcription

### Phase 2: Model Management
1. Implement `--download-model <name>` with progress bar
2. Implement `--list-models` to show available/downloaded models
3. Support `--model` flag to select model
4. Support `--language` flag for language hint

### Phase 3: Server Integration
1. vget-server continues to use Docker with whisper.cpp
2. Add option for server to use downloaded models from `~/.config/vget/models/`

## Performance Expectations

For 90 minutes of audio (with Metal acceleration):

| Platform | Model | Expected Time |
|----------|-------|---------------|
| M4 Max (Metal) | turbo | 4-8 min |
| M4 Max (Metal) | medium | 5-10 min |
| M1 (Metal) | turbo | 8-12 min |

Note: CPU-only transcription is significantly slower (10-20x slower than Metal).

## References

- [whisper.cpp](https://github.com/ggerganov/whisper.cpp)
- [whisper.cpp Go bindings](https://github.com/ggerganov/whisper.cpp/tree/master/bindings/go)
- [Ollama model management](https://github.com/ollama/ollama) (inspiration for model download UX)
