package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

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

Commands:
  vget ai config                  Configure AI providers (TUI wizard)
  vget ai slice <file>            Slice audio into chunks
  vget ai transcribe <file>       Transcribe audio/video to text
  vget ai summarize <file>        Summarize text

Examples:
  vget ai config
  vget ai slice podcast.mp3 --chunk-duration 5m
  vget ai transcribe podcast.mp3
  vget ai transcribe ./podcast.chunks/
  vget ai summarize transcript.md`,
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

// Slice command flags
var (
	chunkDurationFlag time.Duration
	overlapFlag       time.Duration
)

var aiSliceCmd = &cobra.Command{
	Use:   "slice <file>",
	Short: "Slice audio into chunks",
	Long: `Slice an audio/video file into smaller chunks for transcription.

This is a local operation that does not require API keys.
Chunks are saved to a directory with a manifest for resumability.

Flags:
  --chunk-duration   Duration of each chunk (default: 10m)
  --overlap          Overlap between chunks (default: 10s)

Examples:
  vget ai slice podcast.mp3
  vget ai slice podcast.mp3 --chunk-duration 5m
  vget ai slice podcast.mp3 --chunk-duration 5m --overlap 5s`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filePath)
		}

		opts := ai.ChunkOptions{
			ChunkDuration: chunkDurationFlag,
			Overlap:       overlapFlag,
		}
		return ai.SliceOnly(filePath, opts)
	},
}

// Transcribe command flags
var (
	transcribePasswordFlag string
	transcribeAccountFlag  string
)

var aiTranscribeCmd = &cobra.Command{
	Use:   "transcribe <file|chunks-dir>",
	Short: "Transcribe audio/video to text",
	Long: `Transcribe an audio/video file or chunks directory to text.

Input can be:
  - An audio/video file (mp3, m4a, wav, mp4, etc.)
  - A chunks directory created by 'vget ai slice'

Output:
  - {filename}.transcript.md

Examples:
  vget ai transcribe podcast.mp3
  vget ai transcribe podcast.mp3 --password 1234
  vget ai transcribe ./podcast.chunks/
  vget ai transcribe podcast.mp3 --account work`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		// Check if path exists
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			return fmt.Errorf("path not found: %s", inputPath)
		}

		// Load config
		cfg := config.LoadOrDefault()

		// Validate account exists
		account := cfg.AI.GetAccount(transcribeAccountFlag)
		if account == nil {
			if transcribeAccountFlag == "" {
				return fmt.Errorf("no AI accounts configured\nRun: vget ai config")
			}
			return fmt.Errorf("AI account '%s' not found\nRun: vget ai config", transcribeAccountFlag)
		}

		// Validate transcription is configured
		if account.Transcription.APIKeyEncrypted == "" {
			return fmt.Errorf("transcription not configured for account '%s'\nRun: vget ai config", transcribeAccountFlag)
		}

		// Get PIN
		pin := transcribePasswordFlag
		if pin == "" {
			var err error
			pin, err = promptPIN()
			if err != nil {
				return fmt.Errorf("failed to read PIN: %w", err)
			}
		}

		if err := crypto.ValidatePIN(pin); err != nil {
			return err
		}

		// Create and run pipeline
		pipeline, err := ai.NewPipeline(cfg, transcribeAccountFlag, pin)
		if err != nil {
			return fmt.Errorf("failed to initialize AI pipeline: %w", err)
		}

		ctx := context.Background()
		opts := ai.Options{
			Transcribe: true,
		}

		_, err = pipeline.Process(ctx, inputPath, opts)
		return err
	},
}

// Summarize command flags
var (
	summarizePasswordFlag string
	summarizeAccountFlag  string
)

var aiSummarizeCmd = &cobra.Command{
	Use:   "summarize <file|chunks-dir>",
	Short: "Summarize text",
	Long: `Summarize a text file or transcript.

Input can be:
  - A text file (md, txt)
  - A chunks directory with transcripts

Output:
  - {filename}.summary.md

Examples:
  vget ai summarize transcript.md
  vget ai summarize ./podcast.chunks/
  vget ai summarize notes.txt --account work`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		// Check if path exists
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			return fmt.Errorf("path not found: %s", inputPath)
		}

		// Load config
		cfg := config.LoadOrDefault()

		// Validate account exists
		account := cfg.AI.GetAccount(summarizeAccountFlag)
		if account == nil {
			if summarizeAccountFlag == "" {
				return fmt.Errorf("no AI accounts configured\nRun: vget ai config")
			}
			return fmt.Errorf("AI account '%s' not found\nRun: vget ai config", summarizeAccountFlag)
		}

		// Validate summarization is configured
		if account.Summarization.APIKeyEncrypted == "" {
			return fmt.Errorf("summarization not configured for account '%s'\nRun: vget ai config", summarizeAccountFlag)
		}

		// Get PIN
		pin := summarizePasswordFlag
		if pin == "" {
			var err error
			pin, err = promptPIN()
			if err != nil {
				return fmt.Errorf("failed to read PIN: %w", err)
			}
		}

		if err := crypto.ValidatePIN(pin); err != nil {
			return err
		}

		// Create and run pipeline
		pipeline, err := ai.NewPipeline(cfg, summarizeAccountFlag, pin)
		if err != nil {
			return fmt.Errorf("failed to initialize AI pipeline: %w", err)
		}

		ctx := context.Background()
		opts := ai.Options{
			Summarize: true,
		}

		_, err = pipeline.Process(ctx, inputPath, opts)
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
	// Add subcommands
	aiCmd.AddCommand(aiConfigCmd)
	aiCmd.AddCommand(aiAccountsCmd)
	aiCmd.AddCommand(aiSliceCmd)
	aiCmd.AddCommand(aiTranscribeCmd)
	aiCmd.AddCommand(aiSummarizeCmd)

	// Slice command flags
	aiSliceCmd.Flags().DurationVar(&chunkDurationFlag, "chunk-duration", 10*time.Minute, "Duration of each chunk")
	aiSliceCmd.Flags().DurationVar(&overlapFlag, "overlap", 10*time.Second, "Overlap between chunks")

	// Transcribe command flags
	aiTranscribeCmd.Flags().StringVar(&transcribePasswordFlag, "password", "", "4-digit PIN to decrypt API keys")
	aiTranscribeCmd.Flags().StringVar(&transcribeAccountFlag, "account", "", "Use specific AI account")

	// Summarize command flags
	aiSummarizeCmd.Flags().StringVar(&summarizePasswordFlag, "password", "", "4-digit PIN to decrypt API keys")
	aiSummarizeCmd.Flags().StringVar(&summarizeAccountFlag, "account", "", "Use specific AI account")

	rootCmd.AddCommand(aiCmd)
}
