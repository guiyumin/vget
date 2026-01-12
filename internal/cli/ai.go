package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/core/ai"
	aioutput "github.com/guiyumin/vget/internal/core/ai/output"
	"github.com/guiyumin/vget/internal/core/ai/transcriber"
	"github.com/guiyumin/vget/internal/core/auth"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/spf13/cobra"
)

var (
	aiModel    string
	aiLanguage string
	aiFrom     string
	aiRemote   bool
	aiOutput   string
)

// aiCmd is the parent command for all AI features
var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI-powered transcription and more",
	Long: `AI features for vget including speech-to-text transcription.

Models are downloaded on first use and stored in ~/.config/vget/models/

Examples:
  # speech-to-text (alias for transcribe)
  vget ai stt audio.mp3 -l zh            
  vget ai stt audio.mp3 -l zh -o out.srt
  vget ai models
  vget ai models download whisper-large-v3-turbo`,
}

// aiTranscribeCmd transcribes audio/video files
var aiTranscribeCmd = &cobra.Command{
	Use:   "transcribe <file>",
	Short: "Transcribe audio/video to markdown (use 'stt' for short)",
	Long: `Transcribe audio or video files with timestamps.

Tip: Use 'vget ai stt' as a shorter alias.

The transcript is saved as <filename>.transcript.md by default.

Output format is detected from -o extension:
  .md  - Markdown with timestamps (default)
  .srt - SubRip subtitle format
  .vtt - WebVTT subtitle format

Language is required. Common language codes:
  zh - Chinese    en - English    ja - Japanese
  ko - Korean     es - Spanish    fr - French
  de - German     ru - Russian    pt - Portuguese

Examples:
  vget ai stt podcast.mp3 -l zh
  vget ai stt podcast.mp3 -l zh -o subtitles.srt
  vget ai transcribe podcast.mp3 -l en --model whisper-small`,
	Args: cobra.ExactArgs(1),
	Run:  runTranscribe,
}

// aiModelsCmd is the parent command for model management
var aiModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List and manage transcription models",
	Long: `List downloaded models or available models from remote.

By default, shows locally downloaded models.
Use -r/--remote to show models available for download.

Examples:
  vget ai models              # List downloaded models
  vget ai models -r           # List available models from remote
  vget ai models download whisper-large-v3-turbo
  vget ai models rm whisper-small`,
	Run: runModels,
}

// aiModelsDownloadCmd downloads a model
var aiModelsDownloadCmd = &cobra.Command{
	Use:   "download <model>",
	Short: "Download a transcription model",
	Long: `Download a transcription model for local speech-to-text.

Available models:
  whisper-tiny            (78MB) - Fastest, basic quality
  whisper-base           (148MB) - Good for quick drafts
  whisper-small          (488MB) - Balanced for most uses
  whisper-medium         (1.5GB) - Higher accuracy
  whisper-large-v3       (3.1GB) - Highest accuracy, slowest
  whisper-large-v3-turbo (1.6GB) - Best quality + fast (recommended)

Download sources:
  huggingface (default) - Official sources
  vmirror               - vmirror.org (faster in China)

Examples:
  vget ai models download whisper-large-v3-turbo
  vget ai models download whisper-small --from=vmirror`,
	Args: cobra.ExactArgs(1),
	Run:  runModelsDownload,
}

// aiModelsRmCmd removes a downloaded model
var aiModelsRmCmd = &cobra.Command{
	Use:   "rm <model>",
	Short: "Remove a downloaded model",
	Long: `Remove a downloaded model to free up disk space.

Examples:
  vget ai models rm whisper-small
  vget ai models rm whisper-medium`,
	Args: cobra.ExactArgs(1),
	Run:  runModelsRm,
}

