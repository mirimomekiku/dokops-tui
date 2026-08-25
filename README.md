# dok-ops

Unified DevOps and WebOps terminal cockpit designed for high-density host operations, container lifecycle management, reverse proxy automation, zero-downtime deployment pipelines, and network diagnostics. Built on Go and the Charm Bubble Tea / Lipgloss terminal framework.

---

## Architectural Overview

`dok-ops` aggregates 24 operational tools into a single, low-latency, event-driven terminal user interface (TUI). It eliminates context switching between disjointed command-line utilities (`htop`, `ctop`, `systemctl`, `journalctl`, `ss`, `nginx -t`, `certbot`, `lazygit`, `dig`, `curl`, `supervisorctl`, `crontab`) by consolidating monitoring, orchestration, and diagnostics into unified event loops with background asynchronous polling.

```
+-------------------------------------------------------------------------------------------------------+
|  dok-ops v1.0  [Tabs 1-0, K, L, B, W, A, G, C, S, E, O, D, H, N, T]            Host: Linux | Load: 0.12  |
+-------------------------------------------------------------------------------------------------------+
|                                                                                                       |
|  [Active Operational Viewport]                                                                        |
|  - Real-time event subscription via Bubble Tea async commands                                         |
|  - Raw PTY multiplexing via OS pseudoterminal allocation                                             |
|  - Non-blocking I/O execution with direct POSIX signal forwarding                                   |
|                                                                                                       |
+-------------------------------------------------------------------------------------------------------+
|  Keybindings Context Bar                                                            dok-ops Cockpit   |
+-------------------------------------------------------------------------------------------------------+
```

---

## Functional Subsystems & Modules

### 1. Host Telemetry & Resource Profiling

| Module | Identifier | Capabilities | Driver / Package |
|---|---|---|---|
| **System Monitor** | `1` / `views/monitor` | Emulates `htop`: per-core CPU profiling, RAM/Swap allocation gauges, sortable process table (`c`, `m`, `p`, `n`), and interactive `SIGTERM` process termination (`k`). | `gopsutil/v3/cpu`, `mem`, `process` |
| **Live Bandwidth** | `W` / `views/bandwidth` | Emulates `iftop` / `bandwhich`: Real-time network interface traffic inspection (`eth0`, `wlan0`, `docker0`, `lo`), delta transfer rates (KB/s, MB/s), total throughput, and packet drop/error telemetry. | `gopsutil/v3/net` |
| **Disk Space Analyzer** | `D` / `views/disk` | Emulates `ncdu`: Asynchronous filesystem tree traversal, capacity allocation bars, dynamic sorting, interactive directory navigation (`h`/`j`/`k`/`l`), and path deletion (`d`). | `io/fs`, `filepath.WalkDir` |
| **Listening Ports Mapper** | `4` / `views/ports` | Emulates `ss` / `lsof -i`: Scans active TCP/UDP listening sockets and established connections, resolves bound PIDs and process binaries, and supports instant process kill (`k`) to clear bind conflicts. | `gopsutil/v3/net`, `process` |

---

### 2. Container & Service Orchestration

| Module | Identifier | Capabilities | Driver / Package |
|---|---|---|---|
| **Docker Manager** | `2` / `views/containers` | Native Docker daemon client: Real-time container state inspection, start (`u`), stop (`s`), restart (`r`), forced removal (`d`), and non-blocking streaming container log tailing (`l`). | `docker/docker/client`, `docker/api` |
| **Systemd Service Manager** | `3` / `views/services` | Direct DBus system daemon integration: Unit status tracking (active, inactive, failed), unit lifecycle commands (`u`, `s`, `r`), filter toggling (`f`), and journal log viewing (`l`). | `coreos/go-systemd/v22/dbus` |
| **Background Workers & Scheduler** | `9` / `views/workers` | Supervisor and Laravel Horizon worker supervisor: Process list inspection, one-key worker restarts (`r`), live log streaming (`l`), and Artisan schedule inspection (`schedule:list`). | `os/exec` |

