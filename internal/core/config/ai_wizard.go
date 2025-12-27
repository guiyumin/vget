package config

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/core/crypto"
	"github.com/guiyumin/vget/internal/core/i18n"
)

const aiAsciiArt = `
 ██╗   ██╗ ██████╗ ███████╗████████╗     █████╗ ██╗
 ██║   ██║██╔════╝ ██╔════╝╚══██╔══╝    ██╔══██╗██║
 ██║   ██║██║  ███╗█████╗     ██║       ███████║██║
 ╚██╗ ██╔╝██║   ██║██╔══╝     ██║       ██╔══██║██║
  ╚████╔╝ ╚██████╔╝███████╗   ██║       ██║  ██║██║
   ╚═══╝   ╚═════╝ ╚══════╝   ╚═╝       ╚═╝  ╚═╝╚═╝
`

// AI wizard step constants
const (
	aiStepAccountName = iota
	aiStepProvider
	aiStepTranscriptionAPIKey
	aiStepSummarizationAPIKey
	aiStepPIN
	aiStepPINConfirm
	aiStepReview
)

type aiModel struct {
	currentStep int
	cursor      int
	config      *Config
	confirmed   bool
	cancelled   bool
	inputBuffer string
	width       int
	height      int

	// New account being created
	accountName       string
	provider          string
	transcriptionKey  string
	summarizationKey  string
	pin               string
	pinConfirm        string
	reuseKey          bool
	errorMsg          string
}

func initialAIModel(cfg *Config) aiModel {
	return aiModel{
		currentStep: aiStepAccountName,
		cursor:      0,
		config:      cfg,
		accountName: "personal", // default name
	}
}

func (m *aiModel) t() *i18n.Translations {
	return i18n.GetTranslations(m.config.Language)
}

func (m *aiModel) getStepTitle() string {
	switch m.currentStep {
	case aiStepAccountName:
		return "Account Name"
	case aiStepProvider:
		return "Provider"
	case aiStepTranscriptionAPIKey:
		return "Transcription API Key"
	case aiStepSummarizationAPIKey:
		return "Summarization API Key"
	case aiStepPIN:
		return "4-Digit PIN"
	case aiStepPINConfirm:
		return "Confirm PIN"
	case aiStepReview:
		return "Review & Save"
	}
	return ""
}

func (m *aiModel) getStepDescription() string {
	switch m.currentStep {
	case aiStepAccountName:
		return "Enter a name for this AI account (e.g., personal, work)"
	case aiStepProvider:
		return "Choose an AI provider"
	case aiStepTranscriptionAPIKey:
		return "Enter your OpenAI API key for transcription (Whisper)"
	case aiStepSummarizationAPIKey:
		if m.reuseKey {
			return "Use same API key for summarization?"
		}
		switch m.provider {
		case "anthropic":
			return "Enter your Anthropic API key for summarization (Claude)"
		case "qwen":
			return "Enter your DashScope API key for summarization (Qwen)"
		default:
			return "Enter your API key for summarization (GPT)"
		}
	case aiStepPIN:
		return "Enter a 4-digit PIN to encrypt your API keys"
	case aiStepPINConfirm:
		return "Confirm your 4-digit PIN"
	case aiStepReview:
		return "Review your AI configuration"
	}
	return ""
}

func (m *aiModel) getOptions() []struct{ label, value string } {
	switch m.currentStep {
	case aiStepProvider:
		return []struct{ label, value string }{
			{"OpenAI (Recommended - supports transcription + summarization)", "openai"},
			{"Anthropic Claude (summarization only)", "anthropic"},
			{"Alibaba Qwen (summarization only)", "qwen"},
			{"Cancel", ""},
		}
	case aiStepSummarizationAPIKey:
		if m.reuseKey {
			return []struct{ label, value string }{
				{"Yes, use same API key", "yes"},
				{"No, enter different key", "no"},
			}
		}
		return nil
	case aiStepReview:
		return []struct{ label, value string }{
			{"Yes, save configuration", "yes"},
			{"No, cancel", "no"},
		}
	}
	return nil
}

func (m *aiModel) isInputStep() bool {
	switch m.currentStep {
	case aiStepAccountName, aiStepTranscriptionAPIKey, aiStepPIN, aiStepPINConfirm:
		return true
	case aiStepSummarizationAPIKey:
		return !m.reuseKey
	}
	return false
}

func (m aiModel) Init() tea.Cmd {
	return nil
}

