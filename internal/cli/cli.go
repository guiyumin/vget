package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/downloader"
	"github.com/guiyumin/vget/internal/extractor"
	"github.com/guiyumin/vget/internal/updater"
	"github.com/guiyumin/vget/internal/version"
)

type Options struct {
	URL      string
	Output   string
	Quality  string
	Info     bool
	Version  bool
	Help     bool
}

func Run(args []string) error {
	// Handle subcommands first
	if len(args) > 0 {
		switch args[0] {
		case "init":
			return runInit()
		case "update":
			return runUpdate()
		}
	}

	// Check for config file and warn if missing
	if !config.Exists() {
		fmt.Fprintf(os.Stderr, "\033[33mWarning: config file not found. Run 'vget init' to create one.\033[0m\n")
	}

	opts, err := parseArgs(args)
	if err != nil {
		return err
	}

	if opts.Version {
		fmt.Printf("vget %s\n", version.Version)
		return nil
	}

	if opts.Help || opts.URL == "" {
		printUsage()
		return nil
	}

	// Find matching extractor
	ext := extractor.Match(opts.URL)
	if ext == nil {
		return fmt.Errorf("no extractor found for URL: %s", opts.URL)
	}

	fmt.Printf("Extracting: %s\n", opts.URL)

	// Extract video info
	info, err := ext.Extract(opts.URL)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	fmt.Printf("Title: %s\n", info.Title)
	fmt.Printf("Formats: %d available\n", len(info.Formats))

	// Info only mode
	if opts.Info {
		for i, f := range info.Formats {
			fmt.Printf("  [%d] %s %dx%d (%s)\n", i, f.Quality, f.Width, f.Height, f.Ext)
		}
		return nil
	}

	// Select best format (or by quality flag)
	format := selectFormat(info.Formats, opts.Quality)
	if format == nil {
		return errors.New("no suitable format found")
	}

	fmt.Printf("Selected: %s (%s)\n", format.Quality, format.Ext)

	// Determine output filename
	output := opts.Output
	if output == "" {
		output = fmt.Sprintf("%s.%s", info.ID, format.Ext)
	}

	// Download
	dl := downloader.New()
	return dl.Download(format.URL, output, info.Title)
}

func parseArgs(args []string) (*Options, error) {
	opts := &Options{}

	fs := flag.NewFlagSet("vget", flag.ContinueOnError)
	fs.StringVar(&opts.Output, "o", "", "output filename")
	fs.StringVar(&opts.Quality, "q", "", "preferred quality (e.g., 1080p, 720p)")
	fs.BoolVar(&opts.Info, "info", false, "show video info without downloading")
	fs.BoolVar(&opts.Version, "version", false, "show version")
	fs.BoolVar(&opts.Version, "v", false, "show version")
	fs.BoolVar(&opts.Help, "help", false, "show help")
	fs.BoolVar(&opts.Help, "h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Remaining args
	if fs.NArg() > 0 {
		opts.URL = fs.Arg(0)
	}

	return opts, nil
}

func selectFormat(formats []extractor.Format, preferred string) *extractor.Format {
	if len(formats) == 0 {
		return nil
	}

	// If quality specified, try to match
	if preferred != "" {
		for i := range formats {
			if formats[i].Quality == preferred {
				return &formats[i]
			}
		}
	}

	// Otherwise return highest bitrate
	best := &formats[0]
	for i := range formats {
		if formats[i].Bitrate > best.Bitrate {
			best = &formats[i]
		}
	}
	return best
}

func runInit() error {
	// Run interactive wizard (loads existing config as defaults if present)
	cfg, err := config.RunInitWizard()
	if err != nil {
		return err
	}

	// Save config
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("\nSaved %s\n", config.ConfigFileYml)
	return nil
}

func runUpdate() error {
	return updater.Update()
}

func printUsage() {
	fmt.Println(`vget - A modern, blazing-fast, cross-platform downloader cli

Usage:
  vget [options] <url>
  vget <command>

Commands:
  init           Create a .vget.yml config file
  update         Update vget to the latest version

Options:
  -o <file>      Output filename
  -q <quality>   Preferred quality (e.g., 1080p, 720p)
  --info         Show video info without downloading
  -v, --version  Show version
  -h, --help     Show this help

Examples:
  vget init
  vget https://x.com/user/status/123456789
  vget -o video.mp4 https://x.com/user/status/123456789
  vget --info https://x.com/user/status/123456789`)
}
