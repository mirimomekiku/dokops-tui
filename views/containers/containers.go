package containers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type ContainersLoadedMsg []types.Container
type LogsStreamedMsg struct {
	ID   string
	Logs string
}
type ActionCompleteMsg struct {
	ID     string
	Action string
	Err    error
}
type DockerErrorMsg struct {
	Err error
}

type Model struct {
	cli            *client.Client
	containers     []types.Container
	table          table.Model
	logsViewport   viewport.Model
	actionMenu     actionmenu.Model
	viewingLogs    bool
	activeLogID    string
	activeLogName  string
	actionStatus   string
	err            error
	dockerOffline  bool
	width          int
	height         int
	loading        bool
	confirmDelete  bool
	deleteTargetID string
}

func New() Model {
	columns := []table.Column{
		{Title: "ID", Width: 12},
		{Title: "NAMES", Width: 20},
		{Title: "IMAGE", Width: 25},
		{Title: "STATE", Width: 10},
		{Title: "STATUS", Width: 22},
		{Title: "PORTS", Width: 20},
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

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	return Model{
		cli:          cli,
		table:        t,
		logsViewport: vp,
		err:          err,
		loading:      true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchContainers()
}

func (m Model) FetchContainers() tea.Cmd {
	return func() tea.Msg {
		if m.cli == nil {
			var err error
			m.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				return DockerErrorMsg{Err: err}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return DockerErrorMsg{Err: err}
		}

		return ContainersLoadedMsg(containers)
	}
}

func (m Model) FetchLogs(containerID string) tea.Cmd {
	return func() tea.Msg {
		if m.cli == nil {
			return LogsStreamedMsg{ID: containerID, Logs: "Docker client not initialized"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		options := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "200",
			Timestamps: true,
		}

		out, err := m.cli.ContainerLogs(ctx, containerID, options)
		if err != nil {
			return LogsStreamedMsg{ID: containerID, Logs: fmt.Sprintf("Error fetching logs: %v", err)}
		}
		defer out.Close()

		buf, err := io.ReadAll(out)
		if err != nil {
			return LogsStreamedMsg{ID: containerID, Logs: fmt.Sprintf("Error reading logs: %v", err)}
		}

		// Docker logs prepend 8-byte header to each line if multiplexed, sanitize if needed
		cleanLog := cleanDockerLogHeader(buf)
		if strings.TrimSpace(cleanLog) == "" {
			cleanLog = "(No logs available for container)"
		}

		return LogsStreamedMsg{ID: containerID, Logs: cleanLog}
	}
}

func cleanDockerLogHeader(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var sb strings.Builder
	for len(data) > 0 {
		if len(data) >= 8 && (data[0] <= 2) {
			size := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
			data = data[8:]
			if size <= len(data) {
				sb.Write(data[:size])
				data = data[size:]
			} else {
				sb.Write(data)
				break
			}
		} else {
			sb.Write(data)
			break
		}
	}
	return sb.String()
}

func (m Model) PerformAction(containerID, action string) tea.Cmd {
	return func() tea.Msg {
		if m.cli == nil {
			return ActionCompleteMsg{ID: containerID, Action: action, Err: fmt.Errorf("docker offline")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		switch action {
		case "start":
			err = m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
		case "stop":
			stopTimeout := 5
			err = m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})
		case "restart":
			stopTimeout := 5
			err = m.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})
		case "remove":
			err = m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		}

		return ActionCompleteMsg{ID: containerID, Action: action, Err: err}
	}
}

func (m Model) getSelectedContainer() *types.Container {
	if len(m.containers) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.containers) {
		return &m.containers[idx]
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

	case DockerErrorMsg:
		m.dockerOffline = true
		m.err = msg.Err
		m.loading = false
		m.actionStatus = fmt.Sprintf("Docker error: %v", msg.Err)

	case ContainersLoadedMsg:
		m.dockerOffline = false
		m.err = nil
		m.loading = false
		m.containers = msg
		m.updateTableRows()

	case LogsStreamedMsg:
		m.setLogContent(msg.Logs)
		m.logsViewport.GotoBottom()

	case ActionCompleteMsg:
		if msg.Err != nil {
			m.actionStatus = fmt.Sprintf("Failed to %s (%s): %v", msg.Action, msg.ID[:min(12, len(msg.ID))], msg.Err)
		} else {
			m.actionStatus = fmt.Sprintf("Successfully performed '%s' on %s", msg.Action, msg.ID[:min(12, len(msg.ID))])
			cmds = append(cmds, m.FetchContainers())
		}

	case tea.KeyMsg:
		// Handle Action Menu if active
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				sel := m.getSelectedContainer()
				if sel != nil {
					switch action {
					case "start":
						m.actionStatus = fmt.Sprintf("Starting container %s...", sel.ID[:12])
						cmds = append(cmds, m.PerformAction(sel.ID, "start"))
					case "stop":
						m.actionStatus = fmt.Sprintf("Stopping container %s...", sel.ID[:12])
						cmds = append(cmds, m.PerformAction(sel.ID, "stop"))
					case "restart":
						m.actionStatus = fmt.Sprintf("Restarting container %s...", sel.ID[:12])
						cmds = append(cmds, m.PerformAction(sel.ID, "restart"))
					case "logs":
						m.viewingLogs = true
						m.activeLogID = sel.ID
						name := sel.ID[:12]
						if len(sel.Names) > 0 {
							name = strings.TrimPrefix(sel.Names[0], "/")
						}
						m.activeLogName = name
						m.logsViewport.SetContent("Loading logs...")
						cmds = append(cmds, m.FetchLogs(sel.ID))
					case "remove":
						m.deleteTargetID = sel.ID
						m.confirmDelete = true
					case "refresh":
						m.loading = true
						cmds = append(cmds, m.FetchContainers())
					}
				} else if action == "refresh" {
					m.loading = true
					cmds = append(cmds, m.FetchContainers())
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

		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y", "enter":
				cmds = append(cmds, m.PerformAction(m.deleteTargetID, "remove"))
				m.confirmDelete = false
			case "n", "N", "esc", "space":
				m.confirmDelete = false
				m.actionStatus = "Remove cancelled"
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "space":
			sel := m.getSelectedContainer()
			title := "Docker Containers"
			subtitle := "Select an action to execute"
			if sel != nil {
				name := sel.ID[:12]
				if len(sel.Names) > 0 {
					name = strings.TrimPrefix(sel.Names[0], "/")
				}
				title = "Actions: " + name
				subtitle = fmt.Sprintf("Status: %s (%s)", sel.State, sel.Status)
			}
			items := []actionmenu.Item{
				{Key: "logs", Title: "Stream Logs", Description: "View real-time stdout/stderr"},
				{Key: "start", Title: "Start Container", Description: "Start stopped container instance"},
				{Key: "stop", Title: "Stop Container", Description: "Gracefully stop container"},
				{Key: "restart", Title: "Restart Container", Description: "Restart container instance"},
				{Key: "remove", Title: "Delete Container", Description: "Force remove container", Danger: true},
				{Key: "refresh", Title: "Refresh List", Description: "Fetch container states"},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil

		case "enter":
			// Primary action: Stream logs
			sel := m.getSelectedContainer()
			if sel != nil {
				m.viewingLogs = true
				m.activeLogID = sel.ID
				name := sel.ID[:12]
				if len(sel.Names) > 0 {
					name = strings.TrimPrefix(sel.Names[0], "/")
				}
				m.activeLogName = name
				m.logsViewport.SetContent("Loading logs...")
				cmds = append(cmds, m.FetchLogs(sel.ID))
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, c := range m.containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		idShort := c.ID
		if len(idShort) > 12 {
			idShort = idShort[:12]
		}
		var ports []string
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d->%d", p.PublicPort, p.PrivatePort))
			}
		}
		portStr := strings.Join(ports, ", ")
		if portStr == "" {
			portStr = "-"
		}

		rows = append(rows, table.Row{
			idShort,
			name,
			c.Image,
			c.State,
			c.Status,
			portStr,
		})
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	contentHeight := m.height - 6
	if contentHeight < 6 {
		contentHeight = 6
	}
	m.table.SetHeight(contentHeight)

	availableWidth := m.width - 6
	if availableWidth > 80 {
		imageWidth := availableWidth - (12 + 20 + 10 + 22 + 20 + 8)
		if imageWidth < 20 {
			imageWidth = 20
		}
		cols := []table.Column{
			{Title: "ID", Width: 12},
			{Title: "NAMES", Width: 20},
			{Title: "IMAGE", Width: imageWidth},
			{Title: "STATE", Width: 10},
			{Title: "STATUS", Width: 22},
			{Title: "PORTS", Width: 20},
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

	if m.dockerOffline {
		errorCard := lipgloss.NewStyle().
			Foreground(theme.ColorDanger).
			Padding(1, 2).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					lipgloss.NewStyle().Bold(true).Foreground(theme.ColorDanger).Render("Docker daemon offline"),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorText).Render("Cannot connect to Docker daemon socket (/var/run/docker.sock)."),
					lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("Details: %v", m.err)),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("Tip: Ensure Docker is running (`sudo systemctl start docker`)"),
				),
			)
		return errorCard
	}

	if m.viewingLogs {
		logsHeader := lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("Logs: "+m.activeLogName),
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf(" (%s)", m.activeLogID[:min(12, len(m.activeLogID))])),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Esc / q: Close log viewer"),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			logsHeader,
			"",
			m.logsViewport.View(),
		)
	}

	// Status Header
	var running, stopped, paused int
	for _, c := range m.containers {
		switch c.State {
		case "running":
			running++
		case "paused":
			paused++
		default:
			stopped++
		}
	}

	statusHeader := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Containers"),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("running: %d", running)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorWarning).Render(fmt.Sprintf("paused: %d", paused)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("stopped: %d", stopped)),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(total: %d)", len(m.containers))),
	)

	statusLine := ""
	if m.confirmDelete {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("Force remove container %s? (y/N)", m.deleteTargetID[:min(12, len(m.deleteTargetID))]))
	} else if m.actionStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("  " + m.actionStatus)
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
