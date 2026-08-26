package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type GitFileStatus struct {
	Status string // "M", "A", "D", "??", "MM"
	Path   string
	Staged bool
}

type CommitItem struct {
	Hash    string
	Author  string
	Time    string
	Message string
}

type GitStateMsg struct {
	Branch  string
	Files   []GitFileStatus
	Commits []CommitItem
	Err     error
}

type GitActionMsg struct {
	Action string
	Err    error
}

type GitDiffMsg struct {
	FilePath string
	Diff     string
}

type Model struct {
	branch        string
	files         []GitFileStatus
	commits       []CommitItem
	filesTable    table.Model
	commitsTable  table.Model
	diffViewport  viewport.Model
	actionMenu    actionmenu.Model
	viewingDiff   bool
	activeDiff    string
	actionStatus  string
	focusCommits  bool
	isLoading     bool
	width         int
	height        int
	err           error
}

func New() Model {
	ft := table.New(
		table.WithColumns([]table.Column{
			{Title: "STATUS", Width: 8},
			{Title: "FILE PATH", Width: 45},
		}),
		table.WithFocused(true),
		table.WithHeight(6),
	)

	ct := table.New(
		table.WithColumns([]table.Column{
			{Title: "HASH", Width: 10},
			{Title: "AUTHOR", Width: 16},
			{Title: "DATE", Width: 16},
			{Title: "MESSAGE", Width: 35},
		}),
		table.WithFocused(false),
		table.WithHeight(6),
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

	ft.SetStyles(s)
	ct.SetStyles(s)

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1)

	return Model{
		filesTable:   ft,
		commitsTable: ct,
		diffViewport: vp,
		isLoading:    true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchGitState()
}

func (m Model) FetchGitState() tea.Cmd {
	return func() tea.Msg {
		repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
		if err != nil {
			return GitStateMsg{Err: fmt.Errorf("not a git repository: %v", err)}
		}

		// 1. Branch
		branchName := "HEAD"
		head, err := repo.Head()
		if err == nil {
			if head.Name().IsBranch() {
				branchName = head.Name().Short()
			} else {
				branchName = head.Hash().String()[:8]
			}
		}

		// 2. Status via Worktree
		var fileStatuses []GitFileStatus
		if wt, err := repo.Worktree(); err == nil {
			if st, err := wt.Status(); err == nil {
				for path, s := range st {
					code := ""
					staged := false
					if s.Staging != git.Unmodified && s.Staging != git.Untracked {
						code = string(s.Staging)
						staged = true
					} else if s.Worktree != git.Unmodified {
						code = string(s.Worktree)
					}
					if s.Worktree == git.Untracked {
						code = "??"
					}

					fileStatuses = append(fileStatuses, GitFileStatus{
						Status: code,
						Path:   path,
						Staged: staged,
					})
				}
			}
		}

		// 3. Commits History
		var commitItems []CommitItem
		out, err := exec.Command("git", "log", "-n", "15", "--pretty=format:%h|%an|%ar|%s").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				parts := strings.Split(line, "|")
				if len(parts) >= 4 {
					commitItems = append(commitItems, CommitItem{
						Hash:    parts[0],
						Author:  parts[1],
						Time:    parts[2],
						Message: parts[3],
					})
				}
			}
		}

		return GitStateMsg{
			Branch:  branchName,
			Files:   fileStatuses,
			Commits: commitItems,
		}
	}
}

func (m Model) FetchDiff(filePath string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if filePath == "" {
			cmd = exec.Command("git", "diff", "HEAD")
		} else {
			cmd = exec.Command("git", "diff", "HEAD", "--", filePath)
		}

		out, err := cmd.CombinedOutput()
		if err != nil || len(out) == 0 {
			// Try diff without HEAD
			out, _ = exec.Command("git", "diff", "--", filePath).CombinedOutput()
		}

		diffStr := strings.TrimSpace(string(out))
		if diffStr == "" {
			diffStr = "(No diff or new untracked file)"
		}

		// Colorize diff lines
		lines := strings.Split(diffStr, "\n")
		var coloredLines []string
		for _, l := range lines {
			if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
				coloredLines = append(coloredLines, lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(l))
			} else if strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") {
				coloredLines = append(coloredLines, lipgloss.NewStyle().Foreground(theme.ColorDanger).Render(l))
			} else if strings.HasPrefix(l, "@@") {
				coloredLines = append(coloredLines, lipgloss.NewStyle().Foreground(theme.ColorInfo).Bold(true).Render(l))
			} else {
				coloredLines = append(coloredLines, l)
			}
		}

		return GitDiffMsg{
			FilePath: filePath,
			Diff:     strings.Join(coloredLines, "\n"),
		}
	}
}

