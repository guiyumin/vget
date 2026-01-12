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
	"github.com/guiyumin/vget/internal/core/auth"
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
	// Convert model slices to JSON format
	toJSON := func(models []ai.Model) []gin.H {
		result := make([]gin.H, len(models))
		for i, m := range models {
			result[i] = gin.H{
				"id":          m.ID,
				"name":        m.Name,
				"description": m.Description,
				"tier":        m.Tier,
			}
		}
		return result
	}

	// Transcription models by provider
	// Note: Only OpenAI supports cloud transcription (whisper-1)
	// Anthropic and Qwen are summarization-only providers
	transcriptionModels := gin.H{
		"openai": []string{"whisper-1"},
	}

	// Chinese providers use empty models list - user inputs model name manually
	emptyModels := []gin.H{}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"summarization": gin.H{
				"openai":    toJSON(ai.OpenAIModels),
				"anthropic": toJSON(ai.AnthropicModels),
				"qwen":       emptyModels,
				"deepseek":   emptyModels,
				"moonshot":   emptyModels,
				"zhipu":      emptyModels,
				"minimax":    emptyModels,
				"baichuan":   emptyModels,
				"volcengine": emptyModels,
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
	case "openai", "anthropic", "qwen", "deepseek", "moonshot", "zhipu", "minimax", "baichuan", "volcengine":
		// Valid
	default:
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid provider: supported providers are openai, anthropic, qwen, deepseek, moonshot, zhipu, minimax, baichuan, volcengine",
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
	// Check if local AI is supported in this build
	if !IsLocalAISupported() {
		c.JSON(http.StatusOK, Response{
			Code: 200,
			Data: gin.H{
				"available": false,
				"enabled":   false,
				"supported": false,
				"models":    []any{},
			},
			Message: "Local AI not supported in this build",
		})
		return
	}

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

// handleGetVmirrorModels returns available models for download from vmirror CDN
func (s *Server) handleGetVmirrorModels(c *gin.Context) {
	cfg := config.LoadOrDefault()

	// Get models directory
	modelsDir := cfg.AI.LocalASR.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = transcriber.DefaultModelsDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: fmt.Sprintf("failed to get models directory: %v", err),
			})
			return
		}
	}

	manager := transcriber.NewModelManager(modelsDir)

	// Build list of vmirror models with download status
	var models []gin.H
	for _, name := range transcriber.ListVmirrorModels() {
		model := transcriber.GetModel(name)
		if model == nil {
			continue
		}
		models = append(models, gin.H{
			"name":        model.Name,
			"size":        model.Size,
			"description": model.Description,
			"languages":   model.Languages,
			"downloaded":  manager.IsModelDownloaded(model.Name),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"models":     models,
			"models_dir": modelsDir,
		},
		Message: fmt.Sprintf("%d vmirror models available", len(models)),
	})
}

// handleGetModelDownloadAuth returns the cached auth email if registered
func (s *Server) handleGetModelDownloadAuth(c *gin.Context) {
	authData := auth.LoadAuth()
	if authData == nil {
		c.JSON(http.StatusOK, Response{
			Code: 200,
			Data: gin.H{
				"registered": false,
			},
			Message: "Not registered",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"registered": true,
			"email":      authData.Email,
		},
		Message: "Registered",
	})
}

// ModelDownloadAuthRequest is the request body for saving auth
type ModelDownloadAuthRequest struct {
	Email string `json:"email" binding:"required"`
}

// handleSaveModelDownloadAuth saves the email for model downloads
func (s *Server) handleSaveModelDownloadAuth(c *gin.Context) {
	var req ModelDownloadAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "email is required",
		})
		return
	}

	// Validate email
	if !auth.IsValidEmail(req.Email) {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid email format",
		})
		return
	}

	// Save auth
	authData := &auth.AuthData{
		Email:       req.Email,
		Fingerprint: auth.GetDeviceFingerprint(),
	}
	if err := auth.SaveAuth(authData); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to save auth: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"email": req.Email,
		},
		Message: "Auth saved",
	})
}

// ModelDownloadRequest is the request body for downloading a model
type ModelDownloadRequest struct {
	Model  string `json:"model" binding:"required"`
	Source string `json:"source"` // "huggingface" or "vmirror", defaults to "huggingface"
	Email  string `json:"email"`  // required only for vmirror
}

// handleRequestModelDownload downloads a model from Huggingface or vmirror CDN
func (s *Server) handleRequestModelDownload(c *gin.Context) {
	var req ModelDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "model is required",
		})
		return
	}

	// Default to huggingface if source not specified
	if req.Source == "" {
		req.Source = "huggingface"
	}

	// Validate source
	if req.Source != "huggingface" && req.Source != "vmirror" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "source must be 'huggingface' or 'vmirror'",
		})
		return
	}

	// Get model info
	model := transcriber.GetModel(req.Model)
	if model == nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: fmt.Sprintf("unknown model: %s", req.Model),
		})
		return
	}

	var downloadURL string

	if req.Source == "vmirror" {
		// Validate email for vmirror
		if req.Email == "" {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: "email is required for vmirror downloads",
			})
			return
		}
		if !auth.IsValidEmail(req.Email) {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: "invalid email format",
			})
			return
		}

		// Check if model exists on vmirror
		if !transcriber.IsAvailableOnVmirror(req.Model) {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: fmt.Sprintf("model '%s' is not available on vmirror", req.Model),
			})
			return
		}

		// Get vmirror filename for the model
		filename := transcriber.GetVmirrorFilename(req.Model)
		if filename == "" {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: "failed to get vmirror filename",
			})
			return
		}

		// Save auth for future use
		authData := &auth.AuthData{
			Email:       req.Email,
			Fingerprint: auth.GetDeviceFingerprint(),
		}
		if err := auth.SaveAuth(authData); err != nil {
			// Non-fatal, just log
			fmt.Printf("Warning: failed to cache auth: %v\n", err)
		}

		// Request signed URL from auth server
		signedResp, err := auth.RequestSignedURL(req.Email, filename)
		if err != nil {
			errCode := ""
			errMsg := err.Error()
			if err == auth.ErrAuthServerDown {
				errCode = "AUTH_SERVER_DOWN"
				errMsg = "Cannot connect to auth server. Please check your network."
			} else if err == auth.ErrRateLimitExceeded {
				errCode = "RATE_LIMIT"
				errMsg = "Too many downloads. Please try again in a few hours."
			}
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    gin.H{"error_code": errCode},
				Message: errMsg,
			})
			return
		}
		downloadURL = signedResp.URL
	} else {
		// Huggingface: use official URL directly
		downloadURL = model.OfficialURL
	}

	// Get models directory
	cfg := config.LoadOrDefault()
	modelsDir := cfg.AI.LocalASR.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = transcriber.DefaultModelsDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: fmt.Sprintf("failed to get models directory: %v", err),
			})
			return
		}
	}

	// Download the model
	manager := transcriber.NewModelManager(modelsDir)
	_, downloadErr := manager.DownloadModelWithProgress(req.Model, downloadURL, "en")
	if downloadErr != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to download model: %v", downloadErr),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"model":  req.Model,
			"source": req.Source,
			"size":   model.Size,
		},
		Message: "Model downloaded successfully",
	})
}
