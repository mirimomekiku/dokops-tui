# dok-ops

A unified, keyboard-driven terminal dashboard for DevOps and WebOps, built in Go using the Charm Bubble Tea / Lipgloss framework. It replaces disjointed CLI utilities with a single, real-time cockpit.

---

## Quick Start

### Build & Run

```bash
# Build binary
go build -o bin/dok-ops .

# Run
./bin/dok-ops
```

---

## Workspaces & Modules (24 Integrated Tools)

`dok-ops` consolidates 24 modules across **5 primary workspaces**:

### 1. 🖥️ Workspace 1: System
* **Monitor**: Per-core CPU, RAM/Swap, and sortable process killer (`htop`)
* **Bandwidth**: Real-time interface transfer rate monitor (`iftop`)
* **Disk**: Interactive filesystem space analyzer (`ncdu`)
* **Timers**: Unified timeline of crons & systemd timers (`crontab` / `systemctl list-timers`)
* **Services**: Systemd unit lifecycle and journal logs (`systemctl` / `journalctl`)
* **Ports**: Active TCP/UDP listening sockets & conflict killer (`ss` / `lsof -i`)

### 2. 🌐 Workspace 2: WebOps
* **Nginx**: VHost symlink toggle, syntax tester (`nginx -t`), and config viewer
* **Auto-Nginx**: Framework detector (Laravel, Node, SPA, WP) & config generator
* **PHP-FPM**: Multi-PHP socket discovery, daemon restarts & OPcache flush
* **Certbot**: Let's Encrypt SSL provisioner with DNS validation & dry-run
* **SSL/TLS**: Certificate chain, SAN, cipher & expiration analyzer (`openssl s_client`)
* **Workers**: Supervisor/Horizon process manager & Artisan scheduler

### 3. 🚀 Workspace 3: Deploy
* **Deploy Hub**: Multi-repo Git tracking & zero-downtime release pipelines
* **Git**: Branch status, staged/unstaged diff viewer, and stash/pop (`lazygit`)
* **CI Actions**: GitHub Actions workflow run monitor (`gh run list`)
* **.Env**: Configuration drift detector against `.env.example`

### 4. 🗄️ Workspace 4: Net & DB
* **Database**: PostgreSQL/MySQL pool health, slow queries & SQL runner
* **Containers**: Docker container state management and live log streaming (`ctop`)
* **HTTP Tracer**: Request latency waterfall profiler (`curl`)
* **DNS**: Multi-nameserver query resolver with record type toggles (`dig`)
* **Scanner**: Concurrent TCP port scanner with banner grabbing (`nmap`)

### 5. 🛠️ Workspace 5: Tools & PTY
* **Knife**: DevOps offline converters (JWT, Cron, Base64/Hex, Hashes)
* **SSH**: Active SSH sessions auditor, session termination & authorized keys
* **Terminal**: Embedded POSIX pseudo-terminal (PTY) subshell (`$SHELL`)

---

## Global Keybindings

| Key | Action |
| --- | --- |
| `1` - `5` | Switch directly to Workspace 1 - 5 |
| `Left` / `Right` (or `[` / `]`) | Cycle previous / next workspace |
| `Tab` / `Shift+Tab` | Cycle sub-tabs within active workspace |
| `F1` - `F12` | Direct function-key jump to specific tools |
| `j` / `k` (or Arrow keys) | Navigate rows in tables and viewports |
| `Enter` | Submit input / execute operational action |
| `i` / `Ctrl+]` | Focus / unfocus embedded PTY subshell |
| `q` / `Ctrl+C` | Quit |

---

## License

MIT License.
