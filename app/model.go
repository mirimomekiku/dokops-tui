package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"

	"dok-ops/internal/commandpalette"
	"dok-ops/views/autonginx"
	"dok-ops/views/bandwidth"
	"dok-ops/views/certbot"
	"dok-ops/views/ci"
	"dok-ops/views/containers"
	"dok-ops/views/dashboard"
	"dok-ops/views/database"
	"dok-ops/views/deploy"
	"dok-ops/views/disk"
	"dok-ops/views/dns"
	"dok-ops/views/env"
	"dok-ops/views/git"
	"dok-ops/views/httpclient"
	"dok-ops/views/knife"
	"dok-ops/views/monitor"
	"dok-ops/views/nginx"
	"dok-ops/views/phpfpm"
	"dok-ops/views/ports"
	"dok-ops/views/scanner"
	"dok-ops/views/services"
	"dok-ops/views/ssh"
	"dok-ops/views/ssl"
	"dok-ops/views/terminal"
	"dok-ops/views/timers"
	"dok-ops/views/workers"
)

type Tab int

const (
	TabDashboard Tab = iota
	TabMonitor
	TabContainers
	TabServices
	TabPorts
	TabNginx
	TabAutoNginx
	TabDeploy
	TabPHPFPM
	TabWorkers
	TabCertbot
	TabKnife
	TabSSL
	TabDatabase
	TabBandwidth
	TabScanner
	TabGit
	TabCI
	TabSSH
	TabEnv
	TabTimers
	TabDisk
	TabHTTP
	TabDNS
	TabTerminal
)

type Workspace int

const (
	WorkspaceSystem Workspace = iota
	WorkspaceWebOps
	WorkspaceDeploy
	WorkspaceNetDB
	WorkspaceTools
)

type WorkspaceInfo struct {
	ID   Workspace
	Name string
	Tabs []Tab
}

var Workspaces = []WorkspaceInfo{
	{
		ID:   WorkspaceSystem,
		Name: "1: System",
		Tabs: []Tab{TabMonitor, TabBandwidth, TabDisk, TabTimers, TabServices},
	},
	{
		ID:   WorkspaceWebOps,
		Name: "2: WebOps",
		Tabs: []Tab{TabNginx, TabAutoNginx, TabPHPFPM, TabCertbot, TabSSL, TabWorkers},
	},
	{
		ID:   WorkspaceDeploy,
		Name: "3: Deploy",
		Tabs: []Tab{TabDeploy, TabGit, TabCI, TabEnv},
	},
	{
		ID:   WorkspaceNetDB,
		Name: "4: Net & DB",
		Tabs: []Tab{TabDatabase, TabContainers, TabHTTP, TabDNS, TabScanner},
	},
}

var TabDisplayNames = map[Tab]string{
	TabDashboard:  "Dashboard",
	TabMonitor:    "Monitor",
	TabBandwidth:  "Bandwidth",
	TabDisk:       "Disk",
	TabTimers:     "Timers",
	TabServices:   "Services",
	TabPorts:      "Ports",
	TabNginx:      "Nginx",
	TabAutoNginx:  "AutoNginx",
	TabPHPFPM:     "PHP-FPM",
	TabCertbot:    "Certbot",
	TabSSL:        "SSL/TLS",
	TabWorkers:    "Workers",
	TabDeploy:     "Deploy Hub",
	TabGit:        "Git",
	TabCI:         "CI Status",
	TabEnv:        ".Env",
	TabDatabase:   "Database",
	TabContainers: "Containers",
	TabHTTP:       "HTTP Tracer",
	TabDNS:        "DNS",
	TabScanner:    "Scanner",
	TabKnife:      "Knife",
	TabSSH:        "SSH",
	TabTerminal:   "Terminal",
}

type QuickStatsMsg struct {
	Uptime   string
	Hostname string
	OS       string
	Load1    float64
	Load5    float64
	Load15   float64
}

