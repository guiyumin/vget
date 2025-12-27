package downloader

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/guiyumin/vget/internal/core/config"
)

// HLSConfig holds configuration for HLS downloads
type HLSConfig struct {
	Workers    int // Number of parallel segment downloads
	BufferSize int // Buffer size for reading segments
}

// DefaultHLSConfig returns default HLS configuration
func DefaultHLSConfig() HLSConfig {
	return HLSConfig{
		Workers:    8,
		BufferSize: 512 * 1024, // 512KB
	}
}

// hlsState tracks HLS download progress
type hlsState struct {
	downloaded    int64 // Segments downloaded (atomic)
	totalSegments int64 // Total segments
	bytesWritten  int64 // Total bytes written (atomic)
}

func (s *hlsState) getProgress() (downloaded, total int64) {
	return atomic.LoadInt64(&s.downloaded), s.totalSegments
}

func (s *hlsState) getBytes() int64 {
	return atomic.LoadInt64(&s.bytesWritten)
}

func (s *hlsState) addBytes(n int64) {
	atomic.AddInt64(&s.bytesWritten, n)
}

func (s *hlsState) incDownloaded() {
	atomic.AddInt64(&s.downloaded, 1)
}

// RunHLSDownloadTUI downloads an HLS stream with TUI progress
func RunHLSDownloadTUI(m3u8URL, output, displayID, lang string) error {
	return RunHLSDownloadWithHeadersTUI(m3u8URL, output, displayID, lang, nil)
}

// RunHLSDownloadWithHeadersTUI downloads an HLS stream with custom headers and TUI progress
func RunHLSDownloadWithHeadersTUI(m3u8URL, output, displayID, lang string, headers map[string]string) error {
	state := &downloadState{startTime: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start download in background
	go func() {
		err := downloadHLSWithHeaders(ctx, m3u8URL, output, state, DefaultHLSConfig(), headers)
		if err != nil {
			state.setError(err)
		} else {
			state.setDone()
		}
	}()

	// Run TUI
	model := newDownloadModel(output, displayID, lang, state)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m := finalModel.(downloadModel)
	_, _, _, _, downloadErr := m.state.get()
	if downloadErr != nil {
		return downloadErr
	}

	// Convert .ts to .mp4 in Docker environment
	mp4Path, err := convertTsToMp4(output)
	if err != nil {
		// Log warning but don't fail - the .ts file is still usable
		fmt.Printf("Warning: %v\n", err)
	} else if mp4Path != output {
		fmt.Printf("Converted to: %s\n", mp4Path)
	}

	return nil
}

 

// downloadHLSWithHeaders downloads an HLS stream with custom headers
func downloadHLSWithHeaders(ctx context.Context, m3u8URL, output string, state *downloadState, config HLSConfig, headers map[string]string) error {
	// Parse the m3u8 playlist
	playlist, err := ParseM3U8WithHeaders(m3u8URL, headers)
	if err != nil {
		return fmt.Errorf("failed to parse m3u8: %w", err)
	}

	// If master playlist, get the best variant and parse it
	if playlist.IsMaster {
		variant := playlist.SelectBestVariant()
		if variant == nil {
			return fmt.Errorf("no variants found in master playlist")
		}
		playlist, err = ParseM3U8WithHeaders(variant.URL, headers)
		if err != nil {
			return fmt.Errorf("failed to parse variant playlist: %w", err)
		}
	}

	if len(playlist.Segments) == 0 {
		return fmt.Errorf("no segments found in playlist")
	}

	// Get encryption key if needed
	var decryptKey []byte
	var decryptIV []byte
	if playlist.IsEncrypted && playlist.KeyURL != "" {
		decryptKey, err = fetchKeyWithHeaders(playlist.KeyURL, headers)
		if err != nil {
			return fmt.Errorf("failed to fetch encryption key: %w", err)
		}
		if playlist.KeyIV != "" {
			decryptIV, _ = hex.DecodeString(playlist.KeyIV)
		}
	}

	// Create output file
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Set up progress tracking
	// For HLS we estimate total size (unknown until download complete)
	// We'll use segment count for progress
	totalSegments := int64(len(playlist.Segments))
	hlsState := &hlsState{totalSegments: totalSegments}

	// Progress updater
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				downloaded, total := hlsState.getProgress()
				bytes := hlsState.getBytes()
				// Estimate total bytes based on progress
				if downloaded > 0 {
					estimatedTotal := bytes * total / downloaded
					state.update(bytes, estimatedTotal)
				}
			}
		}
	}()
	defer close(progressDone)

	// Download segments
	// We need to maintain order, so we download in parallel but write sequentially
	err = downloadSegmentsOrdered(ctx, playlist.Segments, file, decryptKey, decryptIV, hlsState, config, headers)
	if err != nil {
		return err
	}

	return nil
}

