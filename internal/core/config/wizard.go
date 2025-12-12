package config

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/core/i18n"
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
	logoStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	stepStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	selectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	unselectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cursorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	inputStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	inputCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	labelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Width(14)
	valueStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	containerStyle   = lipgloss.NewStyle().Padding(2, 4)
)

type model struct {
	currentStep int
	cursor      int
	config      *Config
	confirmed   bool
	cancelled   bool
	inputBuffer string
	inputCursor int
	width       int
	height      int
}

func initialModel(cfg *Config) model {
	m := model{
		currentStep: 0,
		cursor:      0,
		config:      cfg,
	}

	// Set initial cursor position for language
	m.setCursorFromConfig()

	return m
}

func (m *model) t() *i18n.Translations {
	return i18n.GetTranslations(m.config.Language)
}

func (m *model) getStepTitle() string {
	t := m.t()
	switch m.currentStep {
	case 0:
		return t.Config.Language
	case 1:
		return t.Config.OutputDir
	case 2:
		return t.Config.Format
	case 3:
		return t.Config.Quality
	case 4:
		return t.Config.Confirm
	}
	return ""
}

func (m *model) getStepDescription() string {
	t := m.t()
	switch m.currentStep {
	case 0:
		return t.Config.LanguageDesc
	case 1:
		return t.Config.OutputDirDesc
	case 2:
		return t.Config.FormatDesc
	case 3:
		return t.Config.QualityDesc
	case 4:
		return t.Config.ConfirmDesc
	}
	return ""
}

func (m *model) getOptions() []struct{ label, value string } {
	t := m.t()
	switch m.currentStep {
	case 0:
		opts := make([]struct{ label, value string }, len(i18n.SupportedLanguages))
		for i, lang := range i18n.SupportedLanguages {
			opts[i] = struct{ label, value string }{lang.Name, lang.Code}
		}
		return opts
	case 2:
		return []struct{ label, value string }{
			{"MP4 " + t.Config.Recommended, "mp4"},
			{"WebM", "webm"},
			{"MKV", "mkv"},
			{t.Config.BestAvailable, "best"},
		}
	case 3:
		return []struct{ label, value string }{
			{t.Config.BestAvailable, "best"},
			{"4K (2160p)", "2160p"},
			{"1080p", "1080p"},
			{"720p", "720p"},
			{"480p", "480p"},
		}
	case 4:
		return []struct{ label, value string }{
			{t.Config.YesSave, "yes"},
			{t.Config.NoCancel, "no"},
		}
	}
	return nil
}

func (m *model) isInputStep() bool {
	return m.currentStep == 1 // Only output dir is input step now
}

func (m *model) setCursorFromConfig() {
	if m.isInputStep() {
		switch m.currentStep {
		case 1:
			if m.config.OutputDir != "" {
				m.inputBuffer = m.config.OutputDir
			} else {
				m.inputBuffer = DefaultDownloadDir()
			}
		}
		m.inputCursor = len(m.inputBuffer)
		return
	}

	var currentValue string
	switch m.currentStep {
	case 0:
		currentValue = m.config.Language
	case 2:
		currentValue = m.config.Format
	case 3:
		currentValue = m.config.Quality
	}

	options := m.getOptions()
	for i, opt := range options {
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
			m.saveCurrentValue()

			if m.currentStep == 4 {
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
			if !m.isInputStep() {
				options := m.getOptions()
				if m.cursor > 0 {
					m.cursor--
				} else {
					m.cursor = len(options) - 1
				}
			}
			return m, nil

		case "down", "j":
			if !m.isInputStep() {
				options := m.getOptions()
				if m.cursor < len(options)-1 {
					m.cursor++
				} else {
					m.cursor = 0
				}
			}
			return m, nil

		case "backspace":
			if m.isInputStep() && len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
			return m, nil

		default:
			if m.isInputStep() && len(msg.String()) == 1 {
				m.inputBuffer += msg.String()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *model) saveCurrentValue() {
	if m.isInputStep() {
		switch m.currentStep {
		case 1:
			m.config.OutputDir = m.inputBuffer
		}
		return
	}

	options := m.getOptions()
	if m.cursor < len(options) {
		value := options[m.cursor].value
		switch m.currentStep {
		case 0:
			m.config.Language = value
		case 2:
			m.config.Format = value
		case 3:
			m.config.Quality = value
		}
	}
}

func (m model) View() string {
	var b strings.Builder
	t := m.t()

	// Logo
	b.WriteString(logoStyle.Render(asciiArt))
	b.WriteString("\n\n")

	// Progress indicator
	progress := fmt.Sprintf(t.Config.StepOf, m.currentStep+1, 5)
	b.WriteString(stepStyle.Render(progress))
	b.WriteString("\n\n")

	// Title
	b.WriteString(titleStyle.Render(m.getStepTitle()))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render(m.getStepDescription()))
	b.WriteString("\n\n")

	// Content
	if m.currentStep == 4 {
		// Review step
		b.WriteString(m.renderReview())
		b.WriteString("\n")
	}

	if m.isInputStep() {
		// Input field
		b.WriteString(inputCursorStyle.Render("> "))
		b.WriteString(inputStyle.Render(m.inputBuffer))
		b.WriteString(inputCursorStyle.Render("█"))
		b.WriteString("\n")
	} else {
		// Options
		options := m.getOptions()
		for i, opt := range options {
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
	help := fmt.Sprintf("← %s • → %s • ↑↓ %s • enter %s • esc %s",
		t.Help.Back, t.Help.Next, t.Help.Select, t.Help.Confirm, t.Help.Quit)
	b.WriteString(helpStyle.Render(help))

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
	t := m.t()

	outputDir := m.config.OutputDir
	if outputDir == "" {
		outputDir = DefaultDownloadDir()
	}

	lines := []struct {
		label string
		value string
	}{
		{t.ConfigReview.Language, getLanguageName(m.config.Language)},
		{t.ConfigReview.OutputDir, outputDir},
		{t.ConfigReview.Format, m.config.Format},
		{t.ConfigReview.Quality, m.config.Quality},
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
		result.config.OutputDir = DefaultDownloadDir()
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
