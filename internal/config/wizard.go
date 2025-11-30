package config

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const asciiArt = `
 ██╗   ██╗ ██████╗ ███████╗████████╗
 ██║   ██║██╔════╝ ██╔════╝╚══██╔══╝
 ██║   ██║██║  ███╗█████╗     ██║
 ╚██╗ ██╔╝██║   ██║██╔══╝     ██║
  ╚████╔╝ ╚██████╔╝███████╗   ██║
   ╚═══╝   ╚═════╝ ╚══════╝   ╚═╝
`

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	stepStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	unselectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cursorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	inputStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	inputCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	reviewStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	labelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(14)
	valueStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	containerStyle   = lipgloss.NewStyle().Padding(2, 4)
)

type step struct {
	title       string
	description string
	options     []option
	isInput     bool
	inputValue  *string
	placeholder string
}

type option struct {
	label string
	value string
}

type model struct {
	steps        []step
	currentStep  int
	cursor       int
	config       *Config
	confirmed    bool
	cancelled    bool
	inputMode    bool
	inputBuffer  string
	inputCursor  int
	width        int
	height       int
}

func initialModel(cfg *Config) model {
	steps := []step{
		{
			title:       "Language",
			description: "Preferred language for metadata",
			options: []option{
				{"English", "en"},
				{"中文", "zh"},
				{"日本語", "ja"},
				{"한국어", "ko"},
				{"Español", "es"},
				{"Français", "fr"},
				{"Deutsch", "de"},
			},
		},
		{
			title:       "Proxy",
			description: "Leave empty for no proxy",
			isInput:     true,
			inputValue:  &cfg.Proxy,
			placeholder: "http://127.0.0.1:7890",
		},
		{
			title:       "Output Directory",
			description: "Where to save downloaded videos",
			isInput:     true,
			inputValue:  &cfg.OutputDir,
			placeholder: ".",
		},
		{
			title:       "Format",
			description: "Preferred video format",
			options: []option{
				{"MP4 (recommended)", "mp4"},
				{"WebM", "webm"},
				{"MKV", "mkv"},
				{"Best available", "best"},
			},
		},
		{
			title:       "Quality",
			description: "Preferred video quality",
			options: []option{
				{"Best available", "best"},
				{"4K (2160p)", "2160p"},
				{"1080p", "1080p"},
				{"720p", "720p"},
				{"480p", "480p"},
			},
		},
		{
			title:       "Confirm",
			description: "Review and save configuration",
			options: []option{
				{"Yes, save", "yes"},
				{"No, cancel", "no"},
			},
		},
	}

	m := model{
		steps:       steps,
		currentStep: 0,
		cursor:      0,
		config:      cfg,
	}

	// Set initial cursor positions based on current config values
	m.setCursorFromConfig()

	return m
}

