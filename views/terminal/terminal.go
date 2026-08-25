package terminal

import (
	"io"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"

	"dok-ops/internal/theme"
)

type TerminalDataMsg string
type TerminalExitMsg struct{ Err error }

type Model struct {
	ptyFile   *os.File
	cmd       *exec.Cmd
	viewport  viewport.Model
	isFocused bool
	width     int
	height    int
	history   string
	exited    bool
	err       error
}

func New() Model {
	vp := viewport.New(80, 24)
	vp.Style = lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1b26")).
		Foreground(lipgloss.Color("#c0caf5"))

	return Model{
		viewport:  vp,
		isFocused: false,
	}
}

func (m *Model) InitPTY() tea.Cmd {
	return func() tea.Msg {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}

		c := exec.Command(shell)
		c.Env = append(os.Environ(), "TERM=xterm-256color")

		ptmx, err := pty.Start(c)
		if err != nil {
			return TerminalExitMsg{Err: err}
		}

		m.ptyFile = ptmx
		m.cmd = c

		// Initial window size
		_ = pty.Setsize(ptmx, &pty.Winsize{
			Rows: uint16(m.height - 6),
			Cols: uint16(m.width - 4),
		})

		return nil
	}
}

func (m Model) Init() tea.Cmd {
	return m.InitPTY()
}

func (m *Model) ReadPTY(p *tea.Program) {
	if m.ptyFile == nil {
		return
	}
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := m.ptyFile.Read(buf)
			if err != nil {
				if err != io.EOF {
					p.Send(TerminalExitMsg{Err: err})
				}
				return
			}
			if n > 0 {
				data := string(buf[:n])
				p.Send(TerminalDataMsg(data))
			}
		}
	}()
}

func (m *Model) SetFocus(focused bool) {
	m.isFocused = focused
}

func (m Model) IsFocused() bool {
	return m.isFocused
}

func (m *Model) Close() {
	if m.ptyFile != nil {
		_ = m.ptyFile.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case TerminalDataMsg:
		m.history += string(msg)
		// Keep history manageable (last 30000 chars)
		if len(m.history) > 50000 {
			m.history = m.history[len(m.history)-30000:]
		}
		m.viewport.SetContent(m.history)
		m.viewport.GotoBottom()

	case TerminalExitMsg:
		m.exited = true
		m.err = msg.Err

	case tea.KeyMsg:
		// When focused, pass raw keys to PTY
		if m.isFocused && m.ptyFile != nil {
			switch msg.String() {
			case "ctrl+]":
				// Escape focus back to main router
				m.isFocused = false
				return m, nil
			default:
				// Forward key raw bytes to PTY
				rawBytes := msgToBytes(msg)
				if len(rawBytes) > 0 {
					_, _ = m.ptyFile.Write(rawBytes)
				}
				return m, nil
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func msgToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeyEnter:
		return []byte("\r")
	case tea.KeyBackspace:
		return []byte("\b")
	case tea.KeyTab:
		return []byte("\t")
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyCtrlC:
		return []byte("\x03")
	case tea.KeyCtrlD:
		return []byte("\x04")
	case tea.KeyCtrlZ:
		return []byte("\x1a")
	case tea.KeyCtrlL:
		return []byte("\x0c")
	case tea.KeyCtrlA:
		return []byte("\x01")
	case tea.KeyCtrlE:
		return []byte("\x05")
	case tea.KeyCtrlU:
		return []byte("\x15")
	case tea.KeyCtrlW:
		return []byte("\x17")
	case tea.KeyCtrlK:
		return []byte("\x0b")
	case tea.KeyEscape:
		return []byte("\x1b")
	default:
		return []byte(msg.String())
	}
}

func (m *Model) updateLayout() {
	vpWidth := m.width - 4
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 6
	if vpHeight < 10 {
		vpHeight = 10
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight

	if m.ptyFile != nil {
		_ = pty.Setsize(m.ptyFile, &pty.Winsize{
			Rows: uint16(vpHeight),
			Cols: uint16(vpWidth),
		})
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	borderColor := theme.ColorBorder
	focusBanner := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Press 'i' to focus shell | Ctrl+] to unfocus | F1-F6 to jump tabs]")
	if m.isFocused {
		borderColor = theme.ColorPrimary
		focusBanner = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(theme.ColorWarning).
			Bold(true).
			Padding(0, 1).
			Render(" 🔴 SHELL CAPTURE ACTIVE (Ctrl+] to unfocus, F1-F6 to switch tabs) ")
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.CardTitleStyle.Render("💻 MULTIPLEXED PTY SUBSHELL"),
		"   ",
		focusBanner,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(contentWidth).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				header,
				"",
				m.viewport.View(),
			),
		)
}