func (m aiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Clear error on any key press
		m.errorMsg = ""

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "left":
			if m.currentStep > 0 {
				m.currentStep = m.getPreviousStep()
				m.cursor = 0
				m.loadStepValue()
			}
			return m, nil

		case "right", "enter":
			if err := m.validateAndSave(); err != nil {
				m.errorMsg = err.Error()
				return m, nil
			}

			if m.currentStep == aiStepReview {
				if m.cursor == 0 {
					m.confirmed = true
				} else {
					m.cancelled = true
				}
				return m, tea.Quit
			}

			m.currentStep = m.getNextStep()
			m.cursor = 0
			m.inputBuffer = ""
			m.loadStepValue()
			return m, nil

		case "up", "k":
			if !m.isInputStep() {
				options := m.getOptions()
				if m.cursor > 0 {
					m.cursor--
				} else if len(options) > 0 {
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
				// For PIN steps, only allow digits
				if m.currentStep == aiStepPIN || m.currentStep == aiStepPINConfirm {
					if msg.String() >= "0" && msg.String() <= "9" && len(m.inputBuffer) < 4 {
						m.inputBuffer += msg.String()
					}
				} else {
					m.inputBuffer += msg.String()
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *aiModel) loadStepValue() {
	switch m.currentStep {
	case aiStepAccountName:
		m.inputBuffer = m.accountName
	case aiStepTranscriptionAPIKey:
		m.inputBuffer = m.transcriptionKey
	case aiStepSummarizationAPIKey:
		if !m.reuseKey {
			m.inputBuffer = m.summarizationKey
		}
	case aiStepPIN:
		m.inputBuffer = m.pin
	case aiStepPINConfirm:
		m.inputBuffer = m.pinConfirm
	}
}

func (m *aiModel) validateAndSave() error {
	switch m.currentStep {
	case aiStepAccountName:
		name := strings.TrimSpace(m.inputBuffer)
		if name == "" {
			return fmt.Errorf("account name cannot be empty")
		}
		if m.config.AI.GetAccount(name) != nil {
			return fmt.Errorf("account '%s' already exists", name)
		}
		m.accountName = name

	case aiStepProvider:
		options := m.getOptions()
		if m.cursor < len(options) {
			m.provider = options[m.cursor].value
			if m.provider == "" {
				m.cancelled = true
			}
		}

	case aiStepTranscriptionAPIKey:
		key := strings.TrimSpace(m.inputBuffer)
		if key == "" {
			return fmt.Errorf("API key cannot be empty")
		}
		m.transcriptionKey = key
		// Check if we should offer to reuse key
		m.reuseKey = true

	case aiStepSummarizationAPIKey:
		if m.reuseKey {
			options := m.getOptions()
			if m.cursor < len(options) {
				if options[m.cursor].value == "yes" {
					m.summarizationKey = m.transcriptionKey
				} else {
					m.reuseKey = false
					m.inputBuffer = ""
					return nil // Don't advance, switch to input mode
				}
			}
		} else {
			key := strings.TrimSpace(m.inputBuffer)
			if key == "" {
				return fmt.Errorf("API key cannot be empty")
			}
			m.summarizationKey = key
		}

	case aiStepPIN:
		if err := crypto.ValidatePIN(m.inputBuffer); err != nil {
			return fmt.Errorf("PIN must be exactly 4 digits")
		}
		m.pin = m.inputBuffer

	case aiStepPINConfirm:
		if m.inputBuffer != m.pin {
			return fmt.Errorf("PINs do not match")
		}
		m.pinConfirm = m.inputBuffer
	}

	return nil
}

func (m *aiModel) getNextStep() int {
	switch m.currentStep {
	case aiStepAccountName:
		return aiStepProvider
	case aiStepProvider:
		if m.provider == "" {
			m.cancelled = true
			return m.currentStep
		}
		// Only OpenAI supports transcription (Whisper)
		if m.provider == "openai" {
			return aiStepTranscriptionAPIKey
		}
		// For Anthropic/Qwen, skip transcription and go straight to summarization
		m.reuseKey = false // Force entering a key since there's no transcription key
		return aiStepSummarizationAPIKey
	case aiStepTranscriptionAPIKey:
		return aiStepSummarizationAPIKey
	case aiStepSummarizationAPIKey:
		return aiStepPIN
	case aiStepPIN:
		return aiStepPINConfirm
	case aiStepPINConfirm:
		return aiStepReview
	}
	return m.currentStep + 1
}

func (m *aiModel) getPreviousStep() int {
	switch m.currentStep {
	case aiStepProvider:
		return aiStepAccountName
	case aiStepTranscriptionAPIKey:
		return aiStepProvider
	case aiStepSummarizationAPIKey:
		// Only OpenAI has a transcription step to go back to
		if m.provider == "openai" {
			m.reuseKey = true // Reset when going back
			return aiStepTranscriptionAPIKey
		}
		return aiStepProvider
	case aiStepPIN:
		return aiStepSummarizationAPIKey
	case aiStepPINConfirm:
		return aiStepPIN
	case aiStepReview:
		return aiStepPINConfirm
	}
	return m.currentStep - 1
}

func (m aiModel) View() string {
	var b strings.Builder
	t := m.t()

	// Logo
	b.WriteString(logoStyle.Render(aiAsciiArt))
	b.WriteString("\n\n")

	// Progress indicator
	totalSteps := 7
	progress := fmt.Sprintf(t.Config.StepOf, m.currentStep+1, totalSteps)
	b.WriteString(stepStyle.Render(progress))
	b.WriteString("\n\n")

	// Title
	b.WriteString(titleStyle.Render(m.getStepTitle()))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render(m.getStepDescription()))
	b.WriteString("\n\n")

	// Error message
	if m.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		b.WriteString(errorStyle.Render("Error: " + m.errorMsg))
		b.WriteString("\n\n")
	}

	// Content
	if m.currentStep == aiStepReview {
		b.WriteString(m.renderReview())
		b.WriteString("\n")
	}

	if m.isInputStep() {
		// Input field
		b.WriteString(inputCursorStyle.Render("> "))

		// Mask display for sensitive fields
		displayText := m.inputBuffer
		if m.currentStep == aiStepTranscriptionAPIKey || m.currentStep == aiStepSummarizationAPIKey {
			if len(displayText) > 4 {
				displayText = displayText[:4] + strings.Repeat("*", len(displayText)-4)
			}
		} else if m.currentStep == aiStepPIN || m.currentStep == aiStepPINConfirm {
			displayText = strings.Repeat("*", len(displayText))
		}

		b.WriteString(inputStyle.Render(displayText))
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

func (m aiModel) renderReview() string {
	var b strings.Builder

	b.WriteString(labelStyle.Render("Account Name:"))
	b.WriteString(valueStyle.Render(m.accountName))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Provider:"))
	b.WriteString(valueStyle.Render(m.provider))
	b.WriteString("\n\n")

	// Only show transcription for OpenAI (other providers don't support it)
	if m.provider == "openai" {
		b.WriteString(labelStyle.Render("Transcription:"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(labelStyle.Render("Model:"))
		b.WriteString(valueStyle.Render("whisper-1"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(labelStyle.Render("API Key:"))
		masked := m.transcriptionKey[:min(4, len(m.transcriptionKey))] + "***"
		b.WriteString(valueStyle.Render(masked))
		b.WriteString("\n\n")
	}

	b.WriteString(labelStyle.Render("Summarization:"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Model:"))
	b.WriteString(valueStyle.Render(m.getDefaultSummarizationModel()))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("API Key:"))
	if m.provider == "openai" && m.summarizationKey == m.transcriptionKey {
		b.WriteString(valueStyle.Render("(same as transcription)"))
	} else {
		masked := m.summarizationKey[:min(4, len(m.summarizationKey))] + "***"
		b.WriteString(valueStyle.Render(masked))
	}
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("PIN:"))
	b.WriteString(valueStyle.Render("****"))
	b.WriteString("\n")

	return b.String()
}

func (m aiModel) getDefaultSummarizationModel() string {
	switch m.provider {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "qwen":
		return "qwen-plus"
	default:
		return "gpt-4o"
	}
}

// RunAIWizard runs an interactive TUI wizard to configure AI settings
func RunAIWizard() (*Config, error) {
	// Load existing config or use defaults
	cfg := LoadOrDefault()

	m := initialAIModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(aiModel)
	if result.cancelled {
		return nil, fmt.Errorf("configuration cancelled")
	}

	account := AIAccount{
		Provider: result.provider,
	}

	// Only OpenAI supports transcription (Whisper)
	if result.provider == "openai" && result.transcriptionKey != "" {
		transcriptionKeyEnc, err := crypto.Encrypt(result.transcriptionKey, result.pin)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt transcription key: %w", err)
		}
		account.Transcription = AIServiceConfig{
			Model:           "whisper-1",
			APIKeyEncrypted: transcriptionKeyEnc,
		}
	}

	// Encrypt summarization API key
	summarizationKeyEnc, err := crypto.Encrypt(result.summarizationKey, result.pin)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt summarization key: %w", err)
	}

	// Set summarization model based on provider
	summarizationModel := result.getDefaultSummarizationModel()
	account.Summarization = AIServiceConfig{
		Model:           summarizationModel,
		APIKeyEncrypted: summarizationKeyEnc,
	}

	result.config.AI.SetAccount(result.accountName, account)

	return result.config, nil
}
