package commandpalette

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

// CommandItem represents a searchable tool or destination in the palette.
type CommandItem struct {
	TabID       int
	Title       string
	Category    string
	Description string
	Keywords    []string
}

type scoredItem struct {
	item  CommandItem
	score int
}

// Model is the state for the fuzzy command palette overlay.
type Model struct {
	input        textinput.Model
	allItems     []CommandItem
	filtered     []scoredItem
	cursor       int
	scrollOffset int
	isOpen       bool
	width        int
	height       int
}

// New creates a new CommandPalette model with a full registry of command items.
func New(items []CommandItem) Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Type a command or tool name..."
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.ColorMuted)
	ti.CharLimit = 80

	m := Model{
		input:    ti,
		allItems: items,
	}
	m.applyFilter()
	return m
}

// Open activates the command palette modal and focuses the search input.
func (m *Model) Open() {
	m.isOpen = true
	m.input.SetValue("")
	m.input.Focus()
	m.cursor = 0
	m.scrollOffset = 0
	m.applyFilter()
}

// Close dismisses the command palette modal.
func (m *Model) Close() {
	m.isOpen = false
	m.input.Blur()
}

// IsOpen returns whether the palette is currently displayed.
func (m Model) IsOpen() bool {
	return m.isOpen
}

// Update handles keyboard messages for the palette modal.
// Returns (selectedTabID, selected, closed)
func (m *Model) Update(msg tea.Msg) (int, bool, bool) {
	if !m.isOpen {
		return 0, false, false
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return 0, false, false

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+k", "ctrl+c":
			m.Close()
			return 0, false, true

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
			return 0, false, false

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.adjustScroll()
			}
			return 0, false, false

		case "enter":
			if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
				selected := m.filtered[m.cursor].item.TabID
				m.Close()
				return selected, true, true
			}
			m.Close()
			return 0, false, true
		}
	}

	// Update text input
	prevVal := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	_ = cmd

	if m.input.Value() != prevVal {
		m.cursor = 0
		m.scrollOffset = 0
		m.applyFilter()
	}

	return 0, false, false
}

// applyFilter ranks all items based on fuzzy matching the query against Title, Category, Description, and Keywords.
func (m *Model) applyFilter() {
	query := strings.TrimSpace(strings.ToLower(m.input.Value()))

	if query == "" {
		m.filtered = make([]scoredItem, len(m.allItems))
		for i, it := range m.allItems {
			m.filtered[i] = scoredItem{item: it, score: 0}
		}
		return
	}

	var scored []scoredItem
	for _, it := range m.allItems {
		score := calculateFuzzyScore(query, it)
		if score > 0 {
			scored = append(scored, scoredItem{item: it, score: score})
		}
	}

	// Sort descending by match score
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filtered = scored
}

func calculateFuzzyScore(query string, item CommandItem) int {
	titleLower := strings.ToLower(item.Title)
	catLower := strings.ToLower(item.Category)
	descLower := strings.ToLower(item.Description)

	// Exact matches get highest priority
	if titleLower == query {
		return 1000
	}
	if strings.HasPrefix(titleLower, query) {
		return 800 + (100 - len(item.Title))
	}
	if strings.Contains(titleLower, query) {
		return 600 + (100 - len(item.Title))
	}

	// Check keywords
	for _, kw := range item.Keywords {
		kwLower := strings.ToLower(kw)
		if kwLower == query {
			return 700
		}
		if strings.HasPrefix(kwLower, query) {
			return 500
		}
		if strings.Contains(kwLower, query) {
			return 400
		}
	}

	if strings.Contains(catLower, query) {
		return 300
	}

	if strings.Contains(descLower, query) {
		return 200
	}

	// Character-by-character fuzzy match on title
	if fuzzyMatch(query, titleLower) {
		return 100
	}

	return 0
}

func fuzzyMatch(pattern, text string) bool {
	pIdx := 0
	pLen := len(pattern)
	for i := 0; i < len(text) && pIdx < pLen; i++ {
		if text[i] == pattern[pIdx] {
			pIdx++
		}
	}
	return pIdx == pLen
}

func (m *Model) adjustScroll() {
	maxVisible := 8
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+maxVisible {
		m.scrollOffset = m.cursor - maxVisible + 1
	}
}

// RenderModal renders the command palette centered over the background view.
func (m Model) RenderModal(bgView string, screenWidth, screenHeight int) string {
	if !m.isOpen {
		return bgView
	}

	modalWidth := 66
	if modalWidth > screenWidth-4 {
		modalWidth = screenWidth - 4
	}
	if modalWidth < 36 {
		modalWidth = 36
	}

	innerWidth := modalWidth - 4

	maxVisible := 8
	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	// 1. Search Input
	inputLine := lipgloss.NewStyle().
		Width(innerWidth).
		Render(m.input.View())

	divider := lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render(strings.Repeat("─", innerWidth))

	// 2. Items list
	var itemRows []string
	if len(m.filtered) == 0 {
		itemRows = append(itemRows,
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  No matching tools or commands."),
		)
	} else {
		for i := start; i < end; i++ {
			it := m.filtered[i].item
			isSelected := (i == m.cursor)

			catTag := fmt.Sprintf("[%s]", it.Category)
			catStyled := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(catTag)

			titleW := 20
			if titleW > innerWidth/3 {
				titleW = innerWidth / 3
			}

			// Description with clean truncation
			desc := it.Description
			maxDescW := innerWidth - titleW - len(catTag) - 8
			if maxDescW < 8 {
				maxDescW = 8
			}
			if len(desc) > maxDescW {
				desc = desc[:maxDescW-1] + "…"
			}

			if isSelected {
				prefix := lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("> ")
				titleStyled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Width(titleW).Render(it.Title)
				descStyled := lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(desc)

				row := lipgloss.JoinHorizontal(lipgloss.Center,
					prefix,
					titleStyled,
					" ",
					catStyled,
					"  ",
					descStyled,
				)

				line := lipgloss.NewStyle().
					Background(lipgloss.Color("#283457")).
					Width(innerWidth).
					Render(row)
				itemRows = append(itemRows, line)
			} else {
				prefix := "  "
				titleStyled := lipgloss.NewStyle().Foreground(theme.ColorText).Width(titleW).Render(it.Title)
				descStyled := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(desc)

				row := lipgloss.JoinHorizontal(lipgloss.Center,
					prefix,
					titleStyled,
					" ",
					catStyled,
					"  ",
					descStyled,
				)

				line := lipgloss.NewStyle().
					Width(innerWidth).
					Render(row)
				itemRows = append(itemRows, line)
			}
		}
	}

	// 3. Simple Footer
	footer := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Render("enter: select  ·  esc: dismiss  ·  ↑/↓: navigate")

	body := lipgloss.JoinVertical(lipgloss.Left,
		inputLine,
		divider,
		lipgloss.JoinVertical(lipgloss.Left, itemRows...),
		divider,
		footer,
	)

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorHighlight).
		Background(theme.ColorDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(body)

	return lipgloss.Place(
		screenWidth,
		screenHeight,
		lipgloss.Center,
		lipgloss.Center,
		modalBox,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(theme.ColorDark),
	)
}
