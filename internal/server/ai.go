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

// AI Configuration request types

// AIAccountRequest is the request body for creating/updating an AI account
// Simplified: just name + provider + API key + optional PIN
type AIAccountRequest struct {
	Name     string `json:"name" binding:"required"`
	Provider string `json:"provider" binding:"required"`
	APIKey   string `json:"api_key" binding:"required"`
	PIN      string `json:"pin"` // Optional - if empty, key stored as plain text
}

// AITranscribeRequest is the request body for transcribing audio
type AITranscribeRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	Account  string `json:"account"`
	Model    string `json:"model"` // Model to use for transcription (e.g., "whisper-1")
	PIN      string `json:"pin"`   // Optional if account uses plain text keys
}

// AISummarizeRequest is the request body for summarizing text
type AISummarizeRequest struct {
	FilePath string `json:"file_path"` // Path to transcript file
	Text     string `json:"text"`      // Or direct text input
	Account  string `json:"account"`
	Model    string `json:"model"` // Model to use for summarization (e.g., "gpt-4o", "claude-3-5-sonnet")
	PIN      string `json:"pin"`   // Optional if account uses plain text keys
}

// handleGetAIConfig returns AI configuration
func (s *Server) handleGetAIConfig(c *gin.Context) {
	cfg := config.LoadOrDefault()

	accounts := make(map[string]gin.H)
	for name, account := range cfg.AI.Accounts {
		// Check if the key is encrypted or plain text
		isEncrypted := account.Transcription.APIKeyEncrypted != "" &&
			!strings.HasPrefix(account.Transcription.APIKeyEncrypted, "plain:")

		accounts[name] = gin.H{
			"provider":     account.Provider,
			"is_encrypted": isEncrypted,
		}
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"accounts":        accounts,
			"default_account": cfg.AI.DefaultAccount,
		},
		Message: "AI config retrieved",
	})
}

// handleAddAIAccount creates or updates an AI account
func (s *Server) handleAddAIAccount(c *gin.Context) {
	var req AIAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "name, provider, and api_key are required",
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
	var apiKeyEnc string
	if req.PIN != "" {
		// Encrypt with PIN
		var err error
		apiKeyEnc, err = crypto.Encrypt(req.APIKey, req.PIN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: fmt.Sprintf("failed to encrypt API key: %v", err),
			})
			return
		}
	} else {
		// Store as plain text (prefix with "plain:" to distinguish)
		apiKeyEnc = "plain:" + req.APIKey
	}

	cfg := config.LoadOrDefault()

	// Create account - store same key for both transcription and summarization
	// Models are selected at runtime, not stored in config
	account := config.AIAccount{
		Provider: req.Provider,
		Transcription: config.AIServiceConfig{
			APIKeyEncrypted: apiKeyEnc,
		},
		Summarization: config.AIServiceConfig{
			APIKeyEncrypted: apiKeyEnc,
		},
	}

	cfg.AI.SetAccount(req.Name, account)

	// Set as default if it's the first account
	if cfg.AI.DefaultAccount == "" {
		cfg.AI.DefaultAccount = req.Name
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
		Data:    gin.H{"name": req.Name},
		Message: "AI account created",
	})
}

// handleDeleteAIAccount deletes an AI account
func (s *Server) handleDeleteAIAccount(c *gin.Context) {
	name := c.Param("name")

	cfg := config.LoadOrDefault()

	if cfg.AI.GetAccount(name) == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "AI account not found",
		})
		return
	}

	cfg.AI.DeleteAccount(name)

	// Clear default if it was this account
	if cfg.AI.DefaultAccount == name {
		accounts := cfg.AI.ListAccounts()
		if len(accounts) > 0 {
			cfg.AI.DefaultAccount = accounts[0]
		} else {
			cfg.AI.DefaultAccount = ""
		}
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
		Data:    gin.H{"name": name},
		Message: "AI account deleted",
	})
}

// handleSetDefaultAIAccount sets the default AI account
func (s *Server) handleSetDefaultAIAccount(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "name is required",
		})
		return
	}

	cfg := config.LoadOrDefault()

	if cfg.AI.GetAccount(req.Name) == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Data:    nil,
			Message: "AI account not found",
		})
		return
	}

	cfg.AI.DefaultAccount = req.Name

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
		Data:    gin.H{"default_account": req.Name},
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

	// Determine which account to use
	accountName := req.Account
	if accountName == "" {
		accountName = cfg.AI.DefaultAccount
	}
	if accountName == "" {
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

	// Set model in config temporarily if provided
	account := cfg.AI.GetAccount(accountName)
	if account != nil && req.Model != "" {
		account.Transcription.Model = req.Model
		cfg.AI.SetAccount(accountName, *account)
	}

	// Create AI pipeline
	pipeline, err := ai.NewPipeline(cfg, accountName, req.PIN)
	if err != nil {
		// Check if it's a PIN error
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

	// Transcribe only
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

	// Determine which account to use
	accountName := req.Account
	if accountName == "" {
		accountName = cfg.AI.DefaultAccount
	}
	if accountName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "no AI account configured. Add an account in Settings first.",
		})
		return
	}

	// Set model in config temporarily if provided
	account := cfg.AI.GetAccount(accountName)
	if account != nil && req.Model != "" {
		account.Summarization.Model = req.Model
		cfg.AI.SetAccount(accountName, *account)
	}

	// Create AI pipeline
	pipeline, err := ai.NewPipeline(cfg, accountName, req.PIN)
	if err != nil {
		// Check if it's a PIN error
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

	// Summarize only
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

// handleListDownloadedAudio lists audio files in the output directory
func (s *Server) handleListDownloadedAudio(c *gin.Context) {
	var audioFiles []gin.H

	audioExtensions := map[string]bool{
		".mp3": true, ".m4a": true, ".wav": true, ".aac": true,
		".ogg": true, ".flac": true, ".opus": true, ".wma": true,
	}

	err := filepath.Walk(s.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if audioExtensions[ext] {
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
		Message: fmt.Sprintf("%d audio files found", len(audioFiles)),
	})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