type QuickStatsTickMsg time.Time

type Model struct {
	ActiveWorkspace Workspace
	ActiveTab       Tab
	LastActiveTab   map[Workspace]Tab
	Width           int
	Height          int
	Program         *tea.Program
	ptyStarted      bool
	QuickStats      QuickStatsMsg

	// Sub-views
	DashboardView  dashboard.Model
	MonitorView    monitor.Model
	ContainersView containers.Model
	ServicesView   services.Model
	PortsView      ports.Model
	NginxView      nginx.Model
	AutoNginxView  autonginx.Model
	DeployView     deploy.Model
	PHPFPMView     phpfpm.Model
	WorkersView    workers.Model
	CertbotView    certbot.Model
	KnifeView      knife.Model
	SSLView        ssl.Model
	DatabaseView   database.Model
	BandwidthView  bandwidth.Model
	ScannerView    scanner.Model
	GitView        git.Model
	CIView         ci.Model
	SSHView        ssh.Model
	EnvView        env.Model
	TimersView     timers.Model
	DiskView       disk.Model
	HTTPView       httpclient.Model
	DNSView        dns.Model
	TerminalView   terminal.Model
	CommandPalette commandpalette.Model
}

func GetAllCommandPaletteItems() []commandpalette.CommandItem {
	return []commandpalette.CommandItem{
		{
			TabID:       int(TabDashboard),
			Title:       "DokOps Dashboard",
			Category:    "System",
			Description: "Main landing dashboard & operations overview",
			Keywords:    []string{"home", "dashboard", "main", "overview", "landing"},
		},
		{
			TabID:       int(TabMonitor),
			Title:       "System Monitor",
			Category:    "System",
			Description: "CPU, memory, load averages, processes (top/htop)",
			Keywords:    []string{"top", "htop", "cpu", "ram", "mem", "processes", "ps", "kill", "system"},
		},
		{
			TabID:       int(TabContainers),
			Title:       "Docker Containers",
			Category:    "Net & DB",
			Description: "Docker containers, live log streaming, start/stop/restart",
			Keywords:    []string{"docker", "containers", "ps", "logs", "compose", "images", "restart", "stop"},
		},
		{
			TabID:       int(TabServices),
			Title:       "Systemd Services",
			Category:    "System",
			Description: "Systemd service units & journalctl log viewer",
			Keywords:    []string{"systemd", "services", "journalctl", "logs", "systemctl", "daemon", "restart"},
		},
		{
			TabID:       int(TabPorts),
			Title:       "Ports & Sockets",
			Category:    "System",
			Description: "Listening sockets & port-to-PID mapper (ss/lsof)",
			Keywords:    []string{"ports", "sockets", "ss", "lsof", "netstat", "tcp", "udp", "listen", "kill port"},
		},
		{
			TabID:       int(TabNginx),
			Title:       "Nginx VHosts",
			Category:    "WebOps",
			Description: "Virtual hosts, sites-available/enabled, nginx -t",
			Keywords:    []string{"nginx", "vhosts", "sites", "web", "conf", "reverse proxy", "reload"},
		},
		{
			TabID:       int(TabAutoNginx),
			Title:       "Auto-Templater",
			Category:    "WebOps",
			Description: "Framework auto-detector and nginx config generator",
			Keywords:    []string{"autonginx", "templater", "laravel", "nextjs", "spa", "generator", "fastcgi"},
		},
		{
			TabID:       int(TabDeploy),
			Title:       "Deploy Hub",
			Category:    "Deploy",
			Description: "Zero-downtime multi-repo deployment pipeline",
			Keywords:    []string{"deploy", "pipeline", "release", "ci/cd", "pull", "migrate", "artisan"},
		},
		{
			TabID:       int(TabPHPFPM),
			Title:       "PHP-FPM Switcher",
			Category:    "WebOps",
			Description: "PHP version pool and OPcache/APCu manager",
			Keywords:    []string{"php", "phpfpm", "fpm", "opcache", "apcu", "fastcgi", "socket", "version"},
		},
		{
			TabID:       int(TabWorkers),
			Title:       "Artisan & Workers",
			Category:    "WebOps",
			Description: "Supervisor queue workers and schedule:list",
			Keywords:    []string{"workers", "artisan", "supervisor", "queue", "schedule", "cron", "jobs"},
		},
		{
			TabID:       int(TabCertbot),
			Title:       "Certbot SSL",
			Category:    "WebOps",
			Description: "Let's Encrypt automated SSL provisioning & DNS checks",
			Keywords:    []string{"certbot", "letsencrypt", "ssl", "tls", "certificates", "https", "provision"},
		},
		{
			TabID:       int(TabKnife),
			Title:       "Swiss Knife",
			Category:    "Tools",
			Description: "System utility tools and quick diagnostics",
			Keywords:    []string{"knife", "tools", "diagnostics", "utilities", "benchmarks"},
		},
		{
			TabID:       int(TabSSL),
			Title:       "SSL/TLS Inspector",
			Category:    "WebOps",
			Description: "Certificate expiration, SANs and cipher suites",
			Keywords:    []string{"ssl", "tls", "cert", "expiration", "s_client", "https", "sans", "expiry"},
		},
		{
			TabID:       int(TabDatabase),
			Title:       "Database Runner",
			Category:    "Net & DB",
			Description: "PostgreSQL & MySQL queries and connection metrics",
			Keywords:    []string{"db", "database", "sql", "postgres", "postgresql", "mysql", "queries", "select"},
		},
		{
			TabID:       int(TabBandwidth),
			Title:       "Live Bandwidth",
			Category:    "System",
			Description: "Real-time interface I/O throughput (iftop)",
			Keywords:    []string{"bandwidth", "iftop", "network", "rx", "tx", "traffic", "speed", "interface"},
		},
		{
			TabID:       int(TabScanner),
			Title:       "Port Scanner",
			Category:    "Net & DB",
			Description: "Concurrent TCP port sweep & banner grabber (nmap lite)",
			Keywords:    []string{"scanner", "nmap", "portscan", "sweep", "network", "reachability", "open ports"},
		},
		{
			TabID:       int(TabGit),
			Title:       "Git Inspector",
			Category:    "Deploy",
			Description: "Git status, diffs, staging and stash (lazygit)",
			Keywords:    []string{"git", "diff", "stage", "unstage", "stash", "lazygit", "commit", "branch"},
		},
		{
			TabID:       int(TabCI),
			Title:       "GitHub Actions CI",
			Category:    "Deploy",
			Description: "Workflow runs, statuses and commit logs",
			Keywords:    []string{"ci", "github actions", "workflows", "runs", "builds", "pipelines", "actions"},
		},
		{
			TabID:       int(TabSSH),
			Title:       "SSH Auditor",
			Category:    "System",
			Description: "Active login sessions and authorized_keys",
			Keywords:    []string{"ssh", "sessions", "logins", "authorized_keys", "tty", "who", "kill session"},
		},
		{
			TabID:       int(TabEnv),
			Title:       ".ENV Validator",
			Category:    "Deploy",
			Description: "Environment drift and missing variables checker",
			Keywords:    []string{"env", "environment", "dotenv", "drift", "config", "variables", "missing"},
		},
		{
			TabID:       int(TabTimers),
			Title:       "Timers & Cron",
			Category:    "System",
			Description: "Systemd timers and cron schedule timeline",
			Keywords:    []string{"timers", "cron", "crontab", "scheduled", "systemd", "timeline", "countdown"},
		},
		{
			TabID:       int(TabDisk),
			Title:       "Disk Space (ncdu)",
			Category:    "System",
			Description: "Hierarchical disk space analyzer & storage cleanup",
			Keywords:    []string{"disk", "ncdu", "du", "storage", "usage", "cleanup", "files", "folder"},
		},
		{
			TabID:       int(TabHTTP),
			Title:       "HTTP Client",
			Category:    "Net & DB",
			Description: "API request runner with latency waterfall and JSON viewer",
			Keywords:    []string{"http", "api", "curl", "postman", "rest", "json", "trace", "waterfall"},
		},
		{
			TabID:       int(TabDNS),
			Title:       "DNS Inspector",
			Category:    "Net & DB",
			Description: "DNS record lookup and nameserver tester (dig)",
			Keywords:    []string{"dns", "dig", "records", "nameserver", "lookup", "mx", "cname", "txt", "ns"},
		},
		{
			TabID:       int(TabTerminal),
			Title:       "Terminal (PTY)",
			Category:    "Tools",
			Description: "Integrated interactive bash shell",
			Keywords:    []string{"terminal", "bash", "shell", "pty", "console", "cmd"},
		},
	}
}