// aiDownloadCmd is an alias for models download
var aiDownloadCmd = &cobra.Command{
	Use:   "download <model>",
	Short: "Download a transcription model (alias for 'models download')",
	Long: `Download a transcription model for local speech-to-text.

This is an alias for 'vget ai models download'.

Examples:
  vget ai download whisper-large-v3-turbo
  vget ai download whisper-small --from=vmirror`,
	Args: cobra.ExactArgs(1),
	Run:  runModelsDownload,
}

// aiSttCmd is speech-to-text command (stt = speech-to-text)
var aiSttCmd = &cobra.Command{
	Use:   "stt <file>",
	Short: "Speech-to-text transcription",
	Long: `Transcribe audio or video files with timestamps.

The transcript is saved as <filename>.transcript.md by default.

Output format is detected from -o extension:
  .md  - Markdown with timestamps (default)
  .srt - SubRip subtitle format
  .vtt - WebVTT subtitle format

Language is required. Common language codes:
  zh - Chinese    en - English    ja - Japanese
  ko - Korean     es - Spanish    fr - French
  de - German     ru - Russian    pt - Portuguese

Examples:
  vget ai stt podcast.mp3 -l zh
  vget ai stt podcast.mp3 -l zh -o subtitles.srt
  vget ai stt podcast.mp3 -l en --model whisper-small`,
	Args: cobra.ExactArgs(1),
	Run:  runTranscribe,
}