func (m *model) setCursorFromConfig() {
	step := m.steps[m.currentStep]
	if step.isInput {
		m.inputBuffer = *step.inputValue
		m.inputCursor = len(m.inputBuffer)
		return
	}

	var currentValue string
	switch m.currentStep {
	case 0:
		currentValue = m.config.Language
	case 3:
		currentValue = m.config.Format
	case 4:
		currentValue = m.config.Quality
	}

	for i, opt := range step.options {
		if opt.value == currentValue {
			m.cursor = i
			break
		}
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		step := m.steps[m.currentStep]

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "left":
			if m.currentStep > 0 {
				m.saveCurrentValue()
				m.currentStep--
				m.cursor = 0
				m.setCursorFromConfig()
			}
			return m, nil

		case "right", "enter":
			if step.isInput {
				*step.inputValue = m.inputBuffer
			}
			m.saveCurrentValue()

			if m.currentStep == len(m.steps)-1 {
				// Confirmation step
				if m.cursor == 0 {
					m.confirmed = true
				} else {
					m.cancelled = true
				}
				return m, tea.Quit
			}

			m.currentStep++
			m.cursor = 0
			m.setCursorFromConfig()
			return m, nil

		case "up", "k":
			if !step.isInput && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if !step.isInput && m.cursor < len(step.options)-1 {
				m.cursor++
			}
			return m, nil

		case "backspace":
			if step.isInput && len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
			return m, nil

		default:
			if step.isInput && len(msg.String()) == 1 {
				m.inputBuffer += msg.String()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *model) saveCurrentValue() {
	step := m.steps[m.currentStep]
	if step.isInput {
		return
	}

	if m.cursor < len(step.options) {
		value := step.options[m.cursor].value
		switch m.currentStep {
		case 0:
			m.config.Language = value
		case 3:
			m.config.Format = value
		case 4:
			m.config.Quality = value
		}
	}
}

func (m model) View() string {
	var b strings.Builder

	// Progress indicator
	progress := fmt.Sprintf("Step %d of %d", m.currentStep+1, len(m.steps))
	b.WriteString(stepStyle.Render(progress))
	b.WriteString("\n\n")

	step := m.steps[m.currentStep]

	// Title
	b.WriteString(titleStyle.Render(step.title))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render(step.description))
	b.WriteString("\n\n")

	// Content
	if m.currentStep == len(m.steps)-1 {
		// Review step
		b.WriteString(m.renderReview())
		b.WriteString("\n")
	}

	if step.isInput {
		// Input field
		display := m.inputBuffer
		if display == "" {
			display = stepStyle.Render(step.placeholder)
		}
		b.WriteString(inputCursorStyle.Render("> "))
		b.WriteString(inputStyle.Render(display))
		b.WriteString(inputCursorStyle.Render("█"))
		b.WriteString("\n")
	} else {
		// Options
		for i, opt := range step.options {
			cursor := "  "
			style := unselectedStyle
			if i == m.cursor {
				cursor = cursorStyle.Render("> ")
				style = selectedStyle
			}
			b.WriteString(cursor)
			b.WriteString(style.Render(opt.label))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("← back • → next • ↑↓ select • enter confirm • esc quit"))

	// Apply padding
	content := containerStyle.Render(b.String())

	// Make it fullscreen
	if m.width > 0 && m.height > 0 {
		content = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
	}

	return content
}

func (m model) renderReview() string {
	var b strings.Builder

	proxy := m.config.Proxy
	if proxy == "" {
		proxy = "(none)"
	}
	outputDir := m.config.OutputDir
	if outputDir == "" {
		outputDir = "."
	}

	lines := []struct {
		label string
		value string
	}{
		{"Language", getLanguageName(m.config.Language)},
		{"Proxy", proxy},
		{"Output Dir", outputDir},
		{"Format", m.config.Format},
		{"Quality", m.config.Quality},
	}

	for _, line := range lines {
		b.WriteString(labelStyle.Render(line.label + ":"))
		b.WriteString(valueStyle.Render(line.value))
		b.WriteString("\n")
	}

	return b.String()
}

// RunInitWizard runs an interactive TUI wizard to configure vget
func RunInitWizard() (*Config, error) {
	// Show ASCII art banner
	fmt.Print("\033[36m") // Cyan color
	fmt.Print(asciiArt)
	fmt.Print("\033[0m") // Reset color
	fmt.Println("  A modern, blazing-fast, cross-platform downloader cli")
	fmt.Println()
	time.Sleep(1 * time.Second)

	// Load existing config or use defaults
	cfg := LoadOrDefault()

	m := initialModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(model)
	if result.cancelled {
		return nil, fmt.Errorf("configuration cancelled")
	}

	// Set defaults for empty values
	if result.config.OutputDir == "" {
		result.config.OutputDir = "."
	}

	return result.config, nil
}

func getLanguageName(code string) string {
	names := map[string]string{
		"en": "English",
		"zh": "中文",
		"ja": "日本語",
		"ko": "한국어",
		"es": "Español",
		"fr": "Français",
		"de": "Deutsch",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}
