# PRD: AI-Powered vget Docker

## Overview

Docker deployment for vget with AI capabilities, featuring a web UI for media processing.

See [ai-powered-vget-prd.md](./ai-powered-vget-prd.md) for shared concepts.

---

## User Flow (Web UI)

1. User opens web UI at `http://localhost:8080`
2. Selects audio/video file from downloads or uploads new file
3. Configures AI options (transcribe, translate, summarize)
4. Selects processing options (model, language)
5. Monitors progress via real-time stepper UI
6. Downloads or views outputs (transcript, translation, SRT, summary)

---

## Runtime Detection Strategy

```
┌─────────────────────────────────────────────────────────────────┐
│                    vget Docker Image                             │
│                    ghcr.io/guiyumin/vget:latest                  │
│                                                                  │
│  Bundled:                                                        │
│  ├── whisper.cpp binary (CUDA-enabled for amd64)                 │
│  ├── IndexTTS (Python, for local TTS with voice cloning)         │
│  └── ffmpeg                                                      │
│                                                                  │
│  On startup:                                                     │
│  ├── Detect NVIDIA GPU (nvidia-smi)                              │
│  │   ├── GPU found → Local transcription mode                    │
│  │   │               • Download models on demand                 │
│  │   │               • From HuggingFace or vmirror (China)       │
│  │   │                                                           │
│  │   └── No GPU → Cloud API mode                                 │
│  │                • OpenAI Whisper API                           │
│  │                • Groq (free tier)                             │
│  │                • Configure in Web UI Settings                 │
│  │                                                               │
│  └── Show capability status in Web UI                            │
└─────────────────────────────────────────────────────────────────┘
```

### Why Only NVIDIA GPU Detection?

**Integrated GPUs (Intel, AMD) are not supported** for local transcription:

| GPU Type | Speed | Support | Worth It? |
|----------|-------|---------|-----------|
| NVIDIA RTX | 10-30x faster than CPU | CUDA (whisper.cpp) | ✅ Yes |
| Apple Silicon | 5-15x faster | Metal (whisper.cpp) | ✅ Yes (CLI only) |
| Intel iGPU | 1-2x faster | Limited OpenCL | ❌ No |
| AMD iGPU | 1-2x faster | Limited ROCm | ❌ No |

**Integrated GPUs are too slow** because:
- Share system memory (no dedicated VRAM)
- Whisper.cpp lacks optimized Intel/AMD GPU backends
- Only marginal speedup over CPU (not worth complexity)
- Still 10-20x slower than NVIDIA discrete GPU

### Why No CPU-Only Local Transcription?

Local CPU transcription is **impractically slow**:
- 1-hour audio → ~2-4 hours on typical NAS CPU
- Memory intensive (requires 4-8GB RAM for large models)
- Poor user experience

API-based transcription is **fast and cost-effective**:
- 1-hour audio → ~2-3 minutes via API
- OpenAI Whisper API: ~$0.36/hour of audio
- Groq: Free tier available

---

## Docker Usage

### Single Image for All Users

One image works for everyone - runtime GPU detection determines behavior:

```yaml
# compose.yml
services:
  vget:
    image: ghcr.io/guiyumin/vget:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config:/home/vget/.config/vget
      - ./downloads:/home/vget/downloads
```

### With NVIDIA GPU

Add GPU access for local transcription:

```yaml
# compose.yml
services:
  vget:
    image: ghcr.io/guiyumin/vget:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config:/home/vget/.config/vget
      - ./downloads:/home/vget/downloads
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

Or with docker run:

```bash
docker run --gpus all -p 8080:8080 \
  -v ./config:/home/vget/.config/vget \
  -v ./downloads:/home/vget/downloads \
  ghcr.io/guiyumin/vget:latest
```

See [Docker GPU Passthrough Guide](./docker-gpu-passthrough.md) for detailed setup (Windows/Linux).

### Runtime Behavior

| Condition | Mode | Model Source |
|-----------|------|--------------|
| `--gpus all` + NVIDIA GPU | Local | Download on demand from HuggingFace/vmirror |
| No GPU flag or no NVIDIA | Cloud API | OpenAI Whisper API, Groq, etc. |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VGET_PORT` | 8080 | Web UI port |
| `VGET_HOST` | 0.0.0.0 | Listen address |
| `VGET_DATA_DIR` | /home/vget/downloads | Download directory |
| `VGET_MODEL_DIR` | /home/vget/.config/vget/models | Model storage |

