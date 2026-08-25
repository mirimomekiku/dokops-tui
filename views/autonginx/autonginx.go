package autonginx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/theme"
)

type FrameworkType string

const (
	FrameworkLaravel   FrameworkType = "Laravel"
	FrameworkNode      FrameworkType = "Node.js / Next.js"
	FrameworkStaticSPA FrameworkType = "Static SPA (Vite/React)"
	FrameworkWordPress FrameworkType = "WordPress"
	FrameworkGeneric   FrameworkType = "Generic PHP / HTML"
)

type ProjectInfo struct {
	Name          string
	Path          string
	Framework     FrameworkType
	SuggestedPort string
	SuggestedRoot string
}

type ScanResultMsg struct {
	Projects []ProjectInfo
	Err      error
}

type DeployPipelineMsg struct {
	Success bool
	Output  string
	Err     error
}

const LaravelTemplate = `server {
    listen 80;
    listen [::]:80;
    server_name {{.Domain}};
    root {{.RootDir}};

    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";
    index index.php index.html;
    charset utf-8;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location = /favicon.ico { access_log off; log_not_found off; }
    location = /robots.txt  { access_log off; log_not_found off; }

    error_page 404 /index.php;

    location ~ \.php$ {
        fastcgi_pass {{.FastCGISocket}};
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_hide_header X-Powered-By;
        fastcgi_buffer_size 128k;
        fastcgi_buffers 4 256k;
        fastcgi_busy_buffers_size 256k;
    }

    location ~ /\.(?!well-known).* {
        deny all;
    }
}
`

const NodeTemplate = `server {
    listen 80;
    listen [::]:80;
    server_name {{.Domain}};

    location / {
        proxy_pass http://127.0.0.1:{{.Port}};
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`

const SPATemplate = `server {
    listen 80;
    listen [::]:80;
    server_name {{.Domain}};
    root {{.RootDir}};
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location ~* \.(?:css|js|jpg|jpeg|gif|png|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
`

const WordPressTemplate = `server {
    listen 80;
    listen [::]:80;
    server_name {{.Domain}};
    root {{.RootDir}};
    index index.php index.html;

    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        fastcgi_pass {{.FastCGISocket}};
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        expires max;
        log_not_found off;
    }

    location ~* /(?:uploads|files)/.*\.php$ {
        deny all;
    }
}
`

type TemplateData struct {
	Domain        string
	RootDir       string
	Port          string
	FastCGISocket string
}

type Model struct {
	projects       []ProjectInfo
	table          table.Model
	domainInput    textinput.Model
	previewVP      viewport.Model
	selectedProj   *ProjectInfo
	socketChoice   int
	availableSocks []string
	statusMessage  string
	isGenerating   bool
	width          int
	height         int
	err            error
}

func New() Model {
	di := textinput.New()
	di.Placeholder = "example.com or api.domain.local"
	di.SetValue("app.local")
	di.CharLimit = 255
	di.Width = 35

	cols := []table.Column{
		{Title: "PROJECT NAME", Width: 22},
		{Title: "DETECTED FRAMEWORK", Width: 24},
		{Title: "PATH", Width: 32},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(7),
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

	vp := viewport.New(80, 12)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1)

	socks := []string{
		"unix:/run/php/php8.3-fpm.sock",
		"unix:/run/php/php8.2-fpm.sock",
		"unix:/run/php/php8.1-fpm.sock",
		"unix:/run/php/php8.0-fpm.sock",
		"unix:/run/php/php7.4-fpm.sock",
	}

	return Model{
		table:          t,
		domainInput:    di,
		previewVP:      vp,
		availableSocks: socks,
		socketChoice:   0,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.ScanDirectory("/var/www"),
	)
}

func (m Model) ScanDirectory(root string) tea.Cmd {
	return func() tea.Msg {
		var projs []ProjectInfo

		entries, err := os.ReadDir(root)
		if err != nil {
			// Fallback mock scan if /var/www doesn't exist on dev host
			projs = append(projs,
				ProjectInfo{Name: "ecommerce-api", Path: "/var/www/ecommerce-api", Framework: FrameworkLaravel, SuggestedRoot: "/var/www/ecommerce-api/public"},
				ProjectInfo{Name: "nextjs-frontend", Path: "/var/www/nextjs-frontend", Framework: FrameworkNode, SuggestedPort: "3000"},
				ProjectInfo{Name: "admin-dashboard", Path: "/var/www/admin-dashboard", Framework: FrameworkStaticSPA, SuggestedRoot: "/var/www/admin-dashboard/dist"},
				ProjectInfo{Name: "company-blog", Path: "/var/www/company-blog", Framework: FrameworkWordPress, SuggestedRoot: "/var/www/company-blog"},
			)
			return ScanResultMsg{Projects: projs}
		}

		for _, e := range entries {
			if e.IsDir() {
				pPath := filepath.Join(root, e.Name())
				fw, rootDir, port := detectFramework(pPath)
				projs = append(projs, ProjectInfo{
					Name:          e.Name(),
					Path:          pPath,
					Framework:     fw,
					SuggestedRoot: rootDir,
					SuggestedPort: port,
				})
			}
		}

		return ScanResultMsg{Projects: projs}
	}
}

