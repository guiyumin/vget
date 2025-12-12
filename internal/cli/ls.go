package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/webdav"
	"github.com/spf13/cobra"
)

var jsonFlag bool

var lsCmd = &cobra.Command{
	Use:   "ls <remote>:<path>",
	Short: "List files in a remote directory",
	Long: `List files in a WebDAV remote directory.

Examples:
  vget ls pikpak:/
  vget ls pikpak:/Movies
  vget ls pikpak:/Movies/Action`,
	Args: cobra.ExactArgs(1),
	RunE: runLs,
}

func init() {
	lsCmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	rootCmd.AddCommand(lsCmd)
}

// FileEntry represents a file or directory for JSON output
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

func runLs(cmd *cobra.Command, args []string) error {
	remotePath := args[0]
	ctx := context.Background()
	cfg := config.LoadOrDefault()

	// Check if it's a WebDAV remote path
	if !webdav.IsRemotePath(remotePath) && !webdav.IsWebDAVURL(remotePath) {
		return fmt.Errorf("invalid remote path: %s\nUse format: <remote>:<path> (e.g., pikpak:/Movies)", remotePath)
	}

	var client *webdav.Client
	var dirPath string
	var err error

	if webdav.IsRemotePath(remotePath) {
		// Parse remote:path format
		serverName, path, err := webdav.ParseRemotePath(remotePath)
		if err != nil {
			return err
		}
		dirPath = path
		if dirPath == "" {
			dirPath = "/"
		}

		server := cfg.GetWebDAVServer(serverName)
		if server == nil {
			return fmt.Errorf("WebDAV server '%s' not found. Add it with 'vget config webdav add %s'", serverName, serverName)
		}

		client, err = webdav.NewClientFromConfig(server)
		if err != nil {
			return fmt.Errorf("failed to create WebDAV client: %w", err)
		}
	} else {
		// Full WebDAV URL
		client, err = webdav.NewClient(remotePath)
		if err != nil {
			return fmt.Errorf("failed to create WebDAV client: %w", err)
		}

		dirPath, err = webdav.ParseURL(remotePath)
		if err != nil {
			return fmt.Errorf("invalid WebDAV URL: %w", err)
		}
		if dirPath == "" {
			dirPath = "/"
		}
	}

	// Check if path is a directory
	info, err := client.Stat(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir {
		return fmt.Errorf("'%s' is not a directory", remotePath)
	}

	// List directory contents
	files, err := client.List(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("failed to list directory: %w", err)
	}

	// Sort: directories first, then files, alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir // dirs first
		}
		return files[i].Name < files[j].Name
	})

	// Build remote path prefix for full paths
	remotePrefix := remotePath
	if remotePrefix[len(remotePrefix)-1] != '/' {
		remotePrefix += "/"
	}

	// JSON output
	if jsonFlag {
		entries := make([]FileEntry, len(files))
		for i, f := range files {
			entries[i] = FileEntry{
				Name:  f.Name,
				Path:  remotePrefix + f.Name,
				IsDir: f.IsDir,
				Size:  f.Size,
			}
		}
		output, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Human-readable output
	if len(files) == 0 {
		fmt.Println("(empty directory)")
		return nil
	}

	// Print header
	fmt.Printf("%s\n", remotePath)

	// Print files
	for _, f := range files {
		if f.IsDir {
			fmt.Printf("  📁 %s/\n", f.Name)
		} else {
			fmt.Printf("  📄 %-40s %s\n", f.Name, formatSize(f.Size))
		}
	}

	return nil
}
