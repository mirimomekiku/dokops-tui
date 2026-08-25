# Features Specification Matrix: dok-ops

Exhaustive functional specification for all 24 integrated DevOps and WebOps modules in `dok-ops`.

---

## Complete 24-Module Index

```
[1] System Monitor         [7] Multi-Repo Deploy Hub   [B] Database Monitor    [E] .Env Drift Validator
[2] Docker Manager         [8] PHP-FPM Switcher        [W] Live Bandwidth      [O] Timers & Cron
[3] Systemd Services       [9] Workers & Artisan       [A] TCP Port Scanner    [D] Disk Space Analyzer
[4] Listening Ports        [0] Certbot SSL Wizard      [G] Git Mini-Inspector  [H] HTTP Latency Tracer
[5] Nginx Site Manager     [K] Swiss Army Knife        [C] CI Pipeline Status  [N] DNS Inspector
[6] Auto-Nginx Templater   [L] SSL/TLS Inspector       [S] SSH Session Auditor [T] Embedded PTY Shell
```

---

## Detailed Module Specifications

### Module 1: System Monitor (`views/monitor`)
- **Keybinding**: `1`
- **Primary Function**: `htop` emulation with real-time CPU core bars, RAM/Swap meters, and process table.
- **Data Source**: `gopsutil/v3/cpu`, `mem`, `process`.
- **Keyboard Shortcuts**:
  - `j` / `k` (or `Up` / `Down`): Scroll process selection.
  - `c`: Sort processes descending by CPU utilization.
  - `m`: Sort processes descending by Memory utilization.
  - `p`: Sort processes numerically by Process ID (PID).
  - `n`: Sort processes alphabetically by Name.
  - `k`: Send `SIGTERM` signal to highlighted process.
  - `r`: Force immediate process list refresh.

---

### Module 2: Docker Container Manager (`views/containers`)
- **Keybinding**: `2`
- **Primary Function**: Inspect container lifecycles, statuses, ports, and tail logs.
- **Data Source**: Docker Engine API client via Unix socket (`/var/run/docker.sock`).
- **Keyboard Shortcuts**:
  - `j` / `k`: Select container from table.
  - `u` or `Enter`: Start stopped container (`client.ContainerStart`).
  - `s`: Stop running container (`client.ContainerStop`).
  - `r`: Restart container (`client.ContainerRestart`).
  - `l`: Open split-screen live log viewer for selected container.
  - `d`: Force delete container (`client.ContainerRemove`).
  - `R`: Refresh container table.

---

### Module 3: Systemd Service Manager (`views/services`)
- **Keybinding**: `3`
- **Primary Function**: Manage Linux systemd units and stream systemd journal logs.
- **Data Source**: DBus connection (`coreos/go-systemd/v22/dbus`) with CLI fallback.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select systemd unit.
  - `u` or `Enter`: Start unit (`systemctl start <unit>`).
  - `s`: Stop unit (`systemctl stop <unit>`).
  - `r`: Restart unit (`systemctl restart <unit>`).
  - `l`: Tail the last 50 lines of systemd journal logs (`journalctl -u <unit>`).
  - `f`: Toggle filter between Active units, Failed units, and All units.
  - `R`: Refresh unit list.

---

### Module 4: Listening Ports & Socket Mapper (`views/ports`)
- **Keybinding**: `4`
- **Primary Function**: Active socket inspector resolving local bind addresses, PIDs, and process names.
- **Data Source**: `gopsutil/v3/net.Connections` and `gopsutil/v3/process`.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select socket entry.
  - `k`: Terminate the bound process to release the port conflict (`bind: address already in use`).
  - `l`: Toggle between Listening-only sockets and All established connections.
  - `t` / `u`: Filter view by TCP or UDP protocols.
  - `r`: Rescan active network sockets.

---

