package server

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/downloader"
	"github.com/guiyumin/vget/internal/core/extractor"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/tracker"
	"github.com/guiyumin/vget/internal/core/version"
	"github.com/guiyumin/vget/internal/core/webdav"
	"github.com/guiyumin/vget/internal/torrent"
)

// Response is the standard API response structure
type Response struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// DownloadRequest is the request body for POST /download
type DownloadRequest struct {
	URL        string `json:"url" binding:"required"`
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
	engine    *gin.Engine
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
	// Warn if no config file exists
	if !config.Exists() {
		lang := s.cfg.Language
		if lang == "" {
			lang = "zh"
		}
		t := i18n.GetTranslations(lang)
		log.Printf("⚠️  %s", t.Server.NoConfigWarning)
		log.Printf("   %s", t.Server.RunInitHint)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Start job queue workers
	s.jobQueue.Start()

	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin engine
	s.engine = gin.New()

	// Add middleware
	s.engine.Use(gin.Recovery())
	s.engine.Use(s.loggingMiddleware())
	if s.apiKey != "" {
		s.engine.Use(s.authMiddleware())
	}

	// API routes
	api := s.engine.Group("/api")
	api.GET("/health", s.handleHealth)
	api.POST("/download", s.handleDownload)
	api.GET("/status/:id", s.handleStatus)
	api.GET("/jobs", s.handleGetJobs)
	api.DELETE("/jobs", s.handleClearJobs)
	api.DELETE("/jobs/:id", s.handleDeleteJob)
	api.GET("/config", s.handleGetConfig)
	api.POST("/config", s.handleSetConfig)
	api.PUT("/config", s.handleUpdateConfig)
	api.GET("/config/webdav", s.handleGetWebDAV)
	api.POST("/config/webdav", s.handleAddWebDAV)
	api.DELETE("/config/webdav/:name", s.handleDeleteWebDAV)
	api.GET("/i18n", s.handleI18n)
	api.POST("/kuaidi100", s.handleKuaidi100)

	// WebDAV browsing routes
	api.GET("/webdav/remotes", s.handleWebDAVRemotes)
	api.GET("/webdav/list", s.handleWebDAVList)
	api.POST("/webdav/download", s.handleWebDAVDownload)

	// Torrent dispatch routes
	api.GET("/config/torrent", s.handleGetTorrentConfig)
	api.POST("/config/torrent", s.handleSetTorrentConfig)
	api.POST("/config/torrent/test", s.handleTestTorrentConnection)
	api.POST("/torrent", s.handleAddTorrent)
	api.GET("/torrent", s.handleListTorrents)

	// Podcast search routes
	api.POST("/podcast/search", s.handlePodcastSearch)
	api.POST("/podcast/episodes", s.handlePodcastEpisodes)

	// Serve embedded UI if available
	if distFS := GetDistFS(); distFS != nil {
		s.setupStaticFiles(distFS)
		log.Println("Serving embedded WebUI at /")
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.engine,
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

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Health endpoint doesn't require auth
		if path == "/api/health" {
			c.Next()
			return
		}

		// Only API routes require auth (those under /api prefix)
		// Exclude /api/download and /api/jobs which need auth
		isProtectedAPIRoute := path == "/api/download" ||
			strings.HasPrefix(path, "/api/status/") ||
			path == "/api/jobs" ||
			strings.HasPrefix(path, "/api/jobs/")

		if !isProtectedAPIRoute {
			c.Next()
			return
		}

		apiKey := c.GetHeader("X-API-Key")
		if apiKey != s.apiKey {
			c.JSON(http.StatusUnauthorized, Response{
				Code:    401,
				Data:    nil,
				Message: "invalid or missing API key",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("%s %s %s", c.Request.Method, c.Request.URL.Path, time.Since(start))
	}
}

// setupStaticFiles serves the embedded SPA with fallback to index.html
func (s *Server) setupStaticFiles(distFS fs.FS) {
	// Serve static assets
	s.engine.GET("/assets/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Request.URL.Path, http.FS(distFS))
	})

	// Serve other static files (favicon, etc)
	s.engine.GET("/vite.svg", func(c *gin.Context) {
		c.FileFromFS("vite.svg", http.FS(distFS))
	})

	// Fallback to index.html for SPA routing
	s.engine.NoRoute(func(c *gin.Context) {
		// Only serve index.html for non-API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, Response{
				Code:    404,
				Data:    nil,
				Message: "not found",
			})
			return
		}

		indexFile, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, string(indexFile))
	})
}

