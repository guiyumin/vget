package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/downloader"
	"github.com/guiyumin/vget/internal/extractor"
	"github.com/guiyumin/vget/internal/version"
)

// Response is the standard API response structure
type Response struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// DownloadRequest is the request body for POST /download
type DownloadRequest struct {
	URL        string `json:"url"`
	Filename   string `json:"filename,omitempty"`
	ReturnFile bool   `json:"return_file,omitempty"`
}

// Server is the HTTP server for vget
type Server struct {
	port      int
	outputDir string
	apiKey    string
	jobQueue  *JobQueue
	cfg       *config.Config
	server    *http.Server
}

// NewServer creates a new HTTP server
func NewServer(port int, outputDir, apiKey string, maxConcurrent int) *Server {
	cfg := config.LoadOrDefault()

	s := &Server{
		port:      port,
		outputDir: outputDir,
		apiKey:    apiKey,
		cfg:       cfg,
	}

	// Create job queue with download function
	s.jobQueue = NewJobQueue(maxConcurrent, outputDir, s.downloadWithExtractor)

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Ensure output directory exists
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Start job queue workers
	s.jobQueue.Start()

	// Setup routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/download", s.handleDownload)
	mux.HandleFunc("/status/", s.handleStatus)
	mux.HandleFunc("/jobs", s.handleJobs)
	mux.HandleFunc("/jobs/", s.handleJobAction)

	// Serve embedded UI if available
	if distFS := GetDistFS(); distFS != nil {
		fileServer := http.FileServer(http.FS(distFS))
		mux.HandleFunc("/", s.handleUI(fileServer, distFS))
		log.Println("Serving embedded WebUI at /")
	}

	// Wrap with auth middleware if API key is set
	var handler http.Handler = mux
	if s.apiKey != "" {
		handler = s.authMiddleware(mux)
	}

	// Add logging middleware
	handler = s.loggingMiddleware(handler)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No timeout for downloads
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting vget server on port %d", s.port)
	log.Printf("Output directory: %s", s.outputDir)
	if s.apiKey != "" {
		log.Printf("API key authentication enabled")
	}

	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	s.jobQueue.Stop()
	return s.server.Shutdown(ctx)
}

// Middleware

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Health endpoint doesn't require auth
		if path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// UI routes don't require auth (static files and SPA routes)
		// API routes start with /download, /status, /jobs
		isAPIRoute := path == "/download" ||
			strings.HasPrefix(path, "/status/") ||
			path == "/jobs" ||
			strings.HasPrefix(path, "/jobs/")

		if !isAPIRoute {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey != s.apiKey {
			s.writeJSON(w, http.StatusUnauthorized, Response{
				Code:    401,
				Data:    nil,
				Message: "invalid or missing API key",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// Handlers

// handleUI serves the embedded SPA with fallback to index.html for client-side routing
func (s *Server) handleUI(fileServer http.Handler, distFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve the file directly
		if path != "/" {
			// Check if file exists
			cleanPath := strings.TrimPrefix(path, "/")
			if _, err := fs.Stat(distFS, cleanPath); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback to index.html for SPA routing
		indexFile, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexFile)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{
			Code:    405,
			Data:    nil,
			Message: "method not allowed",
		})
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Code: 200,
		Data: map[string]string{
			"status":  "ok",
			"version": version.Version,
		},
		Message: "everything is good",
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{
			Code:    405,
			Data:    nil,
			Message: "method not allowed",
		})
		return
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request body",
		})
		return
	}

	if req.URL == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "url is required",
		})
		return
	}

	// If return_file is true, download and stream directly
	if req.ReturnFile {
		s.downloadAndStream(w, req.URL, req.Filename)
		return
	}

	// Otherwise, queue the download
	job, err := s.jobQueue.AddJob(req.URL, req.Filename)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Code: 200,
		Data: map[string]interface{}{
			"id":     job.ID,
			"status": job.Status,
		},
		Message: "download started",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{
			Code:    405,
			Data:    nil,
			Message: "method not allowed",
		})
		return
	}

	// Extract job ID from path: /status/{id}
	id := strings.TrimPrefix(r.URL.Path, "/status/")
	if id == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "job id is required",
		})
		return
	}

	job := s.jobQueue.GetJob(id)
	if job == nil {
		s.writeJSON(w, http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "job not found",
		})
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Code: 200,
		Data: map[string]interface{}{
			"id":       job.ID,
			"status":   job.Status,
			"progress": job.Progress,
			"filename": job.Filename,
			"error":    job.Error,
		},
		Message: string(job.Status),
	})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{
			Code:    405,
			Data:    nil,
			Message: "method not allowed",
		})
		return
	}

	jobs := s.jobQueue.GetAllJobs()

	jobList := make([]map[string]interface{}, len(jobs))
	for i, job := range jobs {
		jobList[i] = map[string]interface{}{
			"id":       job.ID,
			"url":      job.URL,
			"status":   job.Status,
			"progress": job.Progress,
		}
	}

	s.writeJSON(w, http.StatusOK, Response{
		Code: 200,
		Data: map[string]interface{}{
			"jobs": jobList,
		},
		Message: fmt.Sprintf("%d jobs found", len(jobs)),
	})
}