---

### 3. WebOps, Reverse Proxy & SSL Automation

| Module | Identifier | Capabilities | Driver / Package |
|---|---|---|---|
| **Nginx Site Manager** | `5` / `views/nginx` | Scans `/etc/nginx/sites-available` and `sites-enabled`, toggles configuration symlinks (`e`/`Space`), executes dry-run validation (`nginx -t`), reloads daemon (`r`), and renders config viewports (`v`). | `os`, `os/exec`, `path/filepath` |
| **Auto-Nginx Templater** | `6` / `views/autonginx` | Automatic framework signature detection in `/var/www/*` for Laravel (`artisan`), Node/Next.js (`package.json`), Static SPAs (`dist/`), and WordPress (`wp-config.php`), generating production virtual hosts with FastCGI socket routing. | `text/template`, `os` |
| **PHP-FPM Pool Switcher** | `8` / `views/phpfpm` | Discovers installed PHP FastCGI sockets in `/run/php/` (PHP 7.4 through 8.3), manages pool daemon lifecycles (`systemctl restart php8.x-fpm`), and flushes OPcache / APCu bytecode caches (`c`). | `os`, `os/exec` |
| **Certbot SSL Provisioner** | `0` / `views/certbot` | Automated Let's Encrypt SSL wizard: DNS reachability pre-validation via `net.LookupIP`, dry-run ACME challenge testing (`t`), and non-interactive SSL provisioning (`p`) with automatic HTTP-to-HTTPS redirect injection. | `net`, `os/exec` |

---

### 4. CI/CD, Repositories & Task Automation

| Module | Identifier | Capabilities | Driver / Package |
|---|---|---|---|
| **Multi-Repo Deployment Hub** | `7` / `views/deploy` | Scans `/var/www/` for Git repositories, tracking active branch names, worktree dirty states, and commit synchronization. Dispatches fast-forward pulls (`p`) and automated zero-downtime release pipelines (`d`). | `os/exec`, `go-git/v5` |
| **Git Mini-Inspector** | `G` / `views/git` | Emulates `lazygit`: Branch inspection, commit log history, unified syntax-colored diff viewport (`d`), staging/unstaging (`s`/`u`), and branch stashing (`z`/`Z`). | `go-git/v5`, `os/exec` |
| **CI Pipeline Monitor** | `C` / `views/ci` | Integrates with GitHub Actions API: Polls latest repository workflow runs, pipeline status badges (success, failure, in-progress), trigger events, and detailed step metadata (`v`). | `google/go-github/v60` |
| **Cron & Systemd Timers** | `O` / `views/timers` | Aggregates user crontabs (`crontab -l`), system crontabs (`/etc/crontab`, `/etc/cron.d/*`), and systemd timers (`systemctl list-timers`) with next-run countdown calculations. | `robfig/cron/v3` |

---

### 5. Network Diagnostics & Security Auditing

| Module | Identifier | Capabilities | Driver / Package |
|---|---|---|---|
| **TCP Port Scanner** | `A` / `views/scanner` | Emulates `nmap` / `nc -zv`: Concurrent worker pool port scanning with configurable timeouts, preset target suites (Web, Databases, DevOps/Infra), banner grabbing, and latency (RTT) measurement. | `net`, `sync` |
| **SSL/TLS Certificate Inspector** | `L` / `views/ssl` | Connects to remote HTTPS endpoints or inspects local `.crt`/`.pem` files, validating SANs, issuer authority, cipher suites, TLS protocol negotiation, certificate chains, and visual expiration meters. | `crypto/tls`, `crypto/x509` |
| **SSH Session & Key Auditor** | `S` / `views/ssh` | Inspects active user login sessions with one-key session kill (`k` / `pkill -t`), and audits `~/.ssh/authorized_keys` public key types, SHA256 fingerprints, and identity comments. | `golang.org/x/crypto/ssh` |
| **HTTP Latency Tracer** | `H` / `views/httpclient` | Emulates `curl`: Configurable HTTP methods, header/body inputs, sub-millisecond connection phase waterfall breakdown (DNS, TCP, TLS, TTFB, Transfer), and formatted JSON response viewer. | `net/http/httptrace` |
| **DNS Inspector** | `N` / `views/dns` | Emulates `dig`: Query dispatch to upstream resolvers (Cloudflare, Google, Quad9, Local), record type toggles (A, AAAA, CNAME, MX, TXT, NS, SOA, SRV, PTR), TTL tracking, and round-trip timing. | `miekg/dns` |

