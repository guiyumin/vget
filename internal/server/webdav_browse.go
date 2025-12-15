package server

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/webdav"
)

// WebDAVRemoteInfo is the response for a single remote
type WebDAVRemoteInfo struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	HasAuth bool   `json:"hasAuth"`
}

// WebDAVFileInfo is the response for a single file/directory
type WebDAVFileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
}

// WebDAVDownloadRequest is the request body for POST /api/webdav/download
type WebDAVDownloadRequest struct {
	Remote string   `json:"remote" binding:"required"`
	Files  []string `json:"files" binding:"required"`
}

// GET /api/webdav/remotes - List all configured WebDAV servers
func (s *Server) handleWebDAVRemotes(c *gin.Context) {
	cfg := config.LoadOrDefault()

	remotes := make([]WebDAVRemoteInfo, 0, len(cfg.WebDAVServers))
	for name, server := range cfg.WebDAVServers {
		remotes = append(remotes, WebDAVRemoteInfo{
			Name:    name,
			URL:     server.URL,
			HasAuth: server.Username != "",
		})
	}

	// Sort by name for consistent ordering
	sort.Slice(remotes, func(i, j int) bool {
		return remotes[i].Name < remotes[j].Name
	})

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"remotes": remotes},
		Message: "success",
	})
}

// GET /api/webdav/list?remote=xxx&path=/xxx - List directory contents
func (s *Server) handleWebDAVList(c *gin.Context) {
	remoteName := c.Query("remote")
	if remoteName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "remote parameter is required",
		})
		return
	}

	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	cfg := config.LoadOrDefault()
	server := cfg.GetWebDAVServer(remoteName)
	if server == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "remote not found: " + remoteName,
		})
		return
	}

	client, err := webdav.NewClientFromConfig(server)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: "failed to connect to WebDAV server: " + err.Error(),
		})
		return
	}

	files, err := client.List(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: "failed to list directory: " + err.Error(),
		})
		return
	}

	// Convert to response format
	result := make([]WebDAVFileInfo, len(files))
	for i, f := range files {
		result[i] = WebDAVFileInfo{
			Name:  f.Name,
			Path:  f.Path,
			Size:  f.Size,
			IsDir: f.IsDir,
		}
	}

	// Sort: directories first, then by name
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"remote": remoteName,
			"path":   path,
			"files":  result,
		},
		Message: "success",
	})
}

// POST /api/webdav/download - Queue download(s) from WebDAV
func (s *Server) handleWebDAVDownload(c *gin.Context) {
	var req WebDAVDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if len(req.Files) == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "no files specified",
		})
		return
	}

	cfg := config.LoadOrDefault()
	server := cfg.GetWebDAVServer(req.Remote)
	if server == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "remote not found: " + req.Remote,
		})
		return
	}

	// Queue downloads for each file
	jobIDs := make([]string, 0, len(req.Files))
	for _, filePath := range req.Files {
		// Build the remote URL in the format the downloader expects
		url := req.Remote + ":" + filePath

		job, err := s.jobQueue.AddJob(url, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: "failed to queue download: " + err.Error(),
			})
			return
		}
		jobIDs = append(jobIDs, job.ID)
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"jobIds": jobIDs,
			"count":  len(jobIDs),
		},
		Message: "downloads queued",
	})
}
