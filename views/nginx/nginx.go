package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type NginxSite struct {
	Name       string
	Path       string
	Enabled    bool
	LinkTarget string
	ServerName string
	ListenPort string
}

type SitesLoadedMsg struct {
	Sites      []NginxSite
	TestOutput string
	TestErr    error
	Err        error
}

type SyntaxTestMsg struct {
	Output string
	Err    error
}

type NginxActionMsg struct {
	Action string
	Err    error
}

type Model struct {
	sites          []NginxSite
	table          table.Model
	configViewport viewport.Model
	viewingConfig  bool
	activeConfig   string
	activeSiteName string
	syntaxStatus   string
	syntaxOK       bool
	actionStatus   string
	isLoading      bool
	width          int
	height         int
	nginxDir       string
	err            error
	nginxMissing   bool
}

func New() Model {
	cols := []table.Column{
		{Title: "SITE / CONFIG", Width: 25},
		{Title: "STATUS", Width: 12},
		{Title: "PORTS", Width: 10},
		{Title: "SERVER NAME / DOMAIN", Width: 25},
		{Title: "PATH", Width: 35},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(10),
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

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1)

	return Model{
		table:          t,
		configViewport: vp,
		nginxDir:       "/etc/nginx",
		isLoading:      true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchSites()
}

func (m Model) FetchSites() tea.Cmd {
	return func() tea.Msg {
		sitesAvail := filepath.Join(m.nginxDir, "sites-available")
		sitesEnabled := filepath.Join(m.nginxDir, "sites-enabled")
		confD := filepath.Join(m.nginxDir, "conf.d")

		siteMap := make(map[string]*NginxSite)

		// 1. Check sites-available
		if entries, err := os.ReadDir(sitesAvail); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				path := filepath.Join(sitesAvail, e.Name())
				sName, port := parseNginxConf(path)
				siteMap[e.Name()] = &NginxSite{
					Name:       e.Name(),
					Path:       path,
					Enabled:    false,
					ServerName: sName,
					ListenPort: port,
				}
			}
		}

		// 2. Check sites-enabled
		if entries, err := os.ReadDir(sitesEnabled); err == nil {
			for _, e := range entries {
				path := filepath.Join(sitesEnabled, e.Name())
				target, _ := os.Readlink(path)
				if site, exists := siteMap[e.Name()]; exists {
					site.Enabled = true
					site.LinkTarget = target
				} else {
					sName, port := parseNginxConf(path)
					siteMap[e.Name()] = &NginxSite{
						Name:       e.Name(),
						Path:       path,
						Enabled:    true,
						LinkTarget: target,
						ServerName: sName,
						ListenPort: port,
					}
				}
			}
		}

		// 3. Check conf.d
		if entries, err := os.ReadDir(confD); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".conf") {
					path := filepath.Join(confD, e.Name())
					sName, port := parseNginxConf(path)
					siteMap[e.Name()] = &NginxSite{
						Name:       e.Name(),
						Path:       path,
						Enabled:    true,
						ServerName: sName,
						ListenPort: port,
					}
				}
			}
		}

		var list []NginxSite
		for _, s := range siteMap {
			list = append(list, *s)
		}

		sort.Slice(list, func(i, j int) bool {
			if list[i].Enabled != list[j].Enabled {
				return list[i].Enabled // Enabled first
			}
			return list[i].Name < list[j].Name
		})

		// Run quick nginx -t
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tOut, tErr := exec.CommandContext(ctx, "nginx", "-t").CombinedOutput()

		var finalErr error
		if len(list) == 0 && tErr != nil {
			finalErr = fmt.Errorf("nginx directories not found or inaccessible")
		}

		return SitesLoadedMsg{
			Sites:      list,
			TestOutput: strings.TrimSpace(string(tOut)),
			TestErr:    tErr,
			Err:        finalErr,
		}
	}
}

