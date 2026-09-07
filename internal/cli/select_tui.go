package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/webdav"
)

// selectModel is a multi-select list of remote entries (files and
// directories), used when a one-time address offers several of them.
type selectModel struct {
	title     string
	items     []webdav.FileInfo
	cursor    int
	offset    int
	selected  map[int]bool
	confirmed bool
	width     int
	height    int
	t         *i18n.Translations
}

type selectKeyMap struct {
	Up, Down, Toggle, All, Confirm, Quit key.Binding
}

var selectKeys = selectKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k")),
	Down:    key.NewBinding(key.WithKeys("down", "j")),
	Toggle:  key.NewBinding(key.WithKeys(" ", "x")),
	All:     key.NewBinding(key.WithKeys("a")),
	Confirm: key.NewBinding(key.WithKeys("enter")),
	Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c")),
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) visibleLines() int {
	if m.height <= 0 {
		return maxVisibleLines
	}
	n := m.height - 8
	if n > maxVisibleLines {
		return maxVisibleLines
	}
	if n < 5 {
		return 5
	}
	return n
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, selectKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, selectKeys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, selectKeys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(msg, selectKeys.Toggle):
			m.selected[m.cursor] = !m.selected[m.cursor]
		case key.Matches(msg, selectKeys.All):
			all := len(m.selected) == len(m.items)
			for i := range m.items {
				if all {
					delete(m.selected, i)
				} else {
					m.selected[i] = true
				}
			}
		case key.Matches(msg, selectKeys.Confirm):
			m.confirmed = true
			return m, tea.Quit
		}
		visible := m.visibleLines()
		if m.cursor < m.offset {
			m.offset = m.cursor
		} else if m.cursor >= m.offset+visible {
			m.offset = m.cursor - visible + 1
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var b strings.Builder
	b.WriteString(searchTitleStyle.Render("  "+m.title) + "\n\n")
	visible := m.visibleLines()
	end := min(m.offset+visible, len(m.items))
	for i := m.offset; i < end; i++ {
		it := m.items[i]
		cursor := "  "
		if i == m.cursor {
			cursor = searchSelectedStyle.Render("> ")
		}
		check := searchUncheckStyle.Render("[ ]")
		if m.selected[i] {
			check = searchCheckStyle.Render("[x]")
		}
		name := it.Name
		size := ""
		if it.IsDir {
			name = browseDirStyle.Render(name + "/")
		} else {
			size = searchDimStyle.Render(" (" + formatSize(it.Size) + ")")
		}
		if i == m.cursor {
			name = searchSelectedStyle.Render(it.Name)
			if it.IsDir {
				name += "/"
			}
		}
		fmt.Fprintf(&b, "%s%s %s%s\n", cursor, check, name, size)
	}
	if len(m.items) > visible {
		b.WriteString(searchDimStyle.Render(fmt.Sprintf("  (%d-%d of %d)", m.offset+1, end, len(m.items))) + "\n")
	}
	b.WriteString("\n")
	count := 0
	for _, v := range m.selected {
		if v {
			count++
		}
	}
	fmt.Fprintf(&b, "  %s: %d/%d\n", m.t.Search.Selected, count, len(m.items))
	b.WriteString(searchHelpStyle.Render("  "+m.t.Tailcat.SelectHint) + "\n")
	content := searchContainerStyle.Render(b.String())
	if m.width > 0 && m.height > 0 {
		content = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
	}
	return content
}

// RunSelectTUI lets the user pick entries. It returns nil if they quit.
func RunSelectTUI(title string, items []webdav.FileInfo, lang string) ([]webdav.FileInfo, error) {
	m := selectModel{title: title, items: items, selected: map[int]bool{}, t: i18n.T(lang)}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(selectModel)
	if !fm.confirmed {
		return nil, nil
	}
	var out []webdav.FileInfo
	for i, it := range fm.items {
		if fm.selected[i] {
			out = append(out, it)
		}
	}
	return out, nil
}