func NewModel() Model {
	lastTabs := make(map[Workspace]Tab)
	for _, ws := range Workspaces {
		if len(ws.Tabs) > 0 {
			lastTabs[ws.ID] = ws.Tabs[0]
		}
	}

	paletteItems := GetAllCommandPaletteItems()

	return Model{
		ActiveWorkspace: WorkspaceSystem,
		ActiveTab:       TabDashboard,
		LastActiveTab:   lastTabs,
		CommandPalette:  commandpalette.New(paletteItems),
		DashboardView:   dashboard.New(),
		MonitorView:     monitor.New(),
		ContainersView:  containers.New(),
		ServicesView:    services.New(),
		PortsView:       ports.New(),
		NginxView:       nginx.New(),
		AutoNginxView:   autonginx.New(),
		DeployView:      deploy.New(),
		PHPFPMView:      phpfpm.New(),
		WorkersView:     workers.New(),
		CertbotView:     certbot.New(),
		KnifeView:       knife.New(),
		SSLView:         ssl.New(),
		DatabaseView:    database.New(),
		BandwidthView:   bandwidth.New(),
		ScannerView:     scanner.New(),
		GitView:         git.New(),
		CIView:          ci.New(),
		SSHView:         ssh.New(),
		EnvView:         env.New(),
		TimersView:      timers.New(),
		DiskView:        disk.New(""),
		HTTPView:        httpclient.New(),
		DNSView:         dns.New(),
		TerminalView:    terminal.New(),
	}
}

