package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/config"
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
	var sections []SearchSection

	// Podcasts section (not selectable for download, just info)
	if len(result.Data.Podcasts) > 0 {
		var items []SearchItem
		for _, p := range result.Data.Podcasts {
			subtitle := fmt.Sprintf("%s | %d episodes", p.Author, p.EpisodeCount)
			items = append(items, SearchItem{
				Title:      p.Title,
				Subtitle:   subtitle,
				Selectable: false,
			})
		}
		sections = append(sections, SearchSection{
			Title: t.Search.Podcasts,
			Items: items,
		})
	}

	// Episodes section (selectable for download)
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
			})
		}
		sections = append(sections, SearchSection{
			Title: t.Search.Episodes,
			Items: items,
		})
	}

	// Run TUI
	selected, err := RunSearchTUI(sections, query, cfg.Language)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		return nil
	}

	// Download selected episodes
	return downloadSelectedEpisodes(selected)
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
	var result iTunesSearchResponse
	var searchErr error

	go func() {
		searchURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=podcast&entity=podcast",
			url.QueryEscape(query))

		resp, err := http.Get(searchURL)
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

	if result.ResultCount == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Build sections for TUI
	var sections []SearchSection

	// Podcasts section (not selectable - just for info)
	var items []SearchItem
	for _, p := range result.Results {
		items = append(items, SearchItem{
			Title:      p.CollectionName,
			Subtitle:   fmt.Sprintf("%s | %d episodes | %s", p.ArtistName, p.TrackCount, p.PrimaryGenreName),
			Selectable: false,
		})
	}
	sections = append(sections, SearchSection{
		Title: t.Search.Podcasts + " (Apple Podcasts)",
		Items: items,
	})

	// Run TUI
	selected, err := RunSearchTUI(sections, query, cfg.Language)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		return nil
	}

	// For Apple Podcasts, we need to fetch episodes from the selected podcast
	fmt.Println("\nApple Podcast episode download coming soon.")

	return nil
}

// downloadSelectedEpisodes downloads the selected episodes sequentially
func downloadSelectedEpisodes(items []SearchItem) error {
	if len(items) == 0 {
		return nil
	}

	fmt.Printf("\nDownloading %d episode(s)...\n\n", len(items))

	for i, item := range items {
		fmt.Printf("[%d/%d] %s\n", i+1, len(items), item.Title)

		// Use the URL to trigger normal download flow
		if err := runDownload(item.URL); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			// Continue with next episode
		}
		fmt.Println()
	}

	return nil
}

// Search spinner model
type searchSpinnerModel struct {
	spinner spinner.Model
	query   string
	lang    string
	done    chan bool
	quit    bool
}

type searchTickMsg time.Time

func newSearchSpinnerModel(query, lang string, done chan bool) searchSpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return searchSpinnerModel{
		spinner: s,
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
	t := i18n.T(m.lang)
	return fmt.Sprintf("\n  %s %s... %s\n", m.spinner.View(), t.Search.Searching, m.query)
}

func runSearchSpinner(query, lang string, done chan bool) error {
	model := newSearchSpinnerModel(query, lang, done)
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
