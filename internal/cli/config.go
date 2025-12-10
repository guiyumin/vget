package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/i18n"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage vget configuration",
	Long:  "View and modify vget settings, including WebDAV remotes",
}

// vget config show - show current config
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()

		fmt.Println("Current configuration:")
		fmt.Printf("  Language:  %s\n", cfg.Language)
		fmt.Printf("  Proxy:     %s\n", orDefault(cfg.Proxy, "(none)"))
		fmt.Printf("  OutputDir: %s\n", cfg.OutputDir)
		fmt.Printf("  Format:    %s\n", cfg.Format)
		fmt.Printf("  Quality:   %s\n", cfg.Quality)
		fmt.Printf("  Config:    %s\n", config.SavePath())

		if len(cfg.WebDAVServers) > 0 {
			fmt.Println("\nWebDAV servers:")
			for name, server := range cfg.WebDAVServers {
				fmt.Printf("  %s: %s\n", name, server.URL)
			}
		}

		if cfg.Twitter.AuthToken != "" {
			fmt.Println("\nTwitter:")
			fmt.Printf("  auth_token: %s...%s\n", cfg.Twitter.AuthToken[:4], cfg.Twitter.AuthToken[len(cfg.Twitter.AuthToken)-4:])
		}
	},
}

// vget config path - show config file path
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.SavePath())
	},
}

// vget config set KEY VALUE - set a config value
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value in config.yml.

Supported keys:
  language           Language code (en, zh, jp, kr, es, fr, de)
  proxy              Proxy URL (e.g., http://127.0.0.1:7890)
  output_dir         Default download directory
  format             Preferred format (mp4, webm, best)
  quality            Default quality (1080p, 720p, best)
  filename_template  Output filename template
  twitter.auth_token Twitter auth token for NSFW content
  server.port        Server listen port
  server.max_concurrent  Max concurrent downloads
  server.api_key     Server API key

Examples:
  vget config set language en
  vget config set proxy http://127.0.0.1:7890
  vget config set output_dir ~/Videos
  vget config set twitter.auth_token YOUR_TOKEN`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		cfg := config.LoadOrDefault()

		if err := setConfigValue(cfg, key, value); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Set %s = %s\n", key, value)
	},
}

// vget config get KEY - get a config value
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a configuration value from config.yml.

Examples:
  vget config get language
  vget config get proxy
  vget config get twitter.auth_token`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		cfg := config.LoadOrDefault()

		value, err := getConfigValue(cfg, key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(value)
	},
}

// setConfigValue sets a config value by key
func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "language":
		cfg.Language = value
	case "proxy":
		cfg.Proxy = value
	case "output_dir":
		cfg.OutputDir = value
	case "format":
		cfg.Format = value
	case "quality":
		cfg.Quality = value
	case "filename_template":
		cfg.FilenameTemplate = value
	case "twitter.auth_token":
		cfg.Twitter.AuthToken = value
	case "server.port":
		var port int
		if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
			return fmt.Errorf("invalid port number: %s", value)
		}
		cfg.Server.Port = port
	case "server.max_concurrent":
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid number: %s", value)
		}
		cfg.Server.MaxConcurrent = n
	case "server.api_key":
		cfg.Server.APIKey = value
	default:
		return fmt.Errorf("unknown config key: %s\nRun 'vget config set --help' to see supported keys", key)
	}
	return nil
}

// getConfigValue gets a config value by key
func getConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case "language":
		return cfg.Language, nil
	case "proxy":
		return cfg.Proxy, nil
	case "output_dir":
		return cfg.OutputDir, nil
	case "format":
		return cfg.Format, nil
	case "quality":
		return cfg.Quality, nil
	case "filename_template":
		return cfg.FilenameTemplate, nil
	case "twitter.auth_token":
		return cfg.Twitter.AuthToken, nil
	case "server.port":
		return fmt.Sprintf("%d", cfg.Server.Port), nil
	case "server.max_concurrent":
		return fmt.Sprintf("%d", cfg.Server.MaxConcurrent), nil
	case "server.api_key":
		return cfg.Server.APIKey, nil
	default:
		return "", fmt.Errorf("unknown config key: %s\nRun 'vget config get --help' to see supported keys", key)
	}
}

// --- WebDAV remote management ---

var configWebdavCmd = &cobra.Command{
	Use:     "webdav",
	Short:   "Manage WebDAV remotes",
	Aliases: []string{"remote"},
}

var configWebdavListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List configured WebDAV servers",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()
		if len(cfg.WebDAVServers) == 0 {
			fmt.Println("No WebDAV servers configured.")
			fmt.Println("Add one with: vget config webdav add <name>")
			return
		}

		fmt.Println("WebDAV servers:")
		for name, server := range cfg.WebDAVServers {
			if server.Username != "" {
				fmt.Printf("  %s: %s (user: %s)\n", name, server.URL, server.Username)
			} else {
				fmt.Printf("  %s: %s\n", name, server.URL)
			}
		}
	},
}

var configWebdavAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new WebDAV server",
	Long: `Add a new WebDAV server configuration.

Examples:
  vget config webdav add pikpak
  vget config webdav add nextcloud

After adding, download files like:
  vget pikpak:/Movies/video.mp4`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := config.LoadOrDefault()

		if cfg.GetWebDAVServer(name) != nil {
			fmt.Fprintf(os.Stderr, "WebDAV server '%s' already exists.\n", name)
			fmt.Fprintf(os.Stderr, "Delete it first: vget config webdav delete %s\n", name)
			os.Exit(1)
		}

		reader := bufio.NewReader(os.Stdin)

		// Get URL
		fmt.Print("WebDAV URL: ")
		urlStr, _ := reader.ReadString('\n')
		urlStr = strings.TrimSpace(urlStr)
		if urlStr == "" {
			fmt.Fprintln(os.Stderr, "URL is required")
			os.Exit(1)
		}

		// Get username
		fmt.Print("Username (enter to skip): ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)

		// Get password
		var password string
		if username != "" {
			fmt.Print("Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
				os.Exit(1)
			}
			password = string(passwordBytes)
		}

		cfg.SetWebDAVServer(name, config.WebDAVServer{
			URL:      urlStr,
			Username: username,
			Password: password,
		})

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nWebDAV server '%s' added.\n", name)
		fmt.Printf("Usage: vget %s:/path/to/file.mp4\n", name)
	},
}

var configWebdavDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Short:   "Delete a WebDAV server",
	Aliases: []string{"rm", "remove"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := config.LoadOrDefault()

		if cfg.GetWebDAVServer(name) == nil {
			fmt.Fprintf(os.Stderr, "WebDAV server '%s' not found.\n", name)
			os.Exit(1)
		}

		cfg.DeleteWebDAVServer(name)

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("WebDAV server '%s' deleted.\n", name)
	},
}

var configWebdavShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a WebDAV server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := config.LoadOrDefault()

		server := cfg.GetWebDAVServer(name)
		if server == nil {
			fmt.Fprintf(os.Stderr, "WebDAV server '%s' not found.\n", name)
			os.Exit(1)
		}

		fmt.Printf("Name:     %s\n", name)
		fmt.Printf("URL:      %s\n", server.URL)
		if server.Username != "" {
			fmt.Printf("Username: %s\n", server.Username)
			fmt.Printf("Password: %s\n", strings.Repeat("*", len(server.Password)))
		}
	},
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// --- Twitter auth management ---

var configTwitterCmd = &cobra.Command{
	Use:   "twitter",
	Short: "Manage Twitter/X authentication",
}

var configTwitterSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Twitter auth token for NSFW content",
	Long: `Set Twitter authentication token to download age-restricted content.

To get your auth_token:
  1. Open x.com in your browser and log in
  2. Open DevTools (F12) → Application → Cookies → x.com
  3. Find 'auth_token' and copy its value

Example:
  vget config twitter set
  vget config twitter set --token YOUR_AUTH_TOKEN`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()
		t := i18n.T(cfg.Language)

		token, _ := cmd.Flags().GetString("token")
		if token == "" {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("%s: ", t.Twitter.EnterAuthToken)
			input, _ := reader.ReadString('\n')
			token = strings.TrimSpace(input)
		}

		if token == "" {
			fmt.Fprintln(os.Stderr, t.Twitter.AuthRequired)
			os.Exit(1)
		}

		cfg.Twitter.AuthToken = token

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(t.Twitter.AuthSaved)
		fmt.Println(t.Twitter.AuthCanDownload)
	},
}

var configTwitterClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove Twitter authentication",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()
		t := i18n.T(cfg.Language)
		cfg.Twitter.AuthToken = ""

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(t.Twitter.AuthCleared)
	},
}

// --- Sites management ---

