package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing dok-ops..."
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	var activeContent string
	switch m.ActiveTab {
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

	contentHeight := m.Height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentHeight < 10 {
		contentHeight = 10
	}

	body := lipgloss.NewStyle().
		Height(contentHeight).
		Width(m.Width).
		Render(activeContent)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		footer,
	)
}

func (m Model) renderHeader() string {
	logo := theme.TitleStyle.Render("⚡ dok-ops v1.0")

	// Tabs
	var tabs []string
	for i, name := range TabNames {
		if Tab(i) == m.ActiveTab {
			tabs = append(tabs, theme.ActiveTabStyle.Render(name))
		} else {
			tabs = append(tabs, theme.InactiveTabStyle.Render(name))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Quick Stats (Uptime, Load, Host)
	loadStr := fmt.Sprintf("Load: %.2f %.2f %.2f", m.QuickStats.Load1, m.QuickStats.Load5, m.QuickStats.Load15)
	uptimeStr := fmt.Sprintf("Up: %s", m.QuickStats.Uptime)
	hostStr := fmt.Sprintf("%s (%s)", m.QuickStats.Hostname, m.QuickStats.OS)

	quickStats := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Render(fmt.Sprintf("%s | %s | %s", hostStr, uptimeStr, loadStr))

	leftSection := lipgloss.JoinHorizontal(lipgloss.Center, logo, " ", tabBar)
	gapWidth := m.Width - lipgloss.Width(leftSection) - lipgloss.Width(quickStats) - 2
	if gapWidth < 1 {
		gapWidth = 1
	}

	headerBar := lipgloss.JoinHorizontal(lipgloss.Center,
		leftSection,
		strings.Repeat(" ", gapWidth),
		quickStats,
	)

	return lipgloss.NewStyle().
		Background(theme.ColorSurface).
		Width(m.Width).
		Padding(0, 1).
		Render(headerBar)
}

func (m Model) renderFooter() string {
	var hints []string

	switch m.ActiveTab {
	case TabMonitor:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Navigate"),
			theme.KeyStyle.Render("c/m/p/n") + ": " + theme.DescStyle.Render("Sort (CPU/Mem/PID/Name)"),
			theme.KeyStyle.Render("k") + ": " + theme.DescStyle.Render("Kill Proc"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Refresh"),
			theme.KeyStyle.Render("q / Ctrl+C") + ": " + theme.DescStyle.Render("Quit"),
		}
	case TabContainers:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select"),
			theme.KeyStyle.Render("u/Enter") + ": " + theme.DescStyle.Render("Start"),
			theme.KeyStyle.Render("s") + ": " + theme.DescStyle.Render("Stop"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Restart"),
			theme.KeyStyle.Render("l") + ": " + theme.DescStyle.Render("Logs"),
			theme.KeyStyle.Render("d") + ": " + theme.DescStyle.Render("Remove"),
			theme.KeyStyle.Render("R") + ": " + theme.DescStyle.Render("Refresh"),
		}
	case TabServices:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select Unit"),
			theme.KeyStyle.Render("u/Enter") + ": " + theme.DescStyle.Render("Start"),
			theme.KeyStyle.Render("s") + ": " + theme.DescStyle.Render("Stop"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Restart"),
			theme.KeyStyle.Render("l") + ": " + theme.DescStyle.Render("Journal Logs"),
			theme.KeyStyle.Render("f") + ": " + theme.DescStyle.Render("Filter (Active/Failed/All)"),
			theme.KeyStyle.Render("R") + ": " + theme.DescStyle.Render("Refresh"),
		}
	case TabPorts:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select Port"),
			theme.KeyStyle.Render("k") + ": " + theme.DescStyle.Render("Kill Proc (Free Port)"),
			theme.KeyStyle.Render("l") + ": " + theme.DescStyle.Render("Toggle Listen/All"),
			theme.KeyStyle.Render("t/u") + ": " + theme.DescStyle.Render("Filter TCP/UDP"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Refresh"),
		}
	case TabNginx:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select Site"),
			theme.KeyStyle.Render("e/Space") + ": " + theme.DescStyle.Render("Toggle Enable/Disable"),
			theme.KeyStyle.Render("t") + ": " + theme.DescStyle.Render("Syntax Test (nginx -t)"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Reload Nginx"),
			theme.KeyStyle.Render("v/Enter") + ": " + theme.DescStyle.Render("View Config"),
			theme.KeyStyle.Render("R") + ": " + theme.DescStyle.Render("Refresh"),
		}
	case TabAutoNginx:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select Detected Project"),
			theme.KeyStyle.Render("s") + ": " + theme.DescStyle.Render("Cycle FastCGI PHP Socket"),
			theme.KeyStyle.Render("g/Enter") + ": " + theme.DescStyle.Render("Generate & Deploy Nginx Config"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Rescan /var/www"),
		}
	case TabDeploy:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select Repository"),
			theme.KeyStyle.Render("p") + ": " + theme.DescStyle.Render("Fast-forward Pull (git pull --ff-only)"),
			theme.KeyStyle.Render("d/Enter") + ": " + theme.DescStyle.Render("Dispatch Zero-Downtime Pipeline"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Rescan Repos"),
		}
	case TabPHPFPM:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select PHP Version"),
			theme.KeyStyle.Render("r/Enter") + ": " + theme.DescStyle.Render("Restart PHP Daemon"),
			theme.KeyStyle.Render("c") + ": " + theme.DescStyle.Render("Flush OPcache & APCu"),
			theme.KeyStyle.Render("R") + ": " + theme.DescStyle.Render("Refresh Sockets"),
		}
	case TabWorkers:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Switch Workers/Schedule"),
			theme.KeyStyle.Render("r/Enter") + ": " + theme.DescStyle.Render("Restart Worker Process"),
			theme.KeyStyle.Render("l") + ": " + theme.DescStyle.Render("View Worker Log Tail"),
			theme.KeyStyle.Render("R") + ": " + theme.DescStyle.Render("Refresh Workers"),
		}
	case TabCertbot:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Switch Domain/Email"),
			theme.KeyStyle.Render("t") + ": " + theme.DescStyle.Render("Dry-run Challenge Test"),
			theme.KeyStyle.Render("p/Enter") + ": " + theme.DescStyle.Render("Provision Let's Encrypt SSL"),
			theme.KeyStyle.Render("d") + ": " + theme.DescStyle.Render("Lookup DNS"),
		}
	case TabKnife:
		hints = []string{
			theme.KeyStyle.Render("1-4") + ": " + theme.DescStyle.Render("Switch Tool (JWT / Cron / Base64 / Hash)"),
			theme.KeyStyle.Render("Type in input") + ": " + theme.DescStyle.Render("Instant offline conversion"),
		}
	case TabSSL:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Enter") + ": " + theme.DescStyle.Render("Inspect Cert"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Details"),
		}
	case TabDatabase:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Switch Conn/Query"),
			theme.KeyStyle.Render("Enter") + ": " + theme.DescStyle.Render("Connect / Run Query"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Result Table"),
		}
	case TabBandwidth:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Sample Bandwidth"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Select Interface"),
		}
	case TabScanner:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("h/l") + ": " + theme.DescStyle.Render("Select Port Preset"),
			theme.KeyStyle.Render("Enter/r") + ": " + theme.DescStyle.Render("Run Concurrent Scan"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Open Ports"),
		}
	case TabGit:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Switch Files/Commits"),
			theme.KeyStyle.Render("d/Enter") + ": " + theme.DescStyle.Render("View Diff"),
			theme.KeyStyle.Render("s/u") + ": " + theme.DescStyle.Render("Stage/Unstage"),
			theme.KeyStyle.Render("z/Z") + ": " + theme.DescStyle.Render("Stash/Pop"),
		}
	case TabCI:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("v/Enter") + ": " + theme.DescStyle.Render("View Run Details"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Refresh GitHub Actions"),
		}
	case TabSSH:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Switch Sessions/Keys"),
			theme.KeyStyle.Render("k") + ": " + theme.DescStyle.Render("Kill SSH Session"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Refresh Auditor"),
		}
	case TabEnv:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Switch Env/Example"),
			theme.KeyStyle.Render("Enter/r") + ": " + theme.DescStyle.Render("Compare & Validate Drift"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Keys"),
		}
	case TabTimers:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("f") + ": " + theme.DescStyle.Render("Filter (Crons / Systemd Timers)"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Refresh Schedules"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Timeline"),
		}
	case TabDisk:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Navigate"),
			theme.KeyStyle.Render("Enter / l") + ": " + theme.DescStyle.Render("Open Dir"),
			theme.KeyStyle.Render("Backspace / h") + ": " + theme.DescStyle.Render("Parent Dir"),
			theme.KeyStyle.Render("s") + ": " + theme.DescStyle.Render("Toggle Sort"),
			theme.KeyStyle.Render("r") + ": " + theme.DescStyle.Render("Rescan"),
			theme.KeyStyle.Render("d") + ": " + theme.DescStyle.Render("Delete"),
		}
	case TabHTTP:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Cycle Inputs"),
			theme.KeyStyle.Render("h/l") + ": " + theme.DescStyle.Render("Change Method"),
			theme.KeyStyle.Render("Enter") + ": " + theme.DescStyle.Render("Execute Request"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Response"),
		}
	case TabDNS:
		hints = []string{
			theme.KeyStyle.Render("Tab/1-0") + ": " + theme.DescStyle.Render("Tabs"),
			theme.KeyStyle.Render("Tab") + ": " + theme.DescStyle.Render("Cycle Focus"),
			theme.KeyStyle.Render("h/l") + ": " + theme.DescStyle.Render("Change Type/Server"),
			theme.KeyStyle.Render("Enter") + ": " + theme.DescStyle.Render("Resolve DNS"),
			theme.KeyStyle.Render("j/k") + ": " + theme.DescStyle.Render("Scroll Records"),
		}
	case TabTerminal:
		hints = []string{
			theme.KeyStyle.Render("F1-F12") + ": " + theme.DescStyle.Render("Jump Tabs"),
			theme.KeyStyle.Render("i") + ": " + theme.DescStyle.Render("Focus Shell"),
			theme.KeyStyle.Render("Ctrl+]") + ": " + theme.DescStyle.Render("Unfocus Shell"),
			theme.KeyStyle.Render("Ctrl+C") + ": " + theme.DescStyle.Render("Send SIGINT to Shell"),
		}
	}

	footerLeft := strings.Join(hints, "  │  ")
	footerRight := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("dok-ops Cockpit")

	gapWidth := m.Width - lipgloss.Width(footerLeft) - lipgloss.Width(footerRight) - 2
	if gapWidth < 1 {
		gapWidth = 1
	}

	footerBar := lipgloss.JoinHorizontal(lipgloss.Center,
		footerLeft,
		strings.Repeat(" ", gapWidth),
		footerRight,
	)

	return lipgloss.NewStyle().
		Background(theme.ColorSurface).
		Width(m.Width).
		Padding(0, 1).
		Render(footerBar)
}