func runTranscribe(cmd *cobra.Command, args []string) {
	filePath := args[0]

	// Validate language is provided
	if aiLanguage == "" {
		fmt.Fprintf(os.Stderr, "Error: --language is required\n\n")
		fmt.Fprintln(os.Stderr, "Common language codes:")
		fmt.Fprintln(os.Stderr, "  zh - Chinese    en - English    ja - Japanese")
		fmt.Fprintln(os.Stderr, "  ko - Korean     es - Spanish    fr - French")
		fmt.Fprintln(os.Stderr, "  de - German     ru - Russian    pt - Portuguese")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintf(os.Stderr, "  vget ai transcribe %s --language zh\n", filePath)
		os.Exit(1)
	}

	// Validate file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", filePath)
		os.Exit(1)
	}

	// Get models directory
	modelsDir, err := transcriber.DefaultModelsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Determine model to use
	modelName := aiModel
	if modelName == "" {
		modelName = transcriber.DefaultModel
	}

	// Validate model exists
	model := transcriber.GetModel(modelName)
	if model == nil {
		fmt.Fprintf(os.Stderr, "Error: unknown model '%s'\n\n", modelName)
		fmt.Println("Available models:")
		for _, m := range transcriber.ASRModels {
			fmt.Printf("  %-24s (%s) - %s\n", m.Name, m.Size, m.Description)
		}
		os.Exit(1)
	}

	// Check if language is valid at all
	if !transcriber.IsValidLanguage(aiLanguage) {
		fmt.Fprintf(os.Stderr, "Error: unknown language code '%s'\n\n", aiLanguage)
		fmt.Fprintln(os.Stderr, "Common language codes:")
		fmt.Fprintln(os.Stderr, "  zh - Chinese    en - English    ja - Japanese")
		fmt.Fprintln(os.Stderr, "  ko - Korean     es - Spanish    fr - French")
		fmt.Fprintln(os.Stderr, "  de - German     ru - Russian    pt - Portuguese")
		os.Exit(1)
	}

	// Check if model supports the language
	if !transcriber.ModelSupportsLanguage(modelName, aiLanguage) {
		fmt.Fprintf(os.Stderr, "Error: model '%s' does not support language '%s'\n", modelName, aiLanguage)
		os.Exit(1)
	}

	// Check if model is downloaded
	mm := transcriber.NewModelManager(modelsDir)
	if !mm.IsModelDownloaded(modelName) {
		// Get translations
		cfg, _ := config.Load()
		lang := "en"
		if cfg != nil && cfg.Language != "" {
			lang = cfg.Language
		}
		t := i18n.T(lang)

		// Styles
		titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true) // red
		cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))               // cyan
		headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // green
		sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // orange

		fmt.Println(titleStyle.Render(t.AICLI.ModelNotFound) + " " + t.AICLI.DownloadWith)
		fmt.Printf("  %s\n", cmdStyle.Render(fmt.Sprintf("vget ai models download %s", modelName)))
		fmt.Printf("  %s\n\n", hintStyle.Render(t.AICLI.VmirrorHint))
		fmt.Println(headerStyle.Render(t.AICLI.AvailableModels))
		for _, m := range transcriber.ASRModels {
			name := nameStyle.Render(fmt.Sprintf("%-24s", m.Name))
			size := sizeStyle.Render(fmt.Sprintf("(%s)", m.Size))
			desc := descStyle.Render(m.Description)
			fmt.Printf("  %s %s - %s\n", name, size, desc)
		}
		os.Exit(0)
	}

	// Create local ASR config
	localCfg := config.LocalASRConfig{
		Engine:    "whisper",
		Model:     modelName,
		ModelsDir: modelsDir,
		Language:  aiLanguage,
	}

	// Create pipeline with local transcription (no summarization)
	pipeline, err := ai.NewLocalPipeline(localCfg, nil, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Set up TUI progress reporter
	reporter := transcriber.NewTUIProgressReporter()
	pipeline.SetProgressReporter(reporter.ProgressReporter)

	// Run transcription in background
	ctx := context.Background()
	opts := ai.Options{
		Transcribe: true,
		Summarize:  false,
	}

	var result *ai.Result
	var processErr error
	done := make(chan struct{})

	go func() {
		result, processErr = pipeline.Process(ctx, filePath, opts)
		reporter.SetDone()
		close(done)
	}()

	// Run TUI (blocks until transcription completes)
	if err := transcriber.RunTranscribeTUI(filepath.Base(filePath), modelName, reporter); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Wait for process to complete
	<-done

	if processErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", processErr)
		os.Exit(1)
	}

	// Handle custom output path
	outputPath := result.TranscriptPath
	if aiOutput != "" {
		ext := strings.ToLower(filepath.Ext(aiOutput))

		// Convert based on extension
		var outputContent string
		switch ext {
		case ".srt":
			segments := convertSegments(result.Transcript.Segments)
			outputContent = aioutput.ToSRT(segments)
		case ".vtt":
			segments := convertSegments(result.Transcript.Segments)
			outputContent = aioutput.ToVTT(segments)
		default:
			// .md or other - copy markdown as-is
			data, err := os.ReadFile(result.TranscriptPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading transcript: %v\n", err)
				os.Exit(1)
			}
			outputContent = string(data)
		}

		if err := os.WriteFile(aiOutput, []byte(outputContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", aiOutput, err)
			os.Exit(1)
		}
		outputPath = aiOutput
	}

	fmt.Printf("\nTranscript saved: %s\n", outputPath)
}

