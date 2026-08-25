package services

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/coreos/go-systemd/v22/dbus"

	"dok-ops/internal/theme"
)

type UnitItem struct {
	Name        string
	LoadState   string
	ActiveState string
	SubState    string
	Description string
}

type UnitsLoadedMsg struct {
	Units []UnitItem
	Err   error
}

type UnitLogsMsg struct {
	UnitName string
	Logs     string
}

type ServiceActionMsg struct {
	UnitName string
	Action   string
	Err      error
}

type FilterMode int

const (
	FilterAll FilterMode = iota
	FilterActive
	FilterFailed
	FilterInactive
)

type Model struct {
	units          []UnitItem
	filteredUnits  []UnitItem
	table          table.Model
	logsViewport   viewport.Model
	viewingLogs    bool
	activeLogUnit  string
	filter         FilterMode
	actionStatus   string
	isLoading      bool
	width          int
	height         int
	err            error
	systemdOffline bool
}

func New() Model {
	cols := []table.Column{
		{Title: "UNIT", Width: 28},
		{Title: "LOAD", Width: 10},
		{Title: "ACTIVE", Width: 10},
		{Title: "SUB", Width: 12},
		{Title: "DESCRIPTION", Width: 40},
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
		table:        t,
		logsViewport: vp,
		filter:       FilterAll,
		isLoading:    true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchUnits()
}

func (m Model) FetchUnits() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := dbus.NewSystemdConnectionContext(ctx)
		if err == nil {
			defer conn.Close()
			statuses, err := conn.ListUnitsContext(ctx)
			if err == nil {
				var units []UnitItem
				for _, st := range statuses {
					// Focus primarily on services, sockets, timers
					if strings.HasSuffix(st.Name, ".service") || strings.HasSuffix(st.Name, ".socket") || strings.HasSuffix(st.Name, ".timer") {
						units = append(units, UnitItem{
							Name:        st.Name,
							LoadState:   st.LoadState,
							ActiveState: st.ActiveState,
							SubState:    st.SubState,
							Description: st.Description,
						})
					}
				}
				return UnitsLoadedMsg{Units: units}
			}
		}

		// Fallback to systemctl CLI
		out, cliErr := exec.CommandContext(ctx, "systemctl", "list-units", "--all", "--type=service", "--no-pager", "--plain", "--no-legend").Output()
		if cliErr != nil {
			return UnitsLoadedMsg{Err: fmt.Errorf("systemd unavailable: %v", cliErr)}
		}

		var units []UnitItem
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				name := fields[0]
				load := fields[1]
				active := fields[2]
				sub := fields[3]
				desc := ""
				if len(fields) > 4 {
					desc = strings.Join(fields[4:], " ")
				}
				units = append(units, UnitItem{
					Name:        name,
					LoadState:   load,
					ActiveState: active,
					SubState:    sub,
					Description: desc,
				})
			}
		}

		return UnitsLoadedMsg{Units: units}
	}
}

func (m Model) FetchLogs(unitName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "journalctl", "-u", unitName, "-n", "50", "--no-pager")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return UnitLogsMsg{UnitName: unitName, Logs: fmt.Sprintf("Error fetching journal logs: %v\n%s", err, string(out))}
		}

		logStr := strings.TrimSpace(string(out))
		if logStr == "" {
			logStr = "(No journal logs found for " + unitName + ")"
		}

		return UnitLogsMsg{UnitName: unitName, Logs: logStr}
	}
}

func (m Model) PerformAction(unitName, action string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var cmd *exec.Cmd
		switch action {
		case "start":
			cmd = exec.CommandContext(ctx, "systemctl", "start", unitName)
		case "stop":
			cmd = exec.CommandContext(ctx, "systemctl", "stop", unitName)
		case "restart":
			cmd = exec.CommandContext(ctx, "systemctl", "restart", unitName)
		default:
			return ServiceActionMsg{UnitName: unitName, Action: action, Err: fmt.Errorf("unknown action")}
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			return ServiceActionMsg{UnitName: unitName, Action: action, Err: fmt.Errorf("%v: %s", err, string(out))}
		}

		return ServiceActionMsg{UnitName: unitName, Action: action}
	}
}

func (m *Model) applyFilter() {
	m.filteredUnits = nil
	for _, u := range m.units {
		switch m.filter {
		case FilterActive:
			if u.ActiveState == "active" {
				m.filteredUnits = append(m.filteredUnits, u)
			}
		case FilterFailed:
			if u.ActiveState == "failed" || u.SubState == "failed" {
				m.filteredUnits = append(m.filteredUnits, u)
			}
		case FilterInactive:
			if u.ActiveState == "inactive" {
				m.filteredUnits = append(m.filteredUnits, u)
			}
		default:
			m.filteredUnits = append(m.filteredUnits, u)
		}
	}

	// Sort failed first, then alphabetically
	sort.Slice(m.filteredUnits, func(i, j int) bool {
		iFailed := m.filteredUnits[i].ActiveState == "failed"
		jFailed := m.filteredUnits[j].ActiveState == "failed"
		if iFailed != jFailed {
			return iFailed
		}
		return m.filteredUnits[i].Name < m.filteredUnits[j].Name
	})

	m.updateTableRows()
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, u := range m.filteredUnits {
		rows = append(rows, table.Row{
			u.Name,
			u.LoadState,
			u.ActiveState,
			u.SubState,
			u.Description,
		})
	}
	m.table.SetRows(rows)
}

