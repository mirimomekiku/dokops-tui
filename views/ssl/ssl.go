package ssl

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type CertInfo struct {
	SubjectCN      string
	SANs           []string
	IssuerCN       string
	IssuerOrg      string
	NotBefore      time.Time
	NotAfter       time.Time
	DaysRemaining  int
	LifetimePct    float64
	IsExpired      bool
	SerialNumber   string
	SignatureAlgo  string
	TLSVersion     string
	CipherSuite    string
	CertChainNames []string
}

type InspectResultMsg struct {
	Target string
	Info   *CertInfo
	Err    error
}

type Model struct {
	targetInput textinput.Model
	viewport    viewport.Model
	lastInfo    *CertInfo
	lastTarget  string
	isLoading   bool
	width       int
	height      int
	err         error
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "domain:443 or /path/to/cert.pem"
	ti.SetValue("github.com:443")
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 45

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1)

	return Model{
		targetInput: ti,
		viewport:    vp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.InspectTarget(),
	)
}

func (m Model) InspectTarget() tea.Cmd {
	target := strings.TrimSpace(m.targetInput.Value())
	if target == "" {
		target = "github.com:443"
	}

	return func() tea.Msg {
		// Check if target is a local file
		if strings.HasSuffix(target, ".pem") || strings.HasSuffix(target, ".crt") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "./") {
			if _, err := os.Stat(target); err == nil {
				return inspectLocalCert(target)
			}
		}

		// Otherwise inspect remote TLS host
		host := target
		if !strings.Contains(host, ":") {
			host = host + ":443"
		}
		return inspectRemoteCert(host)
	}
}

func inspectRemoteCert(host string) InspectResultMsg {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	conf := &tls.Config{
		InsecureSkipVerify: true, // Allow inspecting expired/self-signed certs
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", host, conf)
	if err != nil {
		return InspectResultMsg{Target: host, Err: err}
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return InspectResultMsg{Target: host, Err: fmt.Errorf("no peer certificates presented")}
	}

	mainCert := state.PeerCertificates[0]
	var chain []string
	for _, c := range state.PeerCertificates {
		name := c.Subject.CommonName
		if name == "" && len(c.DNSNames) > 0 {
			name = c.DNSNames[0]
		}
		chain = append(chain, fmt.Sprintf("%s (Issuer: %s)", name, c.Issuer.CommonName))
	}

	tlsVer := tlsVersionString(state.Version)
	cipher := tls.CipherSuiteName(state.CipherSuite)

	info := parseCertificateData(mainCert, chain, tlsVer, cipher)
	return InspectResultMsg{Target: host, Info: info}
}

func inspectLocalCert(filePath string) InspectResultMsg {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return InspectResultMsg{Target: filePath, Err: err}
	}

	var certs []*x509.Certificate
	for len(data) > 0 {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err == nil {
				certs = append(certs, cert)
			}
		}
	}

	if len(certs) == 0 {
		return InspectResultMsg{Target: filePath, Err: fmt.Errorf("no valid X.509 certificates parsed from file")}
	}

	mainCert := certs[0]
	var chain []string
	for _, c := range certs {
		chain = append(chain, fmt.Sprintf("%s (Issuer: %s)", c.Subject.CommonName, c.Issuer.CommonName))
	}

	info := parseCertificateData(mainCert, chain, "Local File", "N/A")
	return InspectResultMsg{Target: filePath, Info: info}
}

func parseCertificateData(cert *x509.Certificate, chain []string, tlsVer, cipher string) *CertInfo {
	now := time.Now()
	totalLifetime := cert.NotAfter.Sub(cert.NotBefore).Hours()
	elapsed := now.Sub(cert.NotBefore).Hours()

	var pct float64
	if totalLifetime > 0 {
		pct = (elapsed / totalLifetime) * 100.0
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
	}

	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)
	isExpired := now.After(cert.NotAfter)

	issuerOrg := ""
	if len(cert.Issuer.Organization) > 0 {
		issuerOrg = cert.Issuer.Organization[0]
	}

	return &CertInfo{
		SubjectCN:      cert.Subject.CommonName,
		SANs:           cert.DNSNames,
		IssuerCN:       cert.Issuer.CommonName,
		IssuerOrg:      issuerOrg,
		NotBefore:      cert.NotBefore,
		NotAfter:       cert.NotAfter,
		DaysRemaining:  daysRemaining,
		LifetimePct:    pct,
		IsExpired:      isExpired,
		SerialNumber:   cert.SerialNumber.Text(16),
		SignatureAlgo:  cert.SignatureAlgorithm.String(),
		TLSVersion:     tlsVer,
		CipherSuite:    cipher,
		CertChainNames: chain,
	}
}

