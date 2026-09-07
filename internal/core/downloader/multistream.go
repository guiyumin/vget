package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MultiStreamConfig configures multi-stream downloads
type MultiStreamConfig struct {
	Streams    int   // Number of parallel streams (default 12)
	ChunkSize  int64 // Size of each chunk in bytes (default 8MB)
	BufferSize int   // Buffer size per stream (default 1MB)
	UseHTTP2   bool  // Enable HTTP/2 (default true, better for HTTPS)

	// DialContext, if set, replaces the transport's dialer, e.g. to reach a
	// library through a tailcat tunnel. HTTP/2 and proxies are then disabled.
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

// DefaultMultiStreamConfig returns sensible defaults similar to rclone
func DefaultMultiStreamConfig() MultiStreamConfig {
	return MultiStreamConfig{
		Streams:    12,              // 12 parallel streams - balanced for stability
		ChunkSize:  8 * 1024 * 1024, // 8MB chunks - smaller for faster recovery on failure
		BufferSize: 1024 * 1024,     // 1MB buffer per stream
		UseHTTP2:   true,            // Enable HTTP/2 by default for better multiplexing
	}
}

// newTransport builds the HTTP transport used for high-speed downloads.
func newTransport(config MultiStreamConfig) *http.Transport {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        0, // Unlimited idle connections
		MaxIdleConnsPerHost: config.Streams*2 + 10,
		MaxConnsPerHost:     0, // Unlimited connections per host (like rclone)
		IdleConnTimeout:     120 * time.Second,
		DisableCompression:  true,            // Avoid CPU overhead for already compressed media
		ForceAttemptHTTP2:   config.UseHTTP2, // Allow HTTP/2 for better multiplexing
		WriteBufferSize:     128 * 1024,      // 128KB write buffer
		ReadBufferSize:      128 * 1024,      // 128KB read buffer
	}
	if config.DialContext != nil {
		tr.DialContext = config.DialContext
		tr.Proxy = nil
		tr.ForceAttemptHTTP2 = false
	}
	return tr
}

// multiStreamState tracks progress across all streams
type multiStreamState struct {
	downloaded int64 // atomic counter for total bytes downloaded (including resumed)
	total      int64
	startTime  time.Time
	mu         sync.RWMutex
	errors     []error
}

func (s *multiStreamState) addBytes(n int64) {
	atomic.AddInt64(&s.downloaded, n)
}

func (s *multiStreamState) getDownloaded() int64 {
	return atomic.LoadInt64(&s.downloaded)
}

func (s *multiStreamState) addError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, err)
}

func (s *multiStreamState) getErrors() []error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errors
}

// chunk represents a portion of the file to download
type chunk struct {
	index int
	start int64
	end   int64 // inclusive
}

// probeRangeSupport checks if the server supports Range requests using a small ranged GET
// This is more reliable than HEAD because many CDNs only advertise Accept-Ranges on GET
// Returns: totalSize, supportsRange, error
func probeRangeSupport(ctx context.Context, client *http.Client, url, authHeader string) (int64, bool, error) {
	// First try a ranged GET request for just 2 bytes
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Range", "bytes=0-1")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	// Drain the small response body
	io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Server supports ranges - parse Content-Range for total size
		// Format: bytes 0-1/total
		contentRange := resp.Header.Get("Content-Range")
		var start, end, total int64
		if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &total); err == nil {
			return total, true, nil
		}
		// Couldn't parse Content-Range, fall back to HEAD
		return probeWithHEAD(ctx, client, url, authHeader)

	case http.StatusOK:
		// Server returned 200 instead of 206 - doesn't support ranges
		// But we can get the size from Content-Length
		return resp.ContentLength, false, nil

	case http.StatusRequestedRangeNotSatisfiable:
		// 416 means server supports ranges but our range was invalid
		// This shouldn't happen for bytes=0-1, but fall back to HEAD
		return probeWithHEAD(ctx, client, url, authHeader)

	case http.StatusUnauthorized:
		return 0, false, errors.New("unauthorized (401): check the user name and password")

	case http.StatusForbidden:
		return 0, false, errors.New("forbidden (403)")

	case http.StatusNotFound:
		return 0, false, errors.New("not found (404)")

	default:
		return 0, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// probeWithHEAD is a fallback that uses HEAD request to get file size
