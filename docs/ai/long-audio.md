# Long-Form Audio Transcription (Up to 4 Hours)

vget Internal Technical Specification

## 1. Problem Statement

vget must reliably transcribe long-form audio files (up to 4 hours) on-device, without:

- Excessive memory usage
- UI freezing
- Data loss on crash
- Hallucinations caused by context overflow

The system must support:

- Offline execution
- Resumable processing
- Progress reporting
- Deterministic output suitable for professional use

## 2. Design Constraints

- **Offline-only** (no cloud fallback)
- **Consumer hardware** (MacBook Pro / Air, Apple Silicon)
- **Large models** (Whisper medium / large)
- **Audio duration:** up to 4 hours
- **Memory safety:** must not load entire audio into memory
- **Failure tolerance:** partial progress must persist

## 3. High-Level Strategy

Long audio transcription is treated as a pipeline, not a single inference call.

**Core principles:**

1. Convert audio once into a canonical format
2. Detect silence regions and skip them (VAD)
3. Split audio into overlapping chunks
4. Detect language (per-chunk or global)
5. Transcribe chunks independently
6. Persist results incrementally
7. Merge results with fuzzy deduplication
8. Optionally perform speaker diarization
9. Enable resume from last completed chunk

## 4. Audio Preprocessing

### 4.1 Canonical Audio Format

All input audio is converted to:

- **Mono**
- **16 kHz**
- **PCM WAV**

**Rationale:**

- Matches Whisper's native expectations
- Eliminates resampling variability
- Reduces decode overhead per chunk

```
input.mp3 → mono → 16kHz → PCM → canonical.wav
```

Preprocessing is executed once per recording.

### 4.2 Voice Activity Detection (VAD)

Before chunking, apply VAD to identify:

- **Speech regions** — segments containing voice
- **Silence regions** — gaps > 1 second with no speech

**Benefits:**

- Skip silence during transcription (faster processing)
- Use silence boundaries as natural chunk break points
- Reduce hallucinations in silent regions (Whisper can hallucinate on silence)

**Implementation options:**

- Silero VAD (lightweight, accurate)
- WebRTC VAD (fast, less accurate)
- Energy-based detection (simplest fallback)

## 5. Language Detection

### 5.1 Detection Strategy

Two approaches:

| Strategy | When to Use |
|----------|-------------|
| **Global detection** | Single-language content (podcasts, lectures) |
| **Per-chunk detection** | Multi-language content (interviews, code-switching) |

### 5.2 Implementation

- Use Whisper's built-in language detection on first 30 seconds
- Cache detected language for subsequent chunks (global mode)
- Or detect per-chunk and tag segments with language codes

### 5.3 Language Hints

Allow user to provide language hint to:

- Skip detection overhead
- Force specific language for accuracy
- Handle edge cases (accents, dialects)

## 6. Chunking Strategy

### 6.1 Whisper's Native Limitations

Understanding Whisper's architecture is critical for chunking design:

| Constraint | Value | Explanation |
|------------|-------|-------------|
| **Internal context window** | 30 seconds | Whisper's transformer processes 30-sec audio segments |
| **OpenAI API file limit** | 25 MB | ~60-90 min depending on audio compression |
| **Local model** | No hard limit | Limited only by available memory |

**Key insight:** Whisper doesn't have a "60 second limit" — it has a 30-second *context window*. The model can accept longer audio, but it processes internally in 30-second segments. For local inference, we chunk explicitly to:

- Control memory usage
- Enable progress tracking
- Support resume on failure
- Maintain output quality (avoid context drift)

### 6.2 Chunk Size

| Parameter | Value |
|-----------|-------|
| Chunk length | 45 seconds |
| Overlap | 3–5 seconds |
| Effective stride | ~40 seconds |

### 6.3 Overlap Rationale

Overlap ensures:

- Sentences are not cut mid-phrase
- Whisper context remains stable
- Downstream deduplication is possible

**Example:**

```
Chunk 01: 00:00.0 – 00:45.0
Chunk 02: 00:42.0 – 01:27.0
Chunk 03: 01:24.0 – 02:09.0
```

### 6.4 Smart Chunk Boundaries

Prefer breaking chunks at:

1. VAD-detected silence gaps (best)
2. Sentence boundaries from previous chunk (good)
3. Fixed intervals (fallback)

## 7. Chunk Processing Pipeline

Each chunk is processed independently through the following states:

```
WAITING → PROCESSING → DONE
              ↓
           FAILED → RETRY (max N)
```

### 7.1 Execution Model

- Chunks are processed sequentially or with limited concurrency
- Only one model instance is active per worker
- GPU / Metal memory is released after each chunk

## 8. Incremental Persistence

### 8.1 Why Incremental Writes Matter

For long audio:

- Crashes are inevitable
- Power loss may occur
- Users may quit and resume later

**Therefore:** Every completed chunk is immediately persisted to disk.

### 8.2 Persisted Data

