# Local-First Speech-to-Text

## Overview

vget supports local speech-to-text transcription using **whisper.cpp via CGO bindings**. This provides:

- Single Go binary (no external service dependencies)
- Metal acceleration on macOS (M1/M2/M3/M4)
- CUDA acceleration on NVIDIA GPUs
- Small Docker image (~100MB + model files vs ~5-8GB with Python)
- Works well on NAS devices with limited resources

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   vget (Go)                      │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │     transcriber/whisper_local.go          │   │
│  │                                           │   │
│  │  ┌─────────────────────────────────────┐  │   │
│  │  │   whisper.cpp (CGO)                 │  │   │
│  │  │   - Metal acceleration (macOS)      │  │   │
│  │  │   - CUDA acceleration (NVIDIA)      │  │   │
│  │  │   - CPU fallback                    │  │   │
│  │  └─────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────┘   │
│                                                  │
│  Models: ~/.config/vget/models/                  │
│  - ggml-tiny.bin       (75MB, testing only)     │
│  - ggml-small.bin      (244MB, fast)            │
│  - ggml-medium.bin     (769MB, balanced)        │
│  - ggml-large-v3-turbo.bin (1.5GB, best)        │
└─────────────────────────────────────────────────┘
```

## Model Selection

| Model | Size | Speed | Accuracy | Use Case |
|-------|------|-------|----------|----------|
| tiny | 75MB | Fastest | Low | Testing only |
| **small** | 244MB | Fast | Good | **Recommended for NAS** |
| medium | 769MB | Medium | Better | Desktop |
| large-v3-turbo | 1.5GB | Slow | Best | Server with GPU |

**Language Support:** All Whisper models support 100+ languages including Chinese, Japanese, Korean, etc.

## Configuration

In `~/.config/vget/config.yml`:

```yaml
ai:
  local_asr:
    enabled: true
    model: small           # tiny, small, medium, large-v3-turbo
    language: auto         # auto, en, zh, ja, etc.
    models_dir: ""         # optional, defaults to ~/.config/vget/models/
```

## Docker Usage (Recommended)

Docker handles all dependencies automatically:

```bash
# Pull and run
docker compose up -d

# Models are downloaded on first use and cached in the config volume
```

The Docker image includes:
- Pre-built whisper.cpp library (from base image)
- ffmpeg for audio conversion
- All required dependencies

### Base Image Architecture

To speed up builds, whisper.cpp is pre-compiled in a separate base image:

```
ghcr.io/guiyumin/vget-base:latest
    └── Go 1.25 + whisper.cpp (libwhisper.a)

ghcr.io/guiyumin/vget:latest
    └── FROM vget-base + vget application code
```

The base image is built:
- Manually via GitHub Actions workflow dispatch
- Automatically bi-weekly (1st and 15th of each month)

See `.github/workflows/build-base-image.yml` for details.

## Local Development (macOS/Linux)

The whisper.cpp Go bindings include a Makefile that builds the static library. You need to build it once before compiling vget.

### Prerequisites

1. Install build tools:
```bash
# macOS
xcode-select --install

# Linux (Debian/Ubuntu)
sudo apt-get install build-essential git
```

### Build whisper.cpp library

```bash
# Find where Go downloaded the whisper.cpp module
WHISPER_PATH=$(go list -m -f '{{.Dir}}' github.com/ggerganov/whisper.cpp/bindings/go)

# Build the static library
cd "$WHISPER_PATH"
make whisper

# Verify libwhisper.a was created
ls -la libwhisper.a
```

### Build vget

```bash
# Get the whisper.cpp path
WHISPER_PATH=$(go list -m -f '{{.Dir}}' github.com/ggerganov/whisper.cpp/bindings/go)

# Build with CGO
CGO_ENABLED=1 \
C_INCLUDE_PATH="$WHISPER_PATH" \
LIBRARY_PATH="$WHISPER_PATH" \
go build -o build/vget ./cmd/vget

# Build server
CGO_ENABLED=1 \
C_INCLUDE_PATH="$WHISPER_PATH" \
LIBRARY_PATH="$WHISPER_PATH" \
go build -o build/vget-server ./cmd/vget-server
```

### Hardware Acceleration

For Metal (Apple Silicon) or CUDA (NVIDIA), you need to build the library with appropriate flags:

```bash
cd "$WHISPER_PATH"

# macOS with Metal
WHISPER_METAL=1 make whisper

# Linux with CUDA
GGML_CUDA=1 make whisper
```

Then build vget with the same environment variables as above.

## Build Targets (Makefile)

```bash
# Standard build with CGO (CPU only)
make build

# macOS with Metal acceleration
make build-metal

# Linux with CUDA acceleration
make build-cuda

# Build without CGO (disables local transcription)
make build-nocgo
```

## How It Works

1. **Audio Conversion**: Input audio/video is converted to 16kHz mono WAV using ffmpeg
2. **Transcription**: whisper.cpp processes the audio and returns timestamped segments
3. **Model Loading**: Models are loaded on first transcription and cached in memory
4. **Cleanup**: Temporary WAV files are cleaned up after transcription

## Files

| File | Purpose |
|------|---------|
| `internal/core/ai/transcriber/whisper_local.go` | Main transcriber implementation |
| `internal/core/ai/transcriber/whisper_models.go` | Model download and management |
| `internal/core/ai/transcriber/transcriber.go` | Provider factory (openai, local) |
| `internal/core/config/config.go` | LocalASRConfig struct |

## Troubleshooting

### "whisper.h not found"

The whisper.cpp library is not installed or not in the include path.

```bash
# macOS
brew install whisper.cpp
# or build from source (see above)

# Verify
ls /usr/local/include/whisper.h
```

### "ffmpeg not found"

Install ffmpeg for audio conversion:

```bash
# macOS
brew install ffmpeg

# Linux
sudo apt-get install ffmpeg
```

### Model download fails

Models are downloaded from HuggingFace. Check your internet connection and try again.

```bash
# Manual download
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin \
  -O ~/.config/vget/models/ggml-small.bin
```

## References

- [whisper.cpp](https://github.com/ggerganov/whisper.cpp)
- [whisper.cpp Go bindings](https://pkg.go.dev/github.com/ggerganov/whisper.cpp/bindings/go)
- [Whisper models](https://huggingface.co/ggerganov/whisper.cpp)
