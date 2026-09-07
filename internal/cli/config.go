package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/tailcat"
	"github.com/guiyumin/vget/internal/core/webdav"
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
				if server.Tailcat != "" {
					fmt.Printf("    Tailcat:  %s\n", tailcat.Mask(server.Tailcat))
				} else {
					fmt.Printf("    URL:      %s\n", server.URL)
				}
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

		if s := cfg.Serve; s.Dir != "" || s.User != "" || s.Listen != "" || s.Inbox != "" || s.DERP.Host != "" || s.RW || s.Tailcat != nil || len(s.Allow) > 0 {
			fmt.Println("\nServe (vget serve):")
			if s.Dir != "" {
				fmt.Printf("  dir:      %s\n", s.Dir)
			}
			if s.User != "" {
				fmt.Printf("  user:     %s\n", s.User)
				fmt.Printf("  password: %s\n", s.Password)
			}
			fmt.Printf("  tailcat:  %v\n", s.TailcatEnabled())
			if s.Listen != "" {
				fmt.Printf("  listen:   %s\n", s.Listen)
			}
			if s.RW {
				fmt.Printf("  rw:       true\n")
			}
			if s.Inbox != "" {
				fmt.Printf("  inbox:    %s\n", s.Inbox)
			}
			if s.DERP.Host != "" {
				fmt.Printf("  derp:     %s\n", describeDERP(s.DERP))
			}
			if len(s.Allow) > 0 {
				fmt.Printf("  allow:    %s\n", strings.Join(s.Allow, ", "))
			}
		}
	},
}

// describeDERP renders a self-hosted relay setting.
func describeDERP(d config.DERPConfig) string {
	s := d.Host
	if d.Port != 0 {
		s = fmt.Sprintf("%s:%d", d.Host, d.Port)
	}
	if d.STUNPort != 0 {
		s += fmt.Sprintf(" (stun %d)", d.STUNPort)
	}
	if d.CertSHA256 != "" {
		s += " (self-signed cert " + tailcat.Mask(d.CertSHA256) + ")"
	}
	return s
}

