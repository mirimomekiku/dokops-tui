package dns

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	mdns "github.com/miekg/dns"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type DNSRecord struct {
	Name  string
	Type  string
	Class string
	TTL   uint32
	Value string
}

type DNSResultMsg struct {
	Records    []DNSRecord
	RTT        time.Duration
	Server     string
	StatusCode string
	Err        error
}

type FocusField int

const (
	FocusDomain FocusField = iota
	FocusRecordType
	FocusNameserver
	FocusCustomServer
	FocusResults
)

type RecordTypeOption struct {
	Name  string
	QType uint16
}

var RecordTypes = []RecordTypeOption{
	{Name: "A", QType: mdns.TypeA},
	{Name: "AAAA", QType: mdns.TypeAAAA},
	{Name: "CNAME", QType: mdns.TypeCNAME},
	{Name: "MX", QType: mdns.TypeMX},
	{Name: "TXT", QType: mdns.TypeTXT},
	{Name: "NS", QType: mdns.TypeNS},
	{Name: "SOA", QType: mdns.TypeSOA},
	{Name: "SRV", QType: mdns.TypeSRV},
	{Name: "PTR", QType: mdns.TypePTR},
}

type NameserverOption struct {
	Name string
	IP   string
}

var Nameservers = []NameserverOption{
	{Name: "Cloudflare", IP: "1.1.1.1"},
	{Name: "Google", IP: "8.8.8.8"},
	{Name: "Quad9", IP: "9.9.9.9"},
	{Name: "Local (System)", IP: "127.0.0.53"},
	{Name: "Custom", IP: "custom"},
}

type Model struct {
	domainInput       textinput.Model
	customServerInput textinput.Model
	recordTypeIdx     int
	nameserverIdx     int
	focus             FocusField
	records           []DNSRecord
	table             table.Model
	actionMenu        actionmenu.Model
	lastRTT           time.Duration
	lastStatus        string
	lastServer        string
	isLoading         bool
	width             int
	height            int
	err               error
}

func New() Model {
	di := textinput.New()
	di.Placeholder = "example.com"
	di.SetValue("github.com")
	di.Focus()
	di.CharLimit = 255
	di.Width = 35

	csi := textinput.New()
	csi.Placeholder = "10.0.0.1"
	csi.CharLimit = 50
	csi.Width = 20

	cols := []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "TYPE", Width: 8},
		{Title: "CLASS", Width: 8},
		{Title: "TTL", Width: 10},
		{Title: "DATA / TARGET / VALUE", Width: 45},
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
		domainInput:       di,
		customServerInput: csi,
		recordTypeIdx:     0,
		nameserverIdx:     0,
		focus:             FocusDomain,
		table:             t,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.ExecuteQuery(),
	)
}

func (m Model) getActiveServer() string {
	if m.nameserverIdx < len(Nameservers) {
		opt := Nameservers[m.nameserverIdx]
		if opt.IP == "custom" {
			custom := strings.TrimSpace(m.customServerInput.Value())
			if custom == "" {
				return "1.1.1.1"
			}
			return custom
		}
		return opt.IP
	}
	return "1.1.1.1"
}

func (m Model) ExecuteQuery() tea.Cmd {
	domain := strings.TrimSpace(m.domainInput.Value())
	if domain == "" {
		domain = "github.com"
	}
	server := m.getActiveServer()
	qType := RecordTypes[m.recordTypeIdx].QType

	return func() tea.Msg {
		msg := new(mdns.Msg)
		msg.SetQuestion(mdns.Fqdn(domain), qType)
		msg.RecursionDesired = true

		c := new(mdns.Client)
		c.Timeout = 3 * time.Second

		serverAddr := server
		if !strings.Contains(serverAddr, ":") {
			serverAddr += ":53"
		}

		r, rtt, err := c.Exchange(msg, serverAddr)
		if err != nil {
			return DNSResultMsg{
				Server: server,
				Err:    err,
			}
		}

		statusStr := mdns.RcodeToString[r.Rcode]
		records := parseRecords(r.Answer)

		return DNSResultMsg{
			Records:    records,
			RTT:        rtt,
			Server:     server,
			StatusCode: statusStr,
		}
	}
}

