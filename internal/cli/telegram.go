package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/session/tdesktop"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	tgpkg "github.com/guiyumin/vget/internal/extractor/telegram"
	"github.com/spf13/cobra"
)

var telegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Manage Telegram authentication",
	Long:  "Login, logout, and check status of Telegram session for downloading media",
}

var telegramLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Telegram",
	Long: `Login to Telegram to enable media downloads.

Available methods:
  --import-desktop    Import session from Telegram Desktop app

Example:
  vget telegram login --import-desktop`,
	RunE: runTelegramLogin,
}

var telegramLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear Telegram session",
	Long:  "Remove the stored Telegram session. You'll need to login again to download Telegram media.",
	RunE:  runTelegramLogout,
}

var telegramStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Telegram login status",
	Long:  "Check if you're currently logged in to Telegram and show account info.",
	RunE:  runTelegramStatus,
}

func runTelegramLogin(cmd *cobra.Command, args []string) error {
	importDesktop, _ := cmd.Flags().GetBool("import-desktop")

	if !importDesktop {
		// No method specified, show help
		fmt.Println("Please specify a login method:")
		fmt.Println()
		fmt.Println("  vget telegram login --import-desktop")
		fmt.Println("      Import session from Telegram Desktop app")
		fmt.Println("      Requires: Telegram Desktop installed and logged in")
		fmt.Println()
		return nil
	}

	// Find Telegram Desktop tdata directory
	tdataPath := getTelegramDesktopPath()
	if tdataPath == "" {
		return fmt.Errorf("could not find Telegram Desktop data directory.\n" +
			"Make sure Telegram Desktop is installed and you're logged in")
	}

	fmt.Printf("Found Telegram Desktop at: %s\n", tdataPath)

	// Check if Desktop is running (warn user)
	fmt.Println("Note: Close Telegram Desktop before importing for best results.")
	fmt.Println()

	// Read accounts from tdata
	accounts, err := tdesktop.Read(tdataPath, nil)
	if err != nil {
		return fmt.Errorf("failed to read Telegram Desktop data: %w\n"+
			"Make sure Telegram Desktop is closed and you're logged in", err)
	}

	if len(accounts) == 0 {
		return fmt.Errorf("no accounts found in Telegram Desktop.\n" +
			"Make sure you're logged in to Telegram Desktop")
	}

	// Select account
	var account tdesktop.Account
	if len(accounts) == 1 {
		account = accounts[0]
		fmt.Printf("Found account: ID %d\n", account.Authorization.UserID)
	} else {
		// Multiple accounts - fetch user info for each
		fmt.Printf("Found %d accounts, fetching info...\n\n", len(accounts))

		type accountInfo struct {
			account  tdesktop.Account
			name     string
			username string
		}
		infos := make([]accountInfo, len(accounts))

		for i, acc := range accounts {
			infos[i].account = acc
			name, username := getAccountInfo(acc)
			infos[i].name = name
			infos[i].username = username
		}

		for i, info := range infos {
			if info.username != "" {
				fmt.Printf("  [%d] %s (@%s)\n", i+1, info.name, info.username)
			} else if info.name != "" {
				fmt.Printf("  [%d] %s\n", i+1, info.name)
			} else {
				fmt.Printf("  [%d] ID %d\n", i+1, info.account.Authorization.UserID)
			}
		}
		fmt.Println()
		fmt.Print("Select account: ")

		var choice int
		_, err := fmt.Scanln(&choice)
		if err != nil || choice < 1 || choice > len(accounts) {
			return fmt.Errorf("invalid selection")
		}
		account = accounts[choice-1]
	}

	// Convert tdesktop session to gotd session format
	sessionData, err := session.TDesktopSession(account)
	if err != nil {
		return fmt.Errorf("failed to convert session: %w", err)
	}

	// Create session directory
	if err := os.MkdirAll(tgpkg.SessionPath(), 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Save session to file first, then use FileStorage
	storage := &session.FileStorage{Path: tgpkg.SessionFile()}
	loader := session.Loader{Storage: storage}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := loader.Save(ctx, sessionData); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Create client with file-based storage
	client := telegram.NewClient(
		tgpkg.DesktopAppID,
		tgpkg.DesktopAppHash,
		telegram.Options{
			SessionStorage: storage,
		},
	)

	var userInfo string
	err = client.Run(ctx, func(ctx context.Context) error {
		// Verify the session works
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to check auth status: %w", err)
		}

		if !status.Authorized {
			return fmt.Errorf("session import failed - not authorized")
		}

		// Get user info
		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("failed to get user info: %w", err)
		}

		userInfo = formatUserInfo(self)
		return nil
	})

	if err != nil {
		// Clean up failed session file
		os.Remove(tgpkg.SessionFile())
		return err
	}

	fmt.Println()
	fmt.Println("Successfully logged in!")
	fmt.Println(userInfo)
	fmt.Println()
	fmt.Println("You can now download Telegram media:")
	fmt.Println("  vget https://t.me/channel/123")

	return nil
}

