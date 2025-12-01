package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guiyumin/vget/internal/i18n"
)

var (
	searchTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	searchSelectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	searchDimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	searchCheckStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	searchUncheckStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	searchHelpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	searchDurationStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("135")) // purple
	searchContainerStyle = lipgloss.NewStyle().Padding(1, 2)

	// Tab styles
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Underline(true)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// ItemType distinguishes between podcasts and episodes
type ItemType int

const (
	ItemTypePodcast ItemType = iota
	ItemTypeEpisode
)

// SearchItem represents a selectable item (podcast or episode)
type SearchItem struct {
	Title       string
	Subtitle    string // e.g., "Duration: 45:30 | Plays: 1234"
	URL         string
	DownloadURL string   // Direct download URL if available
	Selectable  bool     // Whether this item can be selected
	Type        ItemType // Podcast or Episode
	// For podcasts (used to fetch episodes)
	PodcastID string // iTunes collection ID or Xiaoyuzhou pid
	FeedURL   string // RSS feed URL (for iTunes)
}

// SearchSection represents a section (Podcasts or Episodes)
type SearchSection struct {
	Title string
	Items []SearchItem
}

const maxSelections = 5

type searchModel struct {
	sections      []SearchSection
	activeTab     int            // Which tab is active
	cursors       []int          // Cursor position for each tab
	scrollOffsets []int          // Scroll offset for each tab
	selected      map[int]bool   // Track selected items by global index within active tab
	selectedCount int
	confirmed     bool
	keyBindings   searchKeyMap
	width         int
	height        int
	query         string
	lang          string
}

type searchKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Toggle   key.Binding
	Confirm  key.Binding
	Quit     key.Binding
}

func defaultSearchKeyMap() searchKeyMap {
	return searchKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev tab"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next tab"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" ", "x"),
			key.WithHelp("space/x", "toggle"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
	}
}

func newSearchModel(sections []SearchSection, query, lang string) searchModel {
	// Initialize cursors and scroll offsets for each tab
	cursors := make([]int, len(sections))
	scrollOffsets := make([]int, len(sections))

	return searchModel{
		sections:      sections,
		activeTab:     0,
		cursors:       cursors,
		scrollOffsets: scrollOffsets,
		selected:      make(map[int]bool),
		keyBindings:   defaultSearchKeyMap(),
		query:         query,
		lang:          lang,
	}
}

func (m searchModel) Init() tea.Cmd {
	return nil
}

const maxVisibleLines = 15 // Max items to show at once

// visibleLines returns how many lines can be displayed
func (m searchModel) visibleLines() int {
	if m.height <= 0 {
		return maxVisibleLines
	}
	// Reserve: title (2 lines) + tabs (2 lines) + footer (3 lines) + padding
	available := m.height - 10
	if available > maxVisibleLines {
		return maxVisibleLines
	}
	if available < 5 {
		return 5 // minimum
	}
	return available
}

// currentSection returns the active section
func (m searchModel) currentSection() *SearchSection {
	if m.activeTab >= 0 && m.activeTab < len(m.sections) {
		return &m.sections[m.activeTab]
	}
	return nil
}

// currentItemType returns the type of items in the current tab
func (m searchModel) currentItemType() ItemType {
	section := m.currentSection()
	if section != nil && len(section.Items) > 0 {
		return section.Items[0].Type
	}
	return ItemTypeEpisode
}

// clearSelections clears all selected items
func (m *searchModel) clearSelections() {
	m.selected = make(map[int]bool)
	m.selectedCount = 0
}