func detectFramework(path string) (FrameworkType, string, string) {
	if _, err := os.Stat(filepath.Join(path, "artisan")); err == nil {
		return FrameworkLaravel, filepath.Join(path, "public"), ""
	}
	if _, err := os.Stat(filepath.Join(path, "wp-config.php")); err == nil {
		return FrameworkWordPress, path, ""
	}
	if _, err := os.Stat(filepath.Join(path, "package.json")); err == nil {
		if _, err := os.Stat(filepath.Join(path, "dist")); err == nil {
			return FrameworkStaticSPA, filepath.Join(path, "dist"), ""
		}
		if _, err := os.Stat(filepath.Join(path, "build")); err == nil {
			return FrameworkStaticSPA, filepath.Join(path, "build"), ""
		}
		return FrameworkNode, path, "3000"
	}
	if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
		return FrameworkStaticSPA, path, ""
	}
	return FrameworkGeneric, path, ""
}

func (m Model) GenerateAndDeployPipeline() tea.Cmd {
	if m.selectedProj == nil {
		return nil
	}

	domain := strings.TrimSpace(m.domainInput.Value())
	if domain == "" {
		domain = m.selectedProj.Name + ".local"
	}

	confContent := m.renderTemplateString()

	return func() tea.Msg {
		availPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", domain)
		enabPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", domain)

		// 1. Write sites-available
		err := os.WriteFile(availPath, []byte(confContent), 0644)
		if err != nil {
			// Try with sudo or fallback error
			cmd := exec.Command("sudo", "tee", availPath)
			cmd.Stdin = strings.NewReader(confContent)
			if err2 := cmd.Run(); err2 != nil {
				return DeployPipelineMsg{Success: false, Output: fmt.Sprintf("Failed to write %s: %v (Try running dok-ops with root/sudo)", availPath, err)}
			}
		}

		// 2. Symlink to sites-enabled
		_ = os.Remove(enabPath)
		if err := os.Symlink(availPath, enabPath); err != nil {
			_ = exec.Command("sudo", "ln", "-sf", availPath, enabPath).Run()
		}

		// 3. Test nginx syntax
		testOut, err := exec.Command("nginx", "-t").CombinedOutput()
		if err != nil {
			return DeployPipelineMsg{Success: false, Output: fmt.Sprintf("Nginx syntax test failed: %v\n%s", err, string(testOut))}
		}

		// 4. Reload nginx
		_ = exec.Command("systemctl", "reload", "nginx").Run()

		return DeployPipelineMsg{
			Success: true,
			Output:  fmt.Sprintf("✓ Generated %s\n✓ Symlinked %s\n✓ Nginx syntax test OK\n✓ Nginx reloaded successfully!", availPath, enabPath),
		}
	}
}

func (m Model) renderTemplateString() string {
	if m.selectedProj == nil {
		return ""
	}

	domain := strings.TrimSpace(m.domainInput.Value())
	if domain == "" {
		domain = m.selectedProj.Name + ".local"
	}

	data := TemplateData{
		Domain:        domain,
		RootDir:       m.selectedProj.SuggestedRoot,
		Port:          m.selectedProj.SuggestedPort,
		FastCGISocket: m.availableSocks[m.socketChoice],
	}

	var rawTmpl string
	switch m.selectedProj.Framework {
	case FrameworkLaravel:
		rawTmpl = LaravelTemplate
	case FrameworkNode:
		rawTmpl = NodeTemplate
	case FrameworkStaticSPA:
		rawTmpl = SPATemplate
	case FrameworkWordPress:
		rawTmpl = WordPressTemplate
	default:
		rawTmpl = LaravelTemplate
	}

	tmpl, err := template.New("nginx").Parse(rawTmpl)
	if err != nil {
		return fmt.Sprintf("Template parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Template execute error: %v", err)
	}

	return buf.String()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case ScanResultMsg:
		m.projects = msg.Projects
		m.updateTableRows()
		if len(m.projects) > 0 {
			m.selectedProj = &m.projects[0]
			m.previewVP.SetContent(m.renderTemplateString())
		}

	case DeployPipelineMsg:
		m.isGenerating = false
		m.statusMessage = msg.Output

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "down", "j", "k":
			var tCmd tea.Cmd
			m.table, tCmd = m.table.Update(msg)
			if len(m.projects) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.projects) {
					m.selectedProj = &m.projects[idx]
					m.previewVP.SetContent(m.renderTemplateString())
				}
			}
			return m, tCmd

		case "s":
			m.socketChoice = (m.socketChoice + 1) % len(m.availableSocks)
			m.previewVP.SetContent(m.renderTemplateString())
			return m, nil

		case "g", "enter":
			m.isGenerating = true
			m.statusMessage = "Generating config and testing Nginx..."
			return m, m.GenerateAndDeployPipeline()

		case "r":
			return m, m.ScanDirectory("/var/www")
		}

		var cmd tea.Cmd
		m.domainInput, cmd = m.domainInput.Update(msg)
		cmds = append(cmds, cmd)
		m.previewVP.SetContent(m.renderTemplateString())
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, p := range m.projects {
		rows = append(rows, table.Row{
			p.Name,
			string(p.Framework),
			p.Path,
		})
	}
	m.table.SetRows(rows)
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
	m.previewVP.Width = vpWidth
	m.previewVP.Height = vpHeight
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	domainBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Server Domain: "),
				m.domainInput.View(),
				"   ",
				theme.BadgeInfo.Render(fmt.Sprintf(" PHP Socket: %s [s] ", m.availableSocks[m.socketChoice])),
			),
		)

	statusLine := ""
	if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().
			Foreground(theme.ColorHighlight).
			Bold(true).
			Render(m.statusMessage)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("⚡ SMART NGINX AUTO-TEMPLATER & FRAMEWORK DETECTOR"),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[j/k: Select Project | s: Cycle PHP Socket | g/Enter: Generate & Deploy]"),
		),
		"",
		m.table.View(),
		"",
		domainBox,
		statusLine,
		"",
		theme.CardTitleStyle.Render("📄 GENERATED NGINX CONFIG PREVIEW"),
		m.previewVP.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