// remoteTarget renders where a configured server is reached, masking
// tailcat addresses since they are secrets.
func remoteTarget(server config.WebDAVServer) string {
	if server.Tailcat != "" {
		return "tailcat " + tailcat.Mask(server.Tailcat)
	}
	return server.URL
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
  server.port        Server listen port
  server.max_concurrent  Max concurrent downloads
  server.api_key     Server API key
  serve.dir          Library directory for 'vget serve'
  serve.user         Account name clients of 'vget serve' must present
  serve.password     Account password (re-read on every request)
  serve.tailcat      Open a tailcat address (true/false, default true)
  serve.listen       Extra plain-HTTP listen address, e.g. :7777
  serve.rw           Let clients write to the library (true/false)
  serve.inbox        Write-only upload directory exposed at /inbox
  serve.derp.host    Self-hosted relay host or IP
  serve.derp.port    Self-hosted relay port (default 443)
  serve.derp.stun_port    Self-hosted relay STUN port (default 3478)
  serve.derp.cert_sha256  Relay certificate fingerprint (self-signed)
  serve.allow        Allowed client keys, comma-separated (see 'vget identity')

Examples:
  vget config set language en
  vget config set output_dir ~/Videos
  vget config set twitter.auth_token YOUR_TOKEN
  vget config set serve.user alice
  vget config set serve.derp.host 1.2.3.4`,
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
  server.port        Reset to 0 (uses default)
  server.max_concurrent  Reset to 0 (uses default)
  server.api_key     Clear API key

Examples:
  vget config unset twitter.auth_token`,
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
	case "serve.dir":
		cfg.Serve.Dir = value
	case "serve.user":
		cfg.Serve.User = value
	case "serve.password":
		cfg.Serve.Password = value
	case "serve.tailcat":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("serve.tailcat must be true or false, got %q", value)
		}
		cfg.Serve.Tailcat = &b
	case "serve.listen":
		cfg.Serve.Listen = value
	case "serve.rw":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("serve.rw must be true or false, got %q", value)
		}
		cfg.Serve.RW = b
	case "serve.inbox":
		cfg.Serve.Inbox = value
	case "serve.derp.host":
		host, port, err := tailcat.ParseHostPort(value)
		if err != nil {
			return err
		}
		cfg.Serve.DERP.Host = host
		if port != 0 {
			cfg.Serve.DERP.Port = port
		}
	case "serve.derp.port":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 65535 {
			return fmt.Errorf("invalid port number: %s", value)
		}
		cfg.Serve.DERP.Port = n
	case "serve.derp.stun_port":
		n, err := strconv.Atoi(value)
		if err != nil || n < -1 || n > 65535 {
			return fmt.Errorf("invalid port number: %s", value)
		}
		cfg.Serve.DERP.STUNPort = n
	case "serve.derp.cert_sha256":
		fp := tailcat.NormalizeFingerprint(value)
		if len(fp) != 64 {
			return fmt.Errorf("serve.derp.cert_sha256 must be 64 hex characters (see 'vget derp test')")
		}
		cfg.Serve.DERP.CertSHA256 = fp
	case "serve.allow":
		var keys []string
		for _, k := range strings.Split(value, ",") {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
			}
		}
		if _, err := tailcat.ParseAllowedKeys(keys); err != nil {
			return err
		}
		cfg.Serve.Allow = keys
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
	case "output_dir":
		return cfg.OutputDir, nil
	case "format":
		return cfg.Format, nil
	case "quality":
		return cfg.Quality, nil
	case "twitter.auth_token":
		return cfg.Twitter.AuthToken, nil
	case "server.port":
		return fmt.Sprintf("%d", cfg.Server.Port), nil
	case "server.max_concurrent":
		return fmt.Sprintf("%d", cfg.Server.MaxConcurrent), nil
	case "server.api_key":
		return cfg.Server.APIKey, nil
	case "serve.dir":
		return cfg.Serve.Dir, nil
	case "serve.user":
		return cfg.Serve.User, nil
	case "serve.password":
		return cfg.Serve.Password, nil
	case "serve.tailcat":
		return strconv.FormatBool(cfg.Serve.TailcatEnabled()), nil
	case "serve.listen":
		return cfg.Serve.Listen, nil
	case "serve.rw":
		return strconv.FormatBool(cfg.Serve.RW), nil
	case "serve.inbox":
		return cfg.Serve.Inbox, nil
	case "serve.derp.host":
		return cfg.Serve.DERP.Host, nil
	case "serve.derp.port":
		return strconv.Itoa(cfg.Serve.DERP.Port), nil
	case "serve.derp.stun_port":
		return strconv.Itoa(cfg.Serve.DERP.STUNPort), nil
	case "serve.derp.cert_sha256":
		return cfg.Serve.DERP.CertSHA256, nil
	case "serve.allow":
		return strings.Join(cfg.Serve.Allow, ","), nil
	default:
		return "", fmt.Errorf("unknown config key: %s\nRun 'vget config get --help' to see supported keys", key)
	}
}

// unsetConfigValue clears a config value by key
func unsetConfigValue(cfg *config.Config, key string) error {
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
	case "server.port":
		cfg.Server.Port = 0
	case "server.max_concurrent":
		cfg.Server.MaxConcurrent = 0
	case "server.api_key":
		cfg.Server.APIKey = ""
	case "serve.dir":
		cfg.Serve.Dir = ""
	case "serve.user":
		cfg.Serve.User = ""
	case "serve.password":
		cfg.Serve.Password = ""
	case "serve.tailcat":
		cfg.Serve.Tailcat = nil
	case "serve.listen":
		cfg.Serve.Listen = ""
	case "serve.rw":
		cfg.Serve.RW = false
	case "serve.inbox":
		cfg.Serve.Inbox = ""
	case "serve.derp.host", "serve.derp":
		cfg.Serve.DERP = config.DERPConfig{}
	case "serve.derp.port":
		cfg.Serve.DERP.Port = 0
	case "serve.derp.stun_port":
		cfg.Serve.DERP.STUNPort = 0
	case "serve.derp.cert_sha256":
		cfg.Serve.DERP.CertSHA256 = ""
	case "serve.allow":
		cfg.Serve.Allow = nil
	default:
		return fmt.Errorf("unknown config key: %s\nRun 'vget config unset --help' to see supported keys", key)
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
				fmt.Printf("  %s: %s (user: %s)\n", name, remoteTarget(server), server.Username)
			} else {
				fmt.Printf("  %s: %s\n", name, remoteTarget(server))
			}
		}
	},
}