### Module 5: Nginx Site Manager (`views/nginx`)
- **Keybinding**: `5`
- **Primary Function**: Reverse proxy virtual host manager.
- **Data Source**: `/etc/nginx/sites-available/` and `/etc/nginx/sites-enabled/`.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select virtual host config.
  - `e` or `Space`: Toggle enable/disable by creating or removing `/etc/nginx/sites-enabled/` symlinks.
  - `t`: Run dry-run configuration syntax test (`nginx -t`).
  - `r`: Send reload signal to Nginx daemon (`systemctl reload nginx`).
  - `v` or `Enter`: View raw virtual host configuration file in a scrollable viewport.
  - `R`: Rescan configuration directories.

---

### Module 6: Smart Nginx Auto-Templater (`views/autonginx`)
- **Keybinding**: `6`
- **Primary Function**: Framework auto-detector and production Nginx config generator.
- **Detection Signatures**:
  - **Laravel**: `artisan` $\rightarrow$ Root `/public`, FastCGI socket directives.
  - **Node/Next.js**: `package.json` $\rightarrow$ Reverse proxy upstream `http://127.0.0.1:{port}`.
  - **Static SPA**: `dist/` or `build/` $\rightarrow$ `try_files $uri $uri/ /index.html`.
  - **WordPress**: `wp-config.php` $\rightarrow$ Multisite rewrite rules & uploads protection.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select detected project in `/var/www/`.
  - `s`: Cycle target FastCGI PHP socket (PHP 8.3 down to PHP 7.4).
  - `g` or `Enter`: Generate config, write to `sites-available`, symlink `sites-enabled`, test syntax (`nginx -t`), and reload Nginx.
  - `r`: Rescan `/var/www/`.

---

### Module 7: Multi-Repo Git Deployment Hub (`views/deploy`)
- **Keybinding**: `7`
- **Primary Function**: Multi-repository tracking and zero-downtime release runner.
- **Data Source**: `/var/www/*/.git` inspection.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select repository.
  - `p`: Fast-forward pull (`git pull --ff-only`).
  - `d` or `Enter`: Run atomic deployment pipeline (Maintenance on, git pull, composer install, migrations, caches, asset build, maintenance off) in a scrollable output stream.
  - `r`: Rescan repositories.

---

### Module 8: PHP-FPM Pool & Version Switcher (`views/phpfpm`)
- **Keybinding**: `8`
- **Primary Function**: Multi-PHP socket discovery and daemon controller.
- **Data Source**: `/run/php/*.sock` and `systemctl`.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select installed PHP version.
  - `r` or `Enter`: Restart selected PHP-FPM daemon (`systemctl restart php8.x-fpm`).
  - `c`: Flush OPcache & APCu memory cache via CLI evaluation.
  - `R`: Rescan installed PHP sockets.

---

### Module 9: Background Workers & Task Scheduler (`views/workers`)
- **Keybinding**: `9`
- **Primary Function**: Supervisor/Horizon worker manager and Artisan schedule inspector.
- **Data Source**: `supervisorctl status` and `php artisan schedule:list`.
- **Keyboard Shortcuts**:
  - `Tab`: Switch between Supervisor Workers table and Artisan Schedule table.
  - `r` or `Enter`: Restart selected Supervisor worker process.
  - `l`: View tail of worker log file (`/var/log/supervisor/<name>.log`).
  - `R`: Refresh worker states and schedules.

---

### Module 10: SSL / Certbot Provisioning Wizard (`views/certbot`)
- **Keybinding**: `0`
- **Primary Function**: Automated Let's Encrypt SSL certificate provisioning.
- **Data Source**: `net.LookupIP` and `certbot` CLI.
- **Keyboard Shortcuts**:
  - `Tab`: Switch focus between Domain input and Admin Email input.
  - `t`: Run ACME dry-run challenge test (`--dry-run`) without consuming rate limits.
  - `p` or `Enter`: Execute live certificate issuance and configure automatic HTTP-to-HTTPS redirects.
  - `d`: Check DNS IP resolution for the target domain.

---

