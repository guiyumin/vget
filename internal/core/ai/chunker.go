package ai

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/core/ai/transcriber"
)

const (
	// MaxFileSize is the maximum file size for direct transcription (25MB for OpenAI Whisper)
	MaxFileSize = 25 * 1024 * 1024

	// ChunkDuration is the duration of each chunk (10 minutes)
	ChunkDuration = 10 * time.Minute

	// OverlapDuration is the overlap between chunks (10 seconds)
	OverlapDuration = 10 * time.Second
)

// ChunkInfo represents a chunk of audio.
type ChunkInfo struct {
	Index    int           `json:"index"`
	FilePath string        `json:"file"`
	Start    time.Duration `json:"start"`
	End      time.Duration `json:"end"`
	Status   string        `json:"status"` // pending, transcribed, failed
}

// Manifest stores metadata about chunked audio files for resumability.
type Manifest struct {
	Source           string      `json:"source"`
	SourceHash       string      `json:"source_hash"`
	ChunksDir        string      `json:"chunks_dir"`
	CreatedAt        time.Time   `json:"created_at"`
	Strategy         string      `json:"strategy"`
	OverlapSeconds   int         `json:"overlap_seconds"`
	ChunkDurSeconds  int         `json:"chunk_duration_seconds"`
	TotalDurSeconds  float64     `json:"total_duration_seconds"`
	Chunks           []ChunkInfo `json:"chunks"`
}

// Chunker splits large audio files into smaller chunks.
type Chunker struct {
	maxFileSize     int64
	chunkDuration   time.Duration
	overlapDuration time.Duration
}

// NewChunker creates a new Chunker with default settings.
func NewChunker() *Chunker {
	return &Chunker{
		maxFileSize:     MaxFileSize,
		chunkDuration:   ChunkDuration,
		overlapDuration: OverlapDuration,
	}
}

// NewChunkerWithOptions creates a new Chunker with custom settings.
// Zero values for ChunkDuration or Overlap will use defaults.
func NewChunkerWithOptions(opts ChunkOptions) *Chunker {
	c := NewChunker()
	if opts.ChunkDuration > 0 {
		c.chunkDuration = opts.ChunkDuration
	}
	if opts.Overlap > 0 {
		c.overlapDuration = opts.Overlap
	}
	return c
}

// HasFFmpeg checks if ffmpeg is available.
func (c *Chunker) HasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// NeedsChunking checks if a file needs to be chunked.
func (c *Chunker) NeedsChunking(filePath string) (bool, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	return info.Size() > c.maxFileSize, nil
}

// Split splits an audio file into chunks with overlap.
func (c *Chunker) Split(filePath string) ([]ChunkInfo, error) {
	// Get audio duration
	duration, err := c.getAudioDuration(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio duration: %w", err)
	}

	// Calculate chunk boundaries
	var chunks []ChunkInfo
	chunkDur := c.chunkDuration
	overlap := c.overlapDuration
	stride := chunkDur - overlap

	// Create temp directory for chunks
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filepath.Base(filePath), ext)
	chunkDir := filepath.Join(filepath.Dir(filePath), base+".chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunk directory: %w", err)
	}

	// Split into chunks
	for i := 0; ; i++ {
		start := time.Duration(i) * stride
		end := start + chunkDur

		if start >= duration {
			break
		}

		if end > duration {
			end = duration
		}

		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%03d%s", i+1, ext))

		// Use ffmpeg to extract chunk
		if err := c.extractChunk(filePath, chunkPath, start, end-start); err != nil {
			return nil, fmt.Errorf("failed to extract chunk %d: %w", i+1, err)
		}

		chunks = append(chunks, ChunkInfo{
			Index:    i + 1,
			FilePath: chunkPath,
			Start:    start,
			End:      end,
		})
	}

	return chunks, nil
}

// SplitWithManifest splits an audio file and generates a manifest for resumability.
func (c *Chunker) SplitWithManifest(filePath string) ([]ChunkInfo, *Manifest, error) {
	// Get audio duration
	duration, err := c.getAudioDuration(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get audio duration: %w", err)
	}

	// Calculate file hash for integrity
	hash, err := c.calculateFileHash(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Calculate chunk boundaries
	var chunks []ChunkInfo
	chunkDur := c.chunkDuration
	overlap := c.overlapDuration
	stride := chunkDur - overlap

	// Create chunk directory
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filepath.Base(filePath), ext)
	chunkDir := filepath.Join(filepath.Dir(filePath), base+".chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create chunk directory: %w", err)
	}

	// Split into chunks
	for i := 0; ; i++ {
		start := time.Duration(i) * stride
		end := start + chunkDur

		if start >= duration {
			break
		}

		if end > duration {
			end = duration
		}

		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%03d%s", i+1, ext))

		// Use ffmpeg to extract chunk
		if err := c.extractChunk(filePath, chunkPath, start, end-start); err != nil {
			return nil, nil, fmt.Errorf("failed to extract chunk %d: %w", i+1, err)
		}

		chunks = append(chunks, ChunkInfo{
			Index:    i + 1,
			FilePath: chunkPath,
			Start:    start,
			End:      end,
			Status:   "pending",
		})
	}

	// Create manifest
	absPath, _ := filepath.Abs(filePath)
	manifest := &Manifest{
		Source:          absPath,
		SourceHash:      hash,
		ChunksDir:       chunkDir,
		CreatedAt:       time.Now(),
		Strategy:        "overlap",
		OverlapSeconds:  int(c.overlapDuration.Seconds()),
		ChunkDurSeconds: int(c.chunkDuration.Seconds()),
		TotalDurSeconds: duration.Seconds(),
		Chunks:          chunks,
	}

	// Write manifest to file
	manifestPath := filepath.Join(chunkDir, "manifest.json")
	if err := c.writeManifest(manifest, manifestPath); err != nil {
		return nil, nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	return chunks, manifest, nil
}