// Handlers

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"status":  "ok",
			"version": version.Version,
		},
		Message: "everything is good",
	})
}

func (s *Server) handleDownload(c *gin.Context) {
	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request body: url is required",
		})
		return
	}

	// If return_file is true, download and stream directly
	if req.ReturnFile {
		s.downloadAndStream(c, req.URL, req.Filename)
		return
	}

	// Otherwise, queue the download
	job, err := s.jobQueue.AddJob(req.URL, req.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"id":     job.ID,
			"status": job.Status,
		},
		Message: "download started",
	})
}

func (s *Server) handleStatus(c *gin.Context) {
	id := c.Param("id")

	job := s.jobQueue.GetJob(id)
	if job == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "job not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"id":       job.ID,
			"status":   job.Status,
			"progress": job.Progress,
			"filename": job.Filename,
			"error":    job.Error,
		},
		Message: string(job.Status),
	})
}

func (s *Server) handleGetJobs(c *gin.Context) {
	jobs := s.jobQueue.GetAllJobs()

	jobList := make([]gin.H, len(jobs))
	for i, job := range jobs {
		jobList[i] = gin.H{
			"id":         job.ID,
			"url":        job.URL,
			"status":     job.Status,
			"progress":   job.Progress,
			"downloaded": job.Downloaded,
			"total":      job.Total,
			"filename":   job.Filename,
			"error":      job.Error,
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"jobs": jobList,
		},
		Message: fmt.Sprintf("%d jobs found", len(jobs)),
	})
}

func (s *Server) handleClearJobs(c *gin.Context) {
	count := s.jobQueue.ClearHistory()
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"cleared": count,
		},
		Message: fmt.Sprintf("%d jobs cleared", count),
	})
}

func (s *Server) handleDeleteJob(c *gin.Context) {
	id := c.Param("id")

	// Try to cancel active job first, then try to remove finished job
	if s.jobQueue.CancelJob(id) {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Data:    gin.H{"id": id},
			Message: "job cancelled",
		})
	} else if s.jobQueue.RemoveJob(id) {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Data:    gin.H{"id": id},
			Message: "job removed",
		})
	} else {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "job not found or cannot be cancelled/removed",
		})
	}
}

// ConfigSetRequest is the request body for POST /config
type ConfigSetRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

// ConfigRequest is the request body for PUT /config
type ConfigRequest struct {
	OutputDir string `json:"output_dir,omitempty"`
}

func (s *Server) handleGetConfig(c *gin.Context) {
	cfg := config.LoadOrDefault()

	// Convert WebDAV servers to a simpler format for JSON
	webdavServers := make(map[string]map[string]string)
	for name, server := range cfg.WebDAVServers {
		webdavServers[name] = map[string]string{
			"url":      server.URL,
			"username": server.Username,
			"password": server.Password,
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"output_dir":            s.outputDir,
			"language":              cfg.Language,
			"format":                cfg.Format,
			"quality":               cfg.Quality,
			"twitter_auth_token":    cfg.Twitter.AuthToken,
			"server_port":           cfg.Server.Port,
			"server_max_concurrent": cfg.Server.MaxConcurrent,
			"server_api_key":        cfg.Server.APIKey,
			"webdav_servers":        webdavServers,
			"express":               cfg.Express,
			"torrent_enabled":       cfg.Torrent.Enabled,
			"bilibili_cookie":       cfg.Bilibili.Cookie,
		},
		Message: "config retrieved",
	})
}

func (s *Server) handleSetConfig(c *gin.Context) {
	var req ConfigSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request body: key is required",
		})
		return
	}

	// Load current config, update, save
	cfg := config.LoadOrDefault()
	if err := s.setConfigValue(cfg, req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: err.Error(),
		})
		return
	}

	if err := config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to save config: %v", err),
		})
		return
	}

	// Update server's cached config
	s.cfg = cfg

	// Special handling for output_dir
	if req.Key == "output_dir" {
		if err := os.MkdirAll(req.Value, 0755); err != nil {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: fmt.Sprintf("invalid output directory: %v", err),
			})
			return
		}
		s.outputDir = req.Value
		s.jobQueue.outputDir = req.Value
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"key":   req.Key,
			"value": req.Value,
		},
		Message: fmt.Sprintf("config %s updated", req.Key),
	})
}

