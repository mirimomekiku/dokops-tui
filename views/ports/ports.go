package ports

import (
	"fmt"
	"sort"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type SocketItem struct {
	Protocol    string
	LocalIP     string
	LocalPort   uint32
	RemoteAddr  string
	Status      string
	PID         int32
	ProcessName string
}

type SocketsLoadedMsg struct {
	Sockets []SocketItem
	Err     error
}

type Model struct {
	sockets         []SocketItem
	filteredSockets []SocketItem
	table           table.Model
	actionMenu      actionmenu.Model
	listenOnly      bool
	protoFilter     string // "ALL", "TCP", "UDP"
	confirmKill     bool
	killPID         int32
	killProcName    string
	killStatus      string
	isLoading       bool
	width           int
	height          int
	err             error
}

func New() Model {
	cols := []table.Column{
		{Title: "PROTO", Width: 8},
		{Title: "LOCAL ADDRESS", Width: 18},
		{Title: "PORT", Width: 8},
		{Title: "STATUS", Width: 14},
		{Title: "REMOTE ADDRESS", Width: 22},
		{Title: "PID", Width: 8},
		{Title: "PROCESS NAME", Width: 25},
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

	return Model{
		table:       t,
		listenOnly:  true,
		protoFilter: "ALL",
		isLoading:   true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchSockets()
}

func (m Model) FetchSockets() tea.Cmd {
	return func() tea.Msg {
		conns, err := psnet.Connections("all")
		if err != nil {
			return SocketsLoadedMsg{Err: err}
		}

		// Cache process names by PID
		procNames := make(map[int32]string)

		var sockets []SocketItem
		for _, c := range conns {
			proto := "TCP"
			switch c.Type {
			case 1:
				proto = "TCP"
			case 2:
				proto = "UDP"
			default:
				proto = fmt.Sprintf("TYPE-%d", c.Type)
			}

			status := c.Status
			if status == "" {
				if proto == "UDP" {
					status = "UNCONN"
				} else {
					status = "NONE"
				}
			}

			procName := "-"
			if c.Pid > 0 {
				if name, ok := procNames[c.Pid]; ok {
					procName = name
				} else {
					if p, err := process.NewProcess(c.Pid); err == nil {
						if n, err := p.Name(); err == nil && n != "" {
							procName = n
						} else {
							procName = fmt.Sprintf("PID:%d", c.Pid)
						}
					} else {
						procName = fmt.Sprintf("PID:%d", c.Pid)
					}
					procNames[c.Pid] = procName
				}
			}

			remote := "-"
			if c.Raddr.IP != "" {
				remote = fmt.Sprintf("%s:%d", c.Raddr.IP, c.Raddr.Port)
			}

			localIP := c.Laddr.IP
			if localIP == "" {
				localIP = "*"
			}

			sockets = append(sockets, SocketItem{
				Protocol:    proto,
				LocalIP:     localIP,
				LocalPort:   c.Laddr.Port,
				RemoteAddr:  remote,
				Status:      status,
				PID:         c.Pid,
				ProcessName: procName,
			})
		}

		return SocketsLoadedMsg{Sockets: sockets}
	}
}

func (m *Model) applyFilter() {
	m.filteredSockets = nil
	for _, s := range m.sockets {
		if m.listenOnly && s.Status != "LISTEN" && s.Protocol != "UDP" {
			continue
		}
		if m.protoFilter == "TCP" && s.Protocol != "TCP" {
			continue
		}
		if m.protoFilter == "UDP" && s.Protocol != "UDP" {
			continue
		}
		m.filteredSockets = append(m.filteredSockets, s)
	}

	// Sort by Port ascending, then PID
	sort.Slice(m.filteredSockets, func(i, j int) bool {
		if m.filteredSockets[i].LocalPort != m.filteredSockets[j].LocalPort {
			return m.filteredSockets[i].LocalPort < m.filteredSockets[j].LocalPort
		}
		return m.filteredSockets[i].PID < m.filteredSockets[j].PID
	})

	m.updateTableRows()
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, s := range m.filteredSockets {
		pidStr := "-"
		if s.PID > 0 {
			pidStr = fmt.Sprintf("%d", s.PID)
		}
		rows = append(rows, table.Row{
			s.Protocol,
			s.LocalIP,
			fmt.Sprintf("%d", s.LocalPort),
			s.Status,
			s.RemoteAddr,
			pidStr,
			s.ProcessName,
		})
	}
	m.table.SetRows(rows)
}

func (m Model) getSelectedSocket() *SocketItem {
	if len(m.filteredSockets) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.filteredSockets) {
		return &m.filteredSockets[idx]
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

	case SocketsLoadedMsg:
		m.isLoading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.sockets = msg.Sockets
			m.applyFilter()
		}

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				sel := m.getSelectedSocket()
				switch action {
				case "kill":
					if sel != nil && sel.PID > 0 {
						m.killPID = sel.PID
						m.killProcName = sel.ProcessName
						m.confirmKill = true
					} else {
						m.killStatus = "Selected socket has no associated PID"
					}
				case "toggle_listen":
					m.listenOnly = !m.listenOnly
					m.applyFilter()
				case "filter_tcp":
					m.protoFilter = "TCP"
					m.applyFilter()
				case "filter_udp":
					m.protoFilter = "UDP"
					m.applyFilter()
				case "filter_all":
					m.protoFilter = "ALL"
					m.applyFilter()
				case "refresh":
					m.isLoading = true
					cmds = append(cmds, m.FetchSockets())
				}
			}
			return m, tea.Batch(cmds...)
		}

		if m.confirmKill {
			switch msg.String() {
			case "y", "Y", "enter":
				if m.killPID > 0 {
					err := syscall.Kill(int(m.killPID), syscall.SIGTERM)
					if err != nil {
						m.killStatus = fmt.Sprintf("Failed to kill PID %d (%s): %v", m.killPID, m.killProcName, err)
					} else {
						m.killStatus = fmt.Sprintf("Successfully killed PID %d (%s) — port released", m.killPID, m.killProcName)
						cmds = append(cmds, m.FetchSockets())
					}
				}
				m.confirmKill = false
			case "n", "N", "esc", "space":
				m.confirmKill = false
				m.killStatus = "Kill cancelled"
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "space":
			sel := m.getSelectedSocket()
			title := "Listening Ports"
			subtitle := "Select an action"
			if sel != nil {
				title = fmt.Sprintf("Actions: Port %d (%s)", sel.LocalPort, sel.Protocol)
				subtitle = fmt.Sprintf("Process: %s (PID %d)", sel.ProcessName, sel.PID)
			}
			items := []actionmenu.Item{
				{Key: "kill", Title: "Kill Process (Free Port)", Description: "Terminate process holding this port"},
				{Key: "toggle_listen", Title: "Toggle Listen / All", Description: "Switch between listening & active sockets"},
				{Key: "filter_tcp", Title: "Filter: TCP Only", Description: "Display TCP sockets"},
				{Key: "filter_udp", Title: "Filter: UDP Only", Description: "Display UDP sockets"},
				{Key: "filter_all", Title: "Filter: All Protocols", Description: "Display TCP and UDP sockets"},
				{Key: "refresh", Title: "Refresh Sockets", Description: "Rescan network connections"},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil

		case "enter":
			sel := m.getSelectedSocket()
			if sel != nil {
				m.killStatus = fmt.Sprintf("Selected Port %d (%s) -> %s (PID: %d)", sel.LocalPort, sel.Protocol, sel.ProcessName, sel.PID)
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
	// Reserve: card border(2) + stats bar(1) + hint bar(1) + status(1) + table header(1) + padding(2) = 8
	contentHeight := m.height - 8
	if contentHeight < 4 {
		contentHeight = 4
	}
	m.table.SetHeight(contentHeight)

	availableWidth := m.width - 6
	if availableWidth > 0 {
		// Fixed cols: PROTO(6) PORT(6) STATUS(12) PID(6) = 30; remainder split between LOCAL, REMOTE, PROCESS
		remainder := availableWidth - 30
		if remainder < 30 {
			remainder = 30
		}
		localW := remainder / 3
		remoteW := remainder / 3
		procW := remainder - localW - remoteW
		if localW < 14 {
			localW = 14
		}
		if remoteW < 14 {
			remoteW = 14
		}
		if procW < 14 {
			procW = 14
		}
		cols := []table.Column{
			{Title: "PROTO", Width: 6},
			{Title: "LOCAL", Width: localW},
			{Title: "PORT", Width: 6},
			{Title: "STATUS", Width: 12},
			{Title: "REMOTE", Width: remoteW},
			{Title: "PID", Width: 6},
			{Title: "PROCESS", Width: procW},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var tcpCount, udpCount, listenCount int
	for _, s := range m.sockets {
		if strings.HasPrefix(strings.ToLower(s.Protocol), "tcp") {
			tcpCount++
		} else {
			udpCount++
		}
		if s.Status == "LISTEN" {
			listenCount++
		}
	}

	headerLine := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Ports & Sockets"),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("listening: %d", listenCount)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(fmt.Sprintf("TCP: %d", tcpCount)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorWarning).Render(fmt.Sprintf("UDP: %d", udpCount)),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(total: %d)", len(m.sockets))),
	)

	statusLine := ""
	if m.confirmKill {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("Terminate PID %d (%s) to release port? (y/N)", m.killPID, m.killProcName))
	} else if m.killStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("  " + m.killStatus)
	}

	elements := []string{
		headerLine,
	}
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