---

### 6. DevOps Utilities & Terminal Multiplexing

| Module | Identifier | Capabilities | Driver / Package |
|---|---|---|---|
| **DevOps Swiss Army Knife** | `K` / `views/knife` | Offline utility suite: JWT claim decoder and expiration validator, Cron expression syntax evaluator, Base64/URL/Hex converters, and MD5/SHA256/bcrypt hash generators. | `golang-jwt/jwt/v5`, `robfig/cron/v3`, `bcrypt` |
| **Environment Drift Validator** | `E` / `views/env` | Compares `.env` configuration files against `.env.example` templates, identifying missing keys, unpopulated parameters, type divergences, and syntax anomalies. | `joho/godotenv` |
| **Embedded PTY Terminal** | `T` / `views/terminal` | Dedicated pseudo-terminal subshell runner (`$SHELL` / `/bin/bash`) running inside the dashboard with raw keystroke forwarding and instant escape bindings (`Ctrl+]`). | `creack/pty`, `os/exec` |
| **Database Monitor & Runner** | `B` / `views/database` | PostgreSQL and MySQL connection pool health, slow query inspection (`pg_stat_activity`), and ad-hoc SQL query execution with tabular result formatting. | `database/sql`, `pgx/v5`, `mysql` |

---

## Keybinding Contract

### Global Navigation

| Keybinding | Action |
|---|---|
| `1` - `9`, `0` | Direct navigation to numbered modules (1: Monitor to 0: Certbot) |
| `K`, `L`, `B`, `W`, `A`, `G`, `C`, `S`, `E`, `O`, `D`, `H`, `N`, `T` | Direct alpha shortcut to corresponding operational view |
| `Tab` / `Shift+Tab` | Sequential tab cycling forward / backward |
| `F1` - `F12` | Function key jumps (functional even when raw PTY subshell is focused) |
| `Ctrl+C` / `q` | Clean daemon shutdown and terminal restoration |

### Viewport & Process Controls

| Keybinding | Context | Action |
|---|---|---|
| `j` / `k` or `Up` / `Down` | Tables & Lists | Navigate row selection |
| `Enter` | Forms & Actions | Submit input / execute operational action |
| `k` | System / Ports / SSH | Terminate selected process / kill connection |
| `r` / `R` | All Views | Trigger asynchronous data refresh / daemon reload |
| `d` | Deploy / Disk / Containers | Run deployment pipeline / delete path / remove container |
| `i` | Embedded Terminal | Focus raw interactive PTY subshell input |
| `Ctrl+]` | Embedded Terminal | Unfocus PTY subshell and restore global TUI hotkeys |

---

## Installation & Build

### Requirements
- Linux (x86_64 / aarch64) or macOS
- Go toolchain 1.21 or later
- Optional host tools: `docker`, `systemd`, `nginx`, `certbot`, `git`, `supervisorctl`

### Compile Binary

```bash
# Clone repository
git clone https://github.com/your-org/dok-ops.git
cd dok-ops

# Verify module dependencies
go mod tidy
go vet ./...

# Build optimized binary
go build -ldflags="-s -w" -o bin/dok-ops .
```

### Execution

```bash
# Run standard unprivileged mode
./bin/dok-ops

# Run with system-level privileges for systemd/nginx/certbot operations
sudo ./bin/dok-ops
```

---

## License

MIT License. Designed for DevOps engineers, systems administrators, and infrastructure operators.