func (s *Server) handleUpdateConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request body",
		})
		return
	}

	if req.OutputDir != "" {
		if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: fmt.Sprintf("invalid output directory: %v", err),
			})
			return
		}

		s.outputDir = req.OutputDir
		s.jobQueue.outputDir = req.OutputDir

		cfg := config.LoadOrDefault()
		cfg.OutputDir = req.OutputDir
		if err := config.Save(cfg); err != nil {
			log.Printf("Warning: failed to save config: %v", err)
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"output_dir": s.outputDir,
		},
		Message: "config updated",
	})
}

func (s *Server) handleI18n(c *gin.Context) {
	lang := s.cfg.Language
	if lang == "" {
		lang = "zh"
	}

	t := i18n.GetTranslations(lang)

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"language":      lang,
			"ui":            t.UI,
			"server":        t.Server,
			"config_exists": config.Exists(),
		},
		Message: "translations retrieved",
	})
}

// WebDAV handlers

// WebDAVConfigRequest is the request body for WebDAV server operations
type WebDAVConfigRequest struct {
	Name     string `json:"name" binding:"required"`
	URL      string `json:"url" binding:"required"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleGetWebDAV(c *gin.Context) {
	cfg := config.LoadOrDefault()

	servers := make(map[string]map[string]string)
	for name, server := range cfg.WebDAVServers {
		servers[name] = map[string]string{
			"url":      server.URL,
			"username": server.Username,
			"password": server.Password,
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    servers,
		Message: "webdav servers retrieved",
	})
}

func (s *Server) handleAddWebDAV(c *gin.Context) {
	var req WebDAVConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "name and url are required",
		})
		return
	}

	cfg := config.LoadOrDefault()
	cfg.SetWebDAVServer(req.Name, config.WebDAVServer{
		URL:      req.URL,
		Username: req.Username,
		Password: req.Password,
	})

	if err := config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to save config: %v", err),
		})
		return
	}

	s.cfg = cfg
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"name": req.Name},
		Message: "webdav server added",
	})
}

func (s *Server) handleDeleteWebDAV(c *gin.Context) {
	name := c.Param("name")

	cfg := config.LoadOrDefault()
	if cfg.GetWebDAVServer(name) == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "webdav server not found",
		})
		return
	}

	cfg.DeleteWebDAVServer(name)

	if err := config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to save config: %v", err),
		})
		return
	}

	s.cfg = cfg
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"name": name},
		Message: "webdav server deleted",
	})
}

// Kuaidi100 handler

// TrackRequest is the request body for POST /kuaidi100
type TrackRequest struct {
	TrackingNumber string `json:"tracking_number" binding:"required"`
	Courier        string `json:"courier" binding:"required"`
}

func (s *Server) handleKuaidi100(c *gin.Context) {
	var req TrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "tracking_number and courier are required",
		})
		return
	}

	// Load config to get kuaidi100 credentials
	cfg := config.LoadOrDefault()
	expressCfg := cfg.GetExpressConfig("kuaidi100")
	if expressCfg == nil || expressCfg["key"] == "" || expressCfg["customer"] == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "快递100凭证未配置。请在设置中配置 API Key 和 Customer ID。",
		})
		return
	}

	// Create tracker and query
	t := tracker.NewKuaidi100Tracker(expressCfg["key"], expressCfg["customer"])
	courierCode := tracker.GetCourierCode(req.Courier)

	result, err := t.Track(courierCode, req.TrackingNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("tracking failed: %v", err),
		})
		return
	}

	// Get courier info for display
	courierInfo := tracker.GetCourierInfo(req.Courier)
	courierName := courierCode
	if courierInfo != nil {
		courierName = courierInfo.Name
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"tracking_number": result.Nu,
			"courier_code":    result.Com,
			"courier_name":    courierName,
			"state":           result.State,
			"state_desc":      result.StateDescription(),
			"is_delivered":    result.IsDelivered(),
			"data":            result.Data,
		},
		Message: "tracking info retrieved",
	})
}

// Torrent handlers

// TorrentConfigRequest is the request body for POST /config/torrent
type TorrentConfigRequest struct {
	Enabled         bool   `json:"enabled"`
	Client          string `json:"client"`
	Host            string `json:"host"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	UseHTTPS        bool   `json:"use_https"`
	DefaultSavePath string `json:"default_save_path"`
}

