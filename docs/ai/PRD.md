# PRD: AI Transcription & Summarization

## Overview

AI-powered audio/video transcription and summarization, accessible via Docker web UI.

**Scope:** Audio/Video → Chunk → Transcribe → Translate → Summarize

## User Flow (Web UI)

1. User opens web UI at `http://localhost:8080`
2. Selects audio/video file from downloads or uploads new file
3. Configures AI account (provider, API key, model)
4. Selects processing options (transcribe, translate, summarize)
5. Monitors progress via real-time stepper UI
6. Downloads or views outputs (transcript, translation, SRT, summary)

## Constraints

| Constraint    | Decision                                                      |
| ------------- | ------------------------------------------------------------- |
| Deployment    | Docker only (no standalone CLI)                               |
| Input         | Local audio/video files via web UI                            |
| Transcription | **Local-first** (Parakeet V3, Whisper.cpp), cloud as fallback |
| Translation   | LLM-based (local or cloud)                                    |
| Summarization | LLM-based (local or cloud)                                    |
| Chunking      | Fixed 10-min chunks with 10s overlap                          |
| Output        | Markdown, SRT subtitle files                                  |

---

## Transcription: Local-First Approach

**Why local-first?** Free, private, works offline.

### Dual Engine Architecture

```
┌─────────────────────────────────────────────────────────┐
│                 vget transcriber                         │
├─────────────────────────────────────────────────────────┤
│  Parakeet V3 (sherpa-onnx)  │  Whisper (whisper.cpp)    │
│  - 25 European languages    │  - 99 languages           │
│  - Fastest                  │  - Chinese/Japanese/Korean│
│  - Default engine           │  - GPU accelerated        │
├─────────────────────────────────────────────────────────┤
│  CGO: sherpa-onnx-go        │  CGO: go-whisper          │
├─────────────────────────────────────────────────────────┤
│  libsherpa-onnx-core.so     │  libwhisper.so            │
│  + ONNX Runtime             │  + Metal/CUDA/AVX2        │
└─────────────────────────────────────────────────────────┘
```

### Engine Selection Logic

```
if language == "zh" or detected_language == "zh":
    use Whisper (whisper.cpp) — default for Chinese
elif language in [ja, ko] or detected_language in [ja, ko]:
    use Whisper (whisper.cpp)
else:
    use Parakeet V3 (default for non-CJK)
```

**Note:** User can always override the default in settings.

### Model Options

| Model                  | Engine      | Size   | Languages | Best For      |
| ---------------------- | ----------- | ------ | --------- | ------------- |
| Parakeet V3 INT8       | sherpa-onnx | ~640MB | 25 EU     | Default, fast |
| Whisper Small          | whisper.cpp | ~466MB | 99        | Quick CJK     |
| Whisper Medium         | whisper.cpp | ~1.5GB | 99        | Balanced      |
| Whisper Large V3 Turbo | whisper.cpp | ~1.6GB | 99        | Best accuracy |

### Cloud Fallback (Optional)

Only used when:

- User explicitly selects cloud provider
- Local models not available
- Requires OpenAI API key

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

### Processing Steps

| Step Key         | Name           | Description                                  |
| ---------------- | -------------- | -------------------------------------------- |
| `extract_audio`  | Extract Audio  | Extract audio track from video files         |
| `compress_audio` | Compress Audio | Compress for API upload (< 25MB)             |
| `chunk_audio`    | Chunk Audio    | Split large files into 10-min segments       |
| `transcribe`     | Transcribe     | Speech-to-text (Whisper API or Local ASR)    |
| `merge`          | Merge Chunks   | Combine chunk transcripts with deduplication |
| `translate`      | Translate      | Translate to target language(s)              |
| `generate_srt`   | Generate SRT   | Convert to SRT subtitle format               |
| `summarize`      | Summarize      | Generate summary with key points             |

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
**Provider:** openai/whisper-1

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

---

## Technical Architecture

### Package Structure