func parseNginxConf(filePath string) (string, string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "-", "-"
	}

	serverName := "-"
	listenPort := "-"

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "server_name ") && serverName == "-" {
			val := strings.TrimPrefix(trimmed, "server_name ")
			val = strings.TrimSuffix(val, ";")
			serverName = strings.TrimSpace(val)
		}
		if strings.HasPrefix(trimmed, "listen ") && listenPort == "-" {
			val := strings.TrimPrefix(trimmed, "listen ")
			val = strings.TrimSuffix(val, ";")
			listenPort = strings.TrimSpace(val)
		}
	}

	return serverName, listenPort
}

func (m Model) ToggleSite(site NginxSite) tea.Cmd {
	return func() tea.Msg {
		sitesEnabled := filepath.Join(m.nginxDir, "sites-enabled")
		symlinkPath := filepath.Join(sitesEnabled, site.Name)

		var err error
		if site.Enabled {
			// Disable: remove symlink
			err = os.Remove(symlinkPath)
		} else {
			// Enable: create symlink
			err = os.Symlink(site.Path, symlinkPath)
		}

		action := "Enable"
		if site.Enabled {
			action = "Disable"
		}

		return NginxActionMsg{
			Action: fmt.Sprintf("%s site %s", action, site.Name),
			Err:    err,
		}
	}
}

func (m Model) TestSyntax() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "nginx", "-t").CombinedOutput()
		return SyntaxTestMsg{
			Output: strings.TrimSpace(string(out)),
			Err:    err,
		}
	}
}

func (m Model) ReloadNginx() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var cmd *exec.Cmd
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd = exec.CommandContext(ctx, "systemctl", "reload", "nginx")
		} else {
			cmd = exec.CommandContext(ctx, "nginx", "-s", "reload")
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			return NginxActionMsg{Action: "Reload Nginx", Err: fmt.Errorf("%v: %s", err, string(out))}
		}
		return NginxActionMsg{Action: "Reload Nginx"}
	}
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, s := range m.sites {
		status := "DISABLED"
		if s.Enabled {
			status = "ENABLED"
		}
		rows = append(rows, table.Row{
			s.Name,
			status,
			s.ListenPort,
			s.ServerName,
			s.Path,
		})
	}
	m.table.SetRows(rows)
}

