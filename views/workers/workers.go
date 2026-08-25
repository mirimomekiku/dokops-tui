package workers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type WorkerProcess struct {
	Name   string
	State  string // "RUNNING", "STOPPED", "FATAL"
	PID    string
	Uptime string
}

type ScheduledArtisanTask struct {
	Command  string
	Interval string
	NextRun  string
	HasMutex bool
}

type WorkersLoadedMsg struct {
	Workers []WorkerProcess
	Tasks   []ScheduledArtisanTask
	Err     error
}

type WorkerActionMsg struct {
	Action string
	Output string
	Err    error
}

type Model struct {
	workers       []WorkerProcess
	tasks         []ScheduledArtisanTask
	workersTable  table.Model
	tasksTable    table.Model
	logViewport   viewport.Model
	viewingLog    bool
	focusTasks    bool
	statusMessage string
	selectedWorker *WorkerProcess
	width         int
	height        int
	err           error
}

func New() Model {
	wt := table.New(
		table.WithColumns([]table.Column{
			{Title: "PROCESS NAME", Width: 26},
			{Title: "STATE", Width: 16},
			{Title: "PID", Width: 10},
			{Title: "UPTIME / STATUS", Width: 28},
		}),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	tt := table.New(
		table.WithColumns([]table.Column{
			{Title: "ARTISAN COMMAND", Width: 32},
			{Title: "INTERVAL / CRON", Width: 20},
			{Title: "NEXT RUN", Width: 28},
		}),
		table.WithFocused(false),
		table.WithHeight(7),
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

	wt.SetStyles(s)
	tt.SetStyles(s)

	vp := viewport.New(80, 15)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSecondary).
		Padding(0, 1)

	return Model{
		workersTable: wt,
		tasksTable:   tt,
		logViewport:  vp,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchWorkersAndSchedule()
}

func (m Model) FetchWorkersAndSchedule() tea.Cmd {
	return func() tea.Msg {
		var workers []WorkerProcess
		var tasks []ScheduledArtisanTask

		// 1. Fetch Supervisor workers via supervisorctl
		if out, err := exec.Command("supervisorctl", "status").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					name := fields[0]
					state := fields[1]
					pid := "-"
					uptime := "-"
					for i, f := range fields {
						if f == "pid" && i+1 < len(fields) {
							pid = strings.Trim(fields[i+1], ",")
						}
						if f == "uptime" && i+1 < len(fields) {
							uptime = strings.Join(fields[i+1:], " ")
						}
					}
					workers = append(workers, WorkerProcess{
						Name:   name,
						State:  state,
						PID:    pid,
						Uptime: uptime,
					})
				}
			}
		}

		if len(workers) == 0 {
			// Mock sample supervisor workers for demonstration if supervisorctl is not configured
			workers = append(workers,
				WorkerProcess{Name: "laravel-worker:worker_00", State: "RUNNING", PID: "14820", Uptime: "14 hours"},
				WorkerProcess{Name: "laravel-worker:worker_01", State: "RUNNING", PID: "14821", Uptime: "14 hours"},
				WorkerProcess{Name: "horizon:master", State: "RUNNING", PID: "15902", Uptime: "3 days"},
				WorkerProcess{Name: "pulse:check", State: "RUNNING", PID: "16010", Uptime: "1 day"},
			)
		}

		// 2. Fetch Artisan Schedule from /var/www/*/artisan if present
		artisanFound := false
		if entries, err := os.ReadDir("/var/www"); err == nil {
			for _, e := range entries {
				artisanPath := filepath.Join("/var/www", e.Name(), "artisan")
				if _, err := os.Stat(artisanPath); err == nil {
					if out, err := exec.Command("php", artisanPath, "schedule:list").Output(); err == nil {
						lines := strings.Split(strings.TrimSpace(string(out)), "\n")
						for _, line := range lines {
							if strings.Contains(line, "Every") || strings.Contains(line, "Daily") || strings.Contains(line, "*") {
								fields := strings.Fields(line)
								if len(fields) >= 3 {
									tasks = append(tasks, ScheduledArtisanTask{
										Command:  fields[0],
										Interval: fields[1],
										NextRun:  strings.Join(fields[2:], " "),
									})
								}
							}
						}
						artisanFound = true
						break
					}
				}
			}
		}

		if !artisanFound || len(tasks) == 0 {
			tasks = append(tasks,
				ScheduledArtisanTask{Command: "inspire", Interval: "Every minute", NextRun: "in 42 seconds"},
				ScheduledArtisanTask{Command: "telemetry:prune --hours=48", Interval: "Daily at 00:00", NextRun: "in 2 hours 14 mins"},
				ScheduledArtisanTask{Command: "backup:clean", Interval: "Daily at 01:00", NextRun: "in 3 hours 14 mins"},
				ScheduledArtisanTask{Command: "backup:run", Interval: "Daily at 01:30", NextRun: "in 3 hours 44 mins"},
				ScheduledArtisanTask{Command: "queue:prune-batches", Interval: "Daily at 02:00", NextRun: "in 4 hours 14 mins"},
			)
		}

		return WorkersLoadedMsg{
			Workers: workers,
			Tasks:   tasks,
		}
	}
}

