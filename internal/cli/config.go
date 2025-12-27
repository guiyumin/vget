package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
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
		fmt.Printf("  OutputDir: %s\n", cfg.OutputDir)
		fmt.Printf("  Format:    %s\n", cfg.Format)
		fmt.Printf("  Quality:   %s\n", cfg.Quality)
		fmt.Printf("  Config:    %s\n", config.SavePath())

		if len(cfg.WebDAVServers) > 0 {
			fmt.Println("\nWebDAV servers:")
			for name, server := range cfg.WebDAVServers {
				fmt.Printf("  %s:\n", name)
				fmt.Printf("    URL:      %s\n", server.URL)
				if server.Username != "" {
					fmt.Printf("    Username: %s\n", server.Username)
					fmt.Printf("    Password: %s\n", server.Password)
				}
			}
		}

		if cfg.Twitter.AuthToken != "" {
			fmt.Println("\nTwitter:")
			fmt.Printf("  auth_token: %s\n", cfg.Twitter.AuthToken)
		}

		// Show express tracking providers config
		if len(cfg.Express) > 0 {
			fmt.Println("\nExpress Tracking:")
			for provider, providerCfg := range cfg.Express {
				fmt.Printf("  %s:\n", provider)
				for key, value := range providerCfg {
					fmt.Printf("    %s: %s\n", key, value)
				}
			}
		}

		// Show AI config
		if len(cfg.AI.Accounts) > 0 {
			fmt.Println("\nAI Accounts:")
			for name, account := range cfg.AI.Accounts {
				defaultMarker := ""
				if name == cfg.AI.DefaultAccount {
					defaultMarker = " (default)"
				}
				fmt.Printf("  %s%s:\n", name, defaultMarker)
				fmt.Printf("    Provider: %s\n", account.Provider)
				if account.Transcription.Model != "" {
					fmt.Printf("    Transcription: %s", account.Transcription.Model)
					if account.Transcription.APIKeyEncrypted != "" {
						fmt.Print(" [key encrypted]")
					}
					fmt.Println()
				}
				if account.Summarization.Model != "" {
					fmt.Printf("    Summarization: %s", account.Summarization.Model)
					if account.Summarization.APIKeyEncrypted != "" {
						fmt.Print(" [key encrypted]")
					}
					fmt.Println()
				}
			}
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
  output_dir         Default download directory
  format             Preferred format (mp4, webm, best)
  quality            Default quality (1080p, 720p, best)
  twitter.auth_token Twitter auth token for NSFW content
  bilibili.cookie    Bilibili cookie for member-only content
  server.port        Server listen port
  server.max_concurrent  Max concurrent downloads
  server.api_key     Server API key

AI configuration (use 'vget ai config' for interactive setup):
  ai.default_account          Default account name to use

Express tracking (dynamic keys):
  express.<provider>.<key>  Set express provider config

  Kuaidi100 example:
    express.kuaidi100.key       API authorization key
    express.kuaidi100.customer  Customer ID
    express.kuaidi100.secret    Secret for delivery time API (optional)

Examples:
  vget config set language en
  vget config set output_dir ~/Videos
  vget config set twitter.auth_token YOUR_TOKEN
  vget config set ai.transcription.provider openai
  vget config set ai.transcription.api_key sk-xxx
  vget config set express.kuaidi100.key YOUR_KEY`,
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

// vget config unset KEY - unset/clear a config value
var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Unset a configuration value",
	Long: `Unset (clear) a configuration value in config.yml.

Supported keys:
  language           Reset to empty (uses default)
  output_dir         Reset to empty (uses default)
  format             Reset to empty (uses default)
  quality            Reset to empty (uses default)
  twitter.auth_token Clear Twitter auth token
  bilibili.cookie    Clear Bilibili cookie
  server.port        Reset to 0 (uses default)
  server.max_concurrent  Reset to 0 (uses default)
  server.api_key     Clear API key

Express tracking (dynamic keys):
  express.<provider>.<key>  Clear express provider config value

Examples:
  vget config unset twitter.auth_token
  vget config unset express.kuaidi100.key`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]

		cfg := config.LoadOrDefault()

		if err := unsetConfigValue(cfg, key); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Unset %s\n", key)
	},
}

// setConfigValue sets a config value by key
func setConfigValue(cfg *config.Config, key, value string) error {
	// Handle express.<provider>.<key> pattern (e.g., express.kuaidi100.key)
	if strings.HasPrefix(key, "express.") {
		parts := strings.SplitN(key, ".", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid express config key format: %s\nUse: express.<provider>.<key> (e.g., express.kuaidi100.key)", key)
		}
		provider := parts[1]
		configKey := parts[2]
		cfg.SetExpressConfig(provider, configKey, value)
		return nil
	}

	// Handle ai.* keys
	if strings.HasPrefix(key, "ai.") {
		return setAIConfigValue(cfg, key, value)
	}

	switch key {
	case "language":
		cfg.Language = value
	case "output_dir":
		cfg.OutputDir = value
	case "format":
		cfg.Format = value
	case "quality":
		cfg.Quality = value
	case "twitter.auth_token":
		cfg.Twitter.AuthToken = value
	case "bilibili.cookie":
		cfg.Bilibili.Cookie = value
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

// setAIConfigValue handles ai.* config keys
func setAIConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "ai.default_account":
		// Validate that the account exists
		if cfg.AI.GetAccount(value) == nil && value != "" {
			accounts := cfg.AI.ListAccounts()
			if len(accounts) == 0 {
				return fmt.Errorf("no AI accounts configured. Run: vget ai config")
			}
			return fmt.Errorf("account '%s' not found. Available: %v", value, accounts)
		}
		cfg.AI.DefaultAccount = value
	default:
		return fmt.Errorf("unknown AI config key: %s\nUse 'vget ai config' for interactive AI setup\nSupported: ai.default_account", key)
	}
	return nil
}

// getConfigValue gets a config value by key
func getConfigValue(cfg *config.Config, key string) (string, error) {
	// Handle express.<provider>.<key> pattern (e.g., express.kuaidi100.key)
	if strings.HasPrefix(key, "express.") {
		parts := strings.SplitN(key, ".", 3)
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid express config key format: %s\nUse: express.<provider>.<key> (e.g., express.kuaidi100.key)", key)
		}
		provider := parts[1]
		configKey := parts[2]
		providerCfg := cfg.GetExpressConfig(provider)
		if providerCfg == nil {
			return "", nil
		}
		return providerCfg[configKey], nil
	}

	// Handle ai.* keys
	if strings.HasPrefix(key, "ai.") {
		return getAIConfigValue(cfg, key)
	}

	switch key {
	case "language":
		return cfg.Language, nil
	case "output_dir":
		return cfg.OutputDir, nil
	case "format":
		return cfg.Format, nil
	case "quality":
		return cfg.Quality, nil
	case "twitter.auth_token":
		return cfg.Twitter.AuthToken, nil
	case "bilibili.cookie":
		return cfg.Bilibili.Cookie, nil
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

// getAIConfigValue handles ai.* config keys
func getAIConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case "ai.default_account":
		return cfg.AI.DefaultAccount, nil
	default:
		return "", fmt.Errorf("unknown AI config key: %s\nUse 'vget ai config' for interactive AI setup\nSupported: ai.default_account", key)
	}
}

// unsetConfigValue clears a config value by key
func unsetConfigValue(cfg *config.Config, key string) error {
	// Handle express.<provider>.<key> pattern (e.g., express.kuaidi100.key)
	if strings.HasPrefix(key, "express.") {
		parts := strings.SplitN(key, ".", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid express config key format: %s\nUse: express.<provider>.<key> (e.g., express.kuaidi100.key)", key)
		}
		provider := parts[1]
		configKey := parts[2]
		cfg.DeleteExpressConfig(provider, configKey)
		return nil
	}

	// Handle ai.* keys
	if strings.HasPrefix(key, "ai.") {
		return unsetAIConfigValue(cfg, key)
	}

	switch key {
	case "language":
		cfg.Language = ""
	case "output_dir":
		cfg.OutputDir = ""
	case "format":
		cfg.Format = ""
	case "quality":
		cfg.Quality = ""
	case "twitter.auth_token":
		cfg.Twitter.AuthToken = ""
	case "bilibili.cookie":
		cfg.Bilibili.Cookie = ""
	case "server.port":
		cfg.Server.Port = 0
	case "server.max_concurrent":
		cfg.Server.MaxConcurrent = 0
	case "server.api_key":
		cfg.Server.APIKey = ""
	default:
		return fmt.Errorf("unknown config key: %s\nRun 'vget config unset --help' to see supported keys", key)
	}
	return nil
}

// unsetAIConfigValue handles ai.* config keys
func unsetAIConfigValue(cfg *config.Config, key string) error {
	switch key {
	case "ai.default_account":
		cfg.AI.DefaultAccount = ""
	default:
		return fmt.Errorf("unknown AI config key: %s\nUse 'vget ai config' for interactive AI setup\nSupported: ai.default_account", key)
	}
	return nil
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

// --- Twitter auth management ---

var configTwitterCmd = &cobra.Command{
	Use:        "twitter",
	Short:      "Manage Twitter/X authentication (deprecated)",
	Deprecated: "use 'vget config set twitter.auth_token <value>' instead",
}

var configTwitterSetCmd = &cobra.Command{
	Use:        "set",
	Short:      "Set Twitter auth token (deprecated)",
	Deprecated: "use 'vget config set twitter.auth_token <value>' instead",
	Long: `DEPRECATED: Use 'vget config set twitter.auth_token <value>' instead.

Set Twitter authentication token to download age-restricted content.

To get your auth_token:
  1. Open x.com in your browser and log in
  2. Open DevTools (F12) → Application → Cookies → x.com
  3. Find 'auth_token' and copy its value

New syntax:
  vget config set twitter.auth_token YOUR_AUTH_TOKEN`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()
		t := i18n.T(cfg.Language)

		// Show deprecation warning and exit
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", t.Twitter.DeprecatedSet)
		fmt.Fprintf(os.Stderr, "   %s\n", t.Twitter.DeprecatedUseNew)
		os.Exit(1)
	},
}

var configTwitterClearCmd = &cobra.Command{
	Use:        "clear",
	Short:      "Remove Twitter authentication (deprecated)",
	Deprecated: "use 'vget config unset twitter.auth_token' instead",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()
		t := i18n.T(cfg.Language)

		// Show deprecation warning and exit
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", t.Twitter.DeprecatedClear)
		fmt.Fprintf(os.Stderr, "   %s\n", t.Twitter.DeprecatedUseNewUnset)
		os.Exit(1)
	},
}

// vget config ai - alias for vget ai config
var configAICmd = &cobra.Command{
	Use:   "ai",
	Short: "Configure AI transcription and summarization",
	Long: `Launch interactive TUI wizard to configure AI transcription and summarization providers.

This is an alias for 'vget ai config'.

This wizard will help you set up:
  - Transcription provider (OpenAI Whisper, etc.)
  - Summarization provider (OpenAI GPT, etc.)
  - API keys for each service`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.RunAIWizard()
		if err != nil {
			return err
		}

		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("\nSaved to %s\n", config.SavePath())
		return nil
	},
}

func init() {
	// config subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configUnsetCmd)

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

	// config ai subcommand (alias for vget ai config)
	configCmd.AddCommand(configAICmd)

	rootCmd.AddCommand(configCmd)
}