// calculateFileHash calculates SHA256 hash of a file (first 1MB for speed).
func (c *Chunker) calculateFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	// Read only first 1MB for speed with large files
	if _, err := io.CopyN(h, f, 1024*1024); err != nil && err != io.EOF {
		return "", err
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// writeManifest writes the manifest to a JSON file.
func (c *Chunker) writeManifest(manifest *Manifest, path string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadManifest loads a manifest from the chunks directory.
func LoadManifest(chunksDir string) (*Manifest, error) {
	manifestPath := filepath.Join(chunksDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// getAudioDuration gets the duration of an audio file using ffprobe.
func (c *Chunker) getAudioDuration(filePath string) (time.Duration, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var seconds float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &seconds)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return time.Duration(seconds * float64(time.Second)), nil
}

// extractChunk extracts a portion of audio using ffmpeg.
func (c *Chunker) extractChunk(input, output string, start, duration time.Duration) error {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-ss", formatDuration(start),
		"-i", input,
		"-t", formatDuration(duration),
		"-c", "copy",
		output,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	return nil
}

// formatDuration formats a duration for ffmpeg (HH:MM:SS.mmm).
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := d.Seconds() - float64(hours*3600) - float64(minutes*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", hours, minutes, seconds)
}

// MergeTranscripts merges transcription results from multiple chunks.
func (c *Chunker) MergeTranscripts(results []*transcriber.Result, chunks []ChunkInfo) (*transcriber.Result, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results to merge")
	}

	if len(results) == 1 {
		return results[0], nil
	}

	merged := &transcriber.Result{
		Language: results[0].Language,
	}

	var totalDuration time.Duration
	var allText strings.Builder

	for i, result := range results {
		// Adjust segment timestamps based on chunk start time
		chunkStart := chunks[i].Start

		for _, seg := range result.Segments {
			// Skip segments in the overlap region (except for first chunk)
			if i > 0 && seg.Start < c.overlapDuration {
				continue
			}

			adjusted := transcriber.Segment{
				Start: chunkStart + seg.Start,
				End:   chunkStart + seg.End,
				Text:  seg.Text,
			}
			merged.Segments = append(merged.Segments, adjusted)
		}

		// Concatenate text (with deduplication for overlap)
		text := result.Text
		if i > 0 {
			// Simple deduplication: skip first few words that might overlap
			text = c.removeOverlapText(results[i-1].Text, text)
		}

		if allText.Len() > 0 {
			allText.WriteString(" ")
		}
		allText.WriteString(strings.TrimSpace(text))

		// Track total duration
		if result.Duration > 0 {
			totalDuration += result.Duration
		}
	}

	merged.Text = allText.String()
	merged.Duration = totalDuration

	return merged, nil
}

// removeOverlapText removes overlapping text between chunks.
func (c *Chunker) removeOverlapText(prevText, currText string) string {
	// Simple approach: find common suffix/prefix
	prevWords := strings.Fields(prevText)
	currWords := strings.Fields(currText)

	if len(prevWords) < 5 || len(currWords) < 5 {
		return currText
	}

	// Take last N words from previous chunk
	matchLen := min(20, len(prevWords))
	prevSuffix := prevWords[len(prevWords)-matchLen:]

	// Find match in current chunk
	for i := 0; i < min(matchLen, len(currWords)); i++ {
		match := true
		for j := 0; j < min(3, matchLen-i); j++ {
			if i+j >= len(currWords) || prevSuffix[i+j] != currWords[j] {
				match = false
				break
			}
		}

		if match {
			// Skip matched words
			skipCount := matchLen - i
			if skipCount < len(currWords) {
				return strings.Join(currWords[skipCount:], " ")
			}
		}
	}

	return currText
}

// Cleanup removes temporary chunk files.
func (c *Chunker) Cleanup(chunks []ChunkInfo) {
	if len(chunks) == 0 {
		return
	}

	// Remove chunk directory
	chunkDir := filepath.Dir(chunks[0].FilePath)
	os.RemoveAll(chunkDir)
}