func probeWithHEAD(ctx context.Context, client *http.Client, url, authHeader string) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	resp.Body.Close()

	supportsRange := resp.Header.Get("Accept-Ranges") == "bytes"
	return resp.ContentLength, supportsRange, nil
}

// calculateChunks divides the file into download chunks
// Uses dynamic chunking - fixed chunk size regardless of file size
// This keeps all workers busy throughout the download
func calculateChunks(totalSize int64, chunkSize int64) []chunk {
	var chunks []chunk

	// If file is small, just use one chunk
	if totalSize <= chunkSize {
		return []chunk{{index: 0, start: 0, end: totalSize - 1}}
	}

	// Dynamic chunking: use fixed chunk size, create as many chunks as needed
	// For a 13.5GB file with 64MB chunks = ~210 chunks
	// With 12 workers, each processes ~17 chunks, staying busy throughout
	var start int64
	index := 0
	for start < totalSize {
		end := start + chunkSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}
		chunks = append(chunks, chunk{
			index: index,
			start: start,
			end:   end,
		})
		start = end + 1
		index++
	}

	return chunks
}

// ---- resumable partial downloads ----

// ResumeHint is appended to errors of interrupted downloads whose partial
// file was kept.
const ResumeHint = "partial download kept; run the same command again to resume"

// partMeta describes a partial download (<output>.part.json). A later run
// with the same output name and file size skips the chunks already done.
type partMeta struct {
	Size      int64  `json:"size"`
	ChunkSize int64  `json:"chunk_size"`
	Chunks    int    `json:"chunks"`
	Done      []bool `json:"done"`
}

// resumable is the partial output file plus its chunk bookkeeping.
type resumable struct {
	file     *os.File
	partPath string
	metaPath string
	resumed  bool

	mu   sync.Mutex
	meta partMeta
}

// openResumable opens <output>.part for writing, continuing a previous
// attempt at the same file if its metadata matches, else starting over.
func openResumable(partPath string, size, chunkSize int64, nChunks int) (*resumable, error) {
	r := &resumable{partPath: partPath, metaPath: partPath + ".json"}
	if data, err := os.ReadFile(r.metaPath); err == nil {
		var m partMeta
		if json.Unmarshal(data, &m) == nil && m.Size == size && m.ChunkSize == chunkSize &&
			m.Chunks == nChunks && len(m.Done) == nChunks {
			if fi, err := os.Stat(partPath); err == nil && fi.Size() == size {
				if f, err := os.OpenFile(partPath, os.O_RDWR, 0o644); err == nil {
					r.file, r.meta, r.resumed = f, m, true
					return r, nil
				}
			}
		}
	}
	os.Remove(r.metaPath)
	f, err := os.Create(partPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	// Pre-allocate for efficiency; non-fatal if the filesystem can't.
	_ = f.Truncate(size)
	r.file = f
	r.meta = partMeta{Size: size, ChunkSize: chunkSize, Chunks: nChunks, Done: make([]bool, nChunks)}
	r.save()
	return r, nil
}

func (r *resumable) isDone(i int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.meta.Done[i]
}

func (r *resumable) markDone(i int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.meta.Done[i] = true
	r.save()
}

// doneBytes returns how many bytes the finished chunks cover.
func (r *resumable) doneBytes(chunks []chunk) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, c := range chunks {
		if r.meta.Done[c.index] {
			n += c.end - c.start + 1
		}
	}
	return n
}

// save writes the metadata atomically. r.mu must be held.
func (r *resumable) save() {
	data, err := json.Marshal(r.meta)
	if err != nil {
		return
	}
	tmp := r.metaPath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, r.metaPath)
	}
}

