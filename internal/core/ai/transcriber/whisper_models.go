package transcriber

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// WhisperModel represents a whisper.cpp model.
type WhisperModel struct {
	Name        string // Short name (e.g., "small", "medium", "large-v3-turbo")
	FileName    string // Full filename (e.g., "ggml-small.bin")
	Size        string // Human-readable size
	Description string
	URL         string // Download URL
}

// WhisperModels lists available whisper.cpp models.
var WhisperModels = []WhisperModel{
	{
		Name:        "tiny",
		FileName:    "ggml-tiny.bin",
		Size:        "75MB",
		Description: "Fastest, lowest accuracy (testing only)",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
	},
	{
		Name:        "small",
		FileName:    "ggml-small.bin",
		Size:        "244MB",
		Description: "Fast, good accuracy (recommended for NAS)",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
	},
	{
		Name:        "medium",
		FileName:    "ggml-medium.bin",
		Size:        "769MB",
		Description: "Balanced speed and accuracy",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
	},
	{
		Name:        "large-v3-turbo",
		FileName:    "ggml-large-v3-turbo.bin",
		Size:        "1.5GB",
		Description: "Best accuracy, requires GPU",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
	},
}

// DefaultWhisperModel is the recommended model for most users.
const DefaultWhisperModel = "small"

// ModelManager handles whisper model downloads and caching.
type ModelManager struct {
	modelsDir string
}

// NewModelManager creates a new model manager.
// modelsDir is typically ~/.config/vget/models/
func NewModelManager(modelsDir string) *ModelManager {
	return &ModelManager{modelsDir: modelsDir}
}

// DefaultModelsDir returns the default models directory.
func DefaultModelsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "vget", "models"), nil
}

// GetModel returns a model by name.
func GetModel(name string) *WhisperModel {
	// Normalize name: remove "ggml-" prefix and ".bin" suffix if present
	name = strings.TrimPrefix(name, "ggml-")
	name = strings.TrimSuffix(name, ".bin")

	for _, m := range WhisperModels {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

// ModelPath returns the path to a model file.
// If the model name doesn't include extension, it adds "ggml-" prefix and ".bin" suffix.
func (m *ModelManager) ModelPath(modelName string) string {
	// If it's a full path, return as-is
	if filepath.IsAbs(modelName) {
		return modelName
	}

	// If it ends with .bin, use as filename directly
	if strings.HasSuffix(modelName, ".bin") {
		return filepath.Join(m.modelsDir, modelName)
	}

	// Convert short name to full filename
	fileName := "ggml-" + modelName + ".bin"
	return filepath.Join(m.modelsDir, fileName)
}

// IsModelDownloaded checks if a model is already downloaded.
func (m *ModelManager) IsModelDownloaded(modelName string) bool {
	path := m.ModelPath(modelName)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Check if file has non-zero size
	return info.Size() > 0
}

// EnsureModel downloads a model if not already present.
// Returns the path to the model file.
func (m *ModelManager) EnsureModel(modelName string) (string, error) {
	path := m.ModelPath(modelName)

	// Check if already downloaded
	if m.IsModelDownloaded(modelName) {
		return path, nil
	}

	// Find model info
	model := GetModel(modelName)
	if model == nil {
		return "", fmt.Errorf("unknown model: %s", modelName)
	}

	// Download model
	if err := m.downloadModel(model, path); err != nil {
		return "", err
	}

	return path, nil
}

// downloadModel downloads a model from HuggingFace.
func (m *ModelManager) downloadModel(model *WhisperModel, destPath string) error {
	// Ensure models directory exists
	if err := os.MkdirAll(m.modelsDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Create temp file for download
	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	// Download
	resp, err := http.Get(model.URL)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	// Copy to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write model file: %w", err)
	}

	// Close before rename
	out.Close()

	// Rename to final path
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename model file: %w", err)
	}

	return nil
}

// ListDownloadedModels returns a list of downloaded model names.
func (m *ModelManager) ListDownloadedModels() []string {
	var models []string

	entries, err := os.ReadDir(m.modelsDir)
	if err != nil {
		return models
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "ggml-") && strings.HasSuffix(name, ".bin") {
			// Extract short name
			shortName := strings.TrimPrefix(name, "ggml-")
			shortName = strings.TrimSuffix(shortName, ".bin")
			models = append(models, shortName)
		}
	}

	return models
}

// ListAvailableModels returns info about all available models.
func (m *ModelManager) ListAvailableModels() []WhisperModelInfo {
	var result []WhisperModelInfo

	for _, model := range WhisperModels {
		info := WhisperModelInfo{
			Name:        model.Name,
			Size:        model.Size,
			Description: model.Description,
			Downloaded:  m.IsModelDownloaded(model.Name),
		}
		result = append(result, info)
	}

	return result
}

// WhisperModelInfo contains model info with download status.
type WhisperModelInfo struct {
	Name        string `json:"name"`
	Size        string `json:"size"`
	Description string `json:"description"`
	Downloaded  bool   `json:"downloaded"`
}