// downloadSegmentsOrdered downloads segments in parallel but writes them in order
func downloadSegmentsOrdered(ctx context.Context, segments []Segment, file *os.File,
	decryptKey, decryptIV []byte, hlsState *hlsState, config HLSConfig, headers map[string]string) error {

	type segmentResult struct {
		index int
		data  []byte
		err   error
	}

	// Buffer to hold downloaded segments waiting to be written
	results := make(map[int][]byte)
	resultsChan := make(chan segmentResult, config.Workers)
	var resultsLock sync.Mutex

	// Segment queue
	segmentChan := make(chan Segment, len(segments))
	for _, seg := range segments {
		segmentChan <- seg
	}
	close(segmentChan)

	// Create HTTP client
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConnsPerHost: config.Workers * 2,
			DisableCompression:  true,
		},
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seg := range segmentChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				data, err := downloadSegment(client, seg.URL, decryptKey, decryptIV, seg.Index, headers)
				resultsChan <- segmentResult{
					index: seg.Index,
					data:  data,
					err:   err,
				}
			}
		}()
	}

	// Close results channel when all workers done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and write in order
	nextIndex := 0
	var writeErr error

	for result := range resultsChan {
		if result.err != nil {
			writeErr = result.err
			continue
		}

		resultsLock.Lock()
		results[result.index] = result.data
		hlsState.incDownloaded()

		// Write all consecutive segments we have
		for {
			if data, ok := results[nextIndex]; ok {
				_, err := file.Write(data)
				if err != nil {
					writeErr = err
					resultsLock.Unlock()
					break
				}
				hlsState.addBytes(int64(len(data)))
				delete(results, nextIndex)
				nextIndex++
			} else {
				break
			}
		}
		resultsLock.Unlock()
	}

	if writeErr != nil {
		return fmt.Errorf("failed to write segment: %w", writeErr)
	}

	return nil
}

// downloadSegment downloads a single segment
func downloadSegment(client *http.Client, url string, decryptKey, decryptIV []byte, index int, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// Apply custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("segment %d returned status %d", index, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Decrypt if needed
	if decryptKey != nil {
		data, err = decryptAES128(data, decryptKey, decryptIV, index)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt segment %d: %w", index, err)
		}
	}

	return data, nil
}

// fetchKeyWithHeaders fetches the encryption key from the URL with custom headers
func fetchKeyWithHeaders(url string, headers map[string]string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// Apply custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("key server returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// decryptAES128 decrypts AES-128-CBC encrypted data
func decryptAES128(data, key, iv []byte, segmentIndex int) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// If no IV provided, use segment index as IV (per HLS spec)
	if iv == nil {
		iv = make([]byte, 16)
		// Segment sequence number as big-endian 128-bit integer
		iv[15] = byte(segmentIndex)
		iv[14] = byte(segmentIndex >> 8)
		iv[13] = byte(segmentIndex >> 16)
		iv[12] = byte(segmentIndex >> 24)
	}

	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	// Remove PKCS7 padding
	if len(data) > 0 {
		padding := int(data[len(data)-1])
		if padding > 0 && padding <= aes.BlockSize {
			data = data[:len(data)-padding]
		}
	}

	return data, nil
}

