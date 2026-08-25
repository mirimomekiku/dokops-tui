package bandwidth

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	psnet "github.com/shirou/gopsutil/v3/net"

	"dok-ops/internal/theme"
)

type InterfaceStat struct {
	Name      string
	RxRate    float64 // Bytes per sec
	TxRate    float64 // Bytes per sec
	BytesRecv uint64
	BytesSent uint64
	DropIn    uint64
	DropOut   uint64
	ErrIn     uint64
	ErrOut    uint64
}

type NetBandwidthMsg struct {
	Stats []InterfaceStat
	Err   error
}

type TickMsg time.Time

type Model struct {
	lastIOCounters map[string]psnet.IOCountersStat
	lastSampleTime time.Time
	stats          []InterfaceStat
	table          table.Model
	width          int
	height         int
	err            error
}

func New() Model {
	cols := []table.Column{
		{Title: "INTERFACE", Width: 14},
		{Title: "INGRESS (Rx/s)", Width: 18},
		{Title: "EGRESS (Tx/s)", Width: 18},
		{Title: "TOTAL RX", Width: 14},
		{Title: "TOTAL TX", Width: 14},
		{Title: "ERR / DROP", Width: 14},
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
		lastIOCounters: make(map[string]psnet.IOCountersStat),
		table:          t,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.SampleBandwidth(),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) SampleBandwidth() tea.Cmd {
	return func() tea.Msg {
		counters, err := psnet.IOCounters(true)
		if err != nil {
			return NetBandwidthMsg{Err: err}
		}

		now := time.Now()
		var stats []InterfaceStat

		for _, c := range counters {
			var rxRate, txRate float64
			if prev, exists := m.lastIOCounters[c.Name]; exists && !m.lastSampleTime.IsZero() {
				sec := now.Sub(m.lastSampleTime).Seconds()
				if sec > 0 {
					if c.BytesRecv >= prev.BytesRecv {
						rxRate = float64(c.BytesRecv-prev.BytesRecv) / sec
					}
					if c.BytesSent >= prev.BytesSent {
						txRate = float64(c.BytesSent-prev.BytesSent) / sec
					}
				}
			}

			stats = append(stats, InterfaceStat{
				Name:      c.Name,
				RxRate:    rxRate,
				TxRate:    txRate,
				BytesRecv: c.BytesRecv,
				BytesSent: c.BytesSent,
				DropIn:    c.Dropin,
				DropOut:   c.Dropout,
				ErrIn:     c.Errin,
				ErrOut:    c.Errout,
			})
		}

		return NetBandwidthMsg{Stats: stats}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case TickMsg:
		cmds = append(cmds, m.SampleBandwidth(), tickCmd())

	case NetBandwidthMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.stats = msg.Stats

			// Update cache
			m.lastSampleTime = time.Now()
			for _, s := range msg.Stats {
				m.lastIOCounters[s.Name] = psnet.IOCountersStat{
					Name:      s.Name,
					BytesRecv: s.BytesRecv,
					BytesSent: s.BytesSent,
				}
			}

			// Sort by RxRate descending, non-loopback first
			sort.Slice(m.stats, func(i, j int) bool {
				if m.stats[i].Name == "lo" {
					return false
				}
				if m.stats[j].Name == "lo" {
					return true
				}
				return (m.stats[i].RxRate + m.stats[i].TxRate) > (m.stats[j].RxRate + m.stats[j].TxRate)
			})

			m.updateTableRows()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			cmds = append(cmds, m.SampleBandwidth())
		}
	}

	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	if tableCmd != nil {
		cmds = append(cmds, tableCmd)
	}

	return m, tea.Batch(cmds...)
}

func formatRate(bps float64) string {
	if bps < 1024 {
		return fmt.Sprintf("%6.1f B/s", bps)
	}
	if bps < 1024*1024 {
		return fmt.Sprintf("%6.1f KB/s", bps/1024.0)
	}
	return fmt.Sprintf("%6.1f MB/s", bps/(1024.0*1024.0))
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, s := range m.stats {
		errDropStr := fmt.Sprintf("%d/%d", s.ErrIn+s.ErrOut, s.DropIn+s.DropOut)
		rows = append(rows, table.Row{
			s.Name,
			formatRate(s.RxRate),
			formatRate(s.TxRate),
			theme.FormatBytes(s.BytesRecv),
			theme.FormatBytes(s.BytesSent),
			errDropStr,
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

	availableWidth := m.width - 6
	if availableWidth > 80 {
		cols := []table.Column{
			{Title: "INTERFACE", Width: 16},
			{Title: "INGRESS (Rx/s)", Width: 20},
			{Title: "EGRESS (Tx/s)", Width: 20},
			{Title: "TOTAL RX", Width: 16},
			{Title: "TOTAL TX", Width: 16},
			{Title: "ERR / DROP", Width: 14},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var totalRxRate, totalTxRate float64
	for _, s := range m.stats {
		if s.Name != "lo" {
			totalRxRate += s.RxRate
			totalTxRate += s.TxRate
		}
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" IN: %s ", formatRate(totalRxRate))),
		" ",
		theme.BadgeInfo.Render(fmt.Sprintf(" OUT: %s ", formatRate(totalTxRate))),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(Tracking %d interfaces)", len(m.stats))),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("📶 LIVE BANDWIDTH & INTERFACE MONITOR (iftop)"),
			"  ",
			statsBadge,
		),
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
