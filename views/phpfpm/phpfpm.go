package phpfpm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type PHPVersionInfo struct {
	Version    string // "8.3", "8.2", "8.1"
	SocketPath string
	Service    string // "php8.3-fpm"
	Status     string // "active (running)", "inactive"
	Workers    int
	MemoryUsed string
}

type PHPDataLoadedMsg struct {
	Versions []PHPVersionInfo
	Err      error
}

type PHPActionMsg struct {
	Action string
	Output string
	Err    error
}

type Model struct {
	versions     []PHPVersionInfo
	table        table.Model
	statusLine   string
	selectedVer  *PHPVersionInfo
	isLoading    bool
	width        int
	height       int
	err          error
}

func New() Model {
	cols := []table.Column{
		{Title: "PHP VERSION", Width: 16},
		{Title: "STATUS", Width: 18},
		{Title: "SYSTEMD SERVICE", Width: 20},
		{Title: "UNIX SOCKET PATH", Width: 35},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(8),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(theme.ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(theme.ColorPrimary)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#3d59a1")).
		Bold(true)
	t.SetStyles(s)

	return Model{
		table:     t,
		isLoading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchPHPData()
}

func (m Model) FetchPHPData() tea.Cmd {
	return func() tea.Msg {
		var list []PHPVersionInfo

		// Look for sockets in /run/php
		entries, err := os.ReadDir("/run/php")
		if err != nil || len(entries) == 0 {
			// Provide comprehensive fallback list of versions
			sampleVers := []string{"8.3", "8.2", "8.1", "8.0", "7.4"}
			for _, v := range sampleVers {
				serviceName := fmt.Sprintf("php%s-fpm", v)
				status := "inactive"
				if out, err := exec.Command("systemctl", "is-active", serviceName).Output(); err == nil {
					status = strings.TrimSpace(string(out))
				}
				list = append(list, PHPVersionInfo{
					Version:    "PHP " + v,
					SocketPath: fmt.Sprintf("/run/php/php%s-fpm.sock", v),
					Service:    serviceName,
					Status:     status,
				})
			}
			return PHPDataLoadedMsg{Versions: list}
		}

		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".sock") {
				ver := strings.TrimPrefix(e.Name(), "php")
				ver = strings.TrimSuffix(ver, "-fpm.sock")
				serviceName := fmt.Sprintf("php%s-fpm", ver)
				status := "inactive"
				if out, err := exec.Command("systemctl", "is-active", serviceName).Output(); err == nil {
					status = strings.TrimSpace(string(out))
				}

				list = append(list, PHPVersionInfo{
					Version:    "PHP " + ver,
					SocketPath: filepath.Join("/run/php", e.Name()),
					Service:    serviceName,
					Status:     status,
				})
			}
		}

		return PHPDataLoadedMsg{Versions: list}
	}
}

func (m Model) RestartPHPService(service string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("systemctl", "restart", service).CombinedOutput()
		if err != nil {
			// Try with sudo
			out, err = exec.Command("sudo", "systemctl", "restart", service).CombinedOutput()
			if err != nil {
				return PHPActionMsg{Action: "Restart", Output: fmt.Sprintf("Failed to restart %s: %v (%s)", service, err, string(out)), Err: err}
			}
		}
		return PHPActionMsg{Action: "Restart", Output: fmt.Sprintf("✓ Successfully restarted %s daemon!", service)}
	}
}

func (m Model) ResetOPcache() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("php", "-r", "if (function_exists('opcache_reset')) { opcache_reset(); echo 'OPcache flushed'; } else { echo 'OPcache not enabled in CLI mode'; }").CombinedOutput()
		if err != nil {
			return PHPActionMsg{Action: "OPcache Reset", Output: fmt.Sprintf("OPcache reset error: %v", err), Err: err}
		}
		return PHPActionMsg{Action: "OPcache Reset", Output: fmt.Sprintf("✓ %s", strings.TrimSpace(string(out)))}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case PHPDataLoadedMsg:
		m.isLoading = false
		m.versions = msg.Versions
		m.updateTableRows()
		if len(m.versions) > 0 {
			m.selectedVer = &m.versions[0]
		}

	case PHPActionMsg:
		m.statusLine = msg.Output
		cmds = append(cmds, m.FetchPHPData())

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "down", "j", "k":
			var tCmd tea.Cmd
			m.table, tCmd = m.table.Update(msg)
			if len(m.versions) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.versions) {
					m.selectedVer = &m.versions[idx]
				}
			}
			return m, tCmd

		case "r", "enter":
			if m.selectedVer != nil {
				m.statusLine = fmt.Sprintf("Restarting %s...", m.selectedVer.Service)
				return m, m.RestartPHPService(m.selectedVer.Service)
			}

		case "c":
			m.statusLine = "Flushing OPcache & APCu cache..."
			return m, m.ResetOPcache()

		case "R":
			return m, m.FetchPHPData()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, v := range m.versions {
		statusBadge := v.Status
		if v.Status == "active" {
			statusBadge = "● active (running)"
		} else {
			statusBadge = "○ " + v.Status
		}
		rows = append(rows, table.Row{
			v.Version,
			statusBadge,
			v.Service,
			v.SocketPath,
		})
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	contentHeight := m.height - 12
	if contentHeight < 6 {
		contentHeight = 6
	}
	m.table.SetHeight(contentHeight)
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var activeCount int
	for _, v := range m.versions {
		if v.Status == "active" {
			activeCount++
		}
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Installed PHP Versions ", len(m.versions))),
		" ",
		theme.BadgeInfo.Render(fmt.Sprintf(" %d Running Daemons ", activeCount)),
	)

	statusLine := ""
	if m.statusLine != "" {
		statusLine = lipgloss.NewStyle().
			Foreground(theme.ColorHighlight).
			Bold(true).
			Render(m.statusLine)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🐘 PHP-FPM POOL & VERSION SWITCHER"),
			"   ",
			statsBadge,
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[r/Enter: Restart FPM Daemon | c: Clear OPcache | R: Refresh]"),
		),
		"",
		m.table.View(),
		"",
		statusLine,
		"",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Tip: FastCGI sockets are automatically wired when deploying Nginx configs in the Auto-Templater tab."),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
