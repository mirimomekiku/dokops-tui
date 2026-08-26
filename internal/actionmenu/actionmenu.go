package actionmenu

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

// Item represents a single selectable action in the popup modal
type Item struct {
	Key         string
	Title       string
	Description string
	Icon        string
	Danger      bool
}

// Model manages the state of the universal action popup
type Model struct {
	Title    string
	Subtitle string
	Items    []Item
	Cursor   int
	Active   bool
	Width    int
	Height   int
}

// New creates an initial closed action menu model
func New() Model {
	return Model{
		Active: false,
		Cursor: 0,
	}
}

// Open activates the action menu with provided items and resets the cursor
func (m *Model) Open(title, subtitle string, items []Item) {
	m.Title = title
	m.Subtitle = subtitle
	m.Items = items
	m.Cursor = 0
	m.Active = true
}

// Close deactivates the menu
func (m *Model) Close() {
	m.Active = false
	m.Items = nil
}

// IsOpen returns true if the action menu modal is currently open
func (m Model) IsOpen() bool {
	return m.Active
}

// Update handles keyboard navigation within the modal:
// - j / down: next item
// - k / up: prev item
// - 1-9: trigger item by number
// - enter: select active item (returns action Key)
// - esc / space / q: close menu without action
func (m *Model) Update(msg tea.Msg) (string, bool) {
	if !m.Active {
		return "", false
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return "", false
	}

	switch keyMsg.String() {
	case "j", "down":
		if len(m.Items) > 0 {
			m.Cursor = (m.Cursor + 1) % len(m.Items)
		}
		return "", false

	case "k", "up":
		if len(m.Items) > 0 {
			m.Cursor = (m.Cursor + len(m.Items) - 1) % len(m.Items)
		}
		return "", false

	case "enter":
		if len(m.Items) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Items) {
			actionKey := m.Items[m.Cursor].Key
			m.Close()
			return actionKey, true
		}
		m.Close()
		return "", true

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(keyMsg.String()[0]-'1')
		if idx >= 0 && idx < len(m.Items) {
			actionKey := m.Items[idx].Key
			m.Close()
			return actionKey, true
		}

	case "esc", "space", "q":
		m.Close()
		return "", true
	}

	return "", false
}

// RenderModal renders the centered action menu popup over the given background view
func (m Model) RenderModal(bgView string, width, height int) string {
	if !m.Active || len(m.Items) == 0 {
		return bgView
	}

	modalWidth := 62
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 36 {
		modalWidth = 36
	}

	innerWidth := modalWidth - 4 // account for border + padding

	// ── Header ───────────────────────────────────────────────────────────────
	titleBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1a1b26")).
		Background(theme.ColorPrimary).
		Padding(0, 1).
		Render(m.Title)

	var subtitleStr string
	if m.Subtitle != "" {
		subtitleStr = lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Italic(true).
			Render("  " + m.Subtitle)
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center, titleBadge, subtitleStr)

	// ── Divider ───────────────────────────────────────────────────────────────
	divider := lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render(strings.Repeat("─", innerWidth))

	// ── Action Items ─────────────────────────────────────────────────────────
	var rows []string
	for i, item := range m.Items {
		numBadge := lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Bold(true).
			Render(fmt.Sprintf("%d", i+1))

		titleLine := fmt.Sprintf("%s  %s", item.Title, numBadge)
		if item.Danger {
			titleLine = fmt.Sprintf("[!] %s  %s", item.Title, numBadge)
		}

		if i == m.Cursor {
			// Selected state: full-width highlight row
			var bgColor = lipgloss.Color("#283457")
			var fgColor = lipgloss.Color("#ffffff")
			if item.Danger {
				bgColor = lipgloss.Color("#4a1528")
				fgColor = theme.ColorDanger
			}
			gutterBar := lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true).Render("▐ ")
			titleStyled := lipgloss.NewStyle().Bold(true).Foreground(fgColor).Render(titleLine)

			topLine := lipgloss.NewStyle().
				Background(bgColor).
				Width(innerWidth - 2).
				Padding(0, 1).
				Render(gutterBar + titleStyled)

			var bottomLine string
			if item.Description != "" {
				desc := item.Description
				maxDesc := innerWidth - 6
				if len(desc) > maxDesc {
					desc = desc[:maxDesc-1] + "…"
				}
				bottomLine = lipgloss.NewStyle().
					Background(bgColor).
					Foreground(lipgloss.Color("#a9b1d6")).
					Width(innerWidth - 2).
					Padding(0, 1).
					Render("   " + desc)
			}
			if bottomLine != "" {
				rows = append(rows, topLine+"\n"+bottomLine)
			} else {
				rows = append(rows, topLine)
			}
		} else {
			// Unselected state
			fgColor := theme.ColorText
			if item.Danger {
				fgColor = theme.ColorDanger
			}
			titleStyled := lipgloss.NewStyle().Foreground(fgColor).Render("  " + titleLine)

			topLine := lipgloss.NewStyle().
				Width(innerWidth).
				Padding(0, 1).
				Render(titleStyled)

			var bottomLine string
			if item.Description != "" {
				desc := item.Description
				maxDesc := innerWidth - 4
				if len(desc) > maxDesc {
					desc = desc[:maxDesc-1] + "…"
				}
				bottomLine = lipgloss.NewStyle().
					Foreground(theme.ColorMuted).
					Width(innerWidth).
					Padding(0, 1).
					Render("    " + desc)
			}
			if bottomLine != "" {
				rows = append(rows, topLine+"\n"+bottomLine)
			} else {
				rows = append(rows, topLine)
			}
		}
	}

	listContent := strings.Join(rows, "\n")

	// ── Footer ────────────────────────────────────────────────────────────────
	kNav := theme.KeyBadgeStyle.Render("j/k")
	kNum := theme.KeyBadgeStyle.Render("1-9")
	kEnter := theme.KeyBadgePrimaryStyle.Render("Enter")
	kEsc := theme.KeyBadgeStyle.Render("Esc")

	footerHints := lipgloss.JoinHorizontal(lipgloss.Center,
		kNav, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" Navigate  "),
		kNum, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" Quick-select  "),
		kEnter, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" Execute  "),
		kEsc, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" Cancel"),
	)

	// ── Assemble modal ────────────────────────────────────────────────────────
	modalCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorDark).
		Padding(1, 1).
		Width(modalWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			"",
			divider,
			"",
			listContent,
			"",
			divider,
			"",
			footerHints,
		))

	// Center the modal on the background
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modalCard,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(theme.ColorDark),
	)
}
