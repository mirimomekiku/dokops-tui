package httpclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type LatencyTrace struct {
	DNSLookup    time.Duration
	TCPConnect   time.Duration
	TLSHandshake time.Duration
	ServerTTFB   time.Duration
	TransferTime time.Duration
	TotalTime    time.Duration
}

type ResponseResultMsg struct {
	StatusCode int
	StatusText string
	Proto      string
	Headers    http.Header
	Body       []byte
	Trace      LatencyTrace
	Err        error
}

type FocusArea int

const (
	FocusURL FocusArea = iota
	FocusMethod
	FocusBody
	FocusResponse
)

var HTTPMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"}

type Model struct {
	urlInput     textinput.Model
	bodyInput    textinput.Model
	methodIdx    int
	focus        FocusArea
	viewport     viewport.Model
	lastResponse *ResponseResultMsg
	isLoading    bool
	width        int
	height       int
	err          error
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "https://api.github.com/zen"
	ti.SetValue("https://httpbin.org/get")
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 45

	bi := textinput.New()
	bi.Placeholder = `{"key": "value"}`
	bi.CharLimit = 2000
	bi.Width = 45

	vp := viewport.New(40, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1)
	vp.SetContent("Send a request to inspect response headers, JSON payload, and latency waterfall.")

	return Model{
		urlInput:  ti,
		bodyInput: bi,
		methodIdx: 0,
		focus:     FocusURL,
		viewport:  vp,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func ExecuteRequest(method, targetURL, bodyStr string) tea.Cmd {
	return func() tea.Msg {
		if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
			targetURL = "https://" + targetURL
		}

		var bodyReader io.Reader
		if bodyStr != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
			bodyReader = bytes.NewBufferString(bodyStr)
		}

		req, err := http.NewRequest(method, targetURL, bodyReader)
		if err != nil {
			return ResponseResultMsg{Err: err}
		}

		if bodyStr != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", "dok-ops-httpclient/1.0")

		var start, dnsStart, connStart, tlsStart, respStart, bodyStart time.Time
		var trace LatencyTrace

		clientTrace := &httptrace.ClientTrace{
			DNSStart: func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone: func(_ httptrace.DNSDoneInfo) {
				if !dnsStart.IsZero() {
					trace.DNSLookup = time.Since(dnsStart)
				}
			},
			ConnectStart: func(_, _ string) { connStart = time.Now() },
			ConnectDone: func(_, _ string, _ error) {
				if !connStart.IsZero() {
					trace.TCPConnect = time.Since(connStart)
				}
			},
			TLSHandshakeStart: func() { tlsStart = time.Now() },
			TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
				if !tlsStart.IsZero() {
					trace.TLSHandshake = time.Since(tlsStart)
				}
			},
			GotFirstResponseByte: func() {
				if !respStart.IsZero() {
					trace.ServerTTFB = time.Since(respStart)
				}
				bodyStart = time.Now()
			},
		}

		req = req.WithContext(httptrace.WithClientTrace(req.Context(), clientTrace))

		client := &http.Client{
			Timeout: 15 * time.Second,
		}

		start = time.Now()
		respStart = time.Now()
		resp, err := client.Do(req)
		if err != nil {
			trace.TotalTime = time.Since(start)
			return ResponseResultMsg{Trace: trace, Err: err}
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if !bodyStart.IsZero() {
			trace.TransferTime = time.Since(bodyStart)
		}
		trace.TotalTime = time.Since(start)

		return ResponseResultMsg{
			StatusCode: resp.StatusCode,
			StatusText: resp.Status,
			Proto:      resp.Proto,
			Headers:    resp.Header,
			Body:       bodyBytes,
			Trace:      trace,
			Err:        err,
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

	case ResponseResultMsg:
		m.isLoading = false
		m.lastResponse = &msg
		m.updateResponseViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.cycleFocus(true)
			return m, nil
		case "shift+tab":
			m.cycleFocus(false)
			return m, nil
		}

		if m.focus == FocusMethod {
			switch msg.String() {
			case "left", "h":
				if m.methodIdx > 0 {
					m.methodIdx--
				}
				return m, nil
			case "right", "l":
				if m.methodIdx < len(HTTPMethods)-1 {
					m.methodIdx++
				}
				return m, nil
			case "enter":
				m.focus = FocusURL
				m.urlInput.Focus()
				return m, textinput.Blink
			}
		}

		if m.focus == FocusURL {
			if msg.String() == "enter" {
				m.isLoading = true
				return m, ExecuteRequest(HTTPMethods[m.methodIdx], m.urlInput.Value(), m.bodyInput.Value())
			}
			var cmd tea.Cmd
			m.urlInput, cmd = m.urlInput.Update(msg)
			return m, cmd
		}

		if m.focus == FocusBody {
			if msg.String() == "enter" {
				m.isLoading = true
				return m, ExecuteRequest(HTTPMethods[m.methodIdx], m.urlInput.Value(), m.bodyInput.Value())
			}
			var cmd tea.Cmd
			m.bodyInput, cmd = m.bodyInput.Update(msg)
			return m, cmd
		}

		if m.focus == FocusResponse {
			if msg.String() == "r" || msg.String() == "enter" {
				m.isLoading = true
				return m, ExecuteRequest(HTTPMethods[m.methodIdx], m.urlInput.Value(), m.bodyInput.Value())
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) cycleFocus(forward bool) {
	if forward {
		m.focus = (m.focus + 1) % 4
	} else {
		m.focus = (m.focus + 3) % 4
	}

	m.urlInput.Blur()
	m.bodyInput.Blur()

	switch m.focus {
	case FocusURL:
		m.urlInput.Focus()
	case FocusBody:
		m.bodyInput.Focus()
	}
}

func (m *Model) updateLayout() {
	leftWidth := (m.width - 8) / 2
	if leftWidth < 38 {
		leftWidth = 38
	}
	m.urlInput.Width = leftWidth - 10
	m.bodyInput.Width = leftWidth - 10

	rightWidth := m.width - leftWidth - 8
	if rightWidth < 40 {
		rightWidth = 40
	}
	vpHeight := m.height - 8
	if vpHeight < 12 {
		vpHeight = 12
	}
	m.viewport.Width = rightWidth
	m.viewport.Height = vpHeight
}

func (m *Model) updateResponseViewport() {
	if m.lastResponse == nil {
		return
	}
	if m.lastResponse.Err != nil {
		content := lipgloss.NewStyle().Foreground(theme.ColorDanger).Bold(true).Render(
			fmt.Sprintf("Request Error:\n%v", m.lastResponse.Err),
		)
		m.viewport.SetContent(content)
		return
	}

	var sb strings.Builder

	// Status Line
	badge := theme.BadgeSuccess.Render(fmt.Sprintf(" %s %d %s ", m.lastResponse.Proto, m.lastResponse.StatusCode, m.lastResponse.StatusText))
	if m.lastResponse.StatusCode >= 400 {
		badge = theme.BadgeDanger.Render(fmt.Sprintf(" %s %d %s ", m.lastResponse.Proto, m.lastResponse.StatusCode, m.lastResponse.StatusText))
	}
	sb.WriteString(badge + "\n\n")

	// Headers
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── RESPONSE HEADERS ──") + "\n")
	for k, v := range m.lastResponse.Headers {
		headerKey := lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(k)
		headerVal := lipgloss.NewStyle().Foreground(theme.ColorText).Render(strings.Join(v, ", "))
		sb.WriteString(fmt.Sprintf("%s: %s\n", headerKey, headerVal))
	}
	sb.WriteString("\n")

	// Body
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("── RESPONSE BODY ──") + "\n")
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, m.lastResponse.Body, "", "  "); err == nil {
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorText).Render(prettyJSON.String()))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorText).Render(string(m.lastResponse.Body)))
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoTop()
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	leftWidth := (contentWidth - 4) / 2
	if leftWidth < 38 {
		leftWidth = 38
	}
	rightWidth := contentWidth - leftWidth - 4
	if rightWidth < 38 {
		rightWidth = 38
	}

	// 1. Method Selector
	var methodPills []string
	for i, meth := range HTTPMethods {
		if i == m.methodIdx {
			style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(theme.ColorPrimary).Padding(0, 1)
			methodPills = append(methodPills, style.Render(meth))
		} else {
			style := lipgloss.NewStyle().Foreground(theme.ColorMuted).Background(theme.ColorSurfaceAlt).Padding(0, 1)
			methodPills = append(methodPills, style.Render(meth))
		}
	}
	methodSection := lipgloss.JoinHorizontal(lipgloss.Left, methodPills...)

	// Focus Highlights
	urlBorderColor := theme.ColorBorder
	if m.focus == FocusURL {
		urlBorderColor = theme.ColorPrimary
	}
	bodyBorderColor := theme.ColorBorder
	if m.focus == FocusBody {
		bodyBorderColor = theme.ColorPrimary
	}

	urlBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(urlBorderColor).
		Padding(0, 1).
		Width(leftWidth - 4).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Target URL:"),
				m.urlInput.View(),
			),
		)

	bodyBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bodyBorderColor).
		Padding(0, 1).
		Width(leftWidth - 4).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Payload Body (JSON):"),
				m.bodyInput.View(),
			),
		)

	// Timing Waterfall
	var waterfallLines []string
	waterfallLines = append(waterfallLines, lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("⚡ NETWORK LATENCY WATERFALL"))

	if m.isLoading {
		waterfallLines = append(waterfallLines, "", lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("⏳ Executing HTTP request & tracing latencies..."))
	} else if m.lastResponse != nil {
		t := m.lastResponse.Trace
		totalMs := float64(t.TotalTime.Microseconds()) / 1000.0
		if totalMs <= 0 {
			totalMs = 1.0
		}

		renderTimingBar := func(label string, d time.Duration, col lipgloss.Color) string {
			ms := float64(d.Microseconds()) / 1000.0
			pct := (ms / totalMs) * 100.0
			bar := theme.RenderProgressBar(16, pct, col, theme.ColorSurfaceAlt)
			lbl := lipgloss.NewStyle().Width(14).Foreground(theme.ColorText).Render(label)
			durStr := lipgloss.NewStyle().Width(10).Bold(true).Foreground(col).Render(theme.FormatDuration(d))
			return fmt.Sprintf("%s %s %s", lbl, bar, durStr)
		}

		waterfallLines = append(waterfallLines,
			"",
			renderTimingBar("DNS Lookup", t.DNSLookup, theme.ColorInfo),
			renderTimingBar("TCP Connect", t.TCPConnect, theme.ColorWarning),
			renderTimingBar("TLS Handshake", t.TLSHandshake, theme.ColorSecondary),
			renderTimingBar("Server TTFB", t.ServerTTFB, theme.ColorSuccess),
			renderTimingBar("Transfer Time", t.TransferTime, theme.ColorHighlight),
			strings.Repeat("─", leftWidth-8),
			renderTimingBar("Total Duration", t.TotalTime, theme.ColorPrimary),
		)
	} else {
		waterfallLines = append(waterfallLines, "", lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("No request executed yet. Press Enter to send."))
	}

	waterfallBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(leftWidth - 4).
		Render(lipgloss.JoinVertical(lipgloss.Left, waterfallLines...))

	leftPane := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, theme.CardTitleStyle.Render("🌐 HTTP CLIENT"), "  ", methodSection),
		urlBox,
		bodyBox,
		waterfallBox,
	)

	// Right Pane: Response Viewer
	rightBorderColor := theme.ColorBorder
	if m.focus == FocusResponse {
		rightBorderColor = theme.ColorPrimary
	}
	m.viewport.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rightBorderColor).
		Padding(0, 1)

	rightPane := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("📥 RESPONSE INSPECTION"),
			"  ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Tab: Focus | j/k: Scroll | Enter: Send]"),
		),
		m.viewport.View(),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).Render(leftPane),
		"  ",
		lipgloss.NewStyle().Width(rightWidth).Render(rightPane),
	)
}
