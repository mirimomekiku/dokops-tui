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

## Module Directory

| Key | Module | Purpose | Alternative To |
|:---:|---|---|---|
| `1` | **System Monitor** | Per-core CPU, RAM/Swap, and sortable process killer | `htop` |
| `2` | **Docker Manager** | Container state management and live log streaming | `ctop` / `docker` |
| `3` | **Systemd Services** | Unit lifecycle management and systemd journal logs | `systemctl` / `journalctl` |
| `4` | **Listening Ports** | Active TCP/UDP socket mapper and conflict resolution | `ss` / `lsof -i` |
| `5` | **Nginx Manager** | Site symlink toggle, syntax tester, and config viewer | `nginx -t` |
| `6` | **Auto-Nginx** | Framework detector (Laravel, Node, SPA, WP) & config generator | Manual vhost creation |
| `7` | **Deploy Hub** | Multi-repo Git tracking and zero-downtime release pipelines | Custom deploy scripts |
| `8` | **PHP-FPM Switcher** | Multi-PHP socket discovery, daemon restarts, and OPcache flush | Manual FPM restarts |
| `9` | **Background Workers** | Supervisor/Horizon process manager and Artisan scheduler | `supervisorctl` |
| `0` | **Certbot SSL** | Let's Encrypt SSL provisioner with DNS validation & dry-run | `certbot --nginx` |
| `K` | **DevOps Knife** | Offline converters for JWT, Cron, Base64/Hex, and Hashes | CyberChef |
| `L` | **SSL/TLS Inspector** | Certificate chain, SAN, cipher, and expiration analyzer | `openssl s_client` |
| `B` | **Database Monitor** | PostgreSQL/MySQL pool health, slow queries, and SQL runner | `pg_stat_activity` |
| `W` | **Live Bandwidth** | Interface transfer rate monitor (KB/s, MB/s) | `iftop` / `bandwhich` |
| `A` | **TCP Port Scanner** | Concurrent port scanner with banner grabbing and latency | `nmap` / `nc -zv` |
| `G` | **Git Inspector** | Branch status, staged/unstaged diff viewer, and stash/pop | `lazygit` |
| `C` | **CI Status** | GitHub Actions workflow run monitor | `gh run list` |
| `S` | **SSH Auditor** | Active login sessions, session termination, and authorized keys | `who` / `authorized_keys` |
| `E` | **.Env Validator** | Configuration drift detector against `.env.example` | Custom validators |
| `O` | **Timers & Cron** | Unified timeline of user crons, system crons, and timers | `crontab` / `list-timers` |
| `D` | **Disk Analyzer** | Hierarchical filesystem space usage and directory navigation | `ncdu` |
| `H` | **HTTP Tracer** | Request latency waterfall (DNS, TCP, TLS, TTFB, Transfer) | `curl` |
| `N` | **DNS Inspector** | Multi-nameserver query resolver with record type toggles | `dig` |
| `T` | **Embedded Terminal** | Full POSIX pseudo-terminal subshell | `$SHELL` |

---

## Global Keybindings

| Key | Action |
|---|---|
| `1` - `0` | Jump to numbered tabs (1: Monitor to 0: Certbot) |
| `K`, `L`, `B`, `W`, `A`, `G`, `C`, `S`, `E`, `O`, `D`, `H`, `N`, `T` | Jump to corresponding alpha module |
| `Tab` / `Shift+Tab` | Cycle tabs forward / backward |
| `F1` - `F12` | Function key tab shortcuts |
| `j` / `k` (or Arrow keys) | Navigate rows in tables and viewports |
| `Enter` | Submit input / execute operational action |
| `i` / `Ctrl+]` | Focus / unfocus embedded PTY subshell |
| `q` / `Ctrl+C` | Quit |

---

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md): Event loop, concurrency model, and PTY isolation.
- [FEATURES.md](FEATURES.md): Comprehensive feature and shortcut matrix for all 24 modules.
- [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md): Codebase directory hierarchy.
- [CODEBASE.md](CODEBASE.md): Internal implementation details and state transitions.
- [PACKAGES.md](PACKAGES.md): Dependency catalog.

---

## License

MIT License.
