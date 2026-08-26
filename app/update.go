package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"dok-ops/views/autonginx"
	"dok-ops/views/bandwidth"
	"dok-ops/views/certbot"
	"dok-ops/views/ci"
	"dok-ops/views/containers"
	"dok-ops/views/dashboard"
	"dok-ops/views/database"
	"dok-ops/views/deploy"
	"dok-ops/views/disk"
	"dok-ops/views/dns"
	"dok-ops/views/env"
	"dok-ops/views/git"
	"dok-ops/views/httpclient"
	"dok-ops/views/monitor"
	"dok-ops/views/nginx"
	"dok-ops/views/phpfpm"
	"dok-ops/views/ports"
	"dok-ops/views/scanner"
	"dok-ops/views/services"
	"dok-ops/views/ssh"
	"dok-ops/views/ssl"
	"dok-ops/views/terminal"
	"dok-ops/views/timers"
	"dok-ops/views/workers"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		// Subview content height: window height minus single-row header (3) and footer (3)
		subViewMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: msg.Height - 6,
		}

		var cmd tea.Cmd
		m.DashboardView, cmd = m.DashboardView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.MonitorView, cmd = m.MonitorView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.ContainersView, cmd = m.ContainersView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.ServicesView, cmd = m.ServicesView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.PortsView, cmd = m.PortsView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.NginxView, cmd = m.NginxView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.AutoNginxView, cmd = m.AutoNginxView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.DeployView, cmd = m.DeployView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.PHPFPMView, cmd = m.PHPFPMView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.WorkersView, cmd = m.WorkersView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.CertbotView, cmd = m.CertbotView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.KnifeView, cmd = m.KnifeView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.SSLView, cmd = m.SSLView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.DatabaseView, cmd = m.DatabaseView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.BandwidthView, cmd = m.BandwidthView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.ScannerView, cmd = m.ScannerView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.GitView, cmd = m.GitView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.CIView, cmd = m.CIView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.SSHView, cmd = m.SSHView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.EnvView, cmd = m.EnvView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.TimersView, cmd = m.TimersView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.DiskView, cmd = m.DiskView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.HTTPView, cmd = m.HTTPView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.DNSView, cmd = m.DNSView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		m.TerminalView, cmd = m.TerminalView.Update(subViewMsg)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)

	case QuickStatsTickMsg:
		cmds = append(cmds, FetchQuickStats(), quickStatsTick())

	case QuickStatsMsg:
		m.QuickStats = msg
		m.DashboardView.SetHostInfo(dashboard.HostInfo{
			Hostname: msg.Hostname,
			OS:       msg.OS,
			Uptime:   msg.Uptime,
			Load1:    msg.Load1,
			Load5:    msg.Load5,
			Load15:   msg.Load15,
		})

	// View-specific async messages routing
	case monitor.StatsMsg, monitor.TickMsg:
		var cmd tea.Cmd
		m.MonitorView, cmd = m.MonitorView.Update(msg)
		return m, cmd

	case containers.ContainersLoadedMsg, containers.LogsStreamedMsg, containers.ActionCompleteMsg, containers.DockerErrorMsg:
		var cmd tea.Cmd
		m.ContainersView, cmd = m.ContainersView.Update(msg)
		return m, cmd

	case services.UnitsLoadedMsg, services.UnitLogsMsg, services.ServiceActionMsg:
		var cmd tea.Cmd
		m.ServicesView, cmd = m.ServicesView.Update(msg)
		return m, cmd

	case ports.SocketsLoadedMsg:
		var cmd tea.Cmd
		m.PortsView, cmd = m.PortsView.Update(msg)
		return m, cmd

	case nginx.SitesLoadedMsg, nginx.SyntaxTestMsg, nginx.NginxActionMsg:
		var cmd tea.Cmd
		m.NginxView, cmd = m.NginxView.Update(msg)
		return m, cmd

	case autonginx.ScanResultMsg, autonginx.DeployPipelineMsg:
		var cmd tea.Cmd
		m.AutoNginxView, cmd = m.AutoNginxView.Update(msg)
		return m, cmd

	case deploy.ReposLoadedMsg, deploy.PipelineProgressMsg:
		var cmd tea.Cmd
		m.DeployView, cmd = m.DeployView.Update(msg)
		return m, cmd

	case phpfpm.PHPDataLoadedMsg, phpfpm.PHPActionMsg:
		var cmd tea.Cmd
		m.PHPFPMView, cmd = m.PHPFPMView.Update(msg)
		return m, cmd

	case workers.WorkersLoadedMsg, workers.WorkerActionMsg:
		var cmd tea.Cmd
		m.WorkersView, cmd = m.WorkersView.Update(msg)
		return m, cmd

	case certbot.CertbotResultMsg, certbot.DNSCheckMsg:
		var cmd tea.Cmd
		m.CertbotView, cmd = m.CertbotView.Update(msg)
		return m, cmd

	case ssl.InspectResultMsg:
		var cmd tea.Cmd
		m.SSLView, cmd = m.SSLView.Update(msg)
		return m, cmd

	case database.QueryResultMsg, database.HealthStatsMsg:
		var cmd tea.Cmd
		m.DatabaseView, cmd = m.DatabaseView.Update(msg)
		return m, cmd

	case bandwidth.NetBandwidthMsg, bandwidth.TickMsg:
		var cmd tea.Cmd
		m.BandwidthView, cmd = m.BandwidthView.Update(msg)
		return m, cmd

	case scanner.ScanFinishedMsg:
		var cmd tea.Cmd
		m.ScannerView, cmd = m.ScannerView.Update(msg)
		return m, cmd

	case git.GitStateMsg, git.GitDiffMsg, git.GitActionMsg:
		var cmd tea.Cmd
		m.GitView, cmd = m.GitView.Update(msg)
		return m, cmd

	case ci.WorkflowRunsLoadedMsg:
		var cmd tea.Cmd
		m.CIView, cmd = m.CIView.Update(msg)
		return m, cmd

	case ssh.SSHDataLoadedMsg:
		var cmd tea.Cmd
		m.SSHView, cmd = m.SSHView.Update(msg)
		return m, cmd

	case env.EnvValidationMsg:
		var cmd tea.Cmd
		m.EnvView, cmd = m.EnvView.Update(msg)
		return m, cmd

	case timers.TimersLoadedMsg:
		var cmd tea.Cmd
		m.TimersView, cmd = m.TimersView.Update(msg)
		return m, cmd

	case disk.ScanProgressMsg, disk.ScanCompleteMsg, disk.DeleteCompleteMsg:
		var cmd tea.Cmd
		m.DiskView, cmd = m.DiskView.Update(msg)
		return m, cmd

	case httpclient.ResponseResultMsg:
		var cmd tea.Cmd
		m.HTTPView, cmd = m.HTTPView.Update(msg)
		return m, cmd

	case dns.DNSResultMsg:
		var cmd tea.Cmd
		m.DNSView, cmd = m.DNSView.Update(msg)
		return m, cmd

	case terminal.TerminalDataMsg, terminal.TerminalExitMsg:
		var cmd tea.Cmd
		m.TerminalView, cmd = m.TerminalView.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// If command palette is open, intercept keystrokes
		if m.CommandPalette.IsOpen() {
			selectedTab, selected, closed := m.CommandPalette.Update(msg)
			if selected {
				m.SetTab(Tab(selectedTab))
				return m, nil
			}
			if closed {
				return m, nil
			}
			return m, nil
		}

		// Global Command Palette shortcut: Ctrl+K or / (when not typing in terminal or inputs)
		if msg.String() == "ctrl+k" || (msg.String() == "/" && m.ActiveTab != TabHTTP && m.ActiveTab != TabDNS && m.ActiveTab != TabKnife && m.ActiveTab != TabSSL && m.ActiveTab != TabDatabase && m.ActiveTab != TabEnv && m.ActiveTab != TabCertbot && m.ActiveTab != TabAutoNginx && (m.ActiveTab != TabTerminal || !m.TerminalView.IsFocused())) {
			m.CommandPalette.Open()
			m.TerminalView.SetFocus(false)
			return m, nil
		}

		// F1-F12 always jump directly to tab even if PTY is captured
		switch msg.String() {
		case "f1":
			m.SetTab(TabMonitor)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f2":
			m.SetTab(TabContainers)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f3":
			m.SetTab(TabServices)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f4":
			m.SetTab(TabPorts)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f5":
			m.SetTab(TabNginx)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f6":
			m.SetTab(TabAutoNginx)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f7":
			m.SetTab(TabDeploy)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f8":
			m.SetTab(TabPHPFPM)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f9":
			m.SetTab(TabWorkers)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f10":
			m.SetTab(TabCertbot)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f11":
			m.SetTab(TabGit)
			m.TerminalView.SetFocus(false)
			return m, nil
		case "f12":
			m.SetTab(TabTerminal)
			return m, nil
		}

		// If on terminal tab and focused, pass key directly
		if m.ActiveTab == TabTerminal && m.TerminalView.IsFocused() {
			if msg.String() == "ctrl+]" {
				m.TerminalView.SetFocus(false)
				return m, nil
			}
			var cmd tea.Cmd
			m.TerminalView, cmd = m.TerminalView.Update(msg)
			return m, cmd
		}

		// Global navigation keys
		switch msg.String() {
		case "ctrl+c":
			m.TerminalView.Close()
			return m, tea.Quit
		case "q":
			// Allow 'q' to quit from Dashboard / Monitor / Bandwidth tabs
			if m.ActiveTab == TabDashboard || m.ActiveTab == TabMonitor || m.ActiveTab == TabBandwidth {
				m.TerminalView.Close()
				return m, tea.Quit
			}
		case "esc":
			if m.ActiveTab != TabDashboard && (m.ActiveTab != TabTerminal || !m.TerminalView.IsFocused()) {
				m.ActiveTab = TabDashboard
				return m, nil
			}

		// Workspace Navigation (1 through 5)
		case "1":
			if m.ActiveTab != TabKnife {
				m.SetWorkspace(WorkspaceSystem)
				return m, nil
			}
		case "2":
			if m.ActiveTab != TabKnife {
				m.SetWorkspace(WorkspaceWebOps)
				return m, nil
			}
		case "3":
			if m.ActiveTab != TabKnife {
				m.SetWorkspace(WorkspaceDeploy)
				return m, nil
			}
		case "4":
			if m.ActiveTab != TabKnife {
				m.SetWorkspace(WorkspaceNetDB)
				return m, nil
			}

		// Left / Right arrow navigation for workspaces
		case "left", "[":
			if m.ActiveTab != TabDisk && m.ActiveTab != TabScanner && m.ActiveTab != TabHTTP && m.ActiveTab != TabDNS && m.ActiveTab != TabKnife && m.ActiveTab != TabCertbot {
				m.PrevWorkspace()
				return m, nil
			}
		case "right", "]":
			if m.ActiveTab != TabDisk && m.ActiveTab != TabScanner && m.ActiveTab != TabHTTP && m.ActiveTab != TabDNS && m.ActiveTab != TabKnife && m.ActiveTab != TabCertbot {
				m.NextWorkspace()
				return m, nil
			}

		// Tool cycling: Tab is solely for switching to the next tool across all workspaces
		case "tab":
			if m.ActiveTab == TabDashboard {
				m.SetWorkspace(WorkspaceSystem)
				return m, nil
			}
			m.NextSubTab()
			return m, nil

		// Focus switching / Prev tool: Shift+Tab switches focus inside multi-input/multi-pane views,
		// or goes to the previous tool in single-pane views.
		case "shift+tab":
			if m.ActiveTab == TabDashboard {
				m.SetWorkspace(WorkspaceNetDB)
				return m, nil
			}
			if !m.hasInternalTabHandling() {
				m.PrevSubTab()
				return m, nil
			}
		case "i":
			if m.ActiveTab == TabTerminal {
				m.TerminalView.SetFocus(true)
				return m, nil
			}
		}
	}

	// Route active tab updates
	var cmd tea.Cmd
	switch m.ActiveTab {
	case TabDashboard:
		m.DashboardView, cmd = m.DashboardView.Update(msg)
	case TabMonitor:
		m.MonitorView, cmd = m.MonitorView.Update(msg)
	case TabContainers:
		m.ContainersView, cmd = m.ContainersView.Update(msg)
	case TabServices:
		m.ServicesView, cmd = m.ServicesView.Update(msg)
	case TabPorts:
		m.PortsView, cmd = m.PortsView.Update(msg)
	case TabNginx:
		m.NginxView, cmd = m.NginxView.Update(msg)
	case TabAutoNginx:
		m.AutoNginxView, cmd = m.AutoNginxView.Update(msg)
	case TabDeploy:
		m.DeployView, cmd = m.DeployView.Update(msg)
	case TabPHPFPM:
		m.PHPFPMView, cmd = m.PHPFPMView.Update(msg)
	case TabWorkers:
		m.WorkersView, cmd = m.WorkersView.Update(msg)
	case TabCertbot:
		m.CertbotView, cmd = m.CertbotView.Update(msg)
	case TabKnife:
		m.KnifeView, cmd = m.KnifeView.Update(msg)
	case TabSSL:
		m.SSLView, cmd = m.SSLView.Update(msg)
	case TabDatabase:
		m.DatabaseView, cmd = m.DatabaseView.Update(msg)
	case TabBandwidth:
		m.BandwidthView, cmd = m.BandwidthView.Update(msg)
	case TabScanner:
		m.ScannerView, cmd = m.ScannerView.Update(msg)
	case TabGit:
		m.GitView, cmd = m.GitView.Update(msg)
	case TabCI:
		m.CIView, cmd = m.CIView.Update(msg)
	case TabSSH:
		m.SSHView, cmd = m.SSHView.Update(msg)
	case TabEnv:
		m.EnvView, cmd = m.EnvView.Update(msg)
	case TabTimers:
		m.TimersView, cmd = m.TimersView.Update(msg)
	case TabDisk:
		m.DiskView, cmd = m.DiskView.Update(msg)
	case TabHTTP:
		m.HTTPView, cmd = m.HTTPView.Update(msg)
	case TabDNS:
		m.DNSView, cmd = m.DNSView.Update(msg)
	case TabTerminal:
		m.TerminalView, cmd = m.TerminalView.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