var (
	webdavAddUser string
	webdavAddPass string
)

var configWebdavAddCmd = &cobra.Command{
	Use:   "add <name> [tc-address | url]",
	Short: "Add a WebDAV server or a library shared with 'vget serve'",
	Long: `Add a WebDAV server configuration. The target is either a WebDAV URL or
the tailcat address printed by 'vget serve' on another machine (no public IP
needed). Anything not given on the command line is asked for interactively.

Examples:
  vget config webdav add pikpak
  vget config webdav add nas https://nas.example.com/dav --user me --pass secret
  vget config webdav add alice tcomFwWC... --user friend --pass p1

After adding, browse and download like:
  vget ls alice:/
  vget alice:/Movies/video.mp4`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := config.LoadOrDefault()

		if cfg.GetWebDAVServer(name) != nil {
			fmt.Fprintf(os.Stderr, "WebDAV server '%s' already exists.\n", name)
			fmt.Fprintf(os.Stderr, "Delete it first: vget config webdav delete %s\n", name)
			os.Exit(1)
		}

		reader := bufio.NewReader(os.Stdin)

		// Get the target: a URL or a tailcat address
		var target string
		if len(args) == 2 {
			target = strings.TrimSpace(args[1])
		} else {
			fmt.Print("WebDAV URL or tailcat address: ")
			line, _ := reader.ReadString('\n')
			target = strings.TrimSpace(line)
		}
		if target == "" {
			fmt.Fprintln(os.Stderr, "A URL or tailcat address is required")
			os.Exit(1)
		}
		server := config.WebDAVServer{}
		switch {
		case tailcat.IsAddr(target):
			server.Tailcat = target
		case strings.HasPrefix(target, "tc") && !strings.Contains(target, "://"):
			fmt.Fprintln(os.Stderr, tailcat.Validate(target))
			os.Exit(1)
		default:
			server.URL = target
		}

		// Get username
		username := webdavAddUser
		if !cmd.Flags().Changed("user") {
			fmt.Print("Username (enter to skip): ")
			line, _ := reader.ReadString('\n')
			username = strings.TrimSpace(line)
		}

		// Get password
		password := webdavAddPass
		if username != "" && !cmd.Flags().Changed("pass") {
			fmt.Print("Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
				os.Exit(1)
			}
			password = string(passwordBytes)
		}
		server.Username = username
		server.Password = password

		cfg.SetWebDAVServer(name, server)

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save: %v\n", err)
			os.Exit(1)
		}

		if server.Tailcat != "" {
			fmt.Printf("\nLibrary '%s' added (tailcat, relay %s).\n", name, tailcat.RelayOf(server.Tailcat))
			fmt.Printf("Usage: vget ls %s:/\n", name)
			fmt.Printf("       vget config webdav ping %s\n", name)
		} else {
			fmt.Printf("\nWebDAV server '%s' added.\n", name)
			fmt.Printf("Usage: vget %s:/path/to/file.mp4\n", name)
		}
	},
}

var (
	webdavPingUntilDirect bool
	webdavPingTimeout     time.Duration
	webdavShowFull        bool
)