func parseRecords(answers []mdns.RR) []DNSRecord {
	var records []DNSRecord
	for _, rr := range answers {
		header := rr.Header()
		typeStr := mdns.TypeToString[header.Rrtype]
		classStr := mdns.ClassToString[header.Class]

		var val string
		switch v := rr.(type) {
		case *mdns.A:
			val = v.A.String()
		case *mdns.AAAA:
			val = v.AAAA.String()
		case *mdns.CNAME:
			val = v.Target
		case *mdns.MX:
			val = fmt.Sprintf("%d %s", v.Preference, v.Mx)
		case *mdns.TXT:
			val = strings.Join(v.Txt, " ")
		case *mdns.NS:
			val = v.Ns
		case *mdns.SOA:
			val = fmt.Sprintf("%s %s (Serial %d, Expire %d)", v.Ns, v.Mbox, v.Serial, v.Expire)
		case *mdns.SRV:
			val = fmt.Sprintf("Priority %d Weight %d Port %d -> %s", v.Priority, v.Weight, v.Port, v.Target)
		case *mdns.PTR:
			val = v.Ptr
		default:
			val = rr.String()
		}

		records = append(records, DNSRecord{
			Name:  strings.TrimSuffix(header.Name, "."),
			Type:  typeStr,
			Class: classStr,
			TTL:   header.Ttl,
			Value: val,
		})
	}
	return records
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case DNSResultMsg:
		m.isLoading = false
		m.err = msg.Err
		m.records = msg.Records
		m.lastRTT = msg.RTT
		m.lastServer = msg.Server
		m.lastStatus = msg.StatusCode
		m.updateTableRows()

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				switch action {
				case "query":
					m.isLoading = true
					return m, m.ExecuteQuery()
				case "cycle_type":
					m.recordTypeIdx = (m.recordTypeIdx + 1) % len(RecordTypes)
				case "cycle_server":
					m.nameserverIdx = (m.nameserverIdx + 1) % len(Nameservers)
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "tab":
			m.cycleFocus(true)
			return m, nil
		case "shift+tab":
			m.cycleFocus(false)
			return m, nil
		case "space":
			if m.focus != FocusDomain && m.focus != FocusCustomServer {
				title := "DNS Inspector"
				subtitle := fmt.Sprintf("Domain: %s | Type: %s | Server: %s", m.domainInput.Value(), RecordTypes[m.recordTypeIdx].Name, Nameservers[m.nameserverIdx].Name)
				items := []actionmenu.Item{
					{Key: "query", Title: "Execute DNS Query", Description: "Query nameserver for records"},
					{Key: "cycle_type", Title: "Cycle Record Type", Description: fmt.Sprintf("Current: %s", RecordTypes[m.recordTypeIdx].Name)},
					{Key: "cycle_server", Title: "Cycle Nameserver", Description: fmt.Sprintf("Current: %s", Nameservers[m.nameserverIdx].Name)},
				}
				m.actionMenu.Open(title, subtitle, items)
				return m, nil
			}
		}

		if m.focus == FocusRecordType {
			switch msg.String() {
			case "left", "h", "k", "up":
				if m.recordTypeIdx > 0 {
					m.recordTypeIdx--
				}
				return m, nil
			case "right", "l", "j", "down":
				if m.recordTypeIdx < len(RecordTypes)-1 {
					m.recordTypeIdx++
				}
				return m, nil
			case "enter":
				m.isLoading = true
				return m, m.ExecuteQuery()
			}
		}

		if m.focus == FocusNameserver {
			switch msg.String() {
			case "left", "h", "k", "up":
				if m.nameserverIdx > 0 {
					m.nameserverIdx--
				}
				return m, nil
			case "right", "l", "j", "down":
				if m.nameserverIdx < len(Nameservers)-1 {
					m.nameserverIdx++
				}
				return m, nil
			case "enter":
				m.isLoading = true
				return m, m.ExecuteQuery()
			}
		}

		if m.focus == FocusDomain {
			if msg.String() == "enter" {
				m.isLoading = true
				return m, m.ExecuteQuery()
			}
			var cmd tea.Cmd
			m.domainInput, cmd = m.domainInput.Update(msg)
			return m, cmd
		}

		if m.focus == FocusCustomServer {
			if msg.String() == "enter" {
				m.isLoading = true
				return m, m.ExecuteQuery()
			}
			var cmd tea.Cmd
			m.customServerInput, cmd = m.customServerInput.Update(msg)
			return m, cmd
		}

		if m.focus == FocusResults {
			if msg.String() == "enter" {
				m.isLoading = true
				return m, m.ExecuteQuery()
			}
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) cycleFocus(forward bool) {
	maxFocus := 4
	if Nameservers[m.nameserverIdx].IP == "custom" {
		maxFocus = 5
	}

	if forward {
		m.focus = FocusField((int(m.focus) + 1) % maxFocus)
	} else {
		m.focus = FocusField((int(m.focus) + maxFocus - 1) % maxFocus)
	}

	m.domainInput.Blur()
	m.customServerInput.Blur()

	switch m.focus {
	case FocusDomain:
		m.domainInput.Focus()
	case FocusCustomServer:
		m.customServerInput.Focus()
	}
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, r := range m.records {
		rows = append(rows, table.Row{
			r.Name,
			r.Type,
			r.Class,
			fmt.Sprintf("%d s", r.TTL),
			r.Value,
		})
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	tableHeight := m.height - 14
	if tableHeight < 6 {
		tableHeight = 6
	}
	m.table.SetHeight(tableHeight)

	availableWidth := m.width - 6
	if availableWidth > 60 {
		valWidth := availableWidth - (25 + 8 + 8 + 10 + 10)
		if valWidth < 20 {
			valWidth = 20
		}
		cols := []table.Column{
			{Title: "NAME", Width: 25},
			{Title: "TYPE", Width: 8},
			{Title: "CLASS", Width: 8},
			{Title: "TTL", Width: 10},
			{Title: "DATA / TARGET / VALUE", Width: valWidth},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// 1. Controls Bar: Record Types
	var typePills []string
	for i, rt := range RecordTypes {
		if i == m.recordTypeIdx {
			style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(theme.ColorPrimary).Padding(0, 1)
			typePills = append(typePills, style.Render(rt.Name))
		} else {
			style := lipgloss.NewStyle().Foreground(theme.ColorMuted).Background(theme.ColorSurfaceAlt).Padding(0, 1)
			typePills = append(typePills, style.Render(rt.Name))
		}
	}
	recordTypeRow := lipgloss.JoinHorizontal(lipgloss.Left, typePills...)

	// 2. Nameservers Bar
	var serverPills []string
	for i, ns := range Nameservers {
		label := ns.Name
		if ns.IP != "custom" {
			label = fmt.Sprintf("%s (%s)", ns.Name, ns.IP)
		}
		if i == m.nameserverIdx {
			style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(theme.ColorSecondary).Padding(0, 1)
			serverPills = append(serverPills, style.Render(label))
		} else {
			style := lipgloss.NewStyle().Foreground(theme.ColorMuted).Background(theme.ColorSurfaceAlt).Padding(0, 1)
			serverPills = append(serverPills, style.Render(label))
		}
	}
	nameserverRow := lipgloss.JoinHorizontal(lipgloss.Left, serverPills...)

	// Inputs
	domainPrefix := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  Domain ")
	if m.focus == FocusDomain {
		domainPrefix = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Domain ")
	}
	domainRow := lipgloss.JoinHorizontal(lipgloss.Center,
		domainPrefix,
		m.domainInput.View(),
	)

	if Nameservers[m.nameserverIdx].IP == "custom" {
		csPrefix := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("  Server ")
		if m.focus == FocusCustomServer {
			csPrefix = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render("▶ Server ")
		}
		domainRow = lipgloss.JoinHorizontal(lipgloss.Center,
			domainRow,
			"   ",
			csPrefix,
			m.customServerInput.View(),
		)
	}

	// Status & RTT banner
	var statusLine string
	if m.isLoading {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("Querying nameserver...")
	} else if m.err != nil {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorDanger).Render(fmt.Sprintf("Error: %v", m.err))
	} else {
		statusLine = lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(fmt.Sprintf("● %s", m.lastStatus)),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(fmt.Sprintf("RTT: %s", theme.FormatDuration(m.lastRTT))),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("via %s (%d answers)", m.lastServer, len(m.records))),
		)
	}

	topControls := lipgloss.JoinVertical(lipgloss.Left,
		domainRow,
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Width(12).Bold(true).Foreground(theme.ColorPrimary).Render("Type:"),
			recordTypeRow,
		),
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Width(12).Bold(true).Foreground(theme.ColorSecondary).Render("Server:"),
			nameserverRow,
		),
	)

	elements := []string{
		topControls,
	}
	if statusLine != "" {
		elements = append(elements, "", statusLine)
	}
	elements = append(elements, "", m.table.View())

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, elements...))

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
