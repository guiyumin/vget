package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guiyumin/vget/internal/core/ai"
	"github.com/guiyumin/vget/internal/core/ai/transcriber"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/crypto"
)

// AIAccountRequest is the request body for creating an AI account
type AIAccountRequest struct {
	Label    string `json:"label" binding:"required"`
	Provider string `json:"provider" binding:"required"`
	APIKey   string `json:"api_key" binding:"required"`
	PIN      string `json:"pin"` // Optional - if empty, key stored as plain text
}

// handleGetAIModels returns available AI models for each provider
func (s *Server) handleGetAIModels(c *gin.Context) {
	// Build OpenAI models list from ai package
	openaiModels := make([]gin.H, len(ai.OpenAIModels))
	for i, m := range ai.OpenAIModels {
		openaiModels[i] = gin.H{
			"id":          m.ID,
			"name":        m.Name,
			"description": m.Description,
			"tier":        m.Tier,
		}
	}

	// Anthropic models (for summarization)
	anthropicModels := []gin.H{
		{"id": "claude-sonnet-4-20250514", "name": "Claude Sonnet 4", "description": "Latest balanced model", "tier": "standard"},
		{"id": "claude-3-5-sonnet-20241022", "name": "Claude 3.5 Sonnet", "description": "Fast and capable", "tier": "standard"},
		{"id": "claude-3-5-haiku-20241022", "name": "Claude 3.5 Haiku", "description": "Fastest model", "tier": "fast"},
		{"id": "claude-3-opus-20240229", "name": "Claude 3 Opus", "description": "Most capable", "tier": "flagship"},
	}

	// Qwen models (for summarization)
	qwenModels := []gin.H{
		{"id": "qwen-max", "name": "Qwen Max", "description": "Most capable Qwen model", "tier": "flagship"},
		{"id": "qwen-plus", "name": "Qwen Plus", "description": "Balanced performance", "tier": "standard"},
		{"id": "qwen-turbo", "name": "Qwen Turbo", "description": "Fast responses", "tier": "fast"},
	}

	// Transcription models by provider
	transcriptionModels := gin.H{
		"openai":    []string{"whisper-1"},
		"anthropic": []string{"whisper-1"},
		"qwen":      []string{"paraformer-v2", "whisper-large-v3"},
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"summarization": gin.H{
				"openai":    openaiModels,
				"anthropic": anthropicModels,
				"qwen":      qwenModels,
				"default":   ai.DefaultOpenAIModel,
			},
			"transcription": transcriptionModels,
		},
		Message: "AI models retrieved",
	})
}

// handleGetAIConfig returns AI configuration
func (s *Server) handleGetAIConfig(c *gin.Context) {
	cfg := config.LoadOrDefault()

	// Build accounts list for response
	accounts := make([]gin.H, len(cfg.AI.Accounts))
	var defaultAccount string

	for i, acc := range cfg.AI.Accounts {
		isEncrypted := acc.APIKey != "" && !strings.HasPrefix(acc.APIKey, "plain:")

		accounts[i] = gin.H{
			"label":        acc.Label,
			"provider":     acc.Provider,
			"is_encrypted": isEncrypted,
			"is_default":   acc.IsDefault,
		}

		if acc.IsDefault {
			defaultAccount = acc.Label
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"accounts":        accounts,
			"default_account": defaultAccount,
		},
		Message: "AI config retrieved",
	})
}

// handleAddAIAccount creates a new AI account
func (s *Server) handleAddAIAccount(c *gin.Context) {
	var req AIAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "label, provider, and api_key are required",
		})
		return
	}

	// Validate PIN format if provided
	if req.PIN != "" {
		if err := crypto.ValidatePIN(req.PIN); err != nil {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: "PIN must be exactly 4 digits",
			})
			return
		}
	}

	// Trim whitespace from label
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "label cannot be empty",
		})
		return
	}

	// Validate provider
	switch req.Provider {
	case "openai", "anthropic", "qwen":
		// Valid
	default:
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid provider: must be openai, anthropic, or qwen",
		})
		return
	}

	// Encrypt or store plain text
	var apiKey string
	if req.PIN != "" {
		var err error
		apiKey, err = crypto.Encrypt(req.APIKey, req.PIN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: fmt.Sprintf("failed to encrypt API key: %v", err),
			})
			return
		}
	} else {
		apiKey = "plain:" + req.APIKey
	}

	cfg := config.LoadOrDefault()

	// Check if label already exists
	for _, acc := range cfg.AI.Accounts {
		if acc.Label == req.Label {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: fmt.Sprintf("account '%s' already exists", req.Label),
			})
			return
		}
	}

	// Create account
	account := config.AIAccount{
		Label:     req.Label,
		Provider:  req.Provider,
		APIKey:    apiKey,
		IsDefault: len(cfg.AI.Accounts) == 0, // First account is default
	}

	cfg.AI.AddAccount(account)

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
		Data:    gin.H{"label": req.Label},
		Message: "AI account created",
	})
}

