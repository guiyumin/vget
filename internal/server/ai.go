package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guiyumin/vget/internal/core/ai"
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

// AITranscribeRequest is the request body for transcribing audio
type AITranscribeRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	Account  string `json:"account"` // Account label
	Model    string `json:"model"`   // Model to use
	PIN      string `json:"pin"`     // Optional if account uses plain text keys
}

// AISummarizeRequest is the request body for summarizing text
type AISummarizeRequest struct {
	FilePath string `json:"file_path"` // Path to transcript file
	Text     string `json:"text"`      // Or direct text input
	Account  string `json:"account"`   // Account label
	Model    string `json:"model"`     // Model to use
	PIN      string `json:"pin"`       // Optional if account uses plain text keys
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

// handleTranscribe handles audio transcription
func (s *Server) handleTranscribe(c *gin.Context) {
	var req AITranscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "file_path is required",
		})
		return
	}

	cfg := config.LoadOrDefault()

	// Get account
	account := cfg.AI.GetAccount(req.Account)
	if account == nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "no AI account configured. Add an account in Settings first.",
		})
		return
	}

	// Check if file exists
	filePath := req.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.outputDir, filePath)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: fmt.Sprintf("file not found: %s", req.FilePath),
		})
		return
	}

	// Create AI pipeline
	pipeline, err := ai.NewPipelineWithAccount(account, req.Model, req.PIN)
	if err != nil {
		if strings.Contains(err.Error(), "PIN") || strings.Contains(err.Error(), "decrypt") {
			c.JSON(http.StatusUnauthorized, Response{
				Code:    401,
				Data:    nil,
				Message: "incorrect PIN or corrupted key",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to create AI pipeline: %v", err),
		})
		return
	}

	// Transcribe
	result, err := pipeline.Process(c.Request.Context(), filePath, ai.Options{
		Transcribe: true,
		Summarize:  false,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("transcription failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"text":        result.Transcript.Text,
			"output_path": result.TranscriptPath,
			"duration":    result.Transcript.Duration,
			"language":    result.Transcript.Language,
		},
		Message: "transcription completed",
	})
}

// handleSummarize handles text summarization
func (s *Server) handleSummarize(c *gin.Context) {
	var req AISummarizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "either file_path or text must be provided",
		})
		return
	}

	// Determine input file path
	var filePath string

	if req.FilePath != "" {
		filePath = req.FilePath
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(s.outputDir, filePath)
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Data:    nil,
				Message: fmt.Sprintf("file not found: %s", req.FilePath),
			})
			return
		}
	} else if req.Text != "" {
		// Create a temporary file with the text content
		tmpFile, err := os.CreateTemp("", "vget-summarize-*.txt")
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: "failed to create temporary file",
			})
			return
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(req.Text); err != nil {
			tmpFile.Close()
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: "failed to write temporary file",
			})
			return
		}
		tmpFile.Close()
		filePath = tmpFile.Name()
	} else {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "either file_path or text must be provided",
		})
		return
	}

	cfg := config.LoadOrDefault()

	// Get account
	account := cfg.AI.GetAccount(req.Account)
	if account == nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "no AI account configured. Add an account in Settings first.",
		})
		return
	}

	// Create AI pipeline
	pipeline, err := ai.NewPipelineWithAccount(account, req.Model, req.PIN)
	if err != nil {
		if strings.Contains(err.Error(), "PIN") || strings.Contains(err.Error(), "decrypt") {
			c.JSON(http.StatusUnauthorized, Response{
				Code:    401,
				Data:    nil,
				Message: "incorrect PIN or corrupted key",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to create AI pipeline: %v", err),
		})
		return
	}

	// Summarize
	result, err := pipeline.Process(c.Request.Context(), filePath, ai.Options{
		Transcribe: false,
		Summarize:  true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("summarization failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"summary":     result.Summary.Summary,
			"key_points":  result.Summary.KeyPoints,
			"output_path": result.SummaryPath,
		},
		Message: "summarization completed",
	})
}

// handleListDownloadedAudio lists audio and video files in the output directory
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

	err := filepath.Walk(s.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if mediaExtensions[ext] {
			relPath, _ := filepath.Rel(s.outputDir, path)

			// Check for existing transcript/summary
			basePath := strings.TrimSuffix(path, ext)
			hasTranscript := fileExists(basePath + ".transcript.md")
			hasSummary := fileExists(basePath + ".summary.md")

			audioFiles = append(audioFiles, gin.H{
				"name":           info.Name(),
				"path":           relPath,
				"full_path":      path,
				"size":           info.Size(),
				"mod_time":       info.ModTime(),
				"has_transcript": hasTranscript,
				"has_summary":    hasSummary,
			})
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to list files: %v", err),
		})
		return
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

	// Create uploads directory in output dir
	uploadDir := filepath.Join(s.outputDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: "failed to create upload directory",
		})
		return
	}

	// Save file
	destPath := filepath.Join(uploadDir, file.Filename)
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