func runModels(cmd *cobra.Command, args []string) {
	modelsDir, err := transcriber.DefaultModelsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	mm := transcriber.NewModelManager(modelsDir)

	if aiRemote {
		// Get translations
		cfg, _ := config.Load()
		lang := "en"
		if cfg != nil && cfg.Language != "" {
			lang = cfg.Language
		}
		t := i18n.T(lang)
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

		// Show all available models (remote)
		fmt.Println("Available models (remote):")
		fmt.Println()
		for _, m := range transcriber.ASRModels {
			downloaded := ""
			if mm.IsModelDownloaded(m.Name) {
				downloaded = " [downloaded]"
			}
			fmt.Printf("  %-24s %8s  %s%s\n", m.Name, m.Size, m.Description, downloaded)
		}
		fmt.Println()
		fmt.Println(t.AICLI.DownloadAModel)
		fmt.Println("  vget ai models download <model-name>")
		fmt.Printf("  %s\n", hintStyle.Render(t.AICLI.VmirrorHint))
	} else {
		// Show downloaded models only
		downloaded := mm.ListDownloadedModels()
		if len(downloaded) == 0 {
			// Get translations
			cfg, _ := config.Load()
			lang := "en"
			if cfg != nil && cfg.Language != "" {
				lang = cfg.Language
			}
			t := i18n.T(lang)

			hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

			fmt.Println(t.AICLI.NoModelsDownloaded)
			fmt.Println()
			fmt.Println(t.AICLI.DownloadAModel)
			fmt.Println("  vget ai models download whisper-large-v3-turbo")
			fmt.Printf("  %s\n", hintStyle.Render(t.AICLI.VmirrorHint))
			fmt.Println()
			fmt.Println(t.AICLI.SeeAvailableModels)
			fmt.Println("  vget ai models -r")
			return
		}

		fmt.Println("Downloaded models:")
		fmt.Println()
		for _, name := range downloaded {
			model := transcriber.GetModel(name)
			if model != nil {
				fmt.Printf("  %-24s %8s  %s\n", model.Name, model.Size, model.Description)
			} else {
				fmt.Printf("  %s\n", name)
			}
		}
		fmt.Println()
		fmt.Printf("Models directory: %s\n", modelsDir)
	}
}

func runModelsDownload(cmd *cobra.Command, args []string) {
	modelName := args[0]

	// Validate model name
	model := transcriber.GetModel(modelName)
	if model == nil {
		fmt.Fprintf(os.Stderr, "Error: unknown model '%s'\n\n", modelName)
		fmt.Println("Available models:")
		for _, m := range transcriber.ASRModels {
			fmt.Printf("  %-24s (%s) - %s\n", m.Name, m.Size, m.Description)
		}
		os.Exit(1)
	}

	// Get models directory
	modelsDir, err := transcriber.DefaultModelsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	mm := transcriber.NewModelManager(modelsDir)

	// Check if already downloaded
	if mm.IsModelDownloaded(modelName) {
		fmt.Printf("Model '%s' is already downloaded.\n", modelName)
		fmt.Printf("Location: %s\n", mm.ModelPath(modelName))
		return
	}

	// Determine download URL based on --from flag
	downloadURL := model.OfficialURL // Default: Hugging Face
	source := "Hugging Face"

	switch strings.ToLower(aiFrom) {
	case "vmirror":
		// vmirror.org mirror (faster in China)
		if model.VmirrorURL == "" {
			fmt.Fprintf(os.Stderr, "Error: model '%s' is not available on vmirror\n\n", modelName)
			fmt.Fprintln(os.Stderr, "Models available on vmirror:")
			for _, name := range transcriber.ListVmirrorModels() {
				fmt.Printf("  %s\n", name)
			}
			fmt.Fprintln(os.Stderr, "\nUse default source instead:")
			fmt.Fprintf(os.Stderr, "  vget ai download %s\n", modelName)
			os.Exit(1)
		}
		source = "vmirror.org"

		// Get language for prompts
		lang := "en"
		if cfg, _ := config.Load(); cfg != nil {
			lang = cfg.Language
		}

		// Get signed URL from auth server
		fmt.Println()
		filename := transcriber.GetVmirrorFilename(modelName)
		signedURL, err := auth.GetSignedURL(filename, lang)
		if err != nil {
			t := i18n.T(lang)
			errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
			fmt.Fprintf(os.Stderr, "%s %v\n", errStyle.Render(t.AICLI.AuthFailed), err)
			os.Exit(1)
		}
		downloadURL = signedURL
	case "huggingface", "github", "":
		// Default: Hugging Face (already set)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown source '%s'\n", aiFrom)
		fmt.Fprintln(os.Stderr, "Available sources: huggingface/github (default), vmirror")
		os.Exit(1)
	}

	// Styles for download output
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))   // cyan
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))             // gray
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))             // white
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")) // green
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))               // blue

	// Show download info
	fmt.Println()
	fmt.Println("  " + titleStyle.Render("📦 Downloading "+model.Name+" ("+model.Size+")"))
	fmt.Printf("  %s %s\n", labelStyle.Render("Source:"), valueStyle.Render(source))

	// Get language for i18n
	cfg := config.LoadOrDefault()

	// Download with progress bar
	modelPath, err := mm.DownloadModelWithProgress(modelName, downloadURL, cfg.Language)
	if err != nil {
		errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
		fmt.Fprintf(os.Stderr, "\n%s %v\n", errStyle.Render("Error:"), err)
		if aiFrom != "vmirror" {
			hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			fmt.Fprintf(os.Stderr, "\n%s\n", hintStyle.Render("💡 Tip: Try vmirror if download is slow or blocked:"))
			fmt.Fprintf(os.Stderr, "  vget ai models download %s --from=vmirror\n", modelName)
		}
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓ Download complete!"))
	fmt.Printf("  %s %s\n", labelStyle.Render("Location:"), pathStyle.Render(modelPath))
}