func tlsVersionString(ver uint16) string {
	switch ver {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", ver)
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case InspectResultMsg:
		m.isLoading = false
		m.err = msg.Err
		m.lastTarget = msg.Target
		m.lastInfo = msg.Info
		m.updateViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.isLoading = true
			return m, m.InspectTarget()
		}

		var cmd tea.Cmd
		m.targetInput, cmd = m.targetInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateLayout() {
	vpWidth := m.width - 6
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 10
	if vpHeight < 8 {
		vpHeight = 8
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
}

func (m *Model) updateViewport() {
	if m.err != nil {
		m.viewport.SetContent(lipgloss.NewStyle().Foreground(theme.ColorDanger).Bold(true).Render(
			fmt.Sprintf("Certificate Inspection Error for '%s':\n%v", m.lastTarget, m.err),
		))
		return
	}

	if m.lastInfo == nil {
		m.viewport.SetContent("Enter a domain:443 or cert file path to inspect.")
		return
	}

	info := m.lastInfo
	var sb strings.Builder

	// Status badge & expiration bar
	var expBadge string
	if info.IsExpired {
		expBadge = theme.BadgeDanger.Render(fmt.Sprintf(" EXPIRED (%s ago) ", time.Since(info.NotAfter).Round(time.Hour*24)))
	} else if info.DaysRemaining <= 14 {
		expBadge = theme.BadgeDanger.Render(fmt.Sprintf(" EXPIRING SOON: %d Days Remaining! ", info.DaysRemaining))
	} else if info.DaysRemaining <= 30 {
		expBadge = theme.BadgeWarning.Render(fmt.Sprintf(" EXPIRING: %d Days Remaining ", info.DaysRemaining))
	} else {
		expBadge = theme.BadgeSuccess.Render(fmt.Sprintf(" VALID: %d Days Remaining ", info.DaysRemaining))
	}

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgePrimary.Render(fmt.Sprintf(" %s ", m.lastTarget)),
		" ",
		expBadge,
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(fmt.Sprintf("[%s / %s]", info.TLSVersion, info.CipherSuite)),
	) + "\n\n")

	// Visual Lifetime Progress Bar
	barWidth := m.viewport.Width - 30
	if barWidth < 15 {
		barWidth = 15
	}
	bar := theme.RenderProgressBar(barWidth, info.LifetimePct, theme.ColorSuccess, theme.ColorSurfaceAlt)
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Certificate Lifetime Usage:") + "\n")
	sb.WriteString(fmt.Sprintf("[ %s ] %5.1f%% elapsed\n", bar, info.LifetimePct))
	sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("Valid from: %s  ──>  Valid until: %s", info.NotBefore.Format("2006-01-02 15:04 MST"), info.NotAfter.Format("2006-01-02 15:04 MST"))) + "\n\n")

	// Subject & Issuer Details
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── SUBJECT & ISSUER ──") + "\n")
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("Common Name (CN)"), info.SubjectCN))
	if len(info.SANs) > 0 {
		sb.WriteString(fmt.Sprintf("  • %s: %s\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("SANs / DNS Names"), strings.Join(info.SANs, ", ")))
	}
	sb.WriteString(fmt.Sprintf("  • %s: %s (%s)\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("Issuer"), info.IssuerCN, info.IssuerOrg))
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("Signature Algorithm"), info.SignatureAlgo))
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("Serial Number"), info.SerialNumber))
	sb.WriteString("\n")

	// Certificate Chain
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("── CERTIFICATE TRUST CHAIN ──") + "\n")
	for i, c := range info.CertChainNames {
		indent := strings.Repeat("  ", i)
		arrow := "└── "
		if i == 0 {
			arrow = "├── [Leaf] "
		} else if i == len(info.CertChainNames)-1 {
			arrow = "└── [Root] "
		} else {
			arrow = "├── [Intermediate] "
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", indent, lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(arrow), c))
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoTop()
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Target Host / File: "),
				m.targetInput.View(),
				"  ",
				lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Enter: Inspect]"),
			),
		)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🔒 SSL/TLS CERTIFICATE INSPECTOR (s_client)"),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Verify validity, Let's Encrypt renewals & SANs]"),
		),
		"",
		inputBox,
		"",
		m.viewport.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