// abandon closes the part file, keeping it and its metadata for a later resume.
func (r *resumable) abandon() {
	r.file.Close()
}

// finish closes the part file and moves it to output, dropping the metadata.
func (r *resumable) finish(output string) error {
	if err := r.file.Close(); err != nil {
		return err
	}
	os.Remove(r.metaPath)
	if err := os.Rename(r.partPath, output); err != nil {
		return fmt.Errorf("failed to move finished download into place: %w", err)
	}
	return nil
}

// MultiStreamDownloadWithAuth downloads url to output using parallel HTTP
// Range requests. The download is written to <output>.part and an
// interrupted download of the same file is resumed on the next run. If
// totalSize is 0 the size is probed from the server.
func MultiStreamDownloadWithAuth(ctx context.Context, url, authHeader, output string, totalSize int64, config MultiStreamConfig, state *downloadState) error {
	client := &http.Client{Timeout: 0, Transport: newTransport(config)}

	// Probe for range support using ranged GET (more reliable than HEAD)
	probedSize, supportsRange, err := probeRangeSupport(ctx, client, url, authHeader)
	if err != nil {
		if totalSize <= 0 {
			return fmt.Errorf("failed to probe server: %w", err)
		}
		// If probe fails, assume range is supported (we have totalSize from caller)
		supportsRange = true
	} else if totalSize <= 0 {
		totalSize = probedSize
	}
	if totalSize <= 0 {
		return fmt.Errorf("server did not return the file size")
	}

	state.update(0, totalSize)
	partPath := output + ".part"

	// If no Range support, fall back to single-stream
	if !supportsRange {
		return downloadWithAuthSingleStream(ctx, client, url, authHeader, output, partPath, totalSize, state)
	}

	chunks := calculateChunks(totalSize, config.ChunkSize)
	res, err := openResumable(partPath, totalSize, config.ChunkSize, len(chunks))
	if err != nil {
		return err
	}

	// Create multi-stream state, starting from what a previous run already fetched
	resumed := res.doneBytes(chunks)
	state.setBaseline(resumed)
	msState := &multiStreamState{
		downloaded: resumed,
		total:      totalSize,
		startTime:  state.startTime,
	}
	state.update(resumed, totalSize)

	// Start progress updater goroutine
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				state.update(msState.getDownloaded(), totalSize)
			}
		}
	}()

	// Download chunks in parallel using a worker pool
	var wg sync.WaitGroup
	chunkChan := make(chan chunk, len(chunks))

	// Feed chunks to the channel
	for _, c := range chunks {
		if !res.isDone(c.index) {
			chunkChan <- c
		}
	}
	close(chunkChan)

	// Start worker goroutines
	for i := 0; i < config.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range chunkChan {
				if err := downloadChunk(ctx, client, url, authHeader, res.file, c, config.BufferSize, msState); err != nil {
					msState.addError(fmt.Errorf("chunk %d failed: %w", c.index, err))
					continue
				}
				res.markDone(c.index)
			}
		}()
	}

	// Wait for all downloads to complete
	wg.Wait()
	close(progressDone)

	// Final progress update
	state.update(msState.getDownloaded(), totalSize)

	// Check for errors
	if errs := msState.getErrors(); len(errs) > 0 {
		res.abandon()
		return fmt.Errorf("download failed with %d errors: %v (%s)", len(errs), errs[0], ResumeHint)
	}

	if err := res.finish(output); err != nil {
		return err
	}
	state.setFinalPath(RenameByMagicBytes(output))
	return nil
}

