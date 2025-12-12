package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/version"
	"github.com/guiyumin/vget/internal/server"
)

func main() {
	// Command-line flags
	port := flag.Int("port", 0, "HTTP listen port (default: 8080)")
	output := flag.String("output", "", "output directory for downloads")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vget-server %s\n", version.Version)
		return
	}

	// Load configuration
	cfg := config.LoadOrDefault()

	// Resolve port (flag > config > default)
	serverPort := *port
	if serverPort == 0 {
		if cfg.Server.Port > 0 {
			serverPort = cfg.Server.Port
		} else {
			serverPort = 8080
		}
	}

	// Resolve output directory (flag > config > default)
	outputDir := *output
	if outputDir == "" {
		if cfg.OutputDir != "" {
			outputDir = cfg.OutputDir
		} else {
			outputDir = config.DefaultDownloadDir()
		}
	}

	// Expand ~ in path
	if len(outputDir) >= 2 && outputDir[:2] == "~/" {
		home, _ := os.UserHomeDir()
		outputDir = filepath.Join(home, outputDir[2:])
	}

	// Resolve max concurrent (config > default)
	maxConcurrent := cfg.Server.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	// Get API key from config
	apiKey := cfg.Server.APIKey

	// Create and start server
	srv := server.NewServer(serverPort, outputDir, apiKey, maxConcurrent)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Stop(ctx)
	}()

	log.Printf("Starting vget server on port %d", serverPort)
	log.Printf("Output directory: %s", outputDir)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
