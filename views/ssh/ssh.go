package ssh

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"

	"dok-ops/internal/theme"
)

type SSHSession struct {
	User      string
	TTY       string
	FromIP    string
	LoginTime string
}

type SSHKeyItem struct {
	Type        string
	Fingerprint string
	Comment     string
	LineNum     int
}

type SSHDataLoadedMsg struct {
	Sessions []SSHSession
	Keys     []SSHKeyItem
	Err      error
}

type Model struct {
	sessions      []SSHSession
	keys          []SSHKeyItem
	sessionsTable table.Model
	keysTable     table.Model
	focusKeys     bool
	confirmKill   bool
	killTTY       string
	killStatus    string
	isLoading     bool
	width         int
	height        int
	err           error
}

func New() Model {
	st := table.New(
		table.WithColumns([]table.Column{
			{Title: "USER", Width: 14},
			{Title: "TTY", Width: 12},
			{Title: "REMOTE IP / HOST", Width: 22},
			{Title: "LOGIN TIME", Width: 22},
		}),
		table.WithFocused(true),
		table.WithHeight(5),
	)

	kt := table.New(
		table.WithColumns([]table.Column{
			{Title: "TYPE", Width: 14},
			{Title: "FINGERPRINT (SHA256)", Width: 32},
			{Title: "COMMENT / IDENTITY", Width: 35},
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

	st.SetStyles(s)
	kt.SetStyles(s)

	return Model{
		sessionsTable: st,
		keysTable:     kt,
		isLoading:     true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.FetchSSHData()
}

func (m Model) FetchSSHData() tea.Cmd {
	return func() tea.Msg {
		// 1. Fetch active sessions via 'who'
		var sessions []SSHSession
		out, err := exec.Command("who").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					user := fields[0]
					tty := fields[1]
					loginTime := fields[2]
					if len(fields) > 3 {
						loginTime += " " + fields[3]
					}
					fromIP := "local"
					if len(fields) >= 5 {
						fromIP = strings.Trim(fields[len(fields)-1], "()")
					}
					sessions = append(sessions, SSHSession{
						User:      user,
						TTY:       tty,
						FromIP:    fromIP,
						LoginTime: loginTime,
					})
				}
			}
		}

		// 2. Fetch authorized_keys
		var keys []SSHKeyItem
		home, _ := os.UserHomeDir()
		authKeysPath := filepath.Join(home, ".ssh", "authorized_keys")

		data, err := os.ReadFile(authKeysPath)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for lineIdx, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}

				pubKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(trimmed))
				if err == nil {
					fp := sha256.Sum256(pubKey.Marshal())
					fpStr := "SHA256:" + base64.StdEncoding.EncodeToString(fp[:])
					if comment == "" {
						comment = "(no comment)"
					}
					keys = append(keys, SSHKeyItem{
						Type:        pubKey.Type(),
						Fingerprint: fpStr,
						Comment:     comment,
						LineNum:     lineIdx + 1,
					})
				}
			}
		}

		return SSHDataLoadedMsg{
			Sessions: sessions,
			Keys:     keys,
		}
	}
}

func (m Model) KillSession(tty string) tea.Cmd {
	return func() tea.Msg {
		cleanTTY := strings.TrimPrefix(tty, "pts/")
		cleanTTY = strings.TrimPrefix(cleanTTY, "tty")
		out, err := exec.Command("pkill", "-9", "-t", cleanTTY).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Failed to kill session %s: %v (%s)", tty, err, string(out))
		}
		return fmt.Sprintf("Killed session on %s", tty)
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case SSHDataLoadedMsg:
		m.isLoading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.sessions = msg.Sessions
			m.keys = msg.Keys
			m.updateTables()
		}

	case string:
		m.killStatus = msg
		cmds = append(cmds, m.FetchSSHData())

	case tea.KeyMsg:
		if m.confirmKill {
			switch msg.String() {
			case "y", "Y":
				if m.killTTY != "" {
					cmds = append(cmds, m.KillSession(m.killTTY))
				}
				m.confirmKill = false
			case "n", "N", "esc":
				m.confirmKill = false
				m.killStatus = "Kill cancelled"
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "tab":
			m.focusKeys = !m.focusKeys
			m.sessionsTable.Focus()
			m.keysTable.Focus()
			if m.focusKeys {
				m.sessionsTable.Blur()
			} else {
				m.keysTable.Blur()
			}
			return m, nil

		case "k":
			if !m.focusKeys && len(m.sessions) > 0 {
				idx := m.sessionsTable.Cursor()
				if idx >= 0 && idx < len(m.sessions) {
					m.killTTY = m.sessions[idx].TTY
					m.confirmKill = true
				}
			}

		case "r", "R":
			m.isLoading = true
			cmds = append(cmds, m.FetchSSHData())
		}
	}

	var tCmd tea.Cmd
	if m.focusKeys {
		m.keysTable, tCmd = m.keysTable.Update(msg)
	} else {
		m.sessionsTable, tCmd = m.sessionsTable.Update(msg)
	}
	if tCmd != nil {
		cmds = append(cmds, tCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTables() {
	var sRows []table.Row
	for _, s := range m.sessions {
		sRows = append(sRows, table.Row{s.User, s.TTY, s.FromIP, s.LoginTime})
	}
	m.sessionsTable.SetRows(sRows)

	var kRows []table.Row
	for _, k := range m.keys {
		kRows = append(kRows, table.Row{k.Type, k.Fingerprint, k.Comment})
	}
	m.keysTable.SetRows(kRows)
}

func (m *Model) updateLayout() {
	tableH := (m.height - 14) / 2
	if tableH < 4 {
		tableH = 4
	}
	m.sessionsTable.SetHeight(tableH)
	m.keysTable.SetHeight(tableH)
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Active SSH Sessions ", len(m.sessions))),
		" ",
		theme.BadgeInfo.Render(fmt.Sprintf(" %d Authorized Keys ", len(m.keys))),
	)

	statusLine := ""
	if m.confirmKill {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("⚠️ Terminate SSH session on %s? (y/N)", m.killTTY))
	} else if m.killStatus != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.killStatus)
	}

	sessTitle := theme.CardTitleStyle.Render("💻 ACTIVE LOGIN SESSIONS")
	if !m.focusKeys {
		sessTitle = theme.CardTitleStyle.Foreground(theme.ColorHighlight).Render("💻 ACTIVE LOGIN SESSIONS (Focused)")
	}

	keysTitle := theme.CardTitleStyle.Render("🔑 AUTHORIZED SSH KEYS (~/.ssh/authorized_keys)")
	if m.focusKeys {
		keysTitle = theme.CardTitleStyle.Foreground(theme.ColorHighlight).Render("🔑 AUTHORIZED SSH KEYS (Focused)")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🛡️ SSH SESSIONS & KEY AUDITOR"),
			"   ",
			statsBadge,
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Tab: Switch Pane | k: Kill Session | r: Refresh]"),
		),
		statusLine,
		"",
		sessTitle,
		m.sessionsTable.View(),
		"",
		keysTitle,
		m.keysTable.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