func (m Model) getSelectedUnit() *UnitItem {
	if len(m.filteredUnits) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.filteredUnits) {
		return &m.filteredUnits[idx]
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

	case UnitsLoadedMsg:
		m.isLoading = false
		if msg.Err != nil {
			m.systemdOffline = true
			m.err = msg.Err
		} else {
			m.systemdOffline = false
			m.err = nil
			m.units = msg.Units
			m.applyFilter()
		}

	case UnitLogsMsg:
		m.logsViewport.SetContent(msg.Logs)
		m.logsViewport.GotoBottom()

	case ServiceActionMsg:
		if msg.Err != nil {
			m.actionStatus = fmt.Sprintf("Failed to %s %s: %v", msg.Action, msg.UnitName, msg.Err)
		} else {
			m.actionStatus = fmt.Sprintf("Successfully executed '%s' on %s", msg.Action, msg.UnitName)
			cmds = append(cmds, m.FetchUnits())
		}

	case tea.KeyMsg:
		if m.viewingLogs {
			switch msg.String() {
			case "esc", "q":
				m.viewingLogs = false
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.logsViewport, vpCmd = m.logsViewport.Update(msg)
				return m, vpCmd
			}
		}

		switch msg.String() {
		case "f":
			m.filter = (m.filter + 1) % 4
			m.applyFilter()
		case "R":
			m.isLoading = true
			cmds = append(cmds, m.FetchUnits())
		case "r":
			sel := m.getSelectedUnit()
			if sel != nil {
				m.actionStatus = fmt.Sprintf("Restarting %s...", sel.Name)
				cmds = append(cmds, m.PerformAction(sel.Name, "restart"))
			}
		case "u", "enter":
			sel := m.getSelectedUnit()
			if sel != nil {
				m.actionStatus = fmt.Sprintf("Starting %s...", sel.Name)
				cmds = append(cmds, m.PerformAction(sel.Name, "start"))
			}
		case "s":
			sel := m.getSelectedUnit()
			if sel != nil {
				m.actionStatus = fmt.Sprintf("Stopping %s...", sel.Name)
				cmds = append(cmds, m.PerformAction(sel.Name, "stop"))
			}
		case "l":
			sel := m.getSelectedUnit()
			if sel != nil {
				m.viewingLogs = true
				m.activeLogUnit = sel.Name
				m.logsViewport.SetContent("Loading journal logs for " + sel.Name + "...")
				cmds = append(cmds, m.FetchLogs(sel.Name))
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
	contentHeight := m.height - 10
	if contentHeight < 6 {
		contentHeight = 6
	}
	m.table.SetHeight(contentHeight)

	availableWidth := m.width - 6
	if availableWidth > 70 {
		descWidth := availableWidth - (28 + 10 + 10 + 12 + 10)
		if descWidth < 20 {
			descWidth = 20
		}
		cols := []table.Column{
			{Title: "UNIT", Width: 28},
			{Title: "LOAD", Width: 10},
			{Title: "ACTIVE", Width: 10},
			{Title: "SUB", Width: 12},
			{Title: "DESCRIPTION", Width: descWidth},
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
	m.logsViewport.Width = vpWidth
	m.logsViewport.Height = vpHeight
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.systemdOffline {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorWarning).
			Padding(1, 2).
			Width(contentWidth).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					theme.BadgeWarning.Render(" SYSTEMD NOT DETECTED "),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorText).Bold(true).Render("Unable to connect to systemd DBus socket or execute systemctl."),
					lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("Details: %v", m.err)),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("💡 Tip: This host may be a container, non-systemd Linux distribution, or lacking DBus permissions."),
					lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("Press [R] to retry connection."),
				),
			)
	}

	if m.viewingLogs {
		logsHeader := lipgloss.JoinHorizontal(lipgloss.Center,
			theme.BadgeInfo.Render(" JOURNALCTL LOGS "),
			" ",
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render(m.activeLogUnit),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Esc / q: Close | j/k: Scroll]"),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			logsHeader,
			"",
			m.logsViewport.View(),
		)
	}

	// Status counts
	var active, failed, inactive int
	for _, u := range m.units {
		if u.ActiveState == "failed" || u.SubState == "failed" {
			failed++
		} else if u.ActiveState == "active" {
			active++
		} else {
			inactive++
		}
	}

	filterName := "ALL"
	switch m.filter {
	case FilterActive:
		filterName = "ACTIVE ONLY"
	case FilterFailed:
		filterName = "FAILED ONLY"
	case FilterInactive:
		filterName = "INACTIVE ONLY"
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Active ", active)),
		" ",
		theme.BadgeDanger.Render(fmt.Sprintf(" %d Failed ", failed)),
		" ",
		theme.BadgeWarning.Render(fmt.Sprintf(" %d Inactive ", inactive)),
		"   ",
		theme.BadgeInfo.Render(fmt.Sprintf(" Filter: %s [f] ", filterName)),
		" ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(Showing %d of %d units)", len(m.filteredUnits), len(m.units))),
	)

	statusLine := ""
	if m.actionStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.actionStatus)
	}

	hintBar := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
		"[u/Enter: Start]  [s: Stop]  [r: Restart]  [l: Journal Logs]  [f: Filter]  [R: Refresh]",
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, theme.CardTitleStyle.Render("⚙️ SYSTEMD SERVICES (systemctl)"), "  ", statsBadge),
		hintBar,
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