// GetWorkspaceForTab finds which workspace owns a given tab
func GetWorkspaceForTab(t Tab) Workspace {
	for _, ws := range Workspaces {
		for _, tab := range ws.Tabs {
			if tab == t {
				return ws.ID
			}
		}
	}
	return WorkspaceSystem
}

// SetWorkspace switches to a specific workspace and restores its active tab
func (m *Model) SetWorkspace(ws Workspace) {
	if int(ws) < 0 || int(ws) >= len(Workspaces) {
		return
	}
	m.ActiveWorkspace = ws
	if tab, ok := m.LastActiveTab[ws]; ok {
		m.ActiveTab = tab
	} else if len(Workspaces[ws].Tabs) > 0 {
		m.ActiveTab = Workspaces[ws].Tabs[0]
		m.LastActiveTab[ws] = m.ActiveTab
	}
}

// NextWorkspace moves to the next workspace
func (m *Model) NextWorkspace() {
	next := (int(m.ActiveWorkspace) + 1) % len(Workspaces)
	m.SetWorkspace(Workspace(next))
}

// PrevWorkspace moves to the previous workspace
func (m *Model) PrevWorkspace() {
	prev := (int(m.ActiveWorkspace) + len(Workspaces) - 1) % len(Workspaces)
	m.SetWorkspace(Workspace(prev))
}

// SetTab sets the active tab directly and aligns the active workspace
func (m *Model) SetTab(t Tab) {
	m.ActiveTab = t
	ws := GetWorkspaceForTab(t)
	m.ActiveWorkspace = ws
	m.LastActiveTab[ws] = t
}

