package cli

import (
	"fmt"
	"strings"

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

var loginBilibiliLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear Bilibili credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault()
		cfg.Bilibili.Cookie = ""
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Println("Bilibili credentials cleared")
		return nil
	},
}

func init() {
	loginBilibiliCmd.AddCommand(loginBilibiliStatusCmd)
	loginBilibiliCmd.AddCommand(loginBilibiliLogoutCmd)
	loginCmd.AddCommand(loginBilibiliCmd)
	rootCmd.AddCommand(loginCmd)
}

// Bilibili login TUI

var (
	biliTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00A1D6")) // Bilibili blue

	biliStepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248"))

	biliInputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	biliCursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00A1D6")).
			Bold(true)

	biliHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	biliSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82"))

	biliErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	biliContainerStyle = lipgloss.NewStyle().
				Padding(1, 2)
)

type bilibiliLoginModel struct {
	cookie      string
	saved       bool
	cancelled   bool
	error       string
	width       int
	height      int
	showingHelp bool
}

func newBilibiliLoginModel() bilibiliLoginModel {
	cfg := config.LoadOrDefault()
	return bilibiliLoginModel{
		cookie: cfg.Bilibili.Cookie,
	}
}

func (m bilibiliLoginModel) Init() tea.Cmd {
	return nil
}

func (m bilibiliLoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		case "enter":
			// Validate and save
			if m.cookie == "" {
				m.error = "Cookie cannot be empty"
				return m, nil
			}
			if !strings.Contains(m.cookie, "SESSDATA") {
				m.error = "Cookie must contain SESSDATA"
				return m, nil
			}

			// Save to config
			cfg := config.LoadOrDefault()
			cfg.Bilibili.Cookie = m.cookie
			if err := config.Save(cfg); err != nil {
				m.error = fmt.Sprintf("Failed to save: %v", err)
				return m, nil
			}

			m.saved = true
			return m, tea.Quit

		case "tab":
			m.showingHelp = !m.showingHelp
			return m, nil

		case "backspace":
			if len(m.cookie) > 0 {
				m.cookie = m.cookie[:len(m.cookie)-1]
			}
			m.error = ""
			return m, nil

		case "ctrl+u":
			m.cookie = ""
			m.error = ""
			return m, nil

		case "ctrl+v":
			// Note: clipboard paste is handled by terminal, not bubbletea
			return m, nil

		default:
			// Accept printable characters
			if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
				char := msg.String()
				if msg.Type == tea.KeySpace {
					char = " "
				}
				m.cookie += char
				m.error = ""
			}
			return m, nil
		}
	}

	return m, nil
}

func (m bilibiliLoginModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(biliTitleStyle.Render("Bilibili 登录"))
	b.WriteString("\n\n")

	// Instructions toggle
	if m.showingHelp {
		b.WriteString(biliStepStyle.Render("获取 Cookie 的方法："))
		b.WriteString("\n")
		b.WriteString(biliStepStyle.Render("1. 在浏览器中打开 bilibili.com 并登录"))
		b.WriteString("\n")
		b.WriteString(biliStepStyle.Render("2. 按 F12 打开开发者工具"))
		b.WriteString("\n")
		b.WriteString(biliStepStyle.Render("3. 切换到「应用」(Application) 标签"))
		b.WriteString("\n")
		b.WriteString(biliStepStyle.Render("4. 在左侧找到 Cookies → bilibili.com"))
		b.WriteString("\n")
		b.WriteString(biliStepStyle.Render("5. 复制 SESSDATA、bili_jct、DedeUserID 的值"))
		b.WriteString("\n\n")
		b.WriteString(biliStepStyle.Render("格式：SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx"))
		b.WriteString("\n\n")
	} else {
		b.WriteString(biliStepStyle.Render("按 Tab 显示获取方法"))
		b.WriteString("\n\n")
	}

	// Input field
	b.WriteString(biliStepStyle.Render("Cookie:"))
	b.WriteString("\n")
	b.WriteString(biliCursorStyle.Render("> "))

	// Show truncated cookie if too long
	displayCookie := m.cookie
	maxLen := 60
	if len(displayCookie) > maxLen {
		displayCookie = displayCookie[:maxLen] + "..."
	}
	b.WriteString(biliInputStyle.Render(displayCookie))
	b.WriteString(biliCursorStyle.Render("█"))
	b.WriteString("\n")

	// Error message
	if m.error != "" {
		b.WriteString("\n")
		b.WriteString(biliErrorStyle.Render("✗ " + m.error))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(biliHelpStyle.Render("Enter 保存 • Tab 显示帮助 • Ctrl+U 清空 • Esc 取消"))

	return biliContainerStyle.Render(b.String())
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
		fmt.Println("Cancelled")
		return nil
	}

	if result.saved {
		fmt.Println(biliSuccessStyle.Render("✓ Bilibili cookie saved"))
	}

	return nil
}
