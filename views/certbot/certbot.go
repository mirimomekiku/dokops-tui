package certbot

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type CertbotResultMsg struct {
	Success bool
	Output  string
	Err     error
}

type DNSCheckMsg struct {
	Domain string
	IPs    []string
	Err    error
}

type Model struct {
	domainInput   textinput.Model
	emailInput    textinput.Model
	logViewport   viewport.Model
	focusEmail    bool
	isProcessing  bool
	dnsIPs        []string
	dnsChecked    bool
	statusMessage string
	width         int
	height        int
	err           error
}

func New() Model {
	di := textinput.New()
	di.Placeholder = "example.com or api.example.com"
	di.SetValue("app.example.com")
	di.Focus()
	di.CharLimit = 255
	di.Width = 35

	ei := textinput.New()
	ei.Placeholder = "admin@example.com"
	ei.SetValue("admin@example.com")
	ei.CharLimit = 255
	ei.Width = 35

	vp := viewport.New(80, 14)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1)

	return Model{
		domainInput: di,
		emailInput:  ei,
		logViewport: vp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.CheckDNS(),
	)
}

func (m Model) CheckDNS() tea.Cmd {
	domain := strings.TrimSpace(m.domainInput.Value())
	return func() tea.Msg {
		ips, err := net.LookupIP(domain)
		var ipStrs []string
		if err == nil {
			for _, ip := range ips {
				ipStrs = append(ipStrs, ip.String())
			}
		}
		return DNSCheckMsg{
			Domain: domain,
			IPs:    ipStrs,
			Err:    err,
		}
	}
}

func (m Model) RunCertbot(dryRun bool) tea.Cmd {
	domain := strings.TrimSpace(m.domainInput.Value())
	email := strings.TrimSpace(m.emailInput.Value())

	return func() tea.Msg {
		args := []string{
			"--nginx",
			"-d", domain,
			"--non-interactive",
			"--agree-tos",
			"-m", email,
			"--redirect",
		}
		if dryRun {
			args = append(args, "--dry-run")
		}

		start := time.Now()
		out, err := exec.Command("certbot", args...).CombinedOutput()
		if err != nil {
			// Try with sudo
			sudoArgs := append([]string{"certbot"}, args...)
			out, err = exec.Command("sudo", sudoArgs...).CombinedOutput()
		}

		duration := time.Since(start)
		if err != nil {
			return CertbotResultMsg{
				Success: false,
				Output:  fmt.Sprintf("❌ Certbot failed (took %s):\n%v\n\n%s", theme.FormatDuration(duration), err, string(out)),
				Err:     err,
			}
		}

		dryMsg := ""
		if dryRun {
			dryMsg = " (DRY-RUN SIMULATION SUCCEEDED)"
		}
		return CertbotResultMsg{
			Success: true,
			Output:  fmt.Sprintf("✓ Certificate successfully provisioned for %s%s!\nTook %s.\n\n%s", domain, dryMsg, theme.FormatDuration(duration), string(out)),
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

	case DNSCheckMsg:
		m.dnsChecked = true
		m.dnsIPs = msg.IPs
		m.err = msg.Err

	case CertbotResultMsg:
		m.isProcessing = false
		m.logViewport.SetContent(msg.Output)
		m.logViewport.GotoBottom()
		if msg.Success {
			m.statusMessage = "✓ SSL Certificate successfully configured!"
		} else {
			m.statusMessage = "❌ Provisioning encountered errors. See logs below."
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focusEmail = !m.focusEmail
			if m.focusEmail {
				m.emailInput.Focus()
				m.domainInput.Blur()
			} else {
				m.domainInput.Focus()
				m.emailInput.Blur()
			}
			return m, nil

		case "t":
			// Dry-run
			m.isProcessing = true
			m.statusMessage = "Running Certbot dry-run test (checking Let's Encrypt challenge)..."
			m.logViewport.SetContent("▶ Testing domain challenge with --dry-run...")
			return m, m.RunCertbot(true)

		case "p", "enter":
			// Full provision
			m.isProcessing = true
			m.statusMessage = "Provisioning SSL certificate and configuring Nginx..."
			m.logViewport.SetContent("▶ Requesting certificate from Let's Encrypt and updating Nginx...")
			return m, m.RunCertbot(false)

		case "d":
			return m, m.CheckDNS()
		}

		var cmd tea.Cmd
		if m.focusEmail {
			m.emailInput, cmd = m.emailInput.Update(msg)
		} else {
			m.domainInput, cmd = m.domainInput.Update(msg)
		}
		cmds = append(cmds, cmd)
	}

	var vpCmd tea.Cmd
	m.logViewport, vpCmd = m.logViewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateLayout() {
	vpWidth := m.width - 8
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 18
	if vpHeight < 6 {
		vpHeight = 6
	}
	m.logViewport.Width = vpWidth
	m.logViewport.Height = vpHeight
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	domBorder := theme.ColorBorder
	if !m.focusEmail {
		domBorder = theme.ColorPrimary
	}
	domainBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(domBorder).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Target Domain: "),
				m.domainInput.View(),
			),
		)

	mailBorder := theme.ColorBorder
	if m.focusEmail {
		mailBorder = theme.ColorPrimary
	}
	emailBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(mailBorder).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("Admin Email: "),
				m.emailInput.View(),
			),
		)

	var dnsBadge string
	if len(m.dnsIPs) > 0 {
		dnsBadge = theme.BadgeSuccess.Render(fmt.Sprintf(" DNS Resolved: %s ", strings.Join(m.dnsIPs, ", ")))
	} else if m.dnsChecked {
		dnsBadge = theme.BadgeWarning.Render(" DNS: Unresolved or Local ")
	}

	statusLine := ""
	if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true).Render(m.statusMessage)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🔒 SSL & LET'S ENCRYPT CERTBOT WIZARD"),
			"   ",
			dnsBadge,
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[t: Dry-run Test | p/Enter: Provision Cert | d: Check DNS]"),
		),
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			domainBox,
			"  ",
			emailBox,
		),
		"",
		statusLine,
		"",
		theme.CardTitleStyle.Render("📜 CERTBOT EXECUTION STREAM"),
		m.logViewport.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
