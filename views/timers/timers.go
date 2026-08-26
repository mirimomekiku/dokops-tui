package timers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/robfig/cron/v3"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type ScheduledTask struct {
	SourceType string // "CRON", "SYSTEMD"
	Origin     string // "user (crontab)", "/etc/crontab", "/etc/cron.d/certbot", "apt-daily.timer"
	Schedule   string
	NextRun    time.Time
	NextRunStr string
	Countdown  string
	Target     string // Command or triggered service
	User       string
}

type TimersLoadedMsg struct {
	Tasks []ScheduledTask
	Err   error
}

type FilterMode int

const (
	FilterAll FilterMode = iota
	FilterCronOnly
	FilterSystemdOnly
)

type Model struct {
	tasks         []ScheduledTask
	filteredTasks []ScheduledTask
	table         table.Model
	actionMenu    actionmenu.Model
	filter        FilterMode
	isLoading     bool
	width         int
	height        int
	err           error
}

func New() Model {
	cols := []table.Column{
		{Title: "TYPE", Width: 10},
		{Title: "SOURCE / ORIGIN", Width: 22},
		{Title: "SCHEDULE", Width: 18},
		{Title: "NEXT RUN", Width: 20},
		{Title: "COUNTDOWN", Width: 16},
		{Title: "TRIGGERED COMMAND / SERVICE", Width: 35},
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
		table:     t,
		filter:    FilterAll,
		isLoading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchScheduledTasks()
}

func (m Model) FetchScheduledTasks() tea.Cmd {
	return func() tea.Msg {
		var tasks []ScheduledTask
		cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		now := time.Now()

		// 1. User Crontab
		if out, err := exec.Command("crontab", "-l").Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				fields := strings.Fields(trimmed)
				if len(fields) >= 6 {
					schedStr := strings.Join(fields[:5], " ")
					cmdStr := strings.Join(fields[5:], " ")
					nextRunStr := "N/A"
					countdownStr := "-"
					var nextTime time.Time
					if sched, err := cronParser.Parse(schedStr); err == nil {
						nextTime = sched.Next(now)
						nextRunStr = nextTime.Format("02 Jan 15:04:05")
						countdownStr = "in " + time.Until(nextTime).Round(time.Second).String()
					}
					tasks = append(tasks, ScheduledTask{
						SourceType: "CRON",
						Origin:     "user (crontab)",
						Schedule:   schedStr,
						NextRun:    nextTime,
						NextRunStr: nextRunStr,
						Countdown:  countdownStr,
						Target:     cmdStr,
						User:       os.Getenv("USER"),
					})
				}
			}
		}

		// 2. /etc/crontab and /etc/cron.d/*
		cronFiles := []string{"/etc/crontab"}
		if entries, err := os.ReadDir("/etc/cron.d"); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					cronFiles = append(cronFiles, filepath.Join("/etc/cron.d", e.Name()))
				}
			}
		}

		for _, cf := range cronFiles {
			if data, err := os.ReadFile(cf); err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "=") {
						continue
					}
					fields := strings.Fields(trimmed)
					if len(fields) >= 7 {
						schedStr := strings.Join(fields[:5], " ")
						userStr := fields[5]
						cmdStr := strings.Join(fields[6:], " ")
						nextRunStr := "N/A"
						countdownStr := "-"
						var nextTime time.Time
						if sched, err := cronParser.Parse(schedStr); err == nil {
							nextTime = sched.Next(now)
							nextRunStr = nextTime.Format("02 Jan 15:04:05")
							countdownStr = "in " + time.Until(nextTime).Round(time.Second).String()
						}
						tasks = append(tasks, ScheduledTask{
							SourceType: "CRON",
							Origin:     filepath.Base(cf),
							Schedule:   schedStr,
							NextRun:    nextTime,
							NextRunStr: nextRunStr,
							Countdown:  countdownStr,
							Target:     cmdStr,
							User:       userStr,
						})
					}
				}
			}
		}

		// 3. Systemd Timers via 'systemctl list-timers'
		if out, err := exec.Command("systemctl", "list-timers", "--all", "--no-pager", "--plain", "--no-legend").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 6 {
					// Typical format: [NEXT DATE] [NEXT TIME] [LEFT] [LAST DATE] [LAST TIME] [PASSED] [UNIT] [ACTIVATES]
					unitIdx := -1
					activatesIdx := -1
					for i, f := range fields {
						if strings.HasSuffix(f, ".timer") {
							unitIdx = i
						} else if strings.HasSuffix(f, ".service") || strings.HasSuffix(f, ".target") {
							activatesIdx = i
						}
					}

					if unitIdx > 0 {
						unitName := fields[unitIdx]
						activates := "-"
						if activatesIdx > unitIdx {
							activates = fields[activatesIdx]
						}
						nextStr := fields[0] + " " + fields[1]
						countdown := "-"
						if unitIdx > 2 {
							countdown = strings.Join(fields[2:unitIdx], " ")
						}

						tasks = append(tasks, ScheduledTask{
							SourceType: "SYSTEMD",
							Origin:     unitName,
							Schedule:   "systemd timer",
							NextRunStr: nextStr,
							Countdown:  countdown,
							Target:     activates,
							User:       "root",
						})
					}
				}
			}
		}

		// Sort by NextRun time ascending
		sort.Slice(tasks, func(i, j int) bool {
			if tasks[i].SourceType != tasks[j].SourceType {
				return tasks[i].SourceType == "CRON" // Crons first
			}
			return tasks[i].Origin < tasks[j].Origin
		})

		return TimersLoadedMsg{Tasks: tasks}
	}
}

