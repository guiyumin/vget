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
	Twitter      TwitterTranslations      `yaml:"twitter"`
	Sites        SitesTranslations        `yaml:"sites"`
	UI           UITranslations           `yaml:"ui"`
	Server       ServerTranslations       `yaml:"server"`
	YouTube      YouTubeTranslations      `yaml:"youtube"`
}

type ConfigTranslations struct {
	StepOf        string `yaml:"step_of"`
	Language      string `yaml:"language"`
	LanguageDesc  string `yaml:"language_desc"`
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
	BestAvailable string `yaml:"best_available"`
	Recommended   string `yaml:"recommended"`
}

type ConfigReviewTranslations struct {
	Language  string `yaml:"language"`
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
	Elapsed          string `yaml:"elapsed"`
	AvgSpeed         string `yaml:"avg_speed"`
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

type TwitterTranslations struct {
	EnterAuthToken    string `yaml:"enter_auth_token"`
	AuthSaved         string `yaml:"auth_saved"`
	AuthCanDownload   string `yaml:"auth_can_download"`
	AuthCleared       string `yaml:"auth_cleared"`
	AuthRequired      string `yaml:"auth_required"`
	NsfwLoginRequired string `yaml:"nsfw_login_required"`
	ProtectedTweet    string `yaml:"protected_tweet"`
	TweetUnavailable  string `yaml:"tweet_unavailable"`
	AuthHint             string `yaml:"auth_hint"`
	DeprecatedSet        string `yaml:"deprecated_set"`
	DeprecatedClear      string `yaml:"deprecated_clear"`
	DeprecatedUseNew     string `yaml:"deprecated_use_new"`
	DeprecatedUseNewUnset string `yaml:"deprecated_use_new_unset"`
}

type SitesTranslations struct {
	ConfigureSite   string `yaml:"configure_site"`
	DomainMatch     string `yaml:"domain_match"`
	SelectType      string `yaml:"select_type"`
	OnlyM3u8ForNow  string `yaml:"only_m3u8_for_now"`
	ExistingSites   string `yaml:"existing_sites"`
	SiteAdded       string `yaml:"site_added"`
	SavedTo         string `yaml:"saved_to"`
	Cancelled       string `yaml:"cancelled"`
	EnterConfirm    string `yaml:"enter_confirm"`
	EscCancel       string `yaml:"esc_cancel"`
}

// UITranslations holds translations for the web UI
type UITranslations struct {
	DownloadTo       string `yaml:"download_to" json:"download_to"`
	Edit             string `yaml:"edit" json:"edit"`
	Save             string `yaml:"save" json:"save"`
	Cancel           string `yaml:"cancel" json:"cancel"`
	PasteURL         string `yaml:"paste_url" json:"paste_url"`
	Download         string `yaml:"download" json:"download"`
	BulkDownload     string `yaml:"bulk_download" json:"bulk_download"`
	ComingSoon       string `yaml:"coming_soon" json:"coming_soon"`
	Adding           string `yaml:"adding" json:"adding"`
	Jobs             string `yaml:"jobs" json:"jobs"`
	Total            string `yaml:"total" json:"total"`
	NoDownloads      string `yaml:"no_downloads" json:"no_downloads"`
	PasteHint        string `yaml:"paste_hint" json:"paste_hint"`
	Queued           string `yaml:"queued" json:"queued"`
	Downloading      string `yaml:"downloading" json:"downloading"`
	Completed        string `yaml:"completed" json:"completed"`
	Failed           string `yaml:"failed" json:"failed"`
	Cancelled        string `yaml:"cancelled" json:"cancelled"`
	Settings         string `yaml:"settings" json:"settings"`
	Language         string `yaml:"language" json:"language"`
	Format           string `yaml:"format" json:"format"`
	Quality          string `yaml:"quality" json:"quality"`
	TwitterAuth      string `yaml:"twitter_auth" json:"twitter_auth"`
	ServerPort       string `yaml:"server_port" json:"server_port"`
	MaxConcurrent    string `yaml:"max_concurrent" json:"max_concurrent"`
	APIKey           string `yaml:"api_key" json:"api_key"`
	WebDAVServers    string `yaml:"webdav_servers" json:"webdav_servers"`
	Add              string `yaml:"add" json:"add"`
	Delete           string `yaml:"delete" json:"delete"`
	Name             string `yaml:"name" json:"name"`
	URL              string `yaml:"url" json:"url"`
	Username         string `yaml:"username" json:"username"`
	Password         string `yaml:"password" json:"password"`
	NoWebDAVServers  string `yaml:"no_webdav_servers" json:"no_webdav_servers"`
	Configured       string `yaml:"configured" json:"configured"`
	NotConfigured    string `yaml:"not_configured" json:"not_configured"`
	ClearHistory     string `yaml:"clear_history" json:"clear_history"`
	ClearAll         string `yaml:"clear_all" json:"clear_all"`
	// WebDAV
	WebDAVBrowser    string `yaml:"webdav_browser" json:"webdav_browser"`
	SelectRemote     string `yaml:"select_remote" json:"select_remote"`
	EmptyDirectory   string `yaml:"empty_directory" json:"empty_directory"`
	DownloadSelected string `yaml:"download_selected" json:"download_selected"`
	SelectedFiles    string `yaml:"selected_files" json:"selected_files"`
	Loading          string `yaml:"loading" json:"loading"`
	GoToSettings     string `yaml:"go_to_settings" json:"go_to_settings"`
	// Torrent
	Torrent              string `yaml:"torrent" json:"torrent"`
	TorrentHint          string `yaml:"torrent_hint" json:"torrent_hint"`
	TorrentSubmit        string `yaml:"torrent_submit" json:"torrent_submit"`
	TorrentSubmitting    string `yaml:"torrent_submitting" json:"torrent_submitting"`
	TorrentSuccess       string `yaml:"torrent_success" json:"torrent_success"`
	TorrentNotConfigured string `yaml:"torrent_not_configured" json:"torrent_not_configured"`
	TorrentSettings      string `yaml:"torrent_settings" json:"torrent_settings"`
	TorrentClient        string `yaml:"torrent_client" json:"torrent_client"`
	TorrentHost          string `yaml:"torrent_host" json:"torrent_host"`
	TorrentTest          string `yaml:"torrent_test" json:"torrent_test"`
	TorrentTesting       string `yaml:"torrent_testing" json:"torrent_testing"`
	TorrentTestSuccess   string `yaml:"torrent_test_success" json:"torrent_test_success"`
	TorrentEnabled       string `yaml:"torrent_enabled" json:"torrent_enabled"`
	// Toast
	DownloadQueued  string `yaml:"download_queued" json:"download_queued"`
	DownloadsQueued string `yaml:"downloads_queued" json:"downloads_queued"`
	// Podcast
	Podcast                string `yaml:"podcast" json:"podcast"`
	PodcastSearch          string `yaml:"podcast_search" json:"podcast_search"`
	PodcastSearchHint      string `yaml:"podcast_search_hint" json:"podcast_search_hint"`
	PodcastSearching       string `yaml:"podcast_searching" json:"podcast_searching"`
	PodcastChannels        string `yaml:"podcast_channels" json:"podcast_channels"`
	PodcastEpisodes        string `yaml:"podcast_episodes" json:"podcast_episodes"`
	PodcastNoResults       string `yaml:"podcast_no_results" json:"podcast_no_results"`
	PodcastEpisodesCount   string `yaml:"podcast_episodes_count" json:"podcast_episodes_count"`
	PodcastBack            string `yaml:"podcast_back" json:"podcast_back"`
	PodcastDownloadStarted string `yaml:"podcast_download_started" json:"podcast_download_started"`
}

// ServerTranslations holds translations for server messages
type ServerTranslations struct {
	NoConfigWarning string `yaml:"no_config_warning" json:"no_config_warning"`
	RunInitHint     string `yaml:"run_init_hint" json:"run_init_hint"`
}

// YouTubeTranslations holds translations for YouTube messages
type YouTubeTranslations struct {
	DockerRequired   string `yaml:"docker_required"`
	DockerHintServer string `yaml:"docker_hint_server"`
	DockerHintCLI    string `yaml:"docker_hint_cli"`
}

var (
	translationsCache = make(map[string]*Translations)
	cacheMutex        sync.RWMutex
	defaultLang       = "zh"
)

// SupportedLanguages returns all available language codes
var SupportedLanguages = []struct {
	Code string
	Name string
}{
	{"zh", "中文"},
	{"en", "English"},
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
