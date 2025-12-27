package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/guiyumin/vget/internal/core/ai"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/crypto"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI transcription and summarization",
	Long: `AI-powered transcription and summarization for audio/video files.

Configure AI settings:
  vget ai config              Interactive TUI wizard

Process files:
  vget ai <file> --transcribe              Transcribe audio/video to text
  vget ai <file> --transcribe --summarize  Transcribe and summarize

Flags:
  --password <PIN>   4-digit PIN to decrypt API keys (will prompt if not provided)
  --account <name>   Use specific AI account (uses default if not specified)

Examples:
  vget ai config
  vget ai podcast.mp3 --transcribe --password 1234
  vget ai podcast.mp3 --transcribe --summarize --account work
  vget ai transcript.md --summarize`,
}

var aiConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure AI providers",
	Long: `Launch interactive TUI wizard to configure AI transcription and summarization providers.

This wizard will help you set up:
  - Account name (alias for the configuration)
  - AI provider (OpenAI, etc.)
  - API keys for transcription and summarization
  - 4-digit PIN to encrypt your API keys

Your API keys are encrypted with AES-256-GCM and stored securely.
You will need your PIN every time you use AI features.`,
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

var aiAccountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "List configured AI accounts",
	Long:  `Display all configured AI accounts and their settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault()

		accounts := cfg.AI.ListAccounts()
		if len(accounts) == 0 {
			fmt.Println("No AI accounts configured.")
			fmt.Println("Run: vget ai config")
			return nil
		}

		fmt.Println("Configured AI accounts:")
		fmt.Println()

		for _, name := range accounts {
			account := cfg.AI.GetAccount(name)
			if account == nil {
				continue
			}

			defaultMarker := ""
			if name == cfg.AI.DefaultAccount {
				defaultMarker = " (default)"
			}

			fmt.Printf("  %s%s\n", name, defaultMarker)
			fmt.Printf("    Provider: %s\n", account.Provider)
			fmt.Printf("    Transcription model: %s\n", account.Transcription.Model)
			fmt.Printf("    Summarization model: %s\n", account.Summarization.Model)
			fmt.Println()
		}

		return nil
	},
}

// Flags for ai command
var (
	transcribeFlag bool
	summarizeFlag  bool
	passwordFlag   string
	accountFlag    string
)

var aiProcessCmd = &cobra.Command{
	Use:   "[file]",
	Short: "Process audio/video file with AI",
	Long: `Process an audio or video file with AI transcription and/or summarization.

Operations:
  --transcribe    Convert speech to text (requires audio/video input)
  --summarize     Generate summary (requires text input or --transcribe)

Authentication:
  --password <PIN>   4-digit PIN to decrypt API keys (will prompt if not provided)
  --account <name>   Use specific AI account (uses default if not specified)

Examples:
  vget ai podcast.mp3 --transcribe --password 1234
  vget ai podcast.mp3 --transcribe --summarize
  vget ai transcript.md --summarize --account work`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Validate flags
		if !transcribeFlag && !summarizeFlag {
			return fmt.Errorf("at least one operation required: --transcribe or --summarize")
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filePath)
		}

		// Load config
		cfg := config.LoadOrDefault()

		// Validate account exists
		account := cfg.AI.GetAccount(accountFlag)
		if account == nil {
			if accountFlag == "" {
				return fmt.Errorf("no AI accounts configured\nRun: vget ai config")
			}
			return fmt.Errorf("AI account '%s' not found\nRun: vget ai config", accountFlag)
		}

		// Validate required services are configured
		if transcribeFlag && account.Transcription.APIKeyEncrypted == "" {
			return fmt.Errorf("transcription not configured for account '%s'\nRun: vget ai config", accountFlag)
		}

		if summarizeFlag && account.Summarization.APIKeyEncrypted == "" {
			return fmt.Errorf("summarization not configured for account '%s'\nRun: vget ai config", accountFlag)
		}

		// Get PIN (from flag or prompt)
		pin := passwordFlag
		if pin == "" {
			var err error
			pin, err = promptPIN()
			if err != nil {
				return fmt.Errorf("failed to read PIN: %w", err)
			}
		}

		// Validate PIN format
		if err := crypto.ValidatePIN(pin); err != nil {
			return err
		}

		// Create and run AI pipeline
		pipeline, err := ai.NewPipeline(cfg, accountFlag, pin)
		if err != nil {
			return fmt.Errorf("failed to initialize AI pipeline: %w", err)
		}

		ctx := context.Background()
		opts := ai.Options{
			Transcribe: transcribeFlag,
			Summarize:  summarizeFlag,
		}

		_, err = pipeline.Process(ctx, filePath, opts)
		return err
	},
}

// promptPIN prompts the user to enter their 4-digit PIN securely.
func promptPIN() (string, error) {
	fmt.Print("Enter 4-digit PIN: ")

	// Check if stdin is a terminal
	if term.IsTerminal(int(syscall.Stdin)) {
		// Read password without echo
		pinBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Add newline after hidden input
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(pinBytes)), nil
	}

	// Fallback for non-terminal (e.g., piped input)
	reader := bufio.NewReader(os.Stdin)
	pin, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(pin), nil
}

func init() {
	// Add config subcommand
	aiCmd.AddCommand(aiConfigCmd)

	// Add accounts subcommand
	aiCmd.AddCommand(aiAccountsCmd)

	// Add flags to ai command for processing
	aiCmd.Flags().BoolVar(&transcribeFlag, "transcribe", false, "Transcribe audio/video to text")
	aiCmd.Flags().BoolVar(&summarizeFlag, "summarize", false, "Generate summary from text")
	aiCmd.Flags().StringVar(&passwordFlag, "password", "", "4-digit PIN to decrypt API keys")
	aiCmd.Flags().StringVar(&accountFlag, "account", "", "Use specific AI account (default: uses default account)")

	// Set up ai command to handle files directly
	aiCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// No args, show help
			return cmd.Help()
		}

		// If args provided but it's a subcommand, let cobra handle it
		// Otherwise, process as a file
		filePath := args[0]

		// Check if it's a subcommand
		for _, subCmd := range cmd.Commands() {
			if subCmd.Name() == filePath || contains(subCmd.Aliases, filePath) {
				return nil // Let cobra handle subcommand
			}
		}

		// Process as file
		return aiProcessCmd.RunE(cmd, args)
	}

	// Allow ai command to accept args
	aiCmd.Args = cobra.ArbitraryArgs

	rootCmd.AddCommand(aiCmd)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
