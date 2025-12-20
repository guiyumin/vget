package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to media platforms",
	Long:  "Login to various media platforms to download member-only content",
}

var loginBilibiliCmd = &cobra.Command{
	Use:   "bilibili",
	Short: "Login to Bilibili",
	Long: `Login to Bilibili to download member-only or VIP content.

This opens an interactive TUI to paste your cookie.

To get your cookie:
  1. Open bilibili.com in browser and log in
  2. Press F12 to open DevTools
  3. Go to Application tab
  4. Find Cookies → bilibili.com
  5. Copy SESSDATA, bili_jct, DedeUserID values
  6. Format: SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBilibiliLogin()
	},
}

var loginBilibiliStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Bilibili login status",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadOrDefault()
		if cfg.Bilibili.Cookie != "" && strings.Contains(cfg.Bilibili.Cookie, "SESSDATA") {
			fmt.Println("✓ Bilibili: logged in")
		} else {
			fmt.Println("✗ Bilibili: not logged in")
		}
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from media platforms",
	Long:  "Clear saved credentials for media platforms",
}

var logoutBilibiliCmd = &cobra.Command{
	Use:   "bilibili",
	Short: "Clear Bilibili credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault()
		cfg.Bilibili.Cookie = ""
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Println("✓ Bilibili credentials cleared")
		return nil
	},
}

func init() {
	loginBilibiliCmd.AddCommand(loginBilibiliStatusCmd)
	loginCmd.AddCommand(loginBilibiliCmd)
	logoutCmd.AddCommand(logoutBilibiliCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

// Bilibili login TUI

var (
	biliTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00A1D6")) // Bilibili blue

	biliStepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	biliKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00A1D6")).
			Bold(true)

	biliHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	biliSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82"))

	biliErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

type bilibiliLoginModel struct {
	inputs    []textinput.Model
	focused   int
	saved     bool
	cancelled bool
	error     string
}

func newBilibiliLoginModel() bilibiliLoginModel {
	inputs := make([]textinput.Model, 3)

	// SESSDATA input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "粘贴 SESSDATA 值..."
	inputs[0].CharLimit = 500
	inputs[0].Width = 50
	inputs[0].Prompt = "  SESSDATA    > "
	inputs[0].PromptStyle = biliKeyStyle
	inputs[0].Focus()

	// bili_jct input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "粘贴 bili_jct 值..."
	inputs[1].CharLimit = 100
	inputs[1].Width = 50
	inputs[1].Prompt = "  bili_jct    > "
	inputs[1].PromptStyle = biliKeyStyle

	// DedeUserID input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "粘贴 DedeUserID 值..."
	inputs[2].CharLimit = 50
	inputs[2].Width = 50
	inputs[2].Prompt = "  DedeUserID  > "
	inputs[2].PromptStyle = biliKeyStyle

	// Load existing cookie if any
	cfg := config.LoadOrDefault()
	if cfg.Bilibili.Cookie != "" {
		// Parse existing cookie
		parts := strings.Split(cfg.Bilibili.Cookie, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "SESSDATA=") {
				inputs[0].SetValue(strings.TrimPrefix(part, "SESSDATA="))
			} else if strings.HasPrefix(part, "bili_jct=") {
				inputs[1].SetValue(strings.TrimPrefix(part, "bili_jct="))
			} else if strings.HasPrefix(part, "DedeUserID=") {
				inputs[2].SetValue(strings.TrimPrefix(part, "DedeUserID="))
			}
		}
	}

	return bilibiliLoginModel{
		inputs:  inputs,
		focused: 0,
	}
}

func (m bilibiliLoginModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m bilibiliLoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "tab", "down":
			// Move to next input
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % len(m.inputs)
			m.inputs[m.focused].Focus()
			return m, textinput.Blink

		case "shift+tab", "up":
			// Move to previous input
			m.inputs[m.focused].Blur()
			m.focused--
			if m.focused < 0 {
				m.focused = len(m.inputs) - 1
			}
			m.inputs[m.focused].Focus()
			return m, textinput.Blink

		case "enter":
			// If not on last field, move to next
			if m.focused < len(m.inputs)-1 {
				m.inputs[m.focused].Blur()
				m.focused++
				m.inputs[m.focused].Focus()
				return m, textinput.Blink
			}

			// Validate all fields
			sessdata := strings.TrimSpace(m.inputs[0].Value())
			biliJct := strings.TrimSpace(m.inputs[1].Value())
			dedeUserID := strings.TrimSpace(m.inputs[2].Value())

			if sessdata == "" {
				m.error = "SESSDATA 不能为空"
				m.focused = 0
				m.inputs[0].Focus()
				return m, textinput.Blink
			}

			// Build cookie string
			cookie := fmt.Sprintf("SESSDATA=%s; bili_jct=%s; DedeUserID=%s", sessdata, biliJct, dedeUserID)

			// Save to config
			cfg := config.LoadOrDefault()
			cfg.Bilibili.Cookie = cookie
			if err := config.Save(cfg); err != nil {
				m.error = fmt.Sprintf("保存失败: %v", err)
				return m, nil
			}

			m.saved = true
			return m, tea.Quit
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	cmds = append(cmds, cmd)
	m.error = "" // Clear error on any input

	return m, tea.Batch(cmds...)
}

func (m bilibiliLoginModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString("\n")
	b.WriteString(biliTitleStyle.Render("  ━━━ Bilibili 登录 ━━━"))
	b.WriteString("\n\n")

	// Instructions
	b.WriteString(biliTitleStyle.Render("  获取 Cookie 的方法："))
	b.WriteString("\n\n")
	b.WriteString(biliStepStyle.Render("  1. 在浏览器中打开 "))
	b.WriteString(biliKeyStyle.Render("bilibili.com"))
	b.WriteString(biliStepStyle.Render(" 并登录"))
	b.WriteString("\n")
	b.WriteString(biliStepStyle.Render("  2. 按 "))
	b.WriteString(biliKeyStyle.Render("F12"))
	b.WriteString(biliStepStyle.Render(" 打开开发者工具"))
	b.WriteString("\n")
	b.WriteString(biliStepStyle.Render("  3. 点击顶部「"))
	b.WriteString(biliKeyStyle.Render("Application"))
	b.WriteString(biliStepStyle.Render("」或「"))
	b.WriteString(biliKeyStyle.Render("应用"))
	b.WriteString(biliStepStyle.Render("」标签"))
	b.WriteString("\n")
	b.WriteString(biliStepStyle.Render("  4. 左侧展开 "))
	b.WriteString(biliKeyStyle.Render("Cookies"))
	b.WriteString(biliStepStyle.Render(" → 点击 "))
	b.WriteString(biliKeyStyle.Render("bilibili.com"))
	b.WriteString("\n")
	b.WriteString(biliStepStyle.Render("  5. 分别复制以下三个值:"))
	b.WriteString("\n\n")

	// Divider
	b.WriteString(biliHelpStyle.Render("  ─────────────────────────────────────────────────────────"))
	b.WriteString("\n\n")

	// Input fields
	for i, input := range m.inputs {
		b.WriteString(input.View())
		if i < len(m.inputs)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Error message
	if m.error != "" {
		b.WriteString("\n")
		b.WriteString(biliErrorStyle.Render("  ✗ " + m.error))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(biliHelpStyle.Render("  Tab/↓ 下一项 • Shift+Tab/↑ 上一项 • Enter 保存 • Esc 取消"))
	b.WriteString("\n")

	return b.String()
}

func runBilibiliLogin() error {
	m := newBilibiliLoginModel()
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result := finalModel.(bilibiliLoginModel)
	if result.cancelled {
		fmt.Println("  已取消")
		return nil
	}

	if result.saved {
		fmt.Println(biliSuccessStyle.Render("  ✓ Bilibili Cookie 已保存"))
	}

	return nil
}