func (m Model) RunGitCommand(action string, args ...string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			return GitActionMsg{Action: action, Err: fmt.Errorf("%v: %s", err, string(out))}
		}
		return GitActionMsg{Action: action}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case GitStateMsg:
		m.isLoading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.branch = msg.Branch
			m.files = msg.Files
			m.commits = msg.Commits
			m.updateTables()
		}

	case GitDiffMsg:
		m.activeDiff = msg.FilePath
		m.setDiffContent(msg.Diff)
		m.diffViewport.GotoTop()
		m.viewingDiff = true

	case GitActionMsg:
		if msg.Err != nil {
			m.actionStatus = fmt.Sprintf("Failed to %s: %v", msg.Action, msg.Err)
		} else {
			m.actionStatus = fmt.Sprintf("Successfully executed '%s'", msg.Action)
			cmds = append(cmds, m.FetchGitState())
		}

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				idx := m.filesTable.Cursor()
				switch action {
				case "diff":
					if len(m.files) > 0 && idx >= 0 && idx < len(m.files) {
						cmds = append(cmds, m.FetchDiff(m.files[idx].Path))
					}
				case "stage":
					if len(m.files) > 0 && idx >= 0 && idx < len(m.files) {
						cmds = append(cmds, m.RunGitCommand("Stage File", "add", m.files[idx].Path))
					}
				case "unstage":
					if len(m.files) > 0 && idx >= 0 && idx < len(m.files) {
						cmds = append(cmds, m.RunGitCommand("Unstage File", "reset", "HEAD", "--", m.files[idx].Path))
					}
				case "stash":
					cmds = append(cmds, m.RunGitCommand("Stash Changes", "stash"))
				case "pop":
					cmds = append(cmds, m.RunGitCommand("Pop Stash", "stash", "pop"))
				case "pull":
					cmds = append(cmds, m.RunGitCommand("Git Pull", "pull", "--ff-only"))
				case "refresh":
					m.isLoading = true
					cmds = append(cmds, m.FetchGitState())
				}
			}
			return m, tea.Batch(cmds...)
		}

		if m.viewingDiff {
			switch msg.String() {
			case "esc", "q", "tab", "shift+tab":
				m.viewingDiff = false
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.diffViewport, vpCmd = m.diffViewport.Update(msg)
				return m, vpCmd
			}
		}

		switch msg.String() {
		case "tab", "shift+tab":
			m.focusCommits = !m.focusCommits
			m.filesTable.Focus()
			m.commitsTable.Focus()
			if m.focusCommits {
				m.filesTable.Blur()
			} else {
				m.commitsTable.Blur()
			}
			return m, nil

		case "space":
			title := "Git Inspector"
			subtitle := "Select Git action"
			if !m.focusCommits && len(m.files) > 0 {
				idx := m.filesTable.Cursor()
				if idx >= 0 && idx < len(m.files) {
					title = "Actions: " + m.files[idx].Path
					subtitle = fmt.Sprintf("Status: %s (Staged: %t)", m.files[idx].Status, m.files[idx].Staged)
				}
			}
			items := []actionmenu.Item{
				{Key: "diff", Title: "View Diff", Description: "Show line-by-line diff of file"},
				{Key: "stage", Title: "Stage File", Description: "git add file"},
				{Key: "unstage", Title: "Unstage File", Description: "git reset HEAD file"},
				{Key: "stash", Title: "Stash Changes", Description: "git stash working tree"},
				{Key: "pop", Title: "Pop Stash", Description: "git stash pop last stash"},
				{Key: "pull", Title: "Pull Changes", Description: "git pull --ff-only"},
				{Key: "refresh", Title: "Refresh Repository", Description: "Reload git status and commit log"},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil

		case "enter":
			if !m.focusCommits && len(m.files) > 0 {
				idx := m.filesTable.Cursor()
				if idx >= 0 && idx < len(m.files) {
					cmds = append(cmds, m.FetchDiff(m.files[idx].Path))
				}
			}
		}
	}

	var tCmd tea.Cmd
	if m.focusCommits {
		m.commitsTable, tCmd = m.commitsTable.Update(msg)
	} else {
		m.filesTable, tCmd = m.filesTable.Update(msg)
	}
	if tCmd != nil {
		cmds = append(cmds, tCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTables() {
	var fRows []table.Row
	for _, f := range m.files {
		statusCol := f.Status
		if f.Staged {
			statusCol = "● " + f.Status
		} else {
			statusCol = "○ " + f.Status
		}
		fRows = append(fRows, table.Row{statusCol, f.Path})
	}
	m.filesTable.SetRows(fRows)

	var cRows []table.Row
	for _, c := range m.commits {
		cRows = append(cRows, table.Row{c.Hash, c.Author, c.Time, c.Message})
	}
	m.commitsTable.SetRows(cRows)
}

func (m *Model) updateLayout() {
	vpWidth := m.width - 8
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 8
	if vpHeight < 10 {
		vpHeight = 10
	}
	m.diffViewport.Width = vpWidth
	m.diffViewport.Height = vpHeight

	tableH := (m.height - 14) / 2
	if tableH < 4 {
		tableH = 4
	}
	m.filesTable.SetHeight(tableH)
	m.commitsTable.SetHeight(tableH)
}

func (m *Model) setDiffContent(text string) {
	w := m.diffViewport.Width - 2
	if w < 20 {
		w = 20
	}
	m.diffViewport.SetContent(lipgloss.NewStyle().Width(w).Render(text))
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(theme.ColorDanger).
			Padding(1, 2).
			Render(fmt.Sprintf("Git error: %v", m.err))
	}

	if m.viewingDiff {
		diffHeader := lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("diff: "+m.activeDiff),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Esc: close  j/k: scroll"),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			diffHeader,
			"",
			m.diffViewport.View(),
		)
	}

	// Branch badge + status
	branchLine := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("branch: "),
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSuccess).Render(m.branch),
	)
	if m.actionStatus != "" {
		branchLine = lipgloss.JoinHorizontal(lipgloss.Center,
			branchLine,
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.actionStatus),
		)
	}

	hintLine := lipgloss.NewStyle().Foreground(theme.ColorMuted).
		Render("Tab: switch panes  Enter: diff  Space: actions")

	// Pane labels — show focus indicator simply
	filesLabel := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Changed files")
	if !m.focusCommits {
		filesLabel = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Changed files")
	}

	commitsLabel := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Recent commits")
	if m.focusCommits {
		commitsLabel = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Recent commits")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		branchLine,
		hintLine,
		"",
		filesLabel,
		m.filesTable.View(),
		"",
		commitsLabel,
		m.commitsTable.View(),
	)

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(body)

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
