package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/downloader"
	"github.com/guiyumin/vget/internal/i18n"
	"github.com/spf13/cobra"
)

var (
	podcastFlag bool
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for podcasts and episodes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !podcastFlag {
			fmt.Fprintln(os.Stderr, "Please specify a search type: --podcast")
			os.Exit(1)
		}

		query := args[0]

		// Auto-detect: if query contains Chinese characters, use Xiaoyuzhou
		// Otherwise use iTunes
		if containsChinese(query) {
			if err := searchXiaoyuzhou(query); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := searchITunes(query); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	searchCmd.Flags().BoolVar(&podcastFlag, "podcast", false, "search for podcasts")
	rootCmd.AddCommand(searchCmd)
}

// containsChinese checks if string contains Chinese characters
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// XiaoyuzhouSearchResponse represents the API response
type XiaoyuzhouSearchResponse struct {
	Data struct {
		Episodes []XiaoyuzhouEpisode `json:"episodes"`
		Podcasts []XiaoyuzhouPodcast `json:"podcasts"`
	} `json:"data"`
}

type XiaoyuzhouPodcast struct {
	Type              string `json:"type"`
	Pid               string `json:"pid"`
	Title             string `json:"title"`
	Author            string `json:"author"`
	Brief             string `json:"brief"`
	SubscriptionCount int    `json:"subscriptionCount"`
	EpisodeCount      int    `json:"episodeCount"`
}

type XiaoyuzhouEpisode struct {
	Type      string `json:"type"`
	Eid       string `json:"eid"`
	Pid       string `json:"pid"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	PlayCount int    `json:"playCount"`
	PubDate   string `json:"pubDate"`
	Enclosure struct {
		URL string `json:"url"`
	} `json:"enclosure"`
	Podcast struct {
		Title string `json:"title"`
	} `json:"podcast"`
}

func searchXiaoyuzhou(query string) error {
	cfg := config.LoadOrDefault()
	t := i18n.T(cfg.Language)

	// Show spinner while searching
	done := make(chan bool)
	var result XiaoyuzhouSearchResponse
	var searchErr error

	go func() {
		// Call Xiaoyuzhou search API
		apiURL := "https://ask.xiaoyuzhoufm.com/api/keyword/search"
		payload := fmt.Sprintf(`{"query": "%s"}`, query)

		req, err := http.NewRequest("POST", apiURL, strings.NewReader(payload))
		if err != nil {
			searchErr = err
			done <- true
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			searchErr = err
			done <- true
			return
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			searchErr = err
		}
		done <- true
	}()

	// Run spinner
	if err := runSearchSpinner(query, cfg.Language, done); err != nil {
		return err
	}

	if searchErr != nil {
		return searchErr
	}

	if len(result.Data.Podcasts) == 0 && len(result.Data.Episodes) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Build sections for TUI
	// All items are always selectable:
	// - Podcasts: select to browse episodes
	// - Episodes: select to download

	var sections []SearchSection

	// Podcasts section
	if len(result.Data.Podcasts) > 0 {
		var items []SearchItem
		for _, p := range result.Data.Podcasts {
			subtitle := fmt.Sprintf("%s | %d ep", p.Author, p.EpisodeCount)
			items = append(items, SearchItem{
				Title:      p.Title,
				Subtitle:   subtitle,
				Selectable: true,
				Type:       ItemTypePodcast,
				PodcastID:  p.Pid,
			})
		}
		sections = append(sections, SearchSection{
			Title: t.Search.Podcasts,
			Items: items,
		})
	}

	// Episodes section
	if len(result.Data.Episodes) > 0 {
		var items []SearchItem
		for _, e := range result.Data.Episodes {
			duration := formatEpisodeDuration(e.Duration)
			items = append(items, SearchItem{
				Title:       fmt.Sprintf("%s - %s", e.Podcast.Title, e.Title),
				Subtitle:    duration,
				URL:         fmt.Sprintf("https://www.xiaoyuzhoufm.com/episode/%s", e.Eid),
				DownloadURL: e.Enclosure.URL,
				Selectable:  true,
				Type:        ItemTypeEpisode,
			})
		}
		sections = append(sections, SearchSection{
			Title: t.Search.Episodes,
			Items: items,
		})
	}

	// Run TUI loop (allows going back from episode view)
	for {
		selected, err := RunSearchTUI(sections, query, cfg.Language)
		if err != nil {
			return err
		}

		if len(selected) == 0 {
			return nil
		}

		// Handle selection based on type
		err = handleSelectedItems(selected, "xiaoyuzhou", cfg.Language)
		if err == errGoBack {
			continue // Go back to podcast list
		}
		return err
	}
}

func formatEpisodeDuration(seconds int) string {
	if seconds <= 0 {
		return "?"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// iTunes API response structures
type iTunesSearchResponse struct {
	ResultCount int             `json:"resultCount"`
	Results     []iTunesResult  `json:"results"`
}

type iTunesResult struct {
	WrapperType          string `json:"wrapperType"`
	Kind                 string `json:"kind"`
	CollectionID         int    `json:"collectionId"`
	TrackID              int    `json:"trackId"`
	ArtistName           string `json:"artistName"`
	CollectionName       string `json:"collectionName"`
	TrackName            string `json:"trackName"`
	FeedURL              string `json:"feedUrl"`
	TrackCount           int    `json:"trackCount"`
	PrimaryGenreName     string `json:"primaryGenreName"`
	ReleaseDate          string `json:"releaseDate"`
	TrackTimeMillis      int    `json:"trackTimeMillis"`
	EpisodeURL           string `json:"episodeUrl"`
	EpisodeFileExtension string `json:"episodeFileExtension"`
	ShortDescription     string `json:"shortDescription"`
}

func searchITunes(query string) error {
	cfg := config.LoadOrDefault()
	t := i18n.T(cfg.Language)

	// Show spinner while searching
	done := make(chan bool)
	var podcastResult, episodeResult iTunesSearchResponse
	var searchErr error

	go func() {
		// Search for both podcasts and episodes in parallel
		var wg sync.WaitGroup
		var podcastErr, episodeErr error

		wg.Add(2)

		// Fetch podcasts
		go func() {
			defer wg.Done()
			podcastURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=podcast&entity=podcast&limit=50",
				url.QueryEscape(query))
			resp, err := http.Get(podcastURL)
			if err != nil {
				podcastErr = err
				return
			}
			defer resp.Body.Close()
			if err := json.NewDecoder(resp.Body).Decode(&podcastResult); err != nil {
				podcastErr = err
			}
		}()

		// Fetch episodes
		go func() {
			defer wg.Done()
			episodeURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=podcast&entity=podcastEpisode&limit=200",
				url.QueryEscape(query))
			resp, err := http.Get(episodeURL)
			if err != nil {
				episodeErr = err
				return
			}
			defer resp.Body.Close()
			if err := json.NewDecoder(resp.Body).Decode(&episodeResult); err != nil {
				episodeErr = err
			}
		}()

		wg.Wait()

		// Report first error encountered
		if podcastErr != nil {
			searchErr = podcastErr
		} else if episodeErr != nil {
			searchErr = episodeErr
		}
		done <- true
	}()

	// Run spinner
	if err := runSearchSpinner(query, cfg.Language, done); err != nil {
		return err
	}

	if searchErr != nil {
		return searchErr
	}

	if podcastResult.ResultCount == 0 && episodeResult.ResultCount == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Build sections for TUI - like Xiaoyuzhou, show both podcasts and episodes
	var sections []SearchSection

	// Podcasts section
	if podcastResult.ResultCount > 0 {
		var items []SearchItem
		for _, p := range podcastResult.Results {
			items = append(items, SearchItem{
				Title:      p.CollectionName,
				Subtitle:   fmt.Sprintf("%s | %d ep", p.ArtistName, p.TrackCount),
				Selectable: true,
				Type:       ItemTypePodcast,
				PodcastID:  fmt.Sprintf("%d", p.CollectionID),
				FeedURL:    p.FeedURL,
			})
		}
		sections = append(sections, SearchSection{
			Title: fmt.Sprintf("%s (%d)", t.Search.Podcasts, podcastResult.ResultCount),
			Items: items,
		})
	}

	// Episodes section
	if episodeResult.ResultCount > 0 {
		var items []SearchItem
		for _, p := range episodeResult.Results {
			duration := formatEpisodeDuration(p.TrackTimeMillis / 1000)
			items = append(items, SearchItem{
				Title:       fmt.Sprintf("%s - %s", p.CollectionName, p.TrackName),
				Subtitle:    duration,
				Selectable:  true,
				Type:        ItemTypeEpisode,
				DownloadURL: p.EpisodeURL,
			})
		}
		sections = append(sections, SearchSection{
			Title: fmt.Sprintf("%s (%d)", t.Search.Episodes, episodeResult.ResultCount),
			Items: items,
		})
	}

	// Run TUI loop (allows going back from episode view)
	for {
		selected, err := RunSearchTUI(sections, query, cfg.Language)
		if err != nil {
			return err
		}

		if len(selected) == 0 {
			return nil
		}

		// Handle selection
		err = handleSelectedItems(selected, "itunes", cfg.Language)
		if err == errGoBack {
			continue // Go back to podcast list
		}
		return err
	}
}

// handleSelectedItems processes selected items based on their type
func handleSelectedItems(items []SearchItem, source, lang string) error {
	if len(items) == 0 {
		return nil
	}

	// Check if selected items are podcasts or episodes
	firstItem := items[0]

	if firstItem.Type == ItemTypePodcast {
		// User selected podcasts - fetch episodes for each
		// For simplicity, only handle first selected podcast
		podcast := items[0]
		if len(items) > 1 {
			fmt.Printf("\nNote: Multiple podcasts selected, showing episodes for: %s\n", podcast.Title)
		}

		return fetchAndShowEpisodes(podcast, source, lang)
	}

	// Episodes selected - download them
	return downloadSelectedEpisodes(items)
}

// fetchAndShowEpisodes fetches episodes for a podcast and shows TUI
func fetchAndShowEpisodes(podcast SearchItem, source, lang string) error {
	t := i18n.T(lang)

	// Show spinner while fetching episodes
	done := make(chan bool)
	var episodes []SearchItem
	var fetchErr error

	go func() {
		switch source {
		case "itunes":
			episodes, fetchErr = fetchITunesEpisodes(podcast.PodcastID)
		case "xiaoyuzhou":
			episodes, fetchErr = fetchXiaoyuzhouEpisodes(podcast.PodcastID)
		default:
			fetchErr = fmt.Errorf("unknown source: %s", source)
		}
		done <- true
	}()

	// Run spinner with podcast title
	if err := runFetchEpisodesSpinner(podcast.Title, lang, done); err != nil {
		return err
	}

	if fetchErr != nil {
		return fetchErr
	}

	if len(episodes) == 0 {
		fmt.Println("No episodes found.")
		return nil
	}

	// Build sections for episode selection TUI
	sections := []SearchSection{
		{
			Title: fmt.Sprintf("%s - %s", t.Search.Episodes, podcast.Title),
			Items: episodes,
		},
	}

	// Run TUI for episode selection with back navigation enabled
	selected, err := RunSearchTUIWithBack(sections, podcast.Title, lang, true)
	if err == errGoBack {
		return errGoBack // Propagate back to original search
	}
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		return nil
	}

	// Download selected episodes
	return downloadSelectedEpisodes(selected)
}

// fetchITunesEpisodes fetches episodes for an iTunes podcast
func fetchITunesEpisodes(podcastID string) ([]SearchItem, error) {
	// Use iTunes Lookup API to get episodes
	lookupURL := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&entity=podcastEpisode&limit=50", podcastID)

	resp, err := http.Get(lookupURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		ResultCount int `json:"resultCount"`
		Results     []struct {
			WrapperType          string `json:"wrapperType"`
			TrackID              int    `json:"trackId"`
			TrackName            string `json:"trackName"`
			CollectionName       string `json:"collectionName"`
			ArtistName           string `json:"artistName"`
			EpisodeURL           string `json:"episodeUrl"`
			EpisodeFileExtension string `json:"episodeFileExtension"`
			TrackTimeMillis      int    `json:"trackTimeMillis"`
			ReleaseDate          string `json:"releaseDate"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var episodes []SearchItem
	for _, r := range result.Results {
		// Skip the podcast itself (first result is usually the podcast info)
		if r.WrapperType != "podcastEpisode" {
			continue
		}

		duration := formatEpisodeDuration(r.TrackTimeMillis / 1000)
		episodes = append(episodes, SearchItem{
			Title:       r.TrackName,
			Subtitle:    duration,
			URL:         fmt.Sprintf("https://podcasts.apple.com/podcast/id%s?i=%d", "", r.TrackID),
			DownloadURL: r.EpisodeURL,
			Selectable:  true,
			Type:        ItemTypeEpisode,
		})
	}

	return episodes, nil
}

// fetchXiaoyuzhouEpisodes fetches episodes for a Xiaoyuzhou podcast
func fetchXiaoyuzhouEpisodes(podcastID string) ([]SearchItem, error) {
	// Fetch podcast page which contains __NEXT_DATA__ with episodes
	pageURL := fmt.Sprintf("https://www.xiaoyuzhoufm.com/podcast/%s", podcastID)

	resp, err := http.Get(pageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract __NEXT_DATA__ JSON from the page
	html := string(body)
	startMarker := `<script id="__NEXT_DATA__" type="application/json">`
	endMarker := `</script>`

	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		return nil, fmt.Errorf("could not find episode data on page")
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(html[startIdx:], endMarker)
	if endIdx == -1 {
		return nil, fmt.Errorf("could not parse episode data")
	}

	jsonData := html[startIdx : startIdx+endIdx]

	// Parse the JSON to extract episodes
	var nextData struct {
		Props struct {
			PageProps struct {
				Podcast struct {
					Title    string `json:"title"`
					Episodes []struct {
						Eid       string `json:"eid"`
						Title     string `json:"title"`
						Duration  int    `json:"duration"`
						Enclosure struct {
							URL string `json:"url"`
						} `json:"enclosure"`
					} `json:"episodes"`
				} `json:"podcast"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(jsonData), &nextData); err != nil {
		return nil, fmt.Errorf("failed to parse episode data: %v", err)
	}

	podcast := nextData.Props.PageProps.Podcast
	if len(podcast.Episodes) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	podcastTitle := podcast.Title
	episodes := podcast.Episodes

	var items []SearchItem
	for _, e := range episodes {
		duration := formatEpisodeDuration(e.Duration)
		items = append(items, SearchItem{
			Title:       fmt.Sprintf("%s - %s", podcastTitle, e.Title),
			Subtitle:    duration,
			URL:         fmt.Sprintf("https://www.xiaoyuzhoufm.com/episode/%s", e.Eid),
			DownloadURL: e.Enclosure.URL,
			Selectable:  true,
			Type:        ItemTypeEpisode,
		})
	}

	return items, nil
}

// downloadSelectedEpisodes downloads the selected episodes sequentially
func downloadSelectedEpisodes(items []SearchItem) error {
	if len(items) == 0 {
		return nil
	}

	fmt.Printf("\nDownloading %d episode(s)...\n\n", len(items))

	for i, item := range items {
		fmt.Printf("[%d/%d] %s\n", i+1, len(items), item.Title)

		// If we have a direct download URL, use it
		if item.DownloadURL != "" {
			if err := runDirectDownload(item.DownloadURL, item.Title); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
		} else if item.URL != "" {
			// Use the URL to trigger normal download flow
			if err := runDownload(item.URL); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
		}
		fmt.Println()
	}

	return nil
}

// runDirectDownload downloads a file directly from URL
func runDirectDownload(downloadURL, title string) error {
	// Use the downloader directly
	cfg := config.LoadOrDefault()

	// Determine extension from URL
	ext := "mp3"
	if strings.Contains(downloadURL, ".m4a") {
		ext = "m4a"
	}

	filename := sanitizeFilenameForDownload(title) + "." + ext
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = "."
	}

	// Join directory and filename to create full path
	outputPath := filepath.Join(outputDir, filename)

	d := downloader.New(cfg.Language)
	return d.Download(downloadURL, outputPath, title)
}

// sanitizeFilenameForDownload removes invalid characters from filename
func sanitizeFilenameForDownload(name string) string {
	// Replace invalid characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	result := replacer.Replace(name)
	// Trim spaces and limit length
	result = strings.TrimSpace(result)
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

// Search spinner model
type searchSpinnerModel struct {
	spinner spinner.Model
	message string // The action message (e.g., "Searching", "Fetching episodes for")
	query   string
	lang    string
	done    chan bool
	quit    bool
}

type searchTickMsg time.Time

func newSearchSpinnerModel(message, query, lang string, done chan bool) searchSpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return searchSpinnerModel{
		spinner: s,
		message: message,
		query:   query,
		lang:    lang,
		done:    done,
	}
}

func (m searchSpinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.checkDone())
}

func (m searchSpinnerModel) checkDone() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return searchTickMsg(t)
	})
}

func (m searchSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quit = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case searchTickMsg:
		select {
		case <-m.done:
			return m, tea.Quit
		default:
			return m, m.checkDone()
		}
	}

	return m, nil
}

func (m searchSpinnerModel) View() string {
	return fmt.Sprintf("\n  %s %s... %s\n", m.spinner.View(), m.message, m.query)
}

func runSearchSpinner(query, lang string, done chan bool) error {
	t := i18n.T(lang)
	model := newSearchSpinnerModel(t.Search.Searching, query, lang, done)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if finalModel.(searchSpinnerModel).quit {
		return fmt.Errorf("search cancelled")
	}
	return nil
}

func runFetchEpisodesSpinner(podcastTitle, lang string, done chan bool) error {
	t := i18n.T(lang)
	model := newSearchSpinnerModel(t.Search.FetchingEpisodes, podcastTitle, lang, done)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if finalModel.(searchSpinnerModel).quit {
		return fmt.Errorf("cancelled")
	}
	return nil
}
