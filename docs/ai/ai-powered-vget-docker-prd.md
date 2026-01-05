# PRD: AI-Powered vget Docker

## Overview

Docker deployment for vget with AI capabilities, featuring a web UI for media processing.

See [ai-powered-vget-prd.md](./ai-powered-vget-prd.md) for shared concepts.

---

## User Flow (Web UI)

1. User opens web UI at `http://localhost:8080`
2. Selects audio/video file from downloads or uploads new file
3. Configures AI options (transcribe, translate, summarize)
4. Selects processing options (engine, model, language)
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
│  ├── sherpa-onnx binary (Parakeet models)                        │
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
| NVIDIA RTX | 10-30x faster than CPU | CUDA (whisper.cpp, sherpa-onnx) | ✅ Yes |
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
    model?: string;  // e.g., "whisper-large-v3-turbo", "parakeet-v3"
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

---

## Web UI Components

### Processing Configuration

```typescript
interface ProcessingConfig {
  transcribe: boolean;
  summarize: boolean;
  translateTo: string[];      // ["en", "zh", "jp"]
  model: string;              // "auto", "whisper-large-v3-turbo", "parakeet-v3", etc.
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
ai_model_parakeet: "Parakeet V3 (fast, European)",
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