// TorrentAddRequest is the request body for POST /torrent
type TorrentAddRequest struct {
	URL      string `json:"url" binding:"required"` // Magnet link or .torrent URL
	SavePath string `json:"save_path,omitempty"`
	Paused   bool   `json:"paused,omitempty"`
}

func (s *Server) handleGetTorrentConfig(c *gin.Context) {
	cfg := config.LoadOrDefault()

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"enabled":           cfg.Torrent.Enabled,
			"client":            cfg.Torrent.Client,
			"host":              cfg.Torrent.Host,
			"username":          cfg.Torrent.Username,
			"password":          cfg.Torrent.Password,
			"use_https":         cfg.Torrent.UseHTTPS,
			"default_save_path": cfg.Torrent.DefaultSavePath,
		},
		Message: "torrent config retrieved",
	})
}

func (s *Server) handleSetTorrentConfig(c *gin.Context) {
	var req TorrentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request body",
		})
		return
	}

	// Validate client type if enabled
	if req.Enabled {
		switch req.Client {
		case "transmission", "qbittorrent", "synology":
			// Valid
		default:
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: "invalid client type: must be transmission, qbittorrent, or synology",
			})
			return
		}

		if req.Host == "" {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: "host is required when torrent is enabled",
			})
			return
		}
	}

	cfg := config.LoadOrDefault()
	cfg.Torrent = config.TorrentConfig{
		Enabled:         req.Enabled,
		Client:          req.Client,
		Host:            req.Host,
		Username:        req.Username,
		Password:        req.Password,
		UseHTTPS:        req.UseHTTPS,
		DefaultSavePath: req.DefaultSavePath,
	}

	if err := config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to save config: %v", err),
		})
		return
	}

	s.cfg = cfg
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"enabled": req.Enabled},
		Message: "torrent config saved",
	})
}

func (s *Server) handleTestTorrentConnection(c *gin.Context) {
	cfg := config.LoadOrDefault()

	if !cfg.Torrent.Enabled {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "torrent is not enabled",
		})
		return
	}

	client, err := s.createTorrentClient(&cfg.Torrent)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: fmt.Sprintf("invalid torrent config: %v", err),
		})
		return
	}

	if err := client.Connect(); err != nil {
		c.JSON(http.StatusBadGateway, Response{
			Code:    502,
			Data:    nil,
			Message: fmt.Sprintf("connection failed: %v", err),
		})
		return
	}
	defer client.Close()

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"client": client.Name()},
		Message: "connection successful",
	})
}

func (s *Server) handleAddTorrent(c *gin.Context) {
	var req TorrentAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "url is required",
		})
		return
	}

	cfg := config.LoadOrDefault()

	if !cfg.Torrent.Enabled {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "torrent is not enabled. Configure it in settings first.",
		})
		return
	}

	client, err := s.createTorrentClient(&cfg.Torrent)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: fmt.Sprintf("invalid torrent config: %v", err),
		})
		return
	}

	if err := client.Connect(); err != nil {
		c.JSON(http.StatusBadGateway, Response{
			Code:    502,
			Data:    nil,
			Message: fmt.Sprintf("failed to connect to torrent client: %v", err),
		})
		return
	}
	defer client.Close()

	// Prepare options
	opts := &torrent.AddOptions{
		Paused: req.Paused,
	}
	if req.SavePath != "" {
		opts.SavePath = req.SavePath
	} else if cfg.Torrent.DefaultSavePath != "" {
		opts.SavePath = cfg.Torrent.DefaultSavePath
	}

	// Add torrent based on URL type
	var result *torrent.AddResult
	if torrent.IsMagnetLink(req.URL) {
		result, err = client.AddMagnet(req.URL, opts)
	} else {
		result, err = client.AddTorrentURL(req.URL, opts)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to add torrent: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"id":        result.ID,
			"hash":      result.Hash,
			"name":      result.Name,
			"duplicate": result.Duplicate,
		},
		Message: "torrent added successfully",
	})
}

