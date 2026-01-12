package transcriber

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/downloader"
)

// User-Agent for vget downloads (used for CDN protection)
const vgetUserAgent = "vget/1.0 (+https://github.com/guiyumin/vget)"

// ASRModel represents a speech recognition model.
type ASRModel struct {
	Name        string // Short name (e.g., "whisper-small", "whisper-large-v3-turbo")
	Engine      string // Currently only "whisper"
	DirName     string // Filename for Whisper ggml models
	Size        string // Human-readable size
	Description string
	OfficialURL string // Official download URL (GitHub/Hugging Face)
	VmirrorURL  string // vmirror.org CDN URL (empty if not available)
	Languages   int    // Number of supported languages
	IsFile      bool   // True for single-file models (ggml), false for directories
}

// ASRModels lists available models.
var ASRModels = []ASRModel{
	// Whisper models via whisper.cpp (ggml format)
	{
		Name:        "whisper-tiny",
		Engine:      "whisper",
		DirName:     "whisper-tiny.bin",
		Size:        "78MB",
		Description: "Fastest, basic quality",
		OfficialURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
		VmirrorURL:  "https://cdn2.vmirror.org/models/whisper-tiny.bin",
		Languages:   100,
		IsFile:      true,
	},
	{
		Name:        "whisper-base",
		Engine:      "whisper",
		DirName:     "whisper-base.bin",
		Size:        "148MB",
		Description: "Good for quick drafts",
		OfficialURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
		VmirrorURL:  "https://cdn2.vmirror.org/models/whisper-base.bin",
		Languages:   100,
		IsFile:      true,
	},
	{
		Name:        "whisper-small",
		Engine:      "whisper",
		DirName:     "whisper-small.bin",
		Size:        "488MB",
		Description: "Balanced for most uses",
		OfficialURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
		VmirrorURL:  "https://cdn2.vmirror.org/models/whisper-small.bin",
		Languages:   100,
		IsFile:      true,
	},
	{
		Name:        "whisper-medium",
		Engine:      "whisper",
		DirName:     "whisper-medium.bin",
		Size:        "1.5GB",
		Description: "Higher accuracy",
		OfficialURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
		VmirrorURL:  "https://cdn2.vmirror.org/models/whisper-medium.bin",
		Languages:   100,
		IsFile:      true,
	},
	{
		Name:        "whisper-large-v3",
		Engine:      "whisper",
		DirName:     "whisper-large-v3.bin",
		Size:        "3.1GB",
		Description: "Highest accuracy, slowest",
		OfficialURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin",
		VmirrorURL:  "https://cdn2.vmirror.org/models/whisper-large-v3.bin",
		Languages:   100,
		IsFile:      true,
	},
	{
		Name:        "whisper-large-v3-turbo",
		Engine:      "whisper",
		DirName:     "whisper-large-v3-turbo.bin",
		Size:        "1.6GB",
		Description: "Best quality + fast (recommended)",
		OfficialURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
		VmirrorURL:  "https://cdn2.vmirror.org/models/whisper-large-v3-turbo.bin",
		Languages:   100,
		IsFile:      true,
	},
}

// DefaultModel is the recommended model for most users.
const DefaultModel = "whisper-large-v3-turbo"

// DefaultWhisperModel is the recommended Whisper model.
const DefaultWhisperModel = "whisper-large-v3-turbo"

// ModelManager handles model downloads and caching.
type ModelManager struct {
	modelsDir string
}

// NewModelManager creates a new model manager.
func NewModelManager(modelsDir string) *ModelManager {
	return &ModelManager{modelsDir: modelsDir}
}