func (m *Model) applyFilter() {
	m.filteredTasks = nil
	for _, t := range m.tasks {
		switch m.filter {
		case FilterCronOnly:
			if t.SourceType == "CRON" {
				m.filteredTasks = append(m.filteredTasks, t)
			}
		case FilterSystemdOnly:
			if t.SourceType == "SYSTEMD" {
				m.filteredTasks = append(m.filteredTasks, t)
			}
		default:
			m.filteredTasks = append(m.filteredTasks, t)
		}
	}
	m.updateTableRows()
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, t := range m.filteredTasks {
		rows = append(rows, table.Row{
			t.SourceType,
			t.Origin,
			t.Schedule,
			t.NextRunStr,
			t.Countdown,
			t.Target,
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

	case TimersLoadedMsg:
		m.isLoading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.tasks = msg.Tasks
			m.applyFilter()
		}

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				switch action {
				case "filter_all":
					m.filter = FilterAll
					m.applyFilter()
				case "filter_cron":
					m.filter = FilterCronOnly
					m.applyFilter()
				case "filter_systemd":
					m.filter = FilterSystemdOnly
					m.applyFilter()
				case "refresh":
					m.isLoading = true
					cmds = append(cmds, m.FetchScheduledTasks())
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "space":
			title := "Scheduled Timers & Cron"
			subtitle := "Select filter or action"
			items := []actionmenu.Item{
				{Key: "filter_all", Title: "Filter: All Tasks", Description: "Show cron jobs and systemd timers"},
				{Key: "filter_cron", Title: "Filter: Crontabs Only", Description: "Show user & system crontabs"},
				{Key: "filter_systemd", Title: "Filter: Systemd Timers Only", Description: "Show systemd timer units"},
				{Key: "refresh", Title: "Refresh Timeline", Description: "Recalculate countdowns and schedules"},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil
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
	if availableWidth > 80 {
		targetWidth := availableWidth - (10 + 22 + 18 + 20 + 16 + 10)
		if targetWidth < 20 {
			targetWidth = 20
		}
		cols := []table.Column{
			{Title: "TYPE", Width: 10},
			{Title: "SOURCE / ORIGIN", Width: 22},
			{Title: "SCHEDULE", Width: 18},
			{Title: "NEXT RUN", Width: 20},
			{Title: "COUNTDOWN", Width: 16},
			{Title: "TRIGGERED COMMAND / SERVICE", Width: targetWidth},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var cronCount, timerCount int
	for _, t := range m.tasks {
		if t.SourceType == "CRON" {
			cronCount++
		} else {
			timerCount++
		}
	}

	filterName := "all"
	switch m.filter {
	case FilterCronOnly:
		filterName = "crons only"
	case FilterSystemdOnly:
		filterName = "systemd timers only"
	}

	headerLine := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Scheduled Tasks"),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("● %d cron jobs", cronCount)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(fmt.Sprintf("○ %d systemd timers", timerCount)),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("filter: %s (showing %d of %d)", filterName, len(m.filteredTasks), len(m.tasks))),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		headerLine,
		"",
		m.table.View(),
	)

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(body)

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