| Field | Description |
|-------|-------------|
| `chunk_index` | Sequential chunk number |
| `time_range` | Start and end timestamps |
| `text` | Transcribed content |
| `language` | Detected language code |
| `confidence` | Model confidence score |
| `status` | DONE / FAILED / INAUDIBLE |

## 9. Failure Handling

### 9.1 Retry Policy

- Each chunk may retry up to **2 times**
- Retry uses identical parameters (determinism)
- After retries exhausted:
  - Chunk is marked as `INAUDIBLE` or `FAILED`
  - Pipeline continues

### 9.2 Failure Isolation

A single chunk failure:

- Must **not** cancel the entire job
- Must **not** corrupt previously completed data

## 10. Resume Capability

On restart:

1. Load persisted chunk metadata
2. Identify first unfinished chunk
3. Resume processing from that point

This allows:

- Resume after crash
- Pause / resume by user
- Long jobs across multiple sessions

## 11. Transcript Assembly

### 11.1 Overlap Deduplication

Overlapping regions produce duplicated text. Simple string matching fails because Whisper output is non-deterministic.

**Fuzzy deduplication strategy:**

1. Extract trailing N words from chunk N
2. Extract leading M words from chunk N+1
3. Compute similarity (Levenshtein distance, cosine similarity, or word-level diff)
4. Find best alignment point with similarity threshold (e.g., > 0.8)
5. Remove duplicated segment from chunk N+1

**Example:**

```
Chunk N ends:   "...and that's why the system works efficiently."
Chunk N+1 starts: "the system works efficiently. Now let's discuss..."
                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                   Detected overlap → removed
```

### 11.2 Timestamp Preservation

Each chunk produces time-aligned segments:

```
[01:32:10 – 01:32:45] Speaker: …
```

Timestamps are retained for:

- Navigation
- Playback alignment
- Summary grounding

## 12. Speaker Diarization

### 12.1 What It Solves

Identifies **who** is speaking, not just **what** was said.

### 12.2 Implementation Options

| Approach | Pros | Cons |
|----------|------|------|
| **pyannote.audio** | State-of-the-art accuracy | Heavy, requires GPU |
| **speechbrain** | Good accuracy, modular | Complex setup |
| **Simple clustering** | Lightweight | Less accurate |

### 12.3 Integration Strategy

1. Run diarization as a **separate pass** after transcription
2. Align speaker segments with transcription timestamps
3. Tag each text segment with speaker ID

**Output format:**

```
[00:01:30 – 00:01:45] Speaker A: "Welcome to the show."
[00:01:45 – 00:02:10] Speaker B: "Thanks for having me."
```

### 12.4 Optional Enhancement

- Allow user to name speakers after initial clustering
- Persist speaker voice embeddings for future recognition

## 13. Output Formats

### 13.1 Supported Formats

vget produces timestamped output in multiple formats:

| Format | Use Case |
|--------|----------|
| **SRT** | Video subtitles, player compatibility |
| **Markdown** | Documentation, readability, editing |
| **JSON** | Programmatic access, downstream processing |
| **Plain text** | Simple export, copy-paste |

### 13.2 SRT Format (with timestamps)

```srt
1
00:00:00,000 --> 00:00:04,500
Welcome to today's episode.

2
00:00:04,500 --> 00:00:08,200
We're going to discuss long-form transcription.
```

### 13.3 Markdown Format (with timestamps)

```markdown
## 00:00:00 - Introduction

Welcome to today's episode. We're going to discuss long-form transcription.

## 00:05:30 - Main Topic

The key insight is that 4-hour transcription is a systems problem...
```

### 13.4 Output Structure

Final transcript is structured as:

- Ordered segments
- Grouped by time windows (1–3 min)
- Suitable for: display, summarization, search, export

**Not** a single monolithic text blob.

## 14. Performance Characteristics

### 14.1 Typical Apple Silicon Performance

| Model | 4h Audio Processing Time |
|-------|--------------------------|
| small | ~30–40 min |
| medium | ~1–1.5 h |
| large | ~2–3 h |

### 14.2 User Experience Requirements

- Visible progress indicator
- Estimated remaining time
- Cancel / pause support
- Background processing option

## 15. Summary Pipeline (Post-Transcription)

Long transcripts are summarized hierarchically:

1. **Chunk-level** mini summaries
2. **Section summaries** (30–45 min blocks)
3. **Global executive summary**

This prevents context overflow and improves summary quality.

## 16. Key Design Guarantees

vget guarantees that long-form transcription:

- Never loads entire audio into memory
- Never blocks the UI thread
- Survives crashes
- Produces deterministic output
- Scales linearly with audio duration

## 17. Summary

**4-hour transcription is not a model problem — it is a systems problem.**

vget solves it through:

- Chunking with smart boundaries
- VAD for silence detection
- Language detection (global or per-chunk)
- Fuzzy overlap deduplication
- Incremental persistence
- Fault isolation
- Resumable execution
- Optional speaker diarization
- Hierarchical summarization
- Multiple output formats (SRT, Markdown with timestamps)

This architecture enables professional-grade, offline transcription on consumer hardware.
