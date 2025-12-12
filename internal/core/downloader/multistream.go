package downloader

import (
	"context"
	"fmt"
	"io"
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
	ChunkSize  int64 // Size of each chunk in bytes (default 16MB)
	BufferSize int   // Buffer size per stream (default 1MB)
	UseHTTP2   bool  // Enable HTTP/2 (default true, better for HTTPS)
}

// DefaultMultiStreamConfig returns sensible defaults similar to rclone
func DefaultMultiStreamConfig() MultiStreamConfig {
	return MultiStreamConfig{
		Streams:    12,               // 12 parallel streams - balanced for stability
		ChunkSize:  8 * 1024 * 1024,  // 8MB chunks - smaller for faster recovery on failure
		BufferSize: 1024 * 1024,      // 1MB buffer per stream
		UseHTTP2:   true,             // Enable HTTP/2 by default for better multiplexing
	}
}

// multiStreamState tracks progress across all streams
type multiStreamState struct {
	downloaded int64 // atomic counter for total bytes downloaded
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
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

// MultiStreamDownload downloads a file using multiple parallel HTTP Range requests
func MultiStreamDownload(ctx context.Context, url, output string, config MultiStreamConfig, state *downloadState) error {
	// Create HTTP client with optimized transport for high-speed downloads
	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        0,                 // Unlimited idle connections
			MaxIdleConnsPerHost: config.Streams*2 + 10,
			MaxConnsPerHost:     0,                 // Unlimited connections per host (like rclone)
			IdleConnTimeout:     120 * time.Second,
			DisableCompression:  true,              // Avoid CPU overhead for already compressed media
			ForceAttemptHTTP2:   config.UseHTTP2,   // Allow HTTP/2 for better multiplexing
			WriteBufferSize:     128 * 1024,        // 128KB write buffer
			ReadBufferSize:      128 * 1024,        // 128KB read buffer
		},
	}

	// Probe for range support and get file size using a small ranged GET
	// Many CDNs only advertise Accept-Ranges on GET, not HEAD
	totalSize, supportsRange, err := probeRangeSupport(ctx, client, url, "")
	if err != nil {
		return fmt.Errorf("failed to probe server: %w", err)
	}

	if totalSize <= 0 {
		return fmt.Errorf("server did not return Content-Length")
	}

	// Fall back to single-stream if range not supported
	if !supportsRange {
		return downloadWithProgress(client, url, output, state, nil)
	}

	state.update(0, totalSize)

	// Create the output file
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Pre-allocate file size for efficiency
	if err := file.Truncate(totalSize); err != nil {
		// Non-fatal, continue anyway
	}

	// Calculate chunks
	chunks := calculateChunks(totalSize, config.Streams, config.ChunkSize)

	// Create multi-stream state
	msState := &multiStreamState{
		total:     totalSize,
		startTime: state.startTime,
	}

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
		chunkChan <- c
	}
	close(chunkChan)

	// Start worker goroutines
	for i := 0; i < config.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range chunkChan {
				if err := downloadChunk(ctx, client, url, file, c, config.BufferSize, msState); err != nil {
					msState.addError(fmt.Errorf("chunk %d failed: %w", c.index, err))
				}
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
		return fmt.Errorf("download failed with %d errors: %v", len(errs), errs[0])
	}

	return nil
}

