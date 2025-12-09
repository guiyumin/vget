package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/server"
	"github.com/spf13/cobra"
)

var (
	servePort      int
	serveOutputDir string
	serveDaemon    bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server for remote downloads",
	Long: `Start an HTTP server that accepts download requests via API.

Examples:
  vget serve              # Start server on port 8080
  vget serve -p 9000      # Start server on port 9000
  vget serve -d           # Start server as background daemon
  vget serve -o ~/dl      # Use custom output directory

API Endpoints:
  GET  /health            # Health check
  POST /download          # Queue a download
  GET  /status/:id        # Get job status
  GET  /jobs              # List all jobs
  DELETE /jobs/:id        # Cancel a job`,
	Run: func(cmd *cobra.Command, args []string) {
		// Handle subcommands
		if len(args) > 0 {
			switch args[0] {
			case "stop":
				if err := stopDaemon(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			case "status":
				if err := daemonStatus(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			}
		}

		if err := runServe(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 0, "HTTP listen port (default: 8080)")
	serveCmd.Flags().StringVarP(&serveOutputDir, "output", "o", "", "output directory for downloads")
	serveCmd.Flags().BoolVarP(&serveDaemon, "daemon", "d", false, "run as background daemon")

	rootCmd.AddCommand(serveCmd)
}

func runServe() error {
	cfg := config.LoadOrDefault()

	// Resolve port (flag > config > default)
	port := servePort
	if port == 0 {
		if cfg.Server.Port > 0 {
			port = cfg.Server.Port
		} else {
			port = 8080
		}
	}

	// Resolve output directory (flag > config > default)
	outputDir := serveOutputDir
	if outputDir == "" {
		if cfg.Server.OutputDir != "" {
			outputDir = cfg.Server.OutputDir
		} else {
			outputDir = "./downloads"
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
		maxConcurrent = 3
	}

	// Get API key from config
	apiKey := cfg.Server.APIKey

	// Daemon mode
	if serveDaemon {
		return startDaemon(port, outputDir)
	}

	// Foreground mode
	return runServer(port, outputDir, apiKey, maxConcurrent)
}

func runServer(port int, outputDir, apiKey string, maxConcurrent int) error {
	srv := server.NewServer(port, outputDir, apiKey, maxConcurrent)

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

	return srv.Start()
}

func startDaemon(port int, outputDir string) error {
	// Check if already running
	if pid := getDaemonPID(); pid > 0 {
		// Check if process is actually running
		if processExists(pid) {
			return fmt.Errorf("daemon already running (PID %d)", pid)
		}
		// Stale PID file, remove it
		os.Remove(getPIDFilePath())
	}

	// Get the current executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build arguments
	args := []string{"serve", "-p", strconv.Itoa(port), "-o", outputDir}

	// Create log file
	logFile, err := os.OpenFile(getLogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Start the daemon process
	cmd := exec.Command(executable, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// Detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Save PID
	if err := savePID(cmd.Process.Pid); err != nil {
		cmd.Process.Kill()
		logFile.Close()
		return fmt.Errorf("failed to save PID: %w", err)
	}

	fmt.Printf("vget server started as daemon (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("  Port: %d\n", port)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Log: %s\n", getLogFilePath())
	fmt.Printf("\nUse 'vget serve stop' to stop the daemon\n")

	return nil
}

func stopDaemon() error {
	pid := getDaemonPID()
	if pid <= 0 {
		return fmt.Errorf("daemon is not running")
	}

	// Send SIGTERM
	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(getPIDFilePath())
		return fmt.Errorf("daemon process not found")
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		os.Remove(getPIDFilePath())
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Wait for process to exit
	for i := 0; i < 30; i++ {
		if !processExists(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	os.Remove(getPIDFilePath())
	fmt.Println("Daemon stopped")
	return nil
}

func daemonStatus() error {
	pid := getDaemonPID()
	if pid <= 0 {
		fmt.Println("Daemon is not running")
		return nil
	}

	if !processExists(pid) {
		os.Remove(getPIDFilePath())
		fmt.Println("Daemon is not running (stale PID file removed)")
		return nil
	}

	fmt.Printf("Daemon is running (PID %d)\n", pid)
	fmt.Printf("Log file: %s\n", getLogFilePath())
	return nil
}

// Helper functions for PID file management

func getPIDFilePath() string {
	configDir, err := config.ConfigDir()
	if err != nil {
		return "/tmp/vget-serve.pid"
	}
	return filepath.Join(configDir, "serve.pid")
}

func getLogFilePath() string {
	configDir, err := config.ConfigDir()
	if err != nil {
		return "/tmp/vget-serve.log"
	}
	return filepath.Join(configDir, "serve.log")
}

func savePID(pid int) error {
	pidFile := getPIDFilePath()
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func getDaemonPID() int {
	data, err := os.ReadFile(getPIDFilePath())
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}
	return pid
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
