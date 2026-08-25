package ci

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v60/github"

	"dok-ops/internal/theme"
)

type WorkflowRunItem struct {
	ID         int64
	Name       string
	Status     string
	Conclusion string
	Branch     string
	Commit     string
	Event      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	URL        string
}

type WorkflowRunsLoadedMsg struct {
	Runs []WorkflowRunItem
	Repo string
	Err  error
}

type Model struct {
	repoInput    textinput.Model
	table        table.Model
	viewport     viewport.Model
	runs         []WorkflowRunItem
	viewingRun   bool
	activeRun    *WorkflowRunItem
	activeRepo   string
	isLoading    bool
	width        int
	height       int
	err          error
}

func New() Model {
	ri := textinput.New()
	ri.Placeholder = "owner/repo (e.g. charmbracelet/bubbletea)"
	ri.SetValue(detectLocalRepo())
	ri.Focus()
	ri.CharLimit = 200
	ri.Width = 40

	cols := []table.Column{
		{Title: "WORKFLOW", Width: 22},
		{Title: "STATUS", Width: 12},
		{Title: "EVENT", Width: 12},
		{Title: "BRANCH", Width: 16},
		{Title: "COMMIT", Width: 10},
		{Title: "DATE", Width: 16},
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
		repoInput: ri,
		table:     t,
		viewport:  vp,
		isLoading: true,
	}
}

func detectLocalRepo() string {
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err == nil {
		str := strings.TrimSpace(string(out))
		str = strings.TrimPrefix(str, "https://github.com/")
		str = strings.TrimPrefix(str, "git@github.com:")
		str = strings.TrimSuffix(str, ".git")
		if strings.Contains(str, "/") {
			return str
		}
	}
	return "charmbracelet/bubbletea"
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.FetchWorkflowRuns(),
	)
}

func (m Model) FetchWorkflowRuns() tea.Cmd {
	repoStr := strings.TrimSpace(m.repoInput.Value())
	if repoStr == "" {
		repoStr = "charmbracelet/bubbletea"
	}

	parts := strings.Split(repoStr, "/")
	if len(parts) < 2 {
		return func() tea.Msg {
			return WorkflowRunsLoadedMsg{Err: fmt.Errorf("invalid repository format (expected owner/repo)")}
		}
	}
	owner, repo := parts[0], parts[1]

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
				token = strings.TrimSpace(string(out))
			}
		}

		client := github.NewClient(nil)
		if token != "" {
			client = client.WithAuthToken(token)
		}

		runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, &github.ListWorkflowRunsOptions{
			ListOptions: github.ListOptions{PerPage: 20},
		})
		if err != nil {
			return WorkflowRunsLoadedMsg{Repo: repoStr, Err: err}
		}

		var items []WorkflowRunItem
		for _, r := range runs.WorkflowRuns {
			name := r.GetName()
			status := r.GetStatus()
			conclusion := r.GetConclusion()
			branch := r.GetHeadBranch()
			commit := r.GetHeadSHA()
			if len(commit) > 7 {
				commit = commit[:7]
			}
			event := r.GetEvent()
			items = append(items, WorkflowRunItem{
				ID:         r.GetID(),
				Name:       name,
				Status:     status,
				Conclusion: conclusion,
				Branch:     branch,
				Commit:     commit,
				Event:      event,
				CreatedAt:  r.GetCreatedAt().Time,
				UpdatedAt:  r.GetUpdatedAt().Time,
				URL:        r.GetHTMLURL(),
			})
		}

		return WorkflowRunsLoadedMsg{
			Runs: items,
			Repo: repoStr,
		}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case WorkflowRunsLoadedMsg:
		m.isLoading = false
		m.err = msg.Err
		m.activeRepo = msg.Repo
		if msg.Err == nil {
			m.runs = msg.Runs
			m.updateTableRows()
		}

	case tea.KeyMsg:
		if m.viewingRun {
			switch msg.String() {
			case "esc", "q":
				m.viewingRun = false
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.viewport, vpCmd = m.viewport.Update(msg)
				return m, vpCmd
			}
		}

		switch msg.String() {
		case "enter":
			m.isLoading = true
			return m, m.FetchWorkflowRuns()
		case "v":
			if len(m.runs) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.runs) {
					m.activeRun = &m.runs[idx]
					m.renderRunDetails()
					m.viewingRun = true
				}
			}
		case "r":
			m.isLoading = true
			return m, m.FetchWorkflowRuns()
		}

		var cmd tea.Cmd
		m.repoInput, cmd = m.repoInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	if tableCmd != nil {
		cmds = append(cmds, tableCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) renderRunDetails() {
	if m.activeRun == nil {
		return
	}
	r := m.activeRun
	var sb strings.Builder

	statusBadge := theme.BadgeSuccess.Render(" " + strings.ToUpper(r.Conclusion) + " ")
	if r.Conclusion == "failure" {
		statusBadge = theme.BadgeDanger.Render(" " + strings.ToUpper(r.Conclusion) + " ")
	} else if r.Status == "in_progress" {
		statusBadge = theme.BadgeWarning.Render(" IN PROGRESS ")
	}

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgePrimary.Render(fmt.Sprintf(" Run #%d ", r.ID)),
		" ",
		statusBadge,
		"  ",
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render(r.Name),
	) + "\n\n")

	sb.WriteString(fmt.Sprintf("• Branch:   %s\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(r.Branch)))
	sb.WriteString(fmt.Sprintf("• Commit:   %s\n", lipgloss.NewStyle().Foreground(theme.ColorSecondary).Render(r.Commit)))
	sb.WriteString(fmt.Sprintf("• Event:    %s\n", r.Event))
	sb.WriteString(fmt.Sprintf("• Started:  %s\n", r.CreatedAt.Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("• Updated:  %s\n", r.UpdatedAt.Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("• HTML URL: %s\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(r.URL)))

	m.viewport.SetContent(sb.String())
	m.viewport.GotoTop()
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, r := range m.runs {
		statusStr := r.Conclusion
		if statusStr == "" {
			statusStr = r.Status
		}
		rows = append(rows, table.Row{
			r.Name,
			statusStr,
			r.Event,
			r.Branch,
			r.Commit,
			r.CreatedAt.Format("02 Jan 15:04"),
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

	vpWidth := m.width - 8
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 8
	if vpHeight < 10 {
		vpHeight = 10
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.viewingRun {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Center,
				theme.BadgeInfo.Render(" WORKFLOW RUN DETAILS "),
				"   ",
				lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Esc / q: Back to runs list]"),
			),
			"",
			m.viewport.View(),
		)
	}

	repoBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("GitHub Repo: "),
				m.repoInput.View(),
				"  ",
				lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Enter: Search]"),
			),
		)

	var statusBadge string
	if m.isLoading {
		statusBadge = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("⏳ Polling workflow runs from GitHub API...")
	} else if m.err != nil {
		statusBadge = theme.BadgeDanger.Render(fmt.Sprintf(" API Error: %v ", m.err))
	} else {
		statusBadge = theme.BadgeSuccess.Render(fmt.Sprintf(" %s (%d runs) ", m.activeRepo, len(m.runs)))
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🚀 GITHUB ACTIONS & CI RUNNER STATUS"),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[v: View Run Details | r: Refresh]"),
		),
		"",
		repoBox,
		"",
		statusBadge,
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
