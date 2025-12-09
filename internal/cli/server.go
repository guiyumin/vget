package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/config"
	"github.com/guiyumin/vget/internal/server"
	"github.com/spf13/cobra"
)

// Server command flags
var (
	serverPort      int
	serverOutputDir string
	serverDaemon    bool
)

// Install command flags
var (
	installYes    bool
	installPort   int
	installOutput string
	installUser   string
)

// Service constants
const (
	defaultServicePort   = 8080
	defaultServiceUser   = "vget"
	defaultServiceOutput = "/var/lib/vget/downloads"
	serviceName          = "vget"
	binaryPath           = "/usr/local/bin/vget"
	serviceFilePath      = "/etc/systemd/system/vget.service"
	configDirPath        = "/etc/vget"
	configFilePath       = "/etc/vget/config.yml"
)

// Parent command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the vget HTTP server",
	Long: `Manage the vget HTTP server for remote downloads.

Subcommands:
  start      Start the server
  stop       Stop the server
  restart    Restart the server
  status     Show server status
  logs       View server logs
  install    Install as systemd service
  uninstall  Remove systemd service`,
}

// vget server start
var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the HTTP server",
	Long: `Start the HTTP server for remote downloads.

Examples:
  vget server start              # Start server in foreground on port 8080
  vget server start -d           # Start server as background daemon
  vget server start -p 9000      # Start server on port 9000
  vget server start -d -o ~/dl   # Daemon with custom output directory`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runServerStart(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// vget server stop
var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the server",
	Long:  `Stop the running vget server daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := stopDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// vget server restart
var serverRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the server",
	Long: `Restart the vget server daemon.

Examples:
  vget server restart            # Restart with same settings
  vget server restart -d         # Restart as daemon`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runServerRestart(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// vget server status
var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	Long:  `Check if the vget server daemon is running.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := daemonStatus(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// vget server logs
var serverLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View server logs",
	Long: `View the vget server log file.

Examples:
  vget server logs               # Show recent logs
  vget server logs -f            # Follow logs (tail -f)`,
	Run: func(cmd *cobra.Command, args []string) {
		followLogs, _ := cmd.Flags().GetBool("follow")
		if err := viewLogs(followLogs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// vget server install
var serverInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install as systemd service",
	Long: `Install vget as a systemd service for running the download server.

This command will:
  - Copy the vget binary to /usr/local/bin/
  - Create a systemd service file
  - Create a dedicated user (optional)
  - Enable and start the service

Requires root/sudo privileges.

Examples:
  sudo vget server install              # Interactive installation
  sudo vget server install -y           # Non-interactive with defaults
  sudo vget server install -p 9000      # Custom port
  sudo vget server install -o /data/dl  # Custom output directory`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// vget server uninstall
var serverUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove systemd service",
	Long: `Remove the vget systemd service.

This command will:
  - Stop the service if running
  - Disable the service
  - Remove the service file
  - Optionally remove the vget user

The binary at /usr/local/bin/vget and download files are NOT removed.

Requires root/sudo privileges.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// Start command flags
	serverStartCmd.Flags().IntVarP(&serverPort, "port", "p", 0, "HTTP listen port (default: 8080)")
	serverStartCmd.Flags().StringVarP(&serverOutputDir, "output", "o", "", "output directory for downloads")
	serverStartCmd.Flags().BoolVarP(&serverDaemon, "daemon", "d", false, "run as background daemon")

	// Restart command flags
	serverRestartCmd.Flags().IntVarP(&serverPort, "port", "p", 0, "HTTP listen port (default: 8080)")
	serverRestartCmd.Flags().StringVarP(&serverOutputDir, "output", "o", "", "output directory for downloads")
	serverRestartCmd.Flags().BoolVarP(&serverDaemon, "daemon", "d", false, "run as background daemon")

	// Logs command flags
	serverLogsCmd.Flags().BoolP("follow", "f", false, "follow log output")

	// Install command flags
	serverInstallCmd.Flags().BoolVarP(&installYes, "yes", "y", false, "skip interactive TUI, use defaults")
	serverInstallCmd.Flags().IntVarP(&installPort, "port", "p", 0, "service port (default: 8080)")
	serverInstallCmd.Flags().StringVarP(&installOutput, "output", "o", "", "output directory (default: /var/lib/vget/downloads)")
	serverInstallCmd.Flags().StringVarP(&installUser, "user", "u", "", "user to run service as (default: vget)")

	// Add subcommands to server
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverRestartCmd)
	serverCmd.AddCommand(serverStatusCmd)
	serverCmd.AddCommand(serverLogsCmd)
	serverCmd.AddCommand(serverInstallCmd)
	serverCmd.AddCommand(serverUninstallCmd)

	// Add server to root
	rootCmd.AddCommand(serverCmd)
}

// ============================================================================
// Server start/stop/restart/status/logs
// ============================================================================

func runServerStart() error {
	cfg := config.LoadOrDefault()

	// Resolve port (flag > config > default)
	port := serverPort
	if port == 0 {
		if cfg.Server.Port > 0 {
			port = cfg.Server.Port
		} else {
			port = 8080
		}
	}

	// Resolve output directory (flag > global config > default)
	outputDir := serverOutputDir
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

	// Daemon mode
	if serverDaemon {
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
	args := []string{"server", "start", "-p", strconv.Itoa(port), "-o", outputDir}

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
	fmt.Printf("\nUse 'vget server stop' to stop the daemon\n")

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

func runServerRestart() error {
	// Stop if running
	pid := getDaemonPID()
	if pid > 0 && processExists(pid) {
		fmt.Println("Stopping server...")
		if err := stopDaemon(); err != nil {
			return err
		}
		// Wait a moment for port to be released
		time.Sleep(500 * time.Millisecond)
	}

	// Start again
	fmt.Println("Starting server...")
	return runServerStart()
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

func viewLogs(follow bool) error {
	logPath := getLogFilePath()

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", logPath)
	}

	var cmd *exec.Cmd
	if follow {
		cmd = exec.Command("tail", "-f", logPath)
	} else {
		cmd = exec.Command("tail", "-100", logPath)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ============================================================================
// PID file helpers
// ============================================================================

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

// ============================================================================
// Install/Uninstall
// ============================================================================

type installConfig struct {
	Port      int
	OutputDir string
	User      string
}

func runInstall() error {
	// Check platform support
	if runtime.GOOS != "linux" {
		printUnsupportedPlatform()
		return nil
	}

	// Check for systemd
	if !hasSystemd() {
		fmt.Println("systemd not found. This command requires systemd.")
		fmt.Println("\nFor manual service setup, see:")
		fmt.Println("https://github.com/guiyumin/vget/blob/main/docs/manual-service-setup.md")
		return nil
	}

	// Check for root
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command requires root privileges. Please run with sudo")
	}

	// Get configuration
	cfg := installConfig{
		Port:      defaultServicePort,
		OutputDir: defaultServiceOutput,
		User:      defaultServiceUser,
	}

	// Override with flags
	if installPort > 0 {
		cfg.Port = installPort
	}
	if installOutput != "" {
		cfg.OutputDir = installOutput
	}
	if installUser != "" {
		cfg.User = installUser
	}

	// Non-interactive mode
	if installYes || (installPort > 0 || installOutput != "" || installUser != "") {
		return doInstall(cfg)
	}

	// Interactive TUI mode
	return runInstallTUI()
}

func runInstallTUI() error {
	m := initialInstallModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result := finalModel.(installModel)
	if result.cancelled {
		fmt.Println("Installation cancelled.")
		return nil
	}

	if result.confirmed {
		return doInstall(result.config)
	}

	return nil
}

func doInstall(cfg installConfig) error {
	fmt.Println("\nInstalling vget service...")
	fmt.Println()

	// Step 1: Check if service already exists
	if serviceExists() {
		fmt.Println("  Stopping existing service...")
		runSystemctl("stop", serviceName)
	}

	// Step 2: Create user if needed
	if cfg.User != "root" {
		if !userExists(cfg.User) {
			fmt.Printf("  Creating user '%s'...\n", cfg.User)
			if err := createServiceUser(cfg.User); err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}
			fmt.Printf("  ✓ User '%s' created\n", cfg.User)
		} else {
			fmt.Printf("  ✓ User '%s' exists\n", cfg.User)
		}
	}

	// Step 3: Create output directory
	fmt.Printf("  Creating output directory '%s'...\n", cfg.OutputDir)
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if cfg.User != "root" {
		if err := chownRecursive(cfg.OutputDir, cfg.User); err != nil {
			return fmt.Errorf("failed to set directory ownership: %w", err)
		}
	}
	fmt.Printf("  ✓ Output directory ready\n")

	// Step 4: Copy binary
	fmt.Println("  Copying binary to /usr/local/bin/...")
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	if err := copyFile(executable, binaryPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}
	fmt.Println("  ✓ Binary installed")

	// Step 5: Create config directory and file
	fmt.Println("  Creating service configuration...")
	if err := os.MkdirAll(configDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	configContent := fmt.Sprintf(`# vget service configuration
output_dir: %s
server:
  port: %d
  max_concurrent: 10
`, cfg.OutputDir, cfg.Port)
	if err := os.WriteFile(configFilePath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	fmt.Println("  ✓ Configuration created")

	// Step 6: Create systemd service file
	fmt.Println("  Creating systemd service...")
	serviceContent := generateServiceFile(cfg)
	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}
	fmt.Println("  ✓ Service file created")

	// Step 7: Enable and start service
	fmt.Println("  Enabling service...")
	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	if err := runSystemctl("enable", serviceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}
	fmt.Println("  ✓ Service enabled")

	fmt.Println("  Starting service...")
	if err := runSystemctl("start", serviceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	fmt.Println("  ✓ Service started")

	// Print success message
	fmt.Println()
	printSuccessBox(cfg)

	return nil
}

func runUninstall() error {
	// Check platform support
	if runtime.GOOS != "linux" {
		fmt.Println("vget server uninstall is only supported on Linux with systemd.")
		return nil
	}

	// Check for root
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command requires root privileges. Please run with sudo")
	}

	fmt.Println("Uninstalling vget service...")
	fmt.Println()

	// Stop service
	if serviceExists() {
		fmt.Println("  Stopping service...")
		runSystemctl("stop", serviceName)
		fmt.Println("  ✓ Service stopped")
	}

	// Disable service
	fmt.Println("  Disabling service...")
	runSystemctl("disable", serviceName)
	fmt.Println("  ✓ Service disabled")

	// Remove service file
	if _, err := os.Stat(serviceFilePath); err == nil {
		fmt.Println("  Removing service file...")
		os.Remove(serviceFilePath)
		runSystemctl("daemon-reload")
		fmt.Println("  ✓ Service file removed")
	}

	fmt.Println()
	fmt.Println("vget service has been removed.")
	fmt.Println()
	fmt.Println("The following were NOT removed:")
	fmt.Printf("  - Binary: %s\n", binaryPath)
	fmt.Printf("  - Config: %s\n", configFilePath)
	fmt.Printf("  - Downloads: (check your output directory)\n")
	fmt.Println()
	fmt.Println("To completely remove vget:")
	fmt.Printf("  sudo rm %s\n", binaryPath)
	fmt.Printf("  sudo rm -rf %s\n", configDirPath)

	return nil
}

// ============================================================================
// Install helpers
// ============================================================================

func hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func serviceExists() bool {
	cmd := exec.Command("systemctl", "status", serviceName)
	err := cmd.Run()
	// Service exists if exit code is 0, 3 (stopped), or 4 (no such unit but might have file)
	return err == nil || cmd.ProcessState.ExitCode() == 3
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func userExists(username string) bool {
	_, err := user.Lookup(username)
	return err == nil
}

func createServiceUser(username string) error {
	cmd := exec.Command("useradd", "-r", "-s", "/bin/false", "-d", "/var/lib/vget", username)
	return cmd.Run()
}

func chownRecursive(path, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(name, uid, gid)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func generateServiceFile(cfg installConfig) string {
	return fmt.Sprintf(`# /etc/systemd/system/vget.service
# Generated by vget server install

[Unit]
Description=vget media downloader server
After=network.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s server start --config %s
Restart=always
RestartSec=5
WorkingDirectory=%s

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=%s
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`, cfg.User, cfg.User, binaryPath, configFilePath, cfg.OutputDir, cfg.OutputDir)
}

func printUnsupportedPlatform() {
	fmt.Println()
	fmt.Println("vget server install is only supported on Linux with systemd.")
	fmt.Println()
	fmt.Println("To run vget as a service on macOS, see:")
	fmt.Println("https://github.com/guiyumin/vget/blob/main/docs/manual-service-setup.md")
	fmt.Println()
}

func printSuccessBox(cfg installConfig) {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2)

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("248"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	var content strings.Builder
	content.WriteString(successStyle.Render("✓ vget service installed successfully!"))
	content.WriteString("\n\n")
	content.WriteString(labelStyle.Render("WebUI:    "))
	content.WriteString(valueStyle.Render(fmt.Sprintf("http://localhost:%d", cfg.Port)))
	content.WriteString("\n")
	content.WriteString(labelStyle.Render("Status:   "))
	content.WriteString(valueStyle.Render("sudo systemctl status vget"))
	content.WriteString("\n")
	content.WriteString(labelStyle.Render("Logs:     "))
	content.WriteString(valueStyle.Render("sudo journalctl -u vget -f"))
	content.WriteString("\n")
	content.WriteString(labelStyle.Render("Stop:     "))
	content.WriteString(valueStyle.Render("sudo systemctl stop vget"))
	content.WriteString("\n")
	content.WriteString(labelStyle.Render("Remove:   "))
	content.WriteString(valueStyle.Render("sudo vget server uninstall"))

	fmt.Println(boxStyle.Render(content.String()))
}

// ============================================================================
// TUI Model for interactive installation
// ============================================================================

type installModel struct {
	step      int // 0: overview, 1: configure, 2: installing
	cursor    int
	config    installConfig
	confirmed bool
	cancelled bool
	editing   bool
	editField int
	editBuf   string
	width     int
	height    int
}

func initialInstallModel() installModel {
	return installModel{
		step:   0,
		cursor: 1, // Default to "Install"
		config: installConfig{
			Port:      defaultServicePort,
			OutputDir: defaultServiceOutput,
			User:      defaultServiceUser,
		},
	}
}

func (m installModel) Init() tea.Cmd {
	return nil
}

func (m installModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.editing {
			return m.handleEditInput(msg)
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			if m.step == 1 {
				m.step = 0
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit

		case "left", "h":
			if m.step == 0 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "right", "l":
			if m.step == 0 && m.cursor < 2 {
				m.cursor++
			}
			return m, nil

		case "up", "k":
			if m.step == 1 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.step == 1 && m.cursor < 3 {
				m.cursor++
			}
			return m, nil

		case "enter":
			return m.handleEnter()
		}
	}
	return m, nil
}

func (m installModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // Overview screen
		switch m.cursor {
		case 0: // Configure
			m.step = 1
			m.cursor = 0
		case 1: // Install
			m.confirmed = true
			return m, tea.Quit
		case 2: // Cancel
			m.cancelled = true
			return m, tea.Quit
		}
	case 1: // Configure screen
		switch m.cursor {
		case 0, 1, 2: // Edit fields
			m.editing = true
			m.editField = m.cursor
			switch m.cursor {
			case 0:
				m.editBuf = strconv.Itoa(m.config.Port)
			case 1:
				m.editBuf = m.config.OutputDir
			case 2:
				m.editBuf = m.config.User
			}
		case 3: // Back & Save
			m.step = 0
			m.cursor = 1
		}
	}
	return m, nil
}

func (m installModel) handleEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Save the edit
		switch m.editField {
		case 0:
			if port, err := strconv.Atoi(m.editBuf); err == nil && port > 0 && port < 65536 {
				m.config.Port = port
			}
		case 1:
			if m.editBuf != "" {
				m.config.OutputDir = m.editBuf
			}
		case 2:
			if m.editBuf != "" {
				m.config.User = m.editBuf
			}
		}
		m.editing = false
		return m, nil

	case "esc":
		m.editing = false
		return m, nil

	case "backspace":
		if len(m.editBuf) > 0 {
			m.editBuf = m.editBuf[:len(m.editBuf)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 {
			m.editBuf += msg.String()
		}
		return m, nil
	}
}

func (m installModel) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	checkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("248"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	var content strings.Builder

	switch m.step {
	case 0: // Overview
		content.WriteString(titleStyle.Render("vget service installer"))
		content.WriteString("\n\n")
		content.WriteString("This will install vget as a system service:\n\n")
		content.WriteString(checkStyle.Render("✓") + " Copy binary to /usr/local/bin/vget\n")
		content.WriteString(checkStyle.Render("✓") + " Create systemd service at /etc/systemd/system/\n")
		content.WriteString(checkStyle.Render("✓") + " Enable auto-start on boot\n")
		content.WriteString(checkStyle.Render("✓") + " Start the vget server\n")
		content.WriteString("\n")
		content.WriteString("Service configuration:\n")
		content.WriteString(labelStyle.Render("  Port:        "))
		content.WriteString(valueStyle.Render(strconv.Itoa(m.config.Port)))
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("  Output dir:  "))
		content.WriteString(valueStyle.Render(m.config.OutputDir))
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("  Run as user: "))
		content.WriteString(valueStyle.Render(m.config.User))
		content.WriteString("\n\n")

		// Buttons
		buttons := []string{"Configure", "Install", "Cancel"}
		for i, btn := range buttons {
			if i == m.cursor {
				content.WriteString(selectedStyle.Render("[ " + btn + " ]"))
			} else {
				content.WriteString(unselectedStyle.Render("[ " + btn + " ]"))
			}
			content.WriteString("  ")
		}

	case 1: // Configure
		content.WriteString(titleStyle.Render("Service Configuration"))
		content.WriteString("\n\n")

		fields := []struct {
			label string
			value string
		}{
			{"Port", strconv.Itoa(m.config.Port)},
			{"Output directory", m.config.OutputDir},
			{"Run as user", m.config.User},
		}

		for i, field := range fields {
			if m.cursor == i {
				content.WriteString(selectedStyle.Render("> "))
			} else {
				content.WriteString("  ")
			}
			content.WriteString(labelStyle.Render(field.label + ": "))
			if m.editing && m.editField == i {
				content.WriteString(valueStyle.Render(m.editBuf))
				content.WriteString(selectedStyle.Render("█"))
			} else {
				content.WriteString(valueStyle.Render(field.value))
			}
			content.WriteString("\n")
		}

		content.WriteString("\n")
		if m.cursor == 3 {
			content.WriteString(selectedStyle.Render("[ Back & Save ]"))
		} else {
			content.WriteString(unselectedStyle.Render("[ Back & Save ]"))
		}
	}

	box := boxStyle.Render(content.String())

	// Help
	var help string
	if m.editing {
		help = helpStyle.Render("enter: save • esc: cancel")
	} else if m.step == 0 {
		help = helpStyle.Render("←→: select • enter: confirm • esc: quit")
	} else {
		help = helpStyle.Render("↑↓: select • enter: edit • esc: back")
	}

	result := box + "\n" + help

	if m.width > 0 && m.height > 0 {
		result = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, result)
	}

	return result
}