func (m Model) getSelectedSite() *NginxSite {
	if len(m.sites) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.sites) {
		return &m.sites[idx]
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case SitesLoadedMsg:
		m.isLoading = false
		m.err = msg.Err
		if msg.Err != nil && len(msg.Sites) == 0 {
			m.nginxMissing = true
		} else {
			m.nginxMissing = false
			m.sites = msg.Sites
			m.syntaxStatus = msg.TestOutput
			m.syntaxOK = (msg.TestErr == nil)
			m.updateTableRows()
		}

	case SyntaxTestMsg:
		m.syntaxStatus = msg.Output
		m.syntaxOK = (msg.Err == nil)
		if msg.Err != nil {
			m.actionStatus = "Syntax test failed (see details above)"
		} else {
			m.actionStatus = "Syntax test passed: configuration is valid"
		}

	case NginxActionMsg:
		if msg.Err != nil {
			m.actionStatus = fmt.Sprintf("Failed to %s: %v", msg.Action, msg.Err)
		} else {
			m.actionStatus = fmt.Sprintf("Successfully executed '%s'", msg.Action)
			cmds = append(cmds, m.FetchSites(), m.TestSyntax())
		}

	case tea.KeyMsg:
		if m.viewingConfig {
			switch msg.String() {
			case "esc", "q":
				m.viewingConfig = false
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.configViewport, vpCmd = m.configViewport.Update(msg)
				return m, vpCmd
			}
		}

		switch msg.String() {
		case "e", " ":
			sel := m.getSelectedSite()
			if sel != nil {
				cmds = append(cmds, m.ToggleSite(*sel))
			}
		case "t":
			m.actionStatus = "Running `nginx -t`..."
			cmds = append(cmds, m.TestSyntax())
		case "r":
			m.actionStatus = "Reloading Nginx..."
			cmds = append(cmds, m.ReloadNginx())
		case "R":
			m.isLoading = true
			cmds = append(cmds, m.FetchSites())
		case "v", "enter":
			sel := m.getSelectedSite()
			if sel != nil {
				content, err := os.ReadFile(sel.Path)
				if err != nil {
					m.activeConfig = fmt.Sprintf("Error reading %s: %v", sel.Path, err)
				} else {
					m.activeConfig = string(content)
				}
				m.activeSiteName = sel.Name
				m.configViewport.SetContent(m.activeConfig)
				m.configViewport.GotoTop()
				m.viewingConfig = true
			}
		}
	}

	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	if tableCmd != nil {
		cmds = append(cmds, tableCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateLayout() {
	contentHeight := m.height - 12
	if contentHeight < 6 {
		contentHeight = 6
	}
	m.table.SetHeight(contentHeight)

	availableWidth := m.width - 6
	if availableWidth > 80 {
		pathWidth := availableWidth - (25 + 12 + 10 + 25 + 10)
		if pathWidth < 20 {
			pathWidth = 20
		}
		cols := []table.Column{
			{Title: "SITE / CONFIG", Width: 25},
			{Title: "STATUS", Width: 12},
			{Title: "PORTS", Width: 10},
			{Title: "SERVER NAME / DOMAIN", Width: 25},
			{Title: "PATH", Width: pathWidth},
		}
		m.table.SetColumns(cols)
	}

	vpWidth := m.width - 8
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 8
	if vpHeight < 10 {
		vpHeight = 10
	}
	m.configViewport.Width = vpWidth
	m.configViewport.Height = vpHeight
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.viewingConfig {
		configHeader := lipgloss.JoinHorizontal(lipgloss.Center,
			theme.BadgeInfo.Render(" NGINX CONFIG FILE "),
			" ",
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render(m.activeSiteName),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Esc / q: Close | j/k: Scroll]"),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			configHeader,
			"",
			m.configViewport.View(),
		)
	}

	if m.nginxMissing && len(m.sites) == 0 {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorWarning).
			Padding(1, 2).
			Width(contentWidth).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					theme.BadgeWarning.Render(" NGINX NOT DETECTED "),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorText).Bold(true).Render("No Nginx installation found at /etc/nginx or permission denied."),
					lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Ensure Nginx is installed (`sudo apt install nginx` or `nginx -V`) and readable."),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("Press [R] to rescan directory."),
				),
			)
	}

	// Syntax test banner
	var syntaxBadge string
	if m.syntaxOK {
		syntaxBadge = theme.BadgeSuccess.Render(" nginx -t: OK ")
	} else {
		syntaxBadge = theme.BadgeDanger.Render(" nginx -t: SYNTAX ERROR ")
	}

	var enabledCount, disabledCount int
	for _, s := range m.sites {
		if s.Enabled {
			enabledCount++
		} else {
			disabledCount++
		}
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Enabled ", enabledCount)),
		" ",
		theme.BadgeWarning.Render(fmt.Sprintf(" %d Disabled ", disabledCount)),
		"   ",
		syntaxBadge,
	)

	statusLine := ""
	if m.actionStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.actionStatus)
	}

	var syntaxDetails string
	if m.syntaxStatus != "" {
		syntaxDetails = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("Test Output: %s", m.syntaxStatus))
	}

	hintBar := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
		"[e/Space: Toggle Enable/Disable]  [t: Test Syntax (nginx -t)]  [r: Reload]  [v/Enter: View Config]  [R: Refresh]",
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, theme.CardTitleStyle.Render("🌐 NGINX SITE MANAGER & SYNTAX VERIFIER"), "  ", statsBadge),
		hintBar,
		syntaxDetails,
		statusLine,
		"",
		m.table.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
