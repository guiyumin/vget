package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/webdav"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for vget.

Bash:
  # Add to ~/.bashrc:
  source <(vget completion bash)

  # Or install to system:
  vget completion bash > /etc/bash_completion.d/vget

Zsh:
  # Add to ~/.zshrc:
  source <(vget completion zsh)

  # Or install to fpath:
  vget completion zsh > "${fpath[1]}/_vget"

Fish:
  vget completion fish > ~/.config/fish/completions/vget.fish

PowerShell:
  vget completion powershell >> $PROFILE
`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletion(os.Stdout)
		default:
			return cmd.Help()
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)

	// Enable dynamic completion for root command (for remote paths)
	rootCmd.ValidArgsFunction = completeRemotePath
}

// unescapeShellPath removes common shell escape sequences
func unescapeShellPath(s string) string {
	// Handle common shell escapes
	s = strings.ReplaceAll(s, "\\ ", " ")
	s = strings.ReplaceAll(s, "\\[", "[")
	s = strings.ReplaceAll(s, "\\]", "]")
	s = strings.ReplaceAll(s, "\\(", "(")
	s = strings.ReplaceAll(s, "\\)", ")")
	s = strings.ReplaceAll(s, "\\&", "&")
	s = strings.ReplaceAll(s, "\\'", "'")
	s = strings.ReplaceAll(s, "\\\"", "\"")
	return s
}


// completeRemotePath provides dynamic completion for WebDAV remote paths
func completeRemotePath(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument (the URL)
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// If empty or no colon, suggest configured remotes
	if !strings.Contains(toComplete, ":") {
		return completeRemotes(toComplete)
	}

	// Has colon - complete remote path
	return completeRemoteFiles(toComplete)
}

// completeRemotes returns configured remote names
func completeRemotes(prefix string) ([]string, cobra.ShellCompDirective) {
	cfg := config.LoadOrDefault()
	var completions []string

	for name := range cfg.WebDAVServers {
		remote := name + ":"
		if strings.HasPrefix(remote, prefix) {
			completions = append(completions, remote)
		}
	}

	// Also allow local file completion if no prefix or doesn't match remotes
	if len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	return completions, cobra.ShellCompDirectiveNoSpace
}

// completeRemoteFiles queries WebDAV and returns matching paths
func completeRemoteFiles(toComplete string) ([]string, cobra.ShellCompDirective) {
	// Parse remote:path
	if !webdav.IsRemotePath(toComplete) {
		return nil, cobra.ShellCompDirectiveDefault
	}

	serverName, remotePath, err := webdav.ParseRemotePath(toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	cfg := config.LoadOrDefault()
	server := cfg.GetWebDAVServer(serverName)
	if server == nil {
		return nil, cobra.ShellCompDirectiveError
	}

	client, err := webdav.NewClientFromConfig(server)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Unescape shell escapes in the path for proper comparison
	unescapedPath := unescapeShellPath(remotePath)

	// Determine directory to list and prefix to filter
	dirPath := filepath.Dir(unescapedPath)
	if dirPath == "." {
		dirPath = "/"
	}
	baseName := filepath.Base(unescapedPath)

	ctx := context.Background()

	// Check if the path ends with "/" OR if it's an existing directory
	// This handles the case where zsh strips the trailing slash
	if strings.HasSuffix(toComplete, "/") || strings.HasSuffix(unescapedPath, "/") {
		dirPath = strings.TrimSuffix(unescapedPath, "/")
		if dirPath == "" {
			dirPath = "/"
		}
		baseName = ""
	} else {
		// Check if the path is an existing directory on the remote
		if info, err := client.Stat(ctx, unescapedPath); err == nil && info.IsDir {
			// It's a directory - list its contents
			dirPath = unescapedPath
			baseName = ""
		}
	}
	files, err := client.List(ctx, dirPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	prefix := serverName + ":"
	// Ensure dirPath ends with exactly one slash
	if dirPath != "/" {
		prefix += strings.TrimSuffix(dirPath, "/") + "/"
	} else {
		prefix += "/"
	}

	for _, f := range files {
		if baseName == "" || strings.HasPrefix(f.Name, baseName) {
			// Return the full path for completion
			completion := prefix + f.Name
			if f.IsDir {
				completion += "/"
			}
			completions = append(completions, completion)
		}
	}

	if len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Limit completions to avoid zsh prompt redraw issue with large lists
	// zsh redraws prompt when showing too many completions (threshold varies)
	// Limit to 15 for safe margin; users can type more chars to filter
	// or press Enter to open the TUI browser for full navigation
	const maxCompletions = 15
	if len(completions) > maxCompletions {
		completions = completions[:maxCompletions]
	}

	// Use ShellCompDirectiveNoFileComp to prevent falling back to file completion
	// Use ShellCompDirectiveNoSpace to not add space after directory completions
	return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}
