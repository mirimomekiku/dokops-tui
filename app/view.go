package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return lipgloss.Place(
			80, 24,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Loading…"),
		)
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	var activeContent string
	switch m.ActiveTab {
	case TabDashboard:
		activeContent = m.DashboardView.View()
	case TabMonitor:
		activeContent = m.MonitorView.View()
	case TabContainers:
		activeContent = m.ContainersView.View()
	case TabServices:
		activeContent = m.ServicesView.View()
	case TabPorts:
		activeContent = m.PortsView.View()
	case TabNginx:
		activeContent = m.NginxView.View()
	case TabAutoNginx:
		activeContent = m.AutoNginxView.View()
	case TabDeploy:
		activeContent = m.DeployView.View()
	case TabPHPFPM:
		activeContent = m.PHPFPMView.View()
	case TabWorkers:
		activeContent = m.WorkersView.View()
	case TabCertbot:
		activeContent = m.CertbotView.View()
	case TabKnife:
		activeContent = m.KnifeView.View()
	case TabSSL:
		activeContent = m.SSLView.View()
	case TabDatabase:
		activeContent = m.DatabaseView.View()
	case TabBandwidth:
		activeContent = m.BandwidthView.View()
	case TabScanner:
		activeContent = m.ScannerView.View()
	case TabGit:
		activeContent = m.GitView.View()
	case TabCI:
		activeContent = m.CIView.View()
	case TabSSH:
		activeContent = m.SSHView.View()
	case TabEnv:
		activeContent = m.EnvView.View()
	case TabTimers:
		activeContent = m.TimersView.View()
	case TabDisk:
		activeContent = m.DiskView.View()
	case TabHTTP:
		activeContent = m.HTTPView.View()
	case TabDNS:
		activeContent = m.DNSView.View()
	case TabTerminal:
		activeContent = m.TerminalView.View()
	}

	contentH := m.Height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentH < 4 {
		contentH = 4
	}

	content := lipgloss.NewStyle().
		Width(m.Width).
		Height(contentH).
		Render(activeContent)

	rendered := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	return m.CommandPalette.RenderModal(rendered, m.Width, m.Height)
}

// renderHeader renders a single-line bar: logo · location · search hint
func (m Model) renderHeader() string {
	if m.ActiveTab == TabDashboard {
		title := lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("DokOps"),
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  ·  Operations Cockpit"),
		)

		searchHint := lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().
				Foreground(theme.ColorMuted).
				Background(lipgloss.Color("#2f3549")).
				Padding(0, 1).
				Render("Ctrl+K"),
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  search"),
		)

		gap := m.Width - lipgloss.Width(title) - lipgloss.Width(searchHint) - 4
		if gap < 1 {
			gap = 1
		}

		bar := lipgloss.JoinHorizontal(lipgloss.Center,
			title,
			strings.Repeat(" ", gap),
			searchHint,
		)

		return lipgloss.NewStyle().
			Background(theme.ColorSurface).
			Width(m.Width).
			Padding(0, 2).
			Render(bar)
	}

	activeWS := Workspaces[m.ActiveWorkspace]
	activeTabName := TabDisplayNames[m.ActiveTab]

	// Left: DokOps › Workspace › Tool
	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorHighlight).
		Render("DokOps")

	wsName := activeWS.Name
	// Strip "N: " numbering prefix stored in the workspace name
	if len(wsName) > 3 && wsName[1] == ':' {
		wsName = wsName[3:]
	}

	location := lipgloss.JoinHorizontal(lipgloss.Center,
		logo,
		lipgloss.NewStyle().Foreground(theme.ColorBorder).Render("  ›  "),
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorText).Render(wsName),
		lipgloss.NewStyle().Foreground(theme.ColorBorder).Render("  ›  "),
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render(activeTabName),
	)

	// Right: [Ctrl+K]
	searchHint := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Background(lipgloss.Color("#2f3549")).
			Padding(0, 1).
			Render("Ctrl+K"),
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  search"),
	)

	gap := m.Width - lipgloss.Width(location) - lipgloss.Width(searchHint) - 4
	if gap < 1 {
		gap = 1
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center,
		location,
		strings.Repeat(" ", gap),
		searchHint,
	)

	return lipgloss.NewStyle().
		Background(theme.ColorSurface).
		Width(m.Width).
		Padding(0, 2).
		Render(bar)
}

// renderFooter renders a single-line bar of plain key hints
func (m Model) renderFooter() string {
	sep := lipgloss.NewStyle().Foreground(theme.ColorBorder).Render("   ·   ")

	var hints []string
	switch m.ActiveTab {
	case TabDashboard:
		hints = []string{
			hint("1-4", "workspace"),
			hint("Ctrl+K", "search tools"),
			hint("Tab", "first tool"),
			hint("Ctrl+C", "quit"),
		}
	case TabTerminal:
		hints = []string{
			hint("i", "focus shell"),
			hint("Ctrl+]", "unfocus"),
			hint("1-4", "workspace"),
			hint("Ctrl+K", "search"),
			hint("Ctrl+C", "quit"),
		}
	case TabKnife:
		hints = []string{
			hint("Shift+Tab", "knife sub-tool"),
			hint("Tab", "next tool"),
			hint("1-4", "workspace"),
			hint("Ctrl+K", "search"),
			hint("Ctrl+C", "quit"),
		}
	case TabDatabase, TabCertbot, TabEnv, TabHTTP, TabDNS:
		hints = []string{
			hint("Enter", "execute / save"),
			hint("Space", "actions"),
			hint("Shift+Tab", "switch field"),
			hint("Tab", "next tool"),
			hint("1-4", "workspace"),
			hint("Ctrl+K", "search"),
			hint("Ctrl+C", "quit"),
		}
	case TabGit, TabWorkers, TabSSH, TabAutoNginx:
		hints = []string{
			hint("Enter", "view details"),
			hint("Space", "actions"),
			hint("j/k", "navigate"),
			hint("Shift+Tab", "switch pane"),
			hint("Tab", "next tool"),
			hint("1-4", "workspace"),
			hint("Ctrl+K", "search"),
			hint("Ctrl+C", "quit"),
		}
	default:
		hints = []string{
			hint("Enter", "view / run"),
			hint("Space", "actions"),
			hint("j/k", "navigate"),
			hint("Tab", "next tool"),
			hint("1-4", "workspace"),
			hint("Ctrl+K", "search"),
			hint("Ctrl+C", "quit"),
		}
	}

	left := strings.Join(hints, sep)

	return lipgloss.NewStyle().
		Background(theme.ColorSurface).
		Foreground(theme.ColorMuted).
		Width(m.Width).
		Padding(0, 2).
		Render(left)
}

// hint formats a single "key: desc" pair for the footer
func hint(key, desc string) string {
	return lipgloss.NewStyle().Foreground(theme.ColorText).Bold(true).Render(key) +
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(": "+desc)
}