---

## API Endpoints

### AI Job Management

```
POST   /api/ai/jobs              # Start new AI job
GET    /api/ai/jobs              # List all jobs
GET    /api/ai/jobs/:id          # Get job status and progress
DELETE /api/ai/jobs/:id          # Cancel job
GET    /api/ai/jobs/:id/result   # Get job outputs
```

### Request/Response

```typescript
// POST /api/ai/jobs
interface StartJobRequest {
  file_path: string;
  options: {
    transcribe: boolean;
    summarize: boolean;
    translate_to?: string[];  // ["en", "zh"]
    model?: string;  // e.g., "whisper-large-v3-turbo"
    generate_srt?: boolean;   // also generate SRT after transcription
    generate_vtt?: boolean;   // also generate VTT after transcription
    language?: string;
  };
}

// GET /api/ai/jobs/:id
interface JobStatus {
  id: string;
  file_path: string;
  file_name: string;
  status: "queued" | "processing" | "completed" | "failed" | "cancelled";
  current_step: StepKey;
  steps: ProcessingStep[];
  overall_progress: number;
  result?: JobResult;
  error?: string;
  created_at: string;
  updated_at: string;
}

interface ProcessingStep {
  key: StepKey;
  name: string;
  status: "pending" | "in_progress" | "completed" | "skipped" | "failed";
  progress: number;
  detail?: string;
}

type StepKey =
  | "extract_audio"
  | "compress_audio"
  | "chunk_audio"
  | "transcribe"
  | "merge"
  | "translate"
  | "generate_srt"
  | "summarize";

interface JobResult {
  transcript_path?: string;
  translated_paths?: Record<string, string>;  // { "en": "...", "zh": "..." }
  srt_paths?: Record<string, string>;
  summary_path?: string;
}
```

### Model Management API

```
GET    /api/ai/models             # List available models
GET    /api/ai/models/installed   # List installed models
POST   /api/ai/models/:id/install # Download and install model
DELETE /api/ai/models/:id         # Remove installed model
```

### Text-to-Speech API (Docker only)

```
POST   /api/ai/tts                # Generate speech from text
```

```typescript
// POST /api/ai/tts
interface TTSRequest {
  text: string;              // Text to synthesize
  voice_ref?: string;        // Path to reference audio for voice cloning
  language?: string;         // "zh" or "en" (default: auto-detect)
  emotion?: string;          // Optional emotion control
  output_format?: string;    // "wav" (default) or "mp3"
}

interface TTSResponse {
  audio_path: string;        // Path to generated audio file
  duration: number;          // Duration in seconds
}
```

---

## IndexTTS Integration