// DefaultModelsDir returns the default models directory.
// In Docker, models are stored in /home/vget/downloads/models (persisted via bind mount).
// On host systems, models are stored in ~/.config/vget/models.
func DefaultModelsDir() (string, error) {
	// Docker: use downloads/models directory which is already bind-mounted
	if config.IsRunningInDocker() {
		return "/home/vget/downloads/models", nil
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
	return GetModel(DefaultModel)
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
	// For directory-based models, check if it's a directory
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

// DownloadFromURL downloads a model from a custom URL.
func (m *ModelManager) DownloadFromURL(modelName, url string) (string, error) {
	model := GetModel(modelName)
	if model == nil {
		return "", fmt.Errorf("unknown model: %s", modelName)
	}

	// Create a copy with custom URL
	customModel := *model
	customModel.OfficialURL = url

	if err := m.downloadModel(&customModel); err != nil {
		return "", err
	}

	return m.ModelPath(modelName), nil
}

// DownloadModelWithProgress downloads a model with progress display.
func (m *ModelManager) DownloadModelWithProgress(modelName, url, lang string) (string, error) {
	model := GetModel(modelName)
	if model == nil {
		return "", fmt.Errorf("unknown model: %s", modelName)
	}

	// Ensure models directory exists
	if err := os.MkdirAll(m.modelsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create models directory: %w", err)
	}

	// Download URL
	downloadURL := url
	if downloadURL == "" {
		downloadURL = model.OfficialURL
	}

	// Build headers (just User-Agent, signed URLs handle auth)
	headers := map[string]string{
		"User-Agent": vgetUserAgent,
	}

	// Target path
	target := filepath.Join(m.modelsDir, model.DirName)

	// Try TUI progress bar first, fall back to simple progress if TTY not available
	err := downloader.RunDownloadTUI(downloadURL, target, modelName, lang, headers)
	if err != nil && isNoTTYError(err) {
		// Fall back to simple progress display
		fmt.Printf("URL: %s\n\n", downloadURL)
		if err := m.downloadModelWithSimpleProgress(model, downloadURL, headers); err != nil {
			return "", err
		}
		return target, nil
	}
	if err != nil {
		return "", err
	}

	return target, nil
}

// isNoTTYError checks if error is due to missing TTY
func isNoTTYError(err error) bool {
	return err != nil && (
		// Common TTY-related errors
		err.Error() == "could not open a new TTY: open /dev/tty: device not configured" ||
		err.Error() == "could not open a new TTY: open /dev/tty: no such device or address")
}

// downloadModelWithSimpleProgress downloads with simple console progress
func (m *ModelManager) downloadModelWithSimpleProgress(model *ASRModel, url string, headers map[string]string) error {
	// Create request with headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	target := filepath.Join(m.modelsDir, model.DirName)
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	total := resp.ContentLength
	var current int64
	buf := make([]byte, 32*1024)
	lastPercent := -1

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			current += int64(n)

			// Print progress every 5%
			if total > 0 {
				percent := int(float64(current) / float64(total) * 100)
				if percent/5 > lastPercent/5 {
					fmt.Printf("\r  Progress: %d%% (%s / %s)", percent, formatBytes(current), formatBytes(total))
					lastPercent = percent
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}
	fmt.Println()

	return nil
}

// formatBytes formats bytes to human readable string
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// downloadModel downloads a model (file or archive) from URL.
func (m *ModelManager) downloadModel(model *ASRModel) error {
	// Ensure models directory exists
	if err := os.MkdirAll(m.modelsDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Download with custom User-Agent for CDN protection
	req, err := http.NewRequest("GET", model.OfficialURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", vgetUserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	// Download single-file model (ggml for whisper.cpp)
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

// IsAvailableOnVmirror checks if a model is available on vmirror CDN.
func IsAvailableOnVmirror(modelName string) bool {
	model := GetModel(modelName)
	return model != nil && model.VmirrorURL != ""
}

// GetVmirrorURL returns the vmirror download URL for a model, or empty string if not available.
func GetVmirrorURL(modelName string) string {
	model := GetModel(modelName)
	if model == nil {
		return ""
	}
	return model.VmirrorURL
}

// ListVmirrorModels returns names of models available on vmirror.
func ListVmirrorModels() []string {
	var names []string
	for _, m := range ASRModels {
		if m.VmirrorURL != "" {
			names = append(names, m.Name)
		}
	}
	return names
}

// GetVmirrorFilename returns the filename for a model on vmirror CDN.
// e.g., "whisper-tiny.bin"
func GetVmirrorFilename(modelName string) string {
	model := GetModel(modelName)
	if model == nil {
		return ""
	}
	return model.DirName
}

// Whisper supported languages (100 languages)
var whisperLangs = map[string]bool{
	"af": true, "am": true, "ar": true, "as": true, "az": true,
	"ba": true, "be": true, "bg": true, "bn": true, "bo": true,
	"br": true, "bs": true, "ca": true, "cs": true, "cy": true,
	"da": true, "de": true, "el": true, "en": true, "es": true,
	"et": true, "eu": true, "fa": true, "fi": true, "fo": true,
	"fr": true, "gl": true, "gu": true, "ha": true, "haw": true,
	"he": true, "hi": true, "hr": true, "ht": true, "hu": true,
	"hy": true, "id": true, "is": true, "it": true, "ja": true,
	"jw": true, "ka": true, "kk": true, "km": true, "kn": true,
	"ko": true, "la": true, "lb": true, "ln": true, "lo": true,
	"lt": true, "lv": true, "mg": true, "mi": true, "mk": true,
	"ml": true, "mn": true, "mr": true, "ms": true, "mt": true,
	"my": true, "ne": true, "nl": true, "nn": true, "no": true,
	"oc": true, "pa": true, "pl": true, "ps": true, "pt": true,
	"ro": true, "ru": true, "sa": true, "sd": true, "si": true,
	"sk": true, "sl": true, "sn": true, "so": true, "sq": true,
	"sr": true, "su": true, "sv": true, "sw": true, "ta": true,
	"te": true, "tg": true, "th": true, "tk": true, "tl": true,
	"tr": true, "tt": true, "uk": true, "ur": true, "uz": true,
	"vi": true, "yi": true, "yo": true, "zh": true, "yue": true,
}

// RecommendModel recommends a model based on language.
func RecommendModel(language string) string {
	return DefaultModel
}

// RecommendEngine recommends an engine based on language.
func RecommendEngine(language string) string {
	return "whisper"
}

// ModelSupportsLanguage checks if a model supports a given language.
// Whisper models support 100 languages.
func ModelSupportsLanguage(modelName, lang string) bool {
	model := GetModel(modelName)
	if model == nil {
		return false
	}

	if model.Engine == "whisper" {
		return whisperLangs[lang]
	}

	return false
}

// IsValidLanguage checks if a language code is valid (supported by any model).
func IsValidLanguage(lang string) bool {
	return whisperLangs[lang]
}

