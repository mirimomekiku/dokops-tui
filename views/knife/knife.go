package knife

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/golang-jwt/jwt/v5"
	"github.com/robfig/cron/v3"
	"golang.org/x/crypto/bcrypt"

	"dok-ops/internal/theme"
)

type KnifeTab int

const (
	TabJWT KnifeTab = iota
	TabCron
	TabBase64
	TabHash
)

var KnifeTabNames = []string{
	"1: JWT Inspector",
	"2: Cron Evaluator",
	"3: Base64 / URL / Hex",
	"4: Hash & Bcrypt",
}

type Model struct {
	activeTab KnifeTab
	jwtInput  textinput.Model
	cronInput textinput.Model
	b64Input  textinput.Model
	hashInput textinput.Model
	viewport  viewport.Model
	width     int
	height    int
}

func New() Model {
	ji := textinput.New()
	ji.Placeholder = "Paste JWT token to decode (e.g. eyJ...)"
	ji.Focus()
	ji.CharLimit = 4000
	ji.Width = 60

	ci := textinput.New()
	ci.Placeholder = "e.g. */15 * * * * or 0 3 * * 1-5"
	ci.SetValue("*/15 * * * *")
	ci.CharLimit = 100
	ci.Width = 40

	bi := textinput.New()
	bi.Placeholder = "Enter text to encode/decode..."
	bi.SetValue("")
	bi.CharLimit = 2000
	bi.Width = 50

	hi := textinput.New()
	hi.Placeholder = "Enter string to compute hashes..."
	hi.SetValue("")
	hi.CharLimit = 2000
	hi.Width = 50

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1)

	m := Model{
		activeTab: TabJWT,
		jwtInput:  ji,
		cronInput: ci,
		b64Input:  bi,
		hashInput: hi,
		viewport:  vp,
	}
	m.updateViewportContent()
	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		switch msg.String() {
		case "f1", "1":
			m.activeTab = TabJWT
			m.jwtInput.Focus()
			m.cronInput.Blur()
			m.b64Input.Blur()
			m.hashInput.Blur()
			m.updateViewportContent()
			return m, nil
		case "f2", "2":
			m.activeTab = TabCron
			m.jwtInput.Blur()
			m.cronInput.Focus()
			m.b64Input.Blur()
			m.hashInput.Blur()
			m.updateViewportContent()
			return m, nil
		case "f3", "3":
			m.activeTab = TabBase64
			m.jwtInput.Blur()
			m.cronInput.Blur()
			m.b64Input.Focus()
			m.hashInput.Blur()
			m.updateViewportContent()
			return m, nil
		case "f4", "4":
			m.activeTab = TabHash
			m.jwtInput.Blur()
			m.cronInput.Blur()
			m.b64Input.Blur()
			m.hashInput.Focus()
			m.updateViewportContent()
			return m, nil
		}

		var cmd tea.Cmd
		switch m.activeTab {
		case TabJWT:
			m.jwtInput, cmd = m.jwtInput.Update(msg)
			cmds = append(cmds, cmd)
		case TabCron:
			m.cronInput, cmd = m.cronInput.Update(msg)
			cmds = append(cmds, cmd)
		case TabBase64:
			m.b64Input, cmd = m.b64Input.Update(msg)
			cmds = append(cmds, cmd)
		case TabHash:
			m.hashInput, cmd = m.hashInput.Update(msg)
			cmds = append(cmds, cmd)
		}
		m.updateViewportContent()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateLayout() {
	inputWidth := m.width - 20
	if inputWidth < 30 {
		inputWidth = 30
	}
	m.jwtInput.Width = inputWidth
	m.cronInput.Width = inputWidth
	m.b64Input.Width = inputWidth
	m.hashInput.Width = inputWidth

	vpWidth := m.width - 6
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := m.height - 12
	if vpHeight < 8 {
		vpHeight = 8
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
}

func (m *Model) updateViewportContent() {
	var sb strings.Builder

	switch m.activeTab {
	case TabJWT:
		tokenStr := strings.TrimSpace(m.jwtInput.Value())
		if tokenStr == "" {
			sb.WriteString("Paste a JWT token above to inspect header and claims.")
		} else {
			parts := strings.Split(tokenStr, ".")
			if len(parts) < 2 {
				sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorDanger).Render("Invalid JWT format (expected header.payload.signature)"))
			} else {
				// Parse unverified to inspect claims
				parser := jwt.NewParser()
				token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})

				headerBytes, _ := json.MarshalIndent(token.Header, "", "  ")
				claimsBytes, _ := json.MarshalIndent(token.Claims, "", "  ")

				sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── HEADER (Algorithm & Token Type) ──") + "\n")
				sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(string(headerBytes)) + "\n\n")

				sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("── PAYLOAD CLAIMS ──") + "\n")
				sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorText).Render(string(claimsBytes)) + "\n\n")

				// Expiration analysis
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("── VALIDITY & EXPIRATION ──") + "\n")
					if exp, ok := claims["exp"].(float64); ok {
						expTime := time.Unix(int64(exp), 0)
						now := time.Now()
						if now.After(expTime) {
							sb.WriteString(theme.BadgeDanger.Render(fmt.Sprintf(" EXPIRED: %s (%s ago) ", expTime.Format(time.RFC1123), now.Sub(expTime).Round(time.Second))))
						} else {
							sb.WriteString(theme.BadgeSuccess.Render(fmt.Sprintf(" ACTIVE: Valid until %s (in %s) ", expTime.Format(time.RFC1123), time.Until(expTime).Round(time.Second))))
						}
					} else {
						sb.WriteString(theme.BadgeWarning.Render(" No 'exp' claim found (Token does not expire) "))
					}
					sb.WriteString("\n")
				}

				if err != nil {
					sb.WriteString(fmt.Sprintf("\nParser note: %v\n", err))
				}
			}
		}

	case TabCron:
		cronStr := strings.TrimSpace(m.cronInput.Value())
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── CRON SCHEDULE EVALUATOR ──") + "\n\n")

		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		sched, err := parser.Parse(cronStr)
		if err != nil {
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorDanger).Render(fmt.Sprintf("Invalid Cron Expression: %v\n\nExample syntax: */15 * * * * (every 15 min), 0 0 * * * (daily at midnight)", err)))
		} else {
			sb.WriteString(theme.BadgeSuccess.Render(" VALID CRON EXPRESSION ") + "\n\n")
			sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorInfo).Render("Next 5 Calculated Execution Times:\n"))

			curr := time.Now()
			for i := 1; i <= 5; i++ {
				curr = sched.Next(curr)
				sb.WriteString(fmt.Sprintf("  %d. %s  (%s from now)\n", i, curr.Format("Monday, 02 Jan 2006 15:04:05 MST"), time.Until(curr).Round(time.Second)))
			}
		}

	case TabBase64:
		raw := m.b64Input.Value()
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── ENCODINGS & DECODINGS ──") + "\n\n")

		// Standard Base64 Encode
		b64Enc := base64.StdEncoding.EncodeToString([]byte(raw))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorInfo).Bold(true).Render("Base64 Encoded: ") + b64Enc + "\n\n")

		// Base64 Decode
		if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorSuccess).Bold(true).Render("Base64 Decoded: ") + string(decoded) + "\n\n")
		}

		// URL Encode
		urlEnc := url.QueryEscape(raw)
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorSecondary).Bold(true).Render("URL Encoded: ") + urlEnc + "\n\n")

		// URL Decode
		if urlDec, err := url.QueryUnescape(raw); err == nil && urlDec != raw {
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorWarning).Bold(true).Render("URL Decoded: ") + urlDec + "\n\n")
		}

		// Hex Encode
		hexEnc := hex.EncodeToString([]byte(raw))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true).Render("Hex Encoded: ") + hexEnc + "\n\n")

		// Hex Decode
		if hexDec, err := hex.DecodeString(strings.ReplaceAll(raw, " ", "")); err == nil && len(hexDec) > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorSuccess).Bold(true).Render("Hex Decoded: ") + string(hexDec) + "\n\n")
		}

	case TabHash:
		raw := m.hashInput.Value()
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── CRYPTOGRAPHIC HASHES & BCRYPT ──") + "\n\n")

		// MD5
		md5Hash := md5.Sum([]byte(raw))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorInfo).Bold(true).Render("MD5:     ") + hex.EncodeToString(md5Hash[:]) + "\n")

		// SHA-1
		sha1Hash := sha1.Sum([]byte(raw))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorSecondary).Bold(true).Render("SHA-1:   ") + hex.EncodeToString(sha1Hash[:]) + "\n")

		// SHA-256
		sha256Hash := sha256.Sum256([]byte(raw))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorSuccess).Bold(true).Render("SHA-256: ") + hex.EncodeToString(sha256Hash[:]) + "\n")

		// SHA-512
		sha512Hash := sha512.Sum512([]byte(raw))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorWarning).Bold(true).Render("SHA-512: ") + hex.EncodeToString(sha512Hash[:]) + "\n\n")

		// Bcrypt
		if len(raw) > 0 && len(raw) <= 72 {
			if bHash, err := bcrypt.GenerateFromPassword([]byte(raw), 10); err == nil {
				sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true).Render("Bcrypt (cost 10): ") + string(bHash) + "\n")
			}
		}
	}

	m.viewport.SetContent(sb.String())
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Sub-tabs
	var pills []string
	for i, name := range KnifeTabNames {
		if KnifeTab(i) == m.activeTab {
			pills = append(pills, theme.ActiveTabStyle.Render(name))
		} else {
			pills = append(pills, theme.InactiveTabStyle.Render(name))
		}
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Left, pills...)

	// Input Bar
	var inputView string
	var inputLabel string
	switch m.activeTab {
	case TabJWT:
		inputLabel = "JWT Token:"
		inputView = m.jwtInput.View()
	case TabCron:
		inputLabel = "Cron Expression:"
		inputView = m.cronInput.View()
	case TabBase64:
		inputLabel = "Input Text / Secret:"
		inputView = m.b64Input.View()
	case TabHash:
		inputLabel = "Input String:"
		inputView = m.hashInput.View()
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render(inputLabel+" "),
				inputView,
			),
		)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🔪 DEVOPS SWISS ARMY KNIFE (CyberChef)"),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[1-4: Switch Tool | Type in input to update]"),
		),
		"",
		tabRow,
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