func runTelegramLogout(cmd *cobra.Command, args []string) error {
	sessionFile := tgpkg.SessionFile()

	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		fmt.Println("Not logged in to Telegram.")
		return nil
	}

	if err := os.Remove(sessionFile); err != nil {
		return fmt.Errorf("failed to remove session: %w", err)
	}

	fmt.Println("Logged out from Telegram.")
	return nil
}

func runTelegramStatus(cmd *cobra.Command, args []string) error {
	if !tgpkg.SessionExists() {
		fmt.Println("Not logged in to Telegram.")
		fmt.Println("Run 'vget telegram login' to import your Telegram Desktop session.")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	storage := &session.FileStorage{Path: tgpkg.SessionFile()}

	client := telegram.NewClient(
		tgpkg.DesktopAppID,
		tgpkg.DesktopAppHash,
		telegram.Options{
			SessionStorage: storage,
		},
	)

	var userInfo string
	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to check auth status: %w", err)
		}

		if !status.Authorized {
			return fmt.Errorf("session expired or invalid")
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("failed to get user info: %w", err)
		}

		userInfo = formatUserInfo(self)
		return nil
	})

	if err != nil {
		fmt.Println("Session invalid or expired.")
		fmt.Println("Run 'vget telegram login' to re-import your session.")
		return nil
	}

	fmt.Println("Logged in to Telegram")
	fmt.Println(userInfo)
	return nil
}

func formatUserInfo(self *tg.User) string {
	name := self.FirstName
	if self.LastName != "" {
		name += " " + self.LastName
	}

	info := fmt.Sprintf("  Name: %s", name)
	if self.Username != "" {
		info += fmt.Sprintf("\n  Username: @%s", self.Username)
	}
	info += fmt.Sprintf("\n  ID: %d", self.ID)

	return info
}

func getTelegramDesktopPath() string {
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		paths = []string{
			filepath.Join(home, "Library", "Application Support", "Telegram Desktop", "tdata"),
		}
	case "linux":
		home, _ := os.UserHomeDir()
		paths = []string{
			filepath.Join(home, ".local", "share", "TelegramDesktop", "tdata"),
			// Flatpak
			filepath.Join(home, ".var", "app", "org.telegram.desktop", "data", "TelegramDesktop", "tdata"),
			// Snap
			filepath.Join(home, "snap", "telegram-desktop", "current", ".local", "share", "TelegramDesktop", "tdata"),
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		paths = []string{
			filepath.Join(appData, "Telegram Desktop", "tdata"),
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// getAccountInfo fetches name and username for a tdesktop account
func getAccountInfo(acc tdesktop.Account) (name, username string) {
	sessionData, err := session.TDesktopSession(acc)
	if err != nil {
		return "", ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storage := &session.StorageMemory{}
	loader := session.Loader{Storage: storage}
	if err := loader.Save(ctx, sessionData); err != nil {
		return "", ""
	}

	client := telegram.NewClient(
		tgpkg.DesktopAppID,
		tgpkg.DesktopAppHash,
		telegram.Options{
			SessionStorage: storage,
		},
	)

	_ = client.Run(ctx, func(ctx context.Context) error {
		self, err := client.Self(ctx)
		if err != nil {
			return err
		}
		name = self.FirstName
		if self.LastName != "" {
			name += " " + self.LastName
		}
		username = self.Username
		return nil
	})

	return name, username
}

func init() {
	telegramLoginCmd.Flags().Bool("import-desktop", false, "Import session from Telegram Desktop")
	telegramCmd.AddCommand(telegramLoginCmd)
	telegramCmd.AddCommand(telegramLogoutCmd)
	telegramCmd.AddCommand(telegramStatusCmd)
	rootCmd.AddCommand(telegramCmd)
}