// downloadChunk downloads a single chunk using HTTP Range request with resumable retry logic
// Instead of restarting from byte 0 on failure, it resumes from the last successfully written byte
func downloadChunk(ctx context.Context, client *http.Client, url, authHeader string, file *os.File, c chunk, bufferSize int, state *multiStreamState) error {
	const maxRetries = 10 // More retries since we resume, not restart
	var lastErr error
	currentStart := c.start // Track where we are in the chunk

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Shorter backoff since we're resuming: 500ms, 1s, 2s, 4s... capped at 8s
			backoff := time.Duration(1<<uint(attempt-1)) * 500 * time.Millisecond
			if backoff > 8*time.Second {
				backoff = 8 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Create a sub-chunk from current position to end
		subChunk := chunk{
			index: c.index,
			start: currentStart,
			end:   c.end,
		}

		bytesWritten, newOffset, err := downloadChunkOnce(ctx, client, url, authHeader, file, subChunk, bufferSize, state)
		if err == nil {
			return nil // Success!
		}

		lastErr = err
		// Update currentStart to resume from where we left off
		// bytesWritten already added to state, so we keep that progress
		if bytesWritten > 0 {
			currentStart = newOffset
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Reset attempt counter when we make progress
		if bytesWritten > 0 {
			attempt = 0
		}
	}

	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// downloadChunkOnce performs a single attempt to download a chunk
// Returns bytes written, final offset position, and any error
func downloadChunkOnce(ctx context.Context, client *http.Client, url, authHeader string, file *os.File, c chunk, bufferSize int, state *multiStreamState) (int64, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, c.start, err
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", c.start, c.end))
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, c.start, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return 0, c.start, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf := make([]byte, bufferSize)
	offset := c.start
	expectedEnd := c.end + 1 // end is inclusive
	var totalWritten int64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			// Never write past the chunk: a misbehaving server could
			// otherwise clobber a neighbouring chunk's bytes.
			if int64(n) > expectedEnd-offset {
				n = int(expectedEnd - offset)
			}
			// Write at specific offset (thread-safe with pwrite)
			written, writeErr := file.WriteAt(buf[:n], offset)
			if writeErr != nil {
				return totalWritten, offset, fmt.Errorf("write failed: %w", writeErr)
			}
			offset += int64(written)
			totalWritten += int64(written)
			state.addBytes(int64(written))
			if offset >= expectedEnd {
				break
			}
		}
		if readErr == io.EOF {
			// Verify we got the full chunk
			if offset < expectedEnd {
				return totalWritten, offset, fmt.Errorf("incomplete: got %d/%d bytes", offset-c.start, expectedEnd-c.start)
			}
			break
		}
		if readErr != nil {
			return totalWritten, offset, fmt.Errorf("read failed: %w", readErr)
		}
	}

	return totalWritten, offset, nil
}

// downloadWithAuthSingleStream falls back to single-stream download when Range not supported
func downloadWithAuthSingleStream(ctx context.Context, client *http.Client, url, authHeader, output, partPath string, total int64, state *downloadState) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create output file
	os.Remove(partPath + ".json")
	file, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Download with progress tracking
	buf := make([]byte, 128*1024) // 128KB buffer
	var current int64

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			current += int64(n)
			state.update(current, total)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	// Close file, move into place and rename by magic bytes if needed
	file.Close()
	if err := os.Rename(partPath, output); err != nil {
		return fmt.Errorf("failed to move finished download into place: %w", err)
	}
	state.setFinalPath(RenameByMagicBytes(output))

	return nil
}

// RunMultiStreamDownloadWithAuthTUI runs a multi-stream download with auth and TUI progress
func RunMultiStreamDownloadWithAuthTUI(url, authHeader, output, displayID, lang string, totalSize int64, config MultiStreamConfig) error {
	state := &downloadState{
		startTime: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start download in background
	go func() {
		err := MultiStreamDownloadWithAuth(ctx, url, authHeader, output, totalSize, config, state)
		if err != nil {
			state.setError(err)
		} else {
			state.setDone()
		}
	}()

	model := newDownloadModel(output, displayID, lang, state)

	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		cancel()
		return err
	}

	m := finalModel.(downloadModel)
	_, _, _, done, downloadErr := m.state.get()
	if downloadErr != nil {
		return downloadErr
	}
	if !done {
		// The user quit the progress view; stop the workers so the partial
		// file is left in a consistent state for a later resume.
		cancel()
		return fmt.Errorf("download cancelled (%s)", ResumeHint)
	}

	return nil
}