```
internal/core/ai/
├── ai.go                 # Main orchestrator
├── chunker.go            # Audio chunking with ffmpeg
├── transcriber/
│   ├── transcriber.go    # Interface and factory
│   ├── sherpa.go         # Parakeet V3 (sherpa-onnx) - default
│   ├── whisper.go        # Whisper (whisper.cpp) - CJK languages
│   ├── openai.go         # OpenAI Whisper API - cloud fallback
│   └── models.go         # Model definitions and management
├── translator/
│   ├── translator.go     # Interface
│   └── openai.go         # LLM-based translation
├── summarizer/
│   ├── summarizer.go     # Interface
│   └── openai.go         # LLM implementation
├── srt/
│   └── generator.go      # SRT file generation
└── output/
    └── markdown.go       # Markdown file generation
```

### Core Interfaces

```go
// Transcriber
type Transcriber interface {
    Transcribe(ctx context.Context, audioPath string) (*Result, error)
}

type Result struct {
    RawText  string
    Segments []Segment
    Language string
    Duration time.Duration
}

type Segment struct {
    Start time.Duration
    End   time.Duration
    Text  string
}

// Translator
type Translator interface {
    Translate(ctx context.Context, text string, targetLang string) (*TranslationResult, error)
    TranslateSegments(ctx context.Context, segments []Segment, targetLang string) ([]TranslatedSegment, error)
}

type TranslationResult struct {
    SourceLanguage string
    TargetLanguage string
    OriginalText   string
    TranslatedText string
    Segments       []TranslatedSegment
}

// Summarizer
type Summarizer interface {
    Summarize(ctx context.Context, text string) (*SummarizationResult, error)
}

type SummarizationResult struct {
    Summary   string
    KeyPoints []string
}
```

### Pipeline Options

```go
type Options struct {
    Transcribe    bool
    Summarize     bool
    TranslateTo   []string  // Target languages: ["en", "zh", "jp"]
    OutputFormat  string    // "text" or "srt"
    SummarizeLang string    // Language for summary output
}
```

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
  account: string;
  model?: string;
  options: {
    transcribe: boolean;
    summarize: boolean;
    translate_to?: string[]; // ["en", "zh"]
    output_format?: "text" | "srt";
    summarize_language?: string;
  };
  pin?: string; // For encrypted API keys
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

interface JobResult {
  transcript_path?: string;
  translated_paths?: Record<string, string>; // { "en": "...", "zh": "..." }
  srt_paths?: Record<string, string>;
  summary_path?: string;
}
```

---

## Web UI Components

### Processing Configuration

```typescript
interface ProcessingConfig {
  account: string;
  model: string;
  transcribe: boolean;
  summarize: boolean;
  translateTo: string[]; // ["en", "zh", "jp"]
  outputFormat: "text" | "srt";
  summarizeInLanguage?: string;
}
```

### Step Display (ProcessingStepper)

```typescript
type StepKey =
  | "extract_audio"
  | "compress_audio"
  | "chunk_audio"
  | "transcribe"
  | "merge"
  | "translate"
  | "generate_srt"
  | "summarize";
```

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

// Translation options
ai_translate_to: "Translate to",
ai_output_format: "Output Format",
ai_format_text: "Text (Markdown)",
ai_format_srt: "Subtitles (SRT)",
```

---

## Translation Feature

### Use Cases

1. **Transcript → Translate → Text**

   - Translate transcript to target language(s)
   - Output: `.{lang}.transcript.md`

2. **Transcript → Translate → SRT**

   - Translate with timestamp preservation
   - Output: `.{lang}.srt`

3. **Transcript → Summarize in Target Language**
   - Summarize directly in target language
   - Output: `.{lang}.summary.md`

### Translation Strategy

1. **Segment-based translation** (for SRT):

   - Translate each segment individually
   - Preserve timestamp information
   - Batch segments to reduce API calls (50 per request)

2. **Full-text translation** (for readability):
   - Translate entire transcript as one unit
   - Better flow and context
   - Used for `.transcript.md` output

### Multi-language Support

- Support multiple target languages in single job
- Parallel translation for multiple languages
- Language codes: ISO 639-1 (en, zh, ja, ko, es, fr, de, pt, ru, ar, etc.)

---

## Error Handling