func (s *Server) handleJobAction(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /jobs/{id}
	id := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if id == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "job id is required",
		})
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if s.jobQueue.CancelJob(id) {
			s.writeJSON(w, http.StatusOK, Response{
				Code:    200,
				Data:    map[string]string{"id": id},
				Message: "job cancelled",
			})
		} else {
			s.writeJSON(w, http.StatusNotFound, Response{
				Code:    404,
				Data:    nil,
				Message: "job not found or cannot be cancelled",
			})
		}
	default:
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{
			Code:    405,
			Data:    nil,
			Message: "method not allowed",
		})
	}
}

// Helper functions

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// downloadWithExtractor is the download function used by the job queue
func (s *Server) downloadWithExtractor(ctx context.Context, url, filename string, progressFn func(downloaded, total int64)) error {
	// Find matching extractor
	ext := extractor.Match(url)
	if ext == nil {
		// Try sites.yml for configured sites
		sitesConfig, _ := config.LoadSites()
		if sitesConfig != nil {
			if site := sitesConfig.MatchSite(url); site != nil {
				ext = extractor.NewBrowserExtractor(site, false)
			}
		}
		if ext == nil {
			ext = extractor.NewGenericBrowserExtractor(false)
		}
	}

	// Configure Twitter extractor with auth if available
	if twitterExt, ok := ext.(*extractor.TwitterExtractor); ok {
		if s.cfg.Twitter.AuthToken != "" {
			twitterExt.SetAuth(s.cfg.Twitter.AuthToken)
		}
	}

	// Extract media info
	media, err := ext.Extract(url)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Determine output path based on media type
	var outputPath string
	var downloadURL string
	var headers map[string]string

	switch m := media.(type) {
	case *extractor.VideoMedia:
		if len(m.Formats) == 0 {
			return fmt.Errorf("no video formats available")
		}
		format := selectBestFormat(m.Formats)
		downloadURL = format.URL
		headers = format.Headers

		if filename != "" {
			outputPath = filepath.Join(s.outputDir, filename)
		} else {
			title := extractor.SanitizeFilename(m.Title)
			ext := format.Ext
			if ext == "m3u8" {
				ext = "ts"
			}
			if title != "" {
				outputPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", title, ext))
			} else {
				outputPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", m.ID, ext))
			}
		}

		// Update job filename
		s.updateJobFilename(url, outputPath)

	case *extractor.AudioMedia:
		downloadURL = m.URL

		if filename != "" {
			outputPath = filepath.Join(s.outputDir, filename)
		} else {
			title := extractor.SanitizeFilename(m.Title)
			if title != "" {
				outputPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", title, m.Ext))
			} else {
				outputPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", m.ID, m.Ext))
			}
		}

		s.updateJobFilename(url, outputPath)

	case *extractor.ImageMedia:
		if len(m.Images) == 0 {
			return fmt.Errorf("no images available")
		}
		// Download first image for now
		img := m.Images[0]
		downloadURL = img.URL

		if filename != "" {
			outputPath = filepath.Join(s.outputDir, filename)
		} else {
			title := extractor.SanitizeFilename(m.Title)
			if title != "" {
				outputPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", title, img.Ext))
			} else {
				outputPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", m.ID, img.Ext))
			}
		}

		s.updateJobFilename(url, outputPath)

	default:
		return fmt.Errorf("unsupported media type")
	}

	// Perform download
	return downloadFile(ctx, downloadURL, outputPath, headers, progressFn)
}

func (s *Server) updateJobFilename(url, filename string) {
	// Find job by URL and update filename
	jobs := s.jobQueue.GetAllJobs()
	for _, job := range jobs {
		if job.URL == url {
			s.jobQueue.mu.Lock()
			if j, ok := s.jobQueue.jobs[job.ID]; ok {
				j.Filename = filename
			}
			s.jobQueue.mu.Unlock()
			break
		}
	}
}

