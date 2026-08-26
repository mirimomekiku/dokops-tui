package scanner

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type PortResult struct {
	Port        int
	ServiceName string
	IsOpen      bool
	Latency     time.Duration
	Banner      string
}

type ScanFinishedMsg struct {
	Target   string
	Results  []PortResult
	Duration time.Duration
}

type PresetOption struct {
	Name  string
	Ports []int
}

var CommonPresets = []PresetOption{
	{
		Name:  "Web & APIs",
		Ports: []int{80, 443, 3000, 5000, 8000, 8080, 8443, 8888, 9000},
	},
	{
		Name:  "Databases",
		Ports: []int{3306, 5432, 6379, 27017, 9200, 1433, 1521, 5984},
	},
	{
		Name:  "DevOps & Infra",
		Ports: []int{22, 2375, 2376, 2379, 6443, 9090, 9100, 9092, 10250},
	},
	{
		Name:  "Top 20 Essential",
		Ports: []int{21, 22, 25, 53, 80, 110, 143, 443, 465, 587, 993, 995, 1433, 3306, 3389, 5432, 6379, 8080, 8443, 27017},
	},
}

type Model struct {
	targetInput textinput.Model
	presetIdx   int
	results     []PortResult
	table       table.Model
	actionMenu  actionmenu.Model
	isScanning  bool
	lastTarget  string
	scanTime    time.Duration
	width       int
	height      int
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "127.0.0.1 or domain.com"
	ti.SetValue("127.0.0.1")
	ti.Focus()
	ti.CharLimit = 255
	ti.Width = 35

	cols := []table.Column{
		{Title: "PORT", Width: 10},
		{Title: "SERVICE", Width: 18},
		{Title: "STATUS", Width: 12},
		{Title: "RTT / LATENCY", Width: 16},
		{Title: "GRABBED BANNER / PROTOCOL", Width: 40},
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
		targetInput: ti,
		presetIdx:   0,
		table:       t,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.StartScan(),
	)
}

func (m Model) StartScan() tea.Cmd {
	target := strings.TrimSpace(m.targetInput.Value())
	if target == "" {
		target = "127.0.0.1"
	}
	ports := CommonPresets[m.presetIdx].Ports

	return func() tea.Msg {
		start := time.Now()
		var results []PortResult
		var mu sync.Mutex
		var wg sync.WaitGroup

		sem := make(chan struct{}, 30) // concurrency limit

		for _, p := range ports {
			wg.Add(1)
			go func(port int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				addr := net.JoinHostPort(target, strconv.Itoa(port))
				connStart := time.Now()
				conn, err := net.DialTimeout("tcp", addr, 1200*time.Millisecond)
				rtt := time.Since(connStart)

				serviceName := lookupPortService(port)
				if err != nil {
					mu.Lock()
					results = append(results, PortResult{
						Port:        port,
						ServiceName: serviceName,
						IsOpen:      false,
						Latency:     rtt,
					})
					mu.Unlock()
					return
				}
				defer conn.Close()

				// Banner grabbing
				_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
				_, _ = conn.Write([]byte("\r\n"))
				buf := make([]byte, 256)
				n, _ := conn.Read(buf)
				banner := strings.TrimSpace(string(buf[:n]))
				if banner == "" {
					banner = "(Open / Connected)"
				}

				mu.Lock()
				results = append(results, PortResult{
					Port:        port,
					ServiceName: serviceName,
					IsOpen:      true,
					Latency:     rtt,
					Banner:      banner,
				})
				mu.Unlock()
			}(p)
		}

		wg.Wait()

		// Sort open ports first, then port number
		sort.Slice(results, func(i, j int) bool {
			if results[i].IsOpen != results[j].IsOpen {
				return results[i].IsOpen
			}
			return results[i].Port < results[j].Port
		})

		return ScanFinishedMsg{
			Target:   target,
			Results:  results,
			Duration: time.Since(start),
		}
	}
}

