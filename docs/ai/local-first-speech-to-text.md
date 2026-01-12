# Local-First Speech-to-Text

## Overview

vget uses **whisper.cpp** for local speech-to-text transcription:

- Highly optimized C++ implementation of OpenAI's Whisper
- GPU acceleration: Metal (macOS), CUDA (NVIDIA)
- CPU optimization: AVX2/AVX512 (x86), NEON (ARM)
- Supports 99 languages including Chinese, Japanese, Korean

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    vget transcriber                      │
│                                                         │
│  internal/core/ai/transcriber/                          │
│    ├── whisper.go        → whisper.cpp CGO bindings     │
│    ├── whisper_runner.go → CLI runner (non-CGO)         │
│    └── models.go         → Model management             │
├─────────────────────────────────────────────────────────┤
│  CGO Bindings (optional)                                │
│    └── go-whisper (whisper.cpp)                         │
├─────────────────────────────────────────────────────────┤
│  Native Library                                         │
│    └── libwhisper.so (with Metal/CUDA/AVX2)             │
└─────────────────────────────────────────────────────────┘
```

## Docker Image

Single image for all users - runtime GPU detection determines behavior:

| Condition | Mode | Model Source |
|-----------|------|--------------|
| `--gpus all` + NVIDIA GPU | Local transcription | Download on demand from HuggingFace/vmirror |
| No GPU flag or no NVIDIA | Cloud API | OpenAI Whisper API, Groq, etc. |

Models are not bundled in the image (~300MB base). They are downloaded on first use.

## Whisper Models

| Model | Size | Use Case |
|-------|------|----------|
| whisper-tiny.bin | ~78MB | Quick test |
| whisper-base.bin | ~148MB | Fast drafts |
| whisper-small.bin | ~488MB | Balanced |
| whisper-medium.bin | ~1.5GB | Higher accuracy |
| whisper-large-v3.bin | ~3.1GB | Highest accuracy |
| **whisper-large-v3-turbo.bin** | ~1.6GB | **Best quality + fast (recommended)** |

**Download URLs:**
- Tiny: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin
- Base: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin
- Small: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
- Medium: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin
- Large V3: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin
- Large V3 Turbo: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin

**whisper.cpp optimizations:**
- Apple Silicon: Metal acceleration (GPU)
- NVIDIA: CUDA acceleration
- x86: AVX2/AVX512 SIMD
- ARM: NEON SIMD

## Configuration

In `~/.config/vget/config.yml`:

```yaml
ai:
  local_asr:
    enabled: true
    engine: "whisper"
    model: "whisper-large-v3-turbo"
    language: "auto"      # or specific language code (en, zh, de, fr, etc.)
    models_dir: ""        # empty = default ~/.config/vget/models/
```

## Go API Usage

```go
// Using whisper.cpp via go-whisper
import "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

model, _ := whisper.New(modelPath)
defer model.Close()

ctx, _ := model.NewContext()
ctx.SetLanguage("zh")  // or "auto" for auto-detect

// Process audio samples
ctx.Process(audioSamples, nil, nil)

// Get segments with timestamps
for {
    segment, err := ctx.NextSegment()
    if err != nil { break }
    fmt.Printf("[%v -> %v] %s\n", segment.Start, segment.End, segment.Text)
}
```

## Implementation Files

| File | Purpose |
|------|---------|
| `internal/core/ai/transcriber/whisper.go` | Whisper transcriber (whisper.cpp CGO) |
| `internal/core/ai/transcriber/whisper_runner.go` | Whisper CLI runner (non-CGO) |
| `internal/core/ai/transcriber/models.go` | Model definitions and management |
| `internal/core/ai/transcriber/transcriber.go` | Interface and factory |
| `docker/vget/Dockerfile` | Docker image with whisper.cpp |

## Docker Usage

```bash
# Pull and run
docker compose up -d

# With NVIDIA GPU for local transcription
docker run --gpus all -p 8080:8080 ghcr.io/guiyumin/vget:latest

# Without GPU - uses cloud API (OpenAI Whisper, Groq, etc.)
docker run -p 8080:8080 ghcr.io/guiyumin/vget:latest
```

Models are downloaded on first use from HuggingFace or vmirror (China).

## Supported Languages

Whisper supports 99 languages:

Afrikaans, Albanian, Amharic, Arabic, Armenian, Assamese, Azerbaijani, Bashkir, Basque, Belarusian, Bengali, Bosnian, Breton, Bulgarian, Burmese, Cantonese, Castilian, Catalan, Chinese, Croatian, Czech, Danish, Dutch, English, Estonian, Faroese, Finnish, Flemish, French, Galician, Georgian, German, Greek, Gujarati, Haitian, Haitian Creole, Hausa, Hawaiian, Hebrew, Hindi, Hungarian, Icelandic, Indonesian, Italian, Japanese, Javanese, Kannada, Kazakh, Khmer, Korean, Lao, Latin, Latvian, Letzeburgesch, Lithuanian, Luxembourgish, Macedonian, Malagasy, Malay, Malayalam, Maltese, Maori, Marathi, Moldavian, Moldovan, Mongolian, Myanmar, Nepali, Norwegian, Nynorsk, Occitan, Panjabi, Pashto, Persian, Polish, Portuguese, Punjabi, Pushto, Romanian, Russian, Sanskrit, Serbian, Shona, Sindhi, Sinhala, Sinhalese, Slovak, Slovenian, Somali, Spanish, Sundanese, Swahili, Swedish, Tagalog, Tajik, Tamil, Tatar, Telugu, Thai, Tibetan, Turkish, Turkmen, Ukrainian, Urdu, Uzbek, Valencian, Vietnamese, Welsh, Yiddish, Yoruba

## Troubleshooting

### "libwhisper.so not found"

The whisper.cpp library is not installed.

```bash
# Check if installed
ldconfig -p | grep whisper

# If missing, build from source
git clone --depth 1 https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
cmake -B build -DBUILD_SHARED_LIBS=ON
cmake --build build --config Release
sudo cp build/src/libwhisper.so* /usr/local/lib/
sudo ldconfig
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
# Manual download (Whisper models - ggml format for whisper.cpp)
curl -L -o ~/.config/vget/models/whisper-tiny.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin
curl -L -o ~/.config/vget/models/whisper-small.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
curl -L -o ~/.config/vget/models/whisper-medium.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin
curl -L -o ~/.config/vget/models/whisper-large-v3-turbo.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin
```

## References

- [whisper.cpp GitHub](https://github.com/ggerganov/whisper.cpp)
- [whisper.cpp Go bindings](https://github.com/ggerganov/whisper.cpp/tree/master/bindings/go)
- [GGML Whisper Models](https://huggingface.co/ggerganov/whisper.cpp)
- [Whisper Model Card](https://huggingface.co/openai/whisper-large-v3-turbo)
