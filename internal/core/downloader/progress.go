package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/core/i18n"
)

var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	doneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// downloadState holds the shared download state
type downloadState struct {
	mu          sync.RWMutex
	current     int64
	total       int64
	speed       float64
	done        bool
	err         error
	startTime   time.Time
	endTime     time.Time
	finalSpeed  float64
	finalPath   string
}

func (s *downloadState) update(current, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = current
	s.total = total
	elapsed := time.Since(s.startTime).Seconds()
	if elapsed > 0 {
		s.speed = float64(current) / elapsed
	}
}

func (s *downloadState) setDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endTime = time.Now()
	elapsed := s.endTime.Sub(s.startTime).Seconds()
	if elapsed > 0 {
		s.finalSpeed = float64(s.current) / elapsed
	}
	s.done = true
}

func (s *downloadState) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	s.done = true
}

func (s *downloadState) setFinalPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finalPath = path
}

func (s *downloadState) getFinalPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.finalPath
}

func (s *downloadState) get() (int64, int64, float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current, s.total, s.speed, s.done, s.err
}

func (s *downloadState) getFinal() (elapsed time.Duration, avgSpeed float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.endTime.IsZero() {
		return time.Since(s.startTime), s.speed
	}
	return s.endTime.Sub(s.startTime), s.finalSpeed
}

// tickMsg triggers UI updates
type tickMsg time.Time

// downloadDoneMsg signals download completion
type downloadDoneMsg struct{}

// downloadModel is the Bubble Tea model for download progress
type downloadModel struct {
	progress progress.Model
	spinner  spinner.Model
	t        *i18n.Translations

	output  string
	videoID string

	state *downloadState
}

func newDownloadModel(output, videoID, lang string, state *downloadState) downloadModel {
	// Progress bar with gradient
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
	)

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return downloadModel{
		progress: p,
		spinner:  s,
		t:        i18n.T(lang),
		output:   output,
		videoID:  videoID,
		state:    state,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m downloadModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

func (m downloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case tickMsg:
		current, total, _, done, err := m.state.get()
		if err != nil || done {
			return m, tea.Quit
		}

		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())

		if total > 0 {
			cmd := m.progress.SetPercent(float64(current) / float64(total))
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)

	case downloadDoneMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m downloadModel) View() string {
	current, total, speed, done, err := m.state.get()

	if err != nil {
		return fmt.Sprintf("\n  %s %s: %v\n\n",
			errStyle.Render("✗"),
			m.t.Download.Failed,
			err,
		)
	}

	if done {
		elapsed, avgSpeed := m.state.getFinal()
		// Display full path
		displayPath := m.output
		if finalPath := m.state.getFinalPath(); finalPath != "" {
			displayPath = finalPath
		}
		if absPath, err := filepath.Abs(displayPath); err == nil {
			displayPath = absPath
		}
		return fmt.Sprintf("\n  %s %s\n  %s: %s (%s)\n  %s: %s  |  %s: %s/s\n\n",
			doneStyle.Render("✓"),
			m.t.Download.Completed,
			m.t.Download.FileSaved,
			displayPath,
			formatBytes(current),
			m.t.Download.Elapsed,
			formatDuration(elapsed),
			m.t.Download.AvgSpeed,
			formatBytes(int64(avgSpeed)),
		)
	}

	var s string
	s += "\n"

	// Video ID with spinner
	s += fmt.Sprintf("  %s %s: %s\n\n",
		m.spinner.View(),
		m.t.Download.Downloading,
		infoStyle.Render(m.videoID),
	)

	// Progress bar
	s += fmt.Sprintf("  %s\n\n", m.progress.View())

	// Stats
	if total > 0 {
		percent := float64(current) / float64(total) * 100
		eta := calculateETA(total-current, speed)
		s += fmt.Sprintf("  %s: %.1f%%  |  %s/%s  |  %s: %s/s  |  %s: %s\n",
			m.t.Download.Progress,
			percent,
			formatBytes(current),
			formatBytes(total),
			m.t.Download.Speed,
			formatBytes(int64(speed)),
			m.t.Download.ETA,
			eta,
		)
	} else {
		s += fmt.Sprintf("  %s  |  %s: %s/s\n",
			formatBytes(current),
			m.t.Download.Speed,
			formatBytes(int64(speed)),
		)
	}

	s += "\n"
	s += helpStyle.Render("  Press q to cancel")
	s += "\n"

	return s
}

