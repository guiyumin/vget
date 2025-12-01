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
	cursor        int // Global cursor across all items
	totalItems    int
	selected      map[int]bool     // Track selected items by global index
	selectable    map[int]bool     // Track which items are selectable
	itemTypes     map[int]ItemType // Track item type by global index
	selectedCount int
	confirmed     bool
	keyBindings   searchKeyMap
	width         int
	height        int
	query         string
	lang          string
	scrollOffset  int // For scrolling
}

type searchKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Toggle  key.Binding
	Confirm key.Binding
	Quit    key.Binding
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
		Toggle: key.NewBinding(
			key.WithKeys(" ", "x"),
			key.WithHelp("space/x", "toggle"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "download"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
	}
}

func newSearchModel(sections []SearchSection, query, lang string) searchModel {
	total := 0
	selectable := make(map[int]bool)
	itemTypes := make(map[int]ItemType)
	idx := 0
	for _, s := range sections {
		for _, item := range s.Items {
			if item.Selectable {
				selectable[idx] = true
			}
			itemTypes[idx] = item.Type
			idx++
		}
		total += len(s.Items)
	}

	// Find first selectable item for initial cursor
	cursor := 0
	for i := 0; i < total; i++ {
		if selectable[i] {
			cursor = i
			break
		}
	}

	return searchModel{
		sections:    sections,
		cursor:      cursor,
		totalItems:  total,
		selected:    make(map[int]bool),
		selectable:  selectable,
		itemTypes:   itemTypes,
		keyBindings: defaultSearchKeyMap(),
		query:       query,
		lang:        lang,
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
	// Reserve: title (2 lines) + footer (3 lines) + padding
	available := m.height - 8
	if available > maxVisibleLines {
		return maxVisibleLines
	}
	if available < 5 {
		return 5 // minimum
	}
	return available
}

// adjustScroll ensures cursor is visible
func (m *searchModel) adjustScroll() {
	visible := m.visibleLines()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

// clearSelections clears all selected items
func (m *searchModel) clearSelections() {
	m.selected = make(map[int]bool)
	m.selectedCount = 0
}

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyBindings.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyBindings.Up):
			if m.cursor > 0 {
				oldType := m.itemTypes[m.cursor]
				m.cursor--
				m.adjustScroll()
				// Clear selections if moving to different item type
				if m.itemTypes[m.cursor] != oldType {
					m.clearSelections()
				}
			}

		case key.Matches(msg, m.keyBindings.Down):
			if m.cursor < m.totalItems-1 {
				oldType := m.itemTypes[m.cursor]
				m.cursor++
				m.adjustScroll()
				// Clear selections if moving to different item type
				if m.itemTypes[m.cursor] != oldType {
					m.clearSelections()
				}
			}

		case key.Matches(msg, m.keyBindings.Toggle):
			// Only toggle if item is selectable
			if !m.selectable[m.cursor] {
				break
			}

			// Determine max selections based on item type
			// Podcasts: only 1 (to browse episodes)
			// Episodes: up to 5 (for batch download)
			currentType := m.itemTypes[m.cursor]
			maxSel := maxSelections
			if currentType == ItemTypePodcast {
				maxSel = 1
			}

			if m.selected[m.cursor] {
				// Deselect
				m.selected[m.cursor] = false
				m.selectedCount--
			} else if m.selectedCount < maxSel {
				// Select (only if under limit)
				m.selected[m.cursor] = true
				m.selectedCount++
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

	if m.totalItems == 0 {
		return "\n  No results found.\n\n"
	}

	var b strings.Builder

	// Title with query
	b.WriteString(fmt.Sprintf("  %s: %s\n\n", searchTitleStyle.Render(t.Search.ResultsFor), m.query))

	// Build all lines first
	var lines []string
	var lineToIdx []int // map line index to global item index

	globalIdx := 0
	for _, section := range m.sections {
		if len(section.Items) == 0 {
			continue
		}

		// Section title
		lines = append(lines, fmt.Sprintf("  %s", searchTitleStyle.Render(section.Title)))
		lineToIdx = append(lineToIdx, -1) // section title, not an item

		for _, item := range section.Items {
			cursor := "  "
			if globalIdx == m.cursor {
				cursor = searchSelectedStyle.Render("> ")
			}

			// Show checkbox for selectable items
			var prefix string
			if item.Selectable {
				checkbox := searchUncheckStyle.Render("[ ]")
				if m.selected[globalIdx] {
					checkbox = searchCheckStyle.Render("[x]")
				}
				prefix = checkbox + " "
			} else {
				prefix = "    " // Indent for non-selectable items (align with checkbox)
			}

			// Build the line based on item type
			var line string
			title := item.Title
			if globalIdx == m.cursor {
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
				// Just title
				line = title
			}

			lines = append(lines, fmt.Sprintf("%s%s%s", cursor, prefix, line))
			lineToIdx = append(lineToIdx, globalIdx)
			globalIdx++
		}
		// Add spacing after section
		lines = append(lines, "")
		lineToIdx = append(lineToIdx, -1)
	}

	// Calculate visible range based on cursor position
	visible := m.visibleLines()

	// Find which line the cursor is on
	cursorLine := 0
	for i, idx := range lineToIdx {
		if idx == m.cursor {
			cursorLine = i
			break
		}
	}

	// Calculate scroll offset to keep cursor visible
	startLine := m.scrollOffset
	if cursorLine < startLine {
		startLine = cursorLine
	} else if cursorLine >= startLine+visible {
		startLine = cursorLine - visible + 1
	}
	if startLine < 0 {
		startLine = 0
	}

	endLine := startLine + visible
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Render visible lines
	for i := startLine; i < endLine; i++ {
		b.WriteString(lines[i] + "\n")
	}

	// Show scroll indicator if needed (showing item numbers, not line numbers)
	if len(lines) > visible {
		// Find first and last visible item indices
		firstItem, lastItem := -1, -1
		for i := startLine; i < endLine; i++ {
			if lineToIdx[i] >= 0 {
				if firstItem < 0 {
					firstItem = lineToIdx[i] + 1 // 1-indexed for display
				}
				lastItem = lineToIdx[i] + 1
			}
		}
		if firstItem > 0 && lastItem > 0 {
			scrollInfo := fmt.Sprintf(" (%d-%d of %d)", firstItem, lastItem, m.totalItems)
			b.WriteString(searchDimStyle.Render(scrollInfo) + "\n")
		}
	}

	// Determine current item type for dynamic hints
	currentType := m.itemTypes[m.cursor]
	isPodcast := currentType == ItemTypePodcast && m.selectable[m.cursor]

	// Selection count and help (dynamic based on cursor position)
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

	// Help text (dynamic based on cursor position)
	if isPodcast {
		b.WriteString(searchHelpStyle.Render("  " + t.Search.HelpPodcast))
	} else {
		b.WriteString(searchHelpStyle.Render("  " + t.Search.Help))
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

// GetSelectedItems returns the selected items
func (m searchModel) GetSelectedItems() []SearchItem {
	var items []SearchItem
	globalIdx := 0
	for _, section := range m.sections {
		for _, item := range section.Items {
			if m.selected[globalIdx] {
				items = append(items, item)
			}
			globalIdx++
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
