# Project Structure: dok-ops

A complete directory map and file manifest for the `dok-ops` codebase.

```
dok-ops/
├── main.go                       # Application entrypoint & Bubble Tea runner
├── go.mod                        # Go module dependencies
├── go.sum                        # Cryptographic checksums of dependencies
├── README.md                     # High-level overview & operator guide
├── ARCHITECTURE.md               # TEA architecture, message flow & concurrency model
├── PROJECT_STRUCTURE.md          # Complete directory and file manifest (this file)
├── CODEBASE.md                   # In-depth internal implementation and state transitions
├── PACKAGES.md                   # Direct & transitive Go dependency catalog
├── FEATURES.md                   # Comprehensive 24-module feature breakdown & shortcuts
│
├── app/                          # Master application coordinator & shell router
│   ├── model.go                  # Global application state, subview models & host stats
│   ├── update.go                 # Central event router, message dispatch & global keybindings
│   └── view.go                   # Top header, responsive layout & contextual footers
│
├── internal/                     # Private reusable internal libraries
│   └── theme/
│       └── theme.go              # Lipgloss style tokens, badges, gauges, progress bars & formatters
│
└── views/                        # Modular subview implementations (24 modules)
    ├── autonginx/
    │   └── autonginx.go          # Framework auto-detection & production Nginx config generator
    ├── bandwidth/
    │   └── bandwidth.go          # Real-time network interface ingress/egress rate monitor
    ├── certbot/
    │   └── certbot.go            # Let's Encrypt SSL provisioning wizard with dry-run tests
    ├── ci/
    │   └── ci.go                 # GitHub Actions pipeline workflow monitor & step inspector
    ├── containers/
    │   └── containers.go         # Docker container manager & live log stream viewport
    ├── database/
    │   └── database.go           # PostgreSQL & MySQL health inspector & ad-hoc SQL runner
    ├── deploy/
    │   └── deploy.go             # Multi-repo Git deployment hub & zero-downtime release runner
    ├── disk/
    │   └── disk.go               # Hierarchical filesystem disk space analyzer (ncdu lite)
    ├── dns/
    │   └── dns.go                # DNS record inspector & multi-nameserver query resolver
    ├── env/
    │   └── env.go                # Environment parameter drift & .env.example validator
    ├── git/
    │   └── git.go                # Git repository mini-inspector, commit log & diff viewer
    ├── httpclient/
    │   └── httpclient.go         # HTTP latency tracer & sub-millisecond waterfall profiler
    ├── knife/
    │   └── knife.go              # DevOps Swiss Army Knife (JWT, Cron, Base64/Hex, Hashes)
    ├── monitor/
    │   └── monitor.go            # Per-core CPU, RAM/Swap, and process manager (htop lite)
    ├── nginx/
    │   └── nginx.go              # Nginx vhost symlink manager, syntax tester & config viewer
    ├── phpfpm/
    │   └── phpfpm.go             # Multi-PHP socket discovery, daemon manager & OPcache clearer
    ├── ports/
    │   └── ports.go              # Active TCP/UDP listening ports & process socket conflict killer
    ├── scanner/
    │   └── scanner.go            # Concurrent TCP port scanner, banner grabber & RTT meter
    ├── services/
    │   └── services.go           # Systemd unit manager, service controls & journal log viewer
    ├── ssh/
    │   └── ssh.go                # Active SSH sessions auditor, session killer & authorized_keys
    ├── ssl/
    │   └── ssl.go                # SSL/TLS certificate inspector, SAN parser & expiration gauge
    ├── terminal/
    │   └── terminal.go           # Embedded POSIX pseudo-terminal (PTY) subshell runner
    ├── timers/
    │   └── timers.go             # Unified Cron & Systemd timer timeline with countdowns
    └── workers/
        └── workers.go            # Supervisor/Horizon process manager & Artisan scheduler
```

---

## Directory Roles & Module Isolation

### `app/` (Application Shell)
- **`app/model.go`**: Instantiates and maintains instances of all 24 subview models. Polls host-level system metrics (Load Average, Uptime, Hostname) every 3 seconds.
- **`app/update.go`**: Intercepts `tea.WindowSizeMsg` to propagate responsive dimensions across all child viewports. Handles top-level navigation keys (`1`-`0`, `K`, `L`, `B`, `W`, etc.) and routes incoming async messages to their target subviews.
- **`app/view.go`**: Renders the persistent top bar (Logo, scrolling Tab list, Host load/uptime indicators), computes available viewport height, renders the active module, and displays the contextual keybinding footer.

### `internal/theme/` (Design System)
- **`theme.go`**: Provides semantic Lipgloss styles:
  - `ColorPrimary`, `ColorSecondary`, `ColorSuccess`, `ColorDanger`, `ColorWarning`, `ColorInfo`, `ColorHighlight`, `ColorMuted`, `ColorSurface`.
  - Formatting helpers: `FormatBytes`, `FormatDuration`, `RenderProgressBar`, `RenderGauge`.
  - Badges: `BadgePrimary`, `BadgeSuccess`, `BadgeDanger`, `BadgeWarning`, `BadgeInfo`.

### `views/` (Operational Modules)
Each subdirectory under `views/` encapsulates:
1. **Model Struct**: Internal state, focused inputs, tables, viewports, and cached data.
2. **`New() Model`**: Constructor function setting initial columns, styles, and dimensions.
3. **`Init() tea.Cmd`**: Initial async command triggers (e.g. scanning directories, sampling metrics).
4. **`Update(tea.Msg) (Model, tea.Cmd)`**: Subview message processing and keyboard handler.
5. **`View() string`**: Pure visual rendering logic returning a formatted string.