func runModelsRm(cmd *cobra.Command, args []string) {
	modelName := args[0]

	// Get models directory
	modelsDir, err := transcriber.DefaultModelsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	mm := transcriber.NewModelManager(modelsDir)

	// Check if model exists
	if !mm.IsModelDownloaded(modelName) {
		fmt.Fprintf(os.Stderr, "Error: model '%s' is not downloaded\n", modelName)
		os.Exit(1)
	}

	modelPath := mm.ModelPath(modelName)

	// Remove the model
	if err := os.RemoveAll(modelPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing model: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed model: %s\n", modelName)
}

// convertSegments converts transcriber.Segment to output.Segment
func convertSegments(segments []transcriber.Segment) []aioutput.Segment {
	result := make([]aioutput.Segment, len(segments))
	for i, seg := range segments {
		result[i] = aioutput.Segment{
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		}
	}
	return result
}

func init() {
	// Flags for transcribe command
	aiTranscribeCmd.Flags().StringVar(&aiModel, "model", "", "model to use (default: whisper-large-v3-turbo)")
	aiTranscribeCmd.Flags().StringVarP(&aiLanguage, "language", "l", "", "language code (required, e.g., zh, en, ja)")
	aiTranscribeCmd.Flags().StringVarP(&aiOutput, "output", "o", "", "output file path (.md, .srt, .vtt)")

	// Flags for models command
	aiModelsCmd.Flags().BoolVarP(&aiRemote, "remote", "r", false, "list models available for download")

	// Flags for models download command
	aiModelsDownloadCmd.Flags().StringVar(&aiFrom, "from", "huggingface", "download source: huggingface (default), vmirror")

	// Flags for download alias command
	aiDownloadCmd.Flags().StringVar(&aiFrom, "from", "huggingface", "download source: huggingface (default), vmirror")

	// Flags for stt alias command
	aiSttCmd.Flags().StringVar(&aiModel, "model", "", "model to use (default: whisper-large-v3-turbo)")
	aiSttCmd.Flags().StringVarP(&aiLanguage, "language", "l", "", "language code (required, e.g., zh, en, ja)")
	aiSttCmd.Flags().StringVarP(&aiOutput, "output", "o", "", "output file path (.md, .srt, .vtt)")

	// Add subcommands to models
	aiModelsCmd.AddCommand(aiModelsDownloadCmd)
	aiModelsCmd.AddCommand(aiModelsRmCmd)

	// Add subcommands to ai
	aiCmd.AddCommand(aiTranscribeCmd)
	aiCmd.AddCommand(aiSttCmd)      // Alias for transcribe
	aiCmd.AddCommand(aiModelsCmd)
	aiCmd.AddCommand(aiDownloadCmd) // Alias for models download

	// Add ai command to root
	rootCmd.AddCommand(aiCmd)
}