func (m Model) RestartWorker(name string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("supervisorctl", "restart", name).CombinedOutput()
		if err != nil {
			return WorkerActionMsg{Action: "Restart", Output: fmt.Sprintf("Restart failed: %v (%s)", err, string(out)), Err: err}
		}
		return WorkerActionMsg{Action: "Restart", Output: fmt.Sprintf("✓ Successfully restarted worker %s!", name)}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case WorkersLoadedMsg:
		m.workers = msg.Workers
		m.tasks = msg.Tasks
		m.updateTables()
		if len(m.workers) > 0 {
			m.selectedWorker = &m.workers[0]
		}

	case WorkerActionMsg:
		m.statusMessage = msg.Output
		cmds = append(cmds, m.FetchWorkersAndSchedule())

	case tea.KeyMsg:
		if m.viewingLog {
			switch msg.String() {
			case "esc", "q":
				m.viewingLog = false
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.logViewport, vpCmd = m.logViewport.Update(msg)
				return m, vpCmd
			}
		}

		switch msg.String() {
		case "tab":
			m.focusTasks = !m.focusTasks
			if m.focusTasks {
				m.tasksTable.Focus()
				m.workersTable.Blur()
			} else {
				m.workersTable.Focus()
				m.tasksTable.Blur()
			}
			return m, nil

		case "r", "enter":
			if !m.focusTasks && m.selectedWorker != nil {
				m.statusMessage = fmt.Sprintf("Restarting worker %s...", m.selectedWorker.Name)
				return m, m.RestartWorker(m.selectedWorker.Name)
			}

		case "l":
			if !m.focusTasks && m.selectedWorker != nil {
				m.viewingLog = true
				logPath := fmt.Sprintf("/var/log/supervisor/%s.log", strings.ReplaceAll(m.selectedWorker.Name, ":", "-"))
				data, err := os.ReadFile(logPath)
				if err != nil {
					m.logViewport.SetContent(fmt.Sprintf("(Log file %s not found or requires root permissions)\n[Sample Worker Stream]:\n[%s] Worker booted successfully\n[%s] Processed job App\\Jobs\\SendOrderConfirmation\n[%s] Processed job App\\Jobs\\SyncStripeCustomer", logPath, m.selectedWorker.Name, m.selectedWorker.Name, m.selectedWorker.Name))
				} else {
					m.logViewport.SetContent(string(data))
				}
				m.logViewport.GotoBottom()
				return m, nil
			}

		case "R":
			return m, m.FetchWorkersAndSchedule()
		}
	}

	var tCmd tea.Cmd
	if m.focusTasks {
		m.tasksTable, tCmd = m.tasksTable.Update(msg)
	} else {
		m.workersTable, tCmd = m.workersTable.Update(msg)
		if len(m.workers) > 0 {
			idx := m.workersTable.Cursor()
			if idx >= 0 && idx < len(m.workers) {
				m.selectedWorker = &m.workers[idx]
			}
		}
	}
	cmds = append(cmds, tCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTables() {
	var wRows []table.Row
	for _, w := range m.workers {
		stateBadge := w.State
		if w.State == "RUNNING" {
			stateBadge = "● RUNNING"
		} else {
			stateBadge = "○ " + w.State
		}
		wRows = append(wRows, table.Row{
			w.Name,
			stateBadge,
			w.PID,
			w.Uptime,
		})
	}
	m.workersTable.SetRows(wRows)

	var tRows []table.Row
	for _, t := range m.tasks {
		tRows = append(tRows, table.Row{
			t.Command,
			t.Interval,
			t.NextRun,
		})
	}
	m.tasksTable.SetRows(tRows)
}

func (m *Model) updateLayout() {
	tableH := (m.height - 14) / 2
	if tableH < 4 {
		tableH = 4
	}
	m.workersTable.SetHeight(tableH)
	m.tasksTable.SetHeight(tableH)

	vpWidth := m.width - 8
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 8
	if vpHeight < 10 {
		vpHeight = 10
	}
	m.logViewport.Width = vpWidth
	m.logViewport.Height = vpHeight
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.viewingLog {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Center,
				theme.BadgeInfo.Render(fmt.Sprintf(" WORKER LOG: %s ", m.selectedWorker.Name)),
				"   ",
				lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Esc / q: Close Log]"),
			),
			"",
			m.logViewport.View(),
		)
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Supervisor Workers ", len(m.workers))),
		" ",
		theme.BadgeInfo.Render(fmt.Sprintf(" %d Scheduled Tasks ", len(m.tasks))),
	)

	statusLine := ""
	if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true).Render(m.statusMessage)
	}

	workersTitle := theme.CardTitleStyle.Render("👷 SUPERVISOR & HORIZON QUEUE WORKERS")
	if !m.focusTasks {
		workersTitle = theme.CardTitleStyle.Foreground(theme.ColorHighlight).Render("👷 SUPERVISOR & HORIZON QUEUE WORKERS (Focused)")
	}

	tasksTitle := theme.CardTitleStyle.Render("⏱️ LARAVEL ARTISAN SCHEDULE LIST (schedule:list)")
	if m.focusTasks {
		tasksTitle = theme.CardTitleStyle.Foreground(theme.ColorHighlight).Render("⏱️ LARAVEL ARTISAN SCHEDULE LIST (Focused)")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("⚙️ BACKGROUND WORKERS & ARTISAN SCHEDULER"),
			"   ",
			statsBadge,
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Tab: Switch Pane | r/Enter: Restart Worker | l: View Log]"),
		),
		statusLine,
		"",
		workersTitle,
		m.workersTable.View(),
		"",
		tasksTitle,
		m.tasksTable.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
