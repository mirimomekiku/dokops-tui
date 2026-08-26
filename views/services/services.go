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

	"dok-ops/internal/actionmenu"
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
	actionMenu     actionmenu.Model
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
		m.setLogContent(msg.Logs)
		m.logsViewport.GotoBottom()

	case ServiceActionMsg:
		if msg.Err != nil {
			m.actionStatus = fmt.Sprintf("Failed to %s %s: %v", msg.Action, msg.UnitName, msg.Err)
		} else {
			m.actionStatus = fmt.Sprintf("Successfully executed '%s' on %s", msg.Action, msg.UnitName)
			cmds = append(cmds, m.FetchUnits())
		}

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				sel := m.getSelectedUnit()
				switch action {
				case "start":
					if sel != nil {
						m.actionStatus = fmt.Sprintf("Starting %s...", sel.Name)
						cmds = append(cmds, m.PerformAction(sel.Name, "start"))
					}
				case "stop":
					if sel != nil {
						m.actionStatus = fmt.Sprintf("Stopping %s...", sel.Name)
						cmds = append(cmds, m.PerformAction(sel.Name, "stop"))
					}
				case "restart":
					if sel != nil {
						m.actionStatus = fmt.Sprintf("Restarting %s...", sel.Name)
						cmds = append(cmds, m.PerformAction(sel.Name, "restart"))
					}
				case "logs":
					if sel != nil {
						m.viewingLogs = true
						m.activeLogUnit = sel.Name
						m.setLogContent("Loading journal logs for " + sel.Name + "...")
						cmds = append(cmds, m.FetchLogs(sel.Name))
					}
				case "filter_active":
					m.filter = FilterActive
					m.applyFilter()
				case "filter_failed":
					m.filter = FilterFailed
					m.applyFilter()
				case "filter_all":
					m.filter = FilterAll
					m.applyFilter()
				case "refresh":
					m.isLoading = true
					cmds = append(cmds, m.FetchUnits())
				}
			}
			return m, tea.Batch(cmds...)
		}

		if m.viewingLogs {
			switch msg.String() {
			case "esc", "q", "tab", "shift+tab":
				m.viewingLogs = false
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.logsViewport, vpCmd = m.logsViewport.Update(msg)
				return m, vpCmd
			}
		}

		switch msg.String() {
		case "space":
			sel := m.getSelectedUnit()
			title := "Systemd Services"
			subtitle := "Select an action"
			if sel != nil {
				title = "Actions: " + sel.Name
				subtitle = fmt.Sprintf("Active: %s (%s)", sel.ActiveState, sel.SubState)
			}
			items := []actionmenu.Item{
				{Key: "logs", Title: "View Journal Logs", Description: "Inspect journalctl logs"},
				{Key: "restart", Title: "Restart Service", Description: "systemctl restart"},
				{Key: "start", Title: "Start Service", Description: "systemctl start"},
				{Key: "stop", Title: "Stop Service", Description: "systemctl stop", Danger: true},
				{Key: "filter_active", Title: "Filter: Active Only", Description: "Show only active units"},
				{Key: "filter_failed", Title: "Filter: Failed Only", Description: "Show failed/degraded units"},
				{Key: "filter_all", Title: "Filter: All Units", Description: "Show all loaded units"},
				{Key: "refresh", Title: "Refresh List", Description: "Reload systemctl units"},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil

		case "enter":
			// Primary action: View Journal Logs
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
	contentHeight := m.height - 6
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

func (m *Model) setLogContent(text string) {
	w := m.logsViewport.Width - 2
	if w < 20 {
		w = 20
	}
	m.logsViewport.SetContent(lipgloss.NewStyle().Width(w).Render(text))
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.systemdOffline {
		return lipgloss.NewStyle().
			Foreground(theme.ColorWarning).
			Padding(1, 2).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					lipgloss.NewStyle().Bold(true).Foreground(theme.ColorWarning).Render("Systemd not detected"),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorText).Render("Unable to connect to systemd DBus socket or execute systemctl."),
					lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("Details: %v", m.err)),
				),
			)
	}

	if m.viewingLogs {
		logsHeader := lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("Logs: "+m.activeLogUnit),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Esc / q: Close  j/k: Scroll"),
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
		filterName = "active"
	case FilterFailed:
		filterName = "failed"
	case FilterInactive:
		filterName = "inactive"
	}

	statusHeader := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Services"),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("● %d active", active)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorDanger).Render(fmt.Sprintf("⨯ %d failed", failed)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("○ %d inactive", inactive)),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("filter: %s (showing %d of %d)", filterName, len(m.filteredUnits), len(m.units))),
	)

	statusLine := ""
	if m.actionStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.actionStatus)
	}

	elements := []string{statusHeader}
	if statusLine != "" {
		elements = append(elements, statusLine)
	}
	elements = append(elements, "", m.table.View())

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, elements...))

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
