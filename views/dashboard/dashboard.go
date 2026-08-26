package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type HostInfo struct {
	Hostname string
	OS       string
	Uptime   string
	Load1    float64
	Load5    float64
	Load15   float64
}

type Model struct {
	width    int
	height   int
	hostInfo HostInfo
}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) SetHostInfo(info HostInfo) {
	m.hostInfo = info
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}

	// ── 1. Banner & Subtitle ──────────────────────────────────────────────────
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorHighlight).
		Render("DokOps")

	subtitle := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Render("Server & DevOps Operations Cockpit")

	headerBlock := lipgloss.JoinVertical(lipgloss.Center,
		banner,
		subtitle,
	)

	// ── 2. Search Trigger Box (OpenCode style) ─────────────────────────────────
	searchBoxWidth := 56
	if searchBoxWidth > w-4 {
		searchBoxWidth = w - 4
	}
	if searchBoxWidth < 30 {
		searchBoxWidth = 30
	}

	searchKeyBadge := theme.KeyBadgePrimaryStyle.Render("Ctrl+K / /")
	searchPrompt := lipgloss.NewStyle().Foreground(theme.ColorText).Render(" Search tools, services, containers...")

	searchBoxContent := lipgloss.JoinHorizontal(lipgloss.Center,
		"  ",
		searchKeyBadge,
		searchPrompt,
	)

	searchBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorHighlight).
		Background(theme.ColorDark).
		Width(searchBoxWidth).
		Padding(0, 1).
		Render(searchBoxContent)

	// ── 3. Workspace Overview Cards ───────────────────────────────────────────
	cardWidth := (w - 6) / 2
	if cardWidth < 28 {
		cardWidth = 28
	}
	if cardWidth > 40 {
		cardWidth = 40
	}

	renderCard := func(num, title string, tools []string) string {
		titleLine := lipgloss.JoinHorizontal(lipgloss.Center,
			theme.KeyBadgePrimaryStyle.Render(num),
			" ",
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render(title),
		)

		var toolLines []string
		for _, tool := range tools {
			toolLines = append(toolLines, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  • "+tool))
		}

		cardBody := lipgloss.JoinVertical(lipgloss.Left,
			titleLine,
			"",
			strings.Join(toolLines, "\n"),
		)

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorBorder).
			Background(theme.ColorDark).
			Padding(1, 2).
			Width(cardWidth).
			Render(cardBody)
	}

	card1 := renderCard("1", "System", []string{
		"Monitor (top/htop)",
		"Bandwidth Monitor",
		"Disk Analyzer (ncdu)",
		"Systemd Timers",
		"Systemd Services",
	})

	card2 := renderCard("2", "WebOps", []string{
		"Nginx Manager",
		"Auto-Nginx Generator",
		"PHP-FPM Pools",
		"Certbot SSL",
		"SSL/TLS Inspector",
		"Supervisor Workers",
	})

	card3 := renderCard("3", "Deploy", []string{
		"Deploy Hub",
		"Git Inspector",
		"GitHub CI Status",
		".Env File Validator",
	})

	card4 := renderCard("4", "Net & DB", []string{
		"Database (Postgres/MySQL)",
		"Docker Containers",
		"HTTP Request Tracer",
		"DNS Diagnostics",
		"Network Port Scanner",
	})

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, card1, "  ", card2)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, card3, "  ", card4)
	cardsGrid := lipgloss.JoinVertical(lipgloss.Center, row1, "", row2)

	// ── 4. Hotkeys Cheat Sheet ────────────────────────────────────────────────
	renderHotkey := func(key, desc string) string {
		k := theme.KeyBadgeStyle.Render(key)
		d := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" " + desc)
		return lipgloss.JoinHorizontal(lipgloss.Center, k, d)
	}

	hotkeysRow1 := lipgloss.JoinHorizontal(lipgloss.Center,
		renderHotkey("1-4", "Switch Workspace"),
		"    ",
		renderHotkey("Tab", "Next Tool"),
		"    ",
		renderHotkey("Shift+Tab", "Switch Field/Pane"),
	)

	hotkeysRow2 := lipgloss.JoinHorizontal(lipgloss.Center,
		renderHotkey("Ctrl+K", "Command Palette"),
		"    ",
		renderHotkey("Space", "Action Menu"),
		"    ",
		renderHotkey("Esc", "Return to Dashboard"),
		"    ",
		renderHotkey("Ctrl+C", "Quit"),
	)

	hotkeysBlock := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("── Quick Navigation Keys ──"),
		"",
		hotkeysRow1,
		"",
		hotkeysRow2,
	)

	// ── 5. Real-time Host Info ────────────────────────────────────────────────
	var hostLine string
	if m.hostInfo.Hostname != "" {
		hostLine = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
			fmt.Sprintf("Host: %s (%s)  │  Up: %s  │  Load: %.2f %.2f %.2f",
				m.hostInfo.Hostname, m.hostInfo.OS, m.hostInfo.Uptime,
				m.hostInfo.Load1, m.hostInfo.Load5, m.hostInfo.Load15,
			),
		)
	}

	// ── Assemble Dashboard ────────────────────────────────────────────────────
	elements := []string{
		headerBlock,
		"",
		searchBox,
		"",
		cardsGrid,
		"",
		hotkeysBlock,
	}

	if hostLine != "" {
		elements = append(elements, "", hostLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, elements...)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