var configWebdavPingCmd = &cobra.Command{
	Use:   "ping <name | tc-address>",
	Short: "Check the connection to a library: relay round trip and whether it is direct",
	Long: `Reach a library and report how. For a tailcat library each pong says
whether it came back over a direct path (NAT traversal worked) or through the
relay. --until-direct keeps pinging until a direct path works, bounded by
--timeout, and exits non-zero otherwise.

Examples:
  vget config webdav ping alice
  vget config webdav ping alice --until-direct --timeout 30s
  vget config webdav ping tcomFwWC...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault()
		t := i18n.T(cfg.Language)
		target := args[0]

		addr := ""
		if tailcat.IsAddr(target) {
			addr = target
		} else {
			server := cfg.GetWebDAVServer(target)
			if server == nil {
				return fmt.Errorf("WebDAV server '%s' not found", target)
			}
			if server.Tailcat == "" {
				// A plain URL: just time a request.
				client, err := webdav.NewClientFromConfig(server)
				if err != nil {
					return err
				}
				t0 := time.Now()
				if _, err := client.Stat(context.Background(), "/"); err != nil {
					return err
				}
				fmt.Printf("ok in %s (http, %s)\n", time.Since(t0).Round(time.Millisecond), server.URL)
				return nil
			}
			addr = server.Tailcat
		}

		timeout := webdavPingTimeout
		if webdavPingUntilDirect && !cmd.Flags().Changed("timeout") {
			timeout = 30 * time.Second
		}
		defer closeTunnels()
		first := true
		res, err := tailcat.Ping(context.Background(), addr, webdavPingUntilDirect, timeout, func(r tailcat.PingResult) {
			if first {
				fmt.Printf("%s: %s (%s)\n", t.Tailcat.PingRelayRTT, r.RelayRTT.Round(time.Millisecond), r.Relay)
				first = false
			}
			if r.Direct {
				fmt.Printf("pong in %s %s via %s\n", r.Latency, t.Tailcat.PingDirect, r.Endpoint)
			} else {
				fmt.Printf("pong in %s %s %s\n", r.Latency, t.Tailcat.PingViaRelay, r.Relay)
			}
		})
		if errors.Is(err, tailcat.ErrNoDirectPath) {
			fmt.Fprintln(os.Stderr, fmt.Sprintf(t.Tailcat.PingNoDirect, timeout))
			os.Exit(1)
		}
		if err != nil {
			return err
		}
		_ = res
		return nil
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
		if server.Tailcat != "" {
			fmt.Printf("Transport: tailcat\n")
			if webdavShowFull {
				fmt.Printf("Address:  %s\n", server.Tailcat)
			} else {
				fmt.Printf("Address:  %s  (--full to show)\n", tailcat.Mask(server.Tailcat))
			}
			fmt.Printf("Relay:    %s\n", tailcat.RelayOf(server.Tailcat))
		} else {
			fmt.Printf("URL:      %s\n", server.URL)
		}
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

func init() {
	// config subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configUnsetCmd)

	// config webdav subcommands
	configWebdavAddCmd.Flags().StringVar(&webdavAddUser, "user", "", "account name (skips the prompt)")
	configWebdavAddCmd.Flags().StringVar(&webdavAddPass, "pass", "", "account password (skips the prompt)")
	configWebdavShowCmd.Flags().BoolVar(&webdavShowFull, "full", false, "show the full tailcat address")
	configWebdavPingCmd.Flags().BoolVar(&webdavPingUntilDirect, "until-direct", false, "keep pinging until a direct path works")
	configWebdavPingCmd.Flags().DurationVar(&webdavPingTimeout, "timeout", 10*time.Second, "give up after this long")
	configWebdavCmd.AddCommand(configWebdavListCmd)
	configWebdavCmd.AddCommand(configWebdavAddCmd)
	configWebdavCmd.AddCommand(configWebdavDeleteCmd)
	configWebdavCmd.AddCommand(configWebdavShowCmd)
	configWebdavCmd.AddCommand(configWebdavPingCmd)
	configCmd.AddCommand(configWebdavCmd)

	// config twitter subcommands
	configTwitterSetCmd.Flags().String("token", "", "auth_token value")
	configTwitterCmd.AddCommand(configTwitterSetCmd)
	configTwitterCmd.AddCommand(configTwitterClearCmd)
	configCmd.AddCommand(configTwitterCmd)

	rootCmd.AddCommand(configCmd)
}