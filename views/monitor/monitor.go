package monitor

import (
	"fmt"
	"sort"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"

	"dok-ops/internal/theme"
)

type ProcessInfo struct {
	PID      int32
	Name     string
	CPU      float64
	Mem      float32
	Username string
	Status   string
}

type StatsMsg struct {
	CPUUsage  []float64
	TotalMem  uint64
	UsedMem   uint64
	TotalSwap uint64
	UsedSwap  uint64
	Processes []ProcessInfo
	Err       error
}

type TickMsg time.Time

type SortField int

const (
	SortCPU SortField = iota
	SortMem
	SortPID
	SortName
)

type Model struct {
	width       int
	height      int
	cpuUsage    []float64
	totalMem    uint64
	usedMem     uint64
	totalSwap   uint64
	usedSwap    uint64
	processes   []ProcessInfo
	table       table.Model
	sortField   SortField
	sortDesc    bool
	killStatus  string
	killConfirm bool
	selectedPID int32
	err         error
}

func New() Model {
	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "USER", Width: 12},
		{Title: "CPU%", Width: 8},
		{Title: "MEM%", Width: 8},
		{Title: "STATUS", Width: 10},
		{Title: "COMMAND / NAME", Width: 35},
	}

	t := table.New(
		table.WithColumns(columns),
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
		table:     t,
		sortField: SortCPU,
		sortDesc:  true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(FetchSystemStats(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func FetchSystemStats() tea.Cmd {
	return func() tea.Msg {
		cpuPercent, _ := cpu.Percent(0, true)
		vMem, _ := mem.VirtualMemory()
		sMem, _ := mem.SwapMemory()
		procs, _ := process.Processes()

		var procList []ProcessInfo
		for _, p := range procs {
			name, err := p.Name()
			if err != nil || name == "" {
				name = "[unknown]"
			}
			cpuPct, _ := p.CPUPercent()
			memPct, _ := p.MemoryPercent()
			username, _ := p.Username()
			if username == "" {
				username = "system"
			}
			status, _ := p.Status()
			st := "Running"
			if len(status) > 0 {
				st = status[0]
			}

			procList = append(procList, ProcessInfo{
				PID:      p.Pid,
				Name:     name,
				CPU:      cpuPct,
				Mem:      memPct,
				Username: username,
				Status:   st,
			})
		}

		var totalMem, usedMem, totalSwap, usedSwap uint64
		if vMem != nil {
			totalMem = vMem.Total
			usedMem = vMem.Used
		}
		if sMem != nil {
			totalSwap = sMem.Total
			usedSwap = sMem.Used
		}

		return StatsMsg{
			CPUUsage:  cpuPercent,
			TotalMem:  totalMem,
			UsedMem:   usedMem,
			TotalSwap: totalSwap,
			UsedSwap:  usedSwap,
			Processes: procList,
		}
	}
}

func (m *Model) sortProcesses() {
	sort.Slice(m.processes, func(i, j int) bool {
		switch m.sortField {
		case SortCPU:
			if m.sortDesc {
				return m.processes[i].CPU > m.processes[j].CPU
			}
			return m.processes[i].CPU < m.processes[j].CPU
		case SortMem:
			if m.sortDesc {
				return m.processes[i].Mem > m.processes[j].Mem
			}
			return m.processes[i].Mem < m.processes[j].Mem
		case SortPID:
			if m.sortDesc {
				return m.processes[i].PID > m.processes[j].PID
			}
			return m.processes[i].PID < m.processes[j].PID
		case SortName:
			if m.sortDesc {
				return m.processes[i].Name > m.processes[j].Name
			}
			return m.processes[i].Name < m.processes[j].Name
		default:
			return m.processes[i].CPU > m.processes[j].CPU
		}
	})
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, p := range m.processes {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", p.PID),
			p.Username,
			fmt.Sprintf("%5.1f%%", p.CPU),
			fmt.Sprintf("%4.1f%%", p.Mem),
			p.Status,
			p.Name,
		})
	}
	m.table.SetRows(rows)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case TickMsg:
		cmds = append(cmds, FetchSystemStats(), tickCmd())

	case StatsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.cpuUsage = msg.CPUUsage
			m.totalMem = msg.TotalMem
			m.usedMem = msg.UsedMem
			m.totalSwap = msg.TotalSwap
			m.usedSwap = msg.UsedSwap
			m.processes = msg.Processes
			m.sortProcesses()
			m.updateTableRows()
		}

	case tea.KeyMsg:
		if m.killConfirm {
			switch msg.String() {
			case "y", "Y":
				if m.selectedPID > 0 {
					err := syscall.Kill(int(m.selectedPID), syscall.SIGTERM)
					if err != nil {
						m.killStatus = fmt.Sprintf("Failed to kill PID %d: %v", m.selectedPID, err)
					} else {
						m.killStatus = fmt.Sprintf("Successfully sent SIGTERM to PID %d", m.selectedPID)
						cmds = append(cmds, FetchSystemStats())
					}
				}
				m.killConfirm = false
			case "n", "N", "esc":
				m.killConfirm = false
				m.killStatus = "Kill cancelled"
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "c":
			if m.sortField == SortCPU {
				m.sortDesc = !m.sortDesc
			} else {
				m.sortField = SortCPU
				m.sortDesc = true
			}
			m.sortProcesses()
			m.updateTableRows()
		case "m":
			if m.sortField == SortMem {
				m.sortDesc = !m.sortDesc
			} else {
				m.sortField = SortMem
				m.sortDesc = true
			}
			m.sortProcesses()
			m.updateTableRows()
		case "p":
			if m.sortField == SortPID {
				m.sortDesc = !m.sortDesc
			} else {
				m.sortField = SortPID
				m.sortDesc = false
			}
			m.sortProcesses()
			m.updateTableRows()
		case "n":
			if m.sortField == SortName {
				m.sortDesc = !m.sortDesc
			} else {
				m.sortField = SortName
				m.sortDesc = false
			}
			m.sortProcesses()
			m.updateTableRows()
		case "r":
			cmds = append(cmds, FetchSystemStats())
		case "k":
			if len(m.table.Rows()) > 0 && m.table.Cursor() < len(m.processes) {
				selRow := m.table.SelectedRow()
				if len(selRow) > 0 {
					var pid int32
					fmt.Sscanf(selRow[0], "%d", &pid)
					m.selectedPID = pid
					m.killConfirm = true
				}
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
	tableHeight := m.height - 18
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.table.SetHeight(tableHeight)

	// Adjust column widths based on available width
	availableWidth := m.width - 6
	if availableWidth > 60 {
		cmdWidth := availableWidth - (8 + 12 + 8 + 8 + 10 + 10)
		if cmdWidth < 20 {
			cmdWidth = 20
		}
		cols := []table.Column{
			{Title: "PID", Width: 8},
			{Title: "USER", Width: 12},
			{Title: "CPU%", Width: 8},
			{Title: "MEM%", Width: 8},
			{Title: "STATUS", Width: 10},
			{Title: "COMMAND / NAME", Width: cmdWidth},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// 1. CPU Section
	halfWidth := (contentWidth - 4) / 2
	if halfWidth < 20 {
		halfWidth = 20
	}

	var cpuLines []string
	numCPUs := len(m.cpuUsage)
	for i := 0; i < numCPUs; i += 2 {
		left := ""
		right := ""
		leftPct := m.cpuUsage[i]
		left = theme.RenderGauge(fmt.Sprintf("CPU %2d", i+1), 7, halfWidth-18, leftPct, "")
		if i+1 < numCPUs {
			rightPct := m.cpuUsage[i+1]
			right = theme.RenderGauge(fmt.Sprintf("CPU %2d", i+2), 7, halfWidth-18, rightPct, "")
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(halfWidth).Render(left),
			"  ",
			lipgloss.NewStyle().Width(halfWidth).Render(right),
		)
		cpuLines = append(cpuLines, row)
	}

	cpuSection := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				theme.CardTitleStyle.Render("⚡ CPU CORES USAGE"),
				lipgloss.JoinVertical(lipgloss.Left, cpuLines...),
			),
		)

	// 2. Memory & Swap Section
	var memPct, swapPct float64
	if m.totalMem > 0 {
		memPct = (float64(m.usedMem) / float64(m.totalMem)) * 100.0
	}
	if m.totalSwap > 0 {
		swapPct = (float64(m.usedSwap) / float64(m.totalSwap)) * 100.0
	}

	memDetails := fmt.Sprintf("%s / %s", theme.FormatBytes(m.usedMem), theme.FormatBytes(m.totalMem))
	swapDetails := fmt.Sprintf("%s / %s", theme.FormatBytes(m.usedSwap), theme.FormatBytes(m.totalSwap))

	memGauge := theme.RenderGauge("RAM", 5, halfWidth-24, memPct, memDetails)
	swapGauge := theme.RenderGauge("SWAP", 5, halfWidth-24, swapPct, swapDetails)

	memRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(halfWidth).Render(memGauge),
		"  ",
		lipgloss.NewStyle().Width(halfWidth).Render(swapGauge),
	)

	memSection := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				theme.CardTitleStyle.Render("💾 MEMORY & SWAP"),
				memRow,
			),
		)

	// 3. Process Table Section
	sortIndicator := " [Sort: "
	switch m.sortField {
	case SortCPU:
		sortIndicator += "CPU"
	case SortMem:
		sortIndicator += "MEM"
	case SortPID:
		sortIndicator += "PID"
	case SortName:
		sortIndicator += "NAME"
	}
	if m.sortDesc {
		sortIndicator += " ▼]"
	} else {
		sortIndicator += " ▲]"
	}

	procHeader := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.CardTitleStyle.Render("📊 PROCESS LIST"),
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf(" (%d processes)%s", len(m.processes), sortIndicator)),
	)

	statusLine := ""
	if m.killConfirm {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("⚠️ Kill PID %d? (y/N)", m.selectedPID))
	} else if m.killStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.killStatus)
	}

	procBody := lipgloss.JoinVertical(lipgloss.Left,
		procHeader,
		statusLine,
		m.table.View(),
	)

	procSection := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(procBody)

	return lipgloss.JoinVertical(lipgloss.Left,
		cpuSection,
		memSection,
		procSection,
	)
}