// DownloadHLSWithProgress downloads an HLS stream with a progress callback (for server use)
// Returns the final output path (may be .mp4 if converted in Docker) and error
func DownloadHLSWithProgress(ctx context.Context, m3u8URL, output string, headers map[string]string, progressFn func(downloaded, total int64)) (string, error) {
	hlsConfig := DefaultHLSConfig()

	// Parse the m3u8 playlist
	playlist, err := ParseM3U8WithHeaders(m3u8URL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to parse m3u8: %w", err)
	}

	// If master playlist, get the best variant and parse it
	if playlist.IsMaster {
		variant := playlist.SelectBestVariant()
		if variant == nil {
			return "", fmt.Errorf("no variants found in master playlist")
		}
		playlist, err = ParseM3U8WithHeaders(variant.URL, headers)
		if err != nil {
			return "", fmt.Errorf("failed to parse variant playlist: %w", err)
		}
	}

	if len(playlist.Segments) == 0 {
		return "", fmt.Errorf("no segments found in playlist")
	}

	// Get encryption key if needed
	var decryptKey []byte
	var decryptIV []byte
	if playlist.IsEncrypted && playlist.KeyURL != "" {
		decryptKey, err = fetchKeyWithHeaders(playlist.KeyURL, headers)
		if err != nil {
			return "", fmt.Errorf("failed to fetch encryption key: %w", err)
		}
		if playlist.KeyIV != "" {
			decryptIV, _ = hex.DecodeString(playlist.KeyIV)
		}
	}

	// Create output file
	file, err := os.Create(output)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}

	// Set up progress tracking using segment count
	totalSegments := int64(len(playlist.Segments))
	hlsState := &hlsState{totalSegments: totalSegments}

	// Progress updater goroutine
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if progressFn != nil {
					downloaded, total := hlsState.getProgress()
					bytes := hlsState.getBytes()
					// Report actual bytes with estimated total based on segment progress
					if downloaded > 0 && bytes > 0 && total > 0 {
						estimatedTotal := bytes * total / downloaded
						progressFn(bytes, estimatedTotal)
					} else {
						// No estimate yet, report bytes downloaded with unknown total
						progressFn(bytes, -1)
					}
				}
			}
		}
	}()
	defer close(progressDone)

	// Download segments
	err = downloadSegmentsOrdered(ctx, playlist.Segments, file, decryptKey, decryptIV, hlsState, hlsConfig, headers)
	if err != nil {
		file.Close()
		return "", err
	}

	// Close file before conversion (ffmpeg needs exclusive access)
	file.Close()

	// Final progress update - download complete
	if progressFn != nil {
		finalBytes := hlsState.getBytes()
		progressFn(finalBytes, finalBytes)
	}

	// Convert .ts to .mp4 in Docker environment
	finalPath, convErr := convertTsToMp4(output)
	if convErr != nil {
		// Log warning but don't fail - the .ts file is still usable
		fmt.Printf("Warning: %v\n", convErr)
		return output, nil
	}

	return finalPath, nil
}

// convertTsToMp4 converts a .ts file to .mp4 using ffmpeg (copy, no re-encoding)
// Only runs in Docker environment where ffmpeg is available
// Returns the new .mp4 path if conversion succeeded, otherwise returns original path
func convertTsToMp4(tsPath string) (string, error) {
	// Only convert in Docker
	if !config.IsRunningInDocker() {
		return tsPath, nil
	}

	// Only convert .ts files
	if !strings.HasSuffix(strings.ToLower(tsPath), ".ts") {
		return tsPath, nil
	}

	// Build mp4 output path
	mp4Path := strings.TrimSuffix(tsPath, ".ts") + ".mp4"
	if strings.HasSuffix(tsPath, ".TS") {
		mp4Path = strings.TrimSuffix(tsPath, ".TS") + ".mp4"
	}

	// Run ffmpeg with stream copy (fast, no re-encoding)
	// -threads 1: Limit threads to work on resource-constrained systems (Docker, low ulimits)
	cmd := exec.Command("ffmpeg", "-threads", "1", "-i", tsPath, "-c", "copy", "-y", mp4Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Conversion failed, keep the .ts file
		return tsPath, fmt.Errorf("ffmpeg conversion failed: %w\nOutput: %s", err, string(output))
	}

	// Conversion succeeded, delete the .ts file
	if err := os.Remove(tsPath); err != nil {
		// Log but don't fail - the mp4 was created successfully
		fmt.Printf("Warning: could not remove original .ts file: %v\n", err)
	}

	return mp4Path, nil
}