| Error                     | Behavior                                     |
| ------------------------- | -------------------------------------------- |
| No API key configured     | Show error in UI, prompt to add account      |
| API rate limit            | Retry with exponential backoff (3 attempts)  |
| Chunk transcription fails | Mark chunk as failed, continue, show warning |
| Invalid language code     | Show supported language list                 |
| Translation API error     | Retry with backoff, fallback to partial      |
| SRT timestamp mismatch    | Warn user, use approximation                 |

---

## Docker Configuration

### Multi-Image Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  vget-base (build base)                                     │
│  ├── :latest  - CPU (golang:1.25-bookworm)                  │
│  └── :cuda    - CUDA 12.6 (nvidia/cuda:12.6.3-devel)        │
│                                                             │
│  Contains: Go 1.25, sherpa-onnx libs, whisper.cpp libs      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  vget (application)                                         │
│                                                             │
│  CPU variants:                                              │
│  ├── :latest       - No models (~500MB)                     │
│  ├── :small        - Parakeet + Whisper Small (~1.2GB)      │
│  ├── :medium       - Parakeet + Whisper Medium (~2.0GB)     │
│  └── :large        - Parakeet + Whisper Large Turbo (~2.3GB)│
│                                                             │
│  CUDA variants:                                             │
│  ├── :cuda         - No models + CUDA runtime               │
│  ├── :cuda-small   - Parakeet + Whisper Small + CUDA        │
│  ├── :cuda-medium  - Parakeet + Whisper Medium + CUDA       │
│  └── :cuda-large   - Parakeet + Whisper Large Turbo + CUDA  │
└─────────────────────────────────────────────────────────────┘
```

### Image Variants

| Tag       | Models                            | Size   | Best For                     |
| --------- | --------------------------------- | ------ | ---------------------------- |
| `:latest` | None                              | ~500MB | Download models on first use |
| `:small`  | Parakeet V3 + Whisper Small       | ~1.2GB | NAS <8GB RAM                 |
| `:medium` | Parakeet V3 + Whisper Medium      | ~2.0GB | 8-16GB RAM                   |
| `:large`  | Parakeet V3 + Whisper Large Turbo | ~2.3GB | Best accuracy                |
| `:cuda-*` | Same as above + CUDA              | +2GB   | NVIDIA GPU                   |

### Basic Usage (CPU)

```yaml
# compose.yml
services:
  vget:
    image: ghcr.io/guiyumin/vget:medium
    ports:
      - "8080:8080"
    volumes:
      - ./config:/home/vget/.config/vget
      - ./downloads:/home/vget/downloads
```

### GPU Usage (NVIDIA CUDA)

```yaml
services:
  vget:
    image: ghcr.io/guiyumin/vget:cuda-large
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
              count: 1
              capabilities: [gpu]
```

### Build Args

| Arg             | Values                          | Description         |
| --------------- | ------------------------------- | ------------------- |
| `ENABLE_CUDA`   | `true`/`false`                  | Enable CUDA support |
| `MODEL_VARIANT` | `none`/`small`/`medium`/`large` | Bundle models       |

---

## Implementation Phases

### Phase 1: Core Pipeline ✅

1. Audio chunking with ffmpeg
2. Local transcription (Parakeet V3 + Whisper.cpp)
3. Transcript merging with deduplication
4. Cloud summarization (GPT-4o)
5. Web UI with progress stepper

### Phase 2: Translation & SRT 🚧

1. Translator interface (LLM-based)
2. SRT generator with timestamp preservation
3. Multi-language support
4. UI language selector and format options

### Phase 3: Local LLM

1. Local translation (Ollama, llama.cpp)
2. Local summarization
3. Fully offline pipeline

### Future

- [ ] Additional cloud providers (Anthropic, Qwen)
- [ ] Speaker diarization
- [ ] Resume interrupted processing
- [ ] Batch processing multiple files

---

## Success Criteria

1. Web UI allows file selection and AI processing
2. Progress is visible in real-time via stepper
3. Transcripts are accurate with timestamps
4. Translation preserves meaning and timestamps
5. SRT files are valid and sync with audio
6. Summaries capture key points
7. Large files (>25MB) are automatically chunked
8. Errors are clear and actionable in UI
