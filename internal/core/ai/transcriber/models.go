package transcriber

import (
	"archive/tar"
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/guiyumin/vget/internal/core/config"
)

// ASRModel represents a speech recognition model.
type ASRModel struct {
	Name        string // Short name (e.g., "parakeet-v3", "whisper-small")
	Engine      string // "parakeet" or "whisper"
	DirName     string // Directory name (Parakeet) or filename (Whisper ggml)
	Size        string // Human-readable size
	Description string
	URL         string // Download URL
	Languages   int    // Number of supported languages
	IsFile      bool   // True for single-file models (ggml), false for directories
}

// ASRModels lists available models.
var ASRModels = []ASRModel{
	// Parakeet V3 via sherpa-onnx - best for European languages
	{
		Name:        "parakeet-v3",
		Engine:      "parakeet",
		DirName:     "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8",
		Size:        "640MB",
		Description: "Best for European languages (25 supported), fastest on CPU",
		URL:         "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2",
		Languages:   25,
		IsFile:      false,
	},
	// Whisper models via whisper.cpp (ggml format) - for Chinese and other languages
	{
		Name:        "whisper-small",
		Engine:      "whisper",
		DirName:     "ggml-small.bin",
		Size:        "466MB",
		Description: "Fast, good accuracy, supports Chinese (whisper.cpp)",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
		Languages:   99,
		IsFile:      true,
	},
	{
		Name:        "whisper-medium",
		Engine:      "whisper",
		DirName:     "ggml-medium.bin",
		Size:        "1.5GB",
		Description: "Balanced speed and accuracy (whisper.cpp)",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
		Languages:   99,
		IsFile:      true,
	},
	{
		Name:        "whisper-turbo",
		Engine:      "whisper",
		DirName:     "ggml-large-v3-turbo.bin",
		Size:        "1.6GB",
		Description: "Best accuracy, 8x faster than large (whisper.cpp)",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
		Languages:   99,
		IsFile:      true,
	},
}

// DefaultModel is the recommended model for most users.
const DefaultModel = "parakeet-v3"

// DefaultWhisperModel is the recommended Whisper model for Chinese users.
const DefaultWhisperModel = "whisper-small"

// ModelManager handles model downloads and caching.
type ModelManager struct {
	modelsDir string
}

// NewModelManager creates a new model manager.
func NewModelManager(modelsDir string) *ModelManager {
	return &ModelManager{modelsDir: modelsDir}
}

// DefaultModelsDir returns the default models directory.
// In Docker, models are stored in /home/vget/models to avoid bind mount conflicts.
// On host systems, models are stored in ~/.config/vget/models.
func DefaultModelsDir() (string, error) {
	// Docker: use separate directory from config to avoid bind mount overwriting models
	if config.IsRunningInDocker() {
		return "/home/vget/models", nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "vget", "models"), nil
}

// GetModel returns a model by name.
func GetModel(name string) *ASRModel {
	for _, m := range ASRModels {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

// GetModelByEngine returns the default model for an engine.
func GetModelByEngine(engine string) *ASRModel {
	switch engine {
	case "parakeet":
		return GetModel("parakeet-v3")
	case "whisper":
		return GetModel("whisper-small")
	default:
		return GetModel(DefaultModel)
	}
}

// ModelPath returns the path to a model directory.
func (m *ModelManager) ModelPath(modelName string) string {
	model := GetModel(modelName)
	if model == nil {
		return filepath.Join(m.modelsDir, modelName)
	}
	return filepath.Join(m.modelsDir, model.DirName)
}

// IsModelDownloaded checks if a model is already downloaded.
func (m *ModelManager) IsModelDownloaded(modelName string) bool {
	path := m.ModelPath(modelName)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	model := GetModel(modelName)
	if model != nil && model.IsFile {
		// For file-based models (ggml), check if it's a file
		return !info.IsDir()
	}
	// For directory-based models (Parakeet), check if it's a directory
	return info.IsDir()
}

// EnsureModel downloads a model if not already present.
func (m *ModelManager) EnsureModel(modelName string) (string, error) {
	path := m.ModelPath(modelName)

	if m.IsModelDownloaded(modelName) {
		return path, nil
	}

	model := GetModel(modelName)
	if model == nil {
		return "", fmt.Errorf("unknown model: %s", modelName)
	}

	if err := m.downloadModel(model); err != nil {
		return "", err
	}

	return path, nil
}

// downloadModel downloads a model (file or archive) from URL.
func (m *ModelManager) downloadModel(model *ASRModel) error {
	// Ensure models directory exists
	if err := os.MkdirAll(m.modelsDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Download
	resp, err := http.Get(model.URL)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	// Handle single-file models (ggml for whisper.cpp)
	if model.IsFile {
		target := filepath.Join(m.modelsDir, model.DirName)
		file, err := os.Create(target)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(file, resp.Body); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		return nil
	}

	// Handle tar.bz2 archives (Parakeet via sherpa-onnx)
	bzReader := bzip2.NewReader(resp.Body)
	tarReader := tar.NewReader(bzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Build target path
		target := filepath.Join(m.modelsDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			file, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			file.Close()
		}
	}

	return nil
}

// ListDownloadedModels returns a list of downloaded model names.
func (m *ModelManager) ListDownloadedModels() []string {
	var models []string

	for _, model := range ASRModels {
		if m.IsModelDownloaded(model.Name) {
			models = append(models, model.Name)
		}
	}

	return models
}

// ListAvailableModels returns info about all available models.
func (m *ModelManager) ListAvailableModels() []ASRModelInfo {
	var result []ASRModelInfo

	for _, model := range ASRModels {
		info := ASRModelInfo{
			Name:        model.Name,
			Engine:      model.Engine,
			Size:        model.Size,
			Description: model.Description,
			Languages:   model.Languages,
			Downloaded:  m.IsModelDownloaded(model.Name),
		}
		result = append(result, info)
	}

	return result
}

// ASRModelInfo contains model info with download status.
type ASRModelInfo struct {
	Name        string `json:"name"`
	Engine      string `json:"engine"`
	Size        string `json:"size"`
	Description string `json:"description"`
	Languages   int    `json:"languages"`
	Downloaded  bool   `json:"downloaded"`
}

// RecommendModel recommends a model based on language.
func RecommendModel(language string) string {
	// Languages that require Whisper (not supported by Parakeet)
	whisperOnlyLangs := map[string]bool{
		"zh": true, "ja": true, "ko": true, "ar": true, "he": true,
		"hi": true, "th": true, "vi": true, "id": true, "ms": true,
	}

	language = strings.ToLower(language)
	if whisperOnlyLangs[language] {
		return DefaultWhisperModel
	}

	return DefaultModel
}

// RecommendEngine recommends an engine based on language.
func RecommendEngine(language string) string {
	model := RecommendModel(language)
	m := GetModel(model)
	if m != nil {
		return m.Engine
	}
	return "parakeet"
}
