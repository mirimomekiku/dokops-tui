package ports

import (
	"fmt"
	"sort"
	"syscall"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

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
		if m.confirmKill {
			switch msg.String() {
			case "y", "Y":
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
			case "n", "N", "esc":
				m.confirmKill = false
				m.killStatus = "Kill cancelled"
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "l":
			m.listenOnly = !m.listenOnly
			m.applyFilter()
		case "t":
			if m.protoFilter == "TCP" {
				m.protoFilter = "ALL"
			} else {
				m.protoFilter = "TCP"
			}
			m.applyFilter()
		case "u":
			if m.protoFilter == "UDP" {
				m.protoFilter = "ALL"
			} else {
				m.protoFilter = "UDP"
			}
			m.applyFilter()
		case "r", "R":
			m.isLoading = true
			cmds = append(cmds, m.FetchSockets())
		case "k":
			sel := m.getSelectedSocket()
			if sel != nil && sel.PID > 0 {
				m.killPID = sel.PID
				m.killProcName = sel.ProcessName
				m.confirmKill = true
			} else {
				m.killStatus = "Selected socket has no associated PID or permission denied"
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
	if availableWidth > 80 {
		procWidth := availableWidth - (8 + 18 + 8 + 14 + 22 + 8 + 10)
		if procWidth < 20 {
			procWidth = 20
		}
		cols := []table.Column{
			{Title: "PROTO", Width: 8},
			{Title: "LOCAL ADDRESS", Width: 18},
			{Title: "PORT", Width: 8},
			{Title: "STATUS", Width: 14},
			{Title: "REMOTE ADDRESS", Width: 22},
			{Title: "PID", Width: 8},
			{Title: "PROCESS NAME", Width: procWidth},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	listenLabel := "LISTEN ONLY [l]"
	if !m.listenOnly {
		listenLabel = "ALL SOCKETS [l]"
	}

	var listeningCount, establishedCount int
	for _, s := range m.sockets {
		if s.Status == "LISTEN" || s.Protocol == "UDP" {
			listeningCount++
		} else if s.Status == "ESTABLISHED" {
			establishedCount++
		}
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Listening Ports ", listeningCount)),
		" ",
		theme.BadgeInfo.Render(fmt.Sprintf(" %d Established ", establishedCount)),
		"   ",
		theme.BadgeWarning.Render(fmt.Sprintf(" View: %s ", listenLabel)),
		" ",
		theme.BadgeSecondary.Render(fmt.Sprintf(" Proto: %s [t/u] ", m.protoFilter)),
		" ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(Showing %d of %d sockets)", len(m.filteredSockets), len(m.sockets))),
	)

	statusLine := ""
	if m.confirmKill {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("⚠️ Terminate PID %d (%s) to release port? (y/N)", m.killPID, m.killProcName))
	} else if m.killStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.killStatus)
	}

	hintBar := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
		"[k: Kill Process / Free Port]  [l: Toggle Listen/All]  [t: Filter TCP]  [u: Filter UDP]  [r: Refresh]",
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, theme.CardTitleStyle.Render("🔌 LISTENING PORTS & PROCESS MAPPER (ss / lsof)"), "  ", statsBadge),
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