### Module 11: DevOps Swiss Army Knife ("CyberChef") (`views/knife`)
- **Keybinding**: `K`
- **Primary Function**: Offline conversion suite (JWT, Cron, Base64/Hex, Hashes).
- **Sub-Tools**:
  - `1`: **JWT Inspector**: Decodes headers, claims payload, and expiration timestamp.
  - `2`: **Cron Evaluator**: Evaluates cron strings and computes the next 5 execution timestamps.
  - `3`: **Base64 / URL / Hex**: Real-time bidirectional encoder and decoder.
  - `4`: **Hash Generator**: Real-time computation of MD5, SHA-1, SHA-256, and bcrypt hashes.

---

### Module 12: SSL/TLS Certificate Inspector (`views/ssl`)
- **Keybinding**: `L`
- **Primary Function**: Remote and local certificate validation (`s_client` alternative).
- **Capabilities**: Connects via `crypto/tls`, extracts Subject Alternative Names (SANs), Issuer authority, TLS version, negotiated Cipher Suite, Full Certificate Chain, and visual Expiration Meter.
- **Keyboard Shortcuts**:
  - `Enter`: Connect and inspect target host (`domain:port`) or local file path.
  - `j` / `k`: Scroll certificate details viewport.

---

### Module 13: Database Health & Query Runner (`views/database`)
- **Keybinding**: `B`
- **Primary Function**: PostgreSQL & MySQL connection pool monitoring and SQL query execution.
- **Data Source**: `database/sql` with `pgx/v5` and `mysql` drivers.
- **Keyboard Shortcuts**:
  - `Tab`: Switch between Connection URI input and SQL Query input.
  - `Enter`: Connect to database / execute SQL query.
  - `j` / `k`: Scroll tabular SQL results.

---

### Module 14: Live Bandwidth Monitor (`views/bandwidth`)
- **Keybinding**: `W`
- **Primary Function**: Real-time network interface ingress and egress rate monitor (`iftop` lite).
- **Data Source**: `gopsutil/v3/net.IOCounters`.
- **Keyboard Shortcuts**:
  - `r`: Resample bandwidth counters immediately.
  - `j` / `k`: Select interface from table.

---

### Module 15: TCP Port Scanner (`views/scanner`)
- **Keybinding**: `A`
- **Primary Function**: High-concurrency worker pool port scanner (`nmap` / `nc -zv` alternative).
- **Presets**: Web & APIs (80, 443, 8080, etc.), Databases (3306, 5432, 6379, etc.), DevOps/Infra (22, 2375, 6443, 9090), Top 20 Essential.
- **Keyboard Shortcuts**:
  - `h` / `l` (or `Left` / `Right`): Cycle through port presets.
  - `Enter` or `r`: Run concurrent scan with banner grabbing and latency measurement.
  - `j` / `k`: Scroll open ports table.

---

### Module 16: Git Mini-Inspector (`views/git`)
- **Keybinding**: `G`
- **Primary Function**: `lazygit` ultra-lite repository inspector.
- **Data Source**: `go-git/v5` and `git` CLI.
- **Keyboard Shortcuts**:
  - `Tab`: Switch focus between Modified Files table and Recent Commits table.
  - `d` or `Enter`: View syntax-colored diff in a dedicated viewport.
  - `s`: Stage selected file (`git add`).
  - `u`: Unstage selected file (`git reset HEAD`).
  - `z`: Stash uncommitted changes (`git stash`).
  - `Z`: Pop stash (`git stash pop`).
  - `r`: Refresh git state.

---

### Module 17: GitHub Actions / CI Runner Status (`views/ci`)
- **Keybinding**: `C`
- **Primary Function**: GitHub Actions workflow run monitor.
- **Data Source**: `github.com/google/go-github/v60` (reads `GITHUB_TOKEN` or `gh auth token`).
- **Keyboard Shortcuts**:
  - `Enter`: Search and list workflow runs for the specified repository.
  - `v`: Open detailed workflow run viewport (commit, event, timestamps, HTML URL).
  - `r`: Poll latest runs from GitHub API.

---