// calculateChunks divides the file into download chunks
// Uses dynamic chunking - fixed chunk size regardless of file size
// This keeps all workers busy throughout the download
func calculateChunks(totalSize int64, streams int, chunkSize int64) []chunk {
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

// downloadChunk downloads a single chunk using HTTP Range request with resumable retry logic
// Instead of restarting from byte 0 on failure, it resumes from the last successfully written byte
func downloadChunk(ctx context.Context, client *http.Client, url string, file *os.File, c chunk, bufferSize int, state *multiStreamState) error {
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

		bytesWritten, newOffset, err := downloadChunkOnce(ctx, client, url, file, subChunk, bufferSize, state)
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

		// If we've made no progress at all in this attempt, count it as a real failure
		// Otherwise, reset attempt counter since we made progress
		if bytesWritten > 0 {
			attempt = 0 // Reset retries when we make progress
		}
	}

	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// downloadChunkOnce performs a single attempt to download a chunk
// Returns bytes written, final offset position, and any error
func downloadChunkOnce(ctx context.Context, client *http.Client, url string, file *os.File, c chunk, bufferSize int, state *multiStreamState) (int64, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, c.start, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", c.start, c.end))

	resp, err := client.Do(req)
	if err != nil {
		return 0, c.start, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return 0, c.start, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf := make([]byte, bufferSize)
	offset := c.start
	expectedEnd := c.end + 1 // end is inclusive
	var totalWritten int64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			// Write at specific offset (thread-safe with pwrite)
			written, writeErr := file.WriteAt(buf[:n], offset)
			if writeErr != nil {
				return totalWritten, offset, fmt.Errorf("write failed: %w", writeErr)
			}
			offset += int64(written)
			totalWritten += int64(written)
			state.addBytes(int64(written))
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

// RunMultiStreamDownloadTUI runs a multi-stream download with TUI progress
func RunMultiStreamDownloadTUI(url, output, displayID, lang string, config MultiStreamConfig) error {
	state := &downloadState{
		startTime: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start download in background
	go func() {
		err := MultiStreamDownload(ctx, url, output, config, state)
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
	_, _, _, _, downloadErr := m.state.get()
	if downloadErr != nil {
		return downloadErr
	}

	return nil
}

// MultiStreamDownloadWithAuth downloads a file using multiple parallel HTTP Range requests with auth
func MultiStreamDownloadWithAuth(ctx context.Context, url, authHeader, output string, totalSize int64, config MultiStreamConfig, state *downloadState) error {
	// Create HTTP client with optimized transport for high-speed downloads
	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        0,                 // Unlimited idle connections
			MaxIdleConnsPerHost: config.Streams*2 + 10,
			MaxConnsPerHost:     0,                 // Unlimited connections per host (like rclone)
			IdleConnTimeout:     120 * time.Second,
			DisableCompression:  true,              // Avoid CPU overhead for already compressed media
			ForceAttemptHTTP2:   config.UseHTTP2,   // Allow HTTP/2 for better multiplexing
			WriteBufferSize:     128 * 1024,        // 128KB write buffer
			ReadBufferSize:      128 * 1024,        // 128KB read buffer
		},
	}

	// Probe for range support using ranged GET (more reliable than HEAD)
	_, supportsRange, err := probeRangeSupport(ctx, client, url, authHeader)
	if err != nil {
		// If probe fails, assume range is supported (we have totalSize from caller)
		supportsRange = true
	}

	state.update(0, totalSize)

	// If no Range support, fall back to single-stream
	if !supportsRange {
		return downloadWithAuthSingleStream(ctx, client, url, authHeader, output, totalSize, state)
	}

	// Create the output file
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Pre-allocate file size for efficiency
	if err := file.Truncate(totalSize); err != nil {
		// Non-fatal, continue anyway
	}

	// Calculate chunks
	chunks := calculateChunks(totalSize, config.Streams, config.ChunkSize)

	// Create multi-stream state
	msState := &multiStreamState{
		total:     totalSize,
		startTime: state.startTime,
	}

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
		chunkChan <- c
	}
	close(chunkChan)

	// Start worker goroutines
	for i := 0; i < config.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range chunkChan {
				if err := downloadChunkWithAuth(ctx, client, url, authHeader, file, c, config.BufferSize, msState); err != nil {
					msState.addError(fmt.Errorf("chunk %d failed: %w", c.index, err))
				}
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
		return fmt.Errorf("download failed with %d errors: %v", len(errs), errs[0])
	}

	return nil
}

// downloadChunkWithAuth downloads a single chunk using HTTP Range request with auth
// It includes resumable retry logic - on failure, it resumes from the last written byte
func downloadChunkWithAuth(ctx context.Context, client *http.Client, url, authHeader string, file *os.File, c chunk, bufferSize int, state *multiStreamState) error {
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

		bytesWritten, newOffset, err := downloadChunkWithAuthOnce(ctx, client, url, authHeader, file, subChunk, bufferSize, state)
		if err == nil {
			return nil // Success!
		}

		lastErr = err
		// Update currentStart to resume from where we left off
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

// downloadChunkWithAuthOnce performs a single attempt to download a chunk
// Returns bytes written, final offset, and any error
func downloadChunkWithAuthOnce(ctx context.Context, client *http.Client, url, authHeader string, file *os.File, c chunk, bufferSize int, state *multiStreamState) (int64, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, c.start, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", c.start, c.end))
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, c.start, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return 0, c.start, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf := make([]byte, bufferSize)
	offset := c.start
	expectedEnd := c.end + 1 // end is inclusive
	var totalWritten int64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			// Write at specific offset (thread-safe with pwrite)
			written, writeErr := file.WriteAt(buf[:n], offset)
			if writeErr != nil {
				return totalWritten, offset, fmt.Errorf("write failed: %w", writeErr)
			}
			offset += int64(written)
			totalWritten += int64(written)
			// Update progress in real-time
			state.addBytes(int64(written))
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
func downloadWithAuthSingleStream(ctx context.Context, client *http.Client, url, authHeader, output string, total int64, state *downloadState) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
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
	file, err := os.Create(output)
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
	_, _, _, _, downloadErr := m.state.get()
	if downloadErr != nil {
		return downloadErr
	}

	return nil
}