var configSitesCmd = &cobra.Command{
	Use:   "sites",
	Short: "Configure sites.yml for browser-based extraction",
	Long: `Add a site that requires browser-based extraction.

sites.yml is saved in the current directory and configures which
sites should use browser automation to discover m3u8 URLs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSitesWizard()
	},
}

// Sites wizard TUI styles
var (
	sitesFocusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	sitesBlurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sitesHelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sitesContainerStyle = lipgloss.NewStyle().Padding(1, 2)
)

type sitesModel struct {
	step      int // 0: input domain, 1: select type
	textInput textinput.Model
	cursor    int
	types     []string
	done      bool
	cancelled bool
	lang      string
}

func initialSitesModel(lang string) sitesModel {
	ti := textinput.New()
	ti.Placeholder = "kanav.ad"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40
	ti.PromptStyle = sitesFocusedStyle
	ti.TextStyle = sitesFocusedStyle
	ti.Cursor.Style = sitesFocusedStyle

	return sitesModel{
		step:      0,
		textInput: ti,
		types:     []string{"m3u8"},
		lang:      lang,
	}
}

func (m sitesModel) t() *i18n.Translations {
	return i18n.T(m.lang)
}

func (m sitesModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m sitesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if m.step == 0 {
				if strings.TrimSpace(m.textInput.Value()) == "" {
					return m, nil
				}
				m.step = 1
				m.textInput.Blur()
			} else {
				m.done = true
				return m, tea.Quit
			}
			return m, nil

		case "up":
			if m.step == 1 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down":
			if m.step == 1 && m.cursor < len(m.types)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	if m.step == 0 {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m sitesModel) View() string {
	var b strings.Builder
	t := m.t()

	if m.step == 0 {
		b.WriteString(sitesFocusedStyle.Render(t.Sites.DomainMatch))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
	} else {
		b.WriteString(fmt.Sprintf("Domain: %s\n\n", sitesFocusedStyle.Render(m.textInput.Value())))
		b.WriteString(sitesFocusedStyle.Render(t.Sites.SelectType))
		b.WriteString(" ")
		b.WriteString(sitesHelpStyle.Render(t.Sites.OnlyM3u8ForNow))
		b.WriteString("\n\n")
		for i, tp := range m.types {
			cursor := "  "
			style := sitesBlurredStyle
			if i == m.cursor {
				cursor = sitesFocusedStyle.Render("> ")
				style = sitesFocusedStyle
			}
			b.WriteString(cursor)
			b.WriteString(style.Render(tp))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(sitesHelpStyle.Render(t.Sites.EnterConfirm + " • " + t.Sites.EscCancel))

	return sitesContainerStyle.Render(b.String())
}

func runSitesWizard() error {
	// Load user config for language
	userCfg := config.LoadOrDefault()
	t := i18n.T(userCfg.Language)

	// Load existing sites config
	cfg, err := config.LoadSites()
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &config.SitesConfig{}
	}

	// Show existing sites
	if len(cfg.Sites) > 0 {
		fmt.Println(t.Sites.ExistingSites)
		for _, site := range cfg.Sites {
			fmt.Printf("  %s → %s\n", site.Match, site.Type)
		}
		fmt.Println()
	}

	// Run TUI
	p := tea.NewProgram(initialSitesModel(userCfg.Language))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m := finalModel.(sitesModel)
	if m.cancelled {
		fmt.Println(t.Sites.Cancelled)
		return nil
	}

	match := strings.TrimSpace(m.textInput.Value())
	mediaType := m.types[m.cursor]

	// Check for duplicate
	for _, site := range cfg.Sites {
		if site.Match == match {
			return fmt.Errorf("site '%s' already exists", match)
		}
	}

	cfg.AddSite(match, mediaType)

	if err := config.SaveSites(cfg); err != nil {
		return err
	}

	fmt.Printf("\n✓ %s: %s (type: %s)\n", t.Sites.SiteAdded, match, mediaType)
	fmt.Println(t.Sites.SavedTo)
	return nil
}

func init() {
	// config subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)

	// config webdav subcommands
	configWebdavCmd.AddCommand(configWebdavListCmd)
	configWebdavCmd.AddCommand(configWebdavAddCmd)
	configWebdavCmd.AddCommand(configWebdavDeleteCmd)
	configWebdavCmd.AddCommand(configWebdavShowCmd)
	configCmd.AddCommand(configWebdavCmd)

	// config twitter subcommands
	configTwitterSetCmd.Flags().String("token", "", "auth_token value")
	configTwitterCmd.AddCommand(configTwitterSetCmd)
	configTwitterCmd.AddCommand(configTwitterClearCmd)
	configCmd.AddCommand(configTwitterCmd)

	// config sites (single TUI command)
	configCmd.AddCommand(configSitesCmd)

	rootCmd.AddCommand(configCmd)
}