func lookupPortService(port int) string {
	switch port {
	case 21:
		return "FTP"
	case 22:
		return "SSH"
	case 25:
		return "SMTP"
	case 53:
		return "DNS"
	case 80:
		return "HTTP"
	case 443:
		return "HTTPS"
	case 1433:
		return "MSSQL"
	case 2375, 2376:
		return "Docker API"
	case 3000:
		return "Grafana/Node"
	case 3306:
		return "MySQL/MariaDB"
	case 5432:
		return "PostgreSQL"
	case 6379:
		return "Redis"
	case 6443:
		return "Kubernetes API"
	case 8080:
		return "HTTP Alt"
	case 8443:
		return "HTTPS Alt"
	case 9090:
		return "Prometheus"
	case 9100:
		return "Node Exporter"
	case 9200:
		return "Elasticsearch"
	case 27017:
		return "MongoDB"
	default:
		return fmt.Sprintf("Port %d", port)
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case ScanFinishedMsg:
		m.isScanning = false
		m.lastTarget = msg.Target
		m.scanTime = msg.Duration
		m.results = msg.Results
		m.updateTableRows()

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				switch action {
				case "scan":
					m.isScanning = true
					return m, m.StartScan()
				case "cycle_preset":
					m.presetIdx = (m.presetIdx + 1) % len(CommonPresets)
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "h", "left":
			if m.presetIdx > 0 {
				m.presetIdx--
			}
			return m, nil
		case "l", "right":
			if m.presetIdx < len(CommonPresets)-1 {
				m.presetIdx++
			}
			return m, nil
		case "space":
			title := "Port Scanner"
			subtitle := fmt.Sprintf("Target: %s | Preset: %s", m.targetInput.Value(), CommonPresets[m.presetIdx].Name)
			items := []actionmenu.Item{
				{Key: "scan", Title: "Start Port Scan", Description: "Probe ports concurrently with banner grab"},
				{Key: "cycle_preset", Title: "Cycle Preset", Description: fmt.Sprintf("Current: %s", CommonPresets[m.presetIdx].Name)},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil
		case "enter":
			m.isScanning = true
			return m, m.StartScan()
		}

		var cmd tea.Cmd
		m.targetInput, cmd = m.targetInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	if tableCmd != nil {
		cmds = append(cmds, tableCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, r := range m.results {
		status := "CLOSED"
		if r.IsOpen {
			status = "OPEN"
		}
		banner := r.Banner
		if !r.IsOpen {
			banner = "-"
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", r.Port),
			r.ServiceName,
			status,
			theme.FormatDuration(r.Latency),
			banner,
		})
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	inputWidth := m.width - 45
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.targetInput.Width = inputWidth

	contentHeight := m.height - 8
	if contentHeight < 6 {
		contentHeight = 6
	}
	m.table.SetHeight(contentHeight)

	availableWidth := m.width - 6
	if availableWidth > 80 {
		bannerWidth := availableWidth - (10 + 18 + 12 + 16 + 10)
		if bannerWidth < 20 {
			bannerWidth = 20
		}
		cols := []table.Column{
			{Title: "PORT", Width: 10},
			{Title: "SERVICE", Width: 18},
			{Title: "STATUS", Width: 12},
			{Title: "RTT / LATENCY", Width: 16},
			{Title: "GRABBED BANNER / PROTOCOL", Width: bannerWidth},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Preset pills
	var pills []string
	for i, p := range CommonPresets {
		if i == m.presetIdx {
			pills = append(pills, theme.ActiveTabStyle.Render(p.Name))
		} else {
			pills = append(pills, theme.InactiveTabStyle.Render(p.Name))
		}
	}
	presetRow := lipgloss.JoinHorizontal(lipgloss.Left, pills...)

	targetRow := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Host  "),
		m.targetInput.View(),
		"   ",
		presetRow,
	)

	var openCount int
	for _, r := range m.results {
		if r.IsOpen {
			openCount++
		}
	}

	var statusHeader string
	if m.isScanning {
		statusHeader = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("Scanning ports concurrently...")
	} else {
		statusHeader = lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Port Scan"),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("● %d open", openCount)),
			"  ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("○ %d closed/filtered", len(m.results)-openCount)),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(%d ports in %s)", len(m.results), theme.FormatDuration(m.scanTime))),
		)
	}

	elements := []string{
		targetRow,
		"",
		statusHeader,
		"",
		m.table.View(),
	}

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, elements...))

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
