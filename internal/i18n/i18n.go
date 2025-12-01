package i18n

import (
	"embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed locales/*.yml
var localesFS embed.FS

// Translations holds all translation strings organized by section
type Translations struct {
	Config       ConfigTranslations       `yaml:"config"`
	ConfigReview ConfigReviewTranslations `yaml:"config_review"`
	Help         HelpTranslations         `yaml:"help"`
	Download     DownloadTranslations     `yaml:"download"`
	Errors       ErrorTranslations        `yaml:"errors"`
	Search       SearchTranslations       `yaml:"search"`
}

type ConfigTranslations struct {
	StepOf        string `yaml:"step_of"`
	Language      string `yaml:"language"`
	LanguageDesc  string `yaml:"language_desc"`
	Proxy         string `yaml:"proxy"`
	ProxyDesc     string `yaml:"proxy_desc"`
	OutputDir     string `yaml:"output_dir"`
	OutputDirDesc string `yaml:"output_dir_desc"`
	Format        string `yaml:"format"`
	FormatDesc    string `yaml:"format_desc"`
	Quality       string `yaml:"quality"`
	QualityDesc   string `yaml:"quality_desc"`
	Confirm       string `yaml:"confirm"`
	ConfirmDesc   string `yaml:"confirm_desc"`
	YesSave       string `yaml:"yes_save"`
	NoCancel      string `yaml:"no_cancel"`
	ProxyNone     string `yaml:"proxy_none"`
	BestAvailable string `yaml:"best_available"`
	Recommended   string `yaml:"recommended"`
}

type ConfigReviewTranslations struct {
	Language  string `yaml:"language"`
	Proxy     string `yaml:"proxy"`
	OutputDir string `yaml:"output_dir"`
	Format    string `yaml:"format"`
	Quality   string `yaml:"quality"`
}

type HelpTranslations struct {
	Back    string `yaml:"back"`
	Next    string `yaml:"next"`
	Select  string `yaml:"select"`
	Confirm string `yaml:"confirm"`
	Quit    string `yaml:"quit"`
}

type DownloadTranslations struct {
	Downloading      string `yaml:"downloading"`
	Extracting       string `yaml:"extracting"`
	Completed        string `yaml:"completed"`
	Failed           string `yaml:"failed"`
	Progress         string `yaml:"progress"`
	Speed            string `yaml:"speed"`
	ETA              string `yaml:"eta"`
	FileSaved        string `yaml:"file_saved"`
	NoFormats        string `yaml:"no_formats"`
	SelectFormat     string `yaml:"select_format"`
	FormatsAvailable string `yaml:"formats_available"`
	SelectedFormat   string `yaml:"selected_format"`
	QualityHint      string `yaml:"quality_hint"`
}

type ErrorTranslations struct {
	ConfigNotFound   string `yaml:"config_not_found"`
	InvalidURL       string `yaml:"invalid_url"`
	NetworkError     string `yaml:"network_error"`
	ExtractionFailed string `yaml:"extraction_failed"`
	DownloadFailed   string `yaml:"download_failed"`
	NoExtractor      string `yaml:"no_extractor"`
}

type SearchTranslations struct {
	ResultsFor        string `yaml:"results_for"`
	Searching         string `yaml:"searching"`
	FetchingEpisodes  string `yaml:"fetching_episodes"`
	Podcasts          string `yaml:"podcasts"`
	Episodes          string `yaml:"episodes"`
	SelectHint        string `yaml:"select_hint"`
	SelectPodcastHint string `yaml:"select_podcast_hint"`
	Selected          string `yaml:"selected"`
	Help              string `yaml:"help"`
	HelpPodcast       string `yaml:"help_podcast"`
}

var (
	translationsCache = make(map[string]*Translations)
	cacheMutex        sync.RWMutex
	defaultLang       = "en"
)

// SupportedLanguages returns all available language codes
var SupportedLanguages = []struct {
	Code string
	Name string
}{
	{"en", "English"},
	{"zh", "中文"},
	{"jp", "日本語"},
	{"kr", "한국어"},
	{"es", "Español"},
	{"fr", "Français"},
	{"de", "Deutsch"},
}

// GetTranslations returns translations for the specified language
func GetTranslations(lang string) *Translations {
	cacheMutex.RLock()
	if t, ok := translationsCache[lang]; ok {
		cacheMutex.RUnlock()
		return t
	}
	cacheMutex.RUnlock()

	// Load from file
	t, err := loadTranslations(lang)
	if err != nil {
		// Fall back to English
		if lang != defaultLang {
			return GetTranslations(defaultLang)
		}
		// Return empty translations if even English fails
		return &Translations{}
	}

	cacheMutex.Lock()
	translationsCache[lang] = t
	cacheMutex.Unlock()

	return t
}

func loadTranslations(lang string) (*Translations, error) {
	filename := fmt.Sprintf("locales/%s.yml", lang)
	data, err := localesFS.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var t Translations
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

// T is a convenience function for getting translations
func T(lang string) *Translations {
	return GetTranslations(lang)
}