func calculateETA(remaining int64, speed float64) string {
	if speed <= 0 {
		return "??:??"
	}
	eta := time.Duration(float64(remaining)/speed) * time.Second
	return formatDuration(eta)
}

// RunDownloadTUI runs the download with a TUI progress display
func RunDownloadTUI(url, output, videoID, lang string, headers map[string]string) error {
	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	state := &downloadState{
		startTime: time.Now(),
	}

	// Start download in background
	go func() {
		err := downloadWithProgress(client, url, output, state, headers)
		if err != nil {
			state.setError(err)
		} else {
			state.setDone()
		}
	}()

	model := newDownloadModel(output, videoID, lang, state)

	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m := finalModel.(downloadModel)
	_, _, _, _, downloadErr := m.state.get()
	if downloadErr != nil {
		return downloadErr
	}

	return nil
}

func downloadWithProgress(client *http.Client, url, output string, state *downloadState, headers map[string]string) error {
	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Use custom headers if provided, otherwise use generic browser headers
	if len(headers) > 0 {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	} else {
		// Generic browser headers as default
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := resp.ContentLength
	state.update(0, total)

	// Create output file
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Download with progress tracking
	buf := make([]byte, 32*1024)
	var current int64

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			current += int64(n)
			state.update(current, total)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	// Close file and rename by magic bytes if needed
	file.Close()
	state.setFinalPath(RenameByMagicBytes(output))

	return nil
}

// RunDownloadFromReaderTUI runs the download from a reader with a TUI progress display
func RunDownloadFromReaderTUI(reader io.ReadCloser, size int64, output, displayID, lang string) error {
	state := &downloadState{
		startTime: time.Now(),
	}

	// Start download in background
	go func() {
		err := downloadFromReaderWithProgress(reader, size, output, state)
		if err != nil {
			state.setError(err)
		} else {
			state.setDone()
		}
	}()

	model := newDownloadModel(output, displayID, lang, state)

	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m := finalModel.(downloadModel)
	_, _, _, _, downloadErr := m.state.get()
	if downloadErr != nil {
		return downloadErr
	}

	return nil
}

func downloadFromReaderWithProgress(reader io.ReadCloser, total int64, output string, state *downloadState) error {
	defer reader.Close()

	state.update(0, total)

	// Create output file
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Download with progress tracking
	buf := make([]byte, 32*1024)
	var current int64

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			current += int64(n)
			state.update(current, total)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	// Close file and rename by magic bytes if needed
	file.Close()
	state.setFinalPath(RenameByMagicBytes(output))

	return nil
}

// RunMultiStreamDownloadWithAuthCallback runs a multi-stream download with auth and progress callback (for server use)
func RunMultiStreamDownloadWithAuthCallback(ctx context.Context, url, authHeader, output string, totalSize int64, config MultiStreamConfig, progressFn func(downloaded, total int64)) error {
	state := &downloadState{
		startTime: time.Now(),
	}

	// Start a goroutine to forward progress updates to the callback
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				current, total, _, _, _ := state.get()
				if progressFn != nil {
					progressFn(current, total)
				}
			}
		}
	}()

	err := MultiStreamDownloadWithAuth(ctx, url, authHeader, output, totalSize, config, state)
	close(done)

	// Final progress update
	if progressFn != nil {
		current, total, _, _, _ := state.get()
		progressFn(current, total)
	}

	return err
}