### Module 18: SSH Session & Key Auditor (`views/ssh`)
- **Keybinding**: `S`
- **Primary Function**: Active login session auditor and `~/.ssh/authorized_keys` scanner.
- **Keyboard Shortcuts**:
  - `Tab`: Switch between Active Login Sessions and Authorized SSH Keys.
  - `k`: Terminate active SSH session (`pkill -9 -t <tty>`) with confirmation dialog.
  - `r`: Refresh sessions and public keys.

---

### Module 19: Environment Drift & .env Validator (`views/env`)
- **Keybinding**: `E`
- **Primary Function**: Compare active `.env` against `.env.example` templates.
- **Data Source**: `github.com/joho/godotenv`.
- **Keyboard Shortcuts**:
  - `Tab`: Switch between Target `.env` input and Template `.example` input.
  - `Enter` or `r`: Compare files and categorize variables into Valid, Missing, Empty, and Extra.
  - `j` / `k`: Scroll variable table.

---

### Module 20: Cron & Systemd Timers Timeline (`views/timers`)
- **Keybinding**: `O`
- **Primary Function**: Consolidated schedule timeline for user crontabs, `/etc/crontab`, `/etc/cron.d/*`, and `systemctl list-timers`.
- **Data Source**: `robfig/cron/v3` evaluation.
- **Keyboard Shortcuts**:
  - `f`: Cycle filter (All Tasks / Crontabs Only / Systemd Timers Only).
  - `r`: Refresh schedules and recalculate countdowns.
  - `j` / `k`: Scroll timeline.

---

### Module 21: Disk Space Analyzer (`views/disk`)
- **Keybinding**: `D`
- **Primary Function**: `ncdu` hierarchical filesystem space analyzer.
- **Data Source**: `filepath.WalkDir`.
- **Keyboard Shortcuts**:
  - `j` / `k`: Select directory or file.
  - `Enter` or `l`: Open directory and index child nodes.
  - `Backspace` or `h`: Navigate to parent directory.
  - `s`: Toggle sorting between size descending and name alphabetical.
  - `r`: Rescan active path.
  - `d`: Delete selected file or folder with confirmation.

---

### Module 22: HTTP Latency Tracer (`views/httpclient`)
- **Keybinding**: `H`
- **Primary Function**: `curl` timing waterfall tracer and response inspector.
- **Data Source**: `net/http/httptrace`.
- **Keyboard Shortcuts**:
  - `Tab`: Cycle focus through URL input, Method selector, Header editor, and Body editor.
  - `h` / `l`: Cycle HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD).
  - `Enter`: Execute HTTP request and display latency breakdown (DNS, TCP, TLS, TTFB, Transfer).
  - `j` / `k`: Scroll formatted JSON response body.

---

### Module 23: DNS Record Inspector (`views/dns`)
- **Keybinding**: `N`
- **Primary Function**: `dig` multi-nameserver query inspector.
- **Data Source**: `github.com/miekg/dns`.
- **Keyboard Shortcuts**:
  - `Tab`: Cycle focus between Domain input, Record Type toggle, and Nameserver selector.
  - `h` / `l`: Change Record Type (A, AAAA, CNAME, MX, TXT, NS, SOA, SRV, PTR) or Upstream Nameserver (Cloudflare, Google, Quad9, Local, Custom).
  - `Enter`: Dispatch DNS query and display answers, TTLs, and round-trip times.
  - `j` / `k`: Scroll records table.

---

### Module 24: Embedded PTY Terminal (`views/terminal`)
- **Keybinding**: `T`
- **Primary Function**: Native multiplexed pseudo-terminal subshell runner (`$SHELL` / `/bin/bash`).
- **Data Source**: `creack/pty`.
- **Keyboard Shortcuts**:
  - `i`: Focus subshell and enable raw keystroke streaming.
  - `Ctrl+]`: Unfocus subshell and restore global dashboard navigation shortcuts.
  - `F1` - `F12`: Function key shortcuts (always active regardless of shell focus).
  - `Ctrl+C`: Forwards `SIGINT` to the child shell process when focused; gracefully quits `dok-ops` when unfocused.