func (s *Server) handleListTorrents(c *gin.Context) {
	cfg := config.LoadOrDefault()

	if !cfg.Torrent.Enabled {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "torrent is not enabled",
		})
		return
	}

	client, err := s.createTorrentClient(&cfg.Torrent)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: fmt.Sprintf("invalid torrent config: %v", err),
		})
		return
	}

	if err := client.Connect(); err != nil {
		c.JSON(http.StatusBadGateway, Response{
			Code:    502,
			Data:    nil,
			Message: fmt.Sprintf("failed to connect to torrent client: %v", err),
		})
		return
	}
	defer client.Close()

	torrents, err := client.ListTorrents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to list torrents: %v", err),
		})
		return
	}

	// Convert to JSON-friendly format
	torrentList := make([]gin.H, len(torrents))
	for i, t := range torrents {
		torrentList[i] = gin.H{
			"id":             t.ID,
			"hash":           t.Hash,
			"name":           t.Name,
			"state":          t.State.String(),
			"progress":       t.Progress,
			"size":           t.Size,
			"downloaded":     t.Downloaded,
			"uploaded":       t.Uploaded,
			"download_speed": t.DownloadSpeed,
			"upload_speed":   t.UploadSpeed,
			"ratio":          t.Ratio,
			"eta":            t.ETA,
			"save_path":      t.SavePath,
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"torrents": torrentList,
			"count":    len(torrentList),
		},
		Message: fmt.Sprintf("%d torrents found", len(torrentList)),
	})
}

// createTorrentClient creates a torrent client from config
func (s *Server) createTorrentClient(cfg *config.TorrentConfig) (torrent.Client, error) {
	clientCfg := &torrent.Config{
		Type:     torrent.ClientType(cfg.Client),
		Host:     cfg.Host,
		Username: cfg.Username,
		Password: cfg.Password,
		UseHTTPS: cfg.UseHTTPS,
	}
	return torrent.NewClient(clientCfg)
}

// Helper functions

