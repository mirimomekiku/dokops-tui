package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"

	"dok-ops/views/autonginx"
	"dok-ops/views/bandwidth"
	"dok-ops/views/certbot"
	"dok-ops/views/ci"
	"dok-ops/views/containers"
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
	TabMonitor Tab = iota
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

var TabNames = []string{
	"1: Monitor",
	"2: Docker",
	"3: Services",
	"4: Ports",
	"5: Nginx",
	"6: AutoNginx",
	"7: Deploy",
	"8: PHP-FPM",
	"9: Workers",
	"0: Certbot",
	"K: Knife",
	"L: SSL",
	"B: DB",
	"W: Traffic",
	"A: Scanner",
	"G: Git",
	"C: CI",
	"S: SSH",
	"E: Env",
	"O: Timers",
	"D: Disk",
	"H: HTTP",
	"N: DNS",
	"T: Shell",
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
	ActiveTab   Tab
	Width       int
	Height      int
	Program     *tea.Program
	ptyStarted  bool
	QuickStats  QuickStatsMsg

	// Sub-views
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
}

func NewModel() Model {
	return Model{
		ActiveTab:      TabMonitor,
		MonitorView:    monitor.New(),
		ContainersView: containers.New(),
		ServicesView:   services.New(),
		PortsView:      ports.New(),
		NginxView:      nginx.New(),
		AutoNginxView:  autonginx.New(),
		DeployView:     deploy.New(),
		PHPFPMView:     phpfpm.New(),
		WorkersView:    workers.New(),
		CertbotView:    certbot.New(),
		KnifeView:      knife.New(),
		SSLView:        ssl.New(),
		DatabaseView:   database.New(),
		BandwidthView:  bandwidth.New(),
		ScannerView:    scanner.New(),
		GitView:        git.New(),
		CIView:         ci.New(),
		SSHView:        ssh.New(),
		EnvView:        env.New(),
		TimersView:     timers.New(),
		DiskView:       disk.New(""),
		HTTPView:       httpclient.New(),
		DNSView:        dns.New(),
		TerminalView:   terminal.New(),
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