// NextSubTab cycles to the next sub-tab within the current workspace
func (m *Model) NextSubTab() {
	tabs := Workspaces[m.ActiveWorkspace].Tabs
	for i, t := range tabs {
		if t == m.ActiveTab {
			nextIdx := (i + 1) % len(tabs)
			m.ActiveTab = tabs[nextIdx]
			m.LastActiveTab[m.ActiveWorkspace] = m.ActiveTab
			return
		}
	}
	if len(tabs) > 0 {
		m.ActiveTab = tabs[0]
		m.LastActiveTab[m.ActiveWorkspace] = m.ActiveTab
	}
}

// PrevSubTab cycles to the previous sub-tab within the current workspace
func (m *Model) PrevSubTab() {
	tabs := Workspaces[m.ActiveWorkspace].Tabs
	for i, t := range tabs {
		if t == m.ActiveTab {
			prevIdx := (i + len(tabs) - 1) % len(tabs)
			m.ActiveTab = tabs[prevIdx]
			m.LastActiveTab[m.ActiveWorkspace] = m.ActiveTab
			return
		}
	}
	if len(tabs) > 0 {
		m.ActiveTab = tabs[0]
		m.LastActiveTab[m.ActiveWorkspace] = m.ActiveTab
	}
}

// hasInternalTabHandling returns true if the current view has multiple internal input fields or panes
// that use Tab / Shift+Tab to switch focus within the view.
func (m Model) hasInternalTabHandling() bool {
	switch m.ActiveTab {
	case TabDatabase, TabCertbot, TabEnv, TabHTTP, TabDNS, TabAutoNginx, TabGit, TabWorkers, TabSSH, TabKnife:
		return true
	default:
		return false
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.MonitorView.Init(),
		m.ContainersView.Init(),
		m.ServicesView.Init(),
		m.PortsView.Init(),
		m.NginxView.Init(),
		m.AutoNginxView.Init(),
		m.DeployView.Init(),
		m.PHPFPMView.Init(),
		m.WorkersView.Init(),
		m.CertbotView.Init(),
		m.KnifeView.Init(),
		m.SSLView.Init(),
		m.DatabaseView.Init(),
		m.BandwidthView.Init(),
		m.ScannerView.Init(),
		m.GitView.Init(),
		m.CIView.Init(),
		m.SSHView.Init(),
		m.EnvView.Init(),
		m.TimersView.Init(),
		m.DiskView.Init(),
		m.HTTPView.Init(),
		m.DNSView.Init(),
		m.TerminalView.Init(),
		FetchQuickStats(),
		quickStatsTick(),
	)
}

func quickStatsTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return QuickStatsTickMsg(t)
	})
}

func FetchQuickStats() tea.Cmd {
	return func() tea.Msg {
		hInfo, _ := host.Info()
		lInfo, _ := load.Avg()

		uptimeStr := "-"
		hostname := "localhost"
		osName := "Linux"

		if hInfo != nil {
			dur := time.Duration(hInfo.Uptime) * time.Second
			days := int(dur.Hours() / 24)
			hours := int(dur.Hours()) % 24
			mins := int(dur.Minutes()) % 60
			if days > 0 {
				uptimeStr = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
			} else {
				uptimeStr = fmt.Sprintf("%dh %dm", hours, mins)
			}
			hostname = hInfo.Hostname
			osName = fmt.Sprintf("%s %s", hInfo.OS, hInfo.Platform)
		}

		var l1, l5, l15 float64
		if lInfo != nil {
			l1 = lInfo.Load1
			l5 = lInfo.Load5
			l15 = lInfo.Load15
		}

		return QuickStatsMsg{
			Uptime:   uptimeStr,
			Hostname: hostname,
			OS:       osName,
			Load1:    l1,
			Load5:    l5,
			Load15:   l15,
		}
	}
}