[IndexTTS](https://github.com/index-tts/index-tts) is an industrial-grade zero-shot TTS system with 17.8k+ stars, chosen for vget Docker due to its voice cloning capabilities and Chinese language support.

### Model Versions

| Version | Features | Recommended |
|---------|----------|-------------|
| **IndexTTS-2** | Emotional control, duration control, best quality | ✅ Yes |
| IndexTTS-1.5 | Improved English, stable | For lower VRAM |
| IndexTTS-1.0 | Original release | Legacy |

### System Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| **VRAM** | 8 GB | 12+ GB |
| **RAM** | 16 GB | 32 GB |
| **Storage** | 40 GB free | 60 GB free |
| **GPU** | RTX 3060 | RTX 4090/5090 |
| **CUDA** | 12.8+ | Latest |

### Performance Benchmarks

| GPU | 17s Audio Generation |
|-----|---------------------|
| RTX 3060 12GB | ~228 seconds |
| RTX 4090 24GB | ~15-30 seconds (estimated) |
| CPU only | Not recommended (very slow) |

### Key Features

**Zero-Shot Voice Cloning:**
- Clone any voice from a single audio sample (3-10 seconds recommended)
- No fine-tuning required
- Preserves timbre and speaking style

**Emotional Control:**
```python
# Emotion vector: [happy, angry, sad, afraid, disgusted, melancholic, surprised, calm]
emo_vector = [0.8, 0.0, 0.0, 0.0, 0.0, 0.0, 0.1, 0.1]
```

**Duration Control:**
- First autoregressive TTS with precise duration control
- Natural generation mode also available
- Useful for dubbing and synchronization

**Language Support:**
- Chinese (primary)
- English
- Cross-lingual synthesis (read Chinese text in English voice)

### Docker Integration

```dockerfile
# Additional layers for IndexTTS in vget Docker image
FROM ghcr.io/guiyumin/vget:latest

# Install Python and uv package manager
RUN apt-get update && apt-get install -y \
    python3 python3-pip curl \
    && rm -rf /var/lib/apt/lists/*

# Install uv (required by IndexTTS)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# Clone and install IndexTTS
RUN git clone https://github.com/index-tts/index-tts.git /opt/indextts \
    && cd /opt/indextts \
    && uv sync

# Models downloaded on first use from HuggingFace
ENV INDEXTTS_MODEL_DIR=/home/vget/.config/vget/models/indextts
```

### API Usage Examples

**Basic TTS:**
```bash
curl -X POST http://localhost:8080/api/ai/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "你好，欢迎使用vget",
    "language": "zh"
  }'
```

**Voice Cloning:**
```bash
curl -X POST http://localhost:8080/api/ai/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "This is my cloned voice speaking",
    "voice_ref": "/downloads/my_voice_sample.wav",
    "language": "en"
  }'
```

**Emotional Speech:**
```bash
curl -X POST http://localhost:8080/api/ai/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "I am so happy to see you!",
    "emotion": "happy",
    "voice_ref": "/downloads/speaker.wav"
  }'
```

### Use Cases in vget

1. **Podcast Narration:** Convert transcripts back to speech with consistent voice
2. **Audiobook Generation:** Turn downloaded articles/ebooks into audio
3. **Voice Dubbing:** Clone original speaker voice for translations
4. **Accessibility:** Generate audio versions of text content

### Limitations

- **VRAM hungry:** 8GB minimum, 12GB+ recommended
- **Slow on consumer GPUs:** RTX 3060 takes ~4 minutes for 17s audio
- **Chinese-first:** English quality improving but Chinese is primary focus
- **No streaming:** Generates complete audio, not real-time

### References

- [IndexTTS GitHub](https://github.com/index-tts/index-tts)
- [IndexTTS-2 Review](https://dev.to/czmilo/indextts2-comprehensive-review-in-depth-analysis-of-2025s-most-powerful-emotional-speech-1m9e)
- [HuggingFace Models](https://huggingface.co/IndexTeam)

---

## Web UI Components

### Processing Configuration

```typescript
interface ProcessingConfig {
  transcribe: boolean;
  summarize: boolean;
  translateTo: string[];      // ["en", "zh", "jp"]
  model: string;              // "auto", "whisper-large-v3-turbo", etc.
  language: string;
  generateSrt: boolean;       // also generate SRT after transcription
  generateVtt: boolean;       // also generate VTT after transcription
}
```

### Step Display (ProcessingStepper)

Real-time progress visualization with step-by-step status.

### UI Translations

```typescript
// AI step names
ai_step_extract_audio: "Extract Audio",
ai_step_compress_audio: "Compress Audio",
ai_step_chunk_audio: "Chunk Audio",
ai_step_transcribe: "Transcribe",
ai_step_merge: "Merge Chunks",
ai_step_translate: "Translate",
ai_step_generate_srt: "Generate Subtitles",
ai_step_summarize: "Generate Summary",

// Options
ai_model_auto: "Auto (recommended)",
ai_model_whisper: "Whisper Large V3 Turbo (99 languages)",
ai_translate_to: "Translate to",
ai_generate_srt: "Also generate SRT subtitles",
ai_generate_vtt: "Also generate VTT subtitles",
```

---

## Processing Pipeline

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│   Upload    │ → │   Chunk     │ → │ Transcribe  │ → │  Translate  │ → │  Summarize  │
│  (Web UI)   │   │  (ffmpeg)   │   │  (Whisper)  │   │   (LLM)     │   │   (LLM)     │
└─────────────┘   └─────────────┘   └─────────────┘   └─────────────┘   └─────────────┘
                        │                 │                 │                 │
                        ▼                 ▼                 ▼                 ▼
                 .chunks/ dir      .transcript.md    .{lang}.transcript.md  .summary.md
                                                     .{lang}.srt
```

### Chunking Strategy

- Fixed 10-minute chunks with 10-second overlap
- Deduplication during merge phase
- Handles files up to several hours

---

## Output Files

```
podcast.mp3
  → podcast.transcript.md       (original language transcript)
  → podcast.en.transcript.md    (translated to English)
  → podcast.en.srt              (English subtitles)
  → podcast.summary.md          (summary in original language)
  → podcast.en.summary.md       (summary in English)
```

### Transcript Format (podcast.transcript.md)

```markdown
# Transcript: podcast.mp3

**Duration:** 1h 23m 45s
**Transcribed:** 2024-01-15 10:30:00
**Engine:** whisper/large-v3-turbo

---

[00:00:00] Welcome to today's episode. We're going to discuss...

[00:00:15] The main topic is about building reliable systems...

[00:05:30] Let me give you an example of how this works in practice...
```

### SRT Format (podcast.en.srt)

```srt
1
00:00:00,000 --> 00:00:15,000
Welcome to today's episode. We're going to discuss...

2
00:00:15,000 --> 00:05:30,000
The main topic is about building reliable systems...

3
00:05:30,000 --> 00:10:45,000
Let me give you an example of how this works in practice...
```

---

## Dockerfile

Single Dockerfile for all platforms (multi-arch amd64/arm64):

```dockerfile
# Dockerfile
FROM golang:1.23-bookworm AS builder

WORKDIR /app
COPY . .

# Build without CGO
RUN CGO_ENABLED=0 go build -o vget ./cmd/vget

# Final image
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/vget /usr/local/bin/vget

# Create non-root user
RUN useradd -m -u 1000 vget
USER vget
WORKDIR /home/vget

EXPOSE 8080
CMD ["vget", "server"]
```

### GPU Detection at Runtime

```go
// internal/core/ai/gpu.go
func DetectGPU() bool {
    // Check for NVIDIA GPU
    cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
    if err := cmd.Run(); err == nil {
        return true
    }
    return false
}
```

---

## Error Handling

| Error | Behavior |
|-------|----------|
| No API key configured | Show setup prompt in UI, link to Settings |
| No GPU + no API key | Show clear message: "Configure API key or use NVIDIA GPU" |
| Transcription API error | Retry with backoff, show error details |
| Chunk fails | Skip chunk, warn user, continue |
| Translation API error | Retry with backoff, fallback to partial |
| Disk space low | Check before job, warn user |

---

## Implementation Phases

### Phase 1: Core Docker Setup
- [ ] Single multi-arch image (amd64/arm64)
- [ ] Runtime GPU detection
- [ ] GitHub Actions for image builds

### Phase 2: Web UI
- [ ] AI capability status indicator (GPU/API)
- [ ] API key configuration in Settings
- [ ] Processing configuration UI
- [ ] Real-time progress stepper
- [ ] Result viewer

### Phase 3: Model Management
- [ ] Model download on demand
- [ ] Support HuggingFace and vmirror sources
- [ ] Model list in Web UI

### Phase 4: Text-to-Speech (IndexTTS)
- [ ] Add IndexTTS Python dependencies to Docker image
- [ ] Implement `/api/ai/tts` endpoint
- [ ] Voice cloning from reference audio
- [ ] Web UI for TTS generation
- [ ] Integration with transcription workflow (transcript → speech)

---

## Success Criteria

1. `docker compose up` starts working web UI
2. UI clearly shows AI mode (GPU local / Cloud API)
3. API configuration is intuitive in Settings
4. File selection and processing works
5. Progress is visible in real-time
6. Transcripts are accurate with timestamps
7. GPU auto-detection works correctly
8. Errors are clear and actionable
9. Single image works for all users (~300MB base)