// setConfigValue sets a config value by key
func (s *Server) setConfigValue(cfg *config.Config, key, value string) error {
	// Handle express.<provider>.<key> pattern
	if strings.HasPrefix(key, "express.") {
		parts := strings.SplitN(key, ".", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid express config key format: %s (use express.<provider>.<key>)", key)
		}
		provider := parts[1]
		configKey := parts[2]
		cfg.SetExpressConfig(provider, configKey, value)
		return nil
	}

	switch key {
	case "language":
		cfg.Language = value
	case "output_dir":
		cfg.OutputDir = value
	case "format":
		cfg.Format = value
	case "quality":
		cfg.Quality = value
	case "twitter_auth_token", "twitter.auth_token":
		cfg.Twitter.AuthToken = value
	case "server.max_concurrent", "server_max_concurrent":
		var val int
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return fmt.Errorf("invalid value for max_concurrent: %s", value)
		}
		cfg.Server.MaxConcurrent = val
	case "server.api_key", "server_api_key":
		cfg.Server.APIKey = value
	case "bilibili.cookie", "bilibili_cookie":
		cfg.Bilibili.Cookie = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// downloadWebDAV handles WebDAV URL downloads using multi-stream for better performance
func (s *Server) downloadWebDAV(ctx context.Context, rawURL, filename string, progressFn func(downloaded, total int64)) error {
	var client *webdav.Client
	var filePath string
	var err error

	// Check if it's a remote path (e.g., "pikpak:/path/to/file")
	if webdav.IsRemotePath(rawURL) {
		serverName, filePath, err := webdav.ParseRemotePath(rawURL)
		if err != nil {
			return err
		}

		server := s.cfg.GetWebDAVServer(serverName)
		if server == nil {
			return fmt.Errorf("WebDAV server '%s' not found", serverName)
		}

		client, err = webdav.NewClientFromConfig(server)
		if err != nil {
			return fmt.Errorf("failed to create WebDAV client: %w", err)
		}

		// Get file info
		fileInfo, err := client.Stat(ctx, filePath)
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		if fileInfo.IsDir {
			return fmt.Errorf("cannot download directory: %s", filePath)
		}

		// Determine output filename
		outputFile := filename
		if outputFile == "" {
			outputFile = webdav.ExtractFilename(filePath)
		}
		outputPath := filepath.Join(s.outputDir, outputFile)

		// Update job filename
		s.updateJobFilename(rawURL, outputPath)

		// Download using multi-stream for better performance (same as CLI)
		fileURL := client.GetFileURL(filePath)
		authHeader := client.GetAuthHeader()

		return downloadWebDAVMultiStream(ctx, fileURL, authHeader, outputPath, fileInfo.Size, progressFn)
	}

	// Handle full WebDAV URL
	client, err = webdav.NewClient(rawURL)
	if err != nil {
		return fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	filePath, err = webdav.ParseURL(rawURL)
	if err != nil {
		return fmt.Errorf("invalid WebDAV URL: %w", err)
	}

	fileInfo, err := client.Stat(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.IsDir {
		return fmt.Errorf("cannot download directory: %s", filePath)
	}

	outputFile := filename
	if outputFile == "" {
		outputFile = webdav.ExtractFilename(filePath)
	}
	outputPath := filepath.Join(s.outputDir, outputFile)

	s.updateJobFilename(rawURL, outputPath)

	fileURL := client.GetFileURL(filePath)
	authHeader := client.GetAuthHeader()

	return downloadWebDAVMultiStream(ctx, fileURL, authHeader, outputPath, fileInfo.Size, progressFn)
}

// downloadWebDAVMultiStream uses multi-stream download for better performance
func downloadWebDAVMultiStream(ctx context.Context, url, authHeader, outputPath string, totalSize int64, progressFn func(downloaded, total int64)) error {
	msConfig := downloader.DefaultMultiStreamConfig()
	return downloader.RunMultiStreamDownloadWithAuthCallback(ctx, url, authHeader, outputPath, totalSize, msConfig, progressFn)
}

// downloadWithExtractor is the download function used by the job queue
func (s *Server) downloadWithExtractor(ctx context.Context, url, filename string, progressFn func(downloaded, total int64)) error {
	// Handle WebDAV URLs specially
	if webdav.IsWebDAVURL(url) {
		return s.downloadWebDAV(ctx, url, filename, progressFn)
	}

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
	case *extractor.YouTubeDirectDownload:
		return extractor.DownloadWithYtdlpProgress(ctx, m.URL, s.outputDir, progressFn)

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

		title := extractor.SanitizeFilename(m.Title)
		var filenames []string

		for i, img := range m.Images {
			var imgPath string
			if len(m.Images) == 1 {
				if title != "" {
					imgPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", title, img.Ext))
				} else {
					imgPath = filepath.Join(s.outputDir, fmt.Sprintf("%s.%s", m.ID, img.Ext))
				}
			} else {
				if title != "" {
					imgPath = filepath.Join(s.outputDir, fmt.Sprintf("%s_%d.%s", title, i+1, img.Ext))
				} else {
					imgPath = filepath.Join(s.outputDir, fmt.Sprintf("%s_%d.%s", m.ID, i+1, img.Ext))
				}
			}

			filenames = append(filenames, imgPath)

			if err := downloadFile(ctx, img.URL, imgPath, nil, nil); err != nil {
				return fmt.Errorf("failed to download image %d: %w", i+1, err)
			}
		}

		s.updateJobFilename(url, strings.Join(filenames, ", "))
		return nil

	default:
		return fmt.Errorf("unsupported media type")
	}

	// Check if this is an HLS stream
	if strings.HasSuffix(strings.ToLower(downloadURL), ".m3u8") ||
		strings.Contains(strings.ToLower(downloadURL), ".m3u8?") {
		finalPath, err := downloader.DownloadHLSWithProgress(ctx, downloadURL, outputPath, headers, progressFn)
		if err != nil {
			return err
		}
		if finalPath != outputPath {
			s.updateJobFilename(url, finalPath)
		}
		return nil
	}

	return downloadFile(ctx, downloadURL, outputPath, headers, progressFn)
}

func (s *Server) updateJobFilename(url, filename string) {
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
func (s *Server) downloadAndStream(c *gin.Context, url, filename string) {
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

	if twitterExt, ok := ext.(*extractor.TwitterExtractor); ok {
		if s.cfg.Twitter.AuthToken != "" {
			twitterExt.SetAuth(s.cfg.Twitter.AuthToken)
		}
	}

	media, err := ext.Extract(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
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
	case *extractor.YouTubeDirectDownload:
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "YouTube streaming not supported. Use queued download instead.",
		})
		return

	case *extractor.VideoMedia:
		if len(m.Formats) == 0 {
			c.JSON(http.StatusInternalServerError, Response{
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
			c.JSON(http.StatusInternalServerError, Response{
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
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: "unsupported media type",
		})
		return
	}

	streamFile(c.Writer, downloadURL, outputFilename, headers)
}

func selectBestFormat(formats []extractor.VideoFormat) *extractor.VideoFormat {
	if len(formats) == 0 {
		return nil
	}

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

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 32*1024)
	var downloaded int64

	for {
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

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if resp.ContentLength > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	io.Copy(w, resp.Body)
}
