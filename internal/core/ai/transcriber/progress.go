//go:build !cgo

package transcriber

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))  // cyan
	fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))            // pink
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))  // green
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))            // gray
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))            // white
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))            // gray
	errStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")) // red
)

// transcribeState holds the shared transcription state for TUI
type transcribeState struct {
	mu        sync.RWMutex
	progress  float64 // 0-100
	stage     string  // "extracting", "transcribing", "filtering"
	done      bool
	err       error
	startTime time.Time
	endTime   time.Time
}

func newTranscribeState() *transcribeState {
	return &transcribeState{
		startTime: time.Now(),
		stage:     "preparing",
	}
}

func (s *transcribeState) setProgress(p float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = p
}

func (s *transcribeState) setStage(stage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stage = stage
}

func (s *transcribeState) setDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endTime = time.Now()
	s.done = true
}

func (s *transcribeState) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	s.done = true
}

func (s *transcribeState) get() (float64, string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress, s.stage, s.done, s.err
}

func (s *transcribeState) elapsed() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.endTime.IsZero() {
		return time.Since(s.startTime)
	}
	return s.endTime.Sub(s.startTime)
}

// tickMsg triggers UI updates
type tickMsg time.Time

// transcribeModel is the Bubble Tea model for transcription progress
type transcribeModel struct {
	progress progress.Model
	spinner  spinner.Model

	filename string
	model    string
	state    *transcribeState
}

func newTranscribeModel(filename, model string, state *transcribeState) transcribeModel {
	// Progress bar with gradient
	p := progress.New(
		progress.WithScaledGradient("#FF6B6B", "#4ECDC4"),
		progress.WithWidth(50),
	)

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return transcribeModel{
		progress: p,
		spinner:  s,
		filename: filename,
		model:    model,
		state:    state,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m transcribeModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

func (m transcribeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		prog, _, done, err := m.state.get()
		if err != nil || done {
			return m, tea.Quit
		}

		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())

		cmd := m.progress.SetPercent(prog / 100)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m transcribeModel) View() string {
	prog, stage, done, err := m.state.get()
	elapsed := m.state.elapsed()

	if err != nil {
		return fmt.Sprintf("\n  %s Transcription failed: %v\n\n",
			errStyle.Render("✗"),
			err,
		)
	}

	if done {
		return fmt.Sprintf("\n  %s %s\n  %s %s\n  %s %s\n\n",
			successStyle.Render("✓"),
			titleStyle.Render("Transcription complete"),
			labelStyle.Render("Elapsed:"),
			valueStyle.Render(formatDurationTUI(elapsed)),
			labelStyle.Render("Model:"),
			valueStyle.Render(m.model),
		)
	}

	var s string
	s += "\n"

	// Header with spinner
	stageIcon := m.spinner.View()
	stageText := getStageText(stage)
	s += fmt.Sprintf("  %s %s\n", stageIcon, titleStyle.Render(stageText))

	// File info
	s += fmt.Sprintf("  %s %s\n", labelStyle.Render("File:"), fileStyle.Render(m.filename))
	s += fmt.Sprintf("  %s %s\n\n", labelStyle.Render("Model:"), valueStyle.Render(m.model))

	// Progress bar
	s += fmt.Sprintf("  %s\n\n", m.progress.View())

	// Stats
	s += fmt.Sprintf("  %s %.0f%%  %s  %s %s\n",
		labelStyle.Render("Progress:"),
		prog,
		labelStyle.Render("│"),
		labelStyle.Render("Elapsed:"),
		valueStyle.Render(formatDurationTUI(elapsed)),
	)

	s += "\n"
	s += helpStyle.Render("  Press q to cancel")
	s += "\n"

	return s
}

func getStageText(stage string) string {
	switch stage {
	case "extracting":
		return "Extracting audio..."
	case "converting":
		return "Converting audio..."
	case "transcribing":
		return "Transcribing speech..."
	case "filtering":
		return "Filtering results..."
	default:
		return "Preparing..."
	}
}

func formatDurationTUI(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// TUIProgressReporter wraps ProgressReporter with TUI state for bubbletea.
type TUIProgressReporter struct {
	*ProgressReporter
	state *transcribeState
}

// NewTUIProgressReporter creates a new TUI-enabled progress reporter.
func NewTUIProgressReporter() *TUIProgressReporter {
	state := newTranscribeState()
	return &TUIProgressReporter{
		ProgressReporter: &ProgressReporter{
			progressFn: state.setProgress,
			stageFn:    state.setStage,
			doneFn:     state.setDone,
			errorFn:    state.setError,
		},
		state: state,
	}
}

// RunTranscribeTUI runs the transcription progress TUI.
func RunTranscribeTUI(filename, model string, reporter *TUIProgressReporter) error {
	m := newTranscribeModel(filename, model, reporter.state)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