// handleDeleteAIAccount deletes an AI account
func (s *Server) handleDeleteAIAccount(c *gin.Context) {
	label := c.Param("name")

	cfg := config.LoadOrDefault()

	if cfg.AI.GetAccount(label) == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "AI account not found",
		})
		return
	}

	cfg.AI.DeleteAccount(label)

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
		Data:    gin.H{"label": label},
		Message: "AI account deleted",
	})
}

// handleSetDefaultAIAccount sets the default AI account
func (s *Server) handleSetDefaultAIAccount(c *gin.Context) {
	var req struct {
		Label string `json:"label" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "label is required",
		})
		return
	}

	cfg := config.LoadOrDefault()

	if !cfg.AI.SetDefault(req.Label) {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "AI account not found",
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

	s.cfg = cfg
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"default_account": req.Label},
		Message: "default AI account updated",
	})
}

// handleListDownloadedAudio lists audio and video files in the output directory root only
func (s *Server) handleListDownloadedAudio(c *gin.Context) {
	var audioFiles []gin.H

	mediaExtensions := map[string]bool{
		// Audio
		".mp3": true, ".m4a": true, ".wav": true, ".aac": true,
		".ogg": true, ".flac": true, ".opus": true, ".wma": true,
		// Video
		".mp4": true, ".webm": true, ".mkv": true, ".avi": true,
		".mov": true, ".flv": true, ".wmv": true,
	}

	// Only list files in root directory (not subdirectories)
	entries, err := os.ReadDir(s.outputDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to list files: %v", err),
		})
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !mediaExtensions[ext] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(s.outputDir, name)

		// Check for existing transcript/summary
		basePath := strings.TrimSuffix(fullPath, ext)
		hasTranscript := fileExists(basePath + ".transcript.md")
		hasSummary := fileExists(basePath + ".summary.md")

		audioFiles = append(audioFiles, gin.H{
			"name":           name,
			"path":           name,
			"full_path":      fullPath,
			"size":           info.Size(),
			"mod_time":       info.ModTime(),
			"has_transcript": hasTranscript,
			"has_summary":    hasSummary,
		})
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"files":      audioFiles,
			"output_dir": s.outputDir,
		},
		Message: fmt.Sprintf("%d media files found", len(audioFiles)),
	})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// handleStartAIProcess starts a new AI processing job
func (s *Server) handleStartAIProcess(c *gin.Context) {
	// AI processing is only available in Docker
	if !config.IsRunningInDocker() {
		c.JSON(http.StatusForbidden, Response{
			Code:    403,
			Data:    nil,
			Message: "AI processing is only available in Docker mode",
		})
		return
	}

	var req AIJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "file_path is required",
		})
		return
	}

	// Validate file path
	filePath := req.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.outputDir, filePath)
	}
	req.FilePath = filePath

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: fmt.Sprintf("file not found: %s", req.FilePath),
		})
		return
	}

	// Validate account exists
	cfg := config.LoadOrDefault()
	account := cfg.AI.GetAccount(req.Account)
	if account == nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "no AI account configured. Add an account in Settings first.",
		})
		return
	}

	// Validate PIN if account is encrypted
	isEncrypted := account.APIKey != "" && !strings.HasPrefix(account.APIKey, "plain:")
	if isEncrypted && req.PIN == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "PIN required for encrypted account",
		})
		return
	}

	// Add job to queue
	job, err := s.aiJobQueue.AddJob(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to queue job: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"job_id": job.ID,
			"status": job.Status,
		},
		Message: "AI processing started",
	})
}

// handleGetAIJob returns the status of an AI job
func (s *Server) handleGetAIJob(c *gin.Context) {
	id := c.Param("id")

	job := s.aiJobQueue.GetJob(id)
	if job == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "AI job not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    job,
		Message: string(job.Status),
	})
}

// handleGetAIJobs returns all AI jobs
func (s *Server) handleGetAIJobs(c *gin.Context) {
	jobs := s.aiJobQueue.GetAllJobs()

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"jobs": jobs,
		},
		Message: fmt.Sprintf("%d AI jobs", len(jobs)),
	})
}

// handleCancelAIJob cancels an AI job
func (s *Server) handleCancelAIJob(c *gin.Context) {
	id := c.Param("id")

	if s.aiJobQueue.CancelJob(id) {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Data:    gin.H{"id": id},
			Message: "AI job cancelled",
		})
	} else {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "AI job not found or cannot be cancelled",
		})
	}
}

// handleClearAIJobs clears AI job history
func (s *Server) handleClearAIJobs(c *gin.Context) {
	count := s.aiJobQueue.ClearHistory()

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"cleared": count},
		Message: fmt.Sprintf("Cleared %d AI jobs", count),
	})
}

// handleUploadAudio handles audio file uploads for transcription
func (s *Server) handleUploadAudio(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "file is required",
		})
		return
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	mediaExtensions := map[string]bool{
		// Audio
		".mp3": true, ".m4a": true, ".wav": true, ".aac": true,
		".ogg": true, ".flac": true, ".opus": true, ".wma": true,
		// Video
		".mp4": true, ".webm": true, ".mkv": true, ".avi": true,
		".mov": true, ".flv": true, ".wmv": true,
	}
	if !mediaExtensions[ext] {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid file type. Supported: mp3, m4a, wav, aac, ogg, flac, opus, wma, mp4, webm, mkv, avi, mov, flv, wmv",
		})
		return
	}

	// Save file directly to output directory
	destPath := filepath.Join(s.outputDir, file.Filename)
	if err := c.SaveUploadedFile(file, destPath); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: "failed to save file",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"path":     destPath,
			"filename": file.Filename,
			"size":     file.Size,
		},
		Message: "file uploaded",
	})
}

// handleGetLocalASRCapabilities returns the capabilities of local whisper.cpp transcription
func (s *Server) handleGetLocalASRCapabilities(c *gin.Context) {
	cfg := config.LoadOrDefault()

	// Get models directory
	modelsDir := cfg.AI.LocalASR.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = transcriber.DefaultModelsDir()
		if err != nil {
			c.JSON(http.StatusOK, Response{
				Code: 200,
				Data: gin.H{
					"available": false,
					"enabled":   cfg.AI.LocalASR.Enabled,
					"error":     err.Error(),
				},
				Message: "Failed to get models directory",
			})
			return
		}
	}

	// Create model manager and list available models
	manager := transcriber.NewModelManager(modelsDir)
	models := manager.ListAvailableModels()

	// Check if any model is downloaded
	hasDownloadedModel := false
	for _, m := range models {
		if m.Downloaded {
			hasDownloadedModel = true
			break
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"available":       hasDownloadedModel,
			"enabled":         cfg.AI.LocalASR.Enabled,
			"current_model":   cfg.AI.LocalASR.Model,
			"language":        cfg.AI.LocalASR.Language,
			"models_dir":      modelsDir,
			"models":          models,
			"default_model":   transcriber.DefaultWhisperModel,
		},
		Message: "Local ASR capabilities retrieved",
	})
}

// LocalASRConfigRequest is the request body for updating local ASR settings
type LocalASRConfigRequest struct {
	Enabled  *bool  `json:"enabled"`
	Model    string `json:"model"`
	Language string `json:"language"`
}

// handleUpdateLocalASRConfig updates local ASR settings
func (s *Server) handleUpdateLocalASRConfig(c *gin.Context) {
	var req LocalASRConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid request body",
		})
		return
	}

	cfg := config.LoadOrDefault()

	// Update fields if provided
	if req.Enabled != nil {
		cfg.AI.LocalASR.Enabled = *req.Enabled
	}
	if req.Model != "" {
		cfg.AI.LocalASR.Model = req.Model
	}
	if req.Language != "" {
		cfg.AI.LocalASR.Language = req.Language
	}

	// Save config
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
		Code: 200,
		Data: gin.H{
			"enabled":  cfg.AI.LocalASR.Enabled,
			"model":    cfg.AI.LocalASR.Model,
			"language": cfg.AI.LocalASR.Language,
		},
		Message: "Local ASR config updated",
	})
}