// adjustScroll ensures cursor is visible within current tab
func (m *searchModel) adjustScroll() {
	visible := m.visibleLines()
	cursor := m.cursors[m.activeTab]
	offset := m.scrollOffsets[m.activeTab]

	if cursor < offset {
		m.scrollOffsets[m.activeTab] = cursor
	} else if cursor >= offset+visible {
		m.scrollOffsets[m.activeTab] = cursor - visible + 1
	}
}

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		section := m.currentSection()
		if section == nil {
			if key.Matches(msg, m.keyBindings.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keyBindings.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyBindings.Left), key.Matches(msg, m.keyBindings.ShiftTab):
			if m.activeTab > 0 {
				m.activeTab--
				m.clearSelections()
			}

		case key.Matches(msg, m.keyBindings.Right), key.Matches(msg, m.keyBindings.Tab):
			if m.activeTab < len(m.sections)-1 {
				m.activeTab++
				m.clearSelections()
			}

		case key.Matches(msg, m.keyBindings.Up):
			if m.cursors[m.activeTab] > 0 {
				m.cursors[m.activeTab]--
				m.adjustScroll()
			}

		case key.Matches(msg, m.keyBindings.Down):
			if m.cursors[m.activeTab] < len(section.Items)-1 {
				m.cursors[m.activeTab]++
				m.adjustScroll()
			}

		case key.Matches(msg, m.keyBindings.Toggle):
			cursor := m.cursors[m.activeTab]
			if cursor >= 0 && cursor < len(section.Items) && section.Items[cursor].Selectable {
				// Determine max selections based on item type
				currentType := m.currentItemType()
				maxSel := maxSelections
				if currentType == ItemTypePodcast {
					maxSel = 1
				}

				if m.selected[cursor] {
					// Deselect
					m.selected[cursor] = false
					m.selectedCount--
				} else if m.selectedCount < maxSel {
					// Select (only if under limit)
					m.selected[cursor] = true
					m.selectedCount++
				}
			}

		case key.Matches(msg, m.keyBindings.Confirm):
			m.confirmed = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m searchModel) View() string {
	t := i18n.T(m.lang)

	if len(m.sections) == 0 {
		return "\n  No results found.\n\n"
	}

	var b strings.Builder

	// Title with query
	b.WriteString(fmt.Sprintf("  %s: %s\n\n", searchTitleStyle.Render(t.Search.ResultsFor), m.query))

	// Render tabs
	var tabs []string
	for i, section := range m.sections {
		tabText := section.Title
		if i == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(tabText))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(tabText))
		}
	}
	b.WriteString("  " + strings.Join(tabs, "  │  ") + "\n\n")

	// Get current section
	section := m.currentSection()
	if section == nil || len(section.Items) == 0 {
		b.WriteString("  No items in this section.\n")
	} else {
		// Build lines for current section
		visible := m.visibleLines()
		cursor := m.cursors[m.activeTab]
		offset := m.scrollOffsets[m.activeTab]

		// Adjust offset if needed
		if cursor < offset {
			offset = cursor
		} else if cursor >= offset+visible {
			offset = cursor - visible + 1
		}
		if offset < 0 {
			offset = 0
		}

		endIdx := offset + visible
		if endIdx > len(section.Items) {
			endIdx = len(section.Items)
		}

		for i := offset; i < endIdx; i++ {
			item := section.Items[i]

			cursorStr := "  "
			if i == cursor {
				cursorStr = searchSelectedStyle.Render("> ")
			}

			// Show checkbox for selectable items
			var prefix string
			if item.Selectable {
				checkbox := searchUncheckStyle.Render("[ ]")
				if m.selected[i] {
					checkbox = searchCheckStyle.Render("[x]")
				}
				prefix = checkbox + " "
			} else {
				prefix = "    "
			}

			// Build the line based on item type
			var line string
			title := item.Title
			if i == cursor {
				title = searchSelectedStyle.Render(title)
			}

			if item.Type == ItemTypeEpisode && item.Subtitle != "" {
				// Episode: [duration] title
				duration := searchDurationStyle.Render(fmt.Sprintf("[%s]", item.Subtitle))
				line = fmt.Sprintf("%s %s", duration, title)
			} else if item.Subtitle != "" {
				// Podcast: title (subtitle dimmed)
				line = fmt.Sprintf("%s %s", title, searchDimStyle.Render("("+item.Subtitle+")"))
			} else {
				line = title
			}

			b.WriteString(fmt.Sprintf("%s%s%s\n", cursorStr, prefix, line))
		}

		// Show scroll indicator
		if len(section.Items) > visible {
			scrollInfo := fmt.Sprintf(" (%d-%d of %d)", offset+1, endIdx, len(section.Items))
			b.WriteString(searchDimStyle.Render(scrollInfo) + "\n")
		}
	}

	b.WriteString("\n")

	// Determine current item type for dynamic hints
	isPodcast := m.currentItemType() == ItemTypePodcast

	// Selection count and help
	if m.selectedCount > 0 {
		maxSel := maxSelections
		if isPodcast {
			maxSel = 1
		}
		b.WriteString(fmt.Sprintf("  %s: %d/%d\n", t.Search.Selected, m.selectedCount, maxSel))
	} else {
		if isPodcast {
			b.WriteString(fmt.Sprintf("  %s\n", t.Search.SelectPodcastHint))
		} else {
			b.WriteString(fmt.Sprintf("  "+t.Search.SelectHint+"\n", maxSelections))
		}
	}

	// Help text with tab switching hint
	if isPodcast {
		b.WriteString(searchHelpStyle.Render("  ←/→ switch tabs • " + t.Search.HelpPodcast))
	} else {
		b.WriteString(searchHelpStyle.Render("  ←/→ switch tabs • " + t.Search.Help))
	}
	b.WriteString("\n")

	// Apply container style
	content := searchContainerStyle.Render(b.String())

	// Make it fullscreen
	if m.width > 0 && m.height > 0 {
		content = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
	}

	return content
}

// GetSelectedItems returns the selected items from the active tab
func (m searchModel) GetSelectedItems() []SearchItem {
	var items []SearchItem
	section := m.currentSection()
	if section == nil {
		return items
	}

	for i, item := range section.Items {
		if m.selected[i] {
			items = append(items, item)
		}
	}
	return items
}

// RunSearchTUI runs the search TUI and returns selected items
func RunSearchTUI(sections []SearchSection, query, lang string) ([]SearchItem, error) {
	model := newSearchModel(sections, query, lang)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := finalModel.(searchModel)
	if !m.confirmed {
		return nil, nil // User quit without confirming
	}

	return m.GetSelectedItems(), nil
}