// downloadAndStream extracts and streams the file directly to the response
func (s *Server) downloadAndStream(w http.ResponseWriter, url, filename string) {
	// Find matching extractor
	ext := extractor.Match(url)
	if ext == nil {
		sitesConfig, _ := config.LoadSites()
		if sitesConfig != nil {
			if site := sitesConfig.MatchSite(url); site != nil {
				ext = extractor.NewBrowserExtractor(site, false)
			}
		}
		if ext == nil {
			ext = extractor.NewGenericBrowserExtractor(false)
		}
	}

	// Configure Twitter extractor
	if twitterExt, ok := ext.(*extractor.TwitterExtractor); ok {
		if s.cfg.Twitter.AuthToken != "" {
			twitterExt.SetAuth(s.cfg.Twitter.AuthToken)
		}
	}

	// Extract media info
	media, err := ext.Extract(url)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("extraction failed: %v", err),
		})
		return
	}

	var downloadURL string
	var headers map[string]string
	var outputFilename string

	switch m := media.(type) {
	case *extractor.VideoMedia:
		if len(m.Formats) == 0 {
			s.writeJSON(w, http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: "no video formats available",
			})
			return
		}
		format := selectBestFormat(m.Formats)
		downloadURL = format.URL
		headers = format.Headers

		if filename != "" {
			outputFilename = filename
		} else {
			title := extractor.SanitizeFilename(m.Title)
			ext := format.Ext
			if ext == "m3u8" {
				ext = "ts"
			}
			if title != "" {
				outputFilename = fmt.Sprintf("%s.%s", title, ext)
			} else {
				outputFilename = fmt.Sprintf("%s.%s", m.ID, ext)
			}
		}

	case *extractor.AudioMedia:
		downloadURL = m.URL
		if filename != "" {
			outputFilename = filename
		} else {
			title := extractor.SanitizeFilename(m.Title)
			if title != "" {
				outputFilename = fmt.Sprintf("%s.%s", title, m.Ext)
			} else {
				outputFilename = fmt.Sprintf("%s.%s", m.ID, m.Ext)
			}
		}

	case *extractor.ImageMedia:
		if len(m.Images) == 0 {
			s.writeJSON(w, http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: "no images available",
			})
			return
		}
		img := m.Images[0]
		downloadURL = img.URL
		if filename != "" {
			outputFilename = filename
		} else {
			title := extractor.SanitizeFilename(m.Title)
			if title != "" {
				outputFilename = fmt.Sprintf("%s.%s", title, img.Ext)
			} else {
				outputFilename = fmt.Sprintf("%s.%s", m.ID, img.Ext)
			}
		}

	default:
		s.writeJSON(w, http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: "unsupported media type",
		})
		return
	}

	// Stream the file
	streamFile(w, downloadURL, outputFilename, headers)
}

func selectBestFormat(formats []extractor.VideoFormat) *extractor.VideoFormat {
	if len(formats) == 0 {
		return nil
	}

	// Prefer formats with audio
	var bestWithAudio *extractor.VideoFormat
	for i := range formats {
		f := &formats[i]
		if f.AudioURL != "" {
			if bestWithAudio == nil || f.Bitrate > bestWithAudio.Bitrate {
				bestWithAudio = f
			}
		}
	}
	if bestWithAudio != nil {
		return bestWithAudio
	}

	// Fall back to highest bitrate
	best := &formats[0]
	for i := range formats {
		if formats[i].Bitrate > best.Bitrate {
			best = &formats[i]
		}
	}
	return best
}

func downloadFile(ctx context.Context, url, outputPath string, headers map[string]string, progressFn func(downloaded, total int64)) error {
	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if len(headers) > 0 {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	} else {
		req.Header.Set("User-Agent", downloader.DefaultUserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := resp.ContentLength

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Download with progress tracking
	buf := make([]byte, 32*1024)
	var downloaded int64

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("download failed: %w", readErr)
		}
	}

	return nil
}

func streamFile(w http.ResponseWriter, url, filename string, headers map[string]string) {
	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	if len(headers) > 0 {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	} else {
		req.Header.Set("User-Agent", downloader.DefaultUserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "download request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("upstream returned status %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	// Set response headers
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if resp.ContentLength > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	io.Copy(w, resp.Body)
}
