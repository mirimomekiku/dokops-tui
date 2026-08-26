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

	"dok-ops/internal/actionmenu"
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
	actionMenu    actionmenu.Model
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
				Output:  fmt.Sprintf("Certbot failed (took %s):\n%v\n\n%s", theme.FormatDuration(duration), err, string(out)),
				Err:     err,
			}
		}

		dryMsg := ""
		if dryRun {
			dryMsg = " (DRY-RUN SIMULATION SUCCEEDED)"
		}
		return CertbotResultMsg{
			Success: true,
			Output:  fmt.Sprintf("Certificate successfully provisioned for %s%s\nTook %s.\n\n%s", domain, dryMsg, theme.FormatDuration(duration), string(out)),
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
		m.setLogContent(msg.Output)
		m.logViewport.GotoBottom()
		if msg.Success {
			m.statusMessage = "SSL Certificate successfully configured"
		} else {
			m.statusMessage = "Provisioning encountered errors. See logs below."
		}

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				switch action {
				case "provision":
					m.isProcessing = true
					m.statusMessage = "Provisioning SSL certificate and configuring Nginx..."
					m.setLogContent("▶ Requesting certificate from Let's Encrypt and updating Nginx...")
					return m, m.RunCertbot(false)
				case "dry_run":
					m.isProcessing = true
					m.statusMessage = "Running Certbot dry-run test (checking Let's Encrypt challenge)..."
					m.setLogContent("▶ Testing domain challenge with --dry-run...")
					return m, m.RunCertbot(true)
				case "check_dns":
					return m, m.CheckDNS()
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "tab", "shift+tab", "up", "down", "ctrl+n", "ctrl+p":
			m.focusEmail = !m.focusEmail
			if m.focusEmail {
				m.emailInput.Focus()
				m.domainInput.Blur()
			} else {
				m.domainInput.Focus()
				m.emailInput.Blur()
			}
			return m, nil

		case "enter":
			// Primary action: Full provision
			m.isProcessing = true
			m.statusMessage = "Provisioning SSL certificate and configuring Nginx..."
			m.setLogContent("▶ Requesting certificate from Let's Encrypt and updating Nginx...")
			return m, m.RunCertbot(false)
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

func (m *Model) setLogContent(text string) {
	w := m.logViewport.Width - 2
	if w < 20 {
		w = 20
	}
	m.logViewport.SetContent(lipgloss.NewStyle().Width(w).Render(text))
}

func (m *Model) updateLayout() {
	inputWidth := (m.width - 24) / 2
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.domainInput.Width = inputWidth
	m.emailInput.Width = inputWidth

	vpWidth := m.width - 8
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 8
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

	domPrefix := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  Domain ")
	if !m.focusEmail {
		domPrefix = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Domain ")
	}

	mailPrefix := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  Email  ")
	if m.focusEmail {
		mailPrefix = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Email  ")
	}

	inputRow := lipgloss.JoinHorizontal(lipgloss.Center,
		domPrefix,
		m.domainInput.View(),
		"   ",
		mailPrefix,
		m.emailInput.View(),
	)

	var dnsBadge string
	if len(m.dnsIPs) > 0 {
		dnsBadge = lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("● DNS: %s", strings.Join(m.dnsIPs, ", ")))
	} else if m.dnsChecked {
		dnsBadge = lipgloss.NewStyle().Foreground(theme.ColorWarning).Render("○ DNS: Unresolved/Local")
	}

	statusLine := ""
	if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("  " + m.statusMessage)
	}

	headerLine := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Certbot SSL Provisioner"),
		"   ",
		dnsBadge,
	)

	elements := []string{
		headerLine,
		"",
		inputRow,
	}
	if statusLine != "" {
		elements = append(elements, statusLine)
	}
	elements = append(elements, "", lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("Execution Stream"), m.logViewport.View())

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, elements...))

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